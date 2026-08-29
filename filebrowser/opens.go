package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"time"
)

// recordOpenPath upserts path's last-opened timestamp for userID, driving
// the "opened" sort in the browse listing and (for folders) the landing-page
// redirect in handleBrowse. Errors are logged, not surfaced — an open-
// tracking write is never worth failing the request that triggered it.
func (a *App) recordOpenPath(ctx context.Context, userID int64, path string, isFolder bool) {
	if _, err := a.db.Exec(ctx, `
		INSERT INTO last_opened (user_id, path, is_folder, opened_at) VALUES ($1, $2, $3, now())
		ON CONFLICT (user_id, path) DO UPDATE SET is_folder = EXCLUDED.is_folder, opened_at = now()
	`, userID, path, isFolder); err != nil {
		log.Printf("record open %s: %v", path, err)
	}
}

// lastOpenedFolder returns the path of the folder this user most recently
// browsed to, or "" if they've never browsed one (new user, or history
// predates the is_folder column). The caller still needs to check the path
// is currently valid (isAllowedPath) — roots and grants can change, and the
// folder itself can be deleted or moved, after it was recorded.
func (a *App) lastOpenedFolder(ctx context.Context, userID int64) string {
	var path string
	err := a.db.QueryRow(ctx, `
		SELECT path FROM last_opened WHERE user_id = $1 AND is_folder = TRUE
		ORDER BY opened_at DESC LIMIT 1
	`, userID).Scan(&path)
	if err != nil {
		return ""
	}
	return path
}

// fetchLastOpened batch-loads last-opened timestamps for a set of paths,
// keyed by path. A path with no recorded open is simply absent from the map.
func (a *App) fetchLastOpened(ctx context.Context, userID int64, paths []string) map[string]time.Time {
	out := make(map[string]time.Time, len(paths))
	rows, err := a.db.Query(ctx, `SELECT path, opened_at FROM last_opened WHERE user_id = $1 AND path = ANY($2)`, userID, paths)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var p string
		var t time.Time
		if rows.Scan(&p, &t) == nil {
			out[p] = t
		}
	}
	return out
}

// handleRecordOpen marks a file as just-opened by the current user. Folders
// record their own opens server-side in handleBrowse; files are viewed
// client-side (the preview modal or the playlist player), so the client
// pings this endpoint the moment it opens one.
func (a *App) handleRecordOpen(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
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
	a.recordOpenPath(r.Context(), uid(r), absPath, false)
	w.WriteHeader(http.StatusNoContent)
}
