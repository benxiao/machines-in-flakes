package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

var photoExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".webp": true, ".bmp": true, ".tiff": true, ".heic": true,
}
var videoExts = map[string]bool{
	".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
	".wmv": true, ".webm": true, ".flv": true, ".m4v": true,
}
var audioExts = map[string]bool{
	".mp3": true, ".flac": true, ".ogg": true, ".wav": true,
	".aac": true, ".m4a": true, ".opus": true, ".wma": true,
}
var textExts = map[string]bool{
	".txt": true, ".md": true, ".json": true, ".yaml": true, ".yml": true,
	".toml": true, ".xml": true, ".html": true, ".css": true, ".js": true,
	".go": true, ".py": true, ".sh": true, ".csv": true, ".log": true,
}

// archiveExts are the archive formats browsable as virtual folders (see
// "Archive files browsed as folders" below).
var archiveExts = map[string]bool{".zip": true, ".rar": true}

func classifyExt(ext string) string {
	ext = strings.ToLower(ext)
	switch {
	case photoExts[ext]:
		return "photo"
	case videoExts[ext]:
		return "video"
	case audioExts[ext]:
		return "audio"
	case ext == ".pdf":
		return "pdf"
	case archiveExts[ext]:
		return "archive"
	case textExts[ext]:
		return "text"
	default:
		return "other"
	}
}

func httpErr(w http.ResponseWriter, err error, code int) {
	log.Printf("http error: %v", err)
	http.Error(w, http.StatusText(code), code)
}

func parseID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

func uid(r *http.Request) int64 {
	id, _ := r.Context().Value(ctxUserID).(int64)
	return id
}

func formatSize(n int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.1f GB", float64(n)/gb)
	case n >= mb:
		return fmt.Sprintf("%.1f MB", float64(n)/mb)
	case n >= kb:
		return fmt.Sprintf("%.1f KB", float64(n)/kb)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// allowedRoots returns the current user's enabled indexed roots, either
// directly owned (admin) or via path_grants (non-admin).
func (a *App) allowedRoots(r *http.Request) []string {
	rows, err := a.db.Query(r.Context(), `
		SELECT ip.path FROM indexed_paths ip
		WHERE ip.enabled = TRUE
		  AND (ip.user_id = $1
		       OR EXISTS (SELECT 1 FROM path_grants pg WHERE pg.path_id = ip.id AND pg.user_id = $1))
	`, uid(r))
	if err != nil {
		return nil
	}
	defer rows.Close()
	var roots []string
	for rows.Next() {
		var p string
		if rows.Scan(&p) == nil {
			roots = append(roots, p)
		}
	}
	return roots
}

func underRoot(path, root string) bool {
	return path == root || strings.HasPrefix(path, root+"/")
}

// isAllowedPath checks that absPath is equal to or under one of the current
// user's allowed roots. Symlinked components must also resolve to somewhere
// inside an allowed root, so a link planted in a media tree can't reach out.
func (a *App) isAllowedPath(r *http.Request, absPath string) bool {
	roots := a.allowedRoots(r)
	matched := false
	for _, root := range roots {
		if underRoot(absPath, root) {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}
	target := absPath
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		// Virtual archive paths don't exist on disk (traversing into the
		// archive file yields ENOTDIR, a missing sibling ENOENT); the lexical
		// check above already covered the full virtual path, so the symlink
		// containment check runs against the archive file itself.
		archiveFile, _, ok := splitArchivePath(target)
		if !ok || !(errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)) {
			return false
		}
		target = archiveFile
		resolved, err = filepath.EvalSymlinks(target)
		if err != nil {
			return false
		}
	}
	if resolved == target {
		return true
	}
	for _, root := range roots {
		rr, err := filepath.EvalSymlinks(root)
		if err != nil {
			continue
		}
		if underRoot(resolved, rr) {
			return true
		}
	}
	return false
}

func (a *App) resolveMediaPath(ctx context.Context, absPath string) (string, error) {
	if _, err := os.Stat(absPath); err == nil {
		return absPath, nil
	}
	archiveFile, inner, ok := splitArchivePath(absPath)
	if !ok || inner == "" {
		return "", os.ErrNotExist
	}
	return a.extractArchiveEntry(ctx, archiveFile, inner)
}
