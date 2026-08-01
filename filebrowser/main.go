package main

import (
	"context"
	"crypto/tls"
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

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// certLoader returns a tls.Config.GetCertificate callback that reads
// certFile/keyFile from disk lazily, re-parsing only when the cert file's
// mtime advances -- so a renewed Tailscale cert (see the tailscale-cert
// systemd timer in flake.nix) takes effect on the next handshake with no
// restart. If a reload attempt fails (e.g. mid-write by the renewal job),
// the previously-loaded cert keeps serving rather than failing the request.
func certLoader(certFile, keyFile string) func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	var mu sync.Mutex
	var cached *tls.Certificate
	var cachedModTime time.Time
	return func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		info, err := os.Stat(certFile)
		if err != nil {
			return nil, err
		}
		mu.Lock()
		defer mu.Unlock()
		if cached == nil || info.ModTime().After(cachedModTime) {
			cert, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				if cached != nil {
					log.Printf("tls: reload failed, keeping previous cert: %v", err)
					return cached, nil
				}
				return nil, err
			}
			cached = &cert
			cachedModTime = info.ModTime()
			log.Printf("tls: loaded cert (modified %s)", cachedModTime.Format(time.RFC3339))
		}
		return cached, nil
	}
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
	mux.HandleFunc("GET /auth/google/login", app.handleGoogleLoginStart)
	mux.HandleFunc("GET /auth/google/callback", app.handleGoogleCallback)
	app.registerRoutes(mux)

	certFile := envOr("FB_TLS_CERT", "/var/lib/tailscale-certs/cert.pem")
	keyFile := envOr("FB_TLS_KEY", "/var/lib/tailscale-certs/key.pem")
	srv := &http.Server{
		Addr:    addr,
		Handler: app.withAuth(mux),
		TLSConfig: &tls.Config{
			GetCertificate: certLoader(certFile, keyFile),
		},
	}
	log.Printf("filebrowser listening on %s (https)", addr)
	log.Fatal(srv.ListenAndServeTLS("", ""))
}
