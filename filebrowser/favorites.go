package main

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
)

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

	favItems, tracks, err := a.buildFavoritesQueue(ctx, userID, trimPath)
	if err != nil {
		httpErr(w, err, 500)
		return
	}

	render(w, "favorites", FavoritesPage{ActiveTab: "favorites", IsAdmin: isAdmin(r), Items: favItems, Tracks: tracks})
}

// buildFavoritesQueue rebuilds the flattened favorites playback queue exactly
// as handleFavoritesPage renders it: favorited folders expand to their audio
// contents, favorited individual tracks are single entries, both in
// (position, created_at) order. Shared with handleRecent, which needs to
// reverse-lookup a bare path's index in this same queue.
func (a *App) buildFavoritesQueue(ctx context.Context, userID int64, trimPath func(string) string) ([]FavoriteItem, []PlaylistItem, error) {
	favRows, err := a.db.Query(ctx, `
		SELECT path, is_folder FROM favorites WHERE user_id = $1 ORDER BY position, created_at
	`, userID)
	if err != nil {
		return nil, nil, err
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

	return favItems, tracks, nil
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
