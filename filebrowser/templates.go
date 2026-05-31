package main

import (
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
	ActiveTab   string
	Dir         string
	DirName     string
	Breadcrumbs []Breadcrumb
	Subdirs     []SubdirRow
	Files       []FileRow
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
.badge-dir     { background: rgba(88,166,255,0.12); color: #58a6ff; border: 1px solid rgba(88,166,255,0.3); }
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
<style>` + css + `</style>
</head>
<body>
<header>
  <span class="logo">
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="#58a6ff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="vertical-align:-4px;margin-right:6px"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>File Browser
  </span>
</header>
<nav>
  <a href="/browse" {{if eq .ActiveTab "browse"}}class="active"{{end}}>Browse</a>
  <a href="/paths"  {{if eq .ActiveTab "paths"}}class="active"{{end}}>Paths</a>
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
      <video id="modal-video" src="" controls style="display:none"></video>
      <iframe id="modal-pdf" src="" style="display:none"></iframe>
      <pre id="modal-text" style="display:none"></pre>
    </div>
  </div>
</div>
<script>
var modal = document.getElementById('preview-modal');
function browseDir(el) {
  window.location = '/browse?dir=' + encodeURIComponent(el.dataset.dir);
}
function openPreview(el) {
  var path = el.dataset.path;
  var name = el.dataset.name;
  var type = el.dataset.type;
  var fileUrl = '/file?path=' + encodeURIComponent(path);
  document.getElementById('modal-title').textContent = name;
  var img   = document.getElementById('modal-img');
  var video = document.getElementById('modal-video');
  var pdf   = document.getElementById('modal-pdf');
  var txt   = document.getElementById('modal-text');
  img.style.display = video.style.display = pdf.style.display = txt.style.display = 'none';
  img.src = ''; video.src = ''; pdf.src = '';
  if (type === 'photo') {
    img.src = fileUrl;
    img.style.display = 'block';
  } else if (type === 'video') {
    video.src = fileUrl;
    video.style.display = 'block';
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
  document.getElementById('modal-video').src = '';
}
document.addEventListener('keydown', function(e){ if(e.key==='Escape') closePreview(); });
document.addEventListener('submit', function(e) {
  var action = e.target.getAttribute('action') || '';
  if (action.indexOf('/delete') !== -1) {
    if (!confirm('Remove this path? The file index will be deleted.')) e.preventDefault();
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
</tr>
{{end}}
{{range .Files}}
{{if eq .FileType "other"}}
<tr>
  <td><a href="{{fileURL .AbsPath}}">{{.Filename}}</a></td>
  <td><span class="badge badge-{{.FileType}}">{{upper .FileType}}</span></td>
  <td class="muted">{{.Size}}</td>
  <td class="muted">{{.ModifiedAt}}</td>
</tr>
{{else}}
<tr class="file-row" data-path="{{.AbsPath}}" data-name="{{.Filename}}" data-type="{{.FileType}}" onclick="openPreview(this)">
  <td>{{.Filename}}</td>
  <td><span class="badge badge-{{.FileType}}">{{upper .FileType}}</span></td>
  <td class="muted">{{.Size}}</td>
  <td class="muted">{{.ModifiedAt}}</td>
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
