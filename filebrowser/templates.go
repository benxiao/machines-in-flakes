package main

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
      <div style="display:flex;gap:8px;align-items:center;flex-wrap:wrap">
        {{if and .IsAdmin (not .InArchive)}}<label class="btn btn-primary btn-sm" id="upload-btn" style="cursor:pointer;margin:0" title="Upload files to this folder">&#8679; Upload<input type="file" id="upload-input" multiple style="display:none" onchange="uploadFiles(this.files)"></label><span id="upload-status" style="display:none;color:var(--fg-muted);font-size:13px;white-space:nowrap"></span>{{end}}
        {{if and .IsAdmin (not .InArchive)}}<button id="btn-new-folder" class="btn btn-primary btn-sm" onclick="createFolder()" title="Create a new subdirectory in this folder">+ New Folder</button>{{end}}
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

const recentTmpl = `{{define "content"}}
<div style="margin-bottom:16px">
  <h2 style="margin:0 0 2px">Recent</h2>
  <div class="summary">Pick up where you left off, and what's new in your library</div>
</div>
{{if .Continuing}}
<h3 style="margin:0 0 8px">Continue watching &amp; listening</h3>
<div class="view-grid" style="margin-bottom:24px">
{{range .Continuing}}
<div class="grid-card" data-path="{{.Path}}" data-name="{{.Filename}}" data-type="{{.FileType}}" data-context="{{.Context}}" data-context-id="{{.ContextID}}" data-context-start="{{.ContextStart}}" onclick="openRecentItem(this)">
  {{if eq .FileType "video"}}
  <div class="grid-thumb">
    <img src="{{thumbURL .Path}}" loading="lazy" alt="" style="width:100%;height:100%;object-fit:cover;display:block"
         onerror="this.style.display='none';this.nextElementSibling.style.display='flex'">
    <div style="display:none;width:100%;height:100%;align-items:center;justify-content:center">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="32" height="32" fill="none" stroke="#58a6ff" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="2" width="20" height="20" rx="2"/><line x1="7" y1="2" x2="7" y2="22"/><line x1="17" y1="2" x2="17" y2="22"/><line x1="2" y1="12" x2="22" y2="12"/><line x1="2" y1="7" x2="7" y2="7"/><line x1="17" y1="7" x2="22" y2="7"/><line x1="17" y1="17" x2="22" y2="17"/><line x1="2" y1="17" x2="7" y2="17"/></svg>
    </div>
  </div>
  {{else if .AlbumArt}}
  <div class="grid-thumb">
    <img src="{{thumbURL .AlbumArt}}" loading="lazy" alt="" style="width:100%;height:100%;object-fit:cover;display:block"
         onerror="this.style.display='none';this.nextElementSibling.style.display='flex'">
    <div style="display:none;width:100%;height:100%;align-items:center;justify-content:center">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="32" height="32" fill="none" stroke="#bc60ff" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg>
    </div>
  </div>
  {{else}}
  <div class="grid-icon"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="32" height="32" fill="none" stroke="#bc60ff" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg></div>
  {{end}}
  <div class="grid-name">{{.Filename}}</div>
  <div class="continue-resume">&#9654; {{fmtPos .PositionSec}}</div>
</div>
{{end}}
</div>
{{end}}
{{if .Added}}
<h3 style="margin:0 0 8px">Recently added</h3>
<div class="view-grid" style="margin-bottom:24px">
{{range .Added}}
<div class="grid-card" data-path="{{.Path}}" data-name="{{.Filename}}" data-type="{{.FileType}}" data-context="{{.Context}}" data-context-id="{{.ContextID}}" data-context-start="{{.ContextStart}}" onclick="openRecentItem(this)">
  {{if eq .FileType "video"}}
  <div class="grid-thumb">
    <img src="{{thumbURL .Path}}" loading="lazy" alt="" onerror="this.style.display='none';this.nextElementSibling.style.display='flex'">
    <div style="display:none;width:100%;height:100%;align-items:center;justify-content:center">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="#58a6ff" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="2" width="20" height="20" rx="2"/><line x1="7" y1="2" x2="7" y2="22"/><line x1="17" y1="2" x2="17" y2="22"/><line x1="2" y1="12" x2="22" y2="12"/><line x1="2" y1="7" x2="7" y2="7"/><line x1="17" y1="7" x2="22" y2="7"/><line x1="17" y1="17" x2="22" y2="17"/><line x1="2" y1="17" x2="7" y2="17"/></svg>
    </div>
  </div>
  {{else if eq .FileType "photo"}}
  <div class="grid-thumb">
    <img src="{{thumbURL .Path}}" loading="lazy" alt="" onerror="this.style.display='none';this.nextElementSibling.style.display='flex'">
    <div style="display:none;width:100%;height:100%;align-items:center;justify-content:center">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="#3fb950" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><path d="M21 15l-5-5L5 21"/></svg>
    </div>
  </div>
  {{else if .AlbumArt}}
  <div class="grid-thumb">
    <img src="{{thumbURL .AlbumArt}}" loading="lazy" alt="" onerror="this.style.display='none';this.nextElementSibling.style.display='flex'">
    <div style="display:none;width:100%;height:100%;align-items:center;justify-content:center">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="#bc60ff" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg>
    </div>
  </div>
  {{else}}
  <div class="grid-icon"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="#bc60ff" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg></div>
  {{end}}
  <div class="grid-name">{{.Filename}}</div>
  <div class="continue-resume">Added {{.AddedAt}}</div>
</div>
{{end}}
</div>
{{end}}
{{if and (not .Continuing) (not .Added)}}
<p class="muted">Nothing here yet. Browse to a video or audio file to get started.</p>
{{end}}
<script>
function openRecentItem(el) {
  var type = el.dataset.type, p = el.dataset.path, ctx = el.dataset.context;
  if (ctx === 'playlist') {
    window.location = '/playlists/' + el.dataset.contextId + '?start=' + el.dataset.contextStart;
  } else if (ctx === 'favorites') {
    window.location = '/favorites?start=' + el.dataset.contextStart;
  } else if (type === 'audio') {
    window.location = '/folder/play?file=' + encodeURIComponent(p);
  } else {
    window.location = '/browse?dir=' + encodeURIComponent(p.slice(0, p.lastIndexOf('/'))) + '&open=' + encodeURIComponent(p);
  }
}
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
        <div id="pl-bookmarks-row" class="pl-bookmarks-row" style="display:none"></div>
      </div>
      <div class="pl-transport">
        <div class="pl-speed-wrap">
          <span class="pl-speed-icon">&#177;</span>
          <input type="range" class="pl-speed" id="pl-speed" value="1" min="0.7" max="1.3" step="0.01">
          <span class="pl-speed-label" id="pl-speed-label" onclick="plResetSpeed()" title="Reset to normal speed"></span>
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
  // Deep link from Recent: ?start=<idx> jumps straight to that track,
  // overriding the normal saved-state resume, then cleans the URL.
  var params = new URLSearchParams(window.location.search);
  var startParam = params.get('start');
  if (startParam !== null) {
    params.delete('start');
    var qs = params.toString();
    history.replaceState(null, '', window.location.pathname + (qs ? '?' + qs : ''));
  }
  if (PLAYLIST_ITEMS && PLAYLIST_ITEMS.length > 0) {
    if (startParam !== null) {
      startPlaylistItem(Math.min(parseInt(startParam, 10) || 0, PLAYLIST_ITEMS.length - 1), 0, true);
    } else {
      startPlaylistItem(Math.min((PLAYLIST_STATE && PLAYLIST_STATE.CurrentIndex) || 0, PLAYLIST_ITEMS.length - 1),
                       (PLAYLIST_STATE && PLAYLIST_STATE.PositionSec) || 0);
    }
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
        <div id="pl-bookmarks-row" class="pl-bookmarks-row" style="display:none"></div>
      </div>
      <div class="pl-transport">
        <div class="pl-speed-wrap">
          <span class="pl-speed-icon">&#177;</span>
          <input type="range" class="pl-speed" id="pl-speed" value="1" min="0.7" max="1.3" step="0.01">
          <span class="pl-speed-label" id="pl-speed-label" onclick="plResetSpeed()" title="Reset to normal speed"></span>
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
        <div id="pl-bookmarks-row" class="pl-bookmarks-row" style="display:none"></div>
      </div>
      <div class="pl-transport">
        <div class="pl-speed-wrap">
          <span class="pl-speed-icon">&#177;</span>
          <input type="range" class="pl-speed" id="pl-speed" value="1" min="0.7" max="1.3" step="0.01">
          <span class="pl-speed-label" id="pl-speed-label" onclick="plResetSpeed()" title="Reset to normal speed"></span>
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
  // Deep link from Recent: ?start=<idx> jumps straight to that track.
  var params = new URLSearchParams(window.location.search);
  var startParam = params.get('start');
  if (startParam !== null) {
    params.delete('start');
    var qs = params.toString();
    history.replaceState(null, '', window.location.pathname + (qs ? '?' + qs : ''));
  }
  if (PLAYLIST_ITEMS && PLAYLIST_ITEMS.length > 0) {
    if (startParam !== null) {
      startPlaylistItem(Math.min(parseInt(startParam, 10) || 0, PLAYLIST_ITEMS.length - 1), 0, true);
    } else {
      startPlaylistItem(0, 0);
    }
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
      row.style.cssText = 'display:flex;align-items:center;gap:8px;padding:6px 12px;font-size:13px;border-top:1px solid var(--surface-hover);min-width:0';
      var cb = document.createElement('input');
      cb.type = 'checkbox';
      cb.className = 'dup-check';
      cb.value = p;
      cb.checked = pi > 0;
      cb.style.flexShrink = '0';
      row.appendChild(cb);
      var span = document.createElement('span');
      span.style.cssText = 'min-width:0;overflow-wrap:anywhere';
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
<div class="summary" style="margin-bottom:16px">Play time and most played</div>

<div class="stat-cards">
  <div class="stat-card">
    <div class="stat-card-label">Today</div>
    <div class="stat-card-video">&#9654; {{fmtDur .Totals.TodayVideo}}</div>
    <div class="stat-card-audio">&#9834; {{fmtDur .Totals.TodayAudio}}</div>
  </div>
  <div class="stat-card">
    <div class="stat-card-label">Last 7 days</div>
    <div class="stat-card-video">&#9654; {{fmtDur .Totals.WeekVideo}}</div>
    <div class="stat-card-audio">&#9834; {{fmtDur .Totals.WeekAudio}}</div>
  </div>
  <div class="stat-card">
    <div class="stat-card-label">Last 30 days</div>
    <div class="stat-card-video">&#9654; {{fmtDur .Totals.MonthVideo}}</div>
    <div class="stat-card-audio">&#9834; {{fmtDur .Totals.MonthAudio}}</div>
  </div>
  <div class="stat-card">
    <div class="stat-card-label">All time</div>
    <div class="stat-card-video">&#9654; {{fmtDur .Totals.AllVideo}}</div>
    <div class="stat-card-audio">&#9834; {{fmtDur .Totals.AllAudio}}</div>
  </div>
</div>

{{if .HasPlay}}
<div class="stats-panel">
  <h3 style="margin:0 0 12px">Activity</h3>
  <div class="hm-scroll">
    <div class="hm-inner">
      <div class="hm-months">{{range .Weeks}}<div>{{.Month}}</div>{{end}}</div>
      <div style="display:flex">
        <div class="hm-dow"><span>Mon</span><span></span><span>Wed</span><span></span><span>Fri</span><span></span><span></span></div>
        <div class="hm-grid">
          {{range .Weeks}}<div class="hm-col">{{range .Cells}}<div class="hm-cell {{.Class}}" title="{{.Title}}" onclick="hmShow(this)"></div>{{end}}</div>{{end}}
        </div>
      </div>
    </div>
  </div>
  <div id="hm-caption" class="muted" style="font-size:12px;min-height:17px;margin-top:8px"></div>
  <div class="hm-legend">
    <span style="color:#58a6ff">Video</span>
    <div class="hm-cell hmv1"></div><div class="hm-cell hmv2"></div><div class="hm-cell hmv3"></div><div class="hm-cell hmv4"></div>
    <span style="margin-left:10px;color:#bc60ff">Audio</span>
    <div class="hm-cell hma1"></div><div class="hm-cell hma2"></div><div class="hm-cell hma3"></div><div class="hm-cell hma4"></div>
    <span style="margin-left:10px">less &#8594; more &middot; hue = what you played most that day</span>
  </div>
</div>
<script>
function hmShow(el) { document.getElementById('hm-caption').textContent = el.title; }
// Start scrolled to the most recent weeks (right edge) on narrow screens.
(function() { var s = document.querySelector('.hm-scroll'); if (s) s.scrollLeft = s.scrollWidth; })();
</script>
{{else}}
<p class="muted">No play time recorded in the last year.</p>
{{end}}

{{if .TopFolders}}
<div class="stats-panel">
  <h3 style="margin:0 0 12px">Top folders</h3>
  <div style="display:flex;flex-direction:column;gap:8px">
  {{range .TopFolders}}
  <div style="display:flex;align-items:center;gap:10px">
    <div style="width:150px;flex-shrink:0;font-size:12px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" title="{{.Folder}}">{{.Folder}}</div>
    <div style="flex:1;background:var(--surface-hover);border-radius:3px;height:14px;overflow:hidden">
      <div style="height:100%;width:{{.Pct}}%;background:{{if eq .MediaType "audio"}}#bc60ff{{else}}#58a6ff{{end}};border-radius:3px"></div>
    </div>
    <div style="width:64px;flex-shrink:0;font-size:12px;text-align:right;color:var(--fg-muted)">{{fmtDur .Seconds}}</div>
  </div>
  {{end}}
  </div>
</div>
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
