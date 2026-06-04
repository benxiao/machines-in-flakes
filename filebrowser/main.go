package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	db         *pgxpool.Pool
	ffmpegPath string
	reindexing sync.Map // key: int64 userID → bool
}

func systemTimezone() string {
	target, err := os.Readlink("/etc/localtime")
	if err != nil {
		return time.Local.String()
	}
	const marker = "zoneinfo/"
	if i := strings.LastIndex(target, marker); i >= 0 {
		return target[i+len(marker):]
	}
	return ""
}

const schema = `
CREATE TABLE IF NOT EXISTS indexed_paths (
	id   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	path TEXT NOT NULL UNIQUE
);
CREATE TABLE IF NOT EXISTS video_positions (
	path         TEXT PRIMARY KEY,
	position_sec DOUBLE PRECISION NOT NULL DEFAULT 0,
	watch_count  BIGINT NOT NULL DEFAULT 0,
	updated_at   TIMESTAMPTZ DEFAULT now()
);
CREATE TABLE IF NOT EXISTS playlists (
	id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	name       TEXT NOT NULL,
	created_at TIMESTAMPTZ DEFAULT now()
);
CREATE TABLE IF NOT EXISTS playlist_items (
	id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	playlist_id BIGINT NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
	path        TEXT NOT NULL,
	UNIQUE (playlist_id, path)
);
CREATE TABLE IF NOT EXISTS playlist_state (
	playlist_id   BIGINT PRIMARY KEY REFERENCES playlists(id) ON DELETE CASCADE,
	current_index INT NOT NULL DEFAULT 0,
	position_sec  DOUBLE PRECISION NOT NULL DEFAULT 0,
	updated_at    TIMESTAMPTZ DEFAULT now()
);
CREATE TABLE IF NOT EXISTS users (
	id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	username      TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	created_at    TIMESTAMPTZ DEFAULT now()
);
CREATE TABLE IF NOT EXISTS sessions (
	token      TEXT PRIMARY KEY,
	user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at TIMESTAMPTZ NOT NULL,
	created_at TIMESTAMPTZ DEFAULT now()
);
CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
ALTER TABLE indexed_paths ADD COLUMN IF NOT EXISTS enabled BOOLEAN NOT NULL DEFAULT TRUE;
CREATE TABLE IF NOT EXISTS file_index (
	user_id   BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	path      TEXT NOT NULL,
	filename  TEXT NOT NULL,
	file_type TEXT NOT NULL,
	dir_path  TEXT NOT NULL,
	PRIMARY KEY (user_id, path)
);
CREATE INDEX IF NOT EXISTS file_index_search ON file_index (user_id, lower(filename));
`

const migrations = `
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='indexed_paths' AND column_name='user_id') THEN
    ALTER TABLE indexed_paths ADD COLUMN user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
    UPDATE indexed_paths SET user_id = (SELECT id FROM users ORDER BY id LIMIT 1) WHERE user_id IS NULL;
    ALTER TABLE indexed_paths DROP CONSTRAINT IF EXISTS indexed_paths_path_key;
    ALTER TABLE indexed_paths ADD CONSTRAINT indexed_paths_user_id_path_key UNIQUE (user_id, path);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='settings' AND column_name='user_id') THEN
    ALTER TABLE settings ADD COLUMN user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
    UPDATE settings SET user_id = (SELECT id FROM users ORDER BY id LIMIT 1) WHERE user_id IS NULL;
    IF NOT EXISTS (SELECT 1 FROM settings WHERE user_id IS NULL) THEN
      ALTER TABLE settings DROP CONSTRAINT IF EXISTS settings_pkey;
      ALTER TABLE settings ADD PRIMARY KEY (user_id, key);
    END IF;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='playlists' AND column_name='user_id') THEN
    ALTER TABLE playlists ADD COLUMN user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
    UPDATE playlists SET user_id = (SELECT id FROM users ORDER BY id LIMIT 1) WHERE user_id IS NULL;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='playlist_items' AND column_name='position') THEN
    ALTER TABLE playlist_items ADD COLUMN position INT NOT NULL DEFAULT 0;
    UPDATE playlist_items pi SET position = sub.rn - 1
    FROM (SELECT id, ROW_NUMBER() OVER (PARTITION BY playlist_id ORDER BY id) AS rn FROM playlist_items) sub
    WHERE pi.id = sub.id;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='video_positions' AND column_name='user_id') THEN
    ALTER TABLE video_positions ADD COLUMN user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
    UPDATE video_positions SET user_id = (SELECT id FROM users ORDER BY id LIMIT 1) WHERE user_id IS NULL;
    IF NOT EXISTS (SELECT 1 FROM video_positions WHERE user_id IS NULL) THEN
      ALTER TABLE video_positions DROP CONSTRAINT IF EXISTS video_positions_pkey;
      ALTER TABLE video_positions ADD PRIMARY KEY (user_id, path);
    END IF;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='is_admin') THEN
    ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT FALSE;
    UPDATE users SET is_admin = TRUE WHERE id = (SELECT MIN(id) FROM users);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name='path_grants') THEN
    CREATE TABLE path_grants (
      path_id BIGINT NOT NULL REFERENCES indexed_paths(id) ON DELETE CASCADE,
      user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
      PRIMARY KEY (path_id, user_id)
    );
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name='play_time') THEN
    CREATE TABLE play_time (
      user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
      day     DATE NOT NULL,
      seconds BIGINT NOT NULL DEFAULT 0,
      PRIMARY KEY (user_id, day)
    );
  END IF;
END $$;
`

func (a *App) initSchema(ctx context.Context) error {
	if _, err := a.db.Exec(ctx, schema); err != nil {
		return err
	}
	_, err := a.db.Exec(ctx, migrations)
	return err
}

func (a *App) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /favicon.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "max-age=86400")
		w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="#58a6ff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>`))
	})
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/browse", http.StatusFound)
	})
	mux.HandleFunc("GET /browse", a.handleBrowse)
	mux.HandleFunc("GET /recent", a.handleRecent)
	mux.HandleFunc("GET /file", a.handleServeFile)
	mux.HandleFunc("GET /thumbnail", a.handleThumbnail)
	mux.HandleFunc("GET /hls/playlist", a.handleHLSPlaylist)
	mux.HandleFunc("GET /hls/segment", a.handleHLSSegment)
	mux.HandleFunc("GET /video/position", a.handleGetVideoPosition)
	mux.HandleFunc("POST /video/position", a.handleSaveVideoPosition)
	mux.HandleFunc("GET /play/stats", a.handlePlayStats)
	mux.HandleFunc("GET /settings", a.handleSettingsPage)
	mux.HandleFunc("POST /settings", a.handleSettingsSave)
	mux.HandleFunc("POST /paths", a.handlePathAdd)
	mux.HandleFunc("POST /paths/{id}/delete", a.handlePathDelete)
	mux.HandleFunc("POST /paths/{id}/toggle", a.handlePathToggle)
	mux.HandleFunc("GET /search", a.handleSearch)
	mux.HandleFunc("GET /search/status", a.handleSearchStatus)
	mux.HandleFunc("POST /search/reindex", a.handleSearchReindex)
	mux.HandleFunc("GET /playlists", a.handlePlaylistList)
	mux.HandleFunc("POST /playlists", a.handlePlaylistCreate)
	mux.HandleFunc("GET /playlists/{id}", a.handlePlaylistDetail)
	mux.HandleFunc("POST /playlists/{id}/delete", a.handlePlaylistDelete)
	mux.HandleFunc("POST /playlists/{id}/items", a.handlePlaylistItemAdd)
	mux.HandleFunc("POST /playlists/{id}/reorder", a.handlePlaylistReorder)
	mux.HandleFunc("POST /playlists/{id}/items/{item_id}/delete", a.handlePlaylistItemDelete)
	mux.HandleFunc("GET /playlists/{id}/state", a.handleGetPlaylistState)
	mux.HandleFunc("POST /playlists/{id}/state", a.handleSavePlaylistState)
	mux.HandleFunc("GET /users", a.handleUserList)
	mux.HandleFunc("GET /users/{id}", a.handleUserDetail)
	mux.HandleFunc("POST /users", a.handleUserCreate)
	mux.HandleFunc("POST /users/{id}/delete", a.handleUserDelete)
	mux.HandleFunc("POST /paths/{id}/grant", a.handlePathGrant)
	mux.HandleFunc("POST /paths/{id}/revoke/{uid}", a.handlePathRevoke)
}

func main() {
	ctx := context.Background()
	dsn := os.Getenv("FB_DB_DSN")
	if dsn == "" {
		dsn = "host=/run/postgresql dbname=filebrowser user=filebrowser sslmode=disable"
	}
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("parse dsn: %v", err)
	}
	if tz := systemTimezone(); tz != "" && tz != "UTC" {
		poolCfg.ConnConfig.RuntimeParams["timezone"] = tz
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	app := &App{
		db:         pool,
		ffmpegPath: os.Getenv("FB_FFMPEG"),
	}
	if err := app.initSchema(ctx); err != nil {
		log.Fatalf("init schema: %v", err)
	}
	app.bootstrapAdmin(ctx, os.Getenv("FB_ADMIN_USERNAME"), os.Getenv("FB_ADMIN_PASSWORD"))
	initTemplates()

	// Reindex all users every 6 hours so the search index stays fresh.
	go func() {
		for {
			time.Sleep(6 * time.Hour)
			rows, err := pool.Query(ctx, `SELECT id FROM users`)
			if err != nil {
				continue
			}
			var userIDs []int64
			for rows.Next() {
				var id int64
				if rows.Scan(&id) == nil {
					userIDs = append(userIDs, id)
				}
			}
			rows.Close()
			for _, id := range userIDs {
				uid := id
				go func() {
					c, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
					defer cancel()
					app.reindexUser(c, uid)
				}()
			}
		}
	}()

	addr := os.Getenv("FB_LISTEN")
	if addr == "" {
		addr = ":10094"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /login", app.handleLoginGet)
	mux.HandleFunc("POST /login", app.handleLoginPost)
	mux.HandleFunc("POST /logout", app.handleLogout)
	app.registerRoutes(mux)
	log.Printf("filebrowser listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, app.withAuth(mux)))
}
