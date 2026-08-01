package main

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
