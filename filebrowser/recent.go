package main

import (
	"context"
	"net/http"
	"path/filepath"
	"time"
)

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
