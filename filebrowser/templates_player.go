package main

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
          <button id="pl-sleep-btn" class="pl-mode-btn" onclick="plCycleSleep()" title="Sleep timer">&#9790;</button>
          <button id="pl-bookmark-btn" class="pl-mode-btn" onclick="plAddBookmark()" title="Bookmark this position">&#128278;</button>
        </div>`

const plSharedJS = `
// Speed-slider drag state: value shown on the slider that hasn't been applied
// to the media element yet (playbackRate assignment is throttled — see the
// pl-speed input handler).
var _plPendingRate = null;
var _plSpeedTimer = null;
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
// ---- Sleep timer ----
// Deadline lives in localStorage so it survives page navigation; only the
// page whose timer was running when it expires pauses the audio (_plSleepArmed).
var _plSleepArmed = false, _plSleepVol0 = null;
function _plSleepRemainMs() {
  var u = 0; try { u = parseInt(localStorage.getItem('fb_sleep_until') || '0', 10); } catch(e) {}
  return u - Date.now();
}
function plCycleSleep() {
  var cur = 0;
  if (_plSleepRemainMs() > 0) { try { cur = parseInt(localStorage.getItem('fb_sleep_len') || '0', 10); } catch(e) {} }
  var next = cur === 0 ? 15 : cur === 15 ? 30 : cur === 30 ? 60 : 0;
  try {
    if (next) {
      localStorage.setItem('fb_sleep_until', String(Date.now() + next * 60000));
      localStorage.setItem('fb_sleep_len', String(next));
    } else {
      localStorage.removeItem('fb_sleep_until');
      localStorage.removeItem('fb_sleep_len');
    }
  } catch(e) {}
  var a = document.getElementById('pl-audio');
  if (!next && a && _plSleepVol0 !== null) { a.volume = _plSleepVol0; _plSleepVol0 = null; }
  plLog('sleep timer ' + (next ? next + 'm' : 'off'));
  _plSleepUpdate();
}
function _plSleepUpdate() {
  var btn = document.getElementById('pl-sleep-btn');
  if (!btn) return;
  var rem = _plSleepRemainMs();
  var a = document.getElementById('pl-audio');
  if (rem <= 0) {
    if (_plSleepArmed) {
      _plSleepArmed = false;
      if (a) {
        a.pause();
        if (_plSleepVol0 !== null) { a.volume = _plSleepVol0; _plSleepVol0 = null; }
      }
      plLog('sleep timer expired — paused');
    }
    btn.classList.remove('active');
    btn.innerHTML = '&#9790;';
    btn.title = 'Sleep timer';
    try {
      if (localStorage.getItem('fb_sleep_until')) {
        localStorage.removeItem('fb_sleep_until');
        localStorage.removeItem('fb_sleep_len');
      }
    } catch(e) {}
    return;
  }
  _plSleepArmed = true;
  btn.classList.add('active');
  btn.textContent = Math.ceil(rem / 60000) + 'm';
  btn.title = 'Sleep timer: pause in ' + Math.ceil(rem / 60000) + ' min (tap to change)';
  // Fade out over the final 10 seconds.
  if (a && !a.paused && rem < 10000) {
    if (_plSleepVol0 === null) _plSleepVol0 = a.volume;
    a.volume = _plSleepVol0 * Math.max(0.03, rem / 10000);
  }
}
// ---- End sleep timer ----
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
// Server-side time-stretch: on the MSE path, non-1x speed is done by ffmpeg
// (rubberband) instead of the browser's real-time WSOLA stretcher, which
// warbles on sustained tones.
function plServerStretch() {
  return plMseOk() && Math.abs(DEFAULT_SPEED - 1) >= 0.01;
}
// The rate the <audio> element itself should run at: 1 when the current MSE
// stream is already tempo-stretched server-side, the user speed otherwise.
function _plElementRate() {
  return (_mse && _mse.speed && _mse.speed !== 1) ? 1 : DEFAULT_SPEED;
}
var _plPreload = {path: null, blobUrl: null, blob: null, ctrl: null};
var _plPreloadSkip = {};
var _plActiveBlobUrl = null;
function plAudioUrl(item) {
  var _ltEnabled = true; try { _ltEnabled = localStorage.getItem('fb_lossless_transcode_enabled') !== '0'; } catch(e) {}
  var _kbps = '256'; try { _kbps = localStorage.getItem('fb_lossless_audio_kbps') || '256'; } catch(e) {}
  var tUrl = '/transcode/stream?path=' + encodeURIComponent(item.Path) + '&audio_kbps=' + _kbps;
  if (plMseOk()) {
    if (plServerStretch()) tUrl += '&speed=' + DEFAULT_SPEED;
    return tUrl;
  }
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
  // Abort any in-flight stream fetch: a stretched fetch keeps a server-side
  // ffmpeg encode running for the whole track if left to complete, so a
  // speed change would otherwise leave zombie encodes stacking up.
  if (_mse.fetchCtrl) { try { _mse.fetchCtrl.abort(); } catch(e) {} _mse.fetchCtrl = null; }
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
// Delivers a track's AAC bytes incrementally: onData(Uint8Array) per chunk
// as it arrives off the wire, onEnd(ok, totalBytes) exactly once. The server
// streams ffmpeg's output progressively, so reading the body as a stream
// (instead of buffering the whole response) means the first seconds of audio
// are appendable well under a second in — track starts and speed changes no
// longer wait out the full encode of a 10+ minute movement.
function plMseFetch(st, url, tag, onData, onEnd) {
  function take() {
    plLog('mse fetch preload-hit ' + tag);
    var blob = _plPreload.blob, bu = _plPreload.blobUrl;
    _plPreload = {path: null, blobUrl: null, blob: null, ctrl: null};
    if (bu) { try { URL.revokeObjectURL(bu); } catch(e) {} }
    blob.arrayBuffer().then(
      function(buf) { onData(new Uint8Array(buf)); onEnd(true, buf.byteLength); },
      function() { onEnd(false, 0); });
  }
  function direct() {
    plLog('mse fetch stream ' + tag);
    var ctrl = null;
    try { ctrl = new AbortController(); st.fetchCtrl = ctrl; } catch(e) {}
    var got = 0;
    fetch(url, ctrl ? {signal: ctrl.signal} : undefined)
      .then(function(r) {
        if (!r.ok) throw new Error('http ' + r.status);
        if (!r.body || !r.body.getReader) return r.arrayBuffer().then(function(buf) {
          got = buf.byteLength; onData(new Uint8Array(buf));
        });
        var reader = r.body.getReader();
        function step() {
          return reader.read().then(function(res) {
            if (_mse !== st) { try { reader.cancel(); } catch(e) {} return; }
            if (res.done) return;
            got += res.value.byteLength;
            onData(res.value);
            return step();
          });
        }
        return step();
      })
      .then(function() { if (st.fetchCtrl === ctrl) st.fetchCtrl = null; onEnd(true, got); },
            function() { if (st.fetchCtrl === ctrl) st.fetchCtrl = null; onEnd(false, got); });
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
  // The bound joins st.bounds as soon as its start is known (first chunk
  // about to append), NOT when the whole track has been appended: on
  // quota-limited phones the queue drains over the track's whole runtime,
  // and plTrackPos() must be able to map element→source time mid-append or
  // a speed change/state save early in a long track sees "no position" and
  // restarts from 0. dur stays 0 (unknown) until the done marker.
  if (op.mark) { op.mark.start = st.end; st.bounds.push(op.mark); st.q.shift(); _msePump(st); return; }
  if (op.done) {
    op.done.dur = st.end - op.done.start;
    st.q.shift();
    plLog('mse appended idx=' + op.done.idx + ' dur=' + op.done.dur.toFixed(1) + _mseInfo());
    if (op.cb) op.cb();
    _msePump(st);
    return;
  }
  // Bare callback op (e.g. the first-buffered hook queued right after a
  // track's first audio chunk).
  if (!op.buf) { st.q.shift(); if (op.cb) op.cb(); _msePump(st); return; }
  try {
    sb.appendBuffer(op.buf);
    st.q.shift();
  } catch(e) {
    if (e.name === 'QuotaExceededError') {
      // A long track can exceed the SourceBuffer quota outright (Android
      // budgets can be as small as ~2MB ≈ 65s of 256k AAC). Only issue a
      // remove() when it will actually free something: a remove of an
      // already-empty range is a no-op that still fires updateend, which
      // re-runs this pump into the same throw — a hard infinite loop
      // (observed live: ~70k iterations/sec, playback frozen). When nothing
      // is evictable yet, leave the chunk queued and return; plMseTick
      // (4s interval + timeupdate) re-pumps as playback advances and
      // eviction becomes possible.
      var cur = a ? a.currentTime : 0;
      var evictTo = cur - 5;
      var canEvict = false;
      try { canEvict = sb.buffered.length > 0 && sb.buffered.start(0) < evictTo - 0.5; } catch(e3) {}
      if (canEvict) {
        plLog('mse quota evict t=' + cur.toFixed(0));
        try { sb.remove(0, evictTo); } catch(e2) { plLog('mse evict failed ' + e2.name); }
      } else {
        plLog('mse quota full, waiting t=' + cur.toFixed(0) + ' qlen=' + st.q.length);
      }
    } else {
      st.q.shift();
      plLog('mse append error ' + e.name);
    }
  }
}
function plMseAppendTrack(st, idx, onBuffered, ssReal) {
  if (!PLAYLIST_ITEMS.length) return;
  idx = ((idx % PLAYLIST_ITEMS.length) + PLAYLIST_ITEMS.length) % PLAYLIST_ITEMS.length;
  var item = PLAYLIST_ITEMS[idx];
  if (item.FileType !== 'audio') { st.noMore = true; plLog('mse noMore: idx=' + idx + ' is ' + item.FileType); return; }
  st.fetchingIdx = idx;
  var url = plAudioUrl(item);
  // ss = start offset in source seconds: the server only encodes the
  // remainder, so speed changes / resumes mid-track don't wait for a
  // full-track re-encode. The skipped part is simply not in the buffer.
  if (ssReal > 0) url += '&ss=' + ssReal.toFixed(3);
  var bound = {idx: idx, start: -1, dur: 0, off: ssReal || 0};
  // 384KB ≈ 12s of 256k AAC per append: small enough that a phone with a
  // ~2MB SourceBuffer quota can still make append progress from the space
  // a routine eviction frees (1.5MB chunks required more free space than
  // eviction could ever produce there, deadlocking playback).
  var CH = 384 * 1024;
  var pending = [], pendingBytes = 0, started = false, cbQueued = false;
  function flush() {
    if (!pendingBytes) return;
    var buf = new Uint8Array(pendingBytes), o = 0;
    for (var i = 0; i < pending.length; i++) { buf.set(pending[i], o); o += pending[i].byteLength; }
    pending = []; pendingBytes = 0;
    if (!started) { started = true; st.q.push({mark: bound}); }
    // Always slice to CH-sized appends: the preload-hit path delivers a
    // whole ~10MB track in ONE onData call, and a single appendBuffer that
    // big can exceed the SourceBuffer quota outright — an append no amount
    // of eviction can ever make fit (deadlocks playback at the buffer edge).
    for (var off = 0; off < buf.byteLength; off += CH) {
      st.q.push({buf: buf.buffer.slice(off, Math.min(off + CH, buf.byteLength))});
      if (!cbQueued) {
        cbQueued = true;
        // "First buffered" fires after the first audio chunk of the track is
        // appended — not after the whole track, whose bytes keep streaming
        // in for as long as the encode runs.
        if (onBuffered) st.q.push({cb: onBuffered});
      }
    }
    _msePump(st);
  }
  plMseFetch(st, url, 'idx=' + idx, function(chunk) {
    if (_mse !== st) return;
    pending.push(chunk);
    pendingBytes += chunk.byteLength;
    // First flush goes out small (~3s of audio) so playback starts as soon
    // as possible; later flushes batch up to CH to keep append overhead low.
    if (pendingBytes >= (started ? CH : 96 * 1024)) flush();
  }, function(ok, got) {
    if (_mse !== st) return;
    st.fetchingIdx = -1;
    if (!got) {
      st.nextTryAt = Date.now() + 5000;
      plLog('mse fetch failed idx=' + idx);
      return;
    }
    flush();
    // A mid-stream drop still closes the track's bounds with whatever
    // arrived, so playback plays out the partial audio and chains to the
    // next track instead of stalling at the buffer edge forever.
    if (!ok) plLog('mse stream truncated idx=' + idx + ' got=' + got);
    st.q.push({done: bound});
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
  // Real (source) track-seconds, regardless of server stretch: buffered time
  // runs 1/speed as long as the source, and an ss-offset start means the
  // first b.off source-seconds aren't in the buffer at all. With speed=1 and
  // off=0 this is exactly the old element-time arithmetic, so position
  // saving, stats, seek and display all keep source-seconds semantics.
  var sp = st.speed || 1, off = b.off || 0;
  // dur 0 = still appending, real duration unknown — callers fall back to
  // the element's own duration rather than treating "off" as the length.
  return {idx: b.idx, pos: Math.max(0, cur - b.start) * sp + off, dur: b.dur ? b.dur * sp + off : 0, start: b.start, off: off, sp: sp};
}
// Records a play into video_positions (what Recent reads from) regardless of
// which of the three queue engines is playing — folder-play/favorites never
// wrote here before, so their tracks never showed up in Recent at all.
// delta_sec is always 0: play-time accumulation is already owned by
// savePlState() (playlist-detail posts its own delta to /playlists/{id}/state);
// this call would otherwise double-count it.
function plRecordPosition(completed) {
  var item = PLAYLIST_ITEMS && PLAYLIST_ITEMS[plCurrentIdx];
  if (!item) return;
  var media = getPlMedia();
  var pos = media ? media.currentTime : 0;
  var tp = plTrackPos();
  if (tp) pos = tp.pos;
  fetch('/video/position', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({path: item.Path, position: completed ? 0 : pos, completed: !!completed, delta_sec: 0})
  });
}
// Seek the active audio element to a target position given in real
// source-seconds (what plTrackPos() returns, and what bookmarks store) —
// shared by the seek slider and bookmark jumps.
function plSeekToPos(pos) {
  var a = document.getElementById('pl-audio');
  var tp = plTrackPos();
  if (tp) {
    if (pos < tp.off) {
      // Before the stream's ss start point — that part was never fetched.
      startPlaylistItem(tp.idx, pos, true);
      return;
    }
    var target = tp.start + (pos - tp.off) / tp.sp;
    var evicted = false;
    try { evicted = a.buffered.length > 0 && target < a.buffered.start(0); } catch(e) {}
    if (evicted) { startPlaylistItem(tp.idx, pos, true); }
    else { a.currentTime = target; }
  } else if (a.duration && isFinite(a.duration)) {
    a.currentTime = pos;
  }
  _plUpdateAudioUI();
}
// Clicking the speed label (e.g. "normal" or "+8%") snaps straight back to
// 1x — dragging a slider onto that exact value is fiddly. Routed through
// the slider's own 'change' listener (via a real event) rather than
// duplicating applySpeed's restart/save logic here.
function plResetSpeed() {
  var speed = document.getElementById('pl-speed');
  if (!speed) return;
  speed.value = 1;
  _plPendingRate = 1;
  speed.dispatchEvent(new Event('change'));
}
// ---- Bookmarks ----
var _plBookmarkPath = null; // path the currently-rendered chip row belongs to
function plLoadBookmarks(path) {
  _plBookmarkPath = path;
  var row = document.getElementById('pl-bookmarks-row');
  if (!row) return;
  fetch('/api/bookmarks?path=' + encodeURIComponent(path)).then(function(r) { return r.json(); }).then(function(marks) {
    if (_plBookmarkPath !== path || !row.isConnected) return;
    if (!marks || !marks.length) { row.style.display = 'none'; row.innerHTML = ''; return; }
    row.innerHTML = marks.map(function(b) {
      return '<span class="pl-bookmark-chip" onclick="plJumpBookmark(' + b.position_sec + ')">' +
        fmtTime(b.position_sec) + ' ' + escHtml(b.label) +
        '<span class="pl-bm-del" onclick="event.stopPropagation();plDeleteBookmark(' + b.id + ')">&times;</span></span>';
    }).join('');
    row.style.display = 'flex';
  }).catch(function(){});
}
function plAddBookmark() {
  var path = _plBookmarkPath;
  if (!path) return;
  var tp = plTrackPos();
  var a = document.getElementById('pl-audio');
  var pos = tp ? tp.pos : (a.currentTime || 0);
  var label = prompt('Bookmark label:', fmtTime(pos));
  if (label === null) return;
  fetch('/api/bookmarks', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({path: path, label: label, position_sec: pos})
  }).then(function(r) { if (r.ok) plLoadBookmarks(path); });
}
function plJumpBookmark(pos) { plSeekToPos(pos); }
function plDeleteBookmark(id) {
  var path = _plBookmarkPath;
  fetch('/api/bookmarks/' + id + '/delete', {method: 'POST'}).then(function(r) { if (r.ok && path) plLoadBookmarks(path); });
}
// ---- End bookmarks ----
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
    plRecordPosition(false);
    plLog('mse boundary idx=' + tp.idx);
  } else if (Date.now() - plLastSave > 5000 && cur > 1 && !a.paused) {
    plLastSave = Date.now();
    savePlState();
    plRecordPosition(false);
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
    // Record completion for the track that just finished before plCurrentIdx
    // moves on (startPlaylistItem below reassigns it to n).
    savePlState();
    plRecordPosition(true);
    if (n >= 0) {
      startPlaylistItem(n, 0, true);
    } else if ('mediaSession' in navigator) {
      navigator.mediaSession.playbackState = 'paused';
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
  var st = {ms: ms, sb: null, q: [], bounds: [], end: 0, fetchingIdx: -1, appendedIdx: -1, noMore: false, nextTryAt: 0, objUrl: null,
            speed: plServerStretch() ? DEFAULT_SPEED : 1,
            ssOff: seekTo > 1 ? seekTo : 0};
  _mse = st;
  // Server-stretched streams already run at the requested tempo.
  a.playbackRate = st.speed !== 1 ? 1 : DEFAULT_SPEED;
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
    // Any mid-track start (resume, speed change) streams from a server-side
    // ss offset rather than fetching the whole file and seeking the element:
    // the client seek only worked once the FULL track had appended, which on
    // a quota-limited phone never happens before the queue stalls — the seek
    // silently vanished and playback ran from 0. ss also means only the
    // remainder is encoded (fast start), and plSeekToPos already refetches
    // with a smaller ss if the user later seeks back before this point.
    var ssOff = st.ssOff;
    plMseAppendTrack(st, idx, function() {
      if (_mse !== st) return;
      try {
        var b = st.sb.buffered;
        if (b.length && a.currentTime < b.start(0)) {
          plLog('mse gap-jump(start) ' + a.currentTime.toFixed(3) + ' -> ' + b.start(0).toFixed(3));
          a.currentTime = b.start(0) + 0.01;
        }
      } catch(e) {}
      // autoplay reflects the flag captured when this track started loading;
      // the fetch+append above is async, so a user may have already clicked
      // play in the meantime (a.paused went false) — don't stomp on that.
      plLog('mse first-buffered ' + ((autoplay || !a.paused) ? 'play' : 'pause') + _mseInfo());
      if (autoplay || !a.paused) plPlay(a); else a.pause();
    }, ssOff);
  }, {once: true});
  if (autoplay) plPlay(a);
  plLog('mse start idx=' + idx + ' ap=' + !!autoplay + (seekTo > 1 ? ' seek=' + seekTo.toFixed(0) : '') + (st.speed !== 1 ? ' stretch=' + st.speed : ''));
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
  if (item.FileType === 'audio') { plLoadBookmarks(item.Path); }
  else { _plBookmarkPath = null; var bmRow = document.getElementById('pl-bookmarks-row'); if (bmRow) { bmRow.style.display = 'none'; bmRow.innerHTML = ''; } }
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
      plRecordPosition(true);
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
    plRecordPosition(true);
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
    if (now - plLastSave > 5000 && media.currentTime > 1) { plLastSave = now; savePlState(); plRecordPosition(false); }
    if (media.duration && isFinite(media.duration)) {
      if (!lateKicked && media.duration - media.currentTime < 30) { lateKicked = true; var ni = plNextIdx(idx); if (ni >= 0) plStartPreload(ni); }
      if (media.currentTime >= media.duration - 0.3) plAdvance();
    }
  };
  media._plOnTU = onTU;
  media.addEventListener('timeupdate', onTU);
}
function plPrev() { savePlState(); plRecordPosition(false); var n = plPrevIdx(plCurrentIdx); if (n >= 0) startPlaylistItem(n, 0, true); }
function plNext() { savePlState(); plRecordPosition(false); var n = plNextIdx(plCurrentIdx, true); if (n >= 0) startPlaylistItem(n, 0, true); }
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
  // tp.dur is 0 while the track is still appending (real length unknown) —
  // fall back to the element's duration like the no-bounds case.
  var durT = (tp && tp.dur > 0) ? tp.dur : ((a.duration && isFinite(a.duration)) ? a.duration : 0);
  if (seek && durT > 0) {
    var pct = Math.min(100, curT / durT * 100).toFixed(2);
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
    // is not exactly 0.86 in JS floats). Render from the user-facing speed
    // (pending value while a drag is in flight), NOT the element's
    // playbackRate — that is 1 when the stream is server-stretched, and lags
    // the thumb during a throttled drag.
    var rate = _plPendingRate !== null ? _plPendingRate :
      Math.round(Math.round(DEFAULT_SPEED / PL_SPEED_STEP) * PL_SPEED_STEP * 100) / 100;
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
  setInterval(_plSleepUpdate, 1000);
  _plSleepUpdate();
  ['timeupdate','play','pause','loadedmetadata','durationchange'].forEach(function(ev) {
    a.addEventListener(ev, _plUpdateAudioUI);
  });
  // playbackRate set at track-start time doesn't survive: the MSE branch
  // reassigns a.src to a new MediaSource object URL afterward (plMseStart),
  // which resets playbackRate to 1 — re-assert it once the new track's
  // metadata is actually loaded, regardless of which loading path was used.
  a.addEventListener('loadedmetadata', function() { a.playbackRate = _plElementRate(); _plUpdateAudioUI(); });
  if (seek) {
    seek.addEventListener('input', function() {
      var tp = plTrackPos();
      var dur = tp && tp.dur > 0 ? tp.dur : ((a.duration && isFinite(a.duration)) ? a.duration : 0);
      if (dur > 0) plSeekToPos(parseFloat(seek.value) / 100 * dur);
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
    // Every playbackRate assignment makes the browser's pitch-preserving
    // time-stretcher re-seed, which is audible as a glitch — a drag fires
    // 'input' per 0.01 step, dozens of times a second, i.e. a crackle storm.
    // Keep the label/gradient live from the pending value, but apply the
    // rate to the element at most once per 200ms, and immediately on release.
    var applySpeed = function(commit) {
      if (_plSpeedTimer) { clearTimeout(_plSpeedTimer); _plSpeedTimer = null; }
      if (_plPendingRate === null) return;
      var rate = _plPendingRate;
      var wantStretch = (plMseOk() && Math.abs(rate - 1) >= 0.01) ? rate : 1;
      var needRestart = _mse && _mse.speed !== wantStretch;
      // A server-stretch change means a stream re-fetch + ffmpeg re-encode:
      // only do that on release ('change'), never per throttled drag step —
      // otherwise a single drag stacks a dozen concurrent encodes.
      if (needRestart && !commit) return;
      _plPendingRate = null;
      DEFAULT_SPEED = rate;
      try { localStorage.setItem('fb_default_speed', rate); } catch(e) {}
      if (needRestart) {
        var tp = plTrackPos();
        // plTrackPos is null while the restarted stream's first fetch is
        // still in flight (nothing appended) — fall back to the stream's own
        // ss offset so back-to-back speed changes don't reset to 0.
        var pos = tp ? tp.pos : ((_mse && _mse.ssOff) || 0);
        plLog('speed restart ' + _mse.speed + ' -> ' + wantStretch + ' pos=' + pos.toFixed(1));
        startPlaylistItem(tp ? tp.idx : plCurrentIdx, pos, !a.paused);
      } else {
        a.playbackRate = _plElementRate();
      }
      _plUpdateAudioUI();
    };
    speed.addEventListener('input', function() {
      var v = parseFloat(speed.value);
      // Magnetic snap: a slider has no click-to-reset affordance of its own,
      // and landing on exactly 1.0 by pixel is fiddly, so widen the target.
      if (Math.abs(v - 1) < 0.015) { v = 1; speed.value = 1; }
      _plPendingRate = v;
      if (!_plSpeedTimer) _plSpeedTimer = setTimeout(function() { applySpeed(false); }, 200);
      _plUpdateAudioUI();
    });
    speed.addEventListener('change', function() { applySpeed(true); });
  }
}
`
