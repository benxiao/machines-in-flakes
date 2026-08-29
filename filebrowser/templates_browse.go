package main

// ---- Page templates ----

const browseTmpl = `{{define "content"}}
<script>window.PLAYLISTS = {{.PlaylistsJSON}};</script>
<div class="browse-layout">
  <div class="browse-sidebar" id="browse-sidebar">
    <button class="sidebar-toggle" id="sidebar-toggle" onclick="toggleSidebar()" title="Collapse sidebar">&#8249;</button>
    <button class="sidebar-path-picker" id="sidebar-path-picker" onclick="togglePathMenu(event)" title="Choose a root folder">
      <span>{{if .CurrentRoot}}{{base .CurrentRoot}}{{else}}Choose folder{{end}}</span> &#9662;
    </button>
    <div class="sidebar-paths" id="sidebar-paths" onclick="event.stopPropagation()">
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
      <div style="display:flex;gap:8px;align-items:center;flex-wrap:wrap">
        {{if and .IsAdmin (not .InArchive)}}<label class="btn btn-primary btn-sm" id="upload-btn" style="cursor:pointer;margin:0" title="Upload files to this folder">&#8679; Upload<input type="file" id="upload-input" multiple style="display:none" onchange="uploadFiles(this.files)"></label><span id="upload-status" style="display:none;color:var(--fg-muted);font-size:13px;white-space:nowrap"></span>{{end}}
        {{if and .IsAdmin (not .InArchive)}}<button id="btn-new-folder" class="btn btn-primary btn-sm" onclick="createFolder()" title="Create a new subdirectory in this folder">+ New Folder</button>{{end}}
        <button id="btn-play-all" class="btn btn-primary btn-sm" onclick="playFolderAll()" style="display:none" title="Play all media in this folder in a loop">&#9654; Loop</button>
        {{if or .Files .Subdirs}}<button id="btn-select" class="btn btn-edit btn-sm" onclick="toggleExtMenu(event)" title="Show only folders and files with certain extensions">Filter &#9662;</button>{{end}}
        <div class="view-toggle">
          <button class="btn-view {{if eq .SortBy "opened"}}active{{end}}" onclick="setSort('opened')" title="Sort by last opened">&#128337; Opened</button>
          <button class="btn-view {{if eq .SortBy "date"}}active{{end}}" onclick="setSort('date')" title="Sort by date">&#128197; Date</button>
          <button class="btn-view {{if eq .SortBy "name"}}active{{end}}" onclick="setSort('name')" title="Sort by name">A&#8250;Z Name</button>
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
      <th>{{if eq .SortBy "opened"}}Opened{{else}}Modified{{end}}</th>
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
      <td class="muted">{{if eq $.SortBy "opened"}}{{.LastOpenedAt}}{{else}}{{.ModifiedAt}}{{end}}</td>
      <td></td>
    </tr>
    {{end}}
    {{range .Files}}
    {{if eq .FileType "archive"}}
    <tr class="dir-row" data-dir="{{.AbsPath}}" onclick="browseDir(this)">
      <td onclick="event.stopPropagation()"><input type="checkbox" class="row-check" value="{{.AbsPath}}" data-type="other" data-ext="{{.Extension}}" onchange="updateSelBar()" onclick="event.stopPropagation()" style="cursor:pointer"></td>
      <td>
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="#db6d28" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="vertical-align:-2px;margin-right:6px"><path d="M21 8v13H3V8"/><path d="M1 3h22v5H1z"/><line x1="10" y1="12" x2="14" y2="12"/></svg>{{.Filename}}
      </td>
      <td><span class="badge badge-archive">{{if eq .Extension ".rar"}}RAR{{else}}ZIP{{end}}</span></td>
      <td class="muted">{{.Size}}</td>
      <td class="muted">{{if eq $.SortBy "opened"}}{{.LastOpenedAt}}{{else}}{{.ModifiedAt}}{{end}}</td>
      <td class="muted">—</td>
    </tr>
    {{else if eq .FileType "other"}}
    <tr data-path="{{.AbsPath}}" data-name="{{.Filename}}" data-type="other">
      <td><input type="checkbox" class="row-check" value="{{.AbsPath}}" data-type="other" data-ext="{{.Extension}}" onchange="updateSelBar()" onclick="event.stopPropagation()" style="cursor:pointer"></td>
      <td><a href="{{fileURL .AbsPath}}" data-path="{{.AbsPath}}" onclick="event.stopPropagation(); recordOpen(this.dataset.path)">{{.Filename}}</a></td>
      <td><span class="badge badge-{{.FileType}}">{{upper .FileType}}</span></td>
      <td class="muted">{{.Size}}</td>
      <td class="muted">{{if eq $.SortBy "opened"}}{{.LastOpenedAt}}{{else}}{{.ModifiedAt}}{{end}}</td>
      <td class="muted">—</td>
    </tr>
    {{else}}
    <tr class="file-row" data-path="{{.AbsPath}}" data-name="{{.Filename}}" data-type="{{.FileType}}" onclick="openPreview(this, true)">
      <td><input type="checkbox" class="row-check" value="{{.AbsPath}}" data-type="{{.FileType}}" data-ext="{{.Extension}}" onchange="updateSelBar()" onclick="event.stopPropagation()" style="cursor:pointer"></td>
      <td>{{.Filename}}</td>
      <td><span class="badge badge-{{.FileType}}">{{upper .FileType}}</span></td>
      <td class="muted">{{.Size}}</td>
      <td class="muted">{{if eq $.SortBy "opened"}}{{.LastOpenedAt}}{{else}}{{.ModifiedAt}}{{end}}</td>
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
      <a href="{{fileURL .AbsPath}}" data-path="{{.AbsPath}}" onclick="event.stopPropagation(); recordOpen(this.dataset.path)" style="display:contents">
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
  <button id="sel-fav-btn" class="btn btn-edit btn-sm" style="display:none" onclick="favoriteSelected()">&#9734; Favorite</button>
  <button id="sel-ext-btn" class="btn btn-edit btn-sm" style="display:none" onclick="selectSameExt()"></button>
  <button class="btn btn-edit btn-sm" onclick="downloadSelected()">⬇ Download</button>
  {{if and .IsAdmin (not .InArchive)}}
  <button id="sel-rename" class="btn btn-edit btn-sm" style="display:none" onclick="renameSelected()">&#x270E; Rename</button>
  <button class="btn btn-edit btn-sm" onclick="moveSelected()">&#128193; Move</button>
  <button class="btn btn-danger btn-sm" onclick="deleteSelected()">&#128465; Delete</button>
  <button id="sel-hashdel-btn" class="btn btn-danger btn-sm" style="display:none" onclick="deleteAllCopies()">&#128465; Delete All Copies</button>
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
  // Favorite (music only): shown when a folder or audio file is selected.
  var hasFavable = checks.some(function(c) { return c.dataset.type === 'audio' || c.dataset.type === 'dir'; });
  var favBtn = document.getElementById('sel-fav-btn');
  if (favBtn) favBtn.style.display = hasFavable ? '' : 'none';
  var ren = document.getElementById('sel-rename');
  if (ren) ren.style.display = checks.length === 1 ? '' : 'none';
  var hashDelBtn = document.getElementById('sel-hashdel-btn');
  if (hashDelBtn) hashDelBtn.style.display = (checks.length === 1 && checks[0].dataset.type !== 'dir') ? '' : 'none';
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
// Mobile root-folder picker: same tap-to-open/outside-click-to-close pattern
// as the ext-menu above, just listing .sidebar-paths instead of extensions.
function hidePathMenu() {
  var m = document.getElementById('sidebar-paths');
  if (m) m.classList.remove('open');
}
function togglePathMenu(ev) {
  ev.stopPropagation();
  var menu = document.getElementById('sidebar-paths');
  if (menu.classList.contains('open')) { hidePathMenu(); return; }
  menu.classList.add('open');
  var r = ev.currentTarget.getBoundingClientRect();
  menu.style.top = (r.bottom + 4) + 'px';
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
document.addEventListener('click', hidePathMenu);
document.addEventListener('keydown', function(e) { if (e.key === 'Escape') { hideExtMenu(); hidePathMenu(); } });
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
function deleteAllCopies() {
  var checks = selChecks(true);
  if (checks.length !== 1) return;
  var path = checks[0].value;
  var btn = document.getElementById('sel-hashdel-btn');
  var origText = btn.textContent;
  btn.disabled = true;
  btn.textContent = 'Searching…';
  fetch('/api/file/hash-matches', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({path: path})})
    .then(function(r) {
      if (!r.ok) return r.text().then(function(t) { throw new Error(t || r.statusText); });
      return r.json();
    })
    .then(function(data) {
      btn.disabled = false;
      btn.textContent = origText;
      var paths = data.paths || [];
      if (paths.length <= 1) {
        alert('No other copies of this file were found.');
        return;
      }
      var shown = paths.slice(0, 20).join('\n') + (paths.length > 20 ? '\n…and ' + (paths.length - 20) + ' more' : '');
      var msg = paths.length + ' files share this exact content, including the file you selected:\n\n' + shown +
        '\n\nDelete all ' + paths.length + ' to Trash? No copy of this content will remain.';
      if (!confirm(msg)) return;
      fetch('/api/file/delete', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({paths: paths})})
        .then(function(r) { return r.json(); })
        .then(function(res) {
          if (res.errors && res.errors.length) alert('Some items failed:\n' + res.errors.join('\n'));
          location.reload();
        })
        .catch(function(e) { alert('Delete failed: ' + e); });
    })
    .catch(function(e) {
      btn.disabled = false;
      btn.textContent = origText;
      alert('Search failed: ' + e.message);
    });
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
// Bulk "Favorite" from the select bar: add-only (folders + audio files; video/photo
// skipped). Reuses POST /favorites/toggle, gated by /favorites/list so already-favorited
// items aren't toggled back off.
function favoriteSelected() {
  var checked = selChecks(true).filter(function(c){ return c.dataset.type === 'dir' || c.dataset.type === 'audio'; });
  if (!checked.length) return;
  fetch('/favorites/list').then(function(r){ return r.json(); }).then(function(paths) {
    var have = {}; (paths || []).forEach(function(p){ have[p] = true; });
    var todo = checked.filter(function(c){ return !have[c.value]; });
    var ok = document.getElementById('sel-ok');
    if (!todo.length) { if (ok) { ok.style.display = ''; ok.textContent = 'Already in favorites'; } return; }
    Promise.all(todo.map(function(c){
      return fetch('/favorites/toggle', {method: 'POST', headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({path: c.value, is_folder: c.dataset.type === 'dir'})});
    })).then(function(){
      if (ok) { ok.style.display = ''; ok.textContent = todo.length + (todo.length === 1 ? ' item' : ' items') + ' added to favorites'; }
    }).catch(function(){});
  }).catch(function(){});
}
// Apply saved sort preference if URL has no sort param. 'opened' is the
// server's default (an omitted/empty sort param already means "opened"), so
// only a saved 'date' or 'name' preference needs to force it into the URL.
(function() {
  try {
    var urlSort = new URLSearchParams(window.location.search).get('sort');
    if (urlSort === null) {
      var saved = _getStoredSort();
      if (saved === 'date' || saved === 'name') {
        var p = new URLSearchParams(window.location.search);
        p.set('sort', saved);
        window.location.replace('/browse?' + p.toString());
      }
    }
  } catch(e) {}
})();
// Deep link from search results: ?open=<path> opens that file's preview
// (playing, for video) once the folder listing it belongs to has rendered.
// openPreview() itself is defined in baseTmpl's script, which comes AFTER
// this content block in document order — every other in-page call to it is
// lazy (an onclick handler), so this is the one spot that must wait for
// DOMContentLoaded rather than running inline.
document.addEventListener('DOMContentLoaded', function() {
  var params = new URLSearchParams(window.location.search);
  var openPath = params.get('open');
  if (!openPath) return;
  params.delete('open');
  var qs = params.toString();
  history.replaceState(null, '', window.location.pathname + (qs ? '?' + qs : ''));
  var el = document.querySelector('[data-path="' + CSS.escape(openPath) + '"]');
  if (el) openPreview(el, true);
});
</script>
{{end}}`
