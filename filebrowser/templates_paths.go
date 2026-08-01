package main

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
