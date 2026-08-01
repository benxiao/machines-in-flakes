package main

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
