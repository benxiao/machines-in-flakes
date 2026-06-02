package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
)

// ---- Data structs ----

type BrowsePage struct {
	ActiveTab     string
	Paths         []PathRow  // sidebar
	CurrentRoot   string     // abs path of active sidebar entry
	Dir           string
	DirName       string
	Breadcrumbs   []Breadcrumb
	Subdirs       []SubdirRow
	Files         []FileRow
	Playlists     []PlaylistRow
	PlaylistsJSON template.JS
	DirAlbumArt   string
}

type PlaylistRow struct {
	ID        int64
	Name      string
	ItemCount int
}

type PlaylistItem struct {
	ID       int64
	Path     string
	Name     string
	FileType string
}

type PlaylistState struct {
	CurrentIndex int
	PositionSec  float64
}

type PlaylistsPage struct {
	ActiveTab string
	Playlists []PlaylistRow
	Error     string
}

type PlaylistDetailPage struct {
	ActiveTab string
	ID        int64
	Name      string
	Items     []PlaylistItem
	State     PlaylistState
}

type Breadcrumb struct {
	Name    string
	Path    string
	Current bool
}

type SubdirRow struct {
	AbsPath  string
	Name     string
	AlbumArt string // abs path of cover image inside this dir, empty if none
}

type FileRow struct {
	AbsPath    string
	Filename   string
	Extension  string
	FileType   string
	SizeBytes  int64
	Size       string
	ModifiedAt string
	WatchCount int64
}

type PathsPage struct {
	ActiveTab string
	Paths     []PathRow
	Error     string
}

type RecentItem struct {
	Path        string
	Filename    string
	FileType    string
	Dir         string
	WatchCount  int64
	UpdatedAt   string
	PositionSec float64
	AlbumArt    string
}

type RecentPage struct {
	ActiveTab string
	Items     []RecentItem
}

type PathRow struct {
	ID      int64
	Path    string
	Enabled bool
}

type SettingsPage struct {
	ActiveTab string
	Paths     []PathRow
	PathError string
	Settings  TranscodeSettings
	SavedOK   bool
}

type LoginPage struct {
	Error string
	Next  string
}

type UsersPage struct {
	ActiveTab  string
	Users      []UserRow
	CurrentUID int64
	Error      string
}

type UserRow struct {
	ID        int64
	Username  string
	CreatedAt string
}

// ---- Template engine ----

var pages map[string]*template.Template

func initTemplates() {
	funcMap := template.FuncMap{
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
	add("paths", pathsTmpl)
	add("playlists", playlistsTmpl)
	add("playlist_detail", playlistDetailTmpl)
	add("users", usersTmpl)
	add("settings", settingsTmpl)
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

// ---- CSS ----

const css = `
*, *::before, *::after { box-sizing: border-box; }
body {
  margin: 0;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  font-size: 14px;
  background: #0d1117;
  color: #c9d1d9;
  line-height: 1.5;
}
a { color: #58a6ff; text-decoration: none; }
a:hover { text-decoration: underline; }
header {
  background: #161b22;
  border-bottom: 1px solid #30363d;
  padding: 12px 24px;
}
.logo { font-size: 18px; font-weight: 600; color: #f0f6fc; }
nav {
  background: #161b22;
  border-bottom: 1px solid #30363d;
  padding: 0 24px;
  display: flex;
}
nav a {
  display: inline-block;
  padding: 10px 16px;
  color: #8b949e;
  border-bottom: 2px solid transparent;
  font-size: 14px;
}
nav a:hover { color: #c9d1d9; text-decoration: none; }
nav a.active { color: #f0f6fc; border-bottom-color: #f78166; }
main { padding: 24px; max-width: 1400px; margin: 0 auto; }
h2 { font-size: 20px; font-weight: 600; margin: 0 0 4px; color: #f0f6fc; }
h3 { font-size: 16px; font-weight: 600; margin: 0 0 12px; color: #f0f6fc; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.page-header-left h2 { margin-bottom: 4px; }
.summary { color: #8b949e; font-size: 13px; }
table { width: 100%; border-collapse: collapse; }
th {
  text-align: left;
  padding: 8px 12px;
  background: #161b22;
  border-bottom: 1px solid #30363d;
  color: #8b949e;
  font-weight: 500;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  white-space: nowrap;
}
td {
  padding: 9px 12px;
  border-bottom: 1px solid #21262d;
  vertical-align: middle;
}
tr:last-child td { border-bottom: none; }
tr:hover td { background: #161b22; }
.badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
}
.badge-photo   { background: rgba(63,185,80,0.15);  color: #3fb950; border: 1px solid rgba(63,185,80,0.4); }
.badge-video   { background: rgba(88,166,255,0.15); color: #58a6ff; border: 1px solid rgba(88,166,255,0.4); }
.badge-pdf     { background: rgba(248,81,73,0.15);  color: #f85149; border: 1px solid rgba(248,81,73,0.4); }
.badge-text    { background: rgba(210,153,34,0.15); color: #d29922; border: 1px solid rgba(210,153,34,0.4); }
.badge-other   { background: rgba(139,148,158,0.15);color: #8b949e; border: 1px solid rgba(139,148,158,0.4); }
.badge-audio   { background: rgba(188,96,255,0.15); color: #bc60ff; border: 1px solid rgba(188,96,255,0.4); }
.badge-dir     { background: rgba(88,166,255,0.12); color: #58a6ff; border: 1px solid rgba(88,166,255,0.3); }
.browse-layout { display:flex; margin:-24px; min-height:calc(100vh - 108px); }
.browse-sidebar { width:220px; flex-shrink:0; border-right:1px solid #30363d; padding:0; position:relative; transition:width 0.18s; display:flex; flex-direction:column; }
.browse-sidebar.collapsed { width:28px; }
.browse-sidebar.collapsed .sidebar-paths { display:none; }
.sidebar-toggle { background:transparent; border:none; color:#8b949e; cursor:pointer; font-size:16px; line-height:1; padding:6px 4px; text-align:center; width:100%; flex-shrink:0; }
.sidebar-toggle:hover { color:#f0f6fc; }
.browse-sidebar.collapsed .sidebar-toggle { padding:8px 4px; }
.sidebar-paths { overflow-y:auto; overflow-x:hidden; flex:1; padding:4px 0; }
.browse-sidebar-item { display:block; padding:8px 16px; color:#8b949e; font-size:13px; font-family:monospace; white-space:nowrap; overflow:hidden; text-overflow:ellipsis; text-decoration:none; cursor:pointer; }
.browse-sidebar-item:hover { background:#161b22; color:#c9d1d9; text-decoration:none; }
.browse-sidebar-item.active { background:#1c2128; color:#f0f6fc; border-left:3px solid #58a6ff; padding-left:13px; }
.sidebar-short { display:none; }
.browse-main { flex:1; min-width:0; padding:16px 24px; overflow:hidden; }
.sel-spacer { height: 60px; }
.view-toggle { display:flex; gap:4px; }
.btn-view { background:transparent; border:1px solid #30363d; color:#8b949e; border-radius:6px; padding:4px 10px; font-size:13px; cursor:pointer; line-height:1.4; }
.btn-view:hover { background:#21262d; color:#c9d1d9; }
.btn-view.active { background:#21262d; border-color:#58a6ff; color:#c9d1d9; }
.view-grid { display:grid; grid-template-columns:repeat(auto-fill,minmax(110px,1fr)); gap:6px; }
.grid-card { display:flex; flex-direction:column; align-items:center; padding:10px 6px 8px; border:1px solid transparent; border-radius:6px; cursor:pointer; text-align:center; background:transparent; color:#c9d1d9; text-decoration:none; position:relative; user-select:none; }
.grid-card:hover { background:#161b22; border-color:#30363d; }
.grid-card:hover .grid-chk { opacity:1; }
.grid-thumb { width:88px; height:66px; overflow:hidden; border-radius:4px; margin-bottom:6px; background:#0d1117; display:flex; align-items:center; justify-content:center; flex-shrink:0; }
.grid-thumb img { width:100%; height:100%; object-fit:cover; display:block; }
.grid-icon { width:88px; height:66px; display:flex; align-items:center; justify-content:center; margin-bottom:6px; flex-shrink:0; }
.grid-name { font-size:12px; line-height:1.3; overflow:hidden; display:-webkit-box; -webkit-line-clamp:2; -webkit-box-orient:vertical; max-width:104px; width:100%; word-break:break-word; }
.grid-plays { position:absolute; top:6px; right:6px; font-size:10px; background:rgba(0,0,0,0.6); color:#c9d1d9; border-radius:4px; padding:1px 4px; }
.grid-chk { position:absolute; top:6px; left:6px; opacity:0; transition:opacity 0.1s; }
.grid-card.grid-checked .grid-chk { opacity:1; }
.grid-card.grid-checked { border-color:#58a6ff; background:#1c2128; }
#modal-zoom-wrap.iz-grabbing { cursor: grabbing; }
/* Zoomable image must render at natural size; the transform handles all sizing.
   Override the .modal-body img max-width/height constraints below. */
#modal-zoom-wrap img { max-width: none !important; max-height: none !important; width: auto; height: auto; }
.pl-layout { display: flex; gap: 16px; align-items: flex-start; }
.pl-sidebar { width: 32%; max-height: 80vh; overflow-y: auto; border: 1px solid #30363d; border-radius: 6px; }
.pl-player { flex: 1; min-width: 0; }
.pl-player video, .pl-player audio { width: 100%; max-height: 70vh; display: block; background: #000; }
.pl-item { display: flex; align-items: center; gap: 8px; padding: 8px 12px; border-bottom: 1px solid #21262d; cursor: pointer; }
.pl-item:last-child { border-bottom: none; }
.pl-item:hover { background: #161b22; }
.pl-item.active { background: rgba(88,166,255,0.1); border-left: 3px solid #58a6ff; padding-left: 9px; }
.pl-item-name { flex: 1; font-size: 13px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pl-controls { display: flex; gap: 10px; align-items: center; margin-top: 10px; flex-wrap: wrap; }
.pl-title { font-size: 14px; color: #c9d1d9; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; margin-bottom: 8px; min-height: 1.5em; }
.pl-badge { color: #8b949e; font-size: 12px; }
.pl-sidebar.collapsed { width: auto; min-width: 0; }
.pl-sidebar.collapsed #pl-item-list { display: none; }
.pl-sidebar.collapsed .pl-collapse-btn { transform: rotate(180deg); }
@media (max-width: 768px) {
  .pl-layout { flex-direction: column; }
  .pl-sidebar { width: 100%; max-height: 40vh; }
}
.btn {
  display: inline-block;
  padding: 6px 14px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 500;
  border: 1px solid;
  cursor: pointer;
  text-decoration: none;
  line-height: 1.4;
}
.btn-primary { background: #238636; border-color: #2ea043; color: #fff; }
.btn-primary:hover { background: #2ea043; text-decoration: none; color: #fff; }
.btn-sm { padding: 3px 10px; font-size: 12px; }
.btn-edit { background: transparent; border-color: #30363d; color: #c9d1d9; }
.btn-edit:hover { background: #21262d; text-decoration: none; color: #c9d1d9; }
.btn-danger { background: transparent; border-color: rgba(248,81,73,0.4); color: #f85149; }
.btn-danger:hover { background: rgba(248,81,73,0.1); text-decoration: none; }
.btn-cancel { background: transparent; border-color: #30363d; color: #8b949e; }
.btn-cancel:hover { background: #21262d; text-decoration: none; color: #c9d1d9; }
form.inline { display: inline; margin: 0; }
.section { margin-bottom: 40px; }
.section-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; }
.table-wrap { border: 1px solid #30363d; border-radius: 6px; overflow-x: auto; }
.form-page { max-width: 580px; }
.form-group { margin-bottom: 16px; }
label { display: block; font-size: 13px; color: #8b949e; margin-bottom: 4px; }
input[type=text], select {
  width: 100%;
  padding: 7px 10px;
  background: #0d1117;
  border: 1px solid #30363d;
  border-radius: 6px;
  color: #c9d1d9;
  font-size: 14px;
  font-family: inherit;
}
input:focus, select:focus { outline: none; border-color: #58a6ff; }
.form-actions { display: flex; gap: 8px; margin-top: 24px; align-items: center; }
.error-box {
  color: #f85149;
  font-size: 13px;
  margin-bottom: 16px;
  padding: 10px 14px;
  background: rgba(248,81,73,0.1);
  border-radius: 6px;
  border: 1px solid rgba(248,81,73,0.3);
}
.muted { color: #8b949e; }
.actions-cell { white-space: nowrap; text-align: right; }
.breadcrumb {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #8b949e;
  font-size: 14px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}
.breadcrumb a { color: #58a6ff; }
.breadcrumb .sep { color: #30363d; }
.breadcrumb .current { color: #f0f6fc; font-weight: 500; }
.file-row { cursor: pointer; }
.dir-row td { cursor: pointer; }
.root-card {
  display: block;
  padding: 16px 20px;
  border: 1px solid #30363d;
  border-radius: 8px;
  margin-bottom: 10px;
  background: #161b22;
  color: #c9d1d9;
  text-decoration: none;
}
.root-card:hover { border-color: #58a6ff; background: #1c2128; text-decoration: none; color: #c9d1d9; }
.root-card-path { font-size: 15px; color: #58a6ff; font-family: monospace; }
.root-card-meta { font-size: 12px; color: #8b949e; margin-top: 4px; }
/* Preview modal */
.modal-overlay {
  display: none;
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.88);
  z-index: 1000;
  align-items: center;
  justify-content: center;
}
.modal-overlay.open { display: flex; }
.modal-box {
  background: #161b22;
  border: 1px solid #30363d;
  border-radius: 8px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  max-width: 92vw;
  max-height: 92vh;
  cursor: default;
}
.modal-header {
  padding: 10px 16px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid #30363d;
  flex-shrink: 0;
}
.modal-title { color: #c9d1d9; font-size: 13px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 70vw; }
.modal-close { color: #8b949e; cursor: pointer; font-size: 20px; line-height: 1; padding: 0 4px; }
.modal-close:hover { color: #f0f6fc; }
.modal-body { overflow: auto; flex: 1; display: flex; align-items: center; justify-content: center; }
.modal-body img   { max-width: 90vw; max-height: 85vh; display: block; }
.modal-body video { max-width: 90vw; max-height: 85vh; display: block; }
.modal-body iframe { width: 82vw; height: 84vh; border: none; display: block; }
.modal-body pre {
  padding: 16px;
  margin: 0;
  max-width: 80vw;
  max-height: 80vh;
  overflow: auto;
  color: #c9d1d9;
  font-size: 13px;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: monospace;
}
@media (max-width: 640px) {
  main { padding: 12px; }
  header { padding: 10px 16px; }
  nav { overflow-x: auto; -webkit-overflow-scrolling: touch; padding: 0 12px; }
  nav a { white-space: nowrap; padding: 10px 10px; font-size: 13px; }
  .page-header { flex-direction: column; align-items: flex-start; gap: 10px; }
  .section-header { flex-wrap: wrap; gap: 8px; }
  .btn { min-height: 44px; padding: 10px 16px; }
  .btn-sm { min-height: 36px; padding: 6px 12px; font-size: 13px; }
  /* Hide non-essential table columns on small screens */
  table th:nth-child(4), table td:nth-child(4),
  table th:nth-child(5), table td:nth-child(5),
  table th:nth-child(6), table td:nth-child(6) { display: none; }
  /* Modal: let media fill the screen width */
  .modal-box { max-width: 100vw; max-height: 100vh; border-radius: 0; }
  .modal-body video { max-width: 100vw; max-height: 52vh; }
  .modal-body audio { width: 90vw; }
  .modal-body iframe { width: 98vw; height: 72vh; }
  .modal-body pre { max-width: 96vw; max-height: 60vh; font-size: 12px; }
  .modal-body img { max-width: 96vw; max-height: 70vh; }
  /* Grid: slightly smaller cards, 2+ per row always comfortable */
  .view-grid { grid-template-columns: repeat(auto-fill, minmax(90px, 1fr)); gap: 4px; }
  .grid-thumb, .grid-icon { width: 74px; height: 56px; }
  /* Bulk-select bar: tighter on narrow screens */
  #sel-bar { padding: 8px 12px; gap: 6px; }
  #sel-pl { padding: 8px; font-size: 14px; }
  /* View toggle: compact */
  .btn-view { padding: 4px 8px; font-size: 12px; }
  /* Prevent iOS auto-zoom on form inputs */
  input[type=text], input[type=password], select { font-size: 16px; }
  /* Browse: stack sidebar above content on mobile */
  .browse-layout { flex-direction:column; margin:-12px; min-height:0; }
  .browse-sidebar { width:100% !important; border-right:none; border-bottom:1px solid #30363d; flex-direction:row; transition:none; }
  .browse-sidebar.collapsed { width:100% !important; }
  .browse-sidebar.collapsed .sidebar-paths { display:flex; }
  .sidebar-toggle { display:none; }
  .sidebar-paths { display:flex; flex-direction:row; overflow-x:auto; overflow-y:hidden; padding:4px 8px; -webkit-overflow-scrolling:touch; flex:none; width:100%; }
  .browse-sidebar-item { flex-shrink:0; padding:8px 14px; border-left:none !important; padding-left:14px !important; font-size:13px; min-height:44px; display:flex; align-items:center; }
  .browse-sidebar-item.active { border-bottom:2px solid #58a6ff; border-left:none !important; color:#f0f6fc; background:#1c2128; }
  .sidebar-full { display:none; }
  .sidebar-short { display:inline; }
  .browse-main { padding:12px; }
  /* Settings: single column */
  .settings-grid { grid-template-columns:1fr !important; }
  /* Taller rows for tap targets */
  td { padding:11px 8px; }
}
`

// ---- Base template ----

const baseTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>File Browser</title>
<link rel="icon" type="image/svg+xml" href="/favicon.svg">
<style>` + css + `</style>
</head>
<body>
<header>
  <span class="logo">
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="#58a6ff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="vertical-align:-4px;margin-right:6px"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>File Browser
  </span>
</header>
<nav>
  <a href="/browse"     {{if eq .ActiveTab "browse"}}class="active"{{end}}>Browse</a>
  <a href="/recent"     {{if eq .ActiveTab "recent"}}class="active"{{end}}>Recent</a>
  <a href="/playlists"  {{if eq .ActiveTab "playlists"}}class="active"{{end}}>Playlists</a>
  <a href="/users"      {{if eq .ActiveTab "users"}}class="active"{{end}}>Users</a>
  <a href="/settings"   {{if eq .ActiveTab "settings"}}class="active"{{end}}>Settings</a>
  <form action="/logout" method="post" style="margin:0;margin-left:auto;display:flex;align-items:center;padding:0 4px;flex-shrink:0">
    <button type="submit" style="background:transparent;border:1px solid #30363d;color:#8b949e;padding:4px 12px;border-radius:6px;font-size:13px;cursor:pointer;line-height:1.4;white-space:nowrap">Logout</button>
  </form>
</nav>
<main>
{{template "content" .}}
</main>
<div id="preview-modal" class="modal-overlay" onclick="if(event.target===this)closePreview()">
  <div class="modal-box">
    <div class="modal-header">
      <span class="modal-title" id="modal-title"></span>
      <span class="modal-close" onclick="closePreview()">&times;</span>
    </div>
    <div class="modal-body" id="modal-body">
      <div id="modal-zoom-wrap" style="display:none;overflow:hidden;position:relative;cursor:grab;touch-action:none">
        <img id="modal-img" src="" alt="" style="position:absolute;top:0;left:0;transform-origin:0 0;user-select:none;-webkit-user-select:none;pointer-events:none">
      </div>
      <video id="modal-video" controls style="display:none;max-width:90vw;max-height:78vh"></video>
      <audio id="modal-audio" controls style="display:none;width:90vw;max-width:600px"></audio>
      <iframe id="modal-pdf" src="" style="display:none"></iframe>
      <pre id="modal-text" style="display:none"></pre>
    </div>
    <div id="modal-iz-hint" style="display:none;color:#8b949e;font-size:11px;text-align:center;padding:4px 0;flex-shrink:0;background:#0d1117;border-top:1px solid #21262d">Scroll or pinch to zoom &middot; drag to pan &middot; double-click to zoom&thinsp;/&thinsp;fit</div>
    <div id="modal-media-controls" style="display:none;justify-content:center;align-items:center;gap:12px;padding:10px;background:#0d1117;border-top:1px solid #30363d;flex-shrink:0">
      <button id="modal-seek-back" class="btn btn-edit btn-sm" onclick="seekActiveMedia(-15)">&#9664;&#9664; 15s</button>
      <button id="modal-seek-fwd" class="btn btn-edit btn-sm" onclick="seekActiveMedia(15)">15s &#9654;&#9654;</button>
      <span id="modal-resume-badge" style="color:#8b949e;font-size:12px;margin-left:8px"></span>
    </div>
  </div>
</div>
<script src="https://cdn.jsdelivr.net/npm/hls.js@1.4"></script>
<script>
var _fo = false; try { _fo = !!localStorage.getItem('fb_force_original'); } catch(e) {}
var MOBILE = !_fo && /Mobi|Android|iPhone|iPad|iPod/i.test(navigator.userAgent);
var DEFAULT_VOL = 1; try { var _dv = parseFloat(localStorage.getItem('fb_default_volume')); if (!isNaN(_dv)) DEFAULT_VOL = Math.max(0, Math.min(1, _dv)); } catch(e) {}
var modal = document.getElementById('preview-modal');
function browseDir(el) {
  window.location = '/browse?dir=' + encodeURIComponent(el.dataset.dir);
}
// onReady(videoEl) is called once the player is ready for seeking:
// for hls.js that's after MANIFEST_PARSED; for others after loadedmetadata.
function attachVideo(videoEl, hlsUrl, directUrl, onReady) {
  if (videoEl.hlsInstance) { videoEl.hlsInstance.destroy(); videoEl.hlsInstance = null; }
  if (typeof Hls !== 'undefined' && Hls.isSupported()) {
    var hls = new Hls();
    hls.on(Hls.Events.ERROR, function(event, data) {
      if (data.fatal) { hls.destroy(); videoEl.src = directUrl; videoEl.load(); }
    });
    if (onReady) hls.on(Hls.Events.MANIFEST_PARSED, function() { onReady(videoEl); });
    hls.loadSource(hlsUrl);
    hls.attachMedia(videoEl);
    videoEl.hlsInstance = hls;
  } else if (videoEl.canPlayType('application/vnd.apple.mpegurl')) {
    videoEl.src = hlsUrl; videoEl.load();
    if (onReady) videoEl.addEventListener('loadedmetadata', function() { onReady(videoEl); }, {once: true});
  } else {
    videoEl.src = directUrl; videoEl.load();
    if (onReady) videoEl.addEventListener('loadedmetadata', function() { onReady(videoEl); }, {once: true});
  }
}
function seekActiveMedia(secs) {
  var v = document.getElementById('modal-video');
  if (v && v.style.display !== 'none') { v.currentTime = Math.max(0, v.currentTime + secs); return; }
  var a = document.getElementById('modal-audio');
  if (a && a.style.display !== 'none') { a.currentTime = Math.max(0, a.currentTime + secs); }
}
function fmtTime(s) {
  s = Math.floor(s); var m = Math.floor(s / 60); s = s % 60;
  return m + ':' + (s < 10 ? '0' : '') + s;
}
function saveVideoPos(path, time, completed) {
  fetch('/video/position', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({path: path, position: time, completed: !!completed})
  });
}
function attachMediaResume(mediaEl, path, badge, countWord, onEnded) {
  mediaEl.dataset.resumePath = path;
  mediaEl._lastSave = 0;
  fetch('/video/position?path=' + encodeURIComponent(path))
    .then(function(r) { return r.json(); })
    .then(function(d) {
      if (d.position > 1) {
        var doSeek = function() { mediaEl.currentTime = d.position; };
        if (mediaEl.readyState >= 1) doSeek();
        else mediaEl.addEventListener('loadedmetadata', doSeek, {once: true});
        badge.textContent = 'Resumed from ' + fmtTime(d.position);
        if (d.watch_count > 0) badge.textContent += ' · ' + countWord + ' ' + d.watch_count + '×';
      } else if (d.watch_count > 0) {
        badge.textContent = countWord.charAt(0).toUpperCase() + countWord.slice(1) + ' ' + d.watch_count + '×';
      }
    });
  mediaEl.addEventListener('timeupdate', function onTU() {
    if (mediaEl.dataset.resumePath !== path) { mediaEl.removeEventListener('timeupdate', onTU); return; }
    var now = Date.now();
    if (now - (mediaEl._lastSave || 0) > 5000 && mediaEl.currentTime > 1) {
      mediaEl._lastSave = now;
      saveVideoPos(path, mediaEl.currentTime, false);
    }
  });
  mediaEl.addEventListener('ended', function() {
    saveVideoPos(path, 0, true);
    mediaEl.currentTime = 0;
    badge.textContent = '';
    if (onEnded) onEnded();
  }, {once: true});
}
function dirNextMedia(path) {
  if (!window.dirMediaFiles) return null;
  for (var i = 0; i < window.dirMediaFiles.length - 1; i++) {
    if (window.dirMediaFiles[i].path === path) return window.dirMediaFiles[i + 1];
  }
  return null;
}
// ---- Image zoom/pan ----
var iz = {scale:1, fitScale:1, tx:0, ty:0, dragging:false, lx:0, ly:0, lastT:null};
function izApply() {
  document.getElementById('modal-img').style.transform = 'translate('+iz.tx+'px,'+iz.ty+'px) scale('+iz.scale+')';
}
function izFit() {
  var wrap = document.getElementById('modal-zoom-wrap');
  var img  = document.getElementById('modal-img');
  iz.scale = iz.fitScale;
  iz.tx = (wrap.clientWidth  - img.naturalWidth  * iz.scale) / 2;
  iz.ty = (wrap.clientHeight - img.naturalHeight * iz.scale) / 2;
  izApply();
}
function izZoomAt(cx, cy, factor) {
  var ns = Math.min(Math.max(iz.scale * factor, iz.fitScale * 0.9), 8);
  iz.tx = cx - (cx - iz.tx) * ns / iz.scale;
  iz.ty = cy - (cy - iz.ty) * ns / iz.scale;
  iz.scale = ns;
  izApply();
}
function izInit(wrap, img) {
  iz.scale = 1; iz.tx = 0; iz.ty = 0; iz.dragging = false; iz.lastT = null;
  var doFit = function() {
    // defer one frame so the wrap has been painted and clientWidth/Height are real
    requestAnimationFrame(function() {
      iz.fitScale = Math.min(wrap.clientWidth / img.naturalWidth, wrap.clientHeight / img.naturalHeight);
      iz.fitScale = Math.min(iz.fitScale, 1);
      izFit();
    });
  };
  if (img.complete && img.naturalWidth) doFit();
  else img.addEventListener('load', doFit, {once:true});
  if (!wrap._izBound) {
    wrap._izBound = true;
    wrap.addEventListener('wheel', function(e) {
      e.preventDefault();
      var r = wrap.getBoundingClientRect();
      izZoomAt(e.clientX - r.left, e.clientY - r.top, e.deltaY < 0 ? 1.12 : 1/1.12);
    }, {passive:false});
    wrap.addEventListener('mousedown', function(e) {
      iz.dragging = true; iz.lx = e.clientX; iz.ly = e.clientY;
      wrap.classList.add('iz-grabbing');
    });
    wrap.addEventListener('dblclick', function(e) {
      var r = wrap.getBoundingClientRect();
      // Toggle between fit and 100% (actual pixels) at the click point.
      if (Math.abs(iz.scale - iz.fitScale) < 0.01) izZoomAt(e.clientX - r.left, e.clientY - r.top, 1 / iz.scale);
      else izFit();
    });
    wrap.addEventListener('touchstart', function(e) { e.preventDefault(); iz.lastT = Array.from(e.touches).map(function(t){return {clientX:t.clientX,clientY:t.clientY};}); }, {passive:false});
    wrap.addEventListener('touchmove', function(e) {
      e.preventDefault();
      var r = wrap.getBoundingClientRect(), lt = iz.lastT;
      if (e.touches.length === 2 && lt && lt.length === 2) {
        var d1 = Math.hypot(e.touches[0].clientX - e.touches[1].clientX, e.touches[0].clientY - e.touches[1].clientY);
        var d2 = Math.hypot(lt[0].clientX - lt[1].clientX, lt[0].clientY - lt[1].clientY);
        var mx = (e.touches[0].clientX + e.touches[1].clientX) / 2 - r.left;
        var my = (e.touches[0].clientY + e.touches[1].clientY) / 2 - r.top;
        if (d2 > 0) izZoomAt(mx, my, d1 / d2);
        iz.tx += mx - ((lt[0].clientX + lt[1].clientX) / 2 - r.left);
        iz.ty += my - ((lt[0].clientY + lt[1].clientY) / 2 - r.top);
        izApply();
      } else if (e.touches.length === 1 && lt && lt.length === 1) {
        iz.tx += e.touches[0].clientX - lt[0].clientX;
        iz.ty += e.touches[0].clientY - lt[0].clientY;
        izApply();
      }
      iz.lastT = Array.from(e.touches).map(function(t){return {clientX:t.clientX,clientY:t.clientY};});
    }, {passive:false});
  }
}
document.addEventListener('mousemove', function(e) {
  if (!iz.dragging) return;
  iz.tx += e.clientX - iz.lx; iz.ty += e.clientY - iz.ly;
  iz.lx = e.clientX; iz.ly = e.clientY;
  izApply();
});
document.addEventListener('mouseup', function() {
  if (!iz.dragging) return;
  iz.dragging = false;
  var w = document.getElementById('modal-zoom-wrap');
  if (w) w.classList.remove('iz-grabbing');
});
// ---- End image zoom ----

function openPreview(el, autoplay) {
  var path = el.dataset.path;
  var name = el.dataset.name;
  var type = el.dataset.type;
  var fileUrl = '/file?path=' + encodeURIComponent(path);
  document.getElementById('modal-title').textContent = name;
  var wrap  = document.getElementById('modal-zoom-wrap');
  var img   = document.getElementById('modal-img');
  var video = document.getElementById('modal-video');
  var audio = document.getElementById('modal-audio');
  var pdf   = document.getElementById('modal-pdf');
  var txt   = document.getElementById('modal-text');
  var ctrl  = document.getElementById('modal-media-controls');
  var hint  = document.getElementById('modal-iz-hint');
  var badge = document.getElementById('modal-resume-badge');
  var seekBack = document.getElementById('modal-seek-back');
  var seekFwd  = document.getElementById('modal-seek-fwd');
  wrap.style.display = video.style.display = audio.style.display = pdf.style.display = txt.style.display = 'none';
  ctrl.style.display = hint.style.display = 'none';
  badge.textContent = '';
  img.src = ''; pdf.src = '';
  document.getElementById('modal-body').style.overflow = '';
  if (video.dataset.resumePath){ if (video.hlsInstance) { video.hlsInstance.destroy(); video.hlsInstance = null; } video.src = ''; video.dataset.resumePath = ''; }
  if (audio.dataset.resumePath) { audio.pause(); audio.src = ''; audio.dataset.resumePath = ''; }
  if (type === 'photo') {
    // Give the wrap an explicit viewport size: the image is position:absolute
    // so it contributes no intrinsic size, and a flex container would collapse.
    document.getElementById('modal-body').style.overflow = 'hidden';
    img.src = fileUrl;
    wrap.style.width = '90vw';
    wrap.style.height = '82vh';
    wrap.style.display = 'block';
    hint.style.display = 'block';
    izInit(wrap, img);
  } else if (type === 'video') {
    seekBack.style.display = ''; seekFwd.style.display = '';
    var _nv = dirNextMedia(path);
    if (MOBILE) {
      attachVideo(video, '/hls/playlist?path=' + encodeURIComponent(path), fileUrl,
        autoplay ? function(v) { v.play(); } : null);
    } else {
      video.src = fileUrl;
      video.load();
      if (autoplay) video.addEventListener('canplay', function() { video.play(); }, {once: true});
    }
    video.volume = DEFAULT_VOL;
    video.style.display = 'block';
    ctrl.style.display = 'flex';
    attachMediaResume(video, path, badge, 'watched', _nv ? function() { openPreview({dataset: _nv}, true); } : null);
  } else if (type === 'audio') {
    seekBack.style.display = 'none'; seekFwd.style.display = 'none';
    audio.src = fileUrl;
    audio.load();
    audio.volume = DEFAULT_VOL;
    audio.style.display = 'block';
    ctrl.style.display = 'flex';
    if (autoplay) audio.addEventListener('canplay', function() { audio.play(); }, {once: true});
    var _na = dirNextMedia(path);
    attachMediaResume(audio, path, badge, 'listened', _na ? function() { openPreview({dataset: _na}, true); } : null);
  } else if (type === 'pdf') {
    pdf.src = fileUrl;
    pdf.style.display = 'block';
  } else if (type === 'text') {
    fetch(fileUrl).then(function(r){return r.text();}).then(function(t){
      txt.textContent = t;
      txt.style.display = 'block';
    });
  }
  modal.classList.add('open');
}
function closePreview() {
  modal.classList.remove('open');
  var video = document.getElementById('modal-video');
  var audio = document.getElementById('modal-audio');
  if (video.dataset.resumePath && video.currentTime > 1) saveVideoPos(video.dataset.resumePath, video.currentTime, false);
  if (audio.dataset.resumePath && audio.currentTime > 1) saveVideoPos(audio.dataset.resumePath, audio.currentTime, false);
  video.dataset.resumePath = ''; audio.dataset.resumePath = '';
  document.getElementById('modal-resume-badge').textContent = '';
  if (video.hlsInstance) { video.hlsInstance.destroy(); video.hlsInstance = null; }
  video.src = '';
  audio.pause(); audio.src = ''; audio.style.display = 'none';
  document.getElementById('modal-media-controls').style.display = 'none';
  // Reset image zoom
  document.getElementById('modal-zoom-wrap').style.display = 'none';
  document.getElementById('modal-img').src = '';
  document.getElementById('modal-iz-hint').style.display = 'none';
  document.getElementById('modal-body').style.overflow = '';
  iz.scale = 1; iz.tx = 0; iz.ty = 0; iz.dragging = false;
}
document.addEventListener('keydown', function(e){ if(e.key==='Escape') closePreview(); });
document.addEventListener('submit', function(e) {
  var action = e.target.getAttribute('action') || '';
  if (action.indexOf('/delete') !== -1) {
    if (!confirm('Remove this? This cannot be undone.')) e.preventDefault();
  }
});
</script>
</body>
</html>`

// ---- Page templates ----

const browseTmpl = `{{define "content"}}
<script>window.PLAYLISTS = {{.PlaylistsJSON}};</script>
<div class="browse-layout">
  <div class="browse-sidebar" id="browse-sidebar">
    <button class="sidebar-toggle" id="sidebar-toggle" onclick="toggleSidebar()" title="Collapse sidebar">&#8249;</button>
    <div class="sidebar-paths">
      {{range .Paths}}
      <a class="browse-sidebar-item{{if eq .Path $.CurrentRoot}} active{{end}}" href="{{browseURL .Path}}" title="{{.Path}}">
        <span class="sidebar-full">{{.Path}}</span><span class="sidebar-short">{{base .Path}}</span>
      </a>
      {{else}}
      <span style="padding:8px 16px;color:#8b949e;font-size:12px;display:block">No paths. Add one in <a href="/settings">Settings</a>.</span>
      {{end}}
    </div>
  </div>
  <div class="browse-main">
    {{if .Dir}}
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:12px;flex-wrap:wrap;gap:8px">
      <div class="breadcrumb" style="margin-bottom:0">
        {{range $i, $b := .Breadcrumbs}}
        {{if $i}}<span class="sep">/</span>{{end}}
        {{if $b.Current}}<span class="current">{{$b.Name}}</span>
        {{else}}<a href="{{browseURL $b.Path}}">{{$b.Name}}</a>{{end}}
        {{end}}
      </div>
      <div class="view-toggle">
        <button id="btn-list" class="btn-view" onclick="setView('list')" title="List view">&#9776; List</button>
        <button id="btn-grid" class="btn-view" onclick="setView('grid')" title="Grid view">&#8859; Grid</button>
      </div>
    </div>
    {{if and (not .Subdirs) (not .Files)}}
    <p class="muted">This directory is empty.</p>
    {{else}}
    <div id="view-list">
    <div class="table-wrap">
    <table>
    <thead><tr>
      <th style="width:32px"><input type="checkbox" id="sel-all" onchange="toggleSelectAll(this)" style="cursor:pointer"></th>
      <th>Name</th>
      <th>Type</th>
      <th>Size</th>
      <th>Modified</th>
      <th>Plays</th>
    </tr></thead>
    <tbody>
    {{range .Subdirs}}
    <tr class="dir-row" data-dir="{{.AbsPath}}" onclick="browseDir(this)">
      <td></td>
      <td>
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="#58a6ff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="vertical-align:-2px;margin-right:6px"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>{{.Name}}
      </td>
      <td><span class="badge badge-dir">DIR</span></td>
      <td class="muted">—</td>
      <td class="muted">—</td>
      <td class="muted">—</td>
    </tr>
    {{end}}
    {{range .Files}}
    {{if eq .FileType "other"}}
    <tr>
      <td></td>
      <td><a href="{{fileURL .AbsPath}}">{{.Filename}}</a></td>
      <td><span class="badge badge-{{.FileType}}">{{upper .FileType}}</span></td>
      <td class="muted">{{.Size}}</td>
      <td class="muted">{{.ModifiedAt}}</td>
      <td class="muted">—</td>
    </tr>
    {{else}}
    <tr class="file-row" data-path="{{.AbsPath}}" data-name="{{.Filename}}" data-type="{{.FileType}}" onclick="openPreview(this)">
      <td>{{if or (eq .FileType "video") (eq .FileType "audio")}}<input type="checkbox" class="row-check" value="{{.AbsPath}}" onchange="updateSelBar()" onclick="event.stopPropagation()" style="cursor:pointer">{{end}}</td>
      <td>{{.Filename}}</td>
      <td><span class="badge badge-{{.FileType}}">{{upper .FileType}}</span></td>
      <td class="muted">{{.Size}}</td>
      <td class="muted">{{.ModifiedAt}}</td>
      <td>{{if and (or (eq .FileType "video") (eq .FileType "audio")) (gt .WatchCount 0)}}<span class="badge badge-{{.FileType}}">{{.WatchCount}}×</span>{{else}}<span class="muted">—</span>{{end}}</td>
    </tr>
    {{end}}
    {{end}}
    </tbody>
    </table>
    </div>
    </div>
    <div id="view-grid" class="view-grid" style="display:none">
    {{range .Subdirs}}
    <div class="grid-card" data-dir="{{.AbsPath}}" onclick="browseDir(this)">
      {{if .AlbumArt}}
      <div class="grid-thumb" style="position:relative">
        <img src="{{thumbURL .AlbumArt}}" loading="lazy" alt="" style="width:100%;height:100%;object-fit:cover;display:block"
             onerror="this.style.display='none';this.nextElementSibling.style.display='flex'">
        <div style="display:none;width:100%;height:100%;align-items:center;justify-content:center">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="#58a6ff" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>
        </div>
        <div style="position:absolute;bottom:4px;right:4px;background:rgba(0,0,0,0.55);border-radius:3px;padding:2px 3px;line-height:0" title="Folder">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="#58a6ff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>
        </div>
      </div>
      {{else}}
      <div class="grid-icon"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="#58a6ff" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg></div>
      {{end}}
      <div class="grid-name">{{.Name}}</div>
    </div>
    {{end}}
    {{range .Files}}
    {{if eq .FileType "photo"}}
    <div class="grid-card" data-path="{{.AbsPath}}" data-name="{{.Filename}}" data-type="photo" onclick="gridClick(event,this)">
      <input class="grid-chk row-check" type="checkbox" value="{{.AbsPath}}" onchange="gridCheck(event,this)" onclick="event.stopPropagation()" style="cursor:pointer;width:14px;height:14px">
      <div class="grid-thumb">
        <img src="{{thumbURL .AbsPath}}" loading="lazy" alt="{{.Filename}}" style="width:100%;height:100%;object-fit:cover;display:block"
             onerror="this.style.display='none';this.nextElementSibling.style.display='flex'">
        <div style="display:none;width:100%;height:100%;align-items:center;justify-content:center">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="#3fb950" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><polyline points="21 15 16 10 5 21"/></svg>
        </div>
      </div>
      <div class="grid-name">{{.Filename}}</div>
    </div>
    {{else if eq .FileType "video"}}
    <div class="grid-card" data-path="{{.AbsPath}}" data-name="{{.Filename}}" data-type="video" onclick="gridClick(event,this)">
      {{if gt .WatchCount 0}}<span class="grid-plays">{{.WatchCount}}×</span>{{end}}
      <input class="grid-chk row-check" type="checkbox" value="{{.AbsPath}}" onchange="gridCheck(event,this)" onclick="event.stopPropagation()" style="cursor:pointer;width:14px;height:14px">
      <div class="grid-thumb">
        <img src="{{thumbURL .AbsPath}}" loading="lazy" alt="" style="width:100%;height:100%;object-fit:cover;display:block"
             onerror="this.style.display='none';this.nextElementSibling.style.display='flex'">
        <div style="display:none;width:100%;height:100%;align-items:center;justify-content:center">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="#58a6ff" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="2" width="20" height="20" rx="2"/><line x1="7" y1="2" x2="7" y2="22"/><line x1="17" y1="2" x2="17" y2="22"/><line x1="2" y1="12" x2="22" y2="12"/><line x1="2" y1="7" x2="7" y2="7"/><line x1="17" y1="7" x2="22" y2="7"/><line x1="17" y1="17" x2="22" y2="17"/><line x1="2" y1="17" x2="7" y2="17"/></svg>
        </div>
      </div>
      <div class="grid-name">{{.Filename}}</div>
    </div>
    {{else if eq .FileType "audio"}}
    <div class="grid-card" data-path="{{.AbsPath}}" data-name="{{.Filename}}" data-type="audio" onclick="gridClick(event,this)">
      {{if gt .WatchCount 0}}<span class="grid-plays">{{.WatchCount}}×</span>{{end}}
      <input class="grid-chk row-check" type="checkbox" value="{{.AbsPath}}" onchange="gridCheck(event,this)" onclick="event.stopPropagation()" style="cursor:pointer;width:14px;height:14px">
      {{if $.DirAlbumArt}}
      <div class="grid-thumb">
        <img src="{{thumbURL $.DirAlbumArt}}" loading="lazy" alt="" style="width:100%;height:100%;object-fit:cover;display:block"
             onerror="this.style.display='none';this.nextElementSibling.style.display='flex'">
        <div style="display:none;width:100%;height:100%;align-items:center;justify-content:center">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="#bc60ff" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg>
        </div>
      </div>
      {{else}}
      <div class="grid-icon"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="#bc60ff" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg></div>
      {{end}}
      <div class="grid-name">{{.Filename}}</div>
    </div>
    {{else if eq .FileType "pdf"}}
    <div class="grid-card" data-path="{{.AbsPath}}" data-name="{{.Filename}}" data-type="pdf" onclick="gridClick(event,this)">
      <div class="grid-icon"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="#f85149" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg></div>
      <div class="grid-name">{{.Filename}}</div>
    </div>
    {{else if eq .FileType "text"}}
    <div class="grid-card" data-path="{{.AbsPath}}" data-name="{{.Filename}}" data-type="text" onclick="gridClick(event,this)">
      <div class="grid-icon"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="#d29922" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><line x1="10" y1="9" x2="8" y2="9"/></svg></div>
      <div class="grid-name">{{.Filename}}</div>
    </div>
    {{else}}
    <a class="grid-card" href="{{fileURL .AbsPath}}">
      <div class="grid-icon"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="#8b949e" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg></div>
      <div class="grid-name">{{.Filename}}</div>
    </a>
    {{end}}
    {{end}}
    </div>
    {{end}}
    {{end}}
  </div>
</div>
<div class="sel-spacer"></div>
<div id="sel-bar" style="display:none;position:fixed;bottom:0;left:0;right:0;background:#161b22;border-top:1px solid #30363d;padding:12px 24px;z-index:200;align-items:center;gap:12px;flex-wrap:wrap">
  <span id="sel-count" style="color:#c9d1d9;font-size:14px;white-space:nowrap"></span>
  <select id="sel-pl" style="background:#0d1117;border:1px solid #30363d;border-radius:6px;color:#c9d1d9;font-size:13px;padding:5px 8px">
    <option value="">Add to playlist...</option>
    {{range .Playlists}}<option value="{{.ID}}">{{.Name}}</option>{{end}}
  </select>
  <button class="btn btn-primary btn-sm" onclick="addSelectedToPlaylist()">Add to Playlist</button>
  <button class="btn btn-edit btn-sm" onclick="downloadSelected()">⬇ Download</button>
  <button class="btn btn-edit btn-sm" onclick="clearSelection()">&#x2715; Clear</button>
  <span id="sel-ok" style="display:none;color:#3fb950;font-size:13px"></span>
</div>
<script>
function updateSelBar() {
  var checks = document.querySelectorAll('.row-check:checked');
  var bar = document.getElementById('sel-bar');
  var count = document.getElementById('sel-count');
  bar.style.display = checks.length > 0 ? 'flex' : 'none';
  count.textContent = checks.length + ' item' + (checks.length === 1 ? '' : 's') + ' selected';
  var all = document.querySelectorAll('.row-check');
  var selAll = document.getElementById('sel-all');
  if (selAll) {
    selAll.indeterminate = checks.length > 0 && checks.length < all.length;
    selAll.checked = all.length > 0 && checks.length === all.length;
  }
}
function toggleSelectAll(cb) {
  document.querySelectorAll('.row-check').forEach(function(c) { c.checked = cb.checked; });
  updateSelBar();
}
function clearSelection() {
  document.querySelectorAll('.row-check').forEach(function(c) { c.checked = false; });
  var selAll = document.getElementById('sel-all');
  if (selAll) { selAll.checked = false; selAll.indeterminate = false; }
  updateSelBar();
}
function setView(v) {
  document.getElementById('view-list').style.display = v === 'list' ? '' : 'none';
  document.getElementById('view-grid').style.display = v === 'grid' ? 'grid' : 'none';
  document.getElementById('btn-list').classList.toggle('active', v === 'list');
  document.getElementById('btn-grid').classList.toggle('active', v === 'grid');
  try { localStorage.setItem('fb_view', v); } catch(e) {}
}
function gridClick(event, el) {
  var chk = el.querySelector('.grid-chk');
  if (chk && chk.checked) { chk.checked = false; el.classList.remove('grid-checked'); updateSelBar(); return; }
  openPreview(el);
}
function gridCheck(event, chk) {
  var card = chk.closest('.grid-card');
  if (card) card.classList.toggle('grid-checked', chk.checked);
  updateSelBar();
}
(function() {
  var v = 'grid'; try { v = localStorage.getItem('fb_view') || 'grid'; } catch(e) {}
  setView(v);
})();
function addSelectedToPlaylist() {
  var plId = document.getElementById('sel-pl').value;
  if (!plId) { document.getElementById('sel-pl').focus(); return; }
  var paths = Array.from(document.querySelectorAll('.row-check:checked')).map(function(c) { return c.value; });
  if (paths.length === 0) return;
  Promise.all(paths.map(function(path) {
    return fetch('/playlists/' + plId + '/items', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({path: path})
    });
  })).then(function() {
    var ok = document.getElementById('sel-ok');
    ok.textContent = paths.length + ' item' + (paths.length === 1 ? '' : 's') + ' added';
    ok.style.display = 'inline';
    setTimeout(function() { ok.style.display = 'none'; clearSelection(); }, 1500);
  });
}
function downloadSelected() {
  var paths = Array.from(document.querySelectorAll('.row-check:checked')).map(function(c) { return c.value; });
  paths.forEach(function(path, i) {
    setTimeout(function() {
      var a = document.createElement('a');
      a.href = '/file?path=' + encodeURIComponent(path) + '&dl=1';
      a.download = '';
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
    }, i * 300);
  });
}
// Build sorted media file list for dir auto-advance
(function() {
  var seen = {}, arr = [];
  document.querySelectorAll('[data-type="audio"],[data-type="video"]').forEach(function(el) {
    var p = el.dataset.path;
    if (p && !seen[p]) {
      seen[p] = true;
      arr.push({path: p, name: el.dataset.name || '', type: el.dataset.type});
    }
  });
  arr.sort(function(a, b) { return a.name.toLowerCase().localeCompare(b.name.toLowerCase()); });
  window.dirMediaFiles = arr;
})();
function toggleSidebar() {
  var sb = document.getElementById('browse-sidebar');
  var btn = document.getElementById('sidebar-toggle');
  if (!sb) return;
  sb.classList.toggle('collapsed');
  var col = sb.classList.contains('collapsed');
  btn.innerHTML = col ? '&#8250;' : '&#8249;';
  btn.title = col ? 'Expand sidebar' : 'Collapse sidebar';
  try { localStorage.setItem('fb_sidebar', col ? '1' : ''); } catch(e) {}
}
(function() {
  try {
    if (localStorage.getItem('fb_sidebar') === '1') {
      var sb = document.getElementById('browse-sidebar');
      var btn = document.getElementById('sidebar-toggle');
      if (sb) { sb.classList.add('collapsed'); if (btn) { btn.innerHTML = '&#8250;'; btn.title = 'Expand sidebar'; } }
    }
  } catch(e) {}
})();
</script>
{{end}}`

const recentTmpl = `{{define "content"}}
<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px;flex-wrap:wrap;gap:8px">
  <div>
    <h2 style="margin:0 0 2px">Recent</h2>
    <div class="summary">Last 50 played files</div>
  </div>
  <div class="view-toggle">
    <button id="btn-list" class="btn-view" onclick="setView('list')" title="List view">&#9776; List</button>
    <button id="btn-grid" class="btn-view" onclick="setView('grid')" title="Grid view">&#8859; Grid</button>
  </div>
</div>
{{if not .Items}}
<p class="muted">Nothing played yet. Browse to a video or audio file to get started.</p>
{{else}}
<div id="view-list">
<div class="table-wrap">
<table>
<thead><tr>
  <th>Name</th>
  <th>Type</th>
  <th>Directory</th>
  <th>Last played</th>
  <th>Plays</th>
</tr></thead>
<tbody>
{{range .Items}}
<tr class="file-row" data-path="{{.Path}}" data-name="{{.Filename}}" data-type="{{.FileType}}" onclick="openPreview(this)">
  <td>{{.Filename}}</td>
  <td><span class="badge badge-{{.FileType}}">{{upper .FileType}}</span></td>
  <td class="muted" style="font-size:12px;font-family:monospace">{{.Dir}}</td>
  <td class="muted">{{.UpdatedAt}}</td>
  <td>{{if gt .WatchCount 0}}<span class="badge badge-{{.FileType}}">{{.WatchCount}}×</span>{{else}}<span class="muted">—</span>{{end}}</td>
</tr>
{{end}}
</tbody>
</table>
</div>
</div>
<div id="view-grid" class="view-grid" style="display:none">
{{range .Items}}
{{if eq .FileType "video"}}
<div class="grid-card" data-path="{{.Path}}" data-name="{{.Filename}}" data-type="video" onclick="openPreview(this)">
  {{if gt .WatchCount 0}}<span class="grid-plays">{{.WatchCount}}×</span>{{end}}
  <div class="grid-thumb">
    <img src="{{thumbURL .Path}}" loading="lazy" alt="" style="width:100%;height:100%;object-fit:cover;display:block"
         onerror="this.style.display='none';this.nextElementSibling.style.display='flex'">
    <div style="display:none;width:100%;height:100%;align-items:center;justify-content:center">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="#58a6ff" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="2" width="20" height="20" rx="2"/><line x1="7" y1="2" x2="7" y2="22"/><line x1="17" y1="2" x2="17" y2="22"/><line x1="2" y1="12" x2="22" y2="12"/><line x1="2" y1="7" x2="7" y2="7"/><line x1="17" y1="7" x2="22" y2="7"/><line x1="17" y1="17" x2="22" y2="17"/><line x1="2" y1="17" x2="7" y2="17"/></svg>
    </div>
  </div>
  <div class="grid-name">{{.Filename}}</div>
</div>
{{else if eq .FileType "audio"}}
<div class="grid-card" data-path="{{.Path}}" data-name="{{.Filename}}" data-type="audio" onclick="openPreview(this)">
  {{if gt .WatchCount 0}}<span class="grid-plays">{{.WatchCount}}×</span>{{end}}
  {{if .AlbumArt}}
  <div class="grid-thumb">
    <img src="{{thumbURL .AlbumArt}}" loading="lazy" alt="" style="width:100%;height:100%;object-fit:cover;display:block"
         onerror="this.style.display='none';this.nextElementSibling.style.display='flex'">
    <div style="display:none;width:100%;height:100%;align-items:center;justify-content:center">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="#bc60ff" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg>
    </div>
  </div>
  {{else}}
  <div class="grid-icon"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="#bc60ff" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg></div>
  {{end}}
  <div class="grid-name">{{.Filename}}</div>
</div>
{{end}}
{{end}}
</div>
{{end}}
<script>
function setView(v) {
  document.getElementById('view-list').style.display = v === 'list' ? '' : 'none';
  document.getElementById('view-grid').style.display = v === 'grid' ? 'grid' : 'none';
  document.getElementById('btn-list').classList.toggle('active', v === 'list');
  document.getElementById('btn-grid').classList.toggle('active', v === 'grid');
  try { localStorage.setItem('fb_view', v); } catch(e) {}
}
(function() {
  var v = 'list'; try { v = localStorage.getItem('fb_view') || 'list'; } catch(e) {}
  setView(v);
})();
(function() {
  var seen = {}, arr = [];
  document.querySelectorAll('[data-type="audio"],[data-type="video"]').forEach(function(el) {
    var p = el.dataset.path;
    if (p && !seen[p]) { seen[p] = true; arr.push({path: p, name: el.dataset.name || '', type: el.dataset.type}); }
  });
  window.dirMediaFiles = arr;
})();
</script>
{{end}}`


const pathsTmpl = `{{define "content"}}
<div class="page-header">
  <div class="page-header-left">
    <h2>Paths</h2>
    <div class="summary">Manage browseable directories</div>
  </div>
</div>
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
{{if .Paths}}
<div class="section">
<div class="table-wrap">
<table>
<thead><tr>
  <th>Path</th>
  <th></th>
</tr></thead>
<tbody>
{{range .Paths}}
<tr>
  <td><a href="{{browseURL .Path}}">{{.Path}}</a></td>
  <td class="actions-cell">
    <form class="inline" action="/paths/{{.ID}}/delete" method="post">
      <button class="btn btn-danger btn-sm" type="submit">Remove</button>
    </form>
  </td>
</tr>
{{end}}
</tbody>
</table>
</div>
</div>
{{end}}
<div class="section">
  <div class="section-header"><h3>Add Path</h3></div>
  <div class="form-page">
    <form action="/paths" method="post">
      <div class="form-group">
        <label>Directory path</label>
        <input type="text" name="path" placeholder="/home/rxiao/photos" autofocus>
      </div>
      <div class="form-actions">
        <button class="btn btn-primary" type="submit">Add</button>
      </div>
    </form>
  </div>
</div>
{{end}}`

const playlistsTmpl = `{{define "content"}}
<div class="page-header">
  <div class="page-header-left">
    <h2>Playlists</h2>
    <div class="summary">Manage video and audio playlists</div>
  </div>
</div>
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
{{if .Playlists}}
<div class="section">
<div class="table-wrap">
<table>
<thead><tr>
  <th>Name</th>
  <th>Items</th>
  <th></th>
</tr></thead>
<tbody>
{{range .Playlists}}
<tr>
  <td><a href="/playlists/{{.ID}}">{{.Name}}</a></td>
  <td class="muted">{{.ItemCount}}</td>
  <td class="actions-cell">
    <form class="inline" action="/playlists/{{.ID}}/delete" method="post">
      <button class="btn btn-danger btn-sm" type="submit">Delete</button>
    </form>
  </td>
</tr>
{{end}}
</tbody>
</table>
</div>
</div>
{{end}}
<div class="section">
  <div class="section-header"><h3>New Playlist</h3></div>
  <div class="form-page">
    <form action="/playlists" method="post">
      <div class="form-group">
        <label>Name</label>
        <input type="text" name="name" placeholder="My playlist" autofocus>
      </div>
      <div class="form-actions">
        <button class="btn btn-primary" type="submit">Create</button>
      </div>
    </form>
  </div>
</div>
{{end}}`

const playlistDetailTmpl = `{{define "content"}}
<script>
var PLAYLIST_ID = {{.ID}};
var PLAYLIST_ITEMS = {{toJSON .Items}};
var PLAYLIST_STATE = {{toJSON .State}};
</script>
<div class="page-header">
  <div class="page-header-left">
    <h2>{{.Name}}</h2>
  </div>
  <div>
    <a href="/playlists" class="btn btn-edit btn-sm">&#8592; Playlists</a>
  </div>
</div>
{{if not .Items}}
<p class="muted">No items yet. Browse to a video or audio file and click <strong>+</strong> to add it.</p>
{{else}}
<div class="pl-layout" id="pl-layout">
  <div class="pl-sidebar collapsed" id="pl-sidebar">
    <div style="display:flex;justify-content:space-between;align-items:center;padding:8px 12px;border-bottom:1px solid #30363d">
      <span style="font-size:12px;color:#8b949e;font-weight:500;text-transform:uppercase;letter-spacing:0.5px">Playlist</span>
      <button onclick="togglePlSidebar()" style="background:transparent;border:none;color:#8b949e;cursor:pointer;font-size:16px;line-height:1;padding:0 2px" title="Expand">&#x276F;</button>
    </div>
    <div id="pl-item-list">
    {{range $i, $it := .Items}}
    <div class="pl-item{{if eq $i $.State.CurrentIndex}} active{{end}}" onclick="startPlaylistItem({{$i}}, 0, true)">
      <span class="pl-item-name">{{$it.Name}}</span>
      <span class="badge badge-{{$it.FileType}}" style="flex-shrink:0">{{upper $it.FileType}}</span>
      <button class="btn btn-danger btn-sm" style="flex-shrink:0;padding:2px 7px" onclick="event.stopPropagation();removePlaylistItem({{$it.ID}})">&#x2715;</button>
    </div>
    {{end}}
    </div>
  </div>
  <div class="pl-player">
    <div class="pl-title" id="pl-title"></div>
    <video id="pl-video" controls style="display:none"></video>
    <audio id="pl-audio" controls style="display:none"></audio>
    <div class="pl-controls">
      <button class="btn btn-edit btn-sm" onclick="plPrev()">&#9664; Prev</button>
      <button class="btn btn-edit btn-sm" onclick="plNext()">Next &#9654;</button>
      <span class="pl-badge" id="pl-badge"></span>
    </div>
  </div>
</div>
{{end}}
<script>
var plLastSave = 0;
var plCurrentIdx = (PLAYLIST_STATE && PLAYLIST_STATE.CurrentIndex) || 0;
function getPlMedia() {
  var v = document.getElementById('pl-video'), a = document.getElementById('pl-audio');
  if (v && v.style.display !== 'none') return v;
  if (a && a.style.display !== 'none') return a;
  return null;
}
function savePlState() {
  var media = getPlMedia();
  fetch('/playlists/' + PLAYLIST_ID + '/state', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({current_index: plCurrentIdx, position_sec: media ? media.currentTime : 0})
  });
}
function plPlay(el) { var p = el.play(); if (p && p.catch) p.catch(function(){}); }
function startPlaylistItem(idx, seekTo, autoplay) {
  if (!PLAYLIST_ITEMS || idx < 0 || idx >= PLAYLIST_ITEMS.length) return;
  plCurrentIdx = idx;
  document.querySelectorAll('.pl-item').forEach(function(r, i) { r.classList.toggle('active', i === idx); });
  var rows = document.querySelectorAll('.pl-item');
  if (rows[idx]) rows[idx].scrollIntoView({block: 'nearest'});
  var item = PLAYLIST_ITEMS[idx];
  var fileUrl = '/file?path=' + encodeURIComponent(item.Path);
  var v = document.getElementById('pl-video'), a = document.getElementById('pl-audio');
  document.getElementById('pl-title').textContent = item.Name;
  document.getElementById('pl-badge').textContent = seekTo > 1 ? 'Resumed from ' + fmtTime(seekTo) : '';
  if (v.hlsInstance) { v.hlsInstance.destroy(); v.hlsInstance = null; }
  v.pause(); v.src = ''; v.style.display = 'none';
  a.pause(); a.src = ''; a.style.display = 'none';
  var media;
  var startPlayback = function(el) {
    if (seekTo > 1) {
      el.currentTime = seekTo;
      el.addEventListener('seeked', function() { if (autoplay) plPlay(el); else el.pause(); }, {once: true});
    } else if (autoplay) {
      plPlay(el);
    } else {
      el.pause();
    }
  };
  if (item.FileType === 'video') {
    v.volume = DEFAULT_VOL;
    v.style.display = 'block'; media = v;
    if (MOBILE) {
      attachVideo(v, '/hls/playlist?path=' + encodeURIComponent(item.Path), fileUrl, startPlayback);
    } else {
      v.src = fileUrl; v.load();
      v.addEventListener('loadedmetadata', function() { startPlayback(v); }, {once: true});
    }
  } else {
    a.volume = DEFAULT_VOL;
    a.style.display = 'block'; media = a;
    a.src = fileUrl; a.load();
    a.addEventListener('loadedmetadata', function() { startPlayback(a); }, {once: true});
  }
  // Advance to the next item when this one finishes. Guard so the native
  // 'ended' event and the near-end safety net below can't double-fire.
  var advanced = false;
  var plAdvance = function() {
    if (advanced) return;
    advanced = true;
    savePlState();
    startPlaylistItem(plCurrentIdx + 1, 0, true);
  };
  media.addEventListener('ended', plAdvance, {once: true});
  media.addEventListener('timeupdate', function onTU() {
    var cur = getPlMedia();
    if (!cur || cur !== media) { media.removeEventListener('timeupdate', onTU); return; }
    var now = Date.now();
    if (now - plLastSave > 5000 && cur.currentTime > 1) { plLastSave = now; savePlState(); }
    // HLS VOD sometimes doesn't fire 'ended' if declared duration slightly
    // exceeds the actual media; advance when we reach the very end.
    if (media.duration && isFinite(media.duration) && media.currentTime >= media.duration - 0.3) plAdvance();
  });
}
function plPrev() { savePlState(); startPlaylistItem(plCurrentIdx - 1, 0, true); }
function plNext() { savePlState(); startPlaylistItem(plCurrentIdx + 1, 0, true); }
function escHtml(s) { return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;'); }
function removePlaylistItem(itemId) {
  fetch('/playlists/' + PLAYLIST_ID + '/items/' + itemId + '/delete', {method: 'POST'})
    .then(function(r) {
      if (!r.ok) return;
      var removedIdx = PLAYLIST_ITEMS.findIndex(function(it) { return it.ID === itemId; });
      if (removedIdx === -1) return;
      PLAYLIST_ITEMS.splice(removedIdx, 1);
      renderPlSidebar();
      if (PLAYLIST_ITEMS.length === 0) {
        document.getElementById('pl-title').textContent = '';
        var v = document.getElementById('pl-video'), a = document.getElementById('pl-audio');
        if (v.hlsInstance) { v.hlsInstance.destroy(); v.hlsInstance = null; }
        v.pause(); v.src = ''; v.style.display = 'none';
        a.pause(); a.src = ''; a.style.display = 'none';
      } else if (removedIdx < plCurrentIdx) {
        plCurrentIdx--;
      } else if (removedIdx === plCurrentIdx) {
        startPlaylistItem(Math.min(plCurrentIdx, PLAYLIST_ITEMS.length - 1), 0, true);
      }
    });
}
function renderPlSidebar() {
  document.getElementById('pl-item-list').innerHTML = PLAYLIST_ITEMS.map(function(item, i) {
    return '<div class="pl-item' + (i === plCurrentIdx ? ' active' : '') + '" onclick="startPlaylistItem(' + i + ',0,true)">' +
      '<span class="pl-item-name">' + escHtml(item.Name) + '</span>' +
      '<span class="badge badge-' + item.FileType + '" style="flex-shrink:0">' + item.FileType.toUpperCase() + '</span>' +
      '<button class="btn btn-danger btn-sm" style="flex-shrink:0;padding:2px 7px" onclick="event.stopPropagation();removePlaylistItem(' + item.ID + ')">&#x2715;</button>' +
      '</div>';
  }).join('');
}
function togglePlSidebar() {
  var sb = document.getElementById('pl-sidebar');
  var btn = sb.querySelector('button[onclick="togglePlSidebar()"]');
  sb.classList.toggle('collapsed');
  btn.innerHTML = sb.classList.contains('collapsed') ? '&#x276F;' : '&#x276E;';
  btn.title = sb.classList.contains('collapsed') ? 'Expand' : 'Collapse';
}
window.addEventListener('beforeunload', savePlState);
// This inline script runs during body parse, BEFORE hls.js and the base
// script (attachVideo/fmtTime) load further down the page. Defer the
// initial autostart until DOMContentLoaded so those are defined.
document.addEventListener('DOMContentLoaded', function() {
  if (PLAYLIST_ITEMS && PLAYLIST_ITEMS.length > 0) {
    startPlaylistItem(Math.min((PLAYLIST_STATE && PLAYLIST_STATE.CurrentIndex) || 0, PLAYLIST_ITEMS.length - 1),
                     (PLAYLIST_STATE && PLAYLIST_STATE.PositionSec) || 0);
  }
});
</script>
{{end}}`

// loginTmpl is a standalone page (no nav bar).
const loginTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>File Browser — Login</title>
<link rel="icon" type="image/svg+xml" href="/favicon.svg">
<style>
*, *::before, *::after { box-sizing: border-box; }
body { margin: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; font-size: 14px; background: #0d1117; color: #c9d1d9; display: flex; align-items: center; justify-content: center; min-height: 100vh; }
.login-card { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 32px; width: 100%; max-width: 360px; }
.login-logo { display: flex; align-items: center; gap: 10px; font-size: 18px; font-weight: 600; color: #f0f6fc; margin-bottom: 28px; justify-content: center; }
.form-group { margin-bottom: 16px; }
label { display: block; font-size: 13px; color: #8b949e; margin-bottom: 4px; }
input[type=text], input[type=password] { width: 100%; padding: 8px 10px; background: #0d1117; border: 1px solid #30363d; border-radius: 6px; color: #c9d1d9; font-size: 14px; font-family: inherit; }
input:focus { outline: none; border-color: #58a6ff; }
.btn-primary { display: block; width: 100%; padding: 8px; background: #238636; border: 1px solid #2ea043; color: #fff; border-radius: 6px; font-size: 14px; font-weight: 500; cursor: pointer; margin-top: 20px; }
.btn-primary:hover { background: #2ea043; }
.error-box { color: #f85149; font-size: 13px; margin-bottom: 16px; padding: 10px 14px; background: rgba(248,81,73,0.1); border-radius: 6px; border: 1px solid rgba(248,81,73,0.3); }
</style>
</head>
<body>
<div class="login-card">
  <div class="login-logo">
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="#58a6ff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>
    File Browser
  </div>
  {{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
  <form action="/login" method="post">
    <input type="hidden" name="next" value="{{.Next}}">
    <div class="form-group">
      <label>Username</label>
      <input type="text" name="username" autofocus autocomplete="username">
    </div>
    <div class="form-group">
      <label>Password</label>
      <input type="password" name="password" autocomplete="current-password">
    </div>
    <button type="submit" class="btn-primary">Sign in</button>
  </form>
</div>
</body>
</html>`

const usersTmpl = `{{define "content"}}
<div class="page-header">
  <div class="page-header-left">
    <h2>Users</h2>
    <div class="summary">Manage user accounts</div>
  </div>
</div>
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
{{if .Users}}
<div class="section">
<div class="table-wrap">
<table>
<thead><tr>
  <th>Username</th>
  <th>Joined</th>
  <th></th>
</tr></thead>
<tbody>
{{range .Users}}
<tr>
  <td>{{.Username}}</td>
  <td class="muted">{{.CreatedAt}}</td>
  <td class="actions-cell">
    {{if eq .ID $.CurrentUID}}
    <span class="muted" style="font-size:12px">current</span>
    {{else}}
    <form class="inline" action="/users/{{.ID}}/delete" method="post">
      <button class="btn btn-danger btn-sm" type="submit">Delete</button>
    </form>
    {{end}}
  </td>
</tr>
{{end}}
</tbody>
</table>
</div>
</div>
{{end}}
<div class="section">
  <div class="section-header"><h3>Add User</h3></div>
  <div class="form-page">
    <form action="/users" method="post">
      <div class="form-group">
        <label>Username</label>
        <input type="text" name="username" autofocus autocomplete="off">
      </div>
      <div class="form-group">
        <label>Password</label>
        <input type="password" name="password" autocomplete="new-password">
      </div>
      <div class="form-actions">
        <button class="btn btn-primary" type="submit">Add User</button>
      </div>
    </form>
  </div>
</div>
{{end}}`

const settingsTmpl = `{{define "content"}}
<div class="page-header">
  <div class="page-header-left">
    <h2>Settings</h2>
  </div>
</div>
{{if .PathError}}<div class="error-box">{{.PathError}}</div>{{end}}
{{if .SavedOK}}<div style="color:#3fb950;font-size:13px;margin-bottom:16px;padding:10px 14px;background:rgba(63,185,80,0.1);border-radius:6px;border:1px solid rgba(63,185,80,0.3)">Transcoding settings saved.</div>{{end}}
<div class="section">
  <div class="section-header"><h3>Browseable Paths</h3></div>
  {{if .Paths}}
  <div class="table-wrap" style="margin-bottom:16px">
  <table>
  <thead><tr><th style="width:40px" title="Enabled in Browse">Active</th><th>Path</th><th></th></tr></thead>
  <tbody>
  {{range .Paths}}
  <tr>
    <td style="text-align:center">
      <input type="checkbox" class="path-enabled-check" data-id="{{.ID}}"
        {{if .Enabled}}checked{{end}} title="Enable or disable this path in Browse" style="cursor:pointer;width:16px;height:16px">
    </td>
    <td><a href="{{browseURL .Path}}">{{.Path}}</a></td>
    <td class="actions-cell">
      <form class="inline" action="/paths/{{.ID}}/delete" method="post">
        <button class="btn btn-danger btn-sm" type="submit">Remove</button>
      </form>
    </td>
  </tr>
  {{end}}
  </tbody>
  </table>
  </div>
  {{end}}
  <div class="form-page">
    <form action="/paths" method="post">
      <div class="form-group">
        <label>Add directory path</label>
        <input type="text" name="path" placeholder="/home/rxiao/videos" autocomplete="off">
      </div>
      <div class="form-actions">
        <button class="btn btn-primary" type="submit">Add Path</button>
      </div>
    </form>
  </div>
</div>
<div class="section">
  <div class="section-header"><h3>Video Transcoding</h3></div>
  <div class="form-page" style="max-width:680px">
    <form action="/settings" method="post">
      <div class="settings-grid" style="display:grid;grid-template-columns:1fr 1fr;gap:0 24px">
        <div class="form-group">
          <label>Quality (CRF) <span class="muted" style="font-weight:normal">— lower = better, 18–28 typical</span></label>
          <input type="number" name="crf" value="{{.Settings.CRF}}" min="0" max="51">
        </div>
        <div class="form-group">
          <label>Encode preset <span class="muted" style="font-weight:normal">— slower = smaller file</span></label>
          <select name="preset">
            <option value="ultrafast"{{if eq .Settings.Preset "ultrafast"}} selected{{end}}>ultrafast</option>
            <option value="superfast"{{if eq .Settings.Preset "superfast"}} selected{{end}}>superfast</option>
            <option value="veryfast"{{if eq .Settings.Preset "veryfast"}} selected{{end}}>veryfast</option>
            <option value="faster"{{if eq .Settings.Preset "faster"}} selected{{end}}>faster</option>
            <option value="fast"{{if eq .Settings.Preset "fast"}} selected{{end}}>fast</option>
            <option value="medium"{{if eq .Settings.Preset "medium"}} selected{{end}}>medium</option>
            <option value="slow"{{if eq .Settings.Preset "slow"}} selected{{end}}>slow</option>
            <option value="slower"{{if eq .Settings.Preset "slower"}} selected{{end}}>slower</option>
            <option value="veryslow"{{if eq .Settings.Preset "veryslow"}} selected{{end}}>veryslow</option>
          </select>
        </div>
        <div class="form-group">
          <label>Max width (px) <span class="muted" style="font-weight:normal">— 0 = no limit</span></label>
          <input type="number" name="max_width" value="{{.Settings.MaxWidth}}" min="0" step="2">
        </div>
        <div class="form-group">
          <label>Segment duration (s)</label>
          <input type="number" name="segment_sec" value="{{.Settings.SegmentSec}}" min="2" max="60">
        </div>
        <div class="form-group">
          <label>Video bitrate (kbps)</label>
          <input type="number" name="video_kbps" value="{{.Settings.VideoKbps}}" min="100">
        </div>
        <div class="form-group">
          <label>Audio bitrate (kbps)</label>
          <input type="number" name="audio_kbps" value="{{.Settings.AudioKbps}}" min="32">
        </div>
      </div>
      <div class="form-actions">
        <button class="btn btn-primary" type="submit">Save</button>
        <span class="muted" style="font-size:12px">Changes apply to new HLS segments immediately.</span>
      </div>
    </form>
  </div>
</div>
<div class="section">
  <div class="section-header"><h3>Playback</h3></div>
  <div class="form-page">
    <form action="/settings" method="post">
      <input type="hidden" name="save_playback" value="1">
      <div class="form-group" style="flex-direction:row;align-items:center;gap:10px;border:none;padding:0">
        <input type="checkbox" id="cb-force-original" name="force_original" value="1" style="width:auto;cursor:pointer;margin:0">
        <label for="cb-force-original" style="cursor:pointer;margin:0;font-weight:normal">Force original video on all devices</label>
      </div>
      <div class="form-group" style="flex-direction:row;align-items:center;gap:10px;border:none;padding:0;margin-top:12px">
        <label for="vol-slider" style="margin:0;white-space:nowrap">Default volume: <span id="vol-display">100</span>%</label>
        <input type="range" id="vol-slider" min="0" max="100" value="100" style="width:180px;cursor:pointer;accent-color:#58a6ff"
               oninput="document.getElementById('vol-display').textContent=this.value;document.getElementById('vol-val').value=this.value/100">
        <input type="hidden" id="vol-val" name="default_volume" value="1.0">
      </div>
      <div class="form-actions" style="margin-top:16px">
        <button class="btn btn-primary btn-sm" type="submit">Save Playback Settings</button>
      </div>
      <p class="muted" style="font-size:12px;margin:6px 0 0">Playback settings are saved to your account and synced across devices when you visit this page.</p>
    </form>
  </div>
</div>
<script>
(function(){
  try {
    var fo = {{if .Settings.ForceOriginal}}true{{else}}false{{end}};
    var dv = {{printf "%.4f" .Settings.DefaultVolume}};
    fo ? localStorage.setItem('fb_force_original','1') : localStorage.removeItem('fb_force_original');
    localStorage.setItem('fb_default_volume', String(dv));
    var cb = document.getElementById('cb-force-original');
    if (cb) cb.checked = fo;
    var sl = document.getElementById('vol-slider');
    if (sl) {
      var pct = Math.round(dv * 100);
      sl.value = pct;
      document.getElementById('vol-display').textContent = pct;
      document.getElementById('vol-val').value = dv;
    }
  } catch(e) {}
})();
document.querySelectorAll('.path-enabled-check').forEach(function(cb) {
  cb.addEventListener('change', function() {
    var fd = new FormData();
    fd.append('enabled', cb.checked ? '1' : '0');
    fetch('/paths/' + cb.dataset.id + '/toggle', {method: 'POST', body: fd})
      .then(function(r) { if (!r.ok) { cb.checked = !cb.checked; } });
  });
});
</script>
{{end}}`
