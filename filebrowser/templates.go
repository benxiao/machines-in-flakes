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

type BrowseRootPage struct {
	ActiveTab string
	Cards     []RootCard
}

type RootCard struct {
	ID   int64
	Path string
}

type BrowseDirPage struct {
	ActiveTab     string
	Dir           string
	DirName       string
	Breadcrumbs   []Breadcrumb
	Subdirs       []SubdirRow
	Files         []FileRow
	PlaylistsJSON template.JS
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
	AbsPath string
	Name    string
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

type PathRow struct {
	ID   int64
	Path string
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
	add("browse_root", browseRootTmpl)
	add("browse_dir", browseDirTmpl)
	add("paths", pathsTmpl)
	add("playlists", playlistsTmpl)
	add("playlist_detail", playlistDetailTmpl)
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
.pl-layout { display: flex; gap: 16px; align-items: flex-start; }
.pl-sidebar { width: 32%; min-width: 200px; max-height: 80vh; overflow-y: auto; border: 1px solid #30363d; border-radius: 6px; }
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
  <a href="/playlists"  {{if eq .ActiveTab "playlists"}}class="active"{{end}}>Playlists</a>
  <a href="/paths"      {{if eq .ActiveTab "paths"}}class="active"{{end}}>Paths</a>
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
      <img id="modal-img" src="" alt="" style="display:none">
      <video id="modal-video" controls style="display:none;max-width:90vw;max-height:78vh"></video>
      <audio id="modal-audio" controls style="display:none;width:90vw;max-width:600px"></audio>
      <iframe id="modal-pdf" src="" style="display:none"></iframe>
      <pre id="modal-text" style="display:none"></pre>
    </div>
    <div id="modal-media-controls" style="display:none;justify-content:center;align-items:center;gap:12px;padding:10px;background:#0d1117;border-top:1px solid #30363d;flex-shrink:0">
      <button id="modal-seek-back" class="btn btn-edit btn-sm" onclick="seekActiveMedia(-15)">&#9664;&#9664; 15s</button>
      <button id="modal-seek-fwd" class="btn btn-edit btn-sm" onclick="seekActiveMedia(15)">15s &#9654;&#9654;</button>
      <span id="modal-resume-badge" style="color:#8b949e;font-size:12px;margin-left:8px"></span>
    </div>
  </div>
</div>
<script src="https://cdn.jsdelivr.net/npm/hls.js@1.4"></script>
<script>
var modal = document.getElementById('preview-modal');
function browseDir(el) {
  window.location = '/browse?dir=' + encodeURIComponent(el.dataset.dir);
}
function attachVideo(videoEl, hlsUrl, directUrl) {
  if (videoEl.hlsInstance) { videoEl.hlsInstance.destroy(); videoEl.hlsInstance = null; }
  if (typeof Hls !== 'undefined' && Hls.isSupported()) {
    var hls = new Hls();
    hls.on(Hls.Events.ERROR, function(event, data) {
      if (data.fatal) { hls.destroy(); videoEl.src = directUrl; videoEl.load(); }
    });
    hls.loadSource(hlsUrl);
    hls.attachMedia(videoEl);
    videoEl.hlsInstance = hls;
  } else if (videoEl.canPlayType('application/vnd.apple.mpegurl')) {
    videoEl.src = hlsUrl; videoEl.load();
  } else {
    videoEl.src = directUrl; videoEl.load();
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
function attachMediaResume(mediaEl, path, badge, countWord) {
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
  }, {once: true});
}
function openPreview(el) {
  var path = el.dataset.path;
  var name = el.dataset.name;
  var type = el.dataset.type;
  var fileUrl = '/file?path=' + encodeURIComponent(path);
  document.getElementById('modal-title').textContent = name;
  var img   = document.getElementById('modal-img');
  var video = document.getElementById('modal-video');
  var audio = document.getElementById('modal-audio');
  var pdf   = document.getElementById('modal-pdf');
  var txt   = document.getElementById('modal-text');
  var ctrl  = document.getElementById('modal-media-controls');
  var badge = document.getElementById('modal-resume-badge');
  var seekBack = document.getElementById('modal-seek-back');
  var seekFwd  = document.getElementById('modal-seek-fwd');
  img.style.display = video.style.display = audio.style.display = pdf.style.display = txt.style.display = 'none';
  ctrl.style.display = 'none';
  badge.textContent = '';
  img.src = ''; pdf.src = '';
  if (video.dataset.resumePath) { if (video.hlsInstance) { video.hlsInstance.destroy(); video.hlsInstance = null; } video.src = ''; video.dataset.resumePath = ''; }
  if (audio.dataset.resumePath) { audio.pause(); audio.src = ''; audio.dataset.resumePath = ''; }
  if (type === 'photo') {
    img.src = fileUrl;
    img.style.display = 'block';
  } else if (type === 'video') {
    seekBack.style.display = ''; seekFwd.style.display = '';
    attachVideo(video, '/hls/playlist?path=' + encodeURIComponent(path), fileUrl);
    video.style.display = 'block';
    ctrl.style.display = 'flex';
    attachMediaResume(video, path, badge, 'watched');
  } else if (type === 'audio') {
    seekBack.style.display = 'none'; seekFwd.style.display = 'none';
    audio.src = fileUrl;
    audio.load();
    audio.style.display = 'block';
    ctrl.style.display = 'flex';
    attachMediaResume(audio, path, badge, 'listened');
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
}
// Add-to-playlist dropdown
var _atpPath = null, _atpEl = null;
function toggleAddToPlaylist(event, path) {
  event.stopPropagation();
  if (_atpEl) { _atpEl.remove(); _atpEl = null; }
  if (_atpPath === path) { _atpPath = null; return; }
  _atpPath = path;
  var btn = event.currentTarget;
  var dd = document.createElement('div');
  dd.style.cssText = 'position:fixed;background:#161b22;border:1px solid #30363d;border-radius:6px;padding:4px 0;z-index:300;min-width:160px;box-shadow:0 4px 12px rgba(0,0,0,0.6)';
  var pls = window.PLAYLISTS || [];
  if (pls.length === 0) {
    dd.innerHTML = '<div style="padding:8px 12px;color:#8b949e;font-size:13px">No playlists yet</div>';
  } else {
    pls.forEach(function(pl) {
      var item = document.createElement('button');
      item.className = 'btn btn-edit';
      item.style.cssText = 'display:block;width:100%;text-align:left;border:none;border-radius:0;padding:7px 14px;font-size:13px';
      item.textContent = pl.name || pl.Name;
      var plId = pl.id || pl.ID;
      item.onclick = function(e) {
        e.stopPropagation();
        fetch('/playlists/' + plId + '/items', {
          method: 'POST',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify({path: path})
        }).then(function() {
          btn.textContent = '✓'; btn.style.color = '#3fb950';
          setTimeout(function() { btn.textContent = '+'; btn.style.color = ''; }, 1500);
          closeATP();
        });
      };
      dd.appendChild(item);
    });
  }
  var rect = btn.getBoundingClientRect();
  dd.style.top = (rect.bottom + 2) + 'px';
  dd.style.left = rect.left + 'px';
  document.body.appendChild(dd);
  _atpEl = dd;
}
function closeATP() { if (_atpEl) { _atpEl.remove(); _atpEl = null; } _atpPath = null; }
document.addEventListener('click', function(e) { if (_atpEl && !_atpEl.contains(e.target)) closeATP(); });
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

const browseRootTmpl = `{{define "content"}}
<div class="page-header">
  <div class="page-header-left">
    <h2>Browse</h2>
    <div class="summary">Select a path to explore</div>
  </div>
</div>
{{if not .Cards}}
<p class="muted">No paths configured yet. <a href="/paths">Add a path</a> to get started.</p>
{{else}}
{{range .Cards}}
<a class="root-card" href="{{browseURL .Path}}">
  <div class="root-card-path">{{.Path}}</div>
</a>
{{end}}
{{end}}
{{end}}`

const browseDirTmpl = `{{define "content"}}
<script>window.PLAYLISTS = {{.PlaylistsJSON}};</script>
<div class="breadcrumb">
  {{range $i, $b := .Breadcrumbs}}
  {{if $i}}<span class="sep">/</span>{{end}}
  {{if $b.Current}}<span class="current">{{$b.Name}}</span>
  {{else}}<a href="{{browseURL $b.Path}}">{{$b.Name}}</a>{{end}}
  {{end}}
</div>
{{if and (not .Subdirs) (not .Files)}}
<p class="muted">This directory is empty.</p>
{{else}}
<div class="table-wrap">
<table>
<thead><tr>
  <th>Name</th>
  <th>Type</th>
  <th>Size</th>
  <th>Modified</th>
  <th>Plays</th>
  <th></th>
</tr></thead>
<tbody>
{{range .Subdirs}}
<tr class="dir-row" data-dir="{{.AbsPath}}" onclick="browseDir(this)">
  <td>
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="#58a6ff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="vertical-align:-2px;margin-right:6px"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>{{.Name}}
  </td>
  <td><span class="badge badge-dir">DIR</span></td>
  <td class="muted">—</td>
  <td class="muted">—</td>
  <td class="muted">—</td>
  <td></td>
</tr>
{{end}}
{{range .Files}}
{{if eq .FileType "other"}}
<tr>
  <td><a href="{{fileURL .AbsPath}}">{{.Filename}}</a></td>
  <td><span class="badge badge-{{.FileType}}">{{upper .FileType}}</span></td>
  <td class="muted">{{.Size}}</td>
  <td class="muted">{{.ModifiedAt}}</td>
  <td class="muted">—</td>
  <td></td>
</tr>
{{else}}
<tr class="file-row" data-path="{{.AbsPath}}" data-name="{{.Filename}}" data-type="{{.FileType}}" onclick="openPreview(this)">
  <td>{{.Filename}}</td>
  <td><span class="badge badge-{{.FileType}}">{{upper .FileType}}</span></td>
  <td class="muted">{{.Size}}</td>
  <td class="muted">{{.ModifiedAt}}</td>
  <td>{{if and (or (eq .FileType "video") (eq .FileType "audio")) (gt .WatchCount 0)}}<span class="badge badge-{{.FileType}}">{{.WatchCount}}×</span>{{else}}<span class="muted">—</span>{{end}}</td>
  <td class="actions-cell">{{if or (eq .FileType "video") (eq .FileType "audio")}}<button class="btn btn-edit btn-sm" onclick="event.stopPropagation();toggleAddToPlaylist(event,'{{.AbsPath}}')">+</button>{{end}}</td>
</tr>
{{end}}
{{end}}
</tbody>
</table>
</div>
{{end}}
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
<div class="pl-layout">
  <div class="pl-sidebar">
    <div id="pl-item-list">
    {{range $i, $it := .Items}}
    <div class="pl-item{{if eq $i $.State.CurrentIndex}} active{{end}}" onclick="startPlaylistItem({{$i}}, 0)">
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
function startPlaylistItem(idx, seekTo) {
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
  if (item.FileType === 'video') {
    v.style.display = 'block'; media = v;
    attachVideo(v, '/hls/playlist?path=' + encodeURIComponent(item.Path), fileUrl);
  } else {
    a.style.display = 'block'; media = a;
    a.src = fileUrl; a.load();
  }
  if (seekTo > 1) media.addEventListener('loadedmetadata', function() { media.currentTime = seekTo; }, {once: true});
  media.addEventListener('ended', function() { savePlState(); startPlaylistItem(plCurrentIdx + 1, 0); }, {once: true});
  media.addEventListener('timeupdate', function onTU() {
    var cur = getPlMedia();
    if (!cur || cur !== media) { media.removeEventListener('timeupdate', onTU); return; }
    var now = Date.now();
    if (now - plLastSave > 5000 && cur.currentTime > 1) { plLastSave = now; savePlState(); }
  });
}
function plPrev() { savePlState(); startPlaylistItem(plCurrentIdx - 1, 0); }
function plNext() { savePlState(); startPlaylistItem(plCurrentIdx + 1, 0); }
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
        startPlaylistItem(Math.min(plCurrentIdx, PLAYLIST_ITEMS.length - 1), 0);
      }
    });
}
function renderPlSidebar() {
  document.getElementById('pl-item-list').innerHTML = PLAYLIST_ITEMS.map(function(item, i) {
    return '<div class="pl-item' + (i === plCurrentIdx ? ' active' : '') + '" onclick="startPlaylistItem(' + i + ',0)">' +
      '<span class="pl-item-name">' + escHtml(item.Name) + '</span>' +
      '<span class="badge badge-' + item.FileType + '" style="flex-shrink:0">' + item.FileType.toUpperCase() + '</span>' +
      '<button class="btn btn-danger btn-sm" style="flex-shrink:0;padding:2px 7px" onclick="event.stopPropagation();removePlaylistItem(' + item.ID + ')">&#x2715;</button>' +
      '</div>';
  }).join('');
}
window.addEventListener('beforeunload', savePlState);
if (PLAYLIST_ITEMS && PLAYLIST_ITEMS.length > 0) {
  startPlaylistItem(Math.min((PLAYLIST_STATE && PLAYLIST_STATE.CurrentIndex) || 0, PLAYLIST_ITEMS.length - 1),
                   (PLAYLIST_STATE && PLAYLIST_STATE.PositionSec) || 0);
}
</script>
{{end}}`
