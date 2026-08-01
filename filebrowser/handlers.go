package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"image"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/rwcarlsen/goexif/exif"
	"golang.org/x/crypto/bcrypt"
)

// resolveMediaPath maps a possibly-virtual path to a real file that
// ServeFile/ffmpeg can read. Real paths pass through untouched; archive-
// internal paths extract into the cache. Callers keep the original path for
// DB keys, Content-Disposition and the thumbnail cache key.
// handlePhotoExif returns capture metadata for the photo viewer's info panel:
// pixel dimensions (stdlib image decoders — jpeg/png/gif only, other photo
// formats just omit them) plus EXIF camera/date/GPS fields where the file
// carries them (jpeg/tiff only; goexif returns ErrTagNotPresent otherwise,
// which every field below treats as "leave it blank", not a failure).
func (a *App) handlePhotoExif(w http.ResponseWriter, r *http.Request) {
	absPath := filepath.Clean(r.URL.Query().Get("path"))
	if !a.isAllowedPath(r, absPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if classifyExt(filepath.Ext(absPath)) != "photo" {
		http.Error(w, "not a photo", http.StatusBadRequest)
		return
	}
	servePath, err := a.resolveMediaPath(r.Context(), absPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(servePath)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(servePath)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	defer f.Close()

	out := struct {
		Width, Height int
		Make, Model   string
		DateTaken     string
		GPSLat        float64 `json:"gpsLat,omitempty"`
		GPSLon        float64 `json:"gpsLon,omitempty"`
		HasGPS        bool
		SizeBytes     int64
		ModTime       string
	}{SizeBytes: info.Size(), ModTime: info.ModTime().Local().Format("2006-01-02 15:04")}

	if x, err := exif.Decode(f); err == nil {
		if tag, err := x.Get(exif.Make); err == nil {
			out.Make, _ = tag.StringVal()
		}
		if tag, err := x.Get(exif.Model); err == nil {
			out.Model, _ = tag.StringVal()
		}
		if dt, err := x.DateTime(); err == nil {
			out.DateTaken = dt.Format("2006-01-02 15:04")
		}
		if lat, lon, err := x.LatLong(); err == nil {
			out.GPSLat, out.GPSLon, out.HasGPS = lat, lon, true
		}
	}
	if _, err := f.Seek(0, io.SeekStart); err == nil {
		if cfg, _, err := image.DecodeConfig(f); err == nil {
			out.Width, out.Height = cfg.Width, cfg.Height
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// listArchiveDir lists the immediate children of inner within archiveFile as
// rows whose AbsPaths are virtual paths under dirParam. Directories that
// exist only as prefixes of deeper entries are synthesized.
func listArchiveDir(archiveFile, inner, dirParam string) ([]SubdirRow, []FileRow, error) {
	entries, err := listArchiveEntries(archiveFile)
	if err != nil {
		return nil, nil, err
	}
	prefix := ""
	if inner != "" {
		prefix = inner + "/"
	}
	dirTimes := make(map[string]time.Time)
	var files []FileRow
	for _, e := range entries {
		name := e.Name
		if name == "." || strings.HasPrefix(name, "../") || !strings.HasPrefix(name, prefix) {
			continue
		}
		rest := name[len(prefix):]
		if rest == "" {
			continue
		}
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			// A deeper entry implies a child directory at this level.
			d := rest[:i]
			if t, seen := dirTimes[d]; !seen || e.ModTime.After(t) {
				dirTimes[d] = e.ModTime
			}
			continue
		}
		if e.IsDir {
			if _, seen := dirTimes[rest]; !seen {
				dirTimes[rest] = e.ModTime
			}
			continue
		}
		ext := path.Ext(rest)
		files = append(files, FileRow{
			AbsPath:    dirParam + "/" + rest,
			Filename:   rest,
			Extension:  strings.ToLower(ext),
			FileType:   classifyExt(ext),
			SizeBytes:  e.Size,
			Size:       formatSize(e.Size),
			ModifiedAt: e.ModTime.Format("2006-01-02 15:04"),
			ModTime:    e.ModTime,
		})
	}
	subdirs := make([]SubdirRow, 0, len(dirTimes))
	for d, t := range dirTimes {
		subdirs = append(subdirs, SubdirRow{
			AbsPath:    dirParam + "/" + d,
			Name:       d,
			ModifiedAt: t.Format("2006-01-02 15:04"),
			ModTime:    t,
		})
	}
	return subdirs, files, nil
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

	sortBy := r.URL.Query().Get("sort") // "" or "name" = alphabetical; "date" = newest first

	var subdirs []SubdirRow
	var files []FileRow
	inArchive := false
	if archiveFile, inner, ok := splitArchivePath(dirParam); ok {
		// A .zip/.rar file (or a virtual dir inside one) browses like a folder.
		inArchive = true
		subdirs, files, err = listArchiveDir(archiveFile, inner, dirParam)
		if err != nil {
			httpErr(w, err, 500)
			return
		}
	} else {
		entries, err := os.ReadDir(dirParam)
		if err != nil {
			httpErr(w, err, 500)
			return
		}
		for _, e := range entries {
			if e.Name() == trashDirName {
				continue
			}
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
			absFile := filepath.Join(dirParam, e.Name())
			fileType := classifyExt(ext)
			var art string
			if fileType == "archive" {
				art = findArchiveAlbumArt(absFile)
			}
			files = append(files, FileRow{
				AbsPath:    absFile,
				Filename:   e.Name(),
				Extension:  strings.ToLower(ext),
				FileType:   fileType,
				SizeBytes:  info.Size(),
				Size:       formatSize(info.Size()),
				ModifiedAt: info.ModTime().Format("2006-01-02 15:04"),
				ModTime:    info.ModTime(),
				AlbumArt:   art,
			})
		}
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
		InArchive:     inArchive,
	})
}

func (a *App) handleRecent(w http.ResponseWriter, r *http.Request) {
	userID := uid(r)
	artCache := make(map[string]string)
	albumArt := func(ft, dir string) string {
		if ft != "audio" {
			return ""
		}
		if cached, ok := artCache[dir]; ok {
			return cached
		}
		art := findAlbumArt(dir)
		artCache[dir] = art
		return art
	}

	// Section 1 — Continue (not finished): items with a saved mid-file resume
	// point (position_sec > 30 skips barely-started noise), newest played first.
	continuing, err := func() ([]RecentItem, error) {
		rows, err := a.db.Query(r.Context(), `
			SELECT path, watch_count, updated_at, position_sec
			FROM video_positions
			WHERE user_id = $1 AND position_sec > 30
			  AND EXISTS (
				SELECT 1 FROM indexed_paths ip
				WHERE ip.user_id = $1 AND ip.enabled = TRUE
				  AND (video_positions.path = ip.path OR starts_with(video_positions.path, ip.path || '/'))
			  )
			ORDER BY updated_at DESC
			LIMIT 100
		`, userID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var items []RecentItem
		var nVideo, nAudio int
		for rows.Next() {
			var path string
			var wc int64
			var t time.Time
			var pos float64
			if rows.Scan(&path, &wc, &t, &pos) != nil {
				continue
			}
			ft := classifyExt(filepath.Ext(path))
			// Cap the continue list at 10 video + 10 audio, newest first.
			switch ft {
			case "video":
				if nVideo >= 10 {
					continue
				}
				nVideo++
			case "audio":
				if nAudio >= 10 {
					continue
				}
				nAudio++
			default:
				continue
			}
			dir := filepath.Dir(path)
			items = append(items, RecentItem{
				Path:        path,
				Filename:    filepath.Base(path),
				FileType:    ft,
				Dir:         dir,
				WatchCount:  wc,
				UpdatedAt:   t.Local().Format("2006-01-02 15:04"),
				PositionSec: pos,
				AlbumArt:    albumArt(ft, dir),
			})
			if nVideo >= 10 && nAudio >= 10 {
				break
			}
		}
		return items, rows.Err()
	}()
	if err != nil {
		httpErr(w, err, 500)
		return
	}

	// Section 2 — Recently added: newest files in the library by filesystem
	// mtime (captured at reindex). file_index only holds files under enabled
	// roots, so no extra indexed_paths gating is needed.
	added, err := func() ([]RecentItem, error) {
		rows, err := a.db.Query(r.Context(), `
			SELECT path, filename, file_type, dir_path, mtime
			FROM file_index
			WHERE user_id = $1 AND mtime IS NOT NULL
			  AND file_type IN ('video','audio','photo')
			ORDER BY mtime DESC
			LIMIT 24
		`, userID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var items []RecentItem
		for rows.Next() {
			var path, filename, ft, dir string
			var t time.Time
			if rows.Scan(&path, &filename, &ft, &dir, &t) != nil {
				continue
			}
			items = append(items, RecentItem{
				Path:     path,
				Filename: filename,
				FileType: ft,
				Dir:      dir,
				AddedAt:  t.Local().Format("Jan 2, 2006"),
				AlbumArt: albumArt(ft, dir),
			})
		}
		return items, rows.Err()
	}()
	if err != nil {
		httpErr(w, err, 500)
		return
	}

	a.enrichRecentContext(r.Context(), userID, continuing)
	a.enrichRecentContext(r.Context(), userID, added)
	render(w, "recent", RecentPage{ActiveTab: "recent", IsAdmin: isAdmin(r), Continuing: continuing, Added: added})
}

// enrichRecentContext fills in, per item, which playback context a click
// should resume: a saved Playlist wins over Favorites wins over the plain
// "browse to its folder" fallback. Neither lookup requires schema changes —
// both are derived from tables that already exist for other pages.
func (a *App) enrichRecentContext(ctx context.Context, userID int64, items []RecentItem) {
	if len(items) == 0 {
		return
	}
	_, favTracks, err := a.buildFavoritesQueue(ctx, userID, func(p string) string { return p })
	favIdx := make(map[string]int, len(favTracks))
	if err == nil {
		for i, t := range favTracks {
			if _, ok := favIdx[t.Path]; !ok {
				favIdx[t.Path] = i
			}
		}
	}
	for i := range items {
		var playlistID int64
		var idx int
		// idx is the item's rank under playlist_items' own ORDER BY (position, id) —
		// the array index startPlaylistItem(idx,...) needs, not the raw position
		// column (which can have gaps after reorders/deletions).
		err := a.db.QueryRow(ctx, `
			SELECT pi.playlist_id,
			       (SELECT COUNT(*) FROM playlist_items pi2
			        WHERE pi2.playlist_id = pi.playlist_id
			          AND (pi2.position, pi2.id) < (pi.position, pi.id))
			FROM playlist_items pi JOIN playlists p ON p.id = pi.playlist_id
			WHERE pi.path = $1 AND p.user_id = $2
			ORDER BY pi.playlist_id LIMIT 1
		`, items[i].Path, userID).Scan(&playlistID, &idx)
		if err == nil {
			items[i].Context = "playlist"
			items[i].ContextID = playlistID
			items[i].ContextStart = idx
			continue
		}
		if fi, ok := favIdx[items[i].Path]; ok {
			items[i].Context = "favorites"
			items[i].ContextStart = fi
		}
	}
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
	servePath, err := a.resolveMediaPath(r.Context(), absPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(servePath)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	filename := filepath.Base(absPath)
	if r.URL.Query().Get("dl") == "1" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		http.ServeFile(w, r, servePath)
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
	http.ServeFile(w, r, servePath)
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
	// Require a local path: "//host" and "/\host" are protocol-relative
	// URLs to browsers, i.e. open redirects.
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, `/\`) {
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
	path = filepath.Clean(path)
	// The settings page shows sizes for the user's own roots (even disabled
	// ones) and for enabled granted roots; anything else is off limits.
	var count int
	err := a.db.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM indexed_paths ip
		WHERE ($2 = ip.path OR starts_with($2, ip.path || '/'))
		  AND (ip.user_id = $1
		       OR (ip.enabled AND EXISTS (SELECT 1 FROM path_grants pg WHERE pg.path_id = ip.id AND pg.user_id = $1)))
	`, uid(r), path).Scan(&count)
	if err != nil || count == 0 {
		http.Error(w, "Forbidden", http.StatusForbidden)
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

// handleClientLog lets the mobile player beacon diagnostic events into the
// server journal, since background playback failures on the phone are
// otherwise unobservable (no devtools with the screen off).
func (a *App) handleClientLog(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(io.LimitReader(r.Body, 2048))
	// Strip control characters (newlines, ANSI escapes) so a client can't
	// forge journal lines or inject terminal sequences.
	msg := strings.Map(func(c rune) rune {
		if c < 0x20 || c == 0x7f {
			return ' '
		}
		return c
	}, string(b))
	log.Printf("[client u=%d] %s", uid(r), msg)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handlePDFMarkdown(w http.ResponseWriter, r *http.Request) {
	absPath := filepath.Clean(r.URL.Query().Get("path"))
	if !a.isAllowedPath(r, absPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if strings.ToLower(filepath.Ext(absPath)) != ".pdf" {
		http.Error(w, "not a PDF", http.StatusBadRequest)
		return
	}
	srcPath, err := a.resolveMediaPath(r.Context(), absPath)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	if info, err := os.Stat(srcPath); err != nil || info.IsDir() {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	mdBin := a.markitdownPath
	if mdBin == "" {
		mdBin = "markitdown"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, mdBin, srcPath).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			http.Error(w, "markitdown failed: "+string(exitErr.Stderr), http.StatusInternalServerError)
		} else {
			http.Error(w, "markitdown error: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(out)
}
