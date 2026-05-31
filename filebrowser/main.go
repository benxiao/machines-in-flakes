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
	db *pgxpool.Pool
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
`

func (a *App) initSchema(ctx context.Context) error {
	_, err := a.db.Exec(ctx, schema)
	return err
}

func (a *App) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/browse", http.StatusFound)
	})
	mux.HandleFunc("GET /browse", a.handleBrowse)
	mux.HandleFunc("GET /file", a.handleServeFile)
	mux.HandleFunc("GET /paths", a.handlePathsList)
	mux.HandleFunc("POST /paths", a.handlePathAdd)
	mux.HandleFunc("POST /paths/{id}/delete", a.handlePathDelete)
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

	app := &App{db: pool}
	if err := app.initSchema(ctx); err != nil {
		log.Fatalf("init schema: %v", err)
	}
	initTemplates()

	addr := os.Getenv("FB_LISTEN")
	if addr == "" {
		addr = ":10094"
	}
	mux := http.NewServeMux()
	app.registerRoutes(mux)
	log.Printf("filebrowser listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
