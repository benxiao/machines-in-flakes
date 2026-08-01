package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

func fmtDurStr(sec int64) string {
	h := sec / 3600
	m := (sec % 3600) / 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%ds", sec)
}

// ---- Template engine ----

var pages map[string]*template.Template

func initTemplates() {
	funcMap := template.FuncMap{
		"appVersion": func() string { return appVersion },
		"upper": strings.ToUpper,
		"browseURL": func(path string) template.URL {
			return template.URL("/browse?dir=" + url.QueryEscape(path))
		},
		"fileURL": func(path string) template.URL {
			return template.URL("/file?path=" + url.QueryEscape(path))
		},
		"toJSON": func(v any) template.JS {
			b, _ := json.Marshal(v)
			return template.JS(b)
		},
		"thumbURL": func(path string) template.URL {
			return template.URL("/thumbnail?path=" + url.QueryEscape(path))
		},
		"downloadURL": func(path string) template.URL {
			return template.URL("/file?path=" + url.QueryEscape(path) + "&dl=1")
		},
		"base": func(p string) string {
			if i := strings.LastIndex(p, "/"); i >= 0 {
				return p[i+1:]
			}
			return p
		},
		"dirOf": filepath.Dir,
		"playURL": func(path string) template.URL {
			return template.URL("/folder/play?file=" + url.QueryEscape(path))
		},
		"googleLoginURL": func(next string) template.URL {
			if next == "" {
				return template.URL("/auth/google/login")
			}
			return template.URL("/auth/google/login?next=" + url.QueryEscape(next))
		},
		"fmtDur": fmtDurStr,
		"fmtPos": func(sec float64) string { return fmtDurStr(int64(sec)) },
	}
	base := template.Must(template.New("base").Funcs(funcMap).Parse(baseTmpl))
	add := func(name, content string) {
		t := template.Must(base.Clone())
		template.Must(t.New("content").Parse(content))
		if pages == nil {
			pages = make(map[string]*template.Template)
		}
		pages[name] = t
	}
	add("browse", browseTmpl)
	add("recent", recentTmpl)

	add("stats", statsTmpl)
	add("folder-play", folderPlayTmpl)
	add("favorites", favoritesTmpl)
	add("paths", pathsTmpl)
	add("playlists", playlistsTmpl)
	add("playlist_detail", playlistDetailTmpl)
	add("users", usersTmpl)
	add("user_detail", userDetailTmpl)
	add("settings", settingsTmpl)
	add("trash", trashTmpl)
	add("duplicates", duplicatesTmpl)
	// login uses its own standalone template (no nav/base)
	pages["login"] = template.Must(template.New("login").Funcs(funcMap).Parse(loginTmpl))
}

func render(w http.ResponseWriter, name string, data any) {
	t, ok := pages[name]
	if !ok {
		http.Error(w, "template not found: "+name, 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		log.Printf("render %s: %v", name, err)
	}
}
