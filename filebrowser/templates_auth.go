package main

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
.divider { display: flex; align-items: center; gap: 10px; margin: 20px 0; color: var(--fg-muted); font-size: 12px; }
.divider::before, .divider::after { content: ''; flex: 1; height: 1px; background: var(--border); }
.btn-google { display: flex; align-items: center; justify-content: center; gap: 10px; width: 100%; padding: 8px; background: var(--bg); border: 1px solid var(--border); border-radius: 6px; color: var(--fg); font-size: 14px; font-family: inherit; text-decoration: none; cursor: pointer; }
.btn-google:hover { background: var(--surface-hover); }
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
  <div class="divider">or</div>
  <a href="{{googleLoginURL .Next}}" class="btn-google">
    <svg width="16" height="16" viewBox="0 0 48 48"><path fill="#FFC107" d="M43.6 20.5H42V20H24v8h11.3c-1.6 4.7-6.1 8-11.3 8-6.6 0-12-5.4-12-12s5.4-12 12-12c3.1 0 5.9 1.2 8 3.1l5.7-5.7C34.5 6.1 29.5 4 24 4 12.9 4 4 12.9 4 24s8.9 20 20 20 20-8.9 20-20c0-1.3-.1-2.7-.4-3.5z"/><path fill="#FF3D00" d="M6.3 14.7l6.6 4.8C14.6 15.9 18.9 13 24 13c3.1 0 5.9 1.2 8 3.1l5.7-5.7C34.5 6.1 29.5 4 24 4c-7.7 0-14.3 4.3-17.7 10.7z"/><path fill="#4CAF50" d="M24 44c5.4 0 10.3-2.1 14-5.5l-6.5-5.5c-2 1.5-4.6 2.5-7.5 2.5-5.2 0-9.6-3.3-11.3-8l-6.5 5C9.6 39.6 16.2 44 24 44z"/><path fill="#1976D2" d="M43.6 20.5H42V20H24v8h11.3c-.8 2.3-2.2 4.2-4 5.6l6.5 5.5C41.3 36 44 30.5 44 24c0-1.3-.1-2.7-.4-3.5z"/></svg>
    Sign in with Google
  </a>
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
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
<div class="section">
  <div class="section-header"><h3>Google Sign-In</h3></div>
  <p class="muted" style="padding:0 0 12px">Linking a Google account lets this user sign in with it instead of (or alongside) their password. Leave blank to unlink.</p>
  <form action="/users/{{.ID}}/google-email" method="post" style="display:flex;gap:8px;align-items:center;flex-wrap:wrap">
    <input type="text" name="google_email" value="{{.GoogleEmail}}" placeholder="name@gmail.com" autocomplete="off" style="flex:1;min-width:220px;padding:6px 10px;background:var(--bg);border:1px solid var(--border);border-radius:6px;color:var(--fg);font-size:14px;font-family:inherit">
    <button class="btn btn-primary btn-sm" type="submit">Save</button>
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
