package main

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
