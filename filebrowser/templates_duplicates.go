package main

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
