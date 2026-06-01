package main

import (
	"bytes"
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

// isAllowedPath checks that absPath is equal to or under one of the indexed roots.
func (a *App) isAllowedPath(r *http.Request, absPath string) bool {
	var count int
	err := a.db.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM indexed_paths
		WHERE $1 = path OR starts_with($1, path || '/')
	`, absPath).Scan(&count)
	return err == nil && count > 0
}

func (a *App) handleBrowse(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	dirParam := r.URL.Query().Get("dir")

	if dirParam == "" {
		rows, err := a.db.Query(ctx, `SELECT id, path FROM indexed_paths ORDER BY path`)
		if err != nil {
			httpErr(w, err, 500)
			return
		}
		defer rows.Close()
		var cards []RootCard
		for rows.Next() {
			var c RootCard
			if err := rows.Scan(&c.ID, &c.Path); err != nil {
				continue
			}
			cards = append(cards, c)
		}
		render(w, "browse_root", BrowseRootPage{ActiveTab: "browse", Cards: cards})
		return
	}

	dirParam = filepath.Clean(dirParam)
	if !a.isAllowedPath(r, dirParam) {
		http.NotFound(w, r)
		return
	}

	// Find the indexed root this dir belongs to (for breadcrumb)
	var rootPath string
	a.db.QueryRow(ctx, `
		SELECT path FROM indexed_paths
		WHERE $1 = path OR starts_with($1, path || '/')
		ORDER BY length(path) DESC
		LIMIT 1
	`, dirParam).Scan(&rootPath)

	entries, err := os.ReadDir(dirParam)
	if err != nil {
		httpErr(w, err, 500)
		return
	}

	var subdirs []SubdirRow
	var files []FileRow
	for _, e := range entries {
		if e.IsDir() {
			subdirs = append(subdirs, SubdirRow{
				AbsPath: filepath.Join(dirParam, e.Name()),
				Name:    e.Name(),
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
		wcRows, err := a.db.Query(ctx, `SELECT path, watch_count FROM video_positions WHERE path = ANY($1)`, mediaPaths)
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

	// Fetch playlists for "add to playlist" dropdown.
	plRows, _ := a.db.Query(ctx, `SELECT id, name FROM playlists ORDER BY name`)
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

	render(w, "browse_dir", BrowseDirPage{
		ActiveTab:     "browse",
		Dir:           dirParam,
		DirName:       filepath.Base(dirParam),
		Breadcrumbs:   buildBreadcrumb(dirParam, rootPath),
		Subdirs:       subdirs,
		Files:         files,
		Playlists:     pls,
		PlaylistsJSON: template.JS(plJSON),
	})
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
	rows, err := a.db.Query(r.Context(), `SELECT id, path FROM indexed_paths ORDER BY path`)
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
	_, err = a.db.Exec(r.Context(), `INSERT INTO indexed_paths (path) VALUES ($1) ON CONFLICT DO NOTHING`, path)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	http.Redirect(w, r, "/paths", http.StatusFound)
}

func (a *App) renderPathsWithError(w http.ResponseWriter, r *http.Request, errMsg string) {
	rows, _ := a.db.Query(r.Context(), `SELECT id, path FROM indexed_paths ORDER BY path`)
	var paths []PathRow
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var p PathRow
			rows.Scan(&p.ID, &p.Path)
			paths = append(paths, p)
		}
	}
	render(w, "paths", PathsPage{ActiveTab: "paths", Paths: paths, Error: errMsg})
}

func (a *App) handlePathDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	_, err := a.db.Exec(r.Context(), `DELETE FROM indexed_paths WHERE id=$1`, id)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	http.Redirect(w, r, "/paths", http.StatusFound)
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
		GROUP BY p.id, p.name
		ORDER BY p.name
	`)
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
	err := a.db.QueryRow(r.Context(), `INSERT INTO playlists (name) VALUES ($1) RETURNING id`, name).Scan(&id)
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
	if err := a.db.QueryRow(r.Context(), `SELECT name FROM playlists WHERE id = $1`, id).Scan(&name); err != nil {
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
	if _, err := a.db.Exec(r.Context(), `DELETE FROM playlists WHERE id = $1`, id); err != nil {
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

const hlsSegmentSec = 6

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
	encodedPath := url.QueryEscape(absPath)
	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n")
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n", hlsSegmentSec)
	b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n")
	fullSegments := int(duration) / hlsSegmentSec
	lastDur := duration - float64(fullSegments*hlsSegmentSec)
	for i := range fullSegments {
		if i > 0 {
			b.WriteString("#EXT-X-DISCONTINUITY\n")
		}
		fmt.Fprintf(&b, "#EXTINF:%d.000,\n/hls/segment?path=%s&n=%d\n", hlsSegmentSec, encodedPath, i)
	}
	if lastDur > 0.05 {
		if fullSegments > 0 {
			b.WriteString("#EXT-X-DISCONTINUITY\n")
		}
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
		`SELECT position_sec, watch_count FROM video_positions WHERE path = $1`, absPath,
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
			INSERT INTO video_positions (path, position_sec, watch_count, updated_at)
			VALUES ($1, 0, 1, now())
			ON CONFLICT (path) DO UPDATE
			  SET position_sec = 0,
			      watch_count  = video_positions.watch_count + 1,
			      updated_at   = now()
		`, absPath)
	} else {
		_, err = a.db.Exec(r.Context(), `
			INSERT INTO video_positions (path, position_sec, updated_at)
			VALUES ($1, $2, now())
			ON CONFLICT (path) DO UPDATE
			  SET position_sec = EXCLUDED.position_sec,
			      updated_at   = now()
		`, absPath, req.Position)
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
	startSec := n * hlsSegmentSec
	cmd := exec.CommandContext(r.Context(), a.ffmpegPath,
		"-y",
		"-ss", strconv.Itoa(startSec),
		"-i", absPath,
		"-t", strconv.Itoa(hlsSegmentSec),
		"-c:v", "libx264", "-crf", "23", "-preset", "fast",
		"-profile:v", "main", "-level", "4.0",
		"-pix_fmt", "yuv420p",
		"-bf", "0",
		"-vf", "scale='min(1280,iw)':-2",
		"-b:v", "3000k", "-maxrate", "3000k", "-bufsize", "6000k",
		"-c:a", "aac", "-b:a", "128k",
		"-muxdelay", "0", "-muxpreload", "0",
		"-f", "mpegts", "pipe:1",
	)
	var stderr bytes.Buffer
	cmd.Stdout = w
	cmd.Stderr = &stderr
	w.Header().Set("Content-Type", "video/mp2t")
	if err := cmd.Run(); err != nil {
		log.Printf("transcode segment %s/%d: %v\n%s", absPath, n, err, stderr.String())
	}
}
