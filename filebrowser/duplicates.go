package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// ---- Duplicate finder ----
//
// An on-demand, admin-triggered scan rather than part of the periodic
// reindex — hashing full file content of a large library every cycle would
// be wasteful for what's meant to be an occasional cleanup action. Results
// are transient: held in memory, recomputed each time a scan runs.

type DuplicateGroup struct {
	SizeBytes int64
	Size      string
	Paths     []string
}

type dupScanResult struct {
	ScannedAt time.Time
	Groups    []DuplicateGroup
}

// Hashing is sequential full-file reads that can be large (video); cap
// concurrency so a scan doesn't thrash disk/ZFS throughput.
var dupHashSem = make(chan struct{}, 2)

func hashFile(ctx context.Context, path string) (string, error) {
	select {
	case dupHashSem <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	defer func() { <-dupHashSem }()
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// sameInstant compares mtimes at microsecond granularity. os.FileInfo.ModTime
// is nanosecond-precision but Postgres TIMESTAMPTZ only stores microseconds,
// so a plain time.Time.Equal against a value round-tripped through the DB
// would mismatch on the trailing digits almost every time — silently making
// every cached hash look stale and defeating the cache entirely.
func sameInstant(a, b time.Time) bool {
	return a.UnixMicro() == b.UnixMicro()
}

// dupFile is a walked file's identity for hash-caching purposes: path, size,
// and mtime together decide whether a cached hash in file_hashes is still
// valid (see hashCandidates).
type dupFile struct {
	path  string
	size  int64
	mtime time.Time
}

// hashCandidates resolves SHA-256 for every file in candidates via one
// batched file_hashes lookup, computing+persisting fresh hashes only for
// cache misses (no cached row, or size/mtime drifted since caching). Returns
// path -> hash for every file that hashed successfully; open/read failures
// are silently omitted, leaving that file absent from the result.
func (a *App) hashCandidates(ctx context.Context, candidates []dupFile) map[string]string {
	var candidatePaths []string
	for _, f := range candidates {
		candidatePaths = append(candidatePaths, f.path)
	}
	type cachedHash struct {
		size   int64
		mtime  time.Time
		sha256 string
	}
	cache := make(map[string]cachedHash, len(candidatePaths))
	if len(candidatePaths) > 0 {
		if rows, err := a.db.Query(ctx,
			`SELECT path, size, mtime, sha256 FROM file_hashes WHERE path = ANY($1)`,
			candidatePaths); err != nil {
			log.Printf("hash cache lookup: %v", err)
		} else {
			for rows.Next() {
				var c cachedHash
				var path string
				if rows.Scan(&path, &c.size, &c.mtime, &c.sha256) == nil {
					cache[path] = c
				}
			}
			rows.Close()
		}
	}

	out := make(map[string]string, len(candidates))
	for _, f := range candidates {
		if c, ok := cache[f.path]; ok && c.size == f.size && sameInstant(c.mtime, f.mtime) {
			out[f.path] = c.sha256
			continue
		}
		h, err := hashFile(ctx, f.path)
		if err != nil {
			continue
		}
		if _, err := a.db.Exec(ctx, `
			INSERT INTO file_hashes (path, size, mtime, sha256, hashed_at)
			VALUES ($1, $2, $3, $4, now())
			ON CONFLICT (path) DO UPDATE
			  SET size = EXCLUDED.size, mtime = EXCLUDED.mtime,
			      sha256 = EXCLUDED.sha256, hashed_at = EXCLUDED.hashed_at
		`, f.path, f.size, f.mtime, h); err != nil {
			// Persistence for next time failed, but we still have a valid
			// hash for this run — don't drop the file.
			log.Printf("hash cache: persist %s: %v", f.path, err)
		}
		out[f.path] = h
	}
	return out
}

// scanDuplicates walks the user's enabled roots, grouping files by size
// first (cheap, stat-only) and only hashing files that share a size with at
// least one other file — most files in a real library are unique sizes, so
// this prunes the expensive step drastically.
func (a *App) scanDuplicates(ctx context.Context, userID int64) {
	if _, running := a.dupScanning.LoadOrStore(userID, true); running {
		return
	}
	defer a.dupScanning.Delete(userID)

	rows, err := a.db.Query(ctx, `
		SELECT ip.path FROM indexed_paths ip
		WHERE ip.enabled = TRUE
		  AND (ip.user_id = $1 OR EXISTS (SELECT 1 FROM path_grants pg WHERE pg.path_id = ip.id AND pg.user_id = $1))
	`, userID)
	if err != nil {
		log.Printf("dup scan: query paths: %v", err)
		return
	}
	var roots []string
	for rows.Next() {
		var p string
		if rows.Scan(&p) == nil {
			roots = append(roots, p)
		}
	}
	rows.Close()

	bySize := make(map[int64][]dupFile)
	seen := make(map[string]bool) // nested roots walk the same files twice
	for _, root := range roots {
		filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				if info.Name() == trashDirName {
					return filepath.SkipDir
				}
				return nil
			}
			if seen[p] || info.Size() == 0 {
				return nil
			}
			seen[p] = true
			bySize[info.Size()] = append(bySize[info.Size()], dupFile{p, info.Size(), info.ModTime()})
			return nil
		})
	}

	// Cached hashes let a rescan skip re-reading files that haven't changed
	// since the last scan (by path+size+mtime) — the dominant cost on a
	// large library of unique-sized-but-still-colliding video files.
	var candidates []dupFile
	for _, files := range bySize {
		if len(files) < 2 {
			continue
		}
		candidates = append(candidates, files...)
	}
	hashes := a.hashCandidates(ctx, candidates)

	type hashKey struct {
		size int64
		hash string
	}
	byHash := make(map[hashKey][]string)
	for _, f := range candidates {
		h, ok := hashes[f.path]
		if !ok {
			continue
		}
		k := hashKey{f.size, h}
		byHash[k] = append(byHash[k], f.path)
	}

	var groups []DuplicateGroup
	for k, paths := range byHash {
		if len(paths) < 2 {
			continue
		}
		sort.Strings(paths)
		groups = append(groups, DuplicateGroup{SizeBytes: k.size, Size: formatSize(k.size), Paths: paths})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].SizeBytes > groups[j].SizeBytes })

	a.dupResults.Store(userID, &dupScanResult{ScannedAt: time.Now(), Groups: groups})
	log.Printf("dup scan: user %d found %d duplicate group(s)", userID, len(groups))
}

// handleFileHashMatches finds every file across the user's whole library
// (not just the current folder) that shares the exact content hash of the
// given path, including the path itself — the lookup that backs the browse
// page's "Delete All Copies" button. Unlike scanDuplicates (a whole-library,
// background-triggered scan of every size collision), this always does a
// fresh walk scoped to one target size, so it can run synchronously.
func (a *App) handleFileHashMatches(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	absPath, err := a.fileOpCheck(r, req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	info, err := os.Stat(absPath)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if info.Size() == 0 {
		// Every empty file hashes identically, so matching by hash here would
		// pull in every unrelated empty/placeholder file in the library —
		// scanDuplicates excludes zero-byte files from consideration for the
		// same reason.
		http.Error(w, "cannot match by hash: empty file", http.StatusBadRequest)
		return
	}
	targetSize := info.Size()

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	var candidates []dupFile
	seen := make(map[string]bool) // nested/overlapping roots walk the same files twice
	targetSeen := false
	for _, root := range a.allowedRoots(r) {
		filepath.Walk(root, func(p string, walkInfo os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if walkInfo.IsDir() {
				if walkInfo.Name() == trashDirName {
					return filepath.SkipDir
				}
				return nil
			}
			if seen[p] || walkInfo.Size() != targetSize {
				return nil
			}
			seen[p] = true
			if p == absPath {
				targetSeen = true
			}
			candidates = append(candidates, dupFile{p, targetSize, walkInfo.ModTime()})
			return nil
		})
	}
	if !targetSeen {
		// filepath.Walk doesn't follow symlinks, but isAllowedPath (via
		// fileOpCheck above) does resolve symlinked roots/components — a
		// symlinked indexed root or intermediate dir means the target can be
		// allowed yet never turn up in this walk. Include it directly so the
		// hash comparison below still works instead of reporting 0 matches.
		candidates = append(candidates, dupFile{absPath, targetSize, info.ModTime()})
	}

	hashes := a.hashCandidates(ctx, candidates)
	if ctx.Err() != nil {
		// The walk/hash pass didn't finish before the deadline. This lookup
		// exists specifically to guarantee no copy is missed before a bulk
		// delete, so a partial match list here must not look like a complete
		// one — fail loudly instead of returning what was found so far.
		http.Error(w, "search timed out before checking the whole library; try again", http.StatusGatewayTimeout)
		return
	}
	targetHash, ok := hashes[absPath]
	if !ok {
		httpErr(w, fmt.Errorf("could not hash %s", absPath), 500)
		return
	}
	var matches []string
	for _, f := range candidates {
		if hashes[f.path] == targetHash {
			matches = append(matches, f.path)
		}
	}
	sort.Strings(matches)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"paths": matches})
}

func (a *App) handleDuplicatesPage(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	render(w, "duplicates", DuplicatesPage{ActiveTab: "duplicates", IsAdmin: true})
}

func (a *App) handleDuplicatesScan(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	userID := uid(r)
	go func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		a.scanDuplicates(ctx2, userID)
	}()
	w.WriteHeader(http.StatusNoContent)
}

// handleDuplicatesStatus re-verifies each cached group against disk before
// returning it, so files removed since the last scan (e.g. moved to Trash
// via this same page) don't linger in the report until the next rescan.
func (a *App) handleDuplicatesStatus(w http.ResponseWriter, r *http.Request) {
	userID := uid(r)
	_, running := a.dupScanning.Load(userID)
	resp := map[string]any{"scanning": running}
	if v, ok := a.dupResults.Load(userID); ok {
		res := v.(*dupScanResult)
		var live []DuplicateGroup
		var wasted int64
		for _, g := range res.Groups {
			var paths []string
			for _, p := range g.Paths {
				if _, err := os.Stat(p); err == nil {
					paths = append(paths, p)
				}
			}
			if len(paths) < 2 {
				continue
			}
			live = append(live, DuplicateGroup{SizeBytes: g.SizeBytes, Size: g.Size, Paths: paths})
			wasted += g.SizeBytes * int64(len(paths)-1)
		}
		resp["scanned_at"] = res.ScannedAt.Local().Format("2006-01-02 15:04")
		resp["groups"] = live
		resp["wasted_bytes"] = wasted
		resp["wasted"] = formatSize(wasted)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
