package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nwaples/rardecode/v2"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// ---- Archive files (zip/rar) browsed as folders ----
//
// Files inside a .zip or .rar are addressed with virtual paths that extend
// the archive's real path with the entry path, e.g. /media/photos.zip/a/b.jpg.
// Paths stay opaque strings, so auth checks, position/favorite keys and the
// templates all work unchanged; anything that must read bytes goes through
// resolveMediaPath, which extracts the entry into an on-disk cache.

const archiveCacheDir = "/var/lib/filebrowser/zipcache"

// Extraction copies whole entries out of archives; cap concurrent ones.
var archiveSem = make(chan struct{}, 3)

// splitArchivePath splits a virtual archive path into the archive file and
// the entry path inside it ("" means the archive root). ok is false when no
// path component is an existing regular archive file.
func splitArchivePath(absPath string) (archiveFile, inner string, ok bool) {
	for p := absPath; ; {
		if archiveExts[strings.ToLower(filepath.Ext(p))] {
			if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
				return p, strings.TrimPrefix(strings.TrimPrefix(absPath, p), "/"), true
			}
		}
		parent := filepath.Dir(p)
		if parent == p {
			return "", "", false
		}
		p = parent
	}
}

// extractArchiveEntry extracts one entry into the cache and returns the
// cached file's path. The key covers the archive's mtime, so an updated
// archive never serves stale content; cache names are hashes, so a hostile
// entry name can't escape the cache dir (zip-slip). The entry's extension is
// kept so ServeFile sniffs the right Content-Type.
func (a *App) extractArchiveEntry(ctx context.Context, archiveFile, inner string) (string, error) {
	info, err := os.Stat(archiveFile)
	if err != nil {
		return "", err
	}
	key := fmt.Sprintf("%s|%d|%s", archiveFile, info.ModTime().UnixNano(), inner)
	h := sha256.Sum256([]byte(key))
	cacheFile := filepath.Join(archiveCacheDir, hex.EncodeToString(h[:])+strings.ToLower(filepath.Ext(inner)))
	if _, err := os.Stat(cacheFile); err == nil {
		now := time.Now()
		os.Chtimes(cacheFile, now, now) // keep hot entries alive across cleanup
		return cacheFile, nil
	}

	select {
	case archiveSem <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	defer func() { <-archiveSem }()
	// Another request may have extracted it while we waited on the semaphore.
	if _, err := os.Stat(cacheFile); err == nil {
		return cacheFile, nil
	}

	if err := os.MkdirAll(archiveCacheDir, 0755); err != nil {
		return "", err
	}
	// Unique temp name: two requests for the same entry can race past the
	// cache check above; each writes its own file and rename is atomic.
	tmp, err := os.CreateTemp(archiveCacheDir, "extract-*.tmp")
	if err != nil {
		return "", err
	}
	if strings.EqualFold(filepath.Ext(archiveFile), ".rar") {
		err = copyRarEntry(archiveFile, inner, tmp)
	} else {
		err = copyZipEntry(archiveFile, inner, tmp)
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), cacheFile)
	}
	if err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return cacheFile, nil
}

// copyZipEntry writes a single zip entry's bytes to dst.
func copyZipEntry(zipFile, inner string, dst io.Writer) error {
	zr, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		// inner comes from a Clean-ed absolute path, so it never contains
		// ".."; traversal-named entries simply never match.
		if f.FileInfo().IsDir() || archiveEntryName(f.Name) != inner {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		_, err = io.Copy(dst, rc)
		rc.Close()
		return err
	}
	return fmt.Errorf("entry %q not found in %s: %w", inner, zipFile, os.ErrNotExist)
}

// copyRarEntry writes a single rar entry's bytes to dst. Rar entries are
// only decodable through a sequential scan from the start of the archive
// (solid archives chain each entry's decompression state onto the last), so
// unlike zip there's no direct seek-to-entry shortcut.
func copyRarEntry(rarFile, inner string, dst io.Writer) error {
	rr, err := rardecode.OpenReader(rarFile)
	if err != nil {
		return err
	}
	defer rr.Close()
	for {
		hdr, err := rr.Next()
		if err == io.EOF {
			return fmt.Errorf("entry %q not found in %s: %w", inner, rarFile, os.ErrNotExist)
		}
		if err != nil {
			return err
		}
		if hdr.IsDir || archiveEntryName(hdr.Name) != inner {
			continue
		}
		_, err = io.Copy(dst, rr)
		return err
	}
}

// archiveEntryName normalizes an archive entry name for comparison with
// inner paths (drops "./" and trailing slashes; keeps ".." intact so
// traversal names never match a clean inner path).
func archiveEntryName(name string) string {
	return strings.TrimPrefix(path.Clean(normalizeArchiveName(name)), "/")
}

// normalizeArchiveName guarantees a valid UTF-8 name. Zip/rar entries from
// tools that don't set the Unicode/UTF-8 flag (common with East-Asian
// filenames from older Windows tools) store raw legacy-codepage bytes
// verbatim — archive/zip and rardecode both pass those bytes through
// unchanged. Left as invalid UTF-8, the name breaks the moment it round-trips
// through a browser: the HTML parser (following the page's UTF-8 charset)
// replaces each undecodable byte with U+FFFD while building the DOM, so the
// string a click sends back no longer matches the archive's real entry name
// and listArchiveDir's prefix match fails silently — the folder renders as
// empty even though it has files. Guessing GBK/Shift-JIS fixes the *display*;
// falling back to ToValidUTF8 guarantees the *round-trip* either way, so even
// a wrong guess keeps browsing working, just ugly.
func normalizeArchiveName(name string) string {
	if utf8.ValidString(name) {
		return name
	}
	for _, enc := range []encoding.Encoding{simplifiedchinese.GBK, japanese.ShiftJIS} {
		if decoded, err := enc.NewDecoder().String(name); err == nil && utf8.ValidString(decoded) && !strings.ContainsRune(decoded, utf8.RuneError) {
			return decoded
		}
	}
	return strings.ToValidUTF8(name, "�")
}

// archiveEntry is a format-agnostic view of one archive entry, used by
// listArchiveDir and findArchiveAlbumArt so the directory-synthesis and
// cover-art-scan logic runs identically over zip and rar archives.
type archiveEntry struct {
	Name    string // normalized, "/"-separated, via archiveEntryName
	IsDir   bool
	Size    int64
	ModTime time.Time
}

// listArchiveEntries enumerates every entry in a zip or rar archive.
// Header-only for both formats (no decompression), so it's cheap even for
// solid rar archives, which only need a sequential decode when extracting a
// single entry's bytes.
func listArchiveEntries(archiveFile string) ([]archiveEntry, error) {
	if strings.EqualFold(filepath.Ext(archiveFile), ".rar") {
		return rarEntries(archiveFile)
	}
	return zipEntries(archiveFile)
}

func zipEntries(zipFile string) ([]archiveEntry, error) {
	zr, err := zip.OpenReader(zipFile)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	out := make([]archiveEntry, 0, len(zr.File))
	for _, f := range zr.File {
		out = append(out, archiveEntry{
			Name:    archiveEntryName(f.Name),
			IsDir:   f.FileInfo().IsDir(),
			Size:    int64(f.UncompressedSize64),
			ModTime: f.Modified,
		})
	}
	return out, nil
}

func rarEntries(rarFile string) ([]archiveEntry, error) {
	files, err := rardecode.List(rarFile)
	if err != nil {
		return nil, err
	}
	out := make([]archiveEntry, 0, len(files))
	for _, f := range files {
		out = append(out, archiveEntry{
			Name:    archiveEntryName(f.Name),
			IsDir:   f.IsDir,
			Size:    f.UnPackedSize,
			ModTime: f.ModificationTime,
		})
	}
	return out, nil
}

// findArchiveAlbumArt mirrors findAlbumArt's rules but scans an archive's
// top-level entries instead of a directory, so an archive full of photos
// gets a cover thumbnail in the browse grid just like a folder does.
func findArchiveAlbumArt(archiveFile string) string {
	entries, err := listArchiveEntries(archiveFile)
	if err != nil {
		return ""
	}
	var first string
	for _, e := range entries {
		if e.Name == "" || strings.Contains(e.Name, "/") || e.IsDir {
			continue
		}
		ext := strings.ToLower(path.Ext(e.Name))
		if !photoExts[ext] {
			continue
		}
		virt := archiveFile + "/" + e.Name
		base := strings.ToLower(strings.TrimSuffix(e.Name, ext))
		if base == "cover" || base == "folder" || base == "album" || base == "front" {
			return virt
		}
		if first == "" {
			first = virt
		}
	}
	return first
}
