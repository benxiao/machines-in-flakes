package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	db         *pgxpool.Pool
	ffmpegPath string
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
`

func (a *App) initSchema(ctx context.Context) error {
	_, err := a.db.Exec(ctx, schema)
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
	mux.HandleFunc("GET /file", a.handleServeFile)
	mux.HandleFunc("GET /thumbnail", a.handleThumbnail)
	mux.HandleFunc("GET /hls/playlist", a.handleHLSPlaylist)
	mux.HandleFunc("GET /hls/segment", a.handleHLSSegment)
	mux.HandleFunc("GET /video/position", a.handleGetVideoPosition)
	mux.HandleFunc("POST /video/position", a.handleSaveVideoPosition)
	mux.HandleFunc("GET /settings", a.handleSettingsPage)
	mux.HandleFunc("POST /settings", a.handleSettingsSave)
	mux.HandleFunc("POST /paths", a.handlePathAdd)
	mux.HandleFunc("POST /paths/{id}/delete", a.handlePathDelete)
	mux.HandleFunc("GET /playlists", a.handlePlaylistList)
	mux.HandleFunc("POST /playlists", a.handlePlaylistCreate)
	mux.HandleFunc("GET /playlists/{id}", a.handlePlaylistDetail)
	mux.HandleFunc("POST /playlists/{id}/delete", a.handlePlaylistDelete)
	mux.HandleFunc("POST /playlists/{id}/items", a.handlePlaylistItemAdd)
	mux.HandleFunc("POST /playlists/{id}/items/{item_id}/delete", a.handlePlaylistItemDelete)
	mux.HandleFunc("GET /playlists/{id}/state", a.handleGetPlaylistState)
	mux.HandleFunc("POST /playlists/{id}/state", a.handleSavePlaylistState)
	mux.HandleFunc("GET /users", a.handleUserList)
	mux.HandleFunc("POST /users", a.handleUserCreate)
	mux.HandleFunc("POST /users/{id}/delete", a.handleUserDelete)
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
