package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var appVersion = "dev"

type App struct {
	db               *pgxpool.Pool
	ffmpegPath       string
	markitdownPath   string
	reindexing       sync.Map    // key: int64 userID → bool
	nvencOK          atomic.Bool // h264_nvenc usable (probed at startup)
	pregenBusy       atomic.Bool // thumbnail pre-generation pass running
	dupScanning      sync.Map    // key: int64 userID → bool
	dupResults       sync.Map    // key: int64 userID → *dupScanResult
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
		db:             pool,
		ffmpegPath:     os.Getenv("FB_FFMPEG"),
		markitdownPath: os.Getenv("FB_MARKITDOWN"),
	}
	if err := app.initSchema(ctx); err != nil {
		log.Fatalf("init schema: %v", err)
	}
	app.bootstrapAdmin(ctx, os.Getenv("FB_ADMIN_USERNAME"), os.Getenv("FB_ADMIN_PASSWORD"))
	initTemplates()

	if app.ffmpegPath != "" {
		app.nvencOK.Store(detectNVENC(app.ffmpegPath))
		log.Printf("nvenc: %v", app.nvencOK.Load())
	}

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

	// Purge expired sessions hourly (run once at startup too).
	go func() {
		for {
			if ct, err := pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at < now()`); err == nil && ct.RowsAffected() > 0 {
				log.Printf("sessions: purged %d expired", ct.RowsAffected())
			}
			time.Sleep(time.Hour)
		}
	}()

	// Evict archive extraction cache entries untouched for a day (access
	// bumps mtime, so anything a user keeps playing stays cached).
	go func() {
		for {
			entries, err := os.ReadDir(archiveCacheDir)
			if err == nil {
				purged := 0
				for _, e := range entries {
					info, err := e.Info()
					if err != nil || time.Since(info.ModTime()) < 24*time.Hour {
						continue
					}
					if os.Remove(filepath.Join(archiveCacheDir, e.Name())) == nil {
						purged++
					}
				}
				if purged > 0 {
					log.Printf("archivecache: purged %d stale extraction(s)", purged)
				}
			}
			time.Sleep(time.Hour)
		}
	}()

	// Permanently purge trash items past their 30-day retention window.
	go func() {
		for {
			app.purgeExpiredTrash(ctx)
			time.Sleep(time.Hour)
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
