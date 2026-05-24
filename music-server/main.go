package main

import (
	"log"
	"net/http"
	"os"
	"sync"
)

// App holds shared state for the lifetime of the server.
type App struct {
	musicDir string
	mu       sync.RWMutex
	lib      *Library
}

func main() {
	musicDir := os.Getenv("MUSIC_DIR")
	if musicDir == "" {
		musicDir = "/blue2t/music"
	}

	log.Printf("music-server: scanning %s…", musicDir)
	lib, err := scanLibrary(musicDir)
	if err != nil {
		log.Fatalf("scan: %v", err)
	}
	log.Printf("music-server: %d genres · %d artists · %d albums · %d tracks",
		lib.genreCount(), lib.artistCount(), lib.albumCount(), lib.trackCount())

	app := &App{musicDir: musicDir, lib: lib}
	initTemplates()

	addr := os.Getenv("MUSIC_LISTEN")
	if addr == "" {
		addr = ":10093"
	}
	mux := http.NewServeMux()
	app.registerRoutes(mux)
	log.Printf("music-server: listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func (a *App) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", a.handleRoot)
	mux.HandleFunc("GET /library", a.handleLibrary)
	mux.HandleFunc("GET /library/{genre}", a.handleGenre)
	mux.HandleFunc("GET /library/{genre}/{artist}", a.handleArtist)
	mux.HandleFunc("GET /library/{genre}/{artist}/{album}", a.handleAlbum)
	mux.HandleFunc("GET /cover/{genre}/{artist}/{album}", a.handleCover)
	mux.HandleFunc("GET /audio/{genre}/{artist}/{album}/{file...}", a.handleAudio)
	mux.HandleFunc("GET /api/album/{genre}/{artist}/{album}", a.handleAPIAlbum)
	mux.HandleFunc("GET /search", a.handleSearch)
	mux.HandleFunc("POST /rescan", a.handleRescan)
}
