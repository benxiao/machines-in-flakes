package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
)

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

type trackBookmark struct {
	ID          int64   `json:"id"`
	Label       string  `json:"label"`
	PositionSec float64 `json:"position_sec"`
}

func (a *App) handleListBookmarks(w http.ResponseWriter, r *http.Request) {
	absPath := filepath.Clean(r.URL.Query().Get("path"))
	if !a.isAllowedPath(r, absPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	rows, err := a.db.Query(r.Context(), `
		SELECT id, label, position_sec FROM track_bookmarks
		WHERE user_id = $1 AND path = $2 ORDER BY position_sec ASC
	`, uid(r), absPath)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	defer rows.Close()
	marks := []trackBookmark{}
	for rows.Next() {
		var b trackBookmark
		if rows.Scan(&b.ID, &b.Label, &b.PositionSec) == nil {
			marks = append(marks, b)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(marks)
}

func (a *App) handleAddBookmark(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path        string  `json:"path"`
		Label       string  `json:"label"`
		PositionSec float64 `json:"position_sec"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	absPath := filepath.Clean(req.Path)
	if !a.isAllowedPath(r, absPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if req.PositionSec < 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	label := strings.TrimSpace(req.Label)
	if label == "" {
		label = fmtDurStr(int64(req.PositionSec))
	}
	var id int64
	err := a.db.QueryRow(r.Context(), `
		INSERT INTO track_bookmarks (user_id, path, label, position_sec) VALUES ($1, $2, $3, $4)
		RETURNING id
	`, uid(r), absPath, label, req.PositionSec).Scan(&id)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trackBookmark{ID: id, Label: label, PositionSec: req.PositionSec})
}

func (a *App) handleDeleteBookmark(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if _, err := a.db.Exec(r.Context(), `DELETE FROM track_bookmarks WHERE id = $1 AND user_id = $2`, id, uid(r)); err != nil {
		httpErr(w, err, 500)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// folderKeyFor names the top-level folder a path groups under for the "top
// folders" stat: the first path segment below whichever allowed root most
// specifically contains it (e.g. the composer under a Classical music root),
// or the root's own name if the file sits directly inside it.
func folderKeyFor(absPath string, roots []string) string {
	var best string
	for _, root := range roots {
		if underRoot(absPath, root) && len(root) > len(best) {
			best = root
		}
	}
	if best == "" {
		return ""
	}
	rel := strings.TrimPrefix(absPath, best+"/")
	if rel == absPath {
		return filepath.Base(best)
	}
	if i := strings.IndexByte(rel, '/'); i >= 0 {
		return rel[:i]
	}
	return filepath.Base(best)
}

func (a *App) accumulatePlayTime(ctx context.Context, userID int64, deltaSec int, mediaType, folder string) {
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
	if folder != "" {
		a.db.Exec(ctx, `
			INSERT INTO folder_play_time (user_id, media_type, folder, seconds) VALUES ($1, $2, $3, $4)
			ON CONFLICT (user_id, media_type, folder) DO UPDATE SET seconds = folder_play_time.seconds + EXCLUDED.seconds
		`, userID, mediaType, folder, deltaSec)
	}
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
	mediaType := "video"
	if classifyExt(filepath.Ext(absPath)) == "audio" {
		mediaType = "audio"
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
	a.accumulatePlayTime(r.Context(), uid(r), req.DeltaSec, mediaType, folderKeyFor(absPath, a.allowedRoots(r)))
	w.WriteHeader(http.StatusNoContent)
}
