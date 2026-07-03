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
	"time"
)

// ---- Data structs ----

type BrowsePage struct {
	ActiveTab     string
	IsAdmin       bool
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
	SortBy        string
	InZip         bool // browsing inside a zip archive: read-only, no upload/mutations
}

type PlaylistRow struct {
	ID        int64
	Name      string
	ItemCount int
}

type PlaylistItem struct {
	ID         int64
	Path       string
	Name       string
	FileType   string
	WatchCount int64
}

type FavoriteItem struct {
	Path       string
	Name       string
	Dir        string
	IsFolder   bool
	StartIdx   int
	EndIdx     int
	TrackCount int
}

type FolderPlayPage struct {
	ActiveTab string
	IsAdmin   bool
	Folder    string
	Dir       string
	StartIdx  int
	Items     []PlaylistItem
}

type PlaylistState struct {
	CurrentIndex int
	PositionSec  float64
}

type PlaylistsPage struct {
	ActiveTab string
	IsAdmin   bool
	Playlists []PlaylistRow
	Error     string
}

type PlaylistDetailPage struct {
	ActiveTab string
	IsAdmin   bool
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
	AbsPath    string
	Name       string
	AlbumArt   string // abs path of cover image inside this dir, empty if none
	ModifiedAt string
	ModTime    time.Time
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
	ModTime    time.Time
	AlbumArt   string // for archives: virtual path of a cover image inside the zip, empty if none
}

type GrantedUserRow struct {
	UserID   int64
	Username string
}

type AdminPathRow struct {
	ID      int64
	Path    string
	Granted bool
}

type UserDetailPage struct {
	ActiveTab string
	IsAdmin   bool
	ID        int64
	Username  string
	AllPaths  []AdminPathRow
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
	IsAdmin   bool
	Items     []RecentItem
}

type StatDay struct {
	Label    string // e.g. "02 Jan", used in hover title
	Tick     string // axis label, set on every 5th day only
	VideoSec int64
	AudioSec int64
	VideoPct int // bar segment height as % of the tallest day
	AudioPct int
}

type StatsTotals struct {
	TodayVideo, TodayAudio int64
	WeekVideo, WeekAudio   int64
	MonthVideo, MonthAudio int64
	AllVideo, AllAudio     int64
}

type StatsPage struct {
	ActiveTab  string
	IsAdmin    bool
	Days       []StatDay
	HasPlay    bool // any nonzero day in the chart window
	Totals     StatsTotals
	TopItems   []PlaylistItem
	RecentDone []RecentItem
}

type FavoritesPage struct {
	ActiveTab string
	IsAdmin   bool
	Items     []FavoriteItem
	Tracks    []PlaylistItem
}

type PathRow struct {
	ID           int64
	Path         string
	Enabled      bool
	SizeGB       float64
	GrantedUsers []GrantedUserRow
}

type SettingsPage struct {
	ActiveTab string
	IsAdmin   bool
	Paths     []PathRow
	PathError string
}

type LoginPage struct {
	Error string
	Next  string
}

type UsersPage struct {
	ActiveTab  string
	IsAdmin    bool
	Users      []UserRow
	CurrentUID int64
	Error      string
}

type UserRow struct {
	ID        int64
	Username  string
	CreatedAt string
}

type TrashPage struct {
	ActiveTab string
	IsAdmin   bool
	Items     []TrashItemRow
	Error     string
}

type TrashItemRow struct {
	ID           int64
	Name         string
	OriginalPath string
	IsFolder     bool
	DeletedAt    string
}

type DuplicatesPage struct {
	ActiveTab string
	IsAdmin   bool
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
		"fmtDur": func(sec int64) string {
			h := sec / 3600
			m := (sec % 3600) / 60
			if h > 0 {
				return fmt.Sprintf("%dh %dm", h, m)
			}
			if m > 0 {
				return fmt.Sprintf("%dm", m)
			}
			return fmt.Sprintf("%ds", sec)
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

// ---- CSS ----

// themeVars is shared by the main app stylesheet and the standalone login
// page, which doesn't otherwise share a <style> block with it.
const themeVars = `
:root {
  --bg: #0d1117;
  --bg-panel: #161b22;
  --border: #30363d;
  --fg: #c9d1d9;
  --fg-muted: #8b949e;
  --fg-strong: #f0f6fc;
  --surface-hover: #21262d;
  --surface-active: #1c2128;
}
[data-theme="light"] {
  --bg: #ffffff;
  --bg-panel: #f6f8fa;
  --border: #d0d7de;
  --fg: #1f2328;
  --fg-muted: #59636e;
  --fg-strong: #101828;
  --surface-hover: #f3f4f6;
  --surface-active: #eaeef2;
}
`

const css = themeVars + `
*, *::before, *::after { box-sizing: border-box; }
body {
  margin: 0;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  font-size: 14px;
  background: var(--bg);
  color: var(--fg);
  line-height: 1.5;
}
a { color: #58a6ff; text-decoration: none; }
a:hover { text-decoration: underline; }
header {
  background: var(--bg-panel);
  border-bottom: 1px solid var(--border);
  padding: 12px 24px;
}
.logo { font-size: 18px; font-weight: 600; color: var(--fg-strong); }
nav {
  background: var(--bg-panel);
  border-bottom: 1px solid var(--border);
  padding: 0 24px;
  display: flex;
}
nav a {
  display: inline-block;
  padding: 10px 16px;
  color: var(--fg-muted);
  border-bottom: 2px solid transparent;
  font-size: 14px;
}
nav a:hover { color: var(--fg); text-decoration: none; }
nav a.active { color: var(--fg-strong); border-bottom-color: #f78166; }
main { padding: 24px; max-width: 1400px; margin: 0 auto; }
h2 { font-size: 20px; font-weight: 600; margin: 0 0 4px; color: var(--fg-strong); }
h3 { font-size: 16px; font-weight: 600; margin: 0 0 12px; color: var(--fg-strong); }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.page-header-left h2 { margin-bottom: 4px; }
.summary { color: var(--fg-muted); font-size: 13px; }
table { width: 100%; border-collapse: collapse; }
th {
  text-align: left;
  padding: 8px 12px;
  background: var(--bg-panel);
  border-bottom: 1px solid var(--border);
  color: var(--fg-muted);
  font-weight: 500;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  white-space: nowrap;
}
td {
  padding: 9px 12px;
  border-bottom: 1px solid var(--surface-hover);
  vertical-align: middle;
}
tr:last-child td { border-bottom: none; }
tr:hover td { background: var(--bg-panel); }
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
.badge-other   { background: rgba(139,148,158,0.15);color: var(--fg-muted); border: 1px solid rgba(139,148,158,0.4); }
.badge-audio   { background: rgba(188,96,255,0.15); color: #bc60ff; border: 1px solid rgba(188,96,255,0.4); }
.badge-dir     { background: rgba(88,166,255,0.12); color: #58a6ff; border: 1px solid rgba(88,166,255,0.3); }
.badge-archive { background: rgba(219,109,40,0.15); color: #db6d28; border: 1px solid rgba(219,109,40,0.4); }
.browse-layout { display:flex; margin:-24px; min-height:calc(100vh - 108px); }
.browse-sidebar { width:220px; flex-shrink:0; border-right:1px solid var(--border); padding:0; position:relative; transition:width 0.18s; display:flex; flex-direction:column; }
.browse-sidebar.collapsed { width:28px; }
.browse-sidebar.collapsed .sidebar-paths { display:none; }
.sidebar-toggle { background:transparent; border:none; color:var(--fg-muted); cursor:pointer; font-size:16px; line-height:1; padding:6px 4px; text-align:center; width:100%; flex-shrink:0; }
.sidebar-toggle:hover { color:var(--fg-strong); }
.browse-sidebar.collapsed .sidebar-toggle { padding:8px 4px; }
.sidebar-paths { overflow-y:auto; overflow-x:hidden; flex:1; padding:4px 0; }
.browse-sidebar-item { display:block; padding:8px 16px; color:var(--fg-muted); font-size:13px; font-family:monospace; white-space:nowrap; overflow:hidden; text-overflow:ellipsis; text-decoration:none; cursor:pointer; }
.browse-sidebar-item:hover { background:var(--bg-panel); color:var(--fg); text-decoration:none; }
.browse-sidebar-item.active { background:var(--surface-active); color:var(--fg-strong); border-left:3px solid #58a6ff; padding-left:13px; }
.browse-main { flex:1; min-width:0; padding:16px 24px; overflow:hidden; }
.header-search { position:relative; flex:1; max-width:400px; }
#search-q { width:100%; padding:6px 14px; background:var(--bg); border:1px solid var(--border); border-radius:20px; color:var(--fg); font-size:14px; font-family:inherit; }
#search-q:focus { outline:none; border-color:#58a6ff; }
.search-panel { position:absolute; top:calc(100% + 6px); left:0; right:0; background:var(--bg-panel); border:1px solid var(--border); border-radius:8px; z-index:500; max-height:70vh; overflow-y:auto; box-shadow:0 8px 24px rgba(0,0,0,0.5); }
.search-filters { display:flex; gap:6px; padding:10px 12px; border-bottom:1px solid var(--surface-hover); flex-wrap:wrap; }
.sf-chip { background:transparent; border:1px solid var(--border); color:var(--fg-muted); border-radius:12px; padding:3px 10px; font-size:12px; cursor:pointer; }
.sf-chip.active { background:var(--surface-active); border-color:#58a6ff; color:var(--fg-strong); }
.search-result-group { border-bottom:1px solid var(--surface-hover); }
.search-result-group:last-child { border-bottom:none; }
.search-result { display:flex; gap:10px; padding:10px 12px; cursor:pointer; align-items:center; }
.search-result:hover { background:var(--surface-active); }
.search-result-thumb { width:56px; height:42px; flex-shrink:0; border-radius:4px; overflow:hidden; background:var(--bg); display:flex; align-items:center; justify-content:center; }
.search-result-thumb img { width:100%; height:100%; object-fit:cover; display:block; }
.search-result-name { font-size:13px; color:var(--fg); margin-bottom:2px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.search-result-dir { font-size:11px; font-family:monospace; color:var(--fg-muted); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.sr-expand { background:transparent; border:none; color:var(--fg-muted); cursor:pointer; font-size:14px; padding:6px 8px; flex-shrink:0; border-radius:6px; }
.sr-expand:hover { background:var(--surface-hover); color:var(--fg); }
.sr-file { display:flex; gap:8px; align-items:center; padding:6px 12px 6px 32px; cursor:pointer; font-size:12px; color:var(--fg); }
.sr-file:hover { background:var(--surface-active); }
.sr-file-name { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; min-width:0; }
.sr-more { padding:6px 12px 8px 32px; font-size:11px; color:var(--fg-muted); cursor:pointer; }
.sr-more:hover { color:#58a6ff; }
.sel-spacer { height: 60px; }
.view-toggle { display:flex; gap:4px; }
.btn-view { background:transparent; border:1px solid var(--border); color:var(--fg-muted); border-radius:6px; padding:4px 10px; font-size:13px; cursor:pointer; line-height:1.4; }
.btn-view:hover { background:var(--surface-hover); color:var(--fg); }
.btn-view.active { background:var(--surface-hover); border-color:#58a6ff; color:var(--fg); }
.view-grid { display:grid; grid-template-columns:repeat(auto-fill,minmax(110px,1fr)); gap:6px; }
.grid-card { display:flex; flex-direction:column; align-items:center; padding:10px 6px 8px; border:1px solid transparent; border-radius:6px; cursor:pointer; text-align:center; background:transparent; color:var(--fg); text-decoration:none; position:relative; user-select:none; }
.grid-card:hover { background:var(--bg-panel); border-color:var(--border); }
.grid-card:hover .grid-chk { opacity:1; }
.grid-thumb { width:88px; height:66px; overflow:hidden; border-radius:4px; margin-bottom:6px; background:var(--bg); display:flex; align-items:center; justify-content:center; flex-shrink:0; }
.grid-thumb img { width:100%; height:100%; object-fit:cover; display:block; }
.grid-icon { width:88px; height:66px; display:flex; align-items:center; justify-content:center; margin-bottom:6px; flex-shrink:0; }
.grid-name { font-size:12px; line-height:1.3; overflow:hidden; display:-webkit-box; -webkit-line-clamp:2; -webkit-box-orient:vertical; max-width:104px; width:100%; word-break:break-word; }
.grid-plays { position:absolute; top:6px; right:6px; font-size:10px; background:rgba(0,0,0,0.6); color:#fff; border-radius:4px; padding:1px 4px; }
.grid-chk { position:absolute; top:6px; left:6px; opacity:0; transition:opacity 0.1s; }
.grid-card.grid-checked .grid-chk { opacity:1; }
.grid-card.grid-checked { border-color:#58a6ff; background:var(--surface-active); }
#ext-menu { position:fixed; z-index:300; background:var(--bg-panel); border:1px solid var(--border); border-radius:6px; padding:4px; min-width:170px; max-height:60vh; overflow-y:auto; box-shadow:0 8px 24px rgba(0,0,0,0.4); }
.ext-menu-item { display:flex; align-items:center; gap:8px; padding:6px 10px; border-radius:4px; cursor:pointer; font-size:13px; color:var(--fg); white-space:nowrap; }
.ext-menu-item:hover { background:var(--surface-hover); }
.ext-menu-item .mark { color:#58a6ff; font-size:11px; width:12px; flex-shrink:0; }
.ext-menu-item .cnt { color:var(--fg-muted); font-size:12px; }
.ext-menu-sep { border-top:1px solid var(--border); margin:4px 0; }
#btn-select.filter-on { color:#58a6ff; border-color:#58a6ff; }
#modal-zoom-wrap.iz-grabbing { cursor: grabbing; }
/* Zoomable image must render at natural size; the transform handles all sizing.
   Override the .modal-body img max-width/height constraints below. */
#modal-zoom-wrap img { max-width: none !important; max-height: none !important; width: auto; height: auto; }
.pl-layout { display: flex; flex-direction: column; gap: 12px; }
.pl-player { width: 100%; min-width: 0; }
.pl-player video, .pl-player audio { width: 100%; max-height: 70vh; display: block; background: #000; }
.pl-sidebar { width: 100%; border: 1px solid var(--border); border-radius: 6px; max-height: 400px; overflow-y: auto; }
.pl-item { display: flex; align-items: center; gap: 8px; padding: 8px 12px; border-bottom: 1px solid var(--surface-hover); cursor: pointer; }
.pl-drag { cursor:grab; color:var(--border); padding:0 4px; font-size:14px; flex-shrink:0; user-select:none; line-height:1; }
.pl-drag:hover { color:var(--fg-muted); }
.pl-item.dragging { opacity:0.35; }
.pl-item.drag-over { border-top:2px solid #58a6ff; margin-top:-1px; }
.pl-item:last-child { border-bottom: none; }
.pl-item:hover { background: var(--bg-panel); }
.pl-item.active { background: rgba(88,166,255,0.1); border-left: 3px solid #58a6ff; padding-left: 9px; }
.pl-item-name { flex: 1; font-size: 13px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pl-controls { display: flex; gap: 10px; align-items: center; margin-top: 10px; flex-wrap: wrap; }
.pl-title { font-size: 14px; color: var(--fg); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; margin-bottom: 8px; min-height: 1.5em; }
.pl-badge { color: var(--fg-muted); font-size: 12px; }
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
.btn-edit { background: transparent; border-color: var(--border); color: var(--fg); }
.btn-edit:hover { background: var(--surface-hover); text-decoration: none; color: var(--fg); }
.btn-danger { background: transparent; border-color: rgba(248,81,73,0.4); color: #f85149; }
.btn-danger:hover { background: rgba(248,81,73,0.1); text-decoration: none; }
.btn-cancel { background: transparent; border-color: var(--border); color: var(--fg-muted); }
.btn-cancel:hover { background: var(--surface-hover); text-decoration: none; color: var(--fg); }
form.inline { display: inline; margin: 0; }
.section { margin-bottom: 40px; }
.section-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; }
.table-wrap { border: 1px solid var(--border); border-radius: 6px; overflow-x: auto; }
.form-page { max-width: 580px; }
.form-group { margin-bottom: 16px; }
label { display: block; font-size: 13px; color: var(--fg-muted); margin-bottom: 4px; }
input[type=text], select {
  width: 100%;
  padding: 7px 10px;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--fg);
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
.muted { color: var(--fg-muted); }
.actions-cell { white-space: nowrap; text-align: right; }
.breadcrumb {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--fg-muted);
  font-size: 14px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}
.breadcrumb a { color: #58a6ff; }
.breadcrumb .sep { color: var(--border); }
.breadcrumb .current { color: var(--fg-strong); font-weight: 500; }
.file-row { cursor: pointer; }
.dir-row td { cursor: pointer; }
.root-card {
  display: block;
  padding: 16px 20px;
  border: 1px solid var(--border);
  border-radius: 8px;
  margin-bottom: 10px;
  background: var(--bg-panel);
  color: var(--fg);
  text-decoration: none;
}
.root-card:hover { border-color: #58a6ff; background: var(--surface-active); text-decoration: none; color: var(--fg); }
.root-card-path { font-size: 15px; color: #58a6ff; font-family: monospace; }
.root-card-meta { font-size: 12px; color: var(--fg-muted); margin-top: 4px; }
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
  background: var(--bg-panel);
  border: 1px solid var(--border);
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
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.modal-title { color: var(--fg); font-size: 13px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 70vw; }
.modal-close { color: var(--fg-muted); cursor: pointer; font-size: 20px; line-height: 1; padding: 0 4px; }
.modal-close:hover { color: var(--fg-strong); }
.modal-nav-btn { position: absolute; top: 50%; transform: translateY(-50%); background: rgba(0,0,0,0.45); color: #fff; border: none; font-size: 36px; width: 44px; height: 72px; cursor: pointer; z-index: 10; border-radius: 6px; line-height: 1; padding: 0; user-select: none; }
.modal-nav-btn:hover { background: rgba(0,0,0,0.75); }
.modal-nav-prev { left: 8px; }
.modal-nav-next { right: 8px; }
.modal-body { overflow: auto; flex: 1; display: flex; align-items: center; justify-content: center; }
.modal-body img   { max-width: 90vw; max-height: 85vh; display: block; }
.modal-body video { max-width: 90vw; max-height: 85vh; display: block; }
.modal-box.modal-wide { width: 92vw; }
.modal-box.modal-wide .modal-body video { width: 100%; max-height: calc(92vh - 90px); }
#modal-iz-hint { padding: 4px 0; }
#modal-photo-controls { padding: 10px; }
.modal-box.modal-photo { width: 98vw; max-width: 98vw; max-height: 98vh; }
.modal-box.modal-photo .modal-header { padding: 4px 12px; }
.modal-box.modal-photo #modal-iz-hint { padding: 2px 0; }
.modal-box.modal-photo #modal-photo-controls { padding: 4px 10px; }
.modal-body iframe { width: 82vw; height: 84vh; border: none; display: block; }
.modal-body pre {
  padding: 16px;
  margin: 0;
  max-width: 80vw;
  max-height: 80vh;
  overflow: auto;
  color: var(--fg);
  font-size: 13px;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: monospace;
}
.fav-btn { background:none; border:none; cursor:pointer; color:var(--fg-muted); font-size:16px; padding:2px 4px; line-height:1; }
.fav-btn:hover { color:#e3b341; }
.fav-btn.active { color:#e3b341; }
.pl-unstar-btn { background:none; border:none; color:#e3b341; cursor:pointer; font-size:13px; padding:0 3px; flex-shrink:0; line-height:1; opacity:0.5; transition:opacity 0.15s; }
.pl-unstar-btn:hover { opacity:1; }
.fav-item { display:flex; align-items:center; gap:8px; padding:8px 12px; border-bottom:1px solid var(--surface-hover); cursor:pointer; }
.fav-item:hover { background:var(--bg-panel); }
.fav-item.active { background:rgba(88,166,255,0.1); border-left:3px solid #58a6ff; padding-left:9px; }
.fav-item.dragging { opacity:0.35; }
.fav-item.drag-over { border-top:2px solid #58a6ff; margin-top:-1px; }
.fav-item-icon { flex-shrink:0; font-size:14px; color:var(--fg-muted); }
.fav-item-info { flex:1; min-width:0; display:flex; flex-direction:column; gap:1px; }
.fav-item-name { font-size:13px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.fav-item-path { font-size:11px; color:var(--fg-muted); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.fav-item-count { font-size:11px; color:var(--fg-muted); flex-shrink:0; background:var(--surface-hover); border-radius:8px; padding:1px 6px; }
/* Custom playlist audio player */
.pl-audio-ui { padding:12px 14px; background:var(--bg); border-radius:8px; border:1px solid var(--border); margin-bottom:10px; box-sizing:border-box; width:100%; overflow:hidden; }
.pl-seek-wrap { width:100%; margin-bottom:10px; }
input.pl-seek { -webkit-appearance:none; appearance:none; width:100%; height:4px; background:var(--border); border-radius:2px; outline:none; cursor:pointer; display:block; margin-bottom:5px; }
input.pl-seek::-webkit-slider-thumb { -webkit-appearance:none; width:14px; height:14px; border-radius:50%; background:#bc60ff; cursor:pointer; }
input.pl-seek::-moz-range-thumb { width:14px; height:14px; border-radius:50%; background:#bc60ff; border:none; cursor:pointer; }
.pl-time-row { display:flex; justify-content:space-between; font-size:11px; color:var(--fg-muted); }
.pl-transport { display:grid; grid-template-columns:1fr auto 1fr; align-items:center; }
.pl-transport-btns { display:flex; align-items:center; justify-content:center; gap:16px; }
.pl-nav-btn { background:var(--surface-hover); border:1px solid var(--border); border-radius:50%; width:36px; height:36px; cursor:pointer; color:var(--fg-muted); font-size:11px; display:flex; align-items:center; justify-content:center; padding:0; line-height:1; }
.pl-nav-btn:hover { background:var(--border); color:var(--fg); }
.pl-mode-btn { background:transparent; border:none; border-radius:6px; width:32px; height:32px; cursor:pointer; font-size:15px; opacity:0.35; padding:0; line-height:1; }
.pl-mode-btn:hover { background:var(--surface-hover); }
.pl-mode-btn.active { opacity:1; background:rgba(188,96,255,0.15); }
#pl-play-btn { background:var(--surface-hover); border:1px solid var(--border); border-radius:50%; width:44px; height:44px; cursor:pointer; color:var(--fg); font-size:16px; display:flex; align-items:center; justify-content:center; padding:0; line-height:1; }
#pl-play-btn:hover { background:var(--border); border-color:#bc60ff; }
.pl-vol-wrap { display:flex; align-items:center; gap:5px; justify-self:end; }
.pl-vol-icon { color:var(--fg-muted); font-size:13px; cursor:default; user-select:none; }
input.pl-vol { -webkit-appearance:none; appearance:none; width:70px; height:4px; background:var(--border); border-radius:2px; outline:none; cursor:pointer; }
input.pl-vol::-webkit-slider-thumb { -webkit-appearance:none; width:11px; height:11px; border-radius:50%; background:#58a6ff; cursor:pointer; }
input.pl-vol::-moz-range-thumb { width:11px; height:11px; border-radius:50%; background:#58a6ff; border:none; cursor:pointer; }
@media (pointer: coarse) { .pl-vol-wrap { display:none; } }
.pl-speed-wrap { display:flex; align-items:center; gap:5px; justify-self:start; }
.pl-speed-icon { color:var(--fg-muted); font-size:13px; cursor:default; user-select:none; }
.pl-speed-label { color:var(--fg-muted); font-size:11px; cursor:default; user-select:none; min-width:34px; white-space:nowrap; }
input.pl-speed { -webkit-appearance:none; appearance:none; width:70px; height:4px; background:var(--border); border-radius:2px; outline:none; cursor:pointer; }
input.pl-speed::-webkit-slider-thumb { -webkit-appearance:none; width:11px; height:11px; border-radius:50%; background:#58a6ff; cursor:pointer; }
input.pl-speed::-moz-range-thumb { width:11px; height:11px; border-radius:50%; background:#58a6ff; border:none; cursor:pointer; }
@media (max-width: 640px) {
  main { padding: 12px; }
  header { padding: 10px 16px; flex-wrap: wrap; }
  #play-stats { order: 3; width: 100%; margin-left: 0; justify-content: flex-start; padding-top: 4px; border-top: 1px solid var(--surface-hover); }
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
  .modal-box { max-width: 100vw; max-height: 100vh; width: 100vw; border-radius: 0; }
  .modal-box.modal-wide .modal-body video { max-height: 52vh; }
  .modal-body video { max-width: 100vw; max-height: 52vh; width: 100%; }
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
  /* Search: full-width fixed panel on mobile */
  .header-search { max-width:none; }
  .search-panel { position:fixed; top:80px; left:0; right:0; border-radius:0 0 8px 8px; }
  .search-result { padding:8px 10px; gap:8px; }
  .search-result-thumb { width:44px; height:33px; }
  .search-result-name { font-size:12px; }
  .search-result-dir { font-size:10px; }
  /* Browse: stack sidebar above content on mobile */
  .browse-layout { flex-direction:column; margin:-12px; min-height:0; }
  .browse-sidebar { width:100% !important; border-right:none; border-bottom:1px solid var(--border); flex-direction:row; transition:none; }
  .browse-sidebar.collapsed { width:100% !important; }
  .browse-sidebar.collapsed .sidebar-paths { display:flex; }
  .sidebar-toggle { display:none; }
  .sidebar-paths { display:flex; flex-direction:row; overflow-x:auto; overflow-y:hidden; padding:4px 8px; -webkit-overflow-scrolling:touch; flex:none; width:100%; }
  .browse-sidebar-item { flex-shrink:0; padding:8px 14px; border-left:none !important; padding-left:14px !important; font-size:13px; min-height:44px; display:flex; align-items:center; }
  .browse-sidebar-item.active { border-bottom:2px solid #58a6ff; border-left:none !important; color:var(--fg-strong); background:var(--surface-active); }
  .browse-main { padding:12px; }
  /* Settings: single column */
  .settings-grid { grid-template-columns:1fr !important; }
  /* Taller rows for tap targets */
  td { padding:11px 8px; }
  /* Touch has no hover: grid checkboxes must always be visible */
  .grid-chk { opacity:0.85; width:18px; height:18px; }
  .row-check { width:20px; height:20px; }
  /* Bigger tap targets */
  .fav-btn { font-size:20px; padding:8px; }
  .modal-close { padding:8px 12px; font-size:24px; }
  .sf-chip { padding:8px 14px; }
  .sr-file { min-height:40px; }
  .sr-expand { width:44px; min-height:44px; font-size:16px; }
  .sr-more { padding:10px 12px 12px 32px; }
  .pl-item { padding:11px 12px; }
  .pl-item.active { padding-left:9px; }
  /* HTML5 drag doesn't work on touch; reclaim the row space */
  .pl-drag { display:none; }
  .pl-mode-btn { width:40px; height:40px; font-size:17px; }
  .pl-transport-btns { gap:10px; }
  /* Selection bar wraps to multiple rows with admin buttons */
  .sel-spacer { height:130px; }
  /* Stats chart: 30 tick labels overlap on narrow screens; keep every 10th */
  .stats-ticks > div:not(:nth-child(10n)) { visibility:hidden; }
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
<script>try{var _t=localStorage.getItem('fb_theme');if(_t)document.documentElement.dataset.theme=_t;}catch(e){}</script>
<style>` + css + `</style>
</head>
<body>
<header style="display:flex;align-items:center;gap:16px;padding:10px 24px;background:var(--bg-panel);border-bottom:1px solid var(--border)">
  <span class="logo" style="flex-shrink:0">
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="#58a6ff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="vertical-align:-4px;margin-right:6px"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>File Browser <span style="font-size:0.65em;font-weight:400;opacity:0.45;vertical-align:1px">v{{appVersion}}</span>
  </span>
  <div class="header-search" id="header-search">
    <input id="search-q" type="text" placeholder="Search files…" autocomplete="off"
           oninput="onSearchInput()" onfocus="onSearchFocus()">
    <div id="search-panel" class="search-panel" style="display:none">
      <div class="search-filters">
        <button class="sf-chip active" onclick="setSearchType(this,'all')">All</button>
        <button class="sf-chip" onclick="setSearchType(this,'video')">Video</button>
        <button class="sf-chip" onclick="setSearchType(this,'audio')">Audio</button>
        <button class="sf-chip" onclick="setSearchType(this,'photo')">Photo</button>
      </div>
      <div id="search-results-list"></div>
      <div id="search-status" style="display:none;padding:12px;color:var(--fg-muted);font-size:13px;text-align:center"></div>
    </div>
  </div>
  <span id="play-stats" style="margin-left:auto;color:var(--fg-muted);font-size:12px;white-space:nowrap;flex-shrink:0;display:flex;gap:12px;align-items:center"></span>
  <button id="theme-toggle" class="btn btn-edit btn-sm" onclick="toggleTheme()" title="Toggle light/dark theme" style="flex-shrink:0">&#9789;</button>
</header>
<nav>
  <a href="/browse"     {{if eq .ActiveTab "browse"}}class="active"{{end}}>Browse</a>
  <a href="/recent"     {{if eq .ActiveTab "recent"}}class="active"{{end}}>Recent</a>

  <a href="/stats"      {{if eq .ActiveTab "stats"}}class="active"{{end}}>Stats</a>
  <a href="/favorites"  {{if eq .ActiveTab "favorites"}}class="active"{{end}}>Favorites</a>
  <a href="/playlists"  {{if eq .ActiveTab "playlists"}}class="active"{{end}}>Playlists</a>
  {{if .IsAdmin}}<a href="/users" {{if eq .ActiveTab "users"}}class="active"{{end}}>Users</a>{{end}}
  {{if .IsAdmin}}<a href="/trash" {{if eq .ActiveTab "trash"}}class="active"{{end}}>Trash</a>{{end}}
  {{if .IsAdmin}}<a href="/duplicates" {{if eq .ActiveTab "duplicates"}}class="active"{{end}}>Duplicates</a>{{end}}
  <a href="/settings"   {{if eq .ActiveTab "settings"}}class="active"{{end}}>Settings</a>
  <form action="/logout" method="post" style="margin:0;display:flex;align-items:center;padding:0 4px;flex-shrink:0;margin-left:auto">
    <button type="submit" style="background:transparent;border:1px solid var(--border);color:var(--fg-muted);padding:4px 12px;border-radius:6px;font-size:13px;cursor:pointer;line-height:1.4;white-space:nowrap">Logout</button>
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
      <video id="modal-video" controls style="display:none"></video>
      <audio id="modal-audio" controls style="display:none;width:90vw;max-width:600px"></audio>
      <iframe id="modal-pdf" src="" style="display:none"></iframe>
      <pre id="modal-text" style="display:none"></pre>
    </div>
    <div id="modal-pdf-controls" style="display:none;justify-content:center;align-items:center;gap:12px;padding:10px;background:var(--bg);border-top:1px solid var(--border);flex-shrink:0">
      <button id="modal-pdf-md-btn" class="btn btn-edit btn-sm" onclick="copyPDFMarkdown()">&#128203; Copy as Markdown</button>
    </div>
    <div id="modal-iz-hint" style="display:none;color:var(--fg-muted);font-size:11px;text-align:center;flex-shrink:0;background:var(--bg);border-top:1px solid var(--surface-hover)">Scroll or pinch to zoom &middot; drag to pan &middot; double-click to zoom&thinsp;/&thinsp;fit</div>
    <div id="modal-photo-controls" style="display:none;justify-content:center;align-items:center;gap:12px;background:var(--bg);border-top:1px solid var(--border);flex-shrink:0">
      <button id="modal-slideshow-btn" class="btn btn-edit btn-sm" onclick="toggleSlideshow()">&#9654; Slideshow</button>
    </div>
    <div id="modal-media-controls" style="display:none;justify-content:center;align-items:center;gap:12px;padding:10px;background:var(--bg);border-top:1px solid var(--border);flex-shrink:0">
      <button id="modal-seek-back" class="btn btn-edit btn-sm" onclick="seekActiveMedia(-15)">&#9664;&#9664; 15s</button>
      <button id="modal-seek-fwd" class="btn btn-edit btn-sm" onclick="seekActiveMedia(15)">15s &#9654;&#9654;</button>
      <span id="modal-resume-badge" style="color:var(--fg-muted);font-size:12px;margin-left:8px"></span>
    </div>
  </div>
  <button id="modal-nav-prev" class="modal-nav-btn modal-nav-prev" style="display:none" onclick="event.stopPropagation();modalNavPhoto(-1)">&#8249;</button>
  <button id="modal-nav-next" class="modal-nav-btn modal-nav-next" style="display:none" onclick="event.stopPropagation();modalNavPhoto(1)">&#8250;</button>
</div>
<script src="https://cdn.jsdelivr.net/npm/hls.js@1.4"></script>
<script>
function toggleTheme() {
  var next = document.documentElement.dataset.theme === 'light' ? 'dark' : 'light';
  document.documentElement.dataset.theme = next;
  try { localStorage.setItem('fb_theme', next); } catch(e) {}
}
var _fo = false; try { _fo = !!localStorage.getItem('fb_force_original'); } catch(e) {}
var MOBILE = !_fo && /Mobi|Android|iPhone|iPad|iPod/i.test(navigator.userAgent);
function plLog(msg) {
  try { navigator.sendBeacon('/api/client-log', '[pl] ' + msg + ' vis=' + document.visibilityState + ' @' + location.pathname); } catch(e) {}
}
document.addEventListener('DOMContentLoaded', function() {
  plLog('env mobile=' + MOBILE + ' forceOriginal=' + _fo + ' mse=' + (typeof plMseOk === 'function' ? plMseOk() : 'n/a') + ' ua=' + navigator.userAgent.slice(0, 90));
  // The nav scrolls horizontally on mobile; keep the active tab in view.
  var act = document.querySelector('nav a.active');
  var nav = act && act.parentElement;
  if (act && nav && nav.scrollWidth > nav.clientWidth) {
    nav.scrollLeft = act.offsetLeft - nav.clientWidth / 2 + act.clientWidth / 2;
  }
});
var DEFAULT_VOL = 1; if (!window.matchMedia('(pointer: coarse)').matches) { try { var _dv = parseFloat(localStorage.getItem('fb_default_volume')); if (!isNaN(_dv)) DEFAULT_VOL = Math.max(0, Math.min(1, _dv)); } catch(e) {} }
// Unlike volume, speed has no OS-level equivalent on touch devices, so it's
// not gated behind the coarse-pointer check.
var PL_SPEED_MIN = 0.7, PL_SPEED_MAX = 1.3, PL_SPEED_STEP = 0.01;
var DEFAULT_SPEED = 1; try { var _ds = parseFloat(localStorage.getItem('fb_default_speed')); if (!isNaN(_ds)) DEFAULT_SPEED = Math.max(PL_SPEED_MIN, Math.min(PL_SPEED_MAX, _ds)); } catch(e) {}
function hlsParams() {
  try {
    var ls = localStorage;
    return '&crf=' + (ls.getItem('fb_transcode_crf') || '23') +
      '&preset=' + (ls.getItem('fb_transcode_preset') || 'fast') +
      '&max_width=' + (ls.getItem('fb_transcode_max_width') || '1280') +
      '&video_kbps=' + (ls.getItem('fb_transcode_video_kbps') || '3000') +
      '&audio_kbps=' + (ls.getItem('fb_transcode_audio_kbps') || '128') +
      '&segment_sec=' + (ls.getItem('fb_transcode_segment_sec') || '6') +
      '&audio_hls=' + (ls.getItem('fb_audio_hls_enabled') === '0' ? '0' : '1') +
      '&audio_hls_threshold=' + (ls.getItem('fb_audio_hls_threshold_kbps') || '320');
  } catch(e) { return ''; }
}
var modal = document.getElementById('preview-modal');
(function() {
  function fmtHM(sec) {
    var h = Math.floor(sec / 3600), m = Math.floor((sec % 3600) / 60);
    return h > 0 ? h + 'h ' + m + 'm' : m + 'm';
  }
  function loadPlayStats() {
    fetch('/play/stats').then(function(r) { return r.json(); }).then(function(d) {
      var el = document.getElementById('play-stats');
      if (!el) return;
      el.innerHTML =
        '<span title="Played today">&#9654; ' + fmtHM(d.today_sec) + '</span>' +
        '<span title="Total play time">&#8734; ' + fmtHM(d.total_sec) + '</span>' +
        '<span style="color:#484f58">|</span>' +
        '<span title="Music listened today">&#9835; ' + fmtHM(d.audio_today_sec) + '</span>' +
        '<span title="Total music time">&#8734;&#9835; ' + fmtHM(d.audio_total_sec) + '</span>';
    }).catch(function(){});
  }
  loadPlayStats();
  setInterval(loadPlayStats, 60000);
})();
function _getStoredSort() { try { return localStorage.getItem('fb_sort') || ''; } catch(e) { return ''; } }
function _sortParam() {
  var fromUrl = new URLSearchParams(window.location.search).get('sort');
  var s = fromUrl !== null ? fromUrl : _getStoredSort();
  return s ? '&sort=' + encodeURIComponent(s) : '';
}
function setSort(s) {
  try { localStorage.setItem('fb_sort', s === 'name' ? '' : s); } catch(e) {}
  var params = new URLSearchParams(window.location.search);
  if (s === 'name') { params.delete('sort'); } else { params.set('sort', s); }
  params.set('dir', params.get('dir') || '');
  window.location = '/browse?' + params.toString();
}
function browseDir(el) {
  window.location = '/browse?dir=' + encodeURIComponent(el.dataset.dir) + _sortParam();
}
// onReady(videoEl) is called once the player is ready for seeking:
// for hls.js that's after MANIFEST_PARSED; for others after loadedmetadata.
function attachVideo(videoEl, hlsUrl, directUrl, onReady) {
  if (videoEl.hlsInstance) { videoEl.hlsInstance.destroy(); videoEl.hlsInstance = null; }
  if (typeof Hls !== 'undefined' && Hls.isSupported()) {
    var hls = new Hls();
    hls.on(Hls.Events.ERROR, function(event, data) {
      if (data.fatal) {
        hls.destroy(); videoEl.hlsInstance = null;
        videoEl.src = directUrl; videoEl.load();
        if (onReady) videoEl.addEventListener('loadedmetadata', function() { onReady(videoEl); }, {once: true});
      }
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
// Register an OS-level media session. On Android this keeps Chrome from freezing
// the page during the silent gap between tracks (which otherwise stops playback
// a few seconds into the next track when the screen is off), and provides
// lock-screen / notification controls.
function setMediaSession(title, handlers) {
  if (!('mediaSession' in navigator)) return;
  try {
    navigator.mediaSession.metadata = new MediaMetadata({ title: title || '', album: 'filebrowser' });
    var ms = navigator.mediaSession;
    ms.setActionHandler('play',          handlers.play         || null);
    ms.setActionHandler('pause',         handlers.pause        || null);
    ms.setActionHandler('previoustrack', handlers.prev         || null);
    ms.setActionHandler('nexttrack',     handlers.next         || null);
    ms.setActionHandler('seekbackward',  handlers.seekbackward || null);
    ms.setActionHandler('seekforward',   handlers.seekforward  || null);
  } catch (e) {}
}
// Bind the 'play' event to set playbackState='playing'. We deliberately do NOT
// bind 'pause' here — programmatic pauses during track teardown would immediately
// override the 'playing' state we set in plAdvance/attachMediaResume, causing
// Chrome to freeze the backgrounded page between tracks. 'paused' is set only
// from the explicit user-pause action handler in setMediaSession.
function bindMediaSessionState(media) {
  if (!('mediaSession' in navigator) || media._msBound) return;
  media._msBound = true;
  media.addEventListener('play', function(){ navigator.mediaSession.playbackState = 'playing'; });
}
function clearMediaSession() {
  if (!('mediaSession' in navigator)) return;
  try { navigator.mediaSession.metadata = null; navigator.mediaSession.playbackState = 'none'; } catch (e) {}
}
var _posTracker = {}; // path → last saved position
function _playDelta(path, pos) {
  var last = _posTracker[path];
  _posTracker[path] = pos;
  if (last == null) return 0;
  var d = Math.round(pos - last);
  return (d > 0 && d <= 30) ? d : 0;
}
function saveVideoPos(path, time, completed) {
  var delta = _playDelta(path, time);
  if (completed) delete _posTracker[path];
  fetch('/video/position', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({path: path, position: time, completed: !!completed, delta_sec: delta})
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
    if ('mediaSession' in navigator) navigator.mediaSession.playbackState = 'playing';
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
function dirNextMediaLooping(path) {
  return dirNextMedia(path) || (_folderLoop && window.dirMediaFiles && window.dirMediaFiles.length > 0 ? window.dirMediaFiles[0] : null);
}
// Both wrap around: past the last photo goes to the first and vice versa,
// so the nav buttons are always usable instead of disappearing at the ends.
function photoNext(path) {
  var arr = window.dirPhotoFiles;
  if (!arr || arr.length === 0) return null;
  for (var i = 0; i < arr.length; i++) {
    if (arr[i].path === path) return arr[(i + 1) % arr.length];
  }
  return null;
}
function photoPrev(path) {
  var arr = window.dirPhotoFiles;
  if (!arr || arr.length === 0) return null;
  for (var i = 0; i < arr.length; i++) {
    if (arr[i].path === path) return arr[(i - 1 + arr.length) % arr.length];
  }
  return null;
}
function modalNavPhoto(dir) {
  var img = document.getElementById('modal-img');
  var nxt = dir > 0 ? photoNext(img.dataset.navPath) : photoPrev(img.dataset.navPath);
  if (nxt) openPreview({dataset: nxt}, false);
}
var slideshowTimer = null;
function stopSlideshow() {
  if (!slideshowTimer) return;
  clearInterval(slideshowTimer);
  slideshowTimer = null;
  var btn = document.getElementById('modal-slideshow-btn');
  if (btn) { btn.textContent = '▶ Slideshow'; btn.classList.remove('btn-edit'); }
}
function toggleSlideshow() {
  if (slideshowTimer) { stopSlideshow(); return; }
  slideshowTimer = setInterval(function() { modalNavPhoto(1); }, 4000);
  var btn = document.getElementById('modal-slideshow-btn');
  btn.textContent = '⏸ Slideshow';
  btn.classList.add('btn-edit');
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
  document.getElementById('modal-pdf-controls').style.display = 'none';
  // Not stopped here: openPreview re-enters itself for photo->photo nav
  // (modalNavPhoto, including the slideshow timer's own ticks), so clearing
  // the timer on every call would kill it after a single advance. It's only
  // stopped in closePreview and when a non-photo type takes over the modal.
  document.getElementById('modal-photo-controls').style.display = 'none';
  badge.textContent = '';
  img.src = ''; pdf.src = '';
  document.getElementById('modal-body').style.overflow = '';
  document.getElementById('modal-nav-prev').style.display = document.getElementById('modal-nav-next').style.display = 'none';
  if (video.dataset.resumePath){ if (video.hlsInstance) { video.hlsInstance.destroy(); video.hlsInstance = null; } video.src = ''; video.dataset.resumePath = ''; }
  if (audio.dataset.resumePath) { if (audio.hlsInstance) { audio.hlsInstance.destroy(); audio.hlsInstance = null; } audio.pause(); audio.src = ''; audio.dataset.resumePath = ''; }
  if (type !== 'photo') stopSlideshow();
  if (type === 'photo') {
    // Give the wrap an explicit viewport size: the image is position:absolute
    // so it contributes no intrinsic size, and a flex container would collapse.
    document.getElementById('modal-body').style.overflow = 'hidden';
    img.src = fileUrl;
    img.dataset.navPath = path;
    document.getElementById('modal-nav-prev').style.display = '';
    document.getElementById('modal-nav-next').style.display = '';
    modal.querySelector('.modal-box').classList.add('modal-photo');
    wrap.style.width = '98vw';
    wrap.style.height = 'calc(98vh - 70px)'; // 98vh box minus the thinned header/hint/controls bars
    wrap.style.display = 'block';
    hint.style.display = 'block';
    izInit(wrap, img);
    document.getElementById('modal-photo-controls').style.display = 'flex';
  } else if (type === 'video') {
    modal.querySelector('.modal-box').classList.add('modal-wide');
    seekBack.style.display = ''; seekFwd.style.display = '';
    var _nv = dirNextMediaLooping(path);
    var _vext = path.slice(path.lastIndexOf('.')).toLowerCase();
    var _forceHLS = {'.wmv':1,'.avi':1,'.mkv':1,'.flv':1,'.mov':1}[_vext];
    if (MOBILE || _forceHLS) {
      attachVideo(video, '/hls/playlist?path=' + encodeURIComponent(path) + hlsParams(), fileUrl,
        autoplay ? function(v) { v.play(); } : null);
    } else {
      video.preload = 'auto';
      video.src = fileUrl;
      video.load();
      if (autoplay) video.addEventListener('canplay', function() { video.play(); }, {once: true});
    }
    video.volume = DEFAULT_VOL;
    video.style.display = 'block';
    ctrl.style.display = 'flex';
    setMediaSession(name, {
      play:  function(){ var p = video.play(); if (p && p.catch) p.catch(function(){}); },
      pause: function(){ video.pause(); if ('mediaSession' in navigator) navigator.mediaSession.playbackState = 'paused'; },
      next:  _nv ? function(){ openPreview({dataset: _nv}, true); } : null,
      seekbackward: function(){ seekActiveMedia(-15); },
      seekforward:  function(){ seekActiveMedia(15); }
    });
    bindMediaSessionState(video);
    attachMediaResume(video, path, badge, 'watched', _nv ? function() { openPreview({dataset: _nv}, true); } : null);
  } else if (type === 'audio') {
    // Mobile: hand off to the folder-play page (gapless MSE engine). The
    // modal's per-track src swap lets Android freeze the tab at track ends.
    if (MOBILE) { location.href = '/folder/play?file=' + encodeURIComponent(path); return; }
    seekBack.style.display = 'none'; seekFwd.style.display = 'none';
    audio.volume = DEFAULT_VOL;
    audio.style.display = 'block';
    ctrl.style.display = 'flex';
    var _na = dirNextMediaLooping(path);
    if (!audio._dbgBound) {
      audio._dbgBound = true;
      ['waiting','stalled','error','playing','pause'].forEach(function(ev) {
        audio.addEventListener(ev, function() {
          plLog('preview el ' + ev + ' t=' + (audio.currentTime || 0).toFixed(1) + ' rs=' + audio.readyState + ' net=' + audio.networkState);
        });
      });
      document.addEventListener('visibilitychange', function() {
        if (audio.dataset.resumePath) plLog('preview visibility t=' + (audio.currentTime || 0).toFixed(1) + ' paused=' + audio.paused);
      });
    }
    plLog('preview audio start next=' + !!_na);
    audio.src = fileUrl;
    audio.load();
    if (autoplay) { var p = audio.play(); if (p && p.catch) p.catch(function(e){ plLog('preview play rejected: ' + e.name); }); }
    setMediaSession(name, {
      play:  function(){ var p = audio.play(); if (p && p.catch) p.catch(function(){}); },
      pause: function(){ audio.pause(); if ('mediaSession' in navigator) navigator.mediaSession.playbackState = 'paused'; },
      next:  _na ? function(){ openPreview({dataset: _na}, true); } : null,
      seekbackward: function(){ seekActiveMedia(-15); },
      seekforward:  function(){ seekActiveMedia(15); }
    });
    bindMediaSessionState(audio);
    audio.dataset.resumePath = path;
    audio._lastSave = 0;
    audio.addEventListener('timeupdate', function onTU() {
      if (audio.dataset.resumePath !== path) { audio.removeEventListener('timeupdate', onTU); return; }
      var now = Date.now();
      if (now - (audio._lastSave || 0) > 5000 && audio.currentTime > 1) {
        audio._lastSave = now;
        saveVideoPos(path, audio.currentTime, false);
      }
    });
    audio.addEventListener('ended', function() {
      plLog('preview ended, advancing=' + !!_na);
      if ('mediaSession' in navigator) navigator.mediaSession.playbackState = 'playing';
      saveVideoPos(path, 0, true);
      audio.currentTime = 0;
      badge.textContent = '';
      if (_na) openPreview({dataset: _na}, true);
    }, {once: true});
  } else if (type === 'pdf') {
    pdf.src = fileUrl;
    pdf.dataset.mdPath = path;
    pdf.style.display = 'block';
    document.getElementById('modal-pdf-controls').style.display = 'flex';
  } else if (type === 'text') {
    fetch(fileUrl).then(function(r){return r.text();}).then(function(t){
      txt.textContent = t;
      txt.style.display = 'block';
    });
  }
  modal.classList.add('open');
}
function closePreview() {
  stopSlideshow();
  modal.classList.remove('open');
  modal.querySelector('.modal-box').classList.remove('modal-wide');
  modal.querySelector('.modal-box').classList.remove('modal-photo');
  if (_folderLoop) {
    _folderLoop = false;
    var _pbtn = document.getElementById('btn-play-all');
    if (_pbtn) { _pbtn.textContent = '▶ Loop'; _pbtn.classList.remove('btn-edit'); _pbtn.classList.add('btn-primary'); }
  }
  var video = document.getElementById('modal-video');
  var audio = document.getElementById('modal-audio');
  if (video.dataset.resumePath && video.currentTime > 1) saveVideoPos(video.dataset.resumePath, video.currentTime, false);
  video.dataset.resumePath = ''; audio.dataset.resumePath = '';
  document.getElementById('modal-resume-badge').textContent = '';
  if (video.hlsInstance) { video.hlsInstance.destroy(); video.hlsInstance = null; }
  video.src = '';
  if (audio.hlsInstance) { audio.hlsInstance.destroy(); audio.hlsInstance = null; }
  audio.pause(); audio.src = ''; audio.style.display = 'none';
  clearMediaSession();
  document.getElementById('modal-media-controls').style.display = 'none';
  // Reset image zoom
  document.getElementById('modal-zoom-wrap').style.display = 'none';
  document.getElementById('modal-img').src = '';
  document.getElementById('modal-iz-hint').style.display = 'none';
  document.getElementById('modal-body').style.overflow = '';
  iz.scale = 1; iz.tx = 0; iz.ty = 0; iz.dragging = false;
}
document.addEventListener('keydown', function(e){ if(e.key==='Escape') closePreview(); if(e.key==='ArrowLeft') modalNavPhoto(-1); if(e.key==='ArrowRight') modalNavPhoto(1); });
document.addEventListener('submit', function(e) {
  var action = e.target.getAttribute('action') || '';
  if (action.indexOf('/delete') !== -1) {
    if (!confirm('Remove this? This cannot be undone.')) e.preventDefault();
  }
});
// ---- Search ----
var _searchTimer, _searchType = 'all', _searchOffset = 0, _searchMore = false, _searchLoading = false;
function _anchorSearchPanel() {
  var panel = document.getElementById('search-panel');
  if (!panel || getComputedStyle(panel).position !== 'fixed') return;
  var hdr = document.querySelector('header');
  if (!hdr) return;
  var bottom = hdr.getBoundingClientRect().bottom;
  panel.style.top = bottom + 'px';
  panel.style.maxHeight = (window.innerHeight - bottom - 12) + 'px';
}
function onSearchInput() {
  clearTimeout(_searchTimer);
  _searchTimer = setTimeout(function(){ runSearch(0); }, 300);
  var q = document.getElementById('search-q').value.trim();
  var panel = document.getElementById('search-panel');
  if (panel) { panel.style.display = q.length >= 2 ? '' : 'none'; if (q.length >= 2) _anchorSearchPanel(); }
}
function onSearchFocus() {
  var q = document.getElementById('search-q').value.trim();
  if (q.length >= 2 && !document.getElementById('search-results-list').innerHTML) runSearch(0);
  _anchorSearchPanel();
}
function setSearchType(btn, type) {
  _searchType = type;
  document.querySelectorAll('.sf-chip').forEach(function(c){ c.classList.remove('active'); });
  btn.classList.add('active');
  runSearch(0);
}
function runSearch(offset) {
  var q = document.getElementById('search-q').value.trim();
  var panel = document.getElementById('search-panel');
  if (!panel) return;
  if (q.length < 2) { panel.style.display = 'none'; return; }
  panel.style.display = ''; _anchorSearchPanel();
  if (_searchLoading) return;
  _searchLoading = true;
  var status = document.getElementById('search-status');
  var list = document.getElementById('search-results-list');
  if (offset === 0) { list.innerHTML = ''; status.textContent = 'Searching…'; status.style.display = 'block'; }
  fetch('/search?q=' + encodeURIComponent(q) + '&type=' + _searchType + '&offset=' + offset)
    .then(function(r){ return r.json(); })
    .then(function(results){
      _searchLoading = false;
      status.style.display = 'none';
      if (offset === 0 && (!results || !results.length)) {
        status.textContent = 'No results.'; status.style.display = 'block'; return;
      }
      _searchOffset = offset + (results ? results.length : 0);
      _searchMore = results && results.length === 20;
      if (!results || !results.length) return;
      var html = results.map(function(item){
        var parts = item.dir_path.split('/').filter(Boolean);
        var folderName = parts.length ? parts[parts.length - 1] : item.dir_path;
        var thumb = (item.sample_type === 'video' || item.sample_type === 'photo')
          ? '<img src="/thumbnail?path=' + encodeURIComponent(item.sample_path) + '" loading="lazy" onerror="this.style.display=\'none\'">'
          : '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="28" height="28" fill="none" stroke="#58a6ff" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>';
        var matchLabel = item.match_count + ' file' + (item.match_count === 1 ? '' : 's');
        var inline = item.match_count <= 3 && item.files && item.files.length;
        var chevron = !inline
          ? '<button class="sr-expand" data-dir="' + escAttr(item.dir_path) + '" data-count="' + item.match_count + '" onclick="toggleSearchFiles(event,this)" title="Show matching files">&#9656;</button>'
          : '';
        var filesHtml = inline ? item.files.map(renderSearchFile).join('') : '';
        return '<div class="search-result-group">' +
          '<div class="search-result" data-dir="' + escAttr(item.dir_path) + '" onclick="openSearchResult(this)">' +
            '<div class="search-result-thumb">' + thumb + '</div>' +
            '<div style="min-width:0;flex:1">' +
              '<div class="search-result-name">' + escHtml(folderName) + '</div>' +
              '<div class="search-result-dir">' + escHtml(item.dir_path) + '</div>' +
              '<span class="badge badge-dir" style="font-size:10px">' + escHtml(matchLabel) + '</span>' +
            '</div>' + chevron +
          '</div>' +
          '<div class="sr-files">' + filesHtml + '</div>' +
        '</div>';
      }).join('');
      list.insertAdjacentHTML('beforeend', html);
    }).catch(function(){ _searchLoading = false; status.textContent = 'Search failed.'; status.style.display = 'block'; });
}
document.addEventListener('scroll', function() {
  var panel = document.getElementById('search-panel');
  if (!panel || panel.style.display === 'none') return;
  if (!_searchMore || _searchLoading) return;
  if (panel.scrollTop + panel.clientHeight >= panel.scrollHeight - 60) {
    runSearch(_searchOffset);
  }
}, true);
function openSearchResult(el) {
  // intentionally doesn't preserve sort — search jumps to a fresh folder
  var panel = document.getElementById('search-panel');
  if (panel) panel.style.display = 'none';
  document.getElementById('search-q').value = '';
  window.location = '/browse?dir=' + encodeURIComponent(el.dataset.dir);
}
function renderSearchFile(f) {
  return '<div class="sr-file" data-path="' + escAttr(f.path) + '" data-type="' + escAttr(f.type) + '" onclick="openSearchFile(event,this)">' +
    '<span class="badge badge-' + escAttr(f.type) + '" style="flex-shrink:0">' + escHtml(f.type.toUpperCase()) + '</span>' +
    '<span class="sr-file-name">' + escHtml(f.name) + '</span></div>';
}
function openSearchFile(e, el) {
  e.stopPropagation();
  var panel = document.getElementById('search-panel');
  if (panel) panel.style.display = 'none';
  document.getElementById('search-q').value = '';
  var p = el.dataset.path;
  if (el.dataset.type === 'audio') {
    window.location = '/folder/play?file=' + encodeURIComponent(p);
  } else {
    window.location = '/browse?dir=' + encodeURIComponent(p.slice(0, p.lastIndexOf('/')));
  }
}
function toggleSearchFiles(e, btn) {
  e.stopPropagation();
  var group = btn.closest('.search-result-group');
  var wrap = group ? group.querySelector('.sr-files') : null;
  if (!wrap) return;
  if (wrap.innerHTML && wrap.style.display !== 'none') { wrap.style.display = 'none'; btn.innerHTML = '&#9656;'; return; }
  if (wrap.innerHTML) { wrap.style.display = ''; btn.innerHTML = '&#9662;'; return; }
  btn.innerHTML = '&#8943;';
  var q = document.getElementById('search-q').value.trim();
  var count = parseInt(btn.dataset.count || '0', 10);
  fetch('/search/files?q=' + encodeURIComponent(q) + '&type=' + _searchType + '&dir=' + encodeURIComponent(btn.dataset.dir))
    .then(function(r){ return r.json(); })
    .then(function(files){
      var html = files.map(renderSearchFile).join('');
      if (count > files.length) {
        html += '<div class="sr-more" data-dir="' + escAttr(btn.dataset.dir) + '" onclick="openSearchResult(this)">+ ' + (count - files.length) + ' more — open folder</div>';
      }
      wrap.innerHTML = html;
      wrap.style.display = '';
      btn.innerHTML = '&#9662;';
    })
    .catch(function(){ btn.innerHTML = '&#9656;'; });
}
function escHtml(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}
function escAttr(s) {
  return String(s).replace(/&/g,'&amp;').replace(/"/g,'&quot;').replace(/'/g,'&#39;');
}
document.addEventListener('click', function(e) {
  var hs = document.getElementById('header-search');
  if (hs && !hs.contains(e.target)) {
    var p = document.getElementById('search-panel');
    if (p) p.style.display = 'none';
  }
});
document.addEventListener('keydown', function(e) {
  if (e.key === 'Escape') {
    var p = document.getElementById('search-panel');
    if (p && p.style.display !== 'none') { p.style.display = 'none'; e.stopPropagation(); }
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
      <a class="browse-sidebar-item{{if eq .Path $.CurrentRoot}} active{{end}}" href="{{browseURL .Path}}" title="{{.Path}}">{{base .Path}}</a>
      {{else}}
      <span style="padding:8px 16px;color:var(--fg-muted);font-size:12px;display:block">No paths. Add one in <a href="/settings">Settings</a>.</span>
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
      <div style="display:flex;gap:8px;align-items:center">
        {{if and .IsAdmin (not .InZip)}}<label class="btn btn-primary btn-sm" id="upload-btn" style="cursor:pointer;margin:0" title="Upload files to this folder">&#8679; Upload<input type="file" id="upload-input" multiple style="display:none" onchange="uploadFiles(this.files)"></label><span id="upload-status" style="display:none;color:var(--fg-muted);font-size:13px;white-space:nowrap"></span>{{end}}
        {{if and .IsAdmin (not .InZip)}}<button id="btn-new-folder" class="btn btn-primary btn-sm" onclick="createFolder()" title="Create a new subdirectory in this folder">+ New Folder</button>{{end}}
        <button id="btn-play-all" class="btn btn-primary btn-sm" onclick="playFolderAll()" style="display:none" title="Play all media in this folder in a loop">&#9654; Loop</button>
        {{if or .Files .Subdirs}}<button id="btn-select" class="btn btn-edit btn-sm" onclick="toggleExtMenu(event)" title="Show only folders and files with certain extensions">Filter &#9662;</button>{{end}}
        <div class="view-toggle">
          <button class="btn-view {{if eq .SortBy "date"}}active{{end}}" onclick="setSort('date')" title="Sort by date">&#128197; Date</button>
          <button class="btn-view {{if ne .SortBy "date"}}active{{end}}" onclick="setSort('name')" title="Sort by name">A&#8250;Z Name</button>
        </div>
        <div class="view-toggle">
          <button id="btn-list" class="btn-view" onclick="setView('list')" title="List view">&#9776; List</button>
          <button id="btn-grid" class="btn-view" onclick="setView('grid')" title="Grid view">&#8859; Grid</button>
        </div>
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
      <td onclick="event.stopPropagation()"><input type="checkbox" class="row-check" value="{{.AbsPath}}" data-type="dir" onchange="updateSelBar()" onclick="event.stopPropagation()" style="cursor:pointer"></td>
      <td>
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="#58a6ff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="vertical-align:-2px;margin-right:6px"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>{{.Name}}
      </td>
      <td><span class="badge badge-dir">DIR</span></td>
      <td class="muted">—</td>
      <td class="muted">{{.ModifiedAt}}</td>
      <td></td>
      <td onclick="event.stopPropagation()"><button class="fav-btn" data-path="{{.AbsPath}}" data-folder="1" onclick="toggleFav(this)" title="Favorite folder">☆</button></td>
    </tr>
    {{end}}
    {{range .Files}}
    {{if eq .FileType "archive"}}
    <tr class="dir-row" data-dir="{{.AbsPath}}" onclick="browseDir(this)">
      <td onclick="event.stopPropagation()"><input type="checkbox" class="row-check" value="{{.AbsPath}}" data-type="other" data-ext="{{.Extension}}" onchange="updateSelBar()" onclick="event.stopPropagation()" style="cursor:pointer"></td>
      <td>
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="#db6d28" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="vertical-align:-2px;margin-right:6px"><path d="M21 8v13H3V8"/><path d="M1 3h22v5H1z"/><line x1="10" y1="12" x2="14" y2="12"/></svg>{{.Filename}}
      </td>
      <td><span class="badge badge-archive">ZIP</span></td>
      <td class="muted">{{.Size}}</td>
      <td class="muted">{{.ModifiedAt}}</td>
      <td class="muted">—</td>
      <td></td>
    </tr>
    {{else if eq .FileType "other"}}
    <tr data-path="{{.AbsPath}}" data-name="{{.Filename}}" data-type="other">
      <td><input type="checkbox" class="row-check" value="{{.AbsPath}}" data-type="other" data-ext="{{.Extension}}" onchange="updateSelBar()" onclick="event.stopPropagation()" style="cursor:pointer"></td>
      <td><a href="{{fileURL .AbsPath}}" onclick="event.stopPropagation()">{{.Filename}}</a></td>
      <td><span class="badge badge-{{.FileType}}">{{upper .FileType}}</span></td>
      <td class="muted">{{.Size}}</td>
      <td class="muted">{{.ModifiedAt}}</td>
      <td class="muted">—</td>
      <td></td>
    </tr>
    {{else}}
    <tr class="file-row" data-path="{{.AbsPath}}" data-name="{{.Filename}}" data-type="{{.FileType}}" onclick="openPreview(this, true)">
      <td><input type="checkbox" class="row-check" value="{{.AbsPath}}" data-type="{{.FileType}}" data-ext="{{.Extension}}" onchange="updateSelBar()" onclick="event.stopPropagation()" style="cursor:pointer"></td>
      <td>{{.Filename}}</td>
      <td><span class="badge badge-{{.FileType}}">{{upper .FileType}}</span></td>
      <td class="muted">{{.Size}}</td>
      <td class="muted">{{.ModifiedAt}}</td>
      <td>{{if and (or (eq .FileType "video") (eq .FileType "audio")) (gt .WatchCount 0)}}<span class="badge badge-{{.FileType}}">{{.WatchCount}}×</span>{{else}}<span class="muted">—</span>{{end}}</td>
      <td onclick="event.stopPropagation()">{{if eq .FileType "audio"}}<button class="fav-btn" data-path="{{.AbsPath}}" data-folder="0" onclick="toggleFav(this)" title="Favorite">☆</button>{{end}}</td>
    </tr>
    {{end}}
    {{end}}
    </tbody>
    </table>
    </div>
    </div>
    <div id="view-grid" class="view-grid" style="display:none">
    {{range .Subdirs}}
    <div class="grid-card" data-dir="{{.AbsPath}}" onclick="gridDirClick(event,this)">
      <input class="grid-chk row-check" type="checkbox" value="{{.AbsPath}}" data-type="dir" onchange="gridCheck(event,this)" onclick="event.stopPropagation()" style="cursor:pointer;width:14px;height:14px">
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
      <button class="fav-btn" data-path="{{.AbsPath}}" data-folder="1" onclick="event.stopPropagation();toggleFav(this)" title="Favorite folder" style="position:absolute;top:4px;right:4px;font-size:14px;background:rgba(0,0,0,0.65);border-radius:3px;color:#fff">☆</button>
    </div>
    {{end}}
    {{range .Files}}
    {{if eq .FileType "photo"}}
    <div class="grid-card" data-path="{{.AbsPath}}" data-name="{{.Filename}}" data-type="photo" onclick="gridClick(event,this)">
      <input class="grid-chk row-check" type="checkbox" value="{{.AbsPath}}" data-type="photo" data-ext="{{.Extension}}" onchange="gridCheck(event,this)" onclick="event.stopPropagation()" style="cursor:pointer;width:14px;height:14px">
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
      <input class="grid-chk row-check" type="checkbox" value="{{.AbsPath}}" data-type="video" data-ext="{{.Extension}}" onchange="gridCheck(event,this)" onclick="event.stopPropagation()" style="cursor:pointer;width:14px;height:14px">
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
      <input class="grid-chk row-check" type="checkbox" value="{{.AbsPath}}" data-type="audio" data-ext="{{.Extension}}" onchange="gridCheck(event,this)" onclick="event.stopPropagation()" style="cursor:pointer;width:14px;height:14px">
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
      <button class="fav-btn" data-path="{{.AbsPath}}" data-folder="0" onclick="event.stopPropagation();toggleFav(this)" title="Favorite" style="position:absolute;top:4px;right:4px;font-size:14px;background:rgba(0,0,0,0.65);border-radius:3px;color:#fff">☆</button>
    </div>
    {{else if eq .FileType "pdf"}}
    <div class="grid-card" data-path="{{.AbsPath}}" data-name="{{.Filename}}" data-type="pdf" onclick="gridClick(event,this)">
      <input class="grid-chk row-check" type="checkbox" value="{{.AbsPath}}" data-type="pdf" data-ext="{{.Extension}}" onchange="gridCheck(event,this)" onclick="event.stopPropagation()" style="cursor:pointer;width:14px;height:14px">
      <div class="grid-icon"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="#f85149" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg></div>
      <div class="grid-name">{{.Filename}}</div>
    </div>
    {{else if eq .FileType "text"}}
    <div class="grid-card" data-path="{{.AbsPath}}" data-name="{{.Filename}}" data-type="text" onclick="gridClick(event,this)">
      <input class="grid-chk row-check" type="checkbox" value="{{.AbsPath}}" data-type="text" data-ext="{{.Extension}}" onchange="gridCheck(event,this)" onclick="event.stopPropagation()" style="cursor:pointer;width:14px;height:14px">
      <div class="grid-icon"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="#d29922" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><line x1="10" y1="9" x2="8" y2="9"/></svg></div>
      <div class="grid-name">{{.Filename}}</div>
    </div>
    {{else if eq .FileType "archive"}}
    <div class="grid-card" data-dir="{{.AbsPath}}" onclick="gridDirClick(event,this)">
      <input class="grid-chk row-check" type="checkbox" value="{{.AbsPath}}" data-type="other" data-ext="{{.Extension}}" onchange="gridCheck(event,this)" onclick="event.stopPropagation()" style="cursor:pointer;width:14px;height:14px">
      {{if .AlbumArt}}
      <div class="grid-thumb" style="position:relative">
        <img src="{{thumbURL .AlbumArt}}" loading="lazy" alt="" style="width:100%;height:100%;object-fit:cover;display:block"
             onerror="this.style.display='none';this.nextElementSibling.style.display='flex'">
        <div style="display:none;width:100%;height:100%;align-items:center;justify-content:center">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="#db6d28" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M21 8v13H3V8"/><path d="M1 3h22v5H1z"/><line x1="10" y1="12" x2="14" y2="12"/></svg>
        </div>
        <div style="position:absolute;bottom:4px;right:4px;background:rgba(0,0,0,0.55);border-radius:3px;padding:2px 3px;line-height:0" title="Archive">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="#db6d28" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 8v13H3V8"/><path d="M1 3h22v5H1z"/><line x1="10" y1="12" x2="14" y2="12"/></svg>
        </div>
      </div>
      {{else}}
      <div class="grid-icon"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="#db6d28" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M21 8v13H3V8"/><path d="M1 3h22v5H1z"/><line x1="10" y1="12" x2="14" y2="12"/></svg></div>
      {{end}}
      <div class="grid-name">{{.Filename}}</div>
    </div>
    {{else}}
    <div class="grid-card" data-path="{{.AbsPath}}" data-name="{{.Filename}}" data-type="other" onclick="gridClick(event,this)">
      <input class="grid-chk row-check" type="checkbox" value="{{.AbsPath}}" data-type="other" data-ext="{{.Extension}}" onchange="gridCheck(event,this)" onclick="event.stopPropagation()" style="cursor:pointer;width:14px;height:14px">
      <a href="{{fileURL .AbsPath}}" onclick="event.stopPropagation()" style="display:contents">
        <div class="grid-icon"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="var(--fg-muted)" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg></div>
        <div class="grid-name">{{.Filename}}</div>
      </a>
    </div>
    {{end}}
    {{end}}
    </div>
    {{end}}
    {{end}}
  </div>
</div>
<div class="sel-spacer"></div>
<div id="sel-bar" style="display:none;position:fixed;bottom:0;left:0;right:0;background:var(--bg-panel);border-top:1px solid var(--border);padding:12px 24px;z-index:200;align-items:center;gap:12px;flex-wrap:wrap">
  <span id="sel-count" style="color:var(--fg);font-size:14px;white-space:nowrap"></span>
  <select id="sel-pl" style="background:var(--bg);border:1px solid var(--border);border-radius:6px;color:var(--fg);font-size:13px;padding:5px 8px;display:none">
    <option value="">Add to playlist...</option>
    {{range .Playlists}}<option value="{{.ID}}">{{.Name}}</option>{{end}}
  </select>
  <button id="sel-pl-btn" class="btn btn-primary btn-sm" style="display:none" onclick="addSelectedToPlaylist()">Add to Playlist</button>
  <button id="sel-ext-btn" class="btn btn-edit btn-sm" style="display:none" onclick="selectSameExt()"></button>
  <button class="btn btn-edit btn-sm" onclick="downloadSelected()">⬇ Download</button>
  {{if and .IsAdmin (not .InZip)}}
  <button id="sel-rename" class="btn btn-edit btn-sm" style="display:none" onclick="renameSelected()">&#x270E; Rename</button>
  <button class="btn btn-edit btn-sm" onclick="moveSelected()">&#128193; Move</button>
  <button class="btn btn-danger btn-sm" onclick="deleteSelected()">&#128465; Delete</button>
  {{end}}
  <button class="btn btn-edit btn-sm" onclick="clearSelection()">&#x2715; Clear</button>
  <span id="sel-ok" style="display:none;color:#3fb950;font-size:13px"></span>
</div>
<div id="ext-menu" style="display:none" onclick="event.stopPropagation()"></div>
<script>
function selView() {
  var grid = document.getElementById('view-grid');
  if (grid && grid.style.display !== 'none') return grid;
  return document.getElementById('view-list');
}
// Selection queries are scoped to the visible view: list and grid each render
// a checkbox for the same file, so document-wide queries double-count.
function selChecks(onlyChecked) {
  var v = selView();
  if (!v) return [];
  return Array.from(v.querySelectorAll('.row-check' + (onlyChecked ? ':checked' : '')));
}
function setCheck(c, on) {
  c.checked = on;
  var card = c.closest('.grid-card');
  if (card) card.classList.toggle('grid-checked', on);
}
function extOf(c) { return c.dataset.ext || ''; }
// Extension filter: null shows everything, otherwise a Set of keys to show.
// Folders filter under the 'dir' key (real extensions start with '.' or are
// '' for extensionless files, so it can't collide). Transient — resets on
// navigation.
var extFilter = null;
function fileRowEl(c) { return c.closest('tr') || c.closest('.grid-card'); }
function filterKey(c) { return c.dataset.type === 'dir' ? 'dir' : extOf(c); }
function isFiltered(c) {
  return extFilter !== null && !extFilter.has(filterKey(c));
}
function toggleFilterKey(key) {
  if (extFilter === null) {
    extFilter = new Set([key]);
  } else if (extFilter.has(key)) {
    extFilter.delete(key);
    if (extFilter.size === 0) extFilter = null;
  } else {
    extFilter.add(key);
  }
  applyExtFilter();
}
function applyExtFilter() {
  document.querySelectorAll('.row-check').forEach(function(c) {
    var el = fileRowEl(c);
    if (!el) return;
    var hide = isFiltered(c);
    el.style.display = hide ? 'none' : '';
    if (hide && c.checked) setCheck(c, false);
  });
  var btn = document.getElementById('btn-select');
  if (btn) {
    if (extFilter === null) {
      btn.textContent = 'Filter ▾';
      btn.classList.remove('filter-on');
    } else {
      var exts = Array.from(extFilter).map(function(e) { return e === 'dir' ? 'folders' : (e || '(no ext)'); });
      btn.textContent = 'Filter: ' + (exts.length <= 2 ? exts.join(' ') : exts.length + ' types') + ' ▾';
      btn.classList.add('filter-on');
    }
  }
  updateSelBar();
}
function updateSelBar() {
  var all = selChecks(false).filter(function(c) { return !isFiltered(c); });
  var checks = selChecks(true);
  var bar = document.getElementById('sel-bar');
  var count = document.getElementById('sel-count');
  bar.style.display = checks.length > 0 ? 'flex' : 'none';
  var dirCount = checks.filter(function(c) { return c.dataset.type === 'dir'; }).length;
  var hasDirs = dirCount > 0;
  var hasMedia = checks.some(function(c) { return c.dataset.type === 'audio' || c.dataset.type === 'video'; });
  var label = checks.length + ' item' + (checks.length === 1 ? '' : 's') + ' selected';
  if (hasDirs) label += ' (folder' + (dirCount === 1 ? '' : 's') + ')';
  count.textContent = label;
  var showPl = hasMedia || hasDirs;
  var pl = document.getElementById('sel-pl');
  var plBtn = document.getElementById('sel-pl-btn');
  if (pl) pl.style.display = showPl ? '' : 'none';
  if (plBtn) plBtn.style.display = showPl ? '' : 'none';
  var ren = document.getElementById('sel-rename');
  if (ren) ren.style.display = checks.length === 1 ? '' : 'none';
  // Offer "Select all .ext" when the selection is files sharing one extension
  // and more files with that extension remain unselected.
  var extBtn = document.getElementById('sel-ext-btn');
  if (extBtn) {
    var show = false;
    var files = checks.filter(function(c) { return c.dataset.type !== 'dir'; });
    if (files.length > 0 && !hasDirs && files.every(function(c) { return extOf(c) === extOf(files[0]); })) {
      var ext = extOf(files[0]);
      var total = all.filter(function(c) { return c.dataset.type !== 'dir' && extOf(c) === ext; }).length;
      if (total > files.length) {
        extBtn.textContent = 'Select all ' + (ext || 'without extension') + ' (' + total + ')';
        extBtn.dataset.ext = ext;
        show = true;
      }
    }
    extBtn.style.display = show ? '' : 'none';
  }
  var selAll = document.getElementById('sel-all');
  if (selAll) {
    selAll.indeterminate = checks.length > 0 && checks.length < all.length;
    selAll.checked = all.length > 0 && checks.length === all.length;
  }
}
function toggleSelectAll(cb) {
  selChecks(false).forEach(function(c) { if (!isFiltered(c)) setCheck(c, cb.checked); });
  updateSelBar();
}
function clearSelection() {
  document.querySelectorAll('.row-check').forEach(function(c) { setCheck(c, false); });
  var selAll = document.getElementById('sel-all');
  if (selAll) { selAll.checked = false; selAll.indeterminate = false; }
  updateSelBar();
}
function selectSameExt() {
  var ext = document.getElementById('sel-ext-btn').dataset.ext || '';
  selChecks(false).forEach(function(c) {
    if (c.dataset.type !== 'dir' && extOf(c) === ext && !isFiltered(c)) setCheck(c, true);
  });
  updateSelBar();
}
function hideExtMenu() {
  var m = document.getElementById('ext-menu');
  if (m) m.style.display = 'none';
}
function toggleExtMenu(ev) {
  ev.stopPropagation();
  var menu = document.getElementById('ext-menu');
  if (menu.style.display !== 'none') { hideExtMenu(); return; }
  buildExtMenu();
  var r = ev.currentTarget.getBoundingClientRect();
  menu.style.display = 'block';
  // Clamp inside the viewport: the toolbar button sits near the right edge.
  menu.style.left = Math.max(8, Math.min(r.left, window.innerWidth - menu.offsetWidth - 8)) + 'px';
  menu.style.top = (r.bottom + 4) + 'px';
}
function buildExtMenu() {
  var menu = document.getElementById('ext-menu');
  menu.textContent = '';
  var all = selChecks(false);
  var files = all.filter(function(c) { return c.dataset.type !== 'dir'; });
  var dirCount = all.length - files.length;
  function addItem(label, cnt, checked, onClick) {
    var d = document.createElement('div');
    d.className = 'ext-menu-item';
    var mark = document.createElement('span');
    mark.className = 'mark';
    mark.textContent = '✓';
    if (!checked) mark.style.visibility = 'hidden';
    d.appendChild(mark);
    var l = document.createElement('span');
    l.textContent = label;
    l.style.flex = '1';
    d.appendChild(l);
    if (cnt !== null) {
      var s = document.createElement('span');
      s.className = 'cnt';
      s.textContent = cnt;
      d.appendChild(s);
    }
    d.onclick = function(e) { e.stopPropagation(); onClick(); updateSelBar(); buildExtMenu(); };
    menu.appendChild(d);
  }
  function sep() {
    var d = document.createElement('div');
    d.className = 'ext-menu-sep';
    menu.appendChild(d);
  }
  addItem('Show all', null, extFilter === null, function() { extFilter = null; applyExtFilter(); });
  sep();
  if (dirCount > 0) {
    addItem('Folders', dirCount, extFilter === null || extFilter.has('dir'), function() { toggleFilterKey('dir'); });
  }
  var counts = {};
  files.forEach(function(c) { var e = extOf(c); counts[e] = (counts[e] || 0) + 1; });
  Object.keys(counts).sort(function(a, b) { return counts[b] - counts[a] || (a < b ? -1 : 1); }).forEach(function(ext) {
    var shown = extFilter === null || extFilter.has(ext);
    addItem(ext || '(no extension)', counts[ext], shown, function() { toggleFilterKey(ext); });
  });
  sep();
  addItem('Select shown', null, false, function() {
    all.forEach(function(c) { if (!isFiltered(c)) setCheck(c, true); });
    hideExtMenu();
  });
}
document.addEventListener('click', hideExtMenu);
document.addEventListener('keydown', function(e) { if (e.key === 'Escape') hideExtMenu(); });
function setView(v) {
  var list = document.getElementById('view-list');
  var grid = document.getElementById('view-grid');
  if (!list || !grid) return;
  list.style.display = v === 'list' ? '' : 'none';
  grid.style.display = v === 'grid' ? 'grid' : 'none';
  document.getElementById('btn-list').classList.toggle('active', v === 'list');
  document.getElementById('btn-grid').classList.toggle('active', v === 'grid');
  try { localStorage.setItem('fb_view', v); } catch(e) {}
  hideExtMenu();
  updateSelBar();
}
function gridClick(event, el) {
  var chk = el.querySelector('.grid-chk');
  if (chk && chk.checked) { chk.checked = false; el.classList.remove('grid-checked'); updateSelBar(); return; }
  if (el.dataset.type === 'other') {
    var link = el.querySelector('a[href]');
    if (link) { link.click(); }
    return;
  }
  openPreview(el, true);
}
function copyPDFMarkdown() {
  var path = document.getElementById('modal-pdf').dataset.mdPath;
  if (!path) return;
  var btn = document.getElementById('modal-pdf-md-btn');
  btn.textContent = '⏳ Converting…';
  btn.disabled = true;
  fetch('/api/pdf/markdown?path=' + encodeURIComponent(path))
    .then(function(r) {
      if (!r.ok) return r.text().then(function(t) { throw new Error(t); });
      return r.text();
    })
    .then(function(md) {
      if (navigator.clipboard && navigator.clipboard.writeText) {
        return navigator.clipboard.writeText(md);
      }
      var ta = document.createElement('textarea');
      ta.value = md;
      ta.style.cssText = 'position:fixed;top:0;left:0;opacity:0';
      document.body.appendChild(ta);
      ta.focus();
      ta.select();
      var ok = document.execCommand('copy');
      document.body.removeChild(ta);
      if (!ok) throw new Error('Clipboard unavailable');
    })
    .then(function() {
      btn.textContent = '✓ Copied!';
      setTimeout(function() { btn.textContent = '📋 Copy as Markdown'; btn.disabled = false; }, 2000);
    })
    .catch(function(e) {
      btn.textContent = '📋 Copy as Markdown';
      btn.disabled = false;
      alert('Failed: ' + e.message);
    });
}
function gridDirClick(event, el) {
  var chk = el.querySelector('.grid-chk');
  if (chk && chk.checked) { chk.checked = false; el.classList.remove('grid-checked'); updateSelBar(); return; }
  browseDir(el);
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
  var checked = selChecks(true);
  var dirPaths = checked.filter(function(c) { return c.dataset.type === 'dir'; }).map(function(c) { return c.value; });
  var filePaths = checked.filter(function(c) {
    var t = c.dataset.type;
    return !t || t === 'audio' || t === 'video';
  }).map(function(c) { return c.value; });
  if (dirPaths.length === 0 && filePaths.length === 0) return;
  var promises = [];
  dirPaths.forEach(function(path) {
    promises.push(fetch('/api/folder/playlist-add', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({path: path, playlist_id: parseInt(plId, 10)})
    }).then(function(r) { return r.ok ? r.json() : {added: 0}; }));
  });
  filePaths.forEach(function(path) {
    promises.push(fetch('/playlists/' + plId + '/items', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({path: path})
    }).then(function() { return {added: 1}; }));
  });
  Promise.all(promises).then(function(results) {
    var added = results.reduce(function(acc, r) { return acc + (r.added || 0); }, 0);
    var ok = document.getElementById('sel-ok');
    ok.textContent = added + ' track' + (added === 1 ? '' : 's') + ' added to playlist';
    ok.style.display = 'inline';
    setTimeout(function() { ok.style.display = 'none'; clearSelection(); }, 1500);
  });
}
function downloadSelected() {
  var checked = selChecks(true);
  var i = 0;
  checked.forEach(function(c) {
    var path = c.value;
    var isDir = c.dataset.type === 'dir';
    var idx = i++;
    setTimeout(function() {
      var a = document.createElement('a');
      a.href = isDir
        ? '/api/folder/download?path=' + encodeURIComponent(path)
        : '/file?path=' + encodeURIComponent(path) + '&dl=1';
      a.download = '';
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
    }, idx * 300);
  });
}
function selPaths() {
  return selChecks(true).map(function(c) { return c.value; });
}
function deleteSelected() {
  var checked = selChecks(true);
  if (checked.length === 0) return;
  var dirPaths = checked.filter(function(c) { return c.dataset.type === 'dir'; }).map(function(c) { return c.value; });
  var filePaths = checked.filter(function(c) { return c.dataset.type !== 'dir'; }).map(function(c) { return c.value; });
  var msg = '';
  if (dirPaths.length > 0 && filePaths.length === 0)
    msg = 'Move ' + dirPaths.length + ' folder' + (dirPaths.length === 1 ? '' : 's') + ' to Trash?';
  else if (filePaths.length > 0 && dirPaths.length === 0)
    msg = 'Move ' + filePaths.length + ' file' + (filePaths.length === 1 ? '' : 's') + ' to Trash?';
  else
    msg = 'Move ' + filePaths.length + ' file' + (filePaths.length === 1 ? '' : 's') + ' and ' + dirPaths.length + ' folder' + (dirPaths.length === 1 ? '' : 's') + ' to Trash?';
  if (!confirm(msg)) return;
  var promises = [];
  if (filePaths.length > 0)
    promises.push(fetch('/api/file/delete', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({paths: filePaths})}).then(function(r) { return r.json(); }));
  if (dirPaths.length > 0)
    promises.push(fetch('/api/folder/delete', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({paths: dirPaths})}).then(function(r) { return r.json(); }));
  Promise.all(promises)
    .then(function(results) {
      var errs = [];
      results.forEach(function(res) { if (res.errors && res.errors.length) errs = errs.concat(res.errors); });
      if (errs.length) alert('Some items failed:\n' + errs.join('\n'));
      location.reload();
    })
    .catch(function(e) { alert('Delete failed: ' + e); });
}
function renameSelected() {
  var checked = selChecks(true);
  if (checked.length !== 1) return;
  var isDir = checked[0].dataset.type === 'dir';
  var path = checked[0].value;
  var base = path.split('/').pop();
  var name = prompt('New name:', base);
  if (!name || name === base) return;
  var endpoint = isDir ? '/api/folder/rename' : '/api/file/rename';
  fetch(endpoint, {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({path: path, new_name: name})})
    .then(function(r) {
      if (!r.ok) return r.text().then(function(t) { alert('Rename failed: ' + t); });
      location.reload();
    });
}
function moveSelected() {
  var checked = selChecks(true);
  if (checked.length === 0) return;
  var dirPaths = checked.filter(function(c) { return c.dataset.type === 'dir'; }).map(function(c) { return c.value; });
  var filePaths = checked.filter(function(c) { return c.dataset.type !== 'dir'; }).map(function(c) { return c.value; });
  var label = checked.length + ' item' + (checked.length === 1 ? '' : 's');
  var dest = prompt('Move ' + label + ' to directory:', {{.Dir}});
  if (!dest) return;
  var promises = [];
  if (filePaths.length > 0)
    promises.push(fetch('/api/file/move', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({paths: filePaths, dest_dir: dest})}).then(function(r) { return r.json(); }));
  if (dirPaths.length > 0)
    promises.push(fetch('/api/folder/move', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({paths: dirPaths, dest_dir: dest})}).then(function(r) { return r.json(); }));
  Promise.all(promises)
    .then(function(results) {
      var errs = [];
      results.forEach(function(res) { if (res.errors && res.errors.length) errs = errs.concat(res.errors); });
      if (errs.length) alert('Some items failed:\n' + errs.join('\n'));
      location.reload();
    })
    .catch(function(e) { alert('Move failed: ' + e); });
}
function createFolder() {
  var name = prompt('New folder name:');
  if (!name) return;
  fetch('/api/folder/create', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({dir: {{.Dir}}, name: name})})
    .then(function(r) {
      if (!r.ok) return r.text().then(function(t) { alert('Create folder failed: ' + t); });
      location.reload();
    });
}
function uploadFiles(files) {
  if (!files || files.length === 0) return;
  var btn = document.getElementById('upload-btn');
  var status = document.getElementById('upload-status');
  btn.style.pointerEvents = 'none';
  btn.style.opacity = '0.6';
  status.style.display = 'inline';
  status.textContent = 'Uploading…';
  var form = new FormData();
  for (var i = 0; i < files.length; i++) form.append('files', files[i]);
  var xhr = new XMLHttpRequest();
  xhr.open('POST', '/api/file/upload?dir=' + encodeURIComponent({{.Dir}}));
  xhr.upload.onprogress = function(e) {
    if (e.lengthComputable) status.textContent = 'Uploading ' + Math.round(e.loaded / e.total * 100) + '%…';
  };
  xhr.onload = function() {
    if (xhr.status !== 200) {
      alert('Upload failed: ' + xhr.responseText);
      btn.style.pointerEvents = '';
      btn.style.opacity = '';
      status.style.display = 'none';
      document.getElementById('upload-input').value = '';
      return;
    }
    var res = JSON.parse(xhr.responseText);
    if (res.errors && res.errors.length) alert('Some files failed:\n' + res.errors.join('\n'));
    location.reload();
  };
  xhr.onerror = function() {
    alert('Upload failed');
    btn.style.pointerEvents = '';
    btn.style.opacity = '';
    status.style.display = 'none';
    document.getElementById('upload-input').value = '';
  };
  xhr.send(form);
}
// Build sorted media file list for dir auto-advance
var _folderLoop = false;
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
  var btn = document.getElementById('btn-play-all');
  if (btn && arr.length > 0) btn.style.display = '';
})();
function playFolderAll() {
  if (!window.dirMediaFiles || window.dirMediaFiles.length === 0) return;
  if (MOBILE && window.dirMediaFiles[0].type === 'audio') {
    location.href = '/folder/play?file=' + encodeURIComponent(window.dirMediaFiles[0].path);
    return;
  }
  _folderLoop = true;
  var btn = document.getElementById('btn-play-all');
  if (btn) { btn.textContent = '⟳ Looping'; btn.classList.add('btn-edit'); btn.classList.remove('btn-primary'); }
  openPreview({dataset: window.dirMediaFiles[0]}, true);
}
(function() {
  var seen = {}, arr = [];
  document.querySelectorAll('[data-type="photo"]').forEach(function(el) {
    var p = el.dataset.path;
    if (p && !seen[p]) { seen[p] = true; arr.push({path: p, name: el.dataset.name || '', type: 'photo'}); }
  });
  arr.sort(function(a, b) { return a.name.toLowerCase().localeCompare(b.name.toLowerCase()); });
  window.dirPhotoFiles = arr;
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
function loadFavStates() {
  fetch('/favorites/list').then(function(r){ return r.json(); }).then(function(paths) {
    var set = new Set(paths);
    document.querySelectorAll('.fav-btn').forEach(function(btn) {
      var active = set.has(btn.dataset.path);
      btn.classList.toggle('active', active);
      btn.textContent = active ? '★' : '☆';
    });
  }).catch(function(){});
}
function toggleFav(btn) {
  var path = btn.dataset.path;
  var isFolder = btn.dataset.folder === '1';
  fetch('/favorites/toggle', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({path: path, is_folder: isFolder})
  }).then(function(r){ return r.json(); }).then(function(res) {
    btn.classList.toggle('active', res.favorited);
    btn.textContent = res.favorited ? '★' : '☆';
  }).catch(function(){});
}
loadFavStates();
// Apply saved sort preference if URL has no sort param
(function() {
  try {
    var urlSort = new URLSearchParams(window.location.search).get('sort');
    if (urlSort === null) {
      var saved = _getStoredSort();
      if (saved === 'date') {
        var p = new URLSearchParams(window.location.search);
        p.set('sort', 'date');
        window.location.replace('/browse?' + p.toString());
      }
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
<tr class="file-row" data-dir="{{.Dir}}" style="cursor:pointer" onclick="browseDir(this)">
  <td>{{.Filename}}</td>
  <td><span class="badge badge-{{.FileType}}">{{upper .FileType}}</span></td>
  <td class="muted" style="font-size:12px;font-family:monospace">{{.Dir}}</td>
  <td class="muted">{{.UpdatedAt}}</td>
  <td>{{if and (or (eq .FileType "video") (eq .FileType "audio")) (gt .WatchCount 0)}}<span class="badge badge-{{.FileType}}">{{.WatchCount}}×</span>{{else}}<span class="muted">—</span>{{end}}</td>
</tr>
{{end}}
</tbody>
</table>
</div>
</div>
<div id="view-grid" class="view-grid" style="display:none">
{{range .Items}}
{{if eq .FileType "video"}}
<div class="grid-card" data-dir="{{.Dir}}" onclick="browseDir(this)">
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
<div class="grid-card" data-dir="{{.Dir}}" onclick="browseDir(this)">
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
  var list = document.getElementById('view-list');
  var grid = document.getElementById('view-grid');
  if (!list || !grid) return;
  list.style.display = v === 'list' ? '' : 'none';
  grid.style.display = v === 'grid' ? 'grid' : 'none';
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
(function() {
  var seen = {}, arr = [];
  document.querySelectorAll('[data-type="photo"]').forEach(function(el) {
    var p = el.dataset.path;
    if (p && !seen[p]) { seen[p] = true; arr.push({path: p, name: el.dataset.name || '', type: 'photo'}); }
  });
  window.dirPhotoFiles = arr;
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
  </div>
  <button class="btn btn-primary btn-sm" onclick="showNewPl()">+ New</button>
</div>
<div id="new-pl-form" style="display:none;margin-bottom:16px">
  <form action="/playlists" method="post" style="display:flex;gap:8px;align-items:center;flex-wrap:wrap">
    <input id="new-pl-name" type="text" name="name" placeholder="Playlist name" style="flex:1;min-width:160px;padding:6px 10px;background:var(--bg);border:1px solid var(--border);border-radius:6px;color:var(--fg);font-size:14px;font-family:inherit">
    <button class="btn btn-primary btn-sm" type="submit">Create</button>
    <button class="btn btn-edit btn-sm" type="button" onclick="hideNewPl()">Cancel</button>
  </form>
</div>
<script>
function showNewPl() { document.getElementById('new-pl-form').style.display='block'; document.getElementById('new-pl-name').focus(); }
function hideNewPl() { document.getElementById('new-pl-form').style.display='none'; }
</script>
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
{{end}}`

// plSharedJS contains playlist player functions shared between the playlist
// detail page and the favorites page. Callers must define PLAYLIST_ITEMS,
// plCurrentIdx, getPlMedia, plLastSave, and savePlState before including this.
// Shared player transport bar, spliced into the four playlist-engine
// templates (playlist detail, top played, folder play, favorites).
const plTransportHTML = `<div class="pl-transport-btns">
          <button id="pl-shuffle-btn" class="pl-mode-btn" onclick="plToggleShuffle()" title="Shuffle">&#128256;</button>
          <button class="pl-nav-btn" onclick="plPrev()" title="Previous">&#9664;&#9664;</button>
          <button id="pl-play-btn" onclick="plTogglePlay()" title="Play / Pause">&#9654;</button>
          <button class="pl-nav-btn" onclick="plNext()" title="Next">&#9654;&#9654;</button>
          <button id="pl-repeat-btn" class="pl-mode-btn" onclick="plCycleRepeat()" title="Repeat">&#128257;</button>
        </div>`

const plSharedJS = `
function plPlay(el) { var p = el.play(); if (p && p.catch) p.catch(function(e){ plLog('play rejected: ' + e.name); }); }
// --- Shuffle & repeat ---
var plShuffle = false; try { plShuffle = localStorage.getItem('fb_pl_shuffle') === '1'; } catch(e) {}
var plRepeat = 'all'; try { plRepeat = localStorage.getItem('fb_pl_repeat') || 'all'; } catch(e) {}
var _shufOrder = [];
function plShufRegen(first) {
  var n = PLAYLIST_ITEMS ? PLAYLIST_ITEMS.length : 0;
  _shufOrder = [];
  for (var i = 0; i < n; i++) _shufOrder.push(i);
  for (var i = n - 1; i > 0; i--) {
    var j = Math.floor(Math.random() * (i + 1));
    var t = _shufOrder[i]; _shufOrder[i] = _shufOrder[j]; _shufOrder[j] = t;
  }
  // Anchor the current track first so playback continues from it.
  if (typeof first === 'number' && first >= 0 && n > 1) {
    var p = _shufOrder.indexOf(first);
    if (p > 0) { _shufOrder[p] = _shufOrder[0]; _shufOrder[0] = first; }
  }
}
// Next queue index after "from", or -1 to stop. The order array self-heals on
// queue mutations (length change or unknown index). Manual skips ignore
// repeat-one and wrap even when repeat is off.
function plNextIdx(from, manual) {
  var n = PLAYLIST_ITEMS ? PLAYLIST_ITEMS.length : 0;
  if (!n) return -1;
  if (plRepeat === 'one' && !manual) return from;
  if (!plShuffle) {
    var nx = from + 1;
    if (nx < n) return nx;
    return (plRepeat === 'off' && !manual) ? -1 : 0;
  }
  if (_shufOrder.length !== n || _shufOrder.indexOf(from) < 0) plShufRegen(from);
  var p = _shufOrder.indexOf(from);
  if (p + 1 < n) return _shufOrder[p + 1];
  if (plRepeat === 'off' && !manual) return -1;
  plShufRegen();
  // Don't let the new cycle open with the track that just finished.
  if (n > 1 && _shufOrder[0] === from) {
    var k = 1 + Math.floor(Math.random() * (n - 1));
    _shufOrder[0] = _shufOrder[k]; _shufOrder[k] = from;
  }
  return _shufOrder[0];
}
function plPrevIdx(from) {
  var n = PLAYLIST_ITEMS ? PLAYLIST_ITEMS.length : 0;
  if (!n) return -1;
  if (!plShuffle) return (from - 1 + n) % n;
  if (_shufOrder.length !== n || _shufOrder.indexOf(from) < 0) plShufRegen(from);
  var p = _shufOrder.indexOf(from);
  return _shufOrder[(p - 1 + n) % n];
}
function plModeBtns() {
  var s = document.getElementById('pl-shuffle-btn');
  if (s) s.classList.toggle('active', plShuffle);
  var r = document.getElementById('pl-repeat-btn');
  if (r) {
    r.classList.toggle('active', plRepeat !== 'off');
    r.innerHTML = plRepeat === 'one' ? '&#128258;' : '&#128257;';
    r.title = 'Repeat: ' + plRepeat;
  }
}
function _plModeChanged() {
  // Repeat-off may have marked the MSE stream finished; allow it to resume
  // scheduling if the stream is still open.
  if (_mse && _mse.noMore && _mse.ms && _mse.ms.readyState === 'open') _mse.noMore = false;
  plModeBtns();
}
function plToggleShuffle() {
  plShuffle = !plShuffle;
  try { localStorage.setItem('fb_pl_shuffle', plShuffle ? '1' : '0'); } catch(e) {}
  if (plShuffle) plShufRegen(plCurrentIdx);
  _plModeChanged();
  plLog('shuffle ' + plShuffle);
}
function plCycleRepeat() {
  plRepeat = plRepeat === 'off' ? 'all' : (plRepeat === 'all' ? 'one' : 'off');
  try { localStorage.setItem('fb_pl_repeat', plRepeat); } catch(e) {}
  _plModeChanged();
  plLog('repeat ' + plRepeat);
}
var _PL_MSE = null;
function plMseOk() {
  if (!MOBILE) return false;
  if (_PL_MSE === null) {
    try { _PL_MSE = !!window.MediaSource && !!MediaSource.isTypeSupported && MediaSource.isTypeSupported('audio/aac'); } catch(e) { _PL_MSE = false; }
  }
  var lt = true; try { lt = localStorage.getItem('fb_lossless_transcode_enabled') !== '0'; } catch(e) {}
  return _PL_MSE && lt;
}
var _plPreload = {path: null, blobUrl: null, blob: null, ctrl: null};
var _plPreloadSkip = {};
var _plActiveBlobUrl = null;
function plAudioUrl(item) {
  var _ltEnabled = true; try { _ltEnabled = localStorage.getItem('fb_lossless_transcode_enabled') !== '0'; } catch(e) {}
  var _kbps = '256'; try { _kbps = localStorage.getItem('fb_lossless_audio_kbps') || '256'; } catch(e) {}
  var tUrl = '/transcode/stream?path=' + encodeURIComponent(item.Path) + '&audio_kbps=' + _kbps;
  if (plMseOk()) return tUrl;
  var _losslessP = {'.flac':1,'.wav':1,'.aiff':1,'.alac':1,'.ape':1};
  var _extP = item.Path.slice(item.Path.lastIndexOf('.')).toLowerCase();
  if (MOBILE && !!_losslessP[_extP] && _ltEnabled) return tUrl;
  return '/file?path=' + encodeURIComponent(item.Path);
}
function plStartPreload(idx, attempt) {
  if (!PLAYLIST_ITEMS || !PLAYLIST_ITEMS.length) return;
  idx = ((idx % PLAYLIST_ITEMS.length) + PLAYLIST_ITEMS.length) % PLAYLIST_ITEMS.length;
  var item = PLAYLIST_ITEMS[idx];
  if (item.FileType !== 'audio') return;
  var url = plAudioUrl(item);
  if (_plPreloadSkip[url]) return;
  if (_plPreload.path === url && (_plPreload.blobUrl || _plPreload.ctrl)) return;
  if (_plPreload.ctrl) _plPreload.ctrl.abort();
  if (_plPreload.blobUrl) URL.revokeObjectURL(_plPreload.blobUrl);
  plLog('preload start idx=' + idx);
  var ctrl = new AbortController();
  _plPreload = {path: url, blobUrl: null, blob: null, ctrl: ctrl};
  fetch(url, {signal: ctrl.signal})
    .then(function(r) {
      if (!r.ok) throw new Error('http ' + r.status);
      var len = parseInt(r.headers.get('Content-Length') || '0', 10);
      if (len > 50 * 1024 * 1024) { _plPreloadSkip[url] = true; return null; }
      return r.blob();
    })
    .then(function(blob) {
      if (_plPreload.ctrl !== ctrl) return;
      if (!blob) { _plPreload = {path: null, blobUrl: null, blob: null, ctrl: null}; ctrl.abort(); return; }
      _plPreload.blob = blob;
      _plPreload.blobUrl = URL.createObjectURL(blob);
      _plPreload.ctrl = null;
      plLog('preload ready idx=' + idx + ' bytes=' + blob.size);
    })
    .catch(function() {
      if (_plPreload.ctrl !== ctrl) return;
      _plPreload = {path: null, blobUrl: null, blob: null, ctrl: null};
      plLog('preload failed idx=' + idx + ' attempt=' + (attempt || 0));
      // Retry while audio is playing: the network is unthrottled then, so a
      // transient failure must not permanently disable preload for this track.
      var a = document.getElementById('pl-audio');
      if ((attempt || 0) < 5 && a && !a.paused) {
        setTimeout(function() { plStartPreload(idx, (attempt || 0) + 1); }, 5000);
      }
    });
}
function _useBlobOrFallback(fileUrl, timeoutMs, cb) {
  if (_plPreload.path === fileUrl && _plPreload.blobUrl) {
    var b = _plPreload.blobUrl;
    _plActiveBlobUrl = b; _plPreload = {path: null, blobUrl: null, blob: null, ctrl: null};
    cb(b); return;
  }
  if (_plPreload.path === fileUrl && _plPreload.ctrl) {
    var deadline = Date.now() + timeoutMs;
    var t = setInterval(function() {
      if (_plPreload.path !== fileUrl) { clearInterval(t); cb(fileUrl); return; }
      if (_plPreload.blobUrl) {
        clearInterval(t);
        var b = _plPreload.blobUrl;
        _plActiveBlobUrl = b; _plPreload = {path: null, blobUrl: null, blob: null, ctrl: null};
        cb(b);
      } else if (Date.now() >= deadline || !_plPreload.ctrl) {
        clearInterval(t); cb(fileUrl);
      }
    }, 50);
  } else { cb(fileUrl); }
}
// --- MSE gapless engine (mobile) ---
// One continuous MediaSource feeds the audio element for the whole listening
// session; each next track's AAC bytes are appended into the same
// SourceBuffer. The element never stops, changes src, or needs a new play()
// at track boundaries — which Android Chrome can refuse from a background
// tab — so screen-off playback survives transitions.
var _mse = null;
function plMseTeardown() {
  if (!_mse) return;
  if (_mse.objUrl) { try { URL.revokeObjectURL(_mse.objUrl); } catch(e) {} }
  _mse = null;
}
function _mseInfo() {
  var a = document.getElementById('pl-audio');
  if (!a) return '';
  var s = ' t=' + (a.currentTime || 0).toFixed(2) + ' rs=' + a.readyState + ' paused=' + a.paused;
  try {
    var st = _mse;
    if (st && st.sb && st.sb.buffered.length) {
      s += ' buf=' + st.sb.buffered.start(0).toFixed(2) + '-' + st.sb.buffered.end(st.sb.buffered.length - 1).toFixed(2);
    }
  } catch(e) {}
  return s;
}
function plMseFetch(st, url, tag, cb) {
  function take() {
    plLog('mse fetch preload-hit ' + tag);
    var blob = _plPreload.blob, bu = _plPreload.blobUrl;
    _plPreload = {path: null, blobUrl: null, blob: null, ctrl: null};
    if (bu) { try { URL.revokeObjectURL(bu); } catch(e) {} }
    blob.arrayBuffer().then(cb, function(){ cb(null); });
  }
  function direct() {
    plLog('mse fetch direct ' + tag);
    fetch(url).then(function(r){ if (!r.ok) throw new Error('http ' + r.status); return r.arrayBuffer(); }).then(cb, function(){ cb(null); });
  }
  if (_plPreload.path === url && _plPreload.blob) { take(); return; }
  if (_plPreload.path === url && _plPreload.ctrl) {
    plLog('mse fetch wait-preload ' + tag);
    var deadline = Date.now() + 20000;
    var t = setInterval(function() {
      if (_mse !== st) { clearInterval(t); return; }
      if (_plPreload.path !== url) { clearInterval(t); direct(); return; }
      if (_plPreload.blob) { clearInterval(t); take(); return; }
      if (Date.now() >= deadline || !_plPreload.ctrl) { clearInterval(t); direct(); }
    }, 50);
    return;
  }
  direct();
}
function _msePump(st) {
  if (_mse !== st || !st.sb) return;
  var sb = st.sb;
  if (sb.updating) return;
  try { if (sb.buffered.length) st.end = sb.buffered.end(sb.buffered.length - 1); } catch(e) {}
  if (!st.q.length) return;
  var a = document.getElementById('pl-audio');
  var op = st.q[0];
  if (op.mark) { op.mark.start = st.end; st.q.shift(); _msePump(st); return; }
  if (op.done) {
    op.done.dur = st.end - op.done.start;
    st.bounds.push(op.done);
    st.q.shift();
    plLog('mse appended idx=' + op.done.idx + ' dur=' + op.done.dur.toFixed(1) + _mseInfo());
    if (op.cb) op.cb();
    _msePump(st);
    return;
  }
  try {
    sb.appendBuffer(op.buf);
    st.q.shift();
  } catch(e) {
    if (e.name === 'QuotaExceededError') {
      var cur = a ? a.currentTime : 0;
      plLog('mse quota evict t=' + cur.toFixed(0));
      try { sb.remove(0, Math.max(0.1, cur - 10)); } catch(e2) { st.q.shift(); plLog('mse evict failed ' + e2.name); }
    } else {
      st.q.shift();
      plLog('mse append error ' + e.name);
    }
  }
}
function plMseAppendTrack(st, idx, onBuffered) {
  if (!PLAYLIST_ITEMS.length) return;
  idx = ((idx % PLAYLIST_ITEMS.length) + PLAYLIST_ITEMS.length) % PLAYLIST_ITEMS.length;
  var item = PLAYLIST_ITEMS[idx];
  if (item.FileType !== 'audio') { st.noMore = true; plLog('mse noMore: idx=' + idx + ' is ' + item.FileType); return; }
  st.fetchingIdx = idx;
  plMseFetch(st, plAudioUrl(item), 'idx=' + idx, function(buf) {
    if (_mse !== st) return;
    st.fetchingIdx = -1;
    if (!buf || !buf.byteLength) {
      st.nextTryAt = Date.now() + 5000;
      plLog('mse fetch failed idx=' + idx);
      return;
    }
    var bound = {idx: idx, start: -1, dur: 0};
    st.q.push({mark: bound});
    var CH = 1536 * 1024;
    for (var off = 0; off < buf.byteLength; off += CH) {
      st.q.push({buf: buf.slice(off, Math.min(off + CH, buf.byteLength))});
    }
    st.q.push({done: bound, cb: onBuffered});
    st.appendedIdx = idx;
    _msePump(st);
  });
}
function plTrackPos() {
  var st = _mse;
  var a = document.getElementById('pl-audio');
  if (!st || !st.bounds.length || !a || a.style.display === 'none') return null;
  var cur = a.currentTime, b = st.bounds[0];
  for (var i = 0; i < st.bounds.length; i++) {
    if (st.bounds[i].start <= cur + 0.05) b = st.bounds[i];
  }
  return {idx: b.idx, pos: Math.max(0, cur - b.start), dur: b.dur || 0, start: b.start};
}
function plMseTick() {
  var st = _mse;
  if (!st || !st.sb) return;
  var a = document.getElementById('pl-audio');
  if (!a) return;
  var cur = a.currentTime;
  var tp = plTrackPos();
  if (tp && tp.idx !== plCurrentIdx) {
    plCurrentIdx = tp.idx;
    var item = PLAYLIST_ITEMS[tp.idx];
    document.querySelectorAll('.pl-item').forEach(function(r, i) { r.classList.toggle('active', i === tp.idx); });
    var rows = document.querySelectorAll('.pl-item');
    if (rows[tp.idx]) { try { rows[tp.idx].scrollIntoView({block: 'nearest'}); } catch(e) {} }
    var t = document.getElementById('pl-title'); if (t) t.textContent = item.Name;
    var bdg = document.getElementById('pl-badge'); if (bdg) bdg.textContent = '';
    if (typeof updateFavSidebar === 'function') updateFavSidebar(tp.idx);
    if ('mediaSession' in navigator) {
      navigator.mediaSession.playbackState = 'playing';
      try { navigator.mediaSession.metadata = new MediaMetadata({ title: item.Name, album: 'filebrowser' }); } catch(e) {}
    }
    plLastSave = Date.now();
    savePlState();
    plLog('mse boundary idx=' + tp.idx);
  } else if (Date.now() - plLastSave > 5000 && cur > 1 && !a.paused) {
    plLastSave = Date.now();
    savePlState();
  }
  // Stuck before the buffered range (AAC priming gap or post-eviction seek):
  // Chrome will not jump even tiny unbuffered gaps in MSE on its own.
  try {
    var bb = st.sb.buffered;
    if (bb.length && cur < bb.start(0) && a.readyState < 3) {
      plLog('mse gap-jump ' + cur.toFixed(3) + ' -> ' + bb.start(0).toFixed(3));
      a.currentTime = bb.start(0) + 0.01;
    }
  } catch(e) {}
  if (st.appendedIdx >= 0 && st.fetchingIdx < 0 && !st.q.length && !st.noMore && Date.now() >= (st.nextTryAt || 0)) {
    var remain = st.end - cur;
    if (remain < 180) {
      var ni = plNextIdx(st.appendedIdx);
      if (ni < 0) {
        st.noMore = true;
        plLog('mse no next (repeat off), ending stream after idx=' + st.appendedIdx);
      } else {
        plStartPreload(ni);
        if (remain < 60) {
          plLog('mse sched append idx=' + ni + ' remain=' + remain.toFixed(1) + _mseInfo());
          plMseAppendTrack(st, ni);
        }
      }
    }
  }
  if (st.noMore && st.ms.readyState === 'open' && !st.q.length && st.fetchingIdx < 0 && !st.sb.updating && st.end > 0) {
    try { st.ms.endOfStream(); } catch(e) {}
  }
  try {
    var buf = st.sb.buffered;
    if (!st.sb.updating && !st.q.length && buf.length && buf.start(0) < cur - 90) st.sb.remove(0, cur - 60);
  } catch(e) {}
  _msePump(st);
}
function plMseBindEl(a) {
  if (a._mseBound) return;
  a._mseBound = true;
  a.addEventListener('timeupdate', plMseTick);
  a.addEventListener('ended', function() {
    if (!_mse) return;
    var n = plNextIdx(plCurrentIdx);
    plLog('mse ended, next=' + n);
    plMseTeardown();
    if (n >= 0) {
      startPlaylistItem(n, 0, true);
    } else {
      savePlState();
      if ('mediaSession' in navigator) navigator.mediaSession.playbackState = 'paused';
    }
  });
  ['waiting','stalled','error','playing','pause','seeking'].forEach(function(ev) {
    a.addEventListener(ev, function() { if (_mse) plLog('mse el ' + ev + _mseInfo()); });
  });
  a.addEventListener('waiting', function() {
    var st = _mse;
    if (!st || !st.sb) return;
    try {
      var b = st.sb.buffered;
      if (b.length && a.currentTime < b.start(0)) {
        plLog('mse gap-jump(waiting) ' + a.currentTime.toFixed(3) + ' -> ' + b.start(0).toFixed(3));
        a.currentTime = b.start(0) + 0.01;
      }
    } catch(e) {}
  });
  try {
    document.addEventListener('visibilitychange', function() { if (_mse) plLog('mse visibility' + _mseInfo()); });
  } catch(e) {}
  setInterval(plMseTick, 4000);
}
function plMseStart(idx, seekTo, autoplay) {
  var a = document.getElementById('pl-audio');
  plMseTeardown();
  var ms = new MediaSource();
  var st = {ms: ms, sb: null, q: [], bounds: [], end: 0, fetchingIdx: -1, appendedIdx: -1, noMore: false, nextTryAt: 0, objUrl: null};
  _mse = st;
  st.objUrl = URL.createObjectURL(ms);
  a.src = st.objUrl;
  plMseBindEl(a);
  ms.addEventListener('sourceopen', function() {
    if (_mse !== st) return;
    try {
      st.sb = ms.addSourceBuffer('audio/aac');
      st.sb.mode = 'sequence';
    } catch(e) {
      plLog('mse addSourceBuffer failed ' + e.name + ', falling back');
      _PL_MSE = false;
      plMseTeardown();
      startPlaylistItem(idx, seekTo, autoplay);
      return;
    }
    st.sb.addEventListener('updateend', function() { _msePump(st); });
    st.sb.addEventListener('error', function() { plLog('mse sourcebuffer error'); });
    plLog('mse sourceopen');
    plMseAppendTrack(st, idx, function() {
      if (_mse !== st) return;
      if (seekTo > 1 && Math.abs(a.currentTime - seekTo) > 1) { try { a.currentTime = seekTo; } catch(e) {} }
      try {
        var b = st.sb.buffered;
        if (b.length && a.currentTime < b.start(0)) {
          plLog('mse gap-jump(start) ' + a.currentTime.toFixed(3) + ' -> ' + b.start(0).toFixed(3));
          a.currentTime = b.start(0) + 0.01;
        }
      } catch(e) {}
      plLog('mse first-buffered ' + (autoplay ? 'play' : 'pause') + _mseInfo());
      if (autoplay) plPlay(a); else a.pause();
    });
  }, {once: true});
  if (autoplay) plPlay(a);
  plLog('mse start idx=' + idx + ' ap=' + !!autoplay + (seekTo > 1 ? ' seek=' + seekTo.toFixed(0) : ''));
}
function startPlaylistItem(idx, seekTo, autoplay) {
  if (!PLAYLIST_ITEMS || idx < 0 || idx >= PLAYLIST_ITEMS.length) return;
  plCurrentIdx = idx;
  document.querySelectorAll('.pl-item').forEach(function(r, i) { r.classList.toggle('active', i === idx); });
  var rows = document.querySelectorAll('.pl-item');
  if (rows[idx]) rows[idx].scrollIntoView({block: 'nearest'});
  var item = PLAYLIST_ITEMS[idx];
  plLog('spi idx=' + idx + ' type=' + item.FileType + ' ap=' + !!autoplay + ' seek=' + (seekTo || 0).toFixed(0) + ' mse=' + plMseOk());
  var fileUrl = '/file?path=' + encodeURIComponent(item.Path);
  var v = document.getElementById('pl-video'), a = document.getElementById('pl-audio');
  document.getElementById('pl-title').textContent = item.Name;
  document.getElementById('pl-badge').textContent = seekTo > 1 ? 'Resumed from ' + fmtTime(seekTo) : '';
  if (v.hlsInstance) { v.hlsInstance.destroy(); v.hlsInstance = null; }
  v.pause(); v.src = ''; v.style.display = 'none';
  var media;
  var mseUsed = false;
  if (_mse && (item.FileType !== 'audio' || !plMseOk())) plMseTeardown();
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
  var _losslessExts = {'.flac':1,'.wav':1,'.aiff':1,'.alac':1,'.ape':1};
  var _needsHlsExts = {'.wmv':1,'.avi':1,'.mkv':1,'.flv':1,'.mov':1};
  var _ext = item.Path.slice(item.Path.lastIndexOf('.')).toLowerCase();
  var _forceHLS = !!_needsHlsExts[_ext];
  var usesHLS = (MOBILE || _forceHLS) && item.FileType === 'video';
  var _ltOn = true; try { _ltOn = localStorage.getItem('fb_lossless_transcode_enabled') !== '0'; } catch(e) {}
  var usesTranscode = MOBILE && item.FileType === 'audio' && !!_losslessExts[_ext] && _ltOn;
  if (item.FileType === 'video' || usesHLS) {
    if (a.hlsInstance) { a.hlsInstance.destroy(); a.hlsInstance = null; }
    if (item.FileType !== 'video') { a.pause(); }
    if (item.FileType === 'video') {
      a.pause(); a.style.display = 'none'; _plUpdateAudioUI();
      v.volume = DEFAULT_VOL; v.style.display = 'block'; media = v;
      if (MOBILE || _forceHLS) {
        attachVideo(v, '/hls/playlist?path=' + encodeURIComponent(item.Path) + hlsParams(), fileUrl, startPlayback);
      } else {
        v.preload = 'auto'; v.src = fileUrl; v.load();
        v.addEventListener('loadedmetadata', function() { startPlayback(v); }, {once: true});
      }
    }
  } else if (plMseOk()) {
    media = a; a.style.display = 'block'; a.volume = DEFAULT_VOL; a.playbackRate = DEFAULT_SPEED; _plUpdateAudioUI();
    if (a.hlsInstance) { a.hlsInstance.destroy(); a.hlsInstance = null; }
    if (_plActiveBlobUrl) { URL.revokeObjectURL(_plActiveBlobUrl); _plActiveBlobUrl = null; }
    if (a._plOnEnded) { a.removeEventListener('ended', a._plOnEnded); a._plOnEnded = null; }
    if (a._plOnTU) { a.removeEventListener('timeupdate', a._plOnTU); a._plOnTU = null; }
    mseUsed = true;
    plMseStart(idx, seekTo, autoplay);
  } else if (usesTranscode) {
    var transcodeUrl = plAudioUrl(item);
    media = a; a.style.display = 'block'; a.volume = DEFAULT_VOL; a.playbackRate = DEFAULT_SPEED; _plUpdateAudioUI();
    if (a.hlsInstance) { a.hlsInstance.destroy(); a.hlsInstance = null; }
    if (_plActiveBlobUrl) { URL.revokeObjectURL(_plActiveBlobUrl); _plActiveBlobUrl = null; }
    plStartPreload(idx);
    document.getElementById('pl-badge').textContent = 'Loading…';
    _useBlobOrFallback(transcodeUrl, 20000, function(src) {
      if (plCurrentIdx !== idx) return;
      document.getElementById('pl-badge').textContent = seekTo > 1 ? 'Resumed from ' + fmtTime(seekTo) : '';
      a.src = src;
      if (seekTo > 1) {
        a.addEventListener('loadedmetadata', function() { startPlayback(a); }, {once: true});
      } else {
        plPlay(a);
      }
      var _pIdx = plNextIdx(idx);
      if (_pIdx >= 0) setTimeout(function() { if (plCurrentIdx === idx) plStartPreload(_pIdx); }, 2000);
    });
  } else {
    media = a;
    a.style.display = 'block'; a.volume = DEFAULT_VOL; a.playbackRate = DEFAULT_SPEED; _plUpdateAudioUI();
    if (a.hlsInstance) { a.hlsInstance.destroy(); a.hlsInstance = null; }
    if (_plActiveBlobUrl) { URL.revokeObjectURL(_plActiveBlobUrl); _plActiveBlobUrl = null; }
    _useBlobOrFallback(fileUrl, 10000, function(src) {
      if (plCurrentIdx !== idx) return;
      a.src = src;
      if (seekTo > 1) {
        a.addEventListener('loadedmetadata', function() { startPlayback(a); }, {once: true});
      } else {
        plPlay(a);
      }
      var _pIdx = plNextIdx(idx);
      if (_pIdx >= 0) setTimeout(function() { if (plCurrentIdx === idx) plStartPreload(_pIdx); }, 2000);
    });
  }
  setMediaSession(item.Name, {
    play:  function(){ plPlay(media); },
    pause: function(){ media.pause(); if ('mediaSession' in navigator) navigator.mediaSession.playbackState = 'paused'; },
    prev:  plPrev,
    next:  plNext,
    seekbackward: function(){ media.currentTime = Math.max(0, media.currentTime - 15); },
    seekforward:  function(){ media.currentTime = media.currentTime + 15; }
  });
  bindMediaSessionState(media);
  if (mseUsed) return;
  var advanced = false;
  var lateKicked = false;
  var plAdvance = function() {
    if (advanced || plCurrentIdx !== idx) return;
    advanced = true;
    var n = plNextIdx(idx);
    plLog('plAdvance(old path) from idx=' + idx + ' next=' + n);
    if (n < 0) {
      savePlState();
      if ('mediaSession' in navigator) navigator.mediaSession.playbackState = 'paused';
      return;
    }
    if ('mediaSession' in navigator) {
      navigator.mediaSession.playbackState = 'playing';
      var _nextItem = PLAYLIST_ITEMS[n];
      if (_nextItem) {
        try { navigator.mediaSession.metadata = new MediaMetadata({ title: _nextItem.Name, album: 'filebrowser' }); } catch(e) {}
      }
    }
    savePlState();
    startPlaylistItem(n, 0, true);
  };
  // Audio→audio reuses the same element, so listeners from the previous track
  // must be removed here or stale plAdvance closures fire a double advance.
  if (media._plOnEnded) media.removeEventListener('ended', media._plOnEnded);
  if (media._plOnTU) media.removeEventListener('timeupdate', media._plOnTU);
  media._plOnEnded = plAdvance;
  media.addEventListener('ended', plAdvance, {once: true});
  var onTU = function() {
    if (plCurrentIdx !== idx || getPlMedia() !== media) { media.removeEventListener('timeupdate', onTU); return; }
    var now = Date.now();
    if (now - plLastSave > 5000 && media.currentTime > 1) { plLastSave = now; savePlState(); }
    if (media.duration && isFinite(media.duration)) {
      if (!lateKicked && media.duration - media.currentTime < 30) { lateKicked = true; var ni = plNextIdx(idx); if (ni >= 0) plStartPreload(ni); }
      if (media.currentTime >= media.duration - 0.3) plAdvance();
    }
  };
  media._plOnTU = onTU;
  media.addEventListener('timeupdate', onTU);
}
function plPrev() { savePlState(); var n = plPrevIdx(plCurrentIdx); if (n >= 0) startPlaylistItem(n, 0, true); }
function plNext() { savePlState(); var n = plNextIdx(plCurrentIdx, true); if (n >= 0) startPlaylistItem(n, 0, true); }
function plTogglePlay() {
  var a = document.getElementById('pl-audio');
  if (!a) return;
  if (a.paused) { plPlay(a); } else { a.pause(); }
}
function _plUpdateAudioUI() {
  var a = document.getElementById('pl-audio');
  var ui = document.getElementById('pl-audio-ui');
  if (!a || !ui) return;
  var active = a.style.display !== 'none';
  ui.style.display = active ? '' : 'none';
  if (!active) return;
  var btn = document.getElementById('pl-play-btn');
  var seek = document.getElementById('pl-seek');
  var cur = document.getElementById('pl-time-cur');
  var dur = document.getElementById('pl-time-dur');
  var vol = document.getElementById('pl-vol');
  if (btn) btn.innerHTML = a.paused ? '&#9654;' : '&#9646;&#9646;';
  var tp = plTrackPos();
  var curT = tp ? tp.pos : (a.currentTime || 0);
  var durT = tp ? tp.dur : ((a.duration && isFinite(a.duration)) ? a.duration : 0);
  if (seek && durT > 0) {
    var pct = (curT / durT * 100).toFixed(2);
    seek.value = pct;
    seek.style.background = 'linear-gradient(to right,#bc60ff 0%,#bc60ff ' + pct + '%,var(--border) ' + pct + '%,var(--border) 100%)';
  }
  if (cur) cur.textContent = fmtTime(curT);
  if (dur && durT > 0) dur.textContent = fmtTime(durT);
  if (vol) {
    var vp = Math.round((a.volume || 0) * 100);
    vol.value = vp;
    vol.style.background = 'linear-gradient(to right,#58a6ff 0%,#58a6ff ' + vp + '%,var(--border) ' + vp + '%,var(--border) 100%)';
  }
  var speed = document.getElementById('pl-speed');
  if (speed) {
    // Snap to the step and round off binary floating-point noise (0.8 + 3*0.02
    // is not exactly 0.86 in JS floats).
    var rate = Math.round(Math.round(a.playbackRate / PL_SPEED_STEP) * PL_SPEED_STEP * 100) / 100;
    speed.value = rate;
    // Bipolar control: fill the band between the center (1x / normal speed)
    // and wherever the thumb sits, rather than volume's fill-from-left, so
    // it's visually obvious at a glance which direction and how far. Computed
    // generally rather than assumed-50% so it stays correct if the range is
    // ever widened to something not centered on 1x.
    var pct = (rate - PL_SPEED_MIN) / (PL_SPEED_MAX - PL_SPEED_MIN) * 100;
    var centerPct = (1 - PL_SPEED_MIN) / (PL_SPEED_MAX - PL_SPEED_MIN) * 100;
    var lo = Math.min(centerPct, pct), hi = Math.max(centerPct, pct);
    speed.style.background = 'linear-gradient(to right,var(--border) 0%,var(--border) ' + lo + '%,#58a6ff ' + lo + '%,#58a6ff ' + hi + '%,var(--border) ' + hi + '%,var(--border) 100%)';
    var pctDelta = Math.round((rate - 1) * 100);
    var pctText = pctDelta === 0 ? 'normal' : (pctDelta > 0 ? '+' : '') + pctDelta + '%';
    speed.title = 'Speed: ' + pctText;
    var label = document.getElementById('pl-speed-label');
    if (label) label.textContent = pctText;
  }
}
function plInitAudioUI() {
  var a = document.getElementById('pl-audio');
  var seek = document.getElementById('pl-seek');
  if (!a) return;
  plModeBtns();
  ['timeupdate','play','pause','loadedmetadata','durationchange'].forEach(function(ev) {
    a.addEventListener(ev, _plUpdateAudioUI);
  });
  // playbackRate set at track-start time doesn't survive: the MSE branch
  // reassigns a.src to a new MediaSource object URL afterward (plMseStart),
  // which resets playbackRate to 1 — re-assert it once the new track's
  // metadata is actually loaded, regardless of which loading path was used.
  a.addEventListener('loadedmetadata', function() { a.playbackRate = DEFAULT_SPEED; _plUpdateAudioUI(); });
  if (seek) {
    seek.addEventListener('input', function() {
      var tp = plTrackPos();
      if (tp && tp.dur > 0) {
        var pos = parseFloat(seek.value) / 100 * tp.dur;
        var target = tp.start + pos;
        var evicted = false;
        try { evicted = a.buffered.length > 0 && target < a.buffered.start(0); } catch(e) {}
        if (evicted) { startPlaylistItem(tp.idx, pos, true); }
        else { a.currentTime = target; }
        _plUpdateAudioUI();
      } else if (a.duration && isFinite(a.duration)) {
        a.currentTime = parseFloat(seek.value) / 100 * a.duration;
        _plUpdateAudioUI();
      }
    });
  }
  var vol = document.getElementById('pl-vol');
  if (vol) {
    vol.value = Math.round(DEFAULT_VOL * 100);
    var vp0 = Math.round(DEFAULT_VOL * 100);
    vol.style.background = 'linear-gradient(to right,#58a6ff 0%,#58a6ff ' + vp0 + '%,var(--border) ' + vp0 + '%,var(--border) 100%)';
    vol.addEventListener('input', function() {
      var v = parseFloat(vol.value) / 100;
      a.volume = v;
      DEFAULT_VOL = v;
      try { localStorage.setItem('fb_default_volume', v); } catch(e) {}
      _plUpdateAudioUI();
    });
  }
  var speed = document.getElementById('pl-speed');
  if (speed) {
    speed.addEventListener('input', function() {
      var rate = parseFloat(speed.value);
      a.playbackRate = rate;
      DEFAULT_SPEED = rate;
      try { localStorage.setItem('fb_default_speed', rate); } catch(e) {}
      _plUpdateAudioUI();
    });
  }
}
`

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
  <div class="pl-player">
    <div class="pl-title" id="pl-title"></div>
    <video id="pl-video" controls style="display:none"></video>
    <audio id="pl-audio" style="display:none"></audio>
    <div class="pl-audio-ui" id="pl-audio-ui" style="display:none">
      <div class="pl-seek-wrap">
        <input type="range" class="pl-seek" id="pl-seek" value="0" min="0" max="100" step="0.1">
        <div class="pl-time-row"><span id="pl-time-cur">0:00</span><span id="pl-time-dur">--:--</span></div>
      </div>
      <div class="pl-transport">
        <div class="pl-speed-wrap">
          <span class="pl-speed-icon">&#177;</span>
          <input type="range" class="pl-speed" id="pl-speed" value="1" min="0.7" max="1.3" step="0.01">
          <span class="pl-speed-label" id="pl-speed-label"></span>
        </div>
        ` + plTransportHTML + `
        <div class="pl-vol-wrap">
          <span class="pl-vol-icon">&#128266;</span>
          <input type="range" class="pl-vol" id="pl-vol" value="100" min="0" max="100" step="1">
        </div>
      </div>
    </div>
    <div class="pl-controls">
      <span class="pl-badge" id="pl-badge"></span>
    </div>
  </div>
  <div class="pl-sidebar" id="pl-sidebar">
    <div style="padding:8px 12px;border-bottom:1px solid var(--border)">
      <span style="font-size:12px;color:var(--fg-muted);font-weight:500;text-transform:uppercase;letter-spacing:0.5px">Playlist</span>
    </div>
    <div id="pl-item-list">
    {{range $i, $it := .Items}}
    <div class="pl-item{{if eq $i $.State.CurrentIndex}} active{{end}}" draggable="true" data-idx="{{$i}}" onclick="startPlaylistItem({{$i}}, 0, true)">
      <span class="pl-drag" onclick="event.stopPropagation()">&#8942;&#8942;</span>
      <span class="pl-item-name">{{$it.Name}}</span>
      <span class="badge badge-{{$it.FileType}}" style="flex-shrink:0">{{upper $it.FileType}}</span>
      <button class="btn btn-danger btn-sm" style="flex-shrink:0;padding:2px 7px" onclick="event.stopPropagation();removePlaylistItem({{$it.ID}})">&#x2715;</button>
    </div>
    {{end}}
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
  var pos = media ? media.currentTime : 0;
  var tp = (typeof plTrackPos === 'function') ? plTrackPos() : null;
  if (tp) pos = tp.pos;
  var item = PLAYLIST_ITEMS && PLAYLIST_ITEMS[plCurrentIdx];
  var trackKey = item ? 'pl:' + item.Path : null;
  var delta = trackKey ? _playDelta(trackKey, pos) : 0;
  var mediaType = item ? item.FileType : 'video';
  fetch('/playlists/' + PLAYLIST_ID + '/state', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({current_index: plCurrentIdx, position_sec: pos, delta_sec: delta, media_type: mediaType})
  });
}
` + plSharedJS + `
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
        var v = document.getElementById('pl-video'), _a = document.getElementById('pl-audio');
        if (v.hlsInstance) { v.hlsInstance.destroy(); v.hlsInstance = null; }
        v.pause(); v.src = ''; v.style.display = 'none';
        _a.pause(); _a.src = ''; _a.style.display = 'none';
      } else if (removedIdx < plCurrentIdx) {
        plCurrentIdx--;
      } else if (removedIdx === plCurrentIdx) {
        startPlaylistItem(Math.min(plCurrentIdx, PLAYLIST_ITEMS.length - 1), 0, true);
      }
    });
}
function renderPlSidebar() {
  document.getElementById('pl-item-list').innerHTML = PLAYLIST_ITEMS.map(function(item, i) {
    return '<div class="pl-item' + (i === plCurrentIdx ? ' active' : '') + '" draggable="true" data-idx="' + i + '" onclick="startPlaylistItem(' + i + ',0,true)">' +
      '<span class="pl-drag" onclick="event.stopPropagation()">&#8942;&#8942;</span>' +
      '<span class="pl-item-name">' + escHtml(item.Name) + '</span>' +
      '<span class="badge badge-' + item.FileType + '" style="flex-shrink:0">' + item.FileType.toUpperCase() + '</span>' +
      '<button class="btn btn-danger btn-sm" style="flex-shrink:0;padding:2px 7px" onclick="event.stopPropagation();removePlaylistItem(' + item.ID + ')">&#x2715;</button>' +
      '</div>';
  }).join('');
  bindPlDrag();
}
var _plDragIdx = null;
function bindPlDrag() {
  var list = document.getElementById('pl-item-list');
  list.querySelectorAll('.pl-item[draggable]').forEach(function(el) {
    el.addEventListener('dragstart', function(e) {
      _plDragIdx = parseInt(el.dataset.idx);
      setTimeout(function(){ el.classList.add('dragging'); }, 0);
      e.dataTransfer.effectAllowed = 'move';
    });
    el.addEventListener('dragend', function() {
      el.classList.remove('dragging');
      list.querySelectorAll('.pl-item').forEach(function(r){ r.classList.remove('drag-over'); });
    });
    el.addEventListener('dragover', function(e) {
      e.preventDefault();
      list.querySelectorAll('.pl-item').forEach(function(r){ r.classList.remove('drag-over'); });
      el.classList.add('drag-over');
    });
    el.addEventListener('dragleave', function(){ el.classList.remove('drag-over'); });
    el.addEventListener('drop', function(e) {
      e.preventDefault();
      el.classList.remove('drag-over');
      var toIdx = parseInt(el.dataset.idx);
      if (_plDragIdx === null || _plDragIdx === toIdx) return;
      var moved = PLAYLIST_ITEMS.splice(_plDragIdx, 1)[0];
      PLAYLIST_ITEMS.splice(toIdx, 0, moved);
      if (plCurrentIdx === _plDragIdx) plCurrentIdx = toIdx;
      else if (_plDragIdx < plCurrentIdx && toIdx >= plCurrentIdx) plCurrentIdx--;
      else if (_plDragIdx > plCurrentIdx && toIdx <= plCurrentIdx) plCurrentIdx++;
      renderPlSidebar();
      savePlaylistOrder();
    });
  });
}
function savePlaylistOrder() {
  fetch('/playlists/' + PLAYLIST_ID + '/reorder', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({order: PLAYLIST_ITEMS.map(function(it){ return it.ID; })})
  });
}
window.addEventListener('beforeunload', savePlState);
// This inline script runs during body parse, BEFORE hls.js and the base
// script (attachVideo/fmtTime) load further down the page. Defer the
// initial autostart until DOMContentLoaded so those are defined.
document.addEventListener('DOMContentLoaded', function() {
  plInitAudioUI();
  bindPlDrag();
  if (PLAYLIST_ITEMS && PLAYLIST_ITEMS.length > 0) {
    startPlaylistItem(Math.min((PLAYLIST_STATE && PLAYLIST_STATE.CurrentIndex) || 0, PLAYLIST_ITEMS.length - 1),
                     (PLAYLIST_STATE && PLAYLIST_STATE.PositionSec) || 0);
  }
});
</script>
{{end}}`

const folderPlayTmpl = `{{define "content"}}
<script>
var PLAYLIST_ID = 0;
var PLAYLIST_ITEMS = {{toJSON .Items}};
var PLAYLIST_STATE = null;
var START_IDX = {{.StartIdx}};
</script>
<div class="page-header">
  <div class="page-header-left">
    <h2>&#127925; {{.Folder}}</h2>
    <a class="btn btn-sm" href="/browse?path={{.Dir}}">Back to folder</a>
  </div>
</div>
{{if not .Items}}
<p class="muted">No audio files in this folder.</p>
{{else}}
<div class="pl-layout" id="pl-layout">
  <div class="pl-player">
    <div class="pl-title" id="pl-title"></div>
    <audio id="pl-audio" style="display:none"></audio>
    <div class="pl-audio-ui" id="pl-audio-ui" style="display:none">
      <div class="pl-seek-wrap">
        <input type="range" class="pl-seek" id="pl-seek" value="0" min="0" max="100" step="0.1">
        <div class="pl-time-row"><span id="pl-time-cur">0:00</span><span id="pl-time-dur">--:--</span></div>
      </div>
      <div class="pl-transport">
        <div class="pl-speed-wrap">
          <span class="pl-speed-icon">&#177;</span>
          <input type="range" class="pl-speed" id="pl-speed" value="1" min="0.7" max="1.3" step="0.01">
          <span class="pl-speed-label" id="pl-speed-label"></span>
        </div>
        ` + plTransportHTML + `
        <div class="pl-vol-wrap">
          <span class="pl-vol-icon">&#128266;</span>
          <input type="range" class="pl-vol" id="pl-vol" value="100" min="0" max="100" step="1">
        </div>
      </div>
    </div>
    <video id="pl-video" controls style="display:none"></video>
    <div class="pl-controls">
      <span class="pl-badge" id="pl-badge"></span>
    </div>
  </div>
  <div class="pl-sidebar" id="pl-sidebar">
    <div style="padding:8px 12px;border-bottom:1px solid var(--border)">
      <span style="font-size:12px;color:var(--fg-muted);font-weight:500;text-transform:uppercase;letter-spacing:0.5px">{{len .Items}} tracks</span>
    </div>
    <div id="pl-item-list">
    {{range $i, $it := .Items}}
    <div class="pl-item" data-idx="{{$i}}" onclick="startPlaylistItem({{$i}}, 0, true)">
      <span class="pl-item-name">{{$it.Name}}</span>
    </div>
    {{end}}
    </div>
  </div>
</div>
{{end}}
<script>
var plLastSave = 0;
var plCurrentIdx = 0;
function getPlMedia() {
  var v = document.getElementById('pl-video'), a = document.getElementById('pl-audio');
  if (v && v.style.display !== 'none') return v;
  if (a && a.style.display !== 'none') return a;
  return null;
}
function savePlState() {}
` + plSharedJS + `
document.addEventListener('DOMContentLoaded', function() {
  plInitAudioUI();
  if (PLAYLIST_ITEMS && PLAYLIST_ITEMS.length > 0) {
    startPlaylistItem(Math.min(START_IDX, PLAYLIST_ITEMS.length - 1), 0, true);
  }
});
</script>
{{end}}`

const favoritesTmpl = `{{define "content"}}
<script>
var PLAYLIST_ID = 0;
var PLAYLIST_ITEMS = {{toJSON .Tracks}};
var FAVORITE_ITEMS = {{toJSON .Items}};
var PLAYLIST_STATE = null;
</script>
<div class="page-header">
  <div class="page-header-left">
    <h2>Favorites &#9733;</h2>
  </div>
</div>
{{if not .Items}}
<p class="muted">No favorites yet. In Browse, click &#9734; on a folder or audio file to add it here.</p>
{{else}}
<div class="pl-layout" id="pl-layout">
  <div class="pl-player">
    <div class="pl-title" id="pl-title"></div>
    <video id="pl-video" controls style="display:none"></video>
    <audio id="pl-audio" style="display:none"></audio>
    <div class="pl-audio-ui" id="pl-audio-ui" style="display:none">
      <div class="pl-seek-wrap">
        <input type="range" class="pl-seek" id="pl-seek" value="0" min="0" max="100" step="0.1">
        <div class="pl-time-row"><span id="pl-time-cur">0:00</span><span id="pl-time-dur">--:--</span></div>
      </div>
      <div class="pl-transport">
        <div class="pl-speed-wrap">
          <span class="pl-speed-icon">&#177;</span>
          <input type="range" class="pl-speed" id="pl-speed" value="1" min="0.7" max="1.3" step="0.01">
          <span class="pl-speed-label" id="pl-speed-label"></span>
        </div>
        ` + plTransportHTML + `
        <div class="pl-vol-wrap">
          <span class="pl-vol-icon">&#128266;</span>
          <input type="range" class="pl-vol" id="pl-vol" value="100" min="0" max="100" step="1">
        </div>
      </div>
    </div>
    <div class="pl-controls">
      <span class="pl-badge" id="pl-badge"></span>
    </div>
  </div>
  <div class="pl-sidebar" id="pl-sidebar">
    <div style="padding:8px 12px;border-bottom:1px solid var(--border)">
      <span id="fav-track-count" style="font-size:12px;color:var(--fg-muted);font-weight:500;text-transform:uppercase;letter-spacing:0.5px">{{len .Tracks}} tracks</span>
    </div>
    <div id="pl-item-list">
    {{range $i, $it := .Items}}
    <div class="fav-item" draggable="true" data-idx="{{$i}}" onclick="startPlaylistItem({{$it.StartIdx}}, 0, true)">
      <span class="pl-drag" onclick="event.stopPropagation()">&#8942;&#8942;</span>
      <span class="fav-item-icon">{{if $it.IsFolder}}&#128193;{{else}}&#9834;{{end}}</span>
      <div class="fav-item-info">
        <span class="fav-item-name">{{$it.Name}}</span>
        <span class="fav-item-path">{{$it.Dir}}</span>
      </div>
      {{if $it.IsFolder}}<span class="fav-item-count">{{$it.TrackCount}}</span>{{end}}
      <button class="pl-unstar-btn" onclick="event.stopPropagation();plUnstarFavItem(this,{{$i}})" title="Remove from favorites">&#9733;</button>
    </div>
    {{end}}
    </div>
  </div>
</div>
{{end}}
<script>
var plLastSave = 0;
var plCurrentIdx = 0;
function getPlMedia() {
  var v = document.getElementById('pl-video'), a = document.getElementById('pl-audio');
  if (v && v.style.display !== 'none') return v;
  if (a && a.style.display !== 'none') return a;
  return null;
}
function savePlState() {}
` + plSharedJS + `
var _origSPI = startPlaylistItem;
startPlaylistItem = function(idx, seekTo, autoplay) {
  _origSPI(idx, seekTo, autoplay);
  updateFavSidebar(idx);
};
function updateFavSidebar(trackIdx) {
  var activeEl = null;
  document.querySelectorAll('#pl-item-list .fav-item').forEach(function(el, i) {
    var it = FAVORITE_ITEMS[i];
    var active = it && trackIdx >= it.StartIdx && trackIdx < it.EndIdx;
    el.classList.toggle('active', active);
    if (active) activeEl = el;
  });
  if (activeEl) activeEl.scrollIntoView({block: 'nearest'});
}
var _favDragIdx = null;
function bindFavDrag() {
  var list = document.getElementById('pl-item-list');
  list.querySelectorAll('.fav-item').forEach(function(el) {
    el.addEventListener('dragstart', function(e) {
      _favDragIdx = parseInt(el.dataset.idx);
      setTimeout(function(){ el.classList.add('dragging'); }, 0);
      e.dataTransfer.effectAllowed = 'move';
    });
    el.addEventListener('dragend', function() {
      el.classList.remove('dragging');
      list.querySelectorAll('.fav-item').forEach(function(r){ r.classList.remove('drag-over'); });
    });
    el.addEventListener('dragover', function(e) {
      e.preventDefault();
      list.querySelectorAll('.fav-item').forEach(function(r){ r.classList.remove('drag-over'); });
      el.classList.add('drag-over');
    });
    el.addEventListener('dragleave', function(){ el.classList.remove('drag-over'); });
    el.addEventListener('drop', function(e) {
      e.preventDefault();
      el.classList.remove('drag-over');
      var toIdx = parseInt(el.dataset.idx);
      if (_favDragIdx === null || _favDragIdx === toIdx) return;
      var slices = FAVORITE_ITEMS.map(function(item) {
        return PLAYLIST_ITEMS.slice(item.StartIdx, item.EndIdx);
      });
      var currentPath = PLAYLIST_ITEMS[plCurrentIdx] ? PLAYLIST_ITEMS[plCurrentIdx].Path : null;
      var movedItem = FAVORITE_ITEMS.splice(_favDragIdx, 1)[0];
      FAVORITE_ITEMS.splice(toIdx, 0, movedItem);
      var movedSlice = slices.splice(_favDragIdx, 1)[0];
      slices.splice(toIdx, 0, movedSlice);
      var pos = 0;
      PLAYLIST_ITEMS.length = 0;
      FAVORITE_ITEMS.forEach(function(item, i) {
        slices[i].forEach(function(t){ PLAYLIST_ITEMS.push(t); });
        item.StartIdx = pos;
        item.EndIdx = pos + slices[i].length;
        pos = item.EndIdx;
      });
      if (currentPath) {
        for (var i = 0; i < PLAYLIST_ITEMS.length; i++) {
          if (PLAYLIST_ITEMS[i].Path === currentPath) { plCurrentIdx = i; break; }
        }
      }
      var all = Array.from(list.querySelectorAll('.fav-item'));
      var movedEl = all.splice(_favDragIdx, 1)[0];
      all.splice(toIdx, 0, movedEl);
      all.forEach(function(n, i) {
        n.dataset.idx = i;
        (function(ii, si){ n.onclick = function(){ startPlaylistItem(si, 0, true); }; })(i, FAVORITE_ITEMS[i].StartIdx);
        var ub = n.querySelector('.pl-unstar-btn');
        if (ub) (function(ii,b){ b.onclick = function(ev){ ev.stopPropagation(); plUnstarFavItem(b,ii); }; })(i,ub);
        list.appendChild(n);
      });
      updateFavSidebar(plCurrentIdx);
      saveFavOrder();
    });
  });
}
function saveFavOrder() {
  fetch('/favorites/reorder', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({paths: FAVORITE_ITEMS.map(function(it){ return it.Path; })})
  });
}
function plUnstarFavItem(btn, itemIdx) {
  var item = FAVORITE_ITEMS[itemIdx];
  if (!item) return;
  fetch('/favorites/toggle', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({path: item.Path, is_folder: item.IsFolder})
  }).then(function(r){ return r.json(); }).then(function(res) {
    if (res.favorited) return;
    var start = item.StartIdx, end = item.EndIdx, count = end - start;
    PLAYLIST_ITEMS.splice(start, count);
    FAVORITE_ITEMS.splice(itemIdx, 1);
    for (var i = itemIdx; i < FAVORITE_ITEMS.length; i++) {
      FAVORITE_ITEMS[i].StartIdx -= count;
      FAVORITE_ITEMS[i].EndIdx -= count;
    }
    btn.closest('.fav-item').remove();
    if (FAVORITE_ITEMS.length === 0) {
      var a = document.getElementById('pl-audio');
      if (a) { a.pause(); a.src = ''; }
      document.getElementById('pl-layout').outerHTML = '<p class="muted" style="padding:24px">No favorites yet. In Browse, click &#9734; on a folder or audio file to add it here.</p>';
      return;
    }
    if (plCurrentIdx >= end) {
      plCurrentIdx -= count;
    } else if (plCurrentIdx >= start) {
      plCurrentIdx = Math.max(0, start - 1);
    }
    updateFavSidebar(plCurrentIdx);
    var hdr = document.getElementById('fav-track-count');
    if (hdr) hdr.textContent = PLAYLIST_ITEMS.length + ' tracks';
  }).catch(function(){});
}
document.addEventListener('DOMContentLoaded', function() {
  plInitAudioUI();
  bindFavDrag();
  if (PLAYLIST_ITEMS && PLAYLIST_ITEMS.length > 0) {
    startPlaylistItem(0, 0);
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
<script>try{var _t=localStorage.getItem('fb_theme');if(_t)document.documentElement.dataset.theme=_t;}catch(e){}</script>
<style>` + themeVars + `
*, *::before, *::after { box-sizing: border-box; }
body { margin: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; font-size: 14px; background: var(--bg); color: var(--fg); display: flex; align-items: center; justify-content: center; min-height: 100vh; }
.login-card { background: var(--bg-panel); border: 1px solid var(--border); border-radius: 8px; padding: 32px; width: 100%; max-width: 360px; }
.login-logo { display: flex; align-items: center; gap: 10px; font-size: 18px; font-weight: 600; color: var(--fg-strong); margin-bottom: 28px; justify-content: center; }
.form-group { margin-bottom: 16px; }
label { display: block; font-size: 13px; color: var(--fg-muted); margin-bottom: 4px; }
input[type=text], input[type=password] { width: 100%; padding: 8px 10px; background: var(--bg); border: 1px solid var(--border); border-radius: 6px; color: var(--fg); font-size: 14px; font-family: inherit; }
input:focus { outline: none; border-color: #58a6ff; }
.btn-primary { display: block; width: 100%; padding: 8px; background: #238636; border: 1px solid #2ea043; color: #fff; border-radius: 6px; font-size: 14px; font-weight: 500; cursor: pointer; margin-top: 20px; }
.btn-primary:hover { background: #2ea043; }
.error-box { color: #f85149; font-size: 13px; margin-bottom: 16px; padding: 10px 14px; background: rgba(248,81,73,0.1); border-radius: 6px; border: 1px solid rgba(248,81,73,0.3); }
</style>
</head>
<body>
<button onclick="toggleTheme()" title="Toggle light/dark theme" style="position:fixed;top:16px;right:16px;background:transparent;border:1px solid var(--border);color:var(--fg-muted);width:32px;height:32px;border-radius:6px;cursor:pointer;font-size:16px">&#9789;</button>
<script>
function toggleTheme() {
  var next = document.documentElement.dataset.theme === 'light' ? 'dark' : 'light';
  document.documentElement.dataset.theme = next;
  try { localStorage.setItem('fb_theme', next); } catch(e) {}
}
</script>
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
  </div>
  <button class="btn btn-primary btn-sm" onclick="showNewUser()">+ New</button>
</div>
<div id="new-user-form" style="display:none;margin-bottom:16px">
  <form action="/users" method="post" style="display:flex;gap:8px;align-items:center;flex-wrap:wrap">
    <input id="new-user-name" type="text" name="username" placeholder="Username" autocomplete="off" style="flex:1;min-width:140px;padding:6px 10px;background:var(--bg);border:1px solid var(--border);border-radius:6px;color:var(--fg);font-size:14px;font-family:inherit">
    <input type="password" name="password" placeholder="Password" autocomplete="new-password" style="flex:1;min-width:140px;padding:6px 10px;background:var(--bg);border:1px solid var(--border);border-radius:6px;color:var(--fg);font-size:14px;font-family:inherit">
    <button class="btn btn-primary btn-sm" type="submit">Add</button>
    <button class="btn btn-edit btn-sm" type="button" onclick="hideNewUser()">Cancel</button>
  </form>
</div>
<script>
function showNewUser() { document.getElementById('new-user-form').style.display='block'; document.getElementById('new-user-name').focus(); }
function hideNewUser() { document.getElementById('new-user-form').style.display='none'; }
</script>
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
<tr class="file-row" {{if ne .ID $.CurrentUID}}onclick="location.href='/users/{{.ID}}'" style="cursor:pointer"{{end}}>
  <td>{{.Username}}{{if eq .ID $.CurrentUID}} <span class="muted" style="font-size:11px">(you)</span>{{end}}</td>
  <td class="muted">{{.CreatedAt}}</td>
  <td class="actions-cell">
    {{if ne .ID $.CurrentUID}}
    <form class="inline" action="/users/{{.ID}}/delete" method="post" onclick="event.stopPropagation()">
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
{{end}}`

const userDetailTmpl = `{{define "content"}}
<div class="page-header">
  <div class="page-header-left">
    <a href="/users" style="color:var(--fg-muted);font-size:13px;text-decoration:none;margin-right:8px">&#8592; Users</a>
    <h2>{{.Username}}</h2>
  </div>
  <form action="/users/{{.ID}}/delete" method="post" onsubmit="return confirm('Delete user {{.Username}}?')">
    <button class="btn btn-danger btn-sm" type="submit">Delete User</button>
  </form>
</div>
<div class="section">
  <div class="section-header"><h3>Path Access</h3></div>
  {{if .AllPaths}}
  <div class="table-wrap">
  <table>
  <thead><tr><th style="width:40px">Access</th><th>Path</th></tr></thead>
  <tbody>
  {{range .AllPaths}}
  <tr>
    <td style="text-align:center">
      {{if .Granted}}
      <form class="inline" action="/paths/{{.ID}}/revoke/{{$.ID}}" method="post">
        <input type="checkbox" checked onclick="this.form.submit()" style="cursor:pointer;width:16px;height:16px" title="Revoke access">
      </form>
      {{else}}
      <form class="inline" action="/paths/{{.ID}}/grant" method="post">
        <input type="hidden" name="user_id" value="{{$.ID}}">
        <input type="checkbox" onclick="this.form.submit()" style="cursor:pointer;width:16px;height:16px" title="Grant access">
      </form>
      {{end}}
    </td>
    <td>{{.Path}}</td>
  </tr>
  {{end}}
  </tbody>
  </table>
  </div>
  {{else}}
  <p class="muted" style="padding:12px 0">No paths added yet. Add paths in Settings first.</p>
  {{end}}
</div>
{{end}}`

const trashTmpl = `{{define "content"}}
<div class="page-header">
  <div class="page-header-left">
    <h2>Trash</h2>
  </div>
  {{if .Items}}
  <form action="/trash/empty" method="post" onsubmit="return confirm('Permanently delete all {{len .Items}} item(s) in Trash? This cannot be undone.')">
    <button class="btn btn-danger btn-sm" type="submit">Empty Trash</button>
  </form>
  {{end}}
</div>
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
<p class="muted" style="margin:-4px 0 16px">Deleted files and folders wait here for 30 days before being permanently removed.</p>
{{if .Items}}
<div class="section">
<div class="table-wrap">
<table>
<thead><tr>
  <th>Name</th>
  <th>Type</th>
  <th>Original location</th>
  <th>Deleted</th>
  <th></th>
</tr></thead>
<tbody>
{{range .Items}}
<tr>
  <td>{{.Name}}</td>
  <td><span class="badge badge-{{if .IsFolder}}dir{{else}}other{{end}}">{{if .IsFolder}}FOLDER{{else}}FILE{{end}}</span></td>
  <td class="muted" style="font-size:12px">{{.OriginalPath}}</td>
  <td class="muted">{{.DeletedAt}}</td>
  <td class="actions-cell">
    <form class="inline" action="/trash/{{.ID}}/restore" method="post">
      <button class="btn btn-edit btn-sm" type="submit">Restore</button>
    </form>
    <form class="inline" action="/trash/{{.ID}}/purge" method="post" onsubmit="return confirm('Permanently delete {{.Name}}? This cannot be undone.')">
      <button class="btn btn-danger btn-sm" type="submit">Delete Forever</button>
    </form>
  </td>
</tr>
{{end}}
</tbody>
</table>
</div>
</div>
{{else}}
<p class="muted">Trash is empty.</p>
{{end}}
{{end}}`

const duplicatesTmpl = `{{define "content"}}
<div class="page-header">
  <div class="page-header-left">
    <h2>Duplicate Finder</h2>
  </div>
  <button id="dup-scan-btn" class="btn btn-primary btn-sm" onclick="startDupScan()">Scan for Duplicates</button>
</div>
<div id="dup-status" class="muted" style="margin-bottom:16px">No scan yet.</div>
<div id="dup-groups"></div>
<script>
var dupPollTimer = null;
function startDupScan() {
  document.getElementById('dup-scan-btn').disabled = true;
  document.getElementById('dup-status').textContent = 'Scanning…';
  fetch('/duplicates/scan', {method: 'POST'}).then(function() {
    if (dupPollTimer) clearInterval(dupPollTimer);
    dupPollTimer = setInterval(pollDupStatus, 2000);
  });
}
function pollDupStatus() {
  fetch('/duplicates/status').then(function(r) { return r.json(); }).then(function(data) {
    if (data.scanning) return;
    if (dupPollTimer) { clearInterval(dupPollTimer); dupPollTimer = null; }
    document.getElementById('dup-scan-btn').disabled = false;
    renderDupResult(data);
  });
}
function renderDupResult(data) {
  var status = document.getElementById('dup-status');
  var groupsEl = document.getElementById('dup-groups');
  groupsEl.textContent = '';
  if (!data.scanned_at) {
    status.textContent = 'No scan yet.';
    return;
  }
  var groups = data.groups || [];
  status.textContent = 'Last scan: ' + data.scanned_at + ' — ' + groups.length + ' duplicate group' + (groups.length === 1 ? '' : 's') + ', ' + (data.wasted || '0 B') + ' reclaimable.';
  groups.forEach(function(g, gi) {
    var box = document.createElement('div');
    box.className = 'section';
    box.dataset.group = gi;
    var header = document.createElement('div');
    header.className = 'section-header';
    var h3 = document.createElement('h3');
    h3.textContent = g.Size + ' × ' + g.Paths.length + ' copies';
    header.appendChild(h3);
    box.appendChild(header);
    g.Paths.forEach(function(p, pi) {
      var row = document.createElement('label');
      row.style.cssText = 'display:flex;align-items:center;gap:8px;padding:6px 12px;font-size:13px;border-top:1px solid var(--surface-hover)';
      var cb = document.createElement('input');
      cb.type = 'checkbox';
      cb.className = 'dup-check';
      cb.value = p;
      cb.checked = pi > 0;
      row.appendChild(cb);
      var span = document.createElement('span');
      span.textContent = p;
      row.appendChild(span);
      box.appendChild(row);
    });
    groupsEl.appendChild(box);
  });
  if (groups.length > 0) {
    var delBtn = document.createElement('button');
    delBtn.className = 'btn btn-danger btn-sm';
    delBtn.textContent = 'Delete Selected (to Trash)';
    delBtn.style.margin = '4px 0 24px';
    delBtn.onclick = deleteDupSelected;
    groupsEl.appendChild(delBtn);
  }
}
function deleteDupSelected() {
  var boxes = Array.from(document.querySelectorAll('.dup-check:checked'));
  if (boxes.length === 0) return;
  var allChecked = false;
  document.querySelectorAll('[data-group]').forEach(function(g) {
    var total = g.querySelectorAll('.dup-check').length;
    var checked = g.querySelectorAll('.dup-check:checked').length;
    if (checked === total) allChecked = true;
  });
  if (allChecked) {
    alert('Leave at least one copy unchecked in each group.');
    return;
  }
  var paths = boxes.map(function(c) { return c.value; });
  if (!confirm('Move ' + paths.length + ' duplicate file(s) to Trash?')) return;
  fetch('/api/file/delete', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({paths: paths})})
    .then(function(r) { return r.json(); })
    .then(function(res) {
      if (res.errors && res.errors.length) alert('Some items failed:\n' + res.errors.join('\n'));
      pollDupStatus();
    });
}
pollDupStatus();
</script>
{{end}}`

const settingsTmpl = `{{define "content"}}
<div class="page-header">
  <div class="page-header-left">
    <h2>Settings</h2>
  </div>
  <span id="settings-saved-toast" style="color:#3fb950;font-size:13px;opacity:0;transition:opacity 0.3s">Saved ✓</span>
</div>
{{if .PathError}}<div class="error-box">{{.PathError}}</div>{{end}}
{{if .IsAdmin}}
<div class="section">
  <div class="section-header">
    <h3>Browseable Paths</h3>
    <button class="btn btn-edit btn-sm" onclick="reindexFiles(this)">Reindex Files</button>
  </div>
  {{if .Paths}}
  <div class="table-wrap" style="margin-bottom:16px">
  <table>
  <thead><tr><th style="width:40px" title="Enabled in Browse">Active</th><th>Path</th><th style="width:80px;text-align:right">Size (GB)</th><th></th></tr></thead>
  <tbody>
  {{range .Paths}}
  <tr>
    <td style="text-align:center">
      <input type="checkbox" class="path-enabled-check" data-id="{{.ID}}"
        {{if .Enabled}}checked{{end}} title="Enable or disable this path in Browse" style="cursor:pointer;width:16px;height:16px">
    </td>
    <td><a href="{{browseURL .Path}}">{{.Path}}</a></td>
    <td class="path-size-cell" data-path="{{.Path}}" style="text-align:right;color:var(--fg-muted);font-size:13px">…</td>
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
{{else}}
<div class="section">
  <div class="section-header">
    <h3>Accessible Paths</h3>
    <button class="btn btn-edit btn-sm" onclick="reindexFiles(this)">Reindex Files</button>
  </div>
  {{if .Paths}}
  <div class="table-wrap" style="margin-bottom:16px">
  <table>
  <thead><tr><th>Path</th><th style="width:80px;text-align:right">Size (GB)</th></tr></thead>
  <tbody>
  {{range .Paths}}
  <tr>
    <td><a href="{{browseURL .Path}}">{{.Path}}</a></td>
    <td class="path-size-cell" data-path="{{.Path}}" style="text-align:right;color:var(--fg-muted);font-size:13px">…</td>
  </tr>
  {{end}}
  </tbody>
  </table>
  </div>
  {{else}}
  <p class="muted" style="padding:12px 0">No paths have been granted to your account yet.</p>
  {{end}}
</div>
{{end}}
<div class="section">
  <div class="section-header"><h3>Video Transcoding (Mobile)</h3></div>
  <div class="form-page" style="max-width:680px">
    <div class="form-group" style="flex-direction:row;align-items:center;gap:10px;border:none;padding:0;margin-bottom:16px">
      <input type="checkbox" id="cb-force-original" style="width:auto;cursor:pointer;margin:0"
             onchange="savePlaybackSettings()">
      <label for="cb-force-original" style="cursor:pointer;margin:0;font-weight:normal">Enable video transcoding on mobile</label>
    </div>
    <div class="settings-grid" style="display:grid;grid-template-columns:1fr 1fr;gap:0 24px">
      <div class="form-group">
        <label>Quality (CRF) <span class="muted" style="font-weight:normal">— lower = better, 18–28 typical</span></label>
        <input type="number" id="tc-crf" value="23" min="0" max="51"
               onchange="saveTranscodeSetting('crf', this.value)">
      </div>
      <div class="form-group">
        <label>Encode preset <span class="muted" style="font-weight:normal">— slower = smaller file</span></label>
        <select id="tc-preset" onchange="saveTranscodeSetting('preset', this.value)">
          <option value="ultrafast">ultrafast</option>
          <option value="superfast">superfast</option>
          <option value="veryfast">veryfast</option>
          <option value="faster">faster</option>
          <option value="fast">fast</option>
          <option value="medium">medium</option>
          <option value="slow">slow</option>
          <option value="slower">slower</option>
          <option value="veryslow">veryslow</option>
        </select>
      </div>
      <div class="form-group">
        <label>Max width (px) <span class="muted" style="font-weight:normal">— 0 = no limit</span></label>
        <input type="number" id="tc-max-width" value="1280" min="0" step="2"
               onchange="saveTranscodeSetting('max_width', this.value)">
      </div>
      <div class="form-group">
        <label>Segment duration (s)</label>
        <input type="number" id="tc-segment-sec" value="6" min="2" max="60"
               onchange="saveTranscodeSetting('segment_sec', this.value)">
      </div>
      <div class="form-group">
        <label>Video bitrate (kbps)</label>
        <input type="number" id="tc-video-kbps" value="3000" min="100"
               onchange="saveTranscodeSetting('video_kbps', this.value)">
      </div>
      <div class="form-group">
        <label>Audio bitrate (kbps)</label>
        <input type="number" id="tc-audio-kbps" value="128" min="32"
               onchange="saveTranscodeSetting('audio_kbps', this.value)">
      </div>
    </div>
    <p class="muted" style="font-size:12px;margin-top:8px">Changes apply to new HLS sessions immediately.</p>
  </div>
</div>
<div class="section">
  <div class="section-header"><h3>Lossless Audio Transcoding (Mobile)</h3></div>
  <div class="form-page">
    <div class="form-group" style="flex-direction:row;align-items:center;gap:10px;border:none;padding:0">
      <input type="checkbox" id="cb-lossless-transcode" style="width:auto;cursor:pointer;margin:0"
             onchange="saveLosslessSettings()">
      <label for="cb-lossless-transcode" style="cursor:pointer;margin:0;font-weight:normal">Transcode lossless audio to AAC on mobile</label>
    </div>
    <div class="form-group" style="margin-top:14px" id="lossless-kbps-row">
      <label>Bitrate (kbps)</label>
      <input type="number" id="lossless-audio-kbps" value="256" min="64" max="512" style="max-width:120px"
             onchange="saveLosslessSettings()">
    </div>
    <p class="muted" style="font-size:12px;margin-top:4px">When enabled, FLAC/WAV/AIFF/ALAC/APE are transcoded to AAC and fully buffered before playback — gapless transitions, full seeking, lower bandwidth. When disabled, the original file is served directly.</p>
  </div>
</div>
<div class="section">
  <div class="section-header"><h3>Playback</h3></div>
  <div class="form-page">
    <div class="form-group" style="flex-direction:row;align-items:center;gap:10px;border:none;padding:0">
      <label for="vol-slider" style="margin:0;white-space:nowrap">Default volume: <span id="vol-display">100</span>%</label>
      <input type="range" id="vol-slider" min="0" max="100" value="100" style="width:180px;cursor:pointer;accent-color:#58a6ff"
             oninput="document.getElementById('vol-display').textContent=this.value;document.getElementById('vol-val').value=this.value/100"
             onchange="savePlaybackSettings()">
      <input type="hidden" id="vol-val" value="1.0">
    </div>
    <p class="muted" style="font-size:12px;margin:10px 0 0">Settings are stored locally in your browser.</p>
  </div>
</div>
<script>
(function(){
  try {
    var ls = localStorage;
    document.getElementById('tc-crf').value = ls.getItem('fb_transcode_crf') || '23';
    var sel = document.getElementById('tc-preset');
    if (sel) sel.value = ls.getItem('fb_transcode_preset') || 'fast';
    document.getElementById('tc-max-width').value = ls.getItem('fb_transcode_max_width') || '1280';
    document.getElementById('tc-segment-sec').value = ls.getItem('fb_transcode_segment_sec') || '6';
    document.getElementById('tc-video-kbps').value = ls.getItem('fb_transcode_video_kbps') || '3000';
    document.getElementById('tc-audio-kbps').value = ls.getItem('fb_transcode_audio_kbps') || '128';
    var ltCb = document.getElementById('cb-lossless-transcode');
    if (ltCb) {
      ltCb.checked = ls.getItem('fb_lossless_transcode_enabled') !== '0';
      document.getElementById('lossless-kbps-row').style.display = ltCb.checked ? '' : 'none';
    }
    document.getElementById('lossless-audio-kbps').value = ls.getItem('fb_lossless_audio_kbps') || '256';
    var cb = document.getElementById('cb-force-original');
    if (cb) cb.checked = !ls.getItem('fb_force_original');
    var dv = parseFloat(ls.getItem('fb_default_volume'));
    if (isNaN(dv)) dv = 1;
    dv = Math.max(0, Math.min(1, dv));
    var sl = document.getElementById('vol-slider');
    if (sl) {
      var pct = Math.round(dv * 100);
      sl.value = pct;
      document.getElementById('vol-display').textContent = pct;
      document.getElementById('vol-val').value = dv;
    }
  } catch(e) {}
})();
function reindexFiles(btn) {
  btn.disabled = true; btn.textContent = 'Indexing…';
  fetch('/search/reindex', {method: 'POST'}).then(function() { pollReindexStatus(btn); });
}
var _reindexPoll;
function pollReindexStatus(btn) {
  clearTimeout(_reindexPoll);
  _reindexPoll = setTimeout(function() {
    fetch('/search/status').then(function(r){ return r.json(); }).then(function(s) {
      if (s.running) {
        btn.textContent = 'Indexing… ' + s.count + ' files';
        pollReindexStatus(btn);
      } else {
        btn.disabled = false;
        btn.textContent = 'Done — ' + s.count + ' files ✓';
        setTimeout(function() { btn.textContent = 'Reindex Files'; }, 3000);
      }
    });
  }, 800);
}
document.querySelectorAll('.path-enabled-check').forEach(function(cb) {
  cb.addEventListener('change', function() {
    var fd = new FormData();
    fd.append('enabled', cb.checked ? '1' : '0');
    fetch('/paths/' + cb.dataset.id + '/toggle', {method: 'POST', body: fd})
      .then(function(r) { if (!r.ok) { cb.checked = !cb.checked; } });
  });
});
var _savedTimer;
function showSavedToast() {
  var el = document.getElementById('settings-saved-toast');
  if (!el) return;
  el.style.opacity = '1';
  clearTimeout(_savedTimer);
  _savedTimer = setTimeout(function() { el.style.opacity = '0'; }, 1500);
}
function saveTranscodeSetting(key, value) {
  if (value === '') return;
  try { localStorage.setItem('fb_transcode_' + key, value); } catch(e) {}
  showSavedToast();
}
function saveLosslessSettings() {
  var enabled = document.getElementById('cb-lossless-transcode').checked;
  var kbps = document.getElementById('lossless-audio-kbps').value;
  try {
    localStorage.setItem('fb_lossless_transcode_enabled', enabled ? '1' : '0');
    if (kbps) localStorage.setItem('fb_lossless_audio_kbps', kbps);
  } catch(e) {}
  document.getElementById('lossless-kbps-row').style.display = enabled ? '' : 'none';
  showSavedToast();
}
function savePlaybackSettings() {
  var enableTranscode = document.getElementById('cb-force-original').checked;
  var dv = parseFloat(document.getElementById('vol-val').value);
  if (isNaN(dv)) dv = 1;
  try {
    enableTranscode ? localStorage.removeItem('fb_force_original') : localStorage.setItem('fb_force_original', '1');
    localStorage.setItem('fb_default_volume', String(dv));
  } catch(e) {}
  showSavedToast();
}
document.querySelectorAll('.path-size-cell').forEach(function(cell) {
  var p = cell.dataset.path;
  fetch('/api/path-size?path=' + encodeURIComponent(p))
    .then(function(r) { return r.json(); })
    .then(function(d) { cell.textContent = d.gb.toFixed(1); })
    .catch(function() { cell.textContent = '—'; });
});
</script>
{{end}}`

const statsTmpl = `{{define "content"}}
<h2 style="margin:0 0 2px">Stats</h2>
<div class="summary" style="margin-bottom:16px">Play time and most played, last 30 days</div>

<div style="display:flex;gap:12px;flex-wrap:wrap;margin-bottom:20px">
  <div style="background:var(--bg-panel);border:1px solid var(--border);border-radius:6px;padding:12px 16px;min-width:120px">
    <div class="muted" style="font-size:12px;margin-bottom:6px">Today</div>
    <div style="color:#58a6ff;font-size:14px">&#9654; {{fmtDur .Totals.TodayVideo}}</div>
    <div style="color:#bc60ff;font-size:14px">&#9834; {{fmtDur .Totals.TodayAudio}}</div>
  </div>
  <div style="background:var(--bg-panel);border:1px solid var(--border);border-radius:6px;padding:12px 16px;min-width:120px">
    <div class="muted" style="font-size:12px;margin-bottom:6px">Last 7 days</div>
    <div style="color:#58a6ff;font-size:14px">&#9654; {{fmtDur .Totals.WeekVideo}}</div>
    <div style="color:#bc60ff;font-size:14px">&#9834; {{fmtDur .Totals.WeekAudio}}</div>
  </div>
  <div style="background:var(--bg-panel);border:1px solid var(--border);border-radius:6px;padding:12px 16px;min-width:120px">
    <div class="muted" style="font-size:12px;margin-bottom:6px">Last 30 days</div>
    <div style="color:#58a6ff;font-size:14px">&#9654; {{fmtDur .Totals.MonthVideo}}</div>
    <div style="color:#bc60ff;font-size:14px">&#9834; {{fmtDur .Totals.MonthAudio}}</div>
  </div>
  <div style="background:var(--bg-panel);border:1px solid var(--border);border-radius:6px;padding:12px 16px;min-width:120px">
    <div class="muted" style="font-size:12px;margin-bottom:6px">All time</div>
    <div style="color:#58a6ff;font-size:14px">&#9654; {{fmtDur .Totals.AllVideo}}</div>
    <div style="color:#bc60ff;font-size:14px">&#9834; {{fmtDur .Totals.AllAudio}}</div>
  </div>
</div>

{{if .HasPlay}}
<div style="background:var(--bg-panel);border:1px solid var(--border);border-radius:6px;padding:16px;margin-bottom:20px">
  <div style="display:flex;align-items:flex-end;height:180px;gap:3px">
    {{range .Days}}
    <div style="flex:1;display:flex;flex-direction:column;justify-content:flex-end;height:100%;min-width:0" title="{{.Label}} — video: {{fmtDur .VideoSec}}, audio: {{fmtDur .AudioSec}}">
      <div style="height:{{.AudioPct}}%;background:#bc60ff;border-radius:2px 2px 0 0"></div>
      <div style="height:{{.VideoPct}}%;background:#58a6ff"></div>
    </div>
    {{end}}
  </div>
  <div class="stats-ticks" style="display:flex;gap:3px;margin-top:6px">
    {{range .Days}}
    <div style="flex:1;text-align:center;font-size:9px;color:var(--fg-muted);white-space:nowrap;min-width:0">{{.Tick}}</div>
    {{end}}
  </div>
  <div style="display:flex;gap:16px;margin-top:10px;font-size:12px">
    <span style="color:#58a6ff">&#9632; Video</span>
    <span style="color:#bc60ff">&#9632; Audio</span>
  </div>
</div>
{{else}}
<p class="muted">No play time recorded in the last 30 days.</p>
{{end}}

<div style="display:flex;gap:20px;flex-wrap:wrap;align-items:flex-start">
  <div style="flex:1;min-width:300px">
    <h3 style="margin:0 0 8px">Most played</h3>
    {{if not .TopItems}}
    <p class="muted">Nothing played yet.</p>
    {{else}}
    <div class="table-wrap">
    <table>
    <thead><tr><th>Name</th><th>Type</th><th>Plays</th></tr></thead>
    <tbody>
    {{range .TopItems}}
    <tr>
      <td>{{if eq .FileType "audio"}}<a href="{{playURL .Path}}">{{.Name}}</a>{{else}}<a href="{{browseURL (dirOf .Path)}}">{{.Name}}</a>{{end}}</td>
      <td><span class="badge badge-{{.FileType}}">{{upper .FileType}}</span></td>
      <td><span class="badge badge-{{.FileType}}">{{.WatchCount}}&#215;</span></td>
    </tr>
    {{end}}
    </tbody>
    </table>
    </div>
    {{end}}
  </div>
  <div style="flex:1;min-width:300px">
    <h3 style="margin:0 0 8px">Recently completed</h3>
    {{if not .RecentDone}}
    <p class="muted">Nothing completed yet.</p>
    {{else}}
    <div class="table-wrap">
    <table>
    <thead><tr><th>Name</th><th>Type</th><th>When</th></tr></thead>
    <tbody>
    {{range .RecentDone}}
    <tr class="file-row" data-dir="{{.Dir}}" style="cursor:pointer" onclick="browseDir(this)">
      <td>{{.Filename}}</td>
      <td><span class="badge badge-{{.FileType}}">{{upper .FileType}}</span></td>
      <td class="muted">{{.UpdatedAt}}</td>
    </tr>
    {{end}}
    </tbody>
    </table>
    </div>
    {{end}}
  </div>
</div>
{{end}}`
