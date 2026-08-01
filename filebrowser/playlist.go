package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

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
	var folder string
	if req.DeltaSec > 0 {
		var path string
		if err := a.db.QueryRow(r.Context(),
			`SELECT path FROM playlist_items WHERE playlist_id = $1 ORDER BY position LIMIT 1 OFFSET $2`,
			id, req.CurrentIndex,
		).Scan(&path); err == nil {
			folder = folderKeyFor(path, a.allowedRoots(r))
		}
	}
	a.accumulatePlayTime(r.Context(), uid(r), req.DeltaSec, req.MediaType, folder)
	w.WriteHeader(http.StatusNoContent)
}


// handleFolderPlay renders the playlist player for a folder's audio files, so
// mobile album playback runs on the gapless MSE engine instead of the preview
// modal (whose per-track src swap lets Android freeze the page in background).
func (a *App) handleFolderPlay(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Query().Get("file")
	dir := r.URL.Query().Get("dir")
	if dir == "" && file != "" {
		dir = filepath.Dir(file)
	}
	if dir == "" {
		http.NotFound(w, r)
		return
	}
	dir = filepath.Clean(dir)
	if !a.isAllowedPath(r, dir) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	var items []PlaylistItem
	startIdx := 0
	if archiveFile, inner, ok := splitArchivePath(dir); ok {
		_, zfiles, err := listArchiveDir(archiveFile, inner, dir)
		if err != nil {
			httpErr(w, err, 500)
			return
		}
		sort.Slice(zfiles, func(i, j int) bool {
			return strings.ToLower(zfiles[i].Filename) < strings.ToLower(zfiles[j].Filename)
		})
		for _, f := range zfiles {
			if f.FileType != "audio" {
				continue
			}
			if file != "" && f.AbsPath == filepath.Clean(file) {
				startIdx = len(items)
			}
			items = append(items, PlaylistItem{Path: f.AbsPath, Name: f.Filename, FileType: "audio"})
		}
	} else {
		entries, err := os.ReadDir(dir)
		if err != nil {
			httpErr(w, err, 500)
			return
		}
		for _, e := range entries {
			if e.IsDir() || classifyExt(filepath.Ext(e.Name())) != "audio" {
				continue
			}
			p := filepath.Join(dir, e.Name())
			if file != "" && p == filepath.Clean(file) {
				startIdx = len(items)
			}
			items = append(items, PlaylistItem{Path: p, Name: e.Name(), FileType: "audio"})
		}
	}
	render(w, "folder-play", FolderPlayPage{
		ActiveTab: "browse", IsAdmin: isAdmin(r),
		Folder: filepath.Base(dir), Dir: dir, StartIdx: startIdx, Items: items,
	})
}

func (a *App) handleFolderPlaylistAdd(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path       string `json:"path"`
		PlaylistID int64  `json:"playlist_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PlaylistID == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	absPath := filepath.Clean(req.Path)
	if !a.isAllowedPath(r, absPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if info, err := os.Stat(absPath); err != nil || !info.IsDir() {
		http.Error(w, "not a directory", http.StatusBadRequest)
		return
	}
	var mediaPaths []string
	if werr := filepath.WalkDir(absPath, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ft := classifyExt(filepath.Ext(path))
		if ft == "audio" || ft == "video" {
			mediaPaths = append(mediaPaths, path)
		}
		return nil
	}); werr != nil {
		log.Printf("folder playlist-add walk %s: %v", absPath, werr)
	}
	if len(mediaPaths) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"added": 0})
		return
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	defer tx.Rollback(r.Context())
	added := 0
	for _, p := range mediaPaths {
		tag, err := tx.Exec(r.Context(), `
			INSERT INTO playlist_items (playlist_id, path, position)
			SELECT $1, $2, COALESCE((SELECT MAX(position)+1 FROM playlist_items WHERE playlist_id=$1), 0)
			FROM playlists WHERE id = $1 AND user_id = $3
			ON CONFLICT DO NOTHING
		`, req.PlaylistID, p, uid(r))
		if err != nil {
			httpErr(w, err, 500)
			return
		}
		added += int(tag.RowsAffected())
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpErr(w, err, 500)
		return
	}
	log.Printf("folder playlist-add u=%d: %d media files from %s -> playlist %d", uid(r), added, absPath, req.PlaylistID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"added": added})
}
