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

// isAllowedPath checks that absPath is equal to or under one of the current user's enabled indexed roots.
func (a *App) isAllowedPath(r *http.Request, absPath string) bool {
	var count int
	err := a.db.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM indexed_paths
		WHERE user_id = $1 AND enabled = TRUE AND ($2 = path OR starts_with($2, path || '/'))
	`, uid(r), absPath).Scan(&count)
	return err == nil && count > 0
}

func (a *App) handleBrowse(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := uid(r)

	// Always load sidebar paths.
	pRows, err := a.db.Query(ctx, `SELECT id, path, enabled FROM indexed_paths WHERE user_id = $1 AND enabled = TRUE ORDER BY path`, userID)
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
			http.Redirect(w, r, "/browse?dir="+url.QueryEscape(sidebarPaths[0].Path), http.StatusFound)
			return
		}
		render(w, "browse", BrowsePage{ActiveTab: "browse", Paths: sidebarPaths})
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

	var subdirs []SubdirRow
	var files []FileRow
	for _, e := range entries {
		if e.IsDir() {
			absDir := filepath.Join(dirParam, e.Name())
			subdirs = append(subdirs, SubdirRow{
				AbsPath:  absDir,
				Name:     e.Name(),
				AlbumArt: findAlbumArt(absDir),
			})
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
		})
	}
	sort.Slice(subdirs, func(i, j int) bool { return subdirs[i].Name < subdirs[j].Name })
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Filename) < strings.ToLower(files[j].Filename)
	})

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
	var items []RecentItem
	for rows.Next() {
		var path string
		var wc int64
		var t time.Time
		var pos float64
		if rows.Scan(&path, &wc, &t, &pos) != nil {
			continue
		}
		ext := filepath.Ext(path)
		ft := classifyExt(ext)
		dir := filepath.Dir(path)
		var art string
		if ft == "audio" {
			art = findAlbumArt(dir)
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
	render(w, "recent", RecentPage{ActiveTab: "recent", Items: items})
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
	http.Redirect(w, r, "/settings", http.StatusFound)
}

func (a *App) renderPathsWithError(w http.ResponseWriter, r *http.Request, errMsg string) {
	http.Redirect(w, r, "/settings?err="+url.QueryEscape(errMsg), http.StatusFound)
}

func (a *App) handlePathDelete(w http.ResponseWriter, r *http.Request) {
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
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	r.ParseForm()
	enabled := r.FormValue("enabled") == "1"
	_, err := a.db.Exec(r.Context(), `UPDATE indexed_paths SET enabled=$1 WHERE id=$2 AND user_id=$3`, enabled, id, uid(r))
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Auth ----

type ctxKey string

const ctxUserID ctxKey = "user_id"

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
		err = a.db.QueryRow(r.Context(), `
			SELECT user_id FROM sessions WHERE token = $1 AND expires_at > now()
		`, cookie.Value).Scan(&userID)
		if err != nil {
			http.SetCookie(w, &http.Cookie{Name: "fb_session", Value: "", Path: "/", MaxAge: -1})
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
			return
		}
		ctx := context.WithValue(r.Context(), ctxUserID, userID)
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
	if _, err = a.db.Exec(ctx, `INSERT INTO users (username, password_hash) VALUES ($1, $2)`, username, string(hash)); err != nil {
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
	render(w, "users", UsersPage{ActiveTab: "users", Users: users, CurrentUID: currentUID})
}

func (a *App) handleUserCreate(w http.ResponseWriter, r *http.Request) {
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
	render(w, "users", UsersPage{ActiveTab: "users", Users: users, CurrentUID: currentUID, Error: errMsg})
}

// ---- Playlist handlers ----

type playlistStateResp struct {
	CurrentIndex int     `json:"current_index"`
	PositionSec  float64 `json:"position_sec"`
}

type playlistStateReq struct {
	CurrentIndex int     `json:"current_index"`
	PositionSec  float64 `json:"position_sec"`
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
	render(w, "playlists", PlaylistsPage{ActiveTab: "playlists", Playlists: pls})
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
	itemRows, err := a.db.Query(r.Context(), `SELECT id, path FROM playlist_items WHERE playlist_id = $1 ORDER BY id`, id)
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
	// Order items by filename (case-insensitive), so playback follows name order.
	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	var state PlaylistState
	a.db.QueryRow(r.Context(), `SELECT current_index, position_sec FROM playlist_state WHERE playlist_id = $1`, id).Scan(&state.CurrentIndex, &state.PositionSec)
	render(w, "playlist_detail", PlaylistDetailPage{ActiveTab: "playlists", ID: id, Name: name, Items: items, State: state})
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
	var ownerID int64
	if err := a.db.QueryRow(r.Context(), `SELECT user_id FROM playlists WHERE id = $1`, id).Scan(&ownerID); err != nil || ownerID != uid(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
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
	if _, err := a.db.Exec(r.Context(), `INSERT INTO playlist_items (playlist_id, path) VALUES ($1, $2) ON CONFLICT DO NOTHING`, id, absPath); err != nil {
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
	var ownerID int64
	if err := a.db.QueryRow(r.Context(), `SELECT user_id FROM playlists WHERE id = $1`, id).Scan(&ownerID); err != nil || ownerID != uid(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	itemID, err := strconv.ParseInt(r.PathValue("item_id"), 10, 64)
	if err != nil || itemID <= 0 {
		http.NotFound(w, r)
		return
	}
	if _, err := a.db.Exec(r.Context(), `DELETE FROM playlist_items WHERE id = $1 AND playlist_id = $2`, itemID, id); err != nil {
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
	var ownerID int64
	if err := a.db.QueryRow(r.Context(), `SELECT user_id FROM playlists WHERE id = $1`, id).Scan(&ownerID); err != nil || ownerID != uid(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	var resp playlistStateResp
	a.db.QueryRow(r.Context(), `SELECT current_index, position_sec FROM playlist_state WHERE playlist_id = $1`, id).Scan(&resp.CurrentIndex, &resp.PositionSec)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (a *App) handleSavePlaylistState(w http.ResponseWriter, r *http.Request) {
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
	var req playlistStateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	_, err := a.db.Exec(r.Context(), `
		INSERT INTO playlist_state (playlist_id, current_index, position_sec, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (playlist_id) DO UPDATE
		  SET current_index = EXCLUDED.current_index,
		      position_sec  = EXCLUDED.position_sec,
		      updated_at    = now()
	`, id, req.CurrentIndex, req.PositionSec)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
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
	CRF           int
	Preset        string
	MaxWidth      int
	VideoKbps     int
	AudioKbps     int
	SegmentSec    int
	ForceOriginal bool
	DefaultVolume float64
}

var validPresets = map[string]bool{
	"ultrafast": true, "superfast": true, "veryfast": true, "faster": true,
	"fast": true, "medium": true, "slow": true, "slower": true, "veryslow": true,
}

func (a *App) getTranscodeSettings(ctx context.Context, userID int64) TranscodeSettings {
	s := TranscodeSettings{CRF: 23, Preset: "fast", MaxWidth: 1280, VideoKbps: 3000, AudioKbps: 128, SegmentSec: 6, DefaultVolume: 1.0}
	rows, err := a.db.Query(ctx, `SELECT key, value FROM settings WHERE user_id = $1`, userID)
	if err != nil {
		return s
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if rows.Scan(&k, &v) != nil {
			continue
		}
		switch k {
		case "transcode_crf":
			if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 51 {
				s.CRF = n
			}
		case "transcode_preset":
			if validPresets[v] {
				s.Preset = v
			}
		case "transcode_max_width":
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				s.MaxWidth = n
			}
		case "transcode_video_kbps":
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				s.VideoKbps = n
			}
		case "transcode_audio_kbps":
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				s.AudioKbps = n
			}
		case "transcode_segment_sec":
			if n, err := strconv.Atoi(v); err == nil && n >= 2 && n <= 60 {
				s.SegmentSec = n
			}
		case "playback_force_original":
			s.ForceOriginal = v == "1"
		case "playback_default_volume":
			if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f <= 1 {
				s.DefaultVolume = f
			}
		}
	}
	return s
}

// ---- Settings page handlers ----

func (a *App) handleSettingsPage(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT id, path, enabled FROM indexed_paths WHERE user_id = $1 ORDER BY path`, uid(r))
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	defer rows.Close()
	var paths []PathRow
	for rows.Next() {
		var p PathRow
		if rows.Scan(&p.ID, &p.Path, &p.Enabled) == nil {
			paths = append(paths, p)
		}
	}
	s := a.getTranscodeSettings(r.Context(), uid(r))
	savedOK := r.URL.Query().Get("saved") == "1"
	pathErr := r.URL.Query().Get("err")
	render(w, "settings", SettingsPage{
		ActiveTab: "settings",
		Paths:     paths,
		PathError: pathErr,
		Settings:  s,
		SavedOK:   savedOK,
	})
}

func (a *App) handleSettingsSave(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	userID := uid(r)
	save := func(k, v string) error {
		_, err := a.db.Exec(r.Context(), `
			INSERT INTO settings (user_id, key, value) VALUES ($1, $2, $3)
			ON CONFLICT (user_id, key) DO UPDATE SET value = EXCLUDED.value
		`, userID, k, v)
		return err
	}
	if r.FormValue("save_playback") == "1" {
		fo := "0"
		if r.FormValue("force_original") != "" {
			fo = "1"
		}
		dv := r.FormValue("default_volume")
		if dv == "" {
			dv = "1.0"
		}
		for k, v := range map[string]string{"playback_force_original": fo, "playback_default_volume": dv} {
			if err := save(k, v); err != nil {
				httpErr(w, err, 500)
				return
			}
		}
		http.Redirect(w, r, "/settings?saved=1", http.StatusFound)
		return
	}
	updates := map[string]string{
		"transcode_crf":         r.FormValue("crf"),
		"transcode_preset":      r.FormValue("preset"),
		"transcode_max_width":   r.FormValue("max_width"),
		"transcode_video_kbps":  r.FormValue("video_kbps"),
		"transcode_audio_kbps":  r.FormValue("audio_kbps"),
		"transcode_segment_sec": r.FormValue("segment_sec"),
	}
	for k, v := range updates {
		if v == "" {
			continue
		}
		if err := save(k, v); err != nil {
			httpErr(w, err, 500)
			return
		}
	}
	http.Redirect(w, r, "/settings?saved=1", http.StatusFound)
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
	out, err := exec.CommandContext(r.Context(), ffprobePath,
		"-v", "quiet", "-show_entries", "format=duration", "-of", "csv=p=0", absPath,
	).Output()
	if err != nil {
		http.Error(w, "could not probe video", http.StatusInternalServerError)
		return
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || duration <= 0 {
		http.Error(w, "invalid duration", http.StatusInternalServerError)
		return
	}
	ts := a.getTranscodeSettings(r.Context(), uid(r))
	segSec := ts.SegmentSec
	encodedPath := url.QueryEscape(absPath)
	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n")
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n", segSec)
	b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n")
	fullSegments := int(duration) / segSec
	lastDur := duration - float64(fullSegments*segSec)
	for i := range fullSegments {
		fmt.Fprintf(&b, "#EXTINF:%d.000,\n/hls/segment?path=%s&n=%d\n", segSec, encodedPath, i)
	}
	if lastDur > 0.05 {
		fmt.Fprintf(&b, "#EXTINF:%.3f,\n/hls/segment?path=%s&n=%d\n", lastDur, encodedPath, fullSegments)
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
	ts := a.getTranscodeSettings(r.Context(), uid(r))
	startSec := n * ts.SegmentSec
	vbr := fmt.Sprintf("%dk", ts.VideoKbps)
	abr := fmt.Sprintf("%dk", ts.AudioKbps)
	args := []string{
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
	cmd := exec.CommandContext(r.Context(), a.ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stdout = w
	cmd.Stderr = &stderr
	w.Header().Set("Content-Type", "video/mp2t")
	if err := cmd.Run(); err != nil {
		log.Printf("transcode segment %s/%d: %v\n%s", absPath, n, err, stderr.String())
	}
}
