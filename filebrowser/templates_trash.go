package main

const trashTmpl = `{{define "content"}}
<div class="page-header">
  <div class="page-header-left">
    <h2>Trash</h2>
  </div>
  {{if .Items}}
  <form action="/trash/empty" method="post" onsubmit="return confirm('Permanently delete all {{len .Items}} item(s) in Trash? This cannot be undone.')">
    <button class="btn btn-danger btn-sm" type="submit">Empty Trash</button>
  </form>
  {{end}}
</div>
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
<p class="muted" style="margin:-4px 0 16px">Deleted files and folders wait here for 30 days before being permanently removed.</p>
{{if .Items}}
<div class="section">
<div class="table-wrap">
<table>
<thead><tr>
  <th>Name</th>
  <th>Type</th>
  <th>Original location</th>
  <th>Deleted</th>
  <th></th>
</tr></thead>
<tbody>
{{range .Items}}
<tr>
  <td>{{.Name}}</td>
  <td><span class="badge badge-{{if .IsFolder}}dir{{else}}other{{end}}">{{if .IsFolder}}FOLDER{{else}}FILE{{end}}</span></td>
  <td class="muted" style="font-size:12px">{{.OriginalPath}}</td>
  <td class="muted">{{.DeletedAt}}</td>
  <td class="actions-cell">
    <form class="inline" action="/trash/{{.ID}}/restore" method="post">
      <button class="btn btn-edit btn-sm" type="submit">Restore</button>
    </form>
    <form class="inline" action="/trash/{{.ID}}/purge" method="post" onsubmit="return confirm('Permanently delete {{.Name}}? This cannot be undone.')">
      <button class="btn btn-danger btn-sm" type="submit">Delete Forever</button>
    </form>
  </td>
</tr>
{{end}}
</tbody>
</table>
</div>
</div>
{{else}}
<p class="muted">Trash is empty.</p>
{{end}}
{{end}}`
