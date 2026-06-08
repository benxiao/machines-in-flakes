package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
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

// isAllowedPath checks that absPath is equal to or under one of the current user's enabled indexed roots,
// either directly owned (admin) or via path_grants (non-admin).
func (a *App) isAllowedPath(r *http.Request, absPath string) bool {
	var count int
	err := a.db.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM indexed_paths ip
		WHERE ip.enabled = TRUE
		  AND ($2 = ip.path OR starts_with($2, ip.path || '/'))
		  AND (ip.user_id = $1
		       OR EXISTS (SELECT 1 FROM path_grants pg WHERE pg.path_id = ip.id AND pg.user_id = $1))
	`, uid(r), absPath).Scan(&count)
	return err == nil && count > 0
}

func (a *App) handleBrowse(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := uid(r)

	// Always load sidebar paths (user's own or granted).
	pRows, err := a.db.Query(ctx, `
		SELECT ip.id, ip.path, ip.enabled FROM indexed_paths ip
		WHERE ip.enabled = TRUE
		  AND (ip.user_id = $1 OR EXISTS (SELECT 1 FROM path_grants pg WHERE pg.path_id = ip.id AND pg.user_id = $1))
		ORDER BY ip.path`, userID)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	var sidebarPaths []PathRow
	for pRows.Next() {
		var p PathRow
		if pRows.Scan(&p.ID, &p.Path, &p.Enabled) == nil {
			sidebarPaths = append(sidebarPaths, p)
		}
	}
	pRows.Close()

	dirParam := r.URL.Query().Get("dir")
	if dirParam == "" {
		if len(sidebarPaths) > 0 {
			target := "/browse?dir=" + url.QueryEscape(sidebarPaths[0].Path)
			if s := r.URL.Query().Get("sort"); s != "" {
				target += "&sort=" + url.QueryEscape(s)
			}
			http.Redirect(w, r, target, http.StatusFound)
			return
		}
		render(w, "browse", BrowsePage{ActiveTab: "browse", IsAdmin: isAdmin(r), Paths: sidebarPaths})
		return
	}

	dirParam = filepath.Clean(dirParam)
	if !a.isAllowedPath(r, dirParam) {
		http.NotFound(w, r)
		return
	}

	// Derive current root from sidebar paths (longest matching prefix).
	var rootPath string
	for _, p := range sidebarPaths {
		if (dirParam == p.Path || strings.HasPrefix(dirParam, p.Path+"/")) && len(p.Path) > len(rootPath) {
			rootPath = p.Path
		}
	}

	entries, err := os.ReadDir(dirParam)
	if err != nil {
		httpErr(w, err, 500)
		return
	}

	sortBy := r.URL.Query().Get("sort") // "" or "name" = alphabetical; "date" = newest first

	var subdirs []SubdirRow
	var files []FileRow
	for _, e := range entries {
		if e.IsDir() {
			absDir := filepath.Join(dirParam, e.Name())
			row := SubdirRow{AbsPath: absDir, Name: e.Name(), AlbumArt: findAlbumArt(absDir)}
			if info, err := e.Info(); err == nil {
				row.ModTime = info.ModTime()
				row.ModifiedAt = info.ModTime().Format("2006-01-02 15:04")
			}
			subdirs = append(subdirs, row)
			continue
		}
		info, err := e.Info()
		if err != nil {
			log.Printf("stat %s/%s: %v", dirParam, e.Name(), err)
			continue
		}
		ext := filepath.Ext(e.Name())
		files = append(files, FileRow{
			AbsPath:    filepath.Join(dirParam, e.Name()),
			Filename:   e.Name(),
			Extension:  strings.ToLower(ext),
			FileType:   classifyExt(ext),
			SizeBytes:  info.Size(),
			Size:       formatSize(info.Size()),
			ModifiedAt: info.ModTime().Format("2006-01-02 15:04"),
			ModTime:    info.ModTime(),
		})
	}
	if sortBy == "date" {
		sort.Slice(subdirs, func(i, j int) bool { return subdirs[i].ModTime.After(subdirs[j].ModTime) })
		sort.Slice(files, func(i, j int) bool { return files[i].ModTime.After(files[j].ModTime) })
	} else {
		sort.Slice(subdirs, func(i, j int) bool { return subdirs[i].Name < subdirs[j].Name })
		sort.Slice(files, func(i, j int) bool {
			return strings.ToLower(files[i].Filename) < strings.ToLower(files[j].Filename)
		})
	}

	// Batch-fetch watch counts for video and audio files.
	var mediaPaths []string
	for _, f := range files {
		if f.FileType == "video" || f.FileType == "audio" {
			mediaPaths = append(mediaPaths, f.AbsPath)
		}
	}
	if len(mediaPaths) > 0 {
		wcRows, err := a.db.Query(ctx, `SELECT path, watch_count FROM video_positions WHERE user_id = $1 AND path = ANY($2)`, userID, mediaPaths)
		if err == nil {
			wc := make(map[string]int64)
			for wcRows.Next() {
				var p string
				var c int64
				if wcRows.Scan(&p, &c) == nil {
					wc[p] = c
				}
			}
			wcRows.Close()
			for i := range files {
				if files[i].FileType == "video" || files[i].FileType == "audio" {
					files[i].WatchCount = wc[files[i].AbsPath]
				}
			}
		}
	}

	albumArt := findAlbumArt(dirParam)

	plRows, _ := a.db.Query(ctx, `SELECT id, name FROM playlists WHERE user_id = $1 ORDER BY name`, userID)
	var pls []PlaylistRow
	if plRows != nil {
		for plRows.Next() {
			var pl PlaylistRow
			if plRows.Scan(&pl.ID, &pl.Name) == nil {
				pls = append(pls, pl)
			}
		}
		plRows.Close()
	}
	plJSON, _ := json.Marshal(pls)

	render(w, "browse", BrowsePage{
		ActiveTab:     "browse",
		IsAdmin:       isAdmin(r),
		Paths:         sidebarPaths,
		CurrentRoot:   rootPath,
		Dir:           dirParam,
		DirName:       filepath.Base(dirParam),
		Breadcrumbs:   buildBreadcrumb(dirParam, rootPath),
		Subdirs:       subdirs,
		Files:         files,
		Playlists:     pls,
		PlaylistsJSON: template.JS(plJSON),
		DirAlbumArt:   albumArt,
		SortBy:        sortBy,
	})
}

func (a *App) handleRecent(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `
		SELECT path, watch_count, updated_at, position_sec
		FROM video_positions
		WHERE user_id = $1
		  AND EXISTS (
			SELECT 1 FROM indexed_paths ip
			WHERE ip.user_id = $1 AND ip.enabled = TRUE
			  AND (video_positions.path = ip.path OR starts_with(video_positions.path, ip.path || '/'))
		  )
		ORDER BY updated_at DESC
		LIMIT 50
	`, uid(r))
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	defer rows.Close()
	artCache := make(map[string]string)
	var items []RecentItem
	for rows.Next() {
		var path string
		var wc int64
		var t time.Time
		var pos float64
		if rows.Scan(&path, &wc, &t, &pos) != nil {
			continue
		}
		ft := classifyExt(filepath.Ext(path))
		dir := filepath.Dir(path)
		var art string
		if ft == "audio" {
			if cached, ok := artCache[dir]; ok {
				art = cached
			} else {
				art = findAlbumArt(dir)
				artCache[dir] = art
			}
		}
		items = append(items, RecentItem{
			Path:        path,
			Filename:    filepath.Base(path),
			FileType:    ft,
			Dir:         dir,
			WatchCount:  wc,
			UpdatedAt:   t.Local().Format("2006-01-02 15:04"),
			PositionSec: pos,
			AlbumArt:    art,
		})
	}
	render(w, "recent", RecentPage{ActiveTab: "recent", IsAdmin: isAdmin(r), Items: items})
}

func (a *App) handleUnplayed(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `
		SELECT fi.path, fi.filename, fi.file_type, fi.dir_path
		FROM file_index fi
		WHERE fi.user_id = $1
		  AND fi.file_type IN ('video', 'audio')
		  AND NOT EXISTS (
		    SELECT 1 FROM video_positions vp
		    WHERE vp.user_id = fi.user_id AND vp.path = fi.path
		      AND (vp.watch_count > 0 OR vp.position_sec > 0)
		  )
		ORDER BY lower(fi.filename)
		LIMIT 500
	`, uid(r))
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	defer rows.Close()
	artCache := make(map[string]string)
	var items []UnplayedItem
	for rows.Next() {
		var path, filename, fileType, dir string
		if rows.Scan(&path, &filename, &fileType, &dir) != nil {
			continue
		}
		var art string
		if fileType == "audio" {
			if cached, ok := artCache[dir]; ok {
				art = cached
			} else {
				art = findAlbumArt(dir)
				artCache[dir] = art
			}
		}
		items = append(items, UnplayedItem{Path: path, Filename: filename, FileType: fileType, Dir: dir, AlbumArt: art})
	}
	if err := rows.Err(); err != nil {
		httpErr(w, err, 500)
		return
	}
	render(w, "unplayed", UnplayedPage{ActiveTab: "unplayed", IsAdmin: isAdmin(r), Items: items})
}

func findAlbumArt(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var first string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !photoExts[ext] {
			continue
		}
		absPath := filepath.Join(dir, e.Name())
		base := strings.ToLower(strings.TrimSuffix(e.Name(), ext))
		if base == "cover" || base == "folder" || base == "album" || base == "front" {
			return absPath
		}
		if first == "" {
			first = absPath
		}
	}
	return first
}

func buildBreadcrumb(dir, root string) []Breadcrumb {
	crumbs := []Breadcrumb{{Name: filepath.Base(root), Path: root}}
	if dir == root {
		crumbs[0].Current = true
		return crumbs
	}
	rel, _ := filepath.Rel(root, dir)
	cur := root
	parts := strings.Split(rel, string(filepath.Separator))
	for i, part := range parts {
		cur = filepath.Join(cur, part)
		crumbs = append(crumbs, Breadcrumb{
			Name:    part,
			Path:    cur,
			Current: i == len(parts)-1,
		})
	}
	return crumbs
}

func (a *App) handleServeFile(w http.ResponseWriter, r *http.Request) {
	rawPath := r.URL.Query().Get("path")
	if rawPath == "" {
		http.NotFound(w, r)
		return
	}
	absPath := filepath.Clean(rawPath)
	if !a.isAllowedPath(r, absPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	filename := filepath.Base(absPath)
	if r.URL.Query().Get("dl") == "1" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		http.ServeFile(w, r, absPath)
		return
	}
	fileType := classifyExt(filepath.Ext(filename))
	switch fileType {
	case "photo", "pdf", "video", "text":
		w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, filename))
	case "audio":
		w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, filename))
		w.Header().Set("Cache-Control", "private, max-age=3600")
	default:
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	}
	http.ServeFile(w, r, absPath)
}

func (a *App) handlePathsList(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT id, path FROM indexed_paths WHERE user_id = $1 ORDER BY path`, uid(r))
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	defer rows.Close()
	var paths []PathRow
	for rows.Next() {
		var p PathRow
		if err := rows.Scan(&p.ID, &p.Path); err != nil {
			continue
		}
		paths = append(paths, p)
	}
	render(w, "paths", PathsPage{ActiveTab: "paths", Paths: paths})
}

func (a *App) handlePathAdd(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	r.ParseForm()
	path := strings.TrimSpace(r.FormValue("path"))
	if path == "" {
		http.Redirect(w, r, "/paths", http.StatusFound)
		return
	}
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil {
		a.renderPathsWithError(w, r, fmt.Sprintf("%q: path not found", path))
		return
	}
	if !info.IsDir() {
		a.renderPathsWithError(w, r, fmt.Sprintf("%q is not a directory", path))
		return
	}
	_, err = a.db.Exec(r.Context(), `INSERT INTO indexed_paths (path, user_id) VALUES ($1, $2) ON CONFLICT (user_id, path) DO NOTHING`, path, uid(r))
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	userID := uid(r)
	go func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		a.reindexUser(ctx2, userID)
	}()
	http.Redirect(w, r, "/settings", http.StatusFound)
}

func (a *App) renderPathsWithError(w http.ResponseWriter, r *http.Request, errMsg string) {
	http.Redirect(w, r, "/settings?err="+url.QueryEscape(errMsg), http.StatusFound)
}

func (a *App) handlePathDelete(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	_, err := a.db.Exec(r.Context(), `DELETE FROM indexed_paths WHERE id=$1 AND user_id=$2`, id, uid(r))
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusFound)
}

func (a *App) handlePathToggle(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	enabled := r.FormValue("enabled") == "1"
	tag, err := a.db.Exec(r.Context(), `UPDATE indexed_paths SET enabled=$1 WHERE id=$2 AND user_id=$3`, enabled, id, uid(r))
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	if tag.RowsAffected() == 0 {
		http.NotFound(w, r)
		return
	}
	userID := uid(r)
	go func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		a.reindexUser(ctx2, userID)
	}()
	w.WriteHeader(http.StatusNoContent)
}

// ---- Search ----

type SearchResult struct {
	DirPath    string `json:"dir_path"`
	MatchCount int64  `json:"match_count"`
	SamplePath string `json:"sample_path"`
	SampleType string `json:"sample_type"`
}

func (a *App) handleSearchStatus(w http.ResponseWriter, r *http.Request) {
	userID := uid(r)
	_, running := a.reindexing.Load(userID)
	var count int64
	a.db.QueryRow(r.Context(), `SELECT COUNT(*) FROM file_index WHERE user_id = $1`, userID).Scan(&count)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"running": running, "count": count})
}

func (a *App) reindexUser(ctx context.Context, userID int64) {
	if _, running := a.reindexing.LoadOrStore(userID, true); running {
		return
	}
	defer a.reindexing.Delete(userID)
	rows, err := a.db.Query(ctx, `
		SELECT ip.path FROM indexed_paths ip
		WHERE ip.enabled = TRUE
		  AND (ip.user_id = $1 OR EXISTS (SELECT 1 FROM path_grants pg WHERE pg.path_id = ip.id AND pg.user_id = $1))
	`, userID)
	if err != nil {
		log.Printf("reindex: query paths: %v", err)
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

	type entry struct{ path, filename, fileType, dirPath string }
	var entries []entry
	for _, root := range roots {
		filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			ft := classifyExt(filepath.Ext(p))
			if ft == "other" || ft == "text" {
				return nil
			}
			entries = append(entries, entry{p, filepath.Base(p), ft, filepath.Dir(p)})
			return nil
		})
	}

	tx, err := a.db.Begin(ctx)
	if err != nil {
		log.Printf("reindex: begin tx: %v", err)
		return
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM file_index WHERE user_id = $1`, userID); err != nil {
		log.Printf("reindex: delete: %v", err)
		return
	}
	for _, e := range entries {
		if _, err := tx.Exec(ctx,
			`INSERT INTO file_index (user_id, path, filename, file_type, dir_path) VALUES ($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`,
			userID, e.path, e.filename, e.fileType, e.dirPath,
		); err != nil {
			log.Printf("reindex: insert %s: %v", e.path, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		log.Printf("reindex: commit: %v", err)
		return
	}

	// Purge stale video_positions for files that no longer exist.
	if len(entries) > 0 {
		paths := make([]string, len(entries))
		for i, e := range entries {
			paths[i] = e.path
		}
		if _, err := a.db.Exec(ctx,
			`DELETE FROM video_positions WHERE user_id = $1 AND NOT (path = ANY($2))`,
			userID, paths,
		); err != nil {
			log.Printf("reindex: purge positions: %v", err)
		}
	}

	log.Printf("reindex: user %d indexed %d files", userID, len(entries))
}

func (a *App) handleSearchReindex(w http.ResponseWriter, r *http.Request) {
	userID := uid(r)
	go func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		a.reindexUser(ctx2, userID)
	}()
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	typ := r.URL.Query().Get("type")
	if typ == "" {
		typ = "all"
	}
	w.Header().Set("Content-Type", "application/json")
	if len(q) < 2 {
		w.Write([]byte("[]"))
		return
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	rows, err := a.db.Query(r.Context(), `
		SELECT fi.dir_path,
		       COUNT(*) AS match_count,
		       MIN(fi.path) AS sample_path,
		       MIN(fi.file_type) AS sample_type
		FROM file_index fi
		WHERE fi.user_id = $1
		  AND lower(fi.filename) LIKE '%' || lower($2) || '%'
		  AND ($3 = 'all' OR fi.file_type = $3)
		GROUP BY fi.dir_path
		ORDER BY COUNT(*) DESC, fi.dir_path
		LIMIT 20 OFFSET $4
	`, uid(r), q, typ, offset)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	defer rows.Close()
	var results []SearchResult
	for rows.Next() {
		var sr SearchResult
		if rows.Scan(&sr.DirPath, &sr.MatchCount, &sr.SamplePath, &sr.SampleType) == nil {
			results = append(results, sr)
		}
	}
	if results == nil {
		results = []SearchResult{}
	}
	json.NewEncoder(w).Encode(results)
}

// ---- Auth ----

type ctxKey string

const ctxUserID ctxKey = "user_id"
const ctxIsAdmin ctxKey = "is_admin"

func isAdmin(r *http.Request) bool {
	v, _ := r.Context().Value(ctxIsAdmin).(bool)
	return v
}

func randToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (a *App) withAuth(next http.Handler) http.Handler {
	exempt := map[string]bool{"/login": true, "/logout": true, "/favicon.svg": true}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if exempt[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie("fb_session")
		if err != nil {
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
			return
		}
		var userID int64
		var admin bool
		err = a.db.QueryRow(r.Context(), `
			SELECT s.user_id, u.is_admin FROM sessions s
			JOIN users u ON u.id = s.user_id
			WHERE s.token = $1 AND s.expires_at > now()
		`, cookie.Value).Scan(&userID, &admin)
		if err != nil {
			http.SetCookie(w, &http.Cookie{Name: "fb_session", Value: "", Path: "/", MaxAge: -1})
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
			return
		}
		ctx := context.WithValue(r.Context(), ctxUserID, userID)
		ctx = context.WithValue(ctx, ctxIsAdmin, admin)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *App) bootstrapAdmin(ctx context.Context, username, password string) {
	if username == "" || password == "" {
		return
	}
	var count int
	if err := a.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil || count > 0 {
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("bootstrap admin: %v", err)
		return
	}
	if _, err = a.db.Exec(ctx, `INSERT INTO users (username, password_hash, is_admin) VALUES ($1, $2, TRUE)`, username, string(hash)); err != nil {
		log.Printf("bootstrap admin: %v", err)
	} else {
		log.Printf("created admin user %q", username)
	}
}

func (a *App) handleLoginGet(w http.ResponseWriter, r *http.Request) {
	render(w, "login", LoginPage{Next: r.URL.Query().Get("next")})
}

func (a *App) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	next := r.FormValue("next")

	var userID int64
	var hash string
	err := a.db.QueryRow(r.Context(),
		`SELECT id, password_hash FROM users WHERE username = $1`, username,
	).Scan(&userID, &hash)
	const errMsg = "Invalid username or password."
	if err != nil {
		render(w, "login", LoginPage{Error: errMsg, Next: next})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		render(w, "login", LoginPage{Error: errMsg, Next: next})
		return
	}
	token := randToken()
	if _, err = a.db.Exec(r.Context(), `
		INSERT INTO sessions (token, user_id, expires_at)
		VALUES ($1, $2, now() + interval '30 days')
	`, token, userID); err != nil {
		httpErr(w, err, 500)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "fb_session",
		Value:    token,
		Path:     "/",
		MaxAge:   30 * 24 * 3600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	if next == "" || !strings.HasPrefix(next, "/") {
		next = "/browse"
	}
	http.Redirect(w, r, next, http.StatusFound)
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("fb_session"); err == nil {
		a.db.Exec(r.Context(), `DELETE FROM sessions WHERE token = $1`, cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "fb_session", Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusFound)
}

// ---- User management ----

func (a *App) handleUserList(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	rows, err := a.db.Query(r.Context(), `SELECT id, username, created_at FROM users ORDER BY created_at`)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	defer rows.Close()
	var users []UserRow
	for rows.Next() {
		var u UserRow
		var t time.Time
		if rows.Scan(&u.ID, &u.Username, &t) == nil {
			u.CreatedAt = t.Format("2006-01-02 15:04")
			users = append(users, u)
		}
	}
	currentUID, _ := r.Context().Value(ctxUserID).(int64)
	render(w, "users", UsersPage{ActiveTab: "users", IsAdmin: isAdmin(r), Users: users, CurrentUID: currentUID})
}

func (a *App) handleUserDetail(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	targetID, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var username string
	if err := a.db.QueryRow(r.Context(), `SELECT username FROM users WHERE id = $1`, targetID).Scan(&username); err != nil {
		http.NotFound(w, r)
		return
	}
	// Load all admin paths with granted flag for this user.
	rows, err := a.db.Query(r.Context(), `
		SELECT ip.id, ip.path, (pg.path_id IS NOT NULL) AS granted
		FROM indexed_paths ip
		LEFT JOIN path_grants pg ON pg.path_id = ip.id AND pg.user_id = $2
		WHERE ip.user_id = $1
		ORDER BY ip.path`, uid(r), targetID)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	defer rows.Close()
	var allPaths []AdminPathRow
	for rows.Next() {
		var p AdminPathRow
		if rows.Scan(&p.ID, &p.Path, &p.Granted) == nil {
			allPaths = append(allPaths, p)
		}
	}
	render(w, "user_detail", UserDetailPage{
		ActiveTab: "users",
		IsAdmin:   true,
		ID:        targetID,
		Username:  username,
		AllPaths:  allPaths,
	})
}

func (a *App) handleUserCreate(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	r.ParseForm()
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	if username == "" || password == "" {
		a.renderUsersWithError(w, r, "Username and password are required.")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	_, err = a.db.Exec(r.Context(), `INSERT INTO users (username, password_hash) VALUES ($1, $2)`, username, string(hash))
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			a.renderUsersWithError(w, r, fmt.Sprintf("Username %q already exists.", username))
			return
		}
		httpErr(w, err, 500)
		return
	}
	http.Redirect(w, r, "/users", http.StatusFound)
}

func (a *App) handleUserDelete(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	currentUID, _ := r.Context().Value(ctxUserID).(int64)
	if id == currentUID {
		a.renderUsersWithError(w, r, "You cannot delete your own account.")
		return
	}
	if _, err := a.db.Exec(r.Context(), `DELETE FROM users WHERE id = $1`, id); err != nil {
		httpErr(w, err, 500)
		return
	}
	http.Redirect(w, r, "/users", http.StatusFound)
}

func (a *App) renderUsersWithError(w http.ResponseWriter, r *http.Request, errMsg string) {
	rows, _ := a.db.Query(r.Context(), `SELECT id, username, created_at FROM users ORDER BY created_at`)
	var users []UserRow
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var u UserRow
			var t time.Time
			if rows.Scan(&u.ID, &u.Username, &t) == nil {
				u.CreatedAt = t.Format("2006-01-02 15:04")
				users = append(users, u)
			}
		}
	}
	currentUID, _ := r.Context().Value(ctxUserID).(int64)
	render(w, "users", UsersPage{ActiveTab: "users", IsAdmin: isAdmin(r), Users: users, CurrentUID: currentUID, Error: errMsg})
}

func (a *App) ownsPath(r *http.Request, pathID int64) bool {
	var count int
	a.db.QueryRow(r.Context(), `SELECT COUNT(*) FROM indexed_paths WHERE id=$1 AND user_id=$2`, pathID, uid(r)).Scan(&count)
	return count > 0
}

func (a *App) handlePathGrant(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	pathID, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	r.ParseForm()
	grantUID, err := strconv.ParseInt(r.FormValue("user_id"), 10, 64)
	if err != nil || grantUID <= 0 {
		http.Redirect(w, r, "/users", http.StatusFound)
		return
	}
	if !a.ownsPath(r, pathID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	a.db.Exec(r.Context(), `INSERT INTO path_grants (path_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, pathID, grantUID)
	go func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		a.reindexUser(ctx2, grantUID)
	}()
	http.Redirect(w, r, fmt.Sprintf("/users/%d", grantUID), http.StatusFound)
}

func (a *App) handlePathRevoke(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	pathID, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	revokeUID, err := strconv.ParseInt(r.PathValue("uid"), 10, 64)
	if err != nil || revokeUID <= 0 {
		http.NotFound(w, r)
		return
	}
	if !a.ownsPath(r, pathID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	a.db.Exec(r.Context(), `DELETE FROM path_grants WHERE path_id=$1 AND user_id=$2`, pathID, revokeUID)
	go func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		a.reindexUser(ctx2, revokeUID)
	}()
	http.Redirect(w, r, fmt.Sprintf("/users/%d", revokeUID), http.StatusFound)
}

// ---- Playlist handlers ----

type playlistStateResp struct {
	CurrentIndex int     `json:"current_index"`
	PositionSec  float64 `json:"position_sec"`
}

type playlistStateReq struct {
	CurrentIndex int     `json:"current_index"`
	PositionSec  float64 `json:"position_sec"`
	DeltaSec     int     `json:"delta_sec"`
	MediaType    string  `json:"media_type"`
}

type playlistItemAddReq struct {
	Path string `json:"path"`
}

func (a *App) handlePlaylistList(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `
		SELECT p.id, p.name, COUNT(pi.id) AS item_count
		FROM playlists p
		LEFT JOIN playlist_items pi ON pi.playlist_id = p.id
		WHERE p.user_id = $1
		GROUP BY p.id, p.name
		ORDER BY p.name
	`, uid(r))
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	defer rows.Close()
	var pls []PlaylistRow
	for rows.Next() {
		var pl PlaylistRow
		if rows.Scan(&pl.ID, &pl.Name, &pl.ItemCount) == nil {
			pls = append(pls, pl)
		}
	}
	render(w, "playlists", PlaylistsPage{ActiveTab: "playlists", IsAdmin: isAdmin(r), Playlists: pls})
}

func (a *App) handlePlaylistCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Redirect(w, r, "/playlists", http.StatusFound)
		return
	}
	var id int64
	err := a.db.QueryRow(r.Context(), `INSERT INTO playlists (name, user_id) VALUES ($1, $2) RETURNING id`, name, uid(r)).Scan(&id)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/playlists/%d", id), http.StatusFound)
}

func (a *App) handlePlaylistDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var name string
	if err := a.db.QueryRow(r.Context(), `SELECT name FROM playlists WHERE id = $1 AND user_id = $2`, id, uid(r)).Scan(&name); err != nil {
		http.NotFound(w, r)
		return
	}
	itemRows, err := a.db.Query(r.Context(), `SELECT id, path FROM playlist_items WHERE playlist_id = $1 ORDER BY position, id`, id)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	defer itemRows.Close()
	var items []PlaylistItem
	for itemRows.Next() {
		var it PlaylistItem
		if itemRows.Scan(&it.ID, &it.Path) == nil {
			it.Name = filepath.Base(it.Path)
			it.FileType = classifyExt(filepath.Ext(it.Path))
			items = append(items, it)
		}
	}
	var state PlaylistState
	a.db.QueryRow(r.Context(), `SELECT current_index, position_sec FROM playlist_state WHERE playlist_id = $1`, id).Scan(&state.CurrentIndex, &state.PositionSec)
	render(w, "playlist_detail", PlaylistDetailPage{ActiveTab: "playlists", IsAdmin: isAdmin(r), ID: id, Name: name, Items: items, State: state})
}

func (a *App) handlePlaylistDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if _, err := a.db.Exec(r.Context(), `DELETE FROM playlists WHERE id = $1 AND user_id = $2`, id, uid(r)); err != nil {
		httpErr(w, err, 500)
		return
	}
	http.Redirect(w, r, "/playlists", http.StatusFound)
}

func (a *App) handlePlaylistItemAdd(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var req playlistItemAddReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	absPath := filepath.Clean(req.Path)
	if !a.isAllowedPath(r, absPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	// Ownership folded into INSERT; position set to max+1 so item appends at end.
	if _, err := a.db.Exec(r.Context(), `
		INSERT INTO playlist_items (playlist_id, path, position)
		SELECT $1, $2, COALESCE((SELECT MAX(position)+1 FROM playlist_items WHERE playlist_id=$1), 0)
		FROM playlists WHERE id = $1 AND user_id = $3
		ON CONFLICT DO NOTHING
	`, id, absPath, uid(r)); err != nil {
		httpErr(w, err, 500)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handlePlaylistItemDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	itemID, err := strconv.ParseInt(r.PathValue("item_id"), 10, 64)
	if err != nil || itemID <= 0 {
		http.NotFound(w, r)
		return
	}
	if _, err := a.db.Exec(r.Context(), `
		DELETE FROM playlist_items
		WHERE id = $1 AND playlist_id = $2
		  AND EXISTS (SELECT 1 FROM playlists WHERE id = $2 AND user_id = $3)
	`, itemID, id, uid(r)); err != nil {
		httpErr(w, err, 500)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type playlistReorderReq struct {
	Order []int64 `json:"order"`
}

func (a *App) handlePlaylistReorder(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var ownerID int64
	if err := a.db.QueryRow(r.Context(), `SELECT user_id FROM playlists WHERE id = $1`, id).Scan(&ownerID); err != nil || ownerID != uid(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	var req playlistReorderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	defer tx.Rollback(r.Context())
	for i, itemID := range req.Order {
		if _, err := tx.Exec(r.Context(), `UPDATE playlist_items SET position = $1 WHERE id = $2 AND playlist_id = $3`, i, itemID, id); err != nil {
			httpErr(w, err, 500)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpErr(w, err, 500)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleGetPlaylistState(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var resp playlistStateResp
	a.db.QueryRow(r.Context(), `
		SELECT ps.current_index, ps.position_sec
		FROM playlist_state ps
		JOIN playlists p ON p.id = ps.playlist_id
		WHERE ps.playlist_id = $1 AND p.user_id = $2
	`, id, uid(r)).Scan(&resp.CurrentIndex, &resp.PositionSec)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (a *App) handleSavePlaylistState(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var req playlistStateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// Ownership check folded into the INSERT: the SELECT subquery ensures the playlist
	// belongs to this user before upserting state.
	_, err := a.db.Exec(r.Context(), `
		INSERT INTO playlist_state (playlist_id, current_index, position_sec, updated_at)
		SELECT $1, $2, $3, now() FROM playlists WHERE id = $1 AND user_id = $4
		ON CONFLICT (playlist_id) DO UPDATE
		  SET current_index = EXCLUDED.current_index,
		      position_sec  = EXCLUDED.position_sec,
		      updated_at    = now()
	`, id, req.CurrentIndex, req.PositionSec, uid(r))
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	a.accumulatePlayTime(r.Context(), uid(r), req.DeltaSec, req.MediaType)
	w.WriteHeader(http.StatusNoContent)
}

const thumbCacheDir = "/var/lib/filebrowser/thumbs"

func (a *App) handleThumbnail(w http.ResponseWriter, r *http.Request) {
	rawPath := r.URL.Query().Get("path")
	if rawPath == "" {
		http.NotFound(w, r)
		return
	}
	absPath := filepath.Clean(rawPath)
	if !a.isAllowedPath(r, absPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	h := sha256.Sum256([]byte(absPath))
	cacheFile := filepath.Join(thumbCacheDir, hex.EncodeToString(h[:])+".jpg")

	if _, err := os.Stat(cacheFile); err == nil {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "max-age=86400")
		http.ServeFile(w, r, cacheFile)
		return
	}

	if a.ffmpegPath == "" {
		http.Error(w, "transcoding not configured", http.StatusServiceUnavailable)
		return
	}

	fileType := classifyExt(filepath.Ext(absPath))
	var args []string
	switch fileType {
	case "video":
		args = []string{"-y", "-ss", "10", "-i", absPath, "-vframes", "1", "-vf", "scale=320:-2", "-q:v", "5", "-f", "image2", cacheFile}
	case "photo":
		args = []string{"-y", "-i", absPath, "-vf", "scale=320:-2", "-q:v", "5", "-f", "image2", cacheFile}
	default:
		http.NotFound(w, r)
		return
	}

	if err := os.MkdirAll(thumbCacheDir, 0755); err != nil {
		httpErr(w, err, 500)
		return
	}

	cmd := exec.CommandContext(r.Context(), a.ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		log.Printf("thumbnail %s: %v %s", absPath, err, stderr.String())
		http.Error(w, "thumbnail generation failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "max-age=86400")
	http.ServeFile(w, r, cacheFile)
}

// ---- Transcoding settings ----

type TranscodeSettings struct {
	CRF                int
	Preset             string
	MaxWidth           int
	VideoKbps          int
	AudioKbps          int
	SegmentSec         int
	AudioHLS           bool
	AudioHLSThreshold  int // kbps — files above this bitrate get HLS transcoding
	ForceOriginal      bool
	DefaultVolume      float64
}

var validPresets = map[string]bool{
	"ultrafast": true, "superfast": true, "veryfast": true, "faster": true,
	"fast": true, "medium": true, "slow": true, "slower": true, "veryslow": true,
}

func transcodeParamsFromRequest(r *http.Request) TranscodeSettings {
	q := r.URL.Query()
	s := TranscodeSettings{CRF: 23, Preset: "fast", MaxWidth: 1280, VideoKbps: 3000, AudioKbps: 128, SegmentSec: 6, AudioHLS: true, AudioHLSThreshold: 320}
	if n, err := strconv.Atoi(q.Get("crf")); err == nil && n >= 0 && n <= 51 {
		s.CRF = n
	}
	if v := q.Get("preset"); validPresets[v] {
		s.Preset = v
	}
	if n, err := strconv.Atoi(q.Get("max_width")); err == nil && n >= 0 {
		s.MaxWidth = n
	}
	if n, err := strconv.Atoi(q.Get("video_kbps")); err == nil && n > 0 {
		s.VideoKbps = n
	}
	if n, err := strconv.Atoi(q.Get("audio_kbps")); err == nil && n > 0 {
		s.AudioKbps = n
	}
	if n, err := strconv.Atoi(q.Get("segment_sec")); err == nil && n >= 2 && n <= 60 {
		s.SegmentSec = n
	}
	if v := q.Get("audio_hls"); v != "" {
		s.AudioHLS = v == "1"
	}
	if n, err := strconv.Atoi(q.Get("audio_hls_threshold")); err == nil && n > 0 {
		s.AudioHLSThreshold = n
	}
	return s
}

func tsQueryParams(ts TranscodeSettings) string {
	audioHLS := "0"
	if ts.AudioHLS {
		audioHLS = "1"
	}
	return fmt.Sprintf("&crf=%d&preset=%s&max_width=%d&video_kbps=%d&audio_kbps=%d&segment_sec=%d&audio_hls=%s&audio_hls_threshold=%d",
		ts.CRF, ts.Preset, ts.MaxWidth, ts.VideoKbps, ts.AudioKbps, ts.SegmentSec, audioHLS, ts.AudioHLSThreshold)
}

// ---- Settings page handlers ----

func (a *App) handleSettingsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := uid(r)
	admin := isAdmin(r)

	var pathQuery string
	if admin {
		pathQuery = `SELECT id, path, enabled FROM indexed_paths WHERE user_id = $1 ORDER BY path`
	} else {
		pathQuery = `SELECT ip.id, ip.path, ip.enabled FROM indexed_paths ip JOIN path_grants pg ON pg.path_id = ip.id WHERE pg.user_id = $1 AND ip.enabled = TRUE ORDER BY ip.path`
	}
	rows, err := a.db.Query(ctx, pathQuery, userID)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	var paths []PathRow
	for rows.Next() {
		var p PathRow
		if rows.Scan(&p.ID, &p.Path, &p.Enabled) == nil {
			paths = append(paths, p)
		}
	}
	rows.Close()

	pathErr := r.URL.Query().Get("err")
	render(w, "settings", SettingsPage{
		ActiveTab: "settings",
		IsAdmin:   admin,
		Paths:     paths,
		PathError: pathErr,
	})
}


func (a *App) handlePathSize(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	out, err := exec.CommandContext(r.Context(), "du", "-sb", path).Output()
	var gb float64
	if err == nil {
		var bytes int64
		fmt.Sscanf(string(out), "%d", &bytes)
		gb = float64(bytes) / 1e9
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"gb":%.2f}`, gb)
}

func (a *App) handleHLSPlaylist(w http.ResponseWriter, r *http.Request) {
	if a.ffmpegPath == "" {
		http.Error(w, "transcoding not configured", http.StatusServiceUnavailable)
		return
	}
	rawPath := r.URL.Query().Get("path")
	if rawPath == "" {
		http.NotFound(w, r)
		return
	}
	absPath := filepath.Clean(rawPath)
	if !a.isAllowedPath(r, absPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	ffprobePath := filepath.Join(filepath.Dir(a.ffmpegPath), "ffprobe")
	ts := transcodeParamsFromRequest(r)
	if classifyExt(filepath.Ext(absPath)) == "audio" {
		// Redirect to direct file if audio HLS is disabled or bitrate is below threshold.
		if !ts.AudioHLS {
			http.Redirect(w, r, "/file?path="+url.QueryEscape(absPath), http.StatusFound)
			return
		}
		brOut, _ := exec.CommandContext(r.Context(), ffprobePath,
			"-v", "quiet", "-show_entries", "stream=bit_rate",
			"-select_streams", "a:0", "-of", "csv=p=0", absPath,
		).Output()
		bitrate, _ := strconv.ParseInt(strings.TrimSpace(string(brOut)), 10, 64)
		if bitrate > 0 && bitrate < int64(ts.AudioHLSThreshold)*1000 {
			http.Redirect(w, r, "/file?path="+url.QueryEscape(absPath), http.StatusFound)
			return
		}
	}
	out, err := exec.CommandContext(r.Context(), ffprobePath,
		"-v", "quiet", "-show_entries", "format=duration", "-of", "csv=p=0", absPath,
	).Output()
	if err != nil {
		http.Error(w, "could not probe media", http.StatusInternalServerError)
		return
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || duration <= 0 {
		http.Error(w, "invalid duration", http.StatusInternalServerError)
		return
	}
	segSec := ts.SegmentSec
	encodedPath := url.QueryEscape(absPath)
	tsP := tsQueryParams(ts)
	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n")
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n", segSec)
	b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n")
	fullSegments := int(duration) / segSec
	lastDur := duration - float64(fullSegments*segSec)
	for i := range fullSegments {
		fmt.Fprintf(&b, "#EXTINF:%d.000,\n/hls/segment?path=%s&n=%d%s\n", segSec, encodedPath, i, tsP)
	}
	if lastDur > 0.05 {
		fmt.Fprintf(&b, "#EXTINF:%.3f,\n/hls/segment?path=%s&n=%d%s\n", lastDur, encodedPath, fullSegments, tsP)
	}
	b.WriteString("#EXT-X-ENDLIST\n")
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Write([]byte(b.String()))
}

type videoPositionResp struct {
	Position   float64 `json:"position"`
	WatchCount int64   `json:"watch_count"`
}

type videoPositionReq struct {
	Path      string  `json:"path"`
	Position  float64 `json:"position"`
	Completed bool    `json:"completed"`
	DeltaSec  int     `json:"delta_sec"`
}

func (a *App) handleGetVideoPosition(w http.ResponseWriter, r *http.Request) {
	rawPath := r.URL.Query().Get("path")
	if rawPath == "" {
		http.NotFound(w, r)
		return
	}
	absPath := filepath.Clean(rawPath)
	if !a.isAllowedPath(r, absPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if classifyExt(filepath.Ext(absPath)) == "audio" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(videoPositionResp{})
		return
	}
	var resp videoPositionResp
	err := a.db.QueryRow(r.Context(),
		`SELECT position_sec, watch_count FROM video_positions WHERE user_id = $1 AND path = $2`, uid(r), absPath,
	).Scan(&resp.Position, &resp.WatchCount)
	if err != nil {
		resp = videoPositionResp{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (a *App) accumulatePlayTime(ctx context.Context, userID int64, deltaSec int, mediaType string) {
	if deltaSec <= 0 || deltaSec > 30 {
		return
	}
	if mediaType == "" {
		mediaType = "video"
	}
	a.db.Exec(ctx, `
		INSERT INTO play_time (user_id, day, media_type, seconds) VALUES ($1, CURRENT_DATE, $2, $3)
		ON CONFLICT (user_id, day, media_type) DO UPDATE SET seconds = play_time.seconds + EXCLUDED.seconds
	`, userID, mediaType, deltaSec)
}

func (a *App) handlePlayStats(w http.ResponseWriter, r *http.Request) {
	var todaySec, totalSec, audioTodaySec, audioTotalSec int64
	a.db.QueryRow(r.Context(), `
		SELECT COALESCE(SUM(CASE WHEN day = CURRENT_DATE AND media_type = 'video' THEN seconds END), 0),
		       COALESCE(SUM(CASE WHEN media_type = 'video' THEN seconds END), 0),
		       COALESCE(SUM(CASE WHEN day = CURRENT_DATE AND media_type = 'audio' THEN seconds END), 0),
		       COALESCE(SUM(CASE WHEN media_type = 'audio' THEN seconds END), 0)
		FROM play_time WHERE user_id = $1`, uid(r)).Scan(&todaySec, &totalSec, &audioTodaySec, &audioTotalSec)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"today_sec":%d,"total_sec":%d,"audio_today_sec":%d,"audio_total_sec":%d}`,
		todaySec, totalSec, audioTodaySec, audioTotalSec)
}

func (a *App) handleSaveVideoPosition(w http.ResponseWriter, r *http.Request) {
	var req videoPositionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	absPath := filepath.Clean(req.Path)
	if !a.isAllowedPath(r, absPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if classifyExt(filepath.Ext(absPath)) == "audio" {
		a.accumulatePlayTime(r.Context(), uid(r), req.DeltaSec, "audio")
		if req.Completed {
			a.db.Exec(r.Context(), `
				INSERT INTO video_positions (user_id, path, position_sec, watch_count, updated_at)
				VALUES ($1, $2, 0, 1, now())
				ON CONFLICT (user_id, path) DO UPDATE
				  SET watch_count = video_positions.watch_count + 1,
				      updated_at  = now()
			`, uid(r), absPath)
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var err error
	if req.Completed {
		_, err = a.db.Exec(r.Context(), `
			INSERT INTO video_positions (user_id, path, position_sec, watch_count, updated_at)
			VALUES ($1, $2, 0, 1, now())
			ON CONFLICT (user_id, path) DO UPDATE
			  SET position_sec = 0,
			      watch_count  = video_positions.watch_count + 1,
			      updated_at   = now()
		`, uid(r), absPath)
	} else {
		_, err = a.db.Exec(r.Context(), `
			INSERT INTO video_positions (user_id, path, position_sec, updated_at)
			VALUES ($1, $2, $3, now())
			ON CONFLICT (user_id, path) DO UPDATE
			  SET position_sec = EXCLUDED.position_sec,
			      updated_at   = now()
		`, uid(r), absPath, req.Position)
	}
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	a.accumulatePlayTime(r.Context(), uid(r), req.DeltaSec, "video")
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleHLSSegment(w http.ResponseWriter, r *http.Request) {
	if a.ffmpegPath == "" {
		http.Error(w, "transcoding not configured", http.StatusServiceUnavailable)
		return
	}
	rawPath := r.URL.Query().Get("path")
	nStr := r.URL.Query().Get("n")
	if rawPath == "" || nStr == "" {
		http.NotFound(w, r)
		return
	}
	absPath := filepath.Clean(rawPath)
	if !a.isAllowedPath(r, absPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	n, err := strconv.Atoi(nStr)
	if err != nil || n < 0 {
		http.NotFound(w, r)
		return
	}
	ts := transcodeParamsFromRequest(r)
	startSec := n * ts.SegmentSec
	abr := fmt.Sprintf("%dk", ts.AudioKbps)
	var args []string
	if classifyExt(filepath.Ext(absPath)) == "audio" {
		args = []string{
			"-y",
			"-ss", strconv.Itoa(startSec),
			"-i", absPath,
			"-t", strconv.Itoa(ts.SegmentSec),
			"-vn",
			"-c:a", "aac", "-b:a", abr,
			"-output_ts_offset", strconv.Itoa(startSec),
			"-muxdelay", "0", "-muxpreload", "0",
			"-f", "mpegts", "pipe:1",
		}
	} else {
		vbr := fmt.Sprintf("%dk", ts.VideoKbps)
		args = []string{
			"-y",
			"-ss", strconv.Itoa(startSec),
			"-i", absPath,
			"-t", strconv.Itoa(ts.SegmentSec),
			"-c:v", "libx264",
			"-crf", strconv.Itoa(ts.CRF),
			"-preset", ts.Preset,
			"-profile:v", "main", "-level", "4.0",
			"-pix_fmt", "yuv420p",
			"-bf", "0",
		}
		if ts.MaxWidth > 0 {
			args = append(args, "-vf", fmt.Sprintf("scale='min(%d,iw)':-2", ts.MaxWidth))
		}
		args = append(args,
			"-b:v", vbr, "-maxrate", vbr, "-bufsize", fmt.Sprintf("%dk", ts.VideoKbps*2),
			"-c:a", "aac", "-b:a", abr,
			"-output_ts_offset", strconv.Itoa(startSec),
			"-muxdelay", "0", "-muxpreload", "0",
			"-f", "mpegts", "pipe:1",
		)
	}
	cmd := exec.CommandContext(r.Context(), a.ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stdout = w
	cmd.Stderr = &stderr
	w.Header().Set("Content-Type", "video/mp2t")
	if err := cmd.Run(); err != nil {
		log.Printf("transcode segment %s/%d: %v\n%s", absPath, n, err, stderr.String())
	}
}

func (a *App) handleTranscodeStream(w http.ResponseWriter, r *http.Request) {
	if a.ffmpegPath == "" {
		http.Error(w, "transcoding not configured", http.StatusServiceUnavailable)
		return
	}
	rawPath := r.URL.Query().Get("path")
	if rawPath == "" {
		http.NotFound(w, r)
		return
	}
	absPath := filepath.Clean(rawPath)
	if !a.isAllowedPath(r, absPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	audioKbps := 128
	if n, err := strconv.Atoi(r.URL.Query().Get("audio_kbps")); err == nil && n > 0 && n <= 1024 {
		audioKbps = n
	}
	args := []string{
		"-y", "-i", absPath,
		"-vn",
		"-c:a", "aac", "-b:a", fmt.Sprintf("%dk", audioKbps),
		"-f", "adts", "pipe:1",
	}
	cmd := exec.CommandContext(r.Context(), a.ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stdout = w
	cmd.Stderr = &stderr
	w.Header().Set("Content-Type", "audio/aac")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	if err := cmd.Run(); err != nil {
		log.Printf("transcode stream %s: %v\n%s", absPath, err, stderr.String())
	}
}

func (a *App) handleTopPlayed(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `
		SELECT fi.path, fi.filename, fi.file_type, vp.watch_count
		FROM file_index fi
		JOIN video_positions vp ON vp.user_id = fi.user_id AND vp.path = fi.path
		WHERE fi.user_id = $1 AND fi.file_type = 'audio' AND vp.watch_count > 0
		ORDER BY vp.watch_count DESC
		LIMIT 50
	`, uid(r))
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	defer rows.Close()
	var items []PlaylistItem
	for rows.Next() {
		var path, filename, fileType string
		var count int64
		if rows.Scan(&path, &filename, &fileType, &count) != nil {
			continue
		}
		items = append(items, PlaylistItem{Path: path, Name: filename, FileType: fileType, WatchCount: count})
	}
	if err := rows.Err(); err != nil {
		httpErr(w, err, 500)
		return
	}
	render(w, "top-played", TopPlayedPage{ActiveTab: "top-played", IsAdmin: isAdmin(r), Items: items})
}

func (a *App) handleFavoritesPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := uid(r)

	// Fetch indexed roots for display trimming.
	// Strategy: find the longest matching indexed root, then strip its parent.
	// E.g. indexed root /blue2t/music/Classical → strip /blue2t/music/ → show Classical/Bach/...
	var indexedRoots []string
	if ipRows, err := a.db.Query(ctx, `SELECT path FROM indexed_paths WHERE user_id = $1 AND enabled = TRUE`, userID); err == nil {
		for ipRows.Next() {
			var p string
			if ipRows.Scan(&p) == nil {
				indexedRoots = append(indexedRoots, strings.TrimRight(p, "/"))
			}
		}
		ipRows.Close()
	}
	trimPath := func(p string) string {
		best := ""
		for _, root := range indexedRoots {
			if (strings.HasPrefix(p, root+"/") || p == root) && len(root) > len(best) {
				best = root
			}
		}
		if best == "" {
			return p
		}
		parent := filepath.Dir(best)
		if parent == "." || parent == "/" {
			return p
		}
		rel := strings.TrimRight(strings.TrimPrefix(p, parent+"/"), "/")
		if rel == "" {
			return p
		}
		return rel
	}

	favRows, err := a.db.Query(ctx, `
		SELECT path, is_folder FROM favorites WHERE user_id = $1 ORDER BY position, created_at
	`, userID)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	type rawFav struct {
		Path     string
		IsFolder bool
	}
	var rawFavs []rawFav
	for favRows.Next() {
		var f rawFav
		if favRows.Scan(&f.Path, &f.IsFolder) == nil {
			rawFavs = append(rawFavs, f)
		}
	}
	favRows.Close()

	var favItems []FavoriteItem
	var tracks []PlaylistItem

	for _, f := range rawFavs {
		startIdx := len(tracks)
		if f.IsFolder {
			fileRows, err := a.db.Query(ctx, `
				SELECT path, filename, file_type FROM file_index
				WHERE user_id = $1 AND dir_path = $2 AND file_type = 'audio'
				ORDER BY lower(filename)
			`, userID, f.Path)
			if err == nil {
				for fileRows.Next() {
					var pi PlaylistItem
					if fileRows.Scan(&pi.Path, &pi.Name, &pi.FileType) == nil {
						tracks = append(tracks, pi)
					}
				}
				fileRows.Close()
			}
			endIdx := len(tracks)
			if endIdx == startIdx {
				continue
			}
			favItems = append(favItems, FavoriteItem{
				Path:       f.Path,
				Name:       filepath.Base(f.Path),
				Dir:        trimPath(filepath.Dir(f.Path)),
				IsFolder:   true,
				StartIdx:   startIdx,
				EndIdx:     endIdx,
				TrackCount: endIdx - startIdx,
			})
		} else {
			var pi PlaylistItem
			err := a.db.QueryRow(ctx, `
				SELECT path, filename, file_type FROM file_index
				WHERE user_id = $1 AND path = $2
			`, userID, f.Path).Scan(&pi.Path, &pi.Name, &pi.FileType)
			if err != nil {
				continue
			}
			tracks = append(tracks, pi)
			favItems = append(favItems, FavoriteItem{
				Path:       f.Path,
				Name:       pi.Name,
				Dir:        trimPath(filepath.Dir(f.Path)),
				IsFolder:   false,
				StartIdx:   startIdx,
				EndIdx:     startIdx + 1,
				TrackCount: 1,
			})
		}
	}

	render(w, "favorites", FavoritesPage{ActiveTab: "favorites", IsAdmin: isAdmin(r), Items: favItems, Tracks: tracks})
}

func (a *App) handleFavoriteList(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT path FROM favorites WHERE user_id = $1`, uid(r))
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var p string
		if rows.Scan(&p) == nil {
			paths = append(paths, p)
		}
	}
	if err := rows.Err(); err != nil {
		httpErr(w, err, 500)
		return
	}
	if paths == nil {
		paths = []string{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(paths)
}

func (a *App) handleFavoriteToggle(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path     string `json:"path"`
		IsFolder bool   `json:"is_folder"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Path == "" {
		http.Error(w, "bad request", 400)
		return
	}
	absPath := filepath.Clean(body.Path)
	if !a.isAllowedPath(r, absPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	tag, err := a.db.Exec(r.Context(),
		`INSERT INTO favorites (user_id, path, is_folder, position)
		 VALUES ($1, $2, $3, (SELECT COALESCE(MAX(position)+1, 0) FROM favorites WHERE user_id = $1))
		 ON CONFLICT DO NOTHING`,
		uid(r), absPath, body.IsFolder,
	)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	favorited := tag.RowsAffected() > 0
	if !favorited {
		if _, err := a.db.Exec(r.Context(),
			`DELETE FROM favorites WHERE user_id = $1 AND path = $2`,
			uid(r), absPath,
		); err != nil {
			httpErr(w, err, 500)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"favorited": favorited})
}

func (a *App) handleFavoriteReorder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	defer tx.Rollback(r.Context())
	for i, path := range req.Paths {
		if _, err := tx.Exec(r.Context(),
			`UPDATE favorites SET position = $1 WHERE user_id = $2 AND path = $3`,
			i, uid(r), path,
		); err != nil {
			httpErr(w, err, 500)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpErr(w, err, 500)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
