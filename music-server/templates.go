package main

import (
	"html/template"
)

var (
	libraryTpl *template.Template
	genreTpl   *template.Template
	artistTpl  *template.Template
	albumTpl   *template.Template
	searchTpl  *template.Template
)

var funcMap = template.FuncMap{
	"css": func() template.CSS { return template.CSS(cssContent) },
	"js":  func() template.JS { return template.JS(jsContent) },
	"add": func(a, b int) int { return a + b },
}

func initTemplates() {
	base := template.Must(template.New("base").Funcs(funcMap).Parse(baseTmpl))
	parse := func(content string) *template.Template {
		t := template.Must(base.Clone())
		return template.Must(t.Parse(content))
	}
	libraryTpl = parse(libraryTmpl)
	genreTpl = parse(genreTmpl)
	artistTpl = parse(artistTmpl)
	albumTpl = parse(albumTmpl)
	searchTpl = parse(searchTmpl)
}

// ── Base template ─────────────────────────────────────────────────────────────

const baseTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
<meta name="apple-mobile-web-app-capable" content="yes">
<meta name="theme-color" content="#0d1117">
<title>{{.Title}} – Music</title>
<style>{{css}}</style>
</head>
<body>
<nav class="topnav">
  <a href="/library" data-nav class="nav-logo">♪</a>
  <form id="search-form" action="/search" method="get" class="search-form">
    <input id="search-input" name="q" type="search" class="search-input"
      placeholder="Search artists, albums…"
      value="{{.Query}}"
      autocomplete="off" autocorrect="off" autocapitalize="off">
  </form>
</nav>
<main id="content">
{{template "content" .}}
</main>

<div id="player-bar" class="empty">
  <div id="player-progress" id="prog-wrap">
    <div id="player-progress-fill"></div>
  </div>
  <div id="player-inner">
    <img id="player-art" alt="" style="display:none">
    <div id="player-art-ph" class="art-ph">♪</div>
    <div id="player-info">
      <div id="player-title">Nothing playing</div>
      <div id="player-artist"></div>
    </div>
    <div id="player-btns">
      <button class="ctrl" id="btn-prev" onclick="prevTrack()" aria-label="Previous">⏮</button>
      <button class="ctrl" id="btn-pp" onclick="togglePlay()" aria-label="Play/Pause">▶</button>
      <button class="ctrl" id="btn-next" onclick="nextTrack()" aria-label="Next">⏭</button>
    </div>
  </div>
</div>
<audio id="audio" preload="none"></audio>
<script>{{js}}</script>
</body>
</html>
{{define "content"}}{{end}}`

// ── Page templates ────────────────────────────────────────────────────────────

const libraryTmpl = `{{define "content"}}
<div class="genre-strip">
{{range .Genres}}
  <a href="/library/{{.Name}}" data-nav class="genre-chip">{{.Name}}</a>
{{end}}
</div>
{{range .Genres}}
<div class="section-hdr">{{.Name}}</div>
{{range .Artists}}
<a href="/library/{{.Genre.Name}}/{{.Name}}" data-nav class="artist-row">
  <div class="artist-name">{{.Name}}</div>
  <div class="artist-meta">{{len .Albums}} albums</div>
</a>
{{end}}
{{end}}
{{end}}`

const genreTmpl = `{{define "content"}}
<div class="breadcrumb">
  <a href="/library" data-nav>Library</a> <span class="sep">›</span> {{.Genre.Name}}
</div>
{{range .Genre.Artists}}
<a href="/library/{{.Genre.Name}}/{{.Name}}" data-nav class="artist-row">
  <div class="artist-name">{{.Name}}</div>
  <div class="artist-meta">{{len .Albums}} albums</div>
</a>
{{end}}
{{end}}`

const artistTmpl = `{{define "content"}}
<div class="breadcrumb">
  <a href="/library" data-nav>Library</a> <span class="sep">›</span>
  <a href="/library/{{.Artist.Genre.Name}}" data-nav>{{.Artist.Genre.Name}}</a> <span class="sep">›</span>
  {{.Artist.Name}}
</div>
<div class="album-grid">
{{range .Albums}}
  <a href="{{.PageURL}}" data-nav class="album-card">
    <div class="album-art-wrap">
      {{if .HasCover}}
      <img src="{{.CoverURL}}" alt="{{.Title}}" loading="lazy">
      {{else}}
      <div class="art-ph">♪</div>
      {{end}}
    </div>
    <div class="album-card-info">
      <div class="album-card-title">{{.Title}}</div>
      <div class="album-card-meta">{{.TrackCount}} tracks</div>
    </div>
  </a>
{{end}}
</div>
{{end}}`

const albumTmpl = `{{define "content"}}
<div class="breadcrumb">
  <a href="/library" data-nav>Library</a> <span class="sep">›</span>
  <a href="{{.GenreURL}}" data-nav>{{.Album.Artist.Genre.Name}}</a> <span class="sep">›</span>
  <a href="{{.ArtistURL}}" data-nav>{{.Album.Artist.Name}}</a>
</div>
<div class="album-header">
  <div class="album-cover-wrap">
    {{if .Album.CoverArt}}
    <img src="/cover/{{.Album.Artist.Genre.Name}}/{{.Album.Artist.Name}}/{{.Album.Title}}"
         alt="{{.Album.Title}}">
    {{else}}
    <div class="art-ph">♪</div>
    {{end}}
  </div>
  <div class="album-meta-block">
    <div class="album-title-big">{{.Album.Title}}</div>
    <div class="album-artist-name">{{.Album.Artist.Name}}</div>
    <div class="album-track-count">{{len .Album.Tracks}} tracks</div>
    <button class="play-all-btn" onclick="loadAlbum('{{.APIPath}}', 0)">▶ Play All</button>
  </div>
</div>
<ul class="track-list" id="track-list">
{{range $i, $t := .Album.Tracks}}
  <li class="track-item" data-idx="{{$i}}" onclick="loadAlbum('{{$.APIPath}}', {{$i}})">
    <span class="track-num">{{if .Number}}{{.Number}}{{else}}{{add $i 1}}{{end}}</span>
    <span class="track-title">{{.Title}}</span>
    <span class="track-fmt">{{.Format}}</span>
  </li>
{{end}}
</ul>
{{end}}`

const searchTmpl = `{{define "content"}}
<div class="section-hdr">Search results for "{{.Query}}"</div>
{{if .Artists}}
<div class="section-hdr-sm">Artists</div>
{{range .Artists}}
<a href="{{.URL}}" data-nav class="artist-row">
  <div class="artist-name">{{.Name}}</div>
  <div class="artist-meta">{{.GenreName}} · {{.AlbumCount}} albums</div>
</a>
{{end}}
{{end}}
{{if .Albums}}
<div class="section-hdr-sm">Albums</div>
<div class="album-grid">
{{range .Albums}}
  <a href="{{.URL}}" data-nav class="album-card">
    <div class="album-art-wrap">
      {{if .HasCover}}
      <img src="{{.CoverURL}}" alt="{{.Title}}" loading="lazy">
      {{else}}
      <div class="art-ph">♪</div>
      {{end}}
    </div>
    <div class="album-card-info">
      <div class="album-card-title">{{.Title}}</div>
      <div class="album-card-meta">{{.ArtistName}}</div>
    </div>
  </a>
{{end}}
</div>
{{end}}
{{if and (not .Artists) (not .Albums)}}
<div class="empty-msg">No results for "{{.Query}}"</div>
{{end}}
{{end}}`

// ── CSS ───────────────────────────────────────────────────────────────────────

const cssContent = `
:root {
  --bg:      #0d1117;
  --bg2:     #161b22;
  --bg3:     #21262d;
  --border:  #30363d;
  --text:    #c9d1d9;
  --muted:   #8b949e;
  --accent:  #58a6ff;
  --green:   #3fb950;
  --ph:      84px;
}
*,*::before,*::after { box-sizing: border-box; margin: 0; padding: 0; }
html { -webkit-tap-highlight-color: transparent; }
body {
  background: var(--bg);
  color: var(--text);
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif;
  font-size: 16px;
  line-height: 1.4;
  padding-bottom: calc(var(--ph) + env(safe-area-inset-bottom));
  min-height: 100dvh;
}
a { color: inherit; text-decoration: none; }
button { cursor: pointer; font-family: inherit; }

/* ── Nav ── */
.topnav {
  position: sticky; top: 0; z-index: 20;
  background: var(--bg2);
  border-bottom: 1px solid var(--border);
  display: flex; align-items: center; gap: 10px;
  padding: 8px 14px;
  padding-top: calc(8px + env(safe-area-inset-top));
}
.nav-logo {
  font-size: 24px;
  flex-shrink: 0;
  width: 40px; height: 40px;
  display: flex; align-items: center; justify-content: center;
  border-radius: 8px;
  background: var(--bg3);
  border: 1px solid var(--border);
}
.search-form { flex: 1; }
.search-input {
  width: 100%;
  background: var(--bg3); border: 1px solid var(--border);
  color: var(--text); border-radius: 8px;
  padding: 9px 12px; font-size: 15px;
  -webkit-appearance: none; appearance: none;
}
.search-input:focus { outline: none; border-color: var(--accent); }
.search-input::placeholder { color: var(--muted); }

/* ── Main content ── */
main { padding: 14px 14px 0; }

/* ── Breadcrumb ── */
.breadcrumb {
  font-size: 13px; color: var(--muted);
  margin-bottom: 14px;
  display: flex; flex-wrap: wrap; align-items: center; gap: 4px;
}
.breadcrumb a { color: var(--accent); }
.sep { color: var(--border); }

/* ── Genre strip ── */
.genre-strip {
  display: flex; flex-wrap: wrap; gap: 8px;
  margin-bottom: 20px;
}
.genre-chip {
  background: var(--bg3); border: 1px solid var(--border);
  padding: 8px 16px; border-radius: 20px;
  font-size: 14px; font-weight: 500;
  color: var(--text);
}
.genre-chip:active { background: var(--bg2); }

/* ── Section headers ── */
.section-hdr {
  font-size: 11px; font-weight: 700;
  color: var(--muted); text-transform: uppercase; letter-spacing: 0.7px;
  margin: 18px 0 6px;
}
.section-hdr-sm {
  font-size: 11px; font-weight: 700;
  color: var(--muted); text-transform: uppercase; letter-spacing: 0.7px;
  margin: 14px 0 6px;
}

/* ── Artist row ── */
.artist-row {
  display: flex; justify-content: space-between; align-items: center;
  padding: 13px 4px;
  border-bottom: 1px solid var(--border);
  min-height: 52px;
}
.artist-row:active { background: var(--bg2); margin: 0 -14px; padding-left: 18px; padding-right: 18px; }
.artist-name { font-size: 16px; font-weight: 500; }
.artist-meta { font-size: 13px; color: var(--muted); }

/* ── Album grid ── */
.album-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
  margin-top: 4px;
}
@media (min-width: 500px) {
  .album-grid { grid-template-columns: repeat(3, 1fr); }
}
@media (min-width: 700px) {
  .album-grid { grid-template-columns: repeat(4, 1fr); }
}
.album-card {
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 10px;
  overflow: hidden;
  display: block;
}
.album-card:active { opacity: 0.75; }
.album-art-wrap {
  width: 100%; aspect-ratio: 1;
  background: var(--bg3);
  overflow: hidden;
  display: flex; align-items: center; justify-content: center;
}
.album-art-wrap img {
  width: 100%; height: 100%; object-fit: cover;
  display: block;
}
.art-ph {
  font-size: 40px; color: var(--muted);
  display: flex; align-items: center; justify-content: center;
  width: 100%; height: 100%;
}
.album-card-info { padding: 8px 10px 10px; }
.album-card-title {
  font-size: 13px; font-weight: 600;
  overflow: hidden; text-overflow: ellipsis;
  display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical;
}
.album-card-meta { font-size: 11px; color: var(--muted); margin-top: 3px; }

/* ── Album detail header ── */
.album-header {
  display: flex; gap: 14px; align-items: flex-start;
  margin-bottom: 18px;
}
.album-cover-wrap {
  width: 110px; height: 110px; flex-shrink: 0;
  border-radius: 8px; overflow: hidden;
  background: var(--bg3);
  display: flex; align-items: center; justify-content: center;
}
.album-cover-wrap img { width: 100%; height: 100%; object-fit: cover; }
.album-meta-block { flex: 1; min-width: 0; }
.album-title-big {
  font-size: 17px; font-weight: 700;
  line-height: 1.25;
  margin-bottom: 4px;
}
.album-artist-name { font-size: 14px; color: var(--accent); margin-bottom: 2px; }
.album-track-count { font-size: 13px; color: var(--muted); margin-bottom: 10px; }
.play-all-btn {
  background: var(--accent); color: #0d1117;
  border: none; border-radius: 20px;
  padding: 8px 18px; font-size: 14px; font-weight: 700;
  min-height: 36px;
}
.play-all-btn:active { opacity: 0.8; }

/* ── Track list ── */
.track-list { list-style: none; }
.track-item {
  display: flex; align-items: center; gap: 10px;
  padding: 13px 4px;
  border-bottom: 1px solid var(--border);
  cursor: pointer; min-height: 52px;
}
.track-item:active { background: var(--bg2); margin: 0 -14px; padding-left: 18px; padding-right: 18px; }
.track-item.now-playing { color: var(--green); }
.track-num { width: 26px; text-align: right; color: var(--muted); font-size: 13px; flex-shrink: 0; }
.track-title { flex: 1; font-size: 15px; }
.track-fmt {
  font-size: 10px; color: var(--muted);
  border: 1px solid var(--border); border-radius: 4px;
  padding: 2px 5px; text-transform: uppercase; flex-shrink: 0;
}

/* ── Search ── */
.empty-msg { color: var(--muted); padding: 32px 0; text-align: center; font-size: 15px; }

/* ── Bottom player bar ── */
#player-bar {
  position: fixed; bottom: 0; left: 0; right: 0;
  z-index: 50;
  background: var(--bg2);
  border-top: 1px solid var(--border);
  padding-bottom: env(safe-area-inset-bottom);
}
#player-progress {
  height: 3px; background: var(--border); cursor: pointer;
  position: relative;
}
#player-progress-fill {
  height: 100%; width: 0%; background: var(--accent);
  transition: width 0.4s linear;
}
#player-inner {
  display: flex; align-items: center; gap: 10px;
  padding: 8px 12px;
  height: calc(var(--ph) - 3px);
}
#player-art {
  width: 52px; height: 52px; border-radius: 6px;
  object-fit: cover; flex-shrink: 0;
}
#player-bar .art-ph {
  width: 52px; height: 52px; flex-shrink: 0;
  background: var(--bg3); border-radius: 6px;
  font-size: 24px;
}
#player-info { flex: 1; min-width: 0; }
#player-title {
  font-size: 14px; font-weight: 600;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
#player-artist {
  font-size: 12px; color: var(--muted);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
#player-btns { display: flex; gap: 0; flex-shrink: 0; }
.ctrl {
  background: none; border: none; color: var(--text);
  font-size: 20px;
  width: 44px; height: 44px;
  display: flex; align-items: center; justify-content: center;
  border-radius: 50%;
  -webkit-tap-highlight-color: transparent;
}
.ctrl:active { background: var(--bg3); }
#player-bar.empty #player-info { opacity: 0.4; }
`

// ── JavaScript ────────────────────────────────────────────────────────────────

const jsContent = `
const audio = document.getElementById('audio');
const bar   = document.getElementById('player-bar');
const fill  = document.getElementById('player-progress-fill');
const btnPP = document.getElementById('btn-pp');

let queue = [];
let qi = -1;

// ── Playback ──

function loadAlbum(apiUrl, startIdx) {
  fetch(apiUrl)
    .then(r => r.json())
    .then(data => {
      queue = data.tracks.map(t => Object.assign({}, t, {
        artist: data.artist,
        album:  data.album,
        cover:  data.cover,
        genre:  data.genre,
      }));
      qi = (startIdx != null) ? startIdx : 0;
      saveState();
      playIdx(qi);
    })
    .catch(console.error);
}

function playIdx(i) {
  if (i < 0 || i >= queue.length) return;
  qi = i;
  const t = queue[i];
  audio.src = t.url;
  audio.play().catch(() => {});
  updateBar();
  localStorage.setItem('qi', i);
  markPlaying();
}

function togglePlay() {
  if (!audio.src) return;
  audio.paused ? audio.play().catch(()=>{}) : audio.pause();
}
function prevTrack() { playIdx(qi - 1); }
function nextTrack() { playIdx(qi + 1); }

// ── Bar display ──

function updateBar() {
  if (!queue.length || qi < 0) return;
  const t = queue[qi];
  bar.classList.remove('empty');
  document.getElementById('player-title').textContent = t.title || '';
  document.getElementById('player-artist').textContent =
    [t.artist, t.album].filter(Boolean).join(' – ');
  const img = document.getElementById('player-art');
  const ph  = document.getElementById('player-art-ph');
  if (t.cover) {
    img.src = t.cover;
    img.style.display = '';
    ph.style.display = 'none';
  } else {
    img.style.display = 'none';
    ph.style.display = '';
  }
}

function markPlaying() {
  document.querySelectorAll('.track-item').forEach((el, i) => {
    el.classList.toggle('now-playing', i === qi && queue.length > 0);
  });
}

// ── Audio events ──

audio.addEventListener('ended', () => {
  if (qi + 1 < queue.length) playIdx(qi + 1);
});
audio.addEventListener('play',  () => { btnPP.textContent = '⏸'; });
audio.addEventListener('pause', () => { btnPP.textContent = '▶'; });
audio.addEventListener('timeupdate', () => {
  if (audio.duration) {
    fill.style.width = (audio.currentTime / audio.duration * 100).toFixed(2) + '%';
  }
});

// Seek on progress bar tap
document.getElementById('player-progress').addEventListener('click', e => {
  if (!audio.duration) return;
  const rect = e.currentTarget.getBoundingClientRect();
  audio.currentTime = ((e.clientX - rect.left) / rect.width) * audio.duration;
});

// ── SPA navigation ──

document.addEventListener('click', e => {
  const a = e.target.closest('a[data-nav]');
  if (!a || e.metaKey || e.ctrlKey || e.shiftKey) return;
  e.preventDefault();
  navTo(a.href);
});

document.getElementById('search-form').addEventListener('submit', e => {
  e.preventDefault();
  const q = document.getElementById('search-input').value.trim();
  if (!q) return;
  navTo('/search?q=' + encodeURIComponent(q));
});

window.addEventListener('popstate', () => navTo(location.href, false));

function navTo(url, push) {
  fetch(url)
    .then(r => r.text())
    .then(html => {
      const doc = new DOMParser().parseFromString(html, 'text/html');
      const nc = doc.getElementById('content');
      if (nc) {
        document.getElementById('content').replaceWith(nc);
      }
      if (push !== false) history.pushState(null, '', url);
      // Sync search input
      const sq = new URL(url, location.origin).searchParams.get('q');
      document.getElementById('search-input').value = sq || '';
      markPlaying();
    })
    .catch(() => { location.href = url; });
}

// ── State persistence ──

function saveState() {
  try {
    localStorage.setItem('queue', JSON.stringify(queue));
    localStorage.setItem('qi', qi);
  } catch(e) {}
}

(function restoreState() {
  try {
    const q = localStorage.getItem('queue');
    const i = localStorage.getItem('qi');
    if (q && i !== null) {
      queue = JSON.parse(q);
      qi = parseInt(i, 10);
      if (queue.length > 0 && qi >= 0) {
        updateBar();
        audio.src = queue[qi].url; // ready to play but don't autoplay
      }
    }
  } catch(e) {}
  markPlaying();
})();
`
