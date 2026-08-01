package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"image"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/rwcarlsen/goexif/exif"
)

// resolveMediaPath maps a possibly-virtual path to a real file that
// ServeFile/ffmpeg can read. Real paths pass through untouched; archive-
// internal paths extract into the cache. Callers keep the original path for
// DB keys, Content-Disposition and the thumbnail cache key.
// handlePhotoExif returns capture metadata for the photo viewer's info panel:
// pixel dimensions (stdlib image decoders — jpeg/png/gif only, other photo
// formats just omit them) plus EXIF camera/date/GPS fields where the file
// carries them (jpeg/tiff only; goexif returns ErrTagNotPresent otherwise,
// which every field below treats as "leave it blank", not a failure).
func (a *App) handlePhotoExif(w http.ResponseWriter, r *http.Request) {
	absPath := filepath.Clean(r.URL.Query().Get("path"))
	if !a.isAllowedPath(r, absPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if classifyExt(filepath.Ext(absPath)) != "photo" {
		http.Error(w, "not a photo", http.StatusBadRequest)
		return
	}
	servePath, err := a.resolveMediaPath(r.Context(), absPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(servePath)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(servePath)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	defer f.Close()

	out := struct {
		Width, Height int
		Make, Model   string
		DateTaken     string
		GPSLat        float64 `json:"gpsLat,omitempty"`
		GPSLon        float64 `json:"gpsLon,omitempty"`
		HasGPS        bool
		SizeBytes     int64
		ModTime       string
	}{SizeBytes: info.Size(), ModTime: info.ModTime().Local().Format("2006-01-02 15:04")}

	if x, err := exif.Decode(f); err == nil {
		if tag, err := x.Get(exif.Make); err == nil {
			out.Make, _ = tag.StringVal()
		}
		if tag, err := x.Get(exif.Model); err == nil {
			out.Model, _ = tag.StringVal()
		}
		if dt, err := x.DateTime(); err == nil {
			out.DateTaken = dt.Format("2006-01-02 15:04")
		}
		if lat, lon, err := x.LatLong(); err == nil {
			out.GPSLat, out.GPSLon, out.HasGPS = lat, lon, true
		}
	}
	if _, err := f.Seek(0, io.SeekStart); err == nil {
		if cfg, _, err := image.DecodeConfig(f); err == nil {
			out.Width, out.Height = cfg.Width, cfg.Height
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// listArchiveDir lists the immediate children of inner within archiveFile as
// rows whose AbsPaths are virtual paths under dirParam. Directories that
// exist only as prefixes of deeper entries are synthesized.
func listArchiveDir(archiveFile, inner, dirParam string) ([]SubdirRow, []FileRow, error) {
	entries, err := listArchiveEntries(archiveFile)
	if err != nil {
		return nil, nil, err
	}
	prefix := ""
	if inner != "" {
		prefix = inner + "/"
	}
	dirTimes := make(map[string]time.Time)
	var files []FileRow
	for _, e := range entries {
		name := e.Name
		if name == "." || strings.HasPrefix(name, "../") || !strings.HasPrefix(name, prefix) {
			continue
		}
		rest := name[len(prefix):]
		if rest == "" {
			continue
		}
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			// A deeper entry implies a child directory at this level.
			d := rest[:i]
			if t, seen := dirTimes[d]; !seen || e.ModTime.After(t) {
				dirTimes[d] = e.ModTime
			}
			continue
		}
		if e.IsDir {
			if _, seen := dirTimes[rest]; !seen {
				dirTimes[rest] = e.ModTime
			}
			continue
		}
		ext := path.Ext(rest)
		files = append(files, FileRow{
			AbsPath:    dirParam + "/" + rest,
			Filename:   rest,
			Extension:  strings.ToLower(ext),
			FileType:   classifyExt(ext),
			SizeBytes:  e.Size,
			Size:       formatSize(e.Size),
			ModifiedAt: e.ModTime.Format("2006-01-02 15:04"),
			ModTime:    e.ModTime,
		})
	}
	subdirs := make([]SubdirRow, 0, len(dirTimes))
	for d, t := range dirTimes {
		subdirs = append(subdirs, SubdirRow{
			AbsPath:    dirParam + "/" + d,
			Name:       d,
			ModifiedAt: t.Format("2006-01-02 15:04"),
			ModTime:    t,
		})
	}
	return subdirs, files, nil
}

func (a *App) handleBrowse(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := uid(r)

	// Always load sidebar paths (user's own or granted).
	pRows, err := a.db.Query(ctx, `
		SELECT ip.id, ip.path, ip.enabled FROM indexed_paths ip
		WHERE ip.enabled = TRUE
		  AND (ip.user_id = $1 OR EXISTS (SELECT 1 FROM path_grants pg WHERE pg.path_id = ip.id AND pg.user_id = $1))
		ORDER BY ip.path`, userID)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	var sidebarPaths []PathRow
	for pRows.Next() {
		var p PathRow
		if pRows.Scan(&p.ID, &p.Path, &p.Enabled) == nil {
			sidebarPaths = append(sidebarPaths, p)
		}
	}
	pRows.Close()

	dirParam := r.URL.Query().Get("dir")
	if dirParam == "" {
		if len(sidebarPaths) > 0 {
			target := "/browse?dir=" + url.QueryEscape(sidebarPaths[0].Path)
			if s := r.URL.Query().Get("sort"); s != "" {
				target += "&sort=" + url.QueryEscape(s)
			}
			http.Redirect(w, r, target, http.StatusFound)
			return
		}
		render(w, "browse", BrowsePage{ActiveTab: "browse", IsAdmin: isAdmin(r), Paths: sidebarPaths})
		return
	}

	dirParam = filepath.Clean(dirParam)
	if !a.isAllowedPath(r, dirParam) {
		http.NotFound(w, r)
		return
	}

	// Derive current root from sidebar paths (longest matching prefix).
	var rootPath string
	for _, p := range sidebarPaths {
		if (dirParam == p.Path || strings.HasPrefix(dirParam, p.Path+"/")) && len(p.Path) > len(rootPath) {
			rootPath = p.Path
		}
	}

	sortBy := r.URL.Query().Get("sort") // "" or "name" = alphabetical; "date" = newest first

	var subdirs []SubdirRow
	var files []FileRow
	inArchive := false
	if archiveFile, inner, ok := splitArchivePath(dirParam); ok {
		// A .zip/.rar file (or a virtual dir inside one) browses like a folder.
		inArchive = true
		subdirs, files, err = listArchiveDir(archiveFile, inner, dirParam)
		if err != nil {
			httpErr(w, err, 500)
			return
		}
	} else {
		entries, err := os.ReadDir(dirParam)
		if err != nil {
			httpErr(w, err, 500)
			return
		}
		for _, e := range entries {
			if e.Name() == trashDirName {
				continue
			}
			if e.IsDir() {
				absDir := filepath.Join(dirParam, e.Name())
				row := SubdirRow{AbsPath: absDir, Name: e.Name(), AlbumArt: findAlbumArt(absDir)}
				if info, err := e.Info(); err == nil {
					row.ModTime = info.ModTime()
					row.ModifiedAt = info.ModTime().Format("2006-01-02 15:04")
				}
				subdirs = append(subdirs, row)
				continue
			}
			info, err := e.Info()
			if err != nil {
				log.Printf("stat %s/%s: %v", dirParam, e.Name(), err)
				continue
			}
			ext := filepath.Ext(e.Name())
			absFile := filepath.Join(dirParam, e.Name())
			fileType := classifyExt(ext)
			var art string
			if fileType == "archive" {
				art = findArchiveAlbumArt(absFile)
			}
			files = append(files, FileRow{
				AbsPath:    absFile,
				Filename:   e.Name(),
				Extension:  strings.ToLower(ext),
				FileType:   fileType,
				SizeBytes:  info.Size(),
				Size:       formatSize(info.Size()),
				ModifiedAt: info.ModTime().Format("2006-01-02 15:04"),
				ModTime:    info.ModTime(),
				AlbumArt:   art,
			})
		}
	}
	if sortBy == "date" {
		sort.Slice(subdirs, func(i, j int) bool { return subdirs[i].ModTime.After(subdirs[j].ModTime) })
		sort.Slice(files, func(i, j int) bool { return files[i].ModTime.After(files[j].ModTime) })
	} else {
		sort.Slice(subdirs, func(i, j int) bool { return subdirs[i].Name < subdirs[j].Name })
		sort.Slice(files, func(i, j int) bool {
			return strings.ToLower(files[i].Filename) < strings.ToLower(files[j].Filename)
		})
	}

	// Batch-fetch watch counts for video and audio files.
	var mediaPaths []string
	for _, f := range files {
		if f.FileType == "video" || f.FileType == "audio" {
			mediaPaths = append(mediaPaths, f.AbsPath)
		}
	}
	if len(mediaPaths) > 0 {
		wcRows, err := a.db.Query(ctx, `SELECT path, watch_count FROM video_positions WHERE user_id = $1 AND path = ANY($2)`, userID, mediaPaths)
		if err == nil {
			wc := make(map[string]int64)
			for wcRows.Next() {
				var p string
				var c int64
				if wcRows.Scan(&p, &c) == nil {
					wc[p] = c
				}
			}
			wcRows.Close()
			for i := range files {
				if files[i].FileType == "video" || files[i].FileType == "audio" {
					files[i].WatchCount = wc[files[i].AbsPath]
				}
			}
		}
	}

	albumArt := findAlbumArt(dirParam)

	plRows, _ := a.db.Query(ctx, `SELECT id, name FROM playlists WHERE user_id = $1 ORDER BY name`, userID)
	var pls []PlaylistRow
	if plRows != nil {
		for plRows.Next() {
			var pl PlaylistRow
			if plRows.Scan(&pl.ID, &pl.Name) == nil {
				pls = append(pls, pl)
			}
		}
		plRows.Close()
	}
	plJSON, _ := json.Marshal(pls)

	render(w, "browse", BrowsePage{
		ActiveTab:     "browse",
		IsAdmin:       isAdmin(r),
		Paths:         sidebarPaths,
		CurrentRoot:   rootPath,
		Dir:           dirParam,
		DirName:       filepath.Base(dirParam),
		Breadcrumbs:   buildBreadcrumb(dirParam, rootPath),
		Subdirs:       subdirs,
		Files:         files,
		Playlists:     pls,
		PlaylistsJSON: template.JS(plJSON),
		DirAlbumArt:   albumArt,
		SortBy:        sortBy,
		InArchive:     inArchive,
	})
}


func findAlbumArt(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var first string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !photoExts[ext] {
			continue
		}
		absPath := filepath.Join(dir, e.Name())
		base := strings.ToLower(strings.TrimSuffix(e.Name(), ext))
		if base == "cover" || base == "folder" || base == "album" || base == "front" {
			return absPath
		}
		if first == "" {
			first = absPath
		}
	}
	return first
}

func buildBreadcrumb(dir, root string) []Breadcrumb {
	crumbs := []Breadcrumb{{Name: filepath.Base(root), Path: root}}
	if dir == root {
		crumbs[0].Current = true
		return crumbs
	}
	rel, _ := filepath.Rel(root, dir)
	cur := root
	parts := strings.Split(rel, string(filepath.Separator))
	for i, part := range parts {
		cur = filepath.Join(cur, part)
		crumbs = append(crumbs, Breadcrumb{
			Name:    part,
			Path:    cur,
			Current: i == len(parts)-1,
		})
	}
	return crumbs
}

func (a *App) handleServeFile(w http.ResponseWriter, r *http.Request) {
	rawPath := r.URL.Query().Get("path")
	if rawPath == "" {
		http.NotFound(w, r)
		return
	}
	absPath := filepath.Clean(rawPath)
	if !a.isAllowedPath(r, absPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	servePath, err := a.resolveMediaPath(r.Context(), absPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(servePath)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	filename := filepath.Base(absPath)
	if r.URL.Query().Get("dl") == "1" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		http.ServeFile(w, r, servePath)
		return
	}
	fileType := classifyExt(filepath.Ext(filename))
	switch fileType {
	case "photo", "pdf", "video", "text":
		w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, filename))
	case "audio":
		w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, filename))
		w.Header().Set("Cache-Control", "private, max-age=3600")
	default:
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	}
	http.ServeFile(w, r, servePath)
}
