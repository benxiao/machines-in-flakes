package main

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
main { padding: 24px; }
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
.browse-sidebar { width:max-content; min-width:120px; max-width:180px; flex-shrink:0; border-right:1px solid var(--border); padding:0; position:relative; transition:width 0.18s; display:flex; flex-direction:column; }
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
.header-search { position:relative; width:100%; }
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
.continue-row { display:flex; gap:10px; overflow-x:auto; -webkit-overflow-scrolling:touch; padding-bottom:4px; margin-bottom:20px; }
.continue-row .grid-card { flex:0 0 104px; }
.continue-resume { font-size:10px; color:var(--fg-muted); margin-top:2px; }
.grid-chk { position:absolute; top:6px; left:6px; opacity:0; transition:opacity 0.1s; z-index:2; }
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
input[type=text], input[type=number], input[type=password], select {
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
.iz-hint-touch { display: none; }
@media (pointer: coarse) { .iz-hint-mouse { display: none; } .iz-hint-touch { display: inline; } }
#modal-photo-controls { padding: 10px; }
.modal-box.modal-photo { width: 98vw; max-width: 98vw; height: 98vh; max-height: 98vh; }
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
.pl-bookmarks-row { display:flex; gap:6px; overflow-x:auto; -webkit-overflow-scrolling:touch; margin:-2px 0 8px; padding-bottom:2px; }
.pl-bookmark-chip { display:flex; align-items:center; gap:5px; flex-shrink:0; background:var(--surface-hover); border:1px solid var(--border); border-radius:12px; padding:3px 6px 3px 10px; font-size:11px; color:var(--fg-muted); cursor:pointer; white-space:nowrap; }
.pl-bookmark-chip:hover { border-color:#bc60ff; color:var(--fg); }
.pl-bookmark-chip .pl-bm-del { color:var(--fg-muted); opacity:0.5; padding:0 2px; }
.pl-bookmark-chip .pl-bm-del:hover { opacity:1; color:#f85149; }
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
#pl-sleep-btn.active { font-size:11px; color:#bc60ff; }
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
.pl-speed-label { color:var(--fg-muted); font-size:11px; cursor:pointer; user-select:none; min-width:34px; white-space:nowrap; }
.pl-speed-label:hover { color:#58a6ff; text-decoration:underline; }
input.pl-speed { -webkit-appearance:none; appearance:none; width:70px; height:4px; background:var(--border); border-radius:2px; outline:none; cursor:pointer; }
input.pl-speed::-webkit-slider-thumb { -webkit-appearance:none; width:11px; height:11px; border-radius:50%; background:#58a6ff; cursor:pointer; }
input.pl-speed::-moz-range-thumb { width:11px; height:11px; border-radius:50%; background:#58a6ff; border:none; cursor:pointer; }
/* Stats page */
.stat-cards { display:flex; gap:12px; flex-wrap:wrap; margin-bottom:20px; }
.stat-card { background:var(--bg-panel); border:1px solid var(--border); border-radius:8px; padding:12px 16px; flex:1; min-width:130px; }
.stat-card-label { font-size:11px; text-transform:uppercase; letter-spacing:0.5px; color:var(--fg-muted); margin-bottom:6px; white-space:nowrap; }
.stat-card-video { color:#58a6ff; font-size:14px; font-variant-numeric:tabular-nums; white-space:nowrap; }
.stat-card-audio { color:#bc60ff; font-size:14px; font-variant-numeric:tabular-nums; white-space:nowrap; }
.stats-panel { background:var(--bg-panel); border:1px solid var(--border); border-radius:8px; padding:16px; margin-bottom:20px; }
/* GitHub-style activity heatmap: week columns, Monday-start day rows */
.hm-scroll { overflow-x:auto; -webkit-overflow-scrolling:touch; padding-bottom:4px; }
.hm-inner { display:inline-flex; flex-direction:column; }
.hm-months { display:flex; gap:3px; margin-left:30px; margin-bottom:4px; font-size:10px; color:var(--fg-muted); }
.hm-months > div { width:11px; flex-shrink:0; white-space:nowrap; overflow:visible; }
.hm-dow { display:flex; flex-direction:column; gap:3px; width:30px; flex-shrink:0; font-size:9px; color:var(--fg-muted); }
.hm-dow > span { height:11px; line-height:11px; }
.hm-grid { display:flex; gap:3px; }
.hm-col { display:flex; flex-direction:column; gap:3px; }
.hm-cell { width:11px; height:11px; border-radius:2px; background:var(--surface-hover); flex-shrink:0; cursor:pointer; }
.hm-cell.hmx { visibility:hidden; }
.hm-cell.hmv1 { background:rgba(88,166,255,0.25); }
.hm-cell.hmv2 { background:rgba(88,166,255,0.45); }
.hm-cell.hmv3 { background:rgba(88,166,255,0.7); }
.hm-cell.hmv4 { background:#58a6ff; }
.hm-cell.hma1 { background:rgba(188,96,255,0.25); }
.hm-cell.hma2 { background:rgba(188,96,255,0.45); }
.hm-cell.hma3 { background:rgba(188,96,255,0.7); }
.hm-cell.hma4 { background:#bc60ff; }
.hm-legend { display:flex; align-items:center; gap:6px; margin-top:10px; font-size:11px; color:var(--fg-muted); flex-wrap:wrap; }
.hm-legend .hm-cell { cursor:default; }
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
  /* Photo viewer: true fullscreen; dvh tracks the visible viewport as browser chrome hides */
  .modal-box.modal-photo { width: 100vw; max-width: 100vw; height: 100vh; height: 100dvh; max-height: 100vh; max-height: 100dvh; }
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
  input[type=text], input[type=number], input[type=password], select { font-size: 16px; }
  /* Search: full-width fixed panel on mobile */
  .search-panel { border-radius:0 0 8px 8px; }
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
  /* Stack the player transport: buttons row on top, speed slider centered below
     (volume is already hidden on touch). One grid row each, instead of the
     desktop 1fr-auto-1fr columns that collide at phone widths. */
  .pl-transport { grid-template-columns:1fr; row-gap:14px; justify-items:center; }
  .pl-transport-btns { order:-1; gap:16px; }
  .pl-speed-wrap { justify-self:center; }
  input.pl-speed { width:140px; }
  /* Selection bar wraps to multiple rows with admin buttons */
  .sel-spacer { height:130px; }
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
  <span id="play-stats" style="margin-left:auto;color:var(--fg-muted);font-size:12px;white-space:nowrap;flex-shrink:0;display:flex;gap:12px;align-items:center"></span>
  <button id="theme-toggle" class="btn btn-edit btn-sm" onclick="toggleTheme()" title="Toggle light/dark theme" style="flex-shrink:0">&#9789;</button>
</header>
<div id="search-bar" style="padding:10px 24px;background:var(--bg-panel);border-bottom:1px solid var(--border)">
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
</div>
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
        <img id="modal-img" src="" alt="" draggable="false" style="position:absolute;top:0;left:0;transform-origin:0 0;user-select:none;-webkit-user-select:none;-webkit-user-drag:none;pointer-events:none">
        <div id="modal-exif-panel" style="display:none;position:absolute;top:10px;right:10px;max-width:260px;background:rgba(13,17,23,0.92);border:1px solid var(--border);border-radius:8px;padding:12px 14px;font-size:12px;line-height:1.6;color:#fff;z-index:5"
             onclick="event.stopPropagation()" ontouchstart="event.stopPropagation()" ontouchmove="event.stopPropagation()" ontouchend="event.stopPropagation()" onwheel="event.stopPropagation()" onmousedown="event.stopPropagation()" ondblclick="event.stopPropagation()"></div>
      </div>
      <video id="modal-video" controls style="display:none"></video>
      <audio id="modal-audio" controls style="display:none;width:90vw;max-width:600px"></audio>
      <iframe id="modal-pdf" src="" style="display:none"></iframe>
      <pre id="modal-text" style="display:none"></pre>
    </div>
    <div id="modal-pdf-controls" style="display:none;justify-content:center;align-items:center;gap:12px;padding:10px;background:var(--bg);border-top:1px solid var(--border);flex-shrink:0">
      <button id="modal-pdf-md-btn" class="btn btn-edit btn-sm" onclick="copyPDFMarkdown()">&#128203; Copy as Markdown</button>
    </div>
    <div id="modal-iz-hint" style="display:none;color:var(--fg-muted);font-size:11px;text-align:center;flex-shrink:0;background:var(--bg);border-top:1px solid var(--surface-hover)"><span class="iz-hint-mouse">Scroll to zoom &middot; drag to pan &middot; double-click to zoom&thinsp;/&thinsp;fit</span><span class="iz-hint-touch">Pinch to zoom &middot; swipe for next&thinsp;/&thinsp;prev &middot; double-tap to zoom&thinsp;/&thinsp;fit</span></div>
    <div id="modal-photo-controls" style="display:none;justify-content:center;align-items:center;gap:12px;background:var(--bg);border-top:1px solid var(--border);flex-shrink:0">
      <button id="modal-slideshow-btn" class="btn btn-edit btn-sm" onclick="toggleSlideshow()">&#9654; Slideshow</button>
      <button id="modal-exif-btn" class="btn btn-edit btn-sm" onclick="toggleExifPanel()">&#9432; Info</button>
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
// Some MSE event bindings (e.g. the 'waiting' listener) have no natural
// throttle and can fire in a tight loop while a stream is stalled/buffering
// (observed during a slow speed-restart re-encode) — cap how often this
// actually sends so a stall can't exhaust the page's outstanding-request
// budget and starve the real media fetch itself.
var _plLogLast = 0, _plLogDropped = 0;
function plLog(msg) {
  var now = Date.now();
  if (now - _plLogLast < 100) { _plLogDropped++; return; }
  _plLogLast = now;
  if (_plLogDropped > 0) { msg += ' (+' + _plLogDropped + ' suppressed)'; _plLogDropped = 0; }
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
function toggleExifPanel() {
  var panel = document.getElementById('modal-exif-panel');
  var img = document.getElementById('modal-img');
  var path = img.dataset.navPath;
  if (panel.style.display !== 'none') { panel.style.display = 'none'; return; }
  if (panel.dataset.loadedFor === path) { panel.style.display = 'block'; return; }
  panel.innerHTML = 'Loading&hellip;';
  panel.style.display = 'block';
  fetch('/api/photo/exif?path=' + encodeURIComponent(path)).then(function(r) {
    if (!r.ok) throw new Error('exif fetch failed');
    return r.json();
  }).then(function(d) {
    if (img.dataset.navPath !== path) return; // user navigated away while this was in flight
    var rows = [];
    if (d.Width && d.Height) rows.push(d.Width + ' &times; ' + d.Height);
    if (d.Make || d.Model) rows.push([d.Make, d.Model].filter(Boolean).join(' '));
    if (d.DateTaken) rows.push(d.DateTaken);
    if (d.HasGPS) {
      var lat = d.GPSLat.toFixed(5), lon = d.GPSLon.toFixed(5);
      rows.push('<a href="https://www.openstreetmap.org/?mlat=' + lat + '&mlon=' + lon + '&zoom=14" target="_blank" rel="noopener" style="color:#58a6ff">' + lat + ', ' + lon + '</a>');
    }
    if (d.SizeBytes) {
      var sz = d.SizeBytes >= 1e6 ? (d.SizeBytes / 1e6).toFixed(1) + ' MB' : (d.SizeBytes / 1e3).toFixed(0) + ' KB';
      rows.push(sz + ' &middot; ' + d.ModTime);
    }
    panel.innerHTML = rows.length ? rows.join('<br>') : 'No metadata found.';
    panel.dataset.loadedFor = path;
  }).catch(function() {
    if (img.dataset.navPath === path) panel.innerHTML = 'Could not load info.';
  });
}
// ---- Image zoom/pan ----
var iz = {scale:1, fitScale:1, tx:0, ty:0, dragging:false, lx:0, ly:0, lastT:null, tapT:0, tapX:0, tapY:0,
          swiping:false, swX:0, swY:0, swLX:0, swLY:0, swT:0};
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
// Double-click / double-tap: toggle between fit and 100% (actual pixels),
// bringing the clicked point to the middle of the viewport (clamped so the
// image edge never pulls inside it).
function izTapZoom(cx, cy) {
  var wrap = document.getElementById('modal-zoom-wrap');
  var img  = document.getElementById('modal-img');
  if (Math.abs(iz.scale - iz.fitScale) >= 0.01) { izFit(); return; }
  // If fit already shows actual pixels (small image), zoom to 2x instead.
  var ns = iz.fitScale >= 0.99 ? 2 : 1;
  var px = (cx - iz.tx) / iz.scale, py = (cy - iz.ty) / iz.scale;
  var W = wrap.clientWidth, H = wrap.clientHeight;
  var sw = img.naturalWidth * ns, sh = img.naturalHeight * ns;
  iz.scale = ns;
  iz.tx = sw > W ? Math.min(0, Math.max(W - sw, W / 2 - px * ns)) : (W - sw) / 2;
  iz.ty = sh > H ? Math.min(0, Math.max(H - sh, H / 2 - py * ns)) : (H - sh) / 2;
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
    // Chromium can still start a native image drag from a mousedown over the
    // (pointer-events:none) img, hijacking the gesture: dragstart fires,
    // then the browser sends dragend instead of mouseup, so our own drag
    // state never resets. Block it outright — panning is handled entirely
    // by our own mousemove/mouseup above.
    wrap.addEventListener('dragstart', function(e) { e.preventDefault(); });
    wrap.addEventListener('dblclick', function(e) {
      var r = wrap.getBoundingClientRect();
      izTapZoom(e.clientX - r.left, e.clientY - r.top);
    });
    wrap.addEventListener('touchstart', function(e) {
      e.preventDefault();
      // dblclick never fires on touch (preventDefault above), so detect
      // double-tap by hand: two single-finger starts close in time and space.
      if (e.touches.length === 1) {
        var t = e.touches[0], now = Date.now();
        if (iz.tapT && now - iz.tapT < 350 && Math.hypot(t.clientX - iz.tapX, t.clientY - iz.tapY) < 40) {
          iz.tapT = 0;
          var r = wrap.getBoundingClientRect();
          izTapZoom(t.clientX - r.left, t.clientY - r.top);
        } else { iz.tapT = now; iz.tapX = t.clientX; iz.tapY = t.clientY; }
        // At fit scale a single finger swipes between photos (checked after
        // the tap logic above, so a double-tap that just zoomed in won't arm).
        iz.swiping = Math.abs(iz.scale - iz.fitScale) < 0.01;
        iz.swX = iz.swLX = t.clientX; iz.swY = iz.swLY = t.clientY; iz.swT = now;
      } else { iz.tapT = 0; iz.swiping = false; }
      iz.lastT = Array.from(e.touches).map(function(t){return {clientX:t.clientX,clientY:t.clientY};});
    }, {passive:false});
    wrap.addEventListener('touchend', function(e) {
      iz.lastT = Array.from(e.touches).map(function(t){return {clientX:t.clientX,clientY:t.clientY};});
      if (!iz.swiping || e.touches.length) return;
      var dx = iz.swLX - iz.swX, dy = iz.swLY - iz.swY, dt = Date.now() - iz.swT;
      iz.swiping = false;
      // Fast, mostly-horizontal flick → prev/next; anything else springs back.
      if (Math.abs(dx) > 70 && Math.abs(dx) > 2 * Math.abs(dy) && dt < 600) {
        iz.tapT = 0;
        modalNavPhoto(dx < 0 ? 1 : -1);
      } else if (dx || dy) izFit();
    });
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
        if (iz.swiping) {
          // Carousel-style feedback: follow the finger horizontally only.
          iz.tx += e.touches[0].clientX - lt[0].clientX;
          iz.swLX = e.touches[0].clientX; iz.swLY = e.touches[0].clientY;
        } else {
          iz.tx += e.touches[0].clientX - lt[0].clientX;
          iz.ty += e.touches[0].clientY - lt[0].clientY;
        }
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
// Rotation / viewport resize: recompute the fit; re-fit only if not zoomed in.
window.addEventListener('resize', function() {
  var wrap = document.getElementById('modal-zoom-wrap');
  var img  = document.getElementById('modal-img');
  if (!wrap || wrap.style.display === 'none' || !img.naturalWidth) return;
  var atFit = Math.abs(iz.scale - iz.fitScale) < 0.01;
  iz.fitScale = Math.min(wrap.clientWidth / img.naturalWidth, wrap.clientHeight / img.naturalHeight, 1);
  if (atFit) izFit();
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
    // Each photo needs its own fetch; hide any panel left open from the last one.
    var exifPanel = document.getElementById('modal-exif-panel');
    exifPanel.style.display = 'none';
    exifPanel.dataset.loadedFor = '';
    document.getElementById('modal-nav-prev').style.display = '';
    document.getElementById('modal-nav-next').style.display = '';
    modal.querySelector('.modal-box').classList.add('modal-photo');
    // The .modal-photo box has a definite height, so the flex body has one too
    // and the wrap can simply fill it — no viewport arithmetic.
    wrap.style.width = '100%';
    wrap.style.height = '100%';
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
    attachMediaResume(audio, path, badge, 'played', function() {
      plLog('preview ended, advancing=' + !!_na);
      if (_na) openPreview({dataset: _na}, true);
    });
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
        // Files within a folder are already returned grouped by type (video,
        // audio, photo, pdf, archive) then filename — see fileTypeRankExpr —
        // so a mixed folder's nested list keeps same-type files together
        // instead of interleaving them alphabetically.
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
  } else if (el.dataset.type === 'archive') {
    // Archives (zip/rar) are browsed as virtual directories (their own path,
    // not the parent) — same as clicking one in the normal Browse view.
    window.location = '/browse?dir=' + encodeURIComponent(p);
  } else if (el.dataset.type === 'video' || el.dataset.type === 'photo') {
    // Land in the containing folder AND open the file's own preview
    // (playing, for video) rather than leaving the user to find it again.
    window.location = '/browse?dir=' + encodeURIComponent(p.slice(0, p.lastIndexOf('/'))) + '&open=' + encodeURIComponent(p);
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
