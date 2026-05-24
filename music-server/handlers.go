package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ── Page data structs ─────────────────────────────────────────────────────────

type LibraryPage struct {
	Title  string
	Query  string
	Genres []*Genre
}

type GenrePage struct {
	Title string
	Query string
	Genre *Genre
}

type ArtistPage struct {
	Title  string
	Query  string
	Artist *Artist
	Albums []AlbumCard
}

type AlbumCard struct {
	Title      string
	PageURL    template.URL
	CoverURL   template.URL
	TrackCount int
	HasCover   bool
}

type AlbumPage struct {
	Title   string
	Query   string
	Album   *Album
	APIPath string // used in JS onclick — safe because it's a server-generated path
	GenreURL  template.URL
	ArtistURL template.URL
}

type SearchPage struct {
	Title   string
	Query   string
	Artists []ArtistResult
	Albums  []AlbumResult
}

type ArtistResult struct {
	Name       string
	GenreName  string
	URL        template.URL
	AlbumCount int
}

type AlbumResult struct {
	Title      string
	ArtistName string
	URL        template.URL
	CoverURL   template.URL
	HasCover   bool
}

// ── URL helpers ───────────────────────────────────────────────────────────────

func albumPageURL(g, ar, al string) template.URL {
	return template.URL("/library/" + pe(g) + "/" + pe(ar) + "/" + pe(al))
}
func coverURL(g, ar, al string) template.URL {
	return template.URL("/cover/" + pe(g) + "/" + pe(ar) + "/" + pe(al))
}
func apiAlbumURL(g, ar, al string) string {
	return "/api/album/" + pe(g) + "/" + pe(ar) + "/" + pe(al)
}
func pe(s string) string { return url.PathEscape(s) }

func encodeFilePath(relPath string) string {
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (a *App) handleRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/library", http.StatusSeeOther)
}

func (a *App) handleLibrary(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	lib := a.lib
	a.mu.RUnlock()
	render(w, libraryTpl, &LibraryPage{Title: "Library", Genres: lib.Genres})
}

func (a *App) handleGenre(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	lib := a.lib
	a.mu.RUnlock()
	g := lib.findGenre(r.PathValue("genre"))
	if g == nil {
		http.NotFound(w, r)
		return
	}
	render(w, genreTpl, &GenrePage{Title: g.Name, Genre: g})
}

func (a *App) handleArtist(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	lib := a.lib
	a.mu.RUnlock()
	ar := lib.findArtist(r.PathValue("genre"), r.PathValue("artist"))
	if ar == nil {
		http.NotFound(w, r)
		return
	}
	cards := make([]AlbumCard, len(ar.Albums))
	for i, alb := range ar.Albums {
		cards[i] = AlbumCard{
			Title:      alb.Title,
			PageURL:    albumPageURL(ar.Genre.Name, ar.Name, alb.Title),
			CoverURL:   coverURL(ar.Genre.Name, ar.Name, alb.Title),
			TrackCount: len(alb.Tracks),
			HasCover:   alb.CoverArt != "",
		}
	}
	render(w, artistTpl, &ArtistPage{
		Title:  ar.Name,
		Artist: ar,
		Albums: cards,
	})
}

func (a *App) handleAlbum(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	lib := a.lib
	a.mu.RUnlock()
	g, ar, al := r.PathValue("genre"), r.PathValue("artist"), r.PathValue("album")
	alb := lib.findAlbum(g, ar, al)
	if alb == nil {
		http.NotFound(w, r)
		return
	}
	render(w, albumTpl, &AlbumPage{
		Title:     alb.Title,
		Album:     alb,
		APIPath:   apiAlbumURL(g, ar, al),
		GenreURL:  template.URL("/library/" + pe(g)),
		ArtistURL: template.URL("/library/" + pe(g) + "/" + pe(ar)),
	})
}

func (a *App) handleCover(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	lib := a.lib
	a.mu.RUnlock()
	alb := lib.findAlbum(r.PathValue("genre"), r.PathValue("artist"), r.PathValue("album"))
	if alb == nil || alb.CoverArt == "" {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		fmt.Fprint(w, placeholderSVG)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, filepath.Join(alb.Path, alb.CoverArt))
}

func (a *App) handleAudio(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	lib := a.lib
	a.mu.RUnlock()
	alb := lib.findAlbum(r.PathValue("genre"), r.PathValue("artist"), r.PathValue("album"))
	if alb == nil {
		http.NotFound(w, r)
		return
	}
	file := r.PathValue("file")
	absPath := filepath.Join(alb.Path, filepath.FromSlash(file))
	// Security: ensure resolved path stays within album dir
	rel, err := filepath.Rel(alb.Path, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	f, err := os.Open(absPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		http.Error(w, "stat error", http.StatusInternalServerError)
		return
	}
	// http.ServeContent handles Range, ETag, and MIME type automatically
	http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
}

// ── JSON API ──────────────────────────────────────────────────────────────────

type trackJSON struct {
	Num    int    `json:"num"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	Format string `json:"format"`
}

type albumJSON struct {
	Genre  string      `json:"genre"`
	Artist string      `json:"artist"`
	Album  string      `json:"album"`
	Cover  string      `json:"cover"`
	Tracks []trackJSON `json:"tracks"`
}

func (a *App) handleAPIAlbum(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	lib := a.lib
	a.mu.RUnlock()
	g, ar, al := r.PathValue("genre"), r.PathValue("artist"), r.PathValue("album")
	alb := lib.findAlbum(g, ar, al)
	if alb == nil {
		http.NotFound(w, r)
		return
	}
	resp := albumJSON{
		Genre:  g,
		Artist: ar,
		Album:  al,
		Cover:  string(coverURL(g, ar, al)),
	}
	audioBase := "/audio/" + pe(g) + "/" + pe(ar) + "/" + pe(al) + "/"
	for _, t := range alb.Tracks {
		resp.Tracks = append(resp.Tracks, trackJSON{
			Num:    t.Number,
			Title:  t.Title,
			URL:    audioBase + encodeFilePath(t.Filename),
			Format: t.Format,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ── Search ────────────────────────────────────────────────────────────────────

func (a *App) handleSearch(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	lib := a.lib
	a.mu.RUnlock()
	q := r.URL.Query().Get("q")
	if strings.TrimSpace(q) == "" {
		http.Redirect(w, r, "/library", http.StatusSeeOther)
		return
	}
	rawArtists, rawAlbums := lib.search(q)
	var artists []ArtistResult
	for _, ar := range rawArtists {
		artists = append(artists, ArtistResult{
			Name:       ar.Name,
			GenreName:  ar.Genre.Name,
			URL:        template.URL("/library/" + pe(ar.Genre.Name) + "/" + pe(ar.Name)),
			AlbumCount: len(ar.Albums),
		})
	}
	var albums []AlbumResult
	for _, alb := range rawAlbums {
		albums = append(albums, AlbumResult{
			Title:      alb.Title,
			ArtistName: alb.Artist.Name,
			URL:        albumPageURL(alb.Artist.Genre.Name, alb.Artist.Name, alb.Title),
			CoverURL:   coverURL(alb.Artist.Genre.Name, alb.Artist.Name, alb.Title),
			HasCover:   alb.CoverArt != "",
		})
	}
	render(w, searchTpl, &SearchPage{
		Title:   "Search: " + q,
		Query:   q,
		Artists: artists,
		Albums:  albums,
	})
}

// ── Rescan ────────────────────────────────────────────────────────────────────

func (a *App) handleRescan(w http.ResponseWriter, r *http.Request) {
	lib, err := scanLibrary(a.musicDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.mu.Lock()
	a.lib = lib
	a.mu.Unlock()
	fmt.Fprintf(w, "OK: %d tracks in %d albums\n", lib.trackCount(), lib.albumCount())
}

// ── Render helper ─────────────────────────────────────────────────────────────

func render(w http.ResponseWriter, tpl *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tpl.Execute(w, data); err != nil {
		// Headers already sent; log only
		_ = err
	}
}

const placeholderSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">` +
	`<rect width="100" height="100" fill="#21262d"/>` +
	`<text x="50" y="68" text-anchor="middle" font-size="52" fill="#8b949e">♪</text>` +
	`</svg>`
