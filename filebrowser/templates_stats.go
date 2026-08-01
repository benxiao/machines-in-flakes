package main

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
