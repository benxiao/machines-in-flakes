package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
)

// ---- Page data structs ----

type OptionItem struct {
	ID    int
	Label string
}

type DroneListPage struct {
	ActiveTab string
	Drones    []DroneRow
}

type DroneRow struct {
	ID        int
	Name      string
	FrameName string
	FCName    string
	ESCName   string
	VTXName   string
	Status    string
	BuildDate string
	Motors    int
}

type DroneFormPage struct {
	ActiveTab string
	Error     string
	ID        int
	Name      string
	FrameID   int
	FCID      int
	ESCID     int
	VTXID     int
	Status    string
	BuildDate string
	Notes     string
	Frames    []OptionItem
	FCs       []OptionItem
	ESCs      []OptionItem
	VTXs      []OptionItem
}

type InventoryPage struct {
	ActiveTab string
	Frames    []FrameRow
	FCs       []FCRow
	ESCs      []ESCRow
	Motors    []MotorRow
	VTXs      []VTXRow
}

type FrameRow struct {
	ID          int
	Brand       string
	Name        string
	SizeMM      string
	WeightG     string
	Status      string // kept for retired row dimming
	InstalledOn string
	Available   int
}

type FCRow struct {
	ID          int
	Brand       string
	Name        string
	MCU         string
	Firmware    string
	Status      string
	InstalledOn string
	Available   int
}

type ESCRow struct {
	ID            int
	Brand         string
	Name          string
	CurrentRating string
	CellMax       string
	Status        string
	InstalledOn   string
	Available     int
}

type MotorRow struct {
	ID          int
	Brand       string
	Name        string
	StatorSize  string
	KV          string
	Status      string
	InstalledOn string
	Available   int
}

type VTXRow struct {
	ID          int
	Brand       string
	Name        string
	System      string
	MaxPowerMW  string
	Resolution  string
	Status      string
	InstalledOn string
	Available   int
}

type FrameFormPage struct {
	ActiveTab string
	Error     string
	ID        int
	Brand     string
	Name      string
	SizeMM    string
	WeightG   string
	Notes     string
	Status    string
}

type FCFormPage struct {
	ActiveTab string
	Error     string
	ID        int
	Brand     string
	Name      string
	MCU       string
	Firmware  string
	Notes     string
	Status    string
}

type ESCFormPage struct {
	ActiveTab     string
	Error         string
	ID            int
	Brand         string
	Name          string
	CurrentRating string
	CellMax       string
	Notes         string
	Status        string
}

type MotorFormPage struct {
	ActiveTab  string
	Error      string
	ID         int
	Brand      string
	Name       string
	StatorSize string
	KV         string
	DroneID    int
	Notes      string
	Status     string
	Drones     []OptionItem
}

type VTXFormPage struct {
	ActiveTab  string
	Error      string
	ID         int
	Brand      string
	Name       string
	System     string
	MaxPowerMW string
	Resolution string
	WeightG    string
	Notes      string
	Status     string
}

type BatteryListPage struct {
	ActiveTab string
	Batteries []BatteryRow
}

type BatteryRow struct {
	ID                 int
	Name               string
	Brand              string
	CellCount          int
	CapacityMAh        int
	CycleCount         int
	InternalResistance string
	PurchaseDate       string
	DroneName          string
	Status             string
}

type BatteryFormPage struct {
	ActiveTab          string
	Error              string
	ID                 int
	Name               string
	Brand              string
	CellCount          string
	CapacityMAh        string
	CycleCount         string
	InternalResistance string
	PurchaseDate       string
	DroneID            int
	Status             string
	Drones             []OptionItem
}

type PropListPage struct {
	ActiveTab  string
	Propellers []PropRow
}

type PropRow struct {
	ID               int
	Brand            string
	Name             string
	SizeInch         string
	Pitch            string
	BladeCount       int
	Material         string
	Quantity         int
	ReorderThreshold int
	DroneName        string
	LowStock         bool
}

type PropFormPage struct {
	ActiveTab        string
	Error            string
	ID               int
	Brand            string
	Name             string
	SizeInch         string
	Pitch            string
	BladeCount       string
	Material         string
	Quantity         string
	ReorderThreshold string
	DroneID          int
	Notes            string
	Drones           []OptionItem
}

type PartsListPage struct {
	ActiveTab string
	Parts     []PartRow
}

type PartRow struct {
	ID               int
	Category         string
	Name             string
	Quantity         int
	ReorderThreshold int
	UnitPrice        string
	LowStock         bool
}

type PartFormPage struct {
	ActiveTab        string
	Error            string
	ID               int
	Category         string
	Name             string
	Quantity         string
	ReorderThreshold string
	UnitPrice        string
}

type LogListPage struct {
	ActiveTab string
	Sessions  []SessionRow
}

type SessionRow struct {
	ID          int
	DroneName   string
	Type        string
	SessionDate string
	DurationMin int
	Location    string
	Notes       string
	BatteryList string
}

type BatteryCheck struct {
	ID      int
	Label   string
	Checked bool
}

type SessionFormPage struct {
	ActiveTab   string
	Error       string
	ID          int
	DroneID     int
	Type        string
	SessionDate string
	DurationMin string
	Location    string
	Notes       string
	Drones      []OptionItem
	Batteries   []BatteryCheck
}

type SessionDetailPage struct {
	ActiveTab string
	ID        int
	DroneName string
	Type      string
	Date      string
	Duration  int
	Location  string
	Notes     string
	Batteries []BatteryRow
}

// ---- Template engine ----

var pages map[string]*template.Template

var funcMap = template.FuncMap{
	"formatPrice": func(cents int) string {
		return fmt.Sprintf("$%.2f", float64(cents)/100)
	},
	"dash": func(s string) string {
		if s == "" {
			return "—"
		}
		return s
	},
	"badgeClass": func(status string) string {
		switch status {
		case "flying":
			return "badge-flying"
		case "build":
			return "badge-build"
		case "retired":
			return "badge-retired"
		case "crashed":
			return "badge-crashed"
		case "good":
			return "badge-good"
		case "degraded":
			return "badge-degraded"
		case "dead":
			return "badge-dead"
		case "storage":
			return "badge-storage"
		case "flight":
			return "badge-flight"
		case "maintenance":
			return "badge-maintenance"
		case "crash":
			return "badge-crash"
		case "spare":
			return "badge-spare"
		default:
			return "badge-retired"
		}
	},
	"installed": func(s string) bool {
		return s != ""
	},
	"intStr": func(i int) string {
		return strconv.Itoa(i)
	},
}

func initTemplates() {
	base := template.Must(template.New("base").Funcs(funcMap).Parse(baseTmpl))

	add := func(name, content string) {
		t := template.Must(base.Clone())
		template.Must(t.New("content").Parse(content))
		if pages == nil {
			pages = make(map[string]*template.Template)
		}
		pages[name] = t
	}

	add("drone-list", droneListTmpl)
	add("drone-form", droneFormTmpl)
	add("inventory", inventoryTmpl)
	add("frame-form", frameFormTmpl)
	add("fc-form", fcFormTmpl)
	add("esc-form", escFormTmpl)
	add("motor-form", motorFormTmpl)
	add("vtx-form", vtxFormTmpl)
	add("battery-list", batteryListTmpl)
	add("battery-form", batteryFormTmpl)
	add("prop-list", propListTmpl)
	add("prop-form", propFormTmpl)
	add("parts-list", partsListTmpl)
	add("part-form", partFormTmpl)
	add("log-list", logListTmpl)
	add("session-form", sessionFormTmpl)
	add("session-detail", sessionDetailTmpl)
}

func render(w http.ResponseWriter, name string, data any) {
	t, ok := pages[name]
	if !ok {
		http.Error(w, "template not found: "+name, 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		log.Printf("render %s: %v", name, err)
	}
}

// ---- CSS ----

const css = `
*, *::before, *::after { box-sizing: border-box; }
body {
  margin: 0;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  font-size: 14px;
  background: #0d1117;
  color: #c9d1d9;
  line-height: 1.5;
}
a { color: #58a6ff; text-decoration: none; }
a:hover { text-decoration: underline; }
header {
  background: #161b22;
  border-bottom: 1px solid #30363d;
  padding: 12px 24px;
}
.logo { font-size: 18px; font-weight: 600; color: #f0f6fc; }
nav {
  background: #161b22;
  border-bottom: 1px solid #30363d;
  padding: 0 24px;
  display: flex;
}
nav a {
  display: inline-block;
  padding: 10px 16px;
  color: #8b949e;
  border-bottom: 2px solid transparent;
  font-size: 14px;
}
nav a:hover { color: #c9d1d9; text-decoration: none; }
nav a.active { color: #f0f6fc; border-bottom-color: #f78166; }
main { padding: 24px; max-width: 1400px; margin: 0 auto; }
h2 { font-size: 20px; font-weight: 600; margin: 0 0 4px; color: #f0f6fc; }
h3 { font-size: 16px; font-weight: 600; margin: 0 0 12px; color: #f0f6fc; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.page-header-left h2 { margin-bottom: 4px; }
.summary { color: #8b949e; font-size: 13px; }
table { width: 100%; border-collapse: collapse; }
th {
  text-align: left;
  padding: 8px 12px;
  background: #161b22;
  border-bottom: 1px solid #30363d;
  color: #8b949e;
  font-weight: 500;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  white-space: nowrap;
}
td {
  padding: 9px 12px;
  border-bottom: 1px solid #21262d;
  vertical-align: middle;
}
tr:last-child td { border-bottom: none; }
tr:hover td { background: #161b22; }
tr.low-stock td { background: rgba(248,81,73,0.06); }
tr.low-stock:hover td { background: rgba(248,81,73,0.12); }
tr.retired td { opacity: 0.5; }
.badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
}
.badge-flying     { background: rgba(63,185,80,0.15);  color: #3fb950; border: 1px solid rgba(63,185,80,0.4); }
.badge-build      { background: rgba(210,153,34,0.15); color: #d29922; border: 1px solid rgba(210,153,34,0.4); }
.badge-retired    { background: rgba(139,148,158,0.15);color: #8b949e; border: 1px solid rgba(139,148,158,0.4); }
.badge-crashed    { background: rgba(248,81,73,0.15);  color: #f85149; border: 1px solid rgba(248,81,73,0.4); }
.badge-good       { background: rgba(63,185,80,0.15);  color: #3fb950; border: 1px solid rgba(63,185,80,0.4); }
.badge-degraded   { background: rgba(210,153,34,0.15); color: #d29922; border: 1px solid rgba(210,153,34,0.4); }
.badge-dead       { background: rgba(248,81,73,0.15);  color: #f85149; border: 1px solid rgba(248,81,73,0.4); }
.badge-storage    { background: rgba(139,148,158,0.15);color: #8b949e; border: 1px solid rgba(139,148,158,0.4); }
.badge-flight     { background: rgba(88,166,255,0.15); color: #58a6ff; border: 1px solid rgba(88,166,255,0.4); }
.badge-maintenance{ background: rgba(210,153,34,0.15); color: #d29922; border: 1px solid rgba(210,153,34,0.4); }
.badge-crash      { background: rgba(248,81,73,0.15);  color: #f85149; border: 1px solid rgba(248,81,73,0.4); }
.badge-spare      { background: rgba(63,185,80,0.15);  color: #3fb950; border: 1px solid rgba(63,185,80,0.4); }
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
.btn-edit { background: transparent; border-color: #30363d; color: #c9d1d9; }
.btn-edit:hover { background: #21262d; text-decoration: none; color: #c9d1d9; }
.btn-danger { background: transparent; border-color: rgba(248,81,73,0.4); color: #f85149; }
.btn-danger:hover { background: rgba(248,81,73,0.1); text-decoration: none; }
.btn-cancel { background: transparent; border-color: #30363d; color: #8b949e; }
.btn-cancel:hover { background: #21262d; text-decoration: none; color: #c9d1d9; }
form.inline { display: inline; margin: 0; }
.section { margin-bottom: 40px; }
.section-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; }
.table-wrap { border: 1px solid #30363d; border-radius: 6px; overflow: hidden; }
.form-page { max-width: 580px; }
.form-group { margin-bottom: 16px; }
label { display: block; font-size: 13px; color: #8b949e; margin-bottom: 4px; }
input[type=text], input[type=number], input[type=date], select, textarea {
  width: 100%;
  padding: 7px 10px;
  background: #0d1117;
  border: 1px solid #30363d;
  border-radius: 6px;
  color: #c9d1d9;
  font-size: 14px;
  font-family: inherit;
}
input:focus, select:focus, textarea:focus {
  outline: none;
  border-color: #58a6ff;
}
textarea { min-height: 80px; resize: vertical; }
.form-row { display: flex; gap: 16px; }
.form-row .form-group { flex: 1; min-width: 0; }
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
.muted { color: #8b949e; }
hr { border: none; border-top: 1px solid #30363d; margin: 32px 0; }
.battery-checks { display: flex; flex-wrap: wrap; gap: 8px; }
.battery-check {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  border: 1px solid #30363d;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
}
.battery-check:hover { border-color: #58a6ff; background: #161b22; }
.battery-check input[type=checkbox] { margin: 0; cursor: pointer; }
.actions-cell { white-space: nowrap; }
.installed-badge {
  font-size: 12px;
  color: #58a6ff;
}
`

// ---- Base template ----

const baseTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>FPV Manager</title>
<style>` + css + `</style>
</head>
<body>
<header><span class="logo">FPV Manager</span></header>
<nav>
  <a href="/drones"    {{if eq .ActiveTab "drones"}}class="active"{{end}}>Drones</a>
  <a href="/inventory" {{if eq .ActiveTab "inventory"}}class="active"{{end}}>Inventory</a>
  <a href="/props"     {{if eq .ActiveTab "props"}}class="active"{{end}}>Props</a>
  <a href="/batteries" {{if eq .ActiveTab "batteries"}}class="active"{{end}}>Batteries</a>
  <a href="/parts"     {{if eq .ActiveTab "parts"}}class="active"{{end}}>Parts</a>
  <a href="/log"       {{if eq .ActiveTab "log"}}class="active"{{end}}>Log</a>
</nav>
<main>
{{template "content" .}}
</main>
</body>
</html>`

// ---- Page templates ----

const droneListTmpl = `{{define "content"}}
<div class="page-header">
  <div class="page-header-left">
    <h2>Drones</h2>
  </div>
  <a href="/drones/new" class="btn btn-primary">+ Add Drone</a>
</div>
{{if .Drones}}
<div class="table-wrap">
<table>
<thead><tr>
  <th>Name</th><th>Frame</th><th>FC</th><th>ESC</th><th>VTX</th>
  <th>Motors</th><th>Status</th><th>Build Date</th><th></th>
</tr></thead>
<tbody>
{{range .Drones}}
<tr class="{{if eq .Status "retired"}}retired{{end}}">
  <td><strong>{{.Name}}</strong></td>
  <td class="muted">{{dash .FrameName}}</td>
  <td class="muted">{{dash .FCName}}</td>
  <td class="muted">{{dash .ESCName}}</td>
  <td class="muted">{{dash .VTXName}}</td>
  <td class="muted">{{if gt .Motors 0}}{{.Motors}}{{else}}—{{end}}</td>
  <td><span class="badge {{badgeClass .Status}}">{{.Status}}</span></td>
  <td class="muted">{{dash .BuildDate}}</td>
  <td class="actions-cell">
    <a href="/drones/{{.ID}}/edit" class="btn btn-sm btn-edit">Edit</a>
    <form class="inline" method="POST" action="/drones/{{.ID}}/delete">
      <button class="btn btn-sm btn-danger" type="submit">Delete</button>
    </form>
  </td>
</tr>
{{end}}
</tbody>
</table>
</div>
{{else}}
<p class="muted">No drones yet. <a href="/drones/new">Add your first drone.</a></p>
{{end}}
{{end}}`

const droneFormTmpl = `{{define "content"}}
<div class="page-header">
  <h2>{{if .ID}}Edit Drone{{else}}New Drone{{end}}</h2>
</div>
<div class="form-page">
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
<form method="POST">
  <div class="form-group">
    <label>Name *</label>
    <input type="text" name="name" value="{{.Name}}" required autofocus>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Status</label>
      <select name="status">
        <option value="build"   {{if eq .Status "build"}}selected{{end}}>build</option>
        <option value="flying"  {{if eq .Status "flying"}}selected{{end}}>flying</option>
        <option value="retired" {{if eq .Status "retired"}}selected{{end}}>retired</option>
        <option value="crashed" {{if eq .Status "crashed"}}selected{{end}}>crashed</option>
      </select>
    </div>
    <div class="form-group">
      <label>Build Date</label>
      <input type="date" name="build_date" value="{{.BuildDate}}">
    </div>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Frame</label>
      <select name="frame_id">
        <option value="">— none —</option>
        {{range .Frames}}
        <option value="{{.ID}}" {{if eq $.FrameID .ID}}selected{{end}}>{{.Label}}</option>
        {{end}}
      </select>
    </div>
    <div class="form-group">
      <label>Flight Controller</label>
      <select name="fc_id">
        <option value="">— none —</option>
        {{range .FCs}}
        <option value="{{.ID}}" {{if eq $.FCID .ID}}selected{{end}}>{{.Label}}</option>
        {{end}}
      </select>
    </div>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>ESC</label>
      <select name="esc_id">
        <option value="">— none —</option>
        {{range .ESCs}}
        <option value="{{.ID}}" {{if eq $.ESCID .ID}}selected{{end}}>{{.Label}}</option>
        {{end}}
      </select>
    </div>
    <div class="form-group">
      <label>VTX / Air Unit</label>
      <select name="vtx_id">
        <option value="">— none —</option>
        {{range .VTXs}}
        <option value="{{.ID}}" {{if eq $.VTXID .ID}}selected{{end}}>{{.Label}}</option>
        {{end}}
      </select>
    </div>
  </div>
  <div class="form-group">
    <label>Notes</label>
    <textarea name="notes">{{.Notes}}</textarea>
  </div>
  <div class="form-actions">
    <button class="btn btn-primary" type="submit">{{if .ID}}Save Changes{{else}}Create Drone{{end}}</button>
    <a href="/drones" class="btn btn-cancel">Cancel</a>
  </div>
</form>
</div>
{{end}}`

const inventoryTmpl = `{{define "content"}}
<div class="page-header">
  <h2>Inventory</h2>
</div>

<div class="section">
  <div class="section-header">
    <h3>Frames</h3>
    <a href="/frames/new" class="btn btn-sm btn-primary">+ Add Frame</a>
  </div>
  {{if .Frames}}
  <div class="table-wrap">
  <table>
  <thead><tr><th>Brand</th><th>Name</th><th>Size</th><th>Weight</th><th>Avail.</th><th>Installed On</th><th></th></tr></thead>
  <tbody>
  {{range .Frames}}
  <tr class="{{if eq .Status "retired"}}retired{{end}}">
    <td class="muted">{{dash .Brand}}</td>
    <td>{{.Name}}</td>
    <td class="muted">{{if .SizeMM}}{{.SizeMM}}mm{{else}}—{{end}}</td>
    <td class="muted">{{if .WeightG}}{{.WeightG}}g{{else}}—{{end}}</td>
    <td>{{if gt .Available 0}}<span style="color:#3fb950;font-weight:500">{{.Available}}</span>{{else}}<span class="muted">0</span>{{end}}</td>
    <td>{{if .InstalledOn}}<span class="installed-badge">{{.InstalledOn}}</span>{{else}}<span class="muted">—</span>{{end}}</td>
    <td class="actions-cell">
      <a href="/frames/{{.ID}}/edit" class="btn btn-sm btn-edit">Edit</a>
      <form class="inline" method="POST" action="/frames/{{.ID}}/duplicate">
        <button class="btn btn-sm btn-edit" type="submit" title="Add a spare copy">+1</button>
      </form>
      <form class="inline" method="POST" action="/frames/{{.ID}}/delete">
        <button class="btn btn-sm btn-danger" type="submit">Delete</button>
      </form>
    </td>
  </tr>
  {{end}}
  </tbody></table></div>
  {{else}}<p class="muted">No frames. <a href="/frames/new">Add one.</a></p>{{end}}
</div>

<div class="section">
  <div class="section-header">
    <h3>Flight Controllers</h3>
    <a href="/fcs/new" class="btn btn-sm btn-primary">+ Add FC</a>
  </div>
  {{if .FCs}}
  <div class="table-wrap">
  <table>
  <thead><tr><th>Brand</th><th>Name</th><th>MCU</th><th>Firmware</th><th>Avail.</th><th>Installed On</th><th></th></tr></thead>
  <tbody>
  {{range .FCs}}
  <tr class="{{if eq .Status "retired"}}retired{{end}}">
    <td class="muted">{{dash .Brand}}</td>
    <td>{{.Name}}</td>
    <td class="muted">{{dash .MCU}}</td>
    <td class="muted">{{dash .Firmware}}</td>
    <td>{{if gt .Available 0}}<span style="color:#3fb950;font-weight:500">{{.Available}}</span>{{else}}<span class="muted">0</span>{{end}}</td>
    <td>{{if .InstalledOn}}<span class="installed-badge">{{.InstalledOn}}</span>{{else}}<span class="muted">—</span>{{end}}</td>
    <td class="actions-cell">
      <a href="/fcs/{{.ID}}/edit" class="btn btn-sm btn-edit">Edit</a>
      <form class="inline" method="POST" action="/fcs/{{.ID}}/duplicate">
        <button class="btn btn-sm btn-edit" type="submit" title="Add a spare copy">+1</button>
      </form>
      <form class="inline" method="POST" action="/fcs/{{.ID}}/delete">
        <button class="btn btn-sm btn-danger" type="submit">Delete</button>
      </form>
    </td>
  </tr>
  {{end}}
  </tbody></table></div>
  {{else}}<p class="muted">No flight controllers. <a href="/fcs/new">Add one.</a></p>{{end}}
</div>

<div class="section">
  <div class="section-header">
    <h3>ESCs</h3>
    <a href="/escs/new" class="btn btn-sm btn-primary">+ Add ESC</a>
  </div>
  {{if .ESCs}}
  <div class="table-wrap">
  <table>
  <thead><tr><th>Brand</th><th>Name</th><th>Current</th><th>Max Cell</th><th>Avail.</th><th>Installed On</th><th></th></tr></thead>
  <tbody>
  {{range .ESCs}}
  <tr class="{{if eq .Status "retired"}}retired{{end}}">
    <td class="muted">{{dash .Brand}}</td>
    <td>{{.Name}}</td>
    <td class="muted">{{if .CurrentRating}}{{.CurrentRating}}A{{else}}—{{end}}</td>
    <td class="muted">{{if .CellMax}}{{.CellMax}}S{{else}}—{{end}}</td>
    <td>{{if gt .Available 0}}<span style="color:#3fb950;font-weight:500">{{.Available}}</span>{{else}}<span class="muted">0</span>{{end}}</td>
    <td>{{if .InstalledOn}}<span class="installed-badge">{{.InstalledOn}}</span>{{else}}<span class="muted">—</span>{{end}}</td>
    <td class="actions-cell">
      <a href="/escs/{{.ID}}/edit" class="btn btn-sm btn-edit">Edit</a>
      <form class="inline" method="POST" action="/escs/{{.ID}}/duplicate">
        <button class="btn btn-sm btn-edit" type="submit" title="Add a spare copy">+1</button>
      </form>
      <form class="inline" method="POST" action="/escs/{{.ID}}/delete">
        <button class="btn btn-sm btn-danger" type="submit">Delete</button>
      </form>
    </td>
  </tr>
  {{end}}
  </tbody></table></div>
  {{else}}<p class="muted">No ESCs. <a href="/escs/new">Add one.</a></p>{{end}}
</div>

<div class="section">
  <div class="section-header">
    <h3>Motors</h3>
    <a href="/motors/new" class="btn btn-sm btn-primary">+ Add Motor</a>
  </div>
  {{if .Motors}}
  <div class="table-wrap">
  <table>
  <thead><tr><th>Brand</th><th>Name</th><th>Stator</th><th>KV</th><th>Avail.</th><th>Installed On</th><th></th></tr></thead>
  <tbody>
  {{range .Motors}}
  <tr class="{{if eq .Status "retired"}}retired{{end}}">
    <td class="muted">{{dash .Brand}}</td>
    <td>{{.Name}}</td>
    <td class="muted">{{dash .StatorSize}}</td>
    <td class="muted">{{if .KV}}{{.KV}}kv{{else}}—{{end}}</td>
    <td>{{if gt .Available 0}}<span style="color:#3fb950;font-weight:500">{{.Available}}</span>{{else}}<span class="muted">0</span>{{end}}</td>
    <td>{{if .InstalledOn}}<span class="installed-badge">{{.InstalledOn}}</span>{{else}}<span class="muted">—</span>{{end}}</td>
    <td class="actions-cell">
      <a href="/motors/{{.ID}}/edit" class="btn btn-sm btn-edit">Edit</a>
      <form class="inline" method="POST" action="/motors/{{.ID}}/duplicate">
        <button class="btn btn-sm btn-edit" type="submit" title="Add a spare copy">+1</button>
      </form>
      <form class="inline" method="POST" action="/motors/{{.ID}}/delete">
        <button class="btn btn-sm btn-danger" type="submit">Delete</button>
      </form>
    </td>
  </tr>
  {{end}}
  </tbody></table></div>
  {{else}}<p class="muted">No motors. <a href="/motors/new">Add one.</a></p>{{end}}
</div>

<div class="section">
  <div class="section-header">
    <h3>VTX / Air Units</h3>
    <a href="/vtx/new" class="btn btn-sm btn-primary">+ Add VTX</a>
  </div>
  {{if .VTXs}}
  <div class="table-wrap">
  <table>
  <thead><tr><th>Brand</th><th>Name</th><th>System</th><th>Max Power</th><th>Resolution</th><th>Avail.</th><th>Installed On</th><th></th></tr></thead>
  <tbody>
  {{range .VTXs}}
  <tr class="{{if eq .Status "retired"}}retired{{end}}">
    <td class="muted">{{dash .Brand}}</td>
    <td>{{.Name}}</td>
    <td class="muted">{{dash .System}}</td>
    <td class="muted">{{if .MaxPowerMW}}{{.MaxPowerMW}}mW{{else}}—{{end}}</td>
    <td class="muted">{{dash .Resolution}}</td>
    <td>{{if gt .Available 0}}<span style="color:#3fb950;font-weight:500">{{.Available}}</span>{{else}}<span class="muted">0</span>{{end}}</td>
    <td>{{if .InstalledOn}}<span class="installed-badge">{{.InstalledOn}}</span>{{else}}<span class="muted">—</span>{{end}}</td>
    <td class="actions-cell">
      <a href="/vtx/{{.ID}}/edit" class="btn btn-sm btn-edit">Edit</a>
      <form class="inline" method="POST" action="/vtx/{{.ID}}/duplicate">
        <button class="btn btn-sm btn-edit" type="submit" title="Add a spare copy">+1</button>
      </form>
      <form class="inline" method="POST" action="/vtx/{{.ID}}/delete">
        <button class="btn btn-sm btn-danger" type="submit">Delete</button>
      </form>
    </td>
  </tr>
  {{end}}
  </tbody></table></div>
  {{else}}<p class="muted">No VTX units. <a href="/vtx/new">Add one.</a></p>{{end}}
</div>
{{end}}`

const frameFormTmpl = `{{define "content"}}
<div class="page-header">
  <h2>{{if .ID}}Edit Frame{{else}}New Frame{{end}}</h2>
</div>
<div class="form-page">
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
<form method="POST">
  <div class="form-row">
    <div class="form-group">
      <label>Brand</label>
      <input type="text" name="brand" value="{{.Brand}}" placeholder="e.g. iFlight">
    </div>
    <div class="form-group">
      <label>Name *</label>
      <input type="text" name="name" value="{{.Name}}" required autofocus placeholder="e.g. Nazgul5 V3">
    </div>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Size (mm)</label>
      <input type="number" name="size_mm" value="{{.SizeMM}}" placeholder="e.g. 250">
    </div>
    <div class="form-group">
      <label>Weight (g)</label>
      <input type="number" name="weight_g" value="{{.WeightG}}" placeholder="e.g. 78">
    </div>
    <div class="form-group">
      <label>Status</label>
      <select name="status">
        <option value="spare"   {{if eq .Status "spare"}}selected{{end}}>spare</option>
        <option value="crashed" {{if eq .Status "crashed"}}selected{{end}}>crashed</option>
        <option value="retired" {{if eq .Status "retired"}}selected{{end}}>retired</option>
      </select>
    </div>
  </div>
  <div class="form-group">
    <label>Notes</label>
    <textarea name="notes">{{.Notes}}</textarea>
  </div>
  <div class="form-actions">
    <button class="btn btn-primary" type="submit">{{if .ID}}Save{{else}}Add Frame{{end}}</button>
    <a href="/inventory" class="btn btn-cancel">Cancel</a>
  </div>
</form>
</div>
{{end}}`

const fcFormTmpl = `{{define "content"}}
<div class="page-header">
  <h2>{{if .ID}}Edit Flight Controller{{else}}New Flight Controller{{end}}</h2>
</div>
<div class="form-page">
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
<form method="POST">
  <div class="form-row">
    <div class="form-group">
      <label>Brand</label>
      <input type="text" name="brand" value="{{.Brand}}" placeholder="e.g. SpeedyBee">
    </div>
    <div class="form-group">
      <label>Name *</label>
      <input type="text" name="name" value="{{.Name}}" required autofocus placeholder="e.g. F7 V3">
    </div>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>MCU</label>
      <input type="text" name="mcu" value="{{.MCU}}" placeholder="e.g. H743">
    </div>
    <div class="form-group">
      <label>Firmware</label>
      <input type="text" name="firmware" value="{{.Firmware}}" placeholder="e.g. Betaflight 4.5.1">
    </div>
    <div class="form-group">
      <label>Status</label>
      <select name="status">
        <option value="spare"   {{if eq .Status "spare"}}selected{{end}}>spare</option>
        <option value="retired" {{if eq .Status "retired"}}selected{{end}}>retired</option>
      </select>
    </div>
  </div>
  <div class="form-group">
    <label>Notes</label>
    <textarea name="notes">{{.Notes}}</textarea>
  </div>
  <div class="form-actions">
    <button class="btn btn-primary" type="submit">{{if .ID}}Save{{else}}Add FC{{end}}</button>
    <a href="/inventory" class="btn btn-cancel">Cancel</a>
  </div>
</form>
</div>
{{end}}`

const escFormTmpl = `{{define "content"}}
<div class="page-header">
  <h2>{{if .ID}}Edit ESC{{else}}New ESC{{end}}</h2>
</div>
<div class="form-page">
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
<form method="POST">
  <div class="form-row">
    <div class="form-group">
      <label>Brand</label>
      <input type="text" name="brand" value="{{.Brand}}" placeholder="e.g. Mamba">
    </div>
    <div class="form-group">
      <label>Name *</label>
      <input type="text" name="name" value="{{.Name}}" required autofocus placeholder="e.g. F50 Pro 4-in-1">
    </div>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Current Rating (A)</label>
      <input type="number" name="current_rating" value="{{.CurrentRating}}" placeholder="e.g. 45">
    </div>
    <div class="form-group">
      <label>Max Cell (S)</label>
      <input type="number" name="cell_max" value="{{.CellMax}}" placeholder="e.g. 6">
    </div>
    <div class="form-group">
      <label>Status</label>
      <select name="status">
        <option value="spare"   {{if eq .Status "spare"}}selected{{end}}>spare</option>
        <option value="retired" {{if eq .Status "retired"}}selected{{end}}>retired</option>
      </select>
    </div>
  </div>
  <div class="form-group">
    <label>Notes</label>
    <textarea name="notes">{{.Notes}}</textarea>
  </div>
  <div class="form-actions">
    <button class="btn btn-primary" type="submit">{{if .ID}}Save{{else}}Add ESC{{end}}</button>
    <a href="/inventory" class="btn btn-cancel">Cancel</a>
  </div>
</form>
</div>
{{end}}`

const motorFormTmpl = `{{define "content"}}
<div class="page-header">
  <h2>{{if .ID}}Edit Motor{{else}}New Motor{{end}}</h2>
</div>
<div class="form-page">
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
<form method="POST">
  <div class="form-row">
    <div class="form-group">
      <label>Brand</label>
      <input type="text" name="brand" value="{{.Brand}}" placeholder="e.g. T-Motor">
    </div>
    <div class="form-group">
      <label>Name *</label>
      <input type="text" name="name" value="{{.Name}}" required autofocus placeholder="e.g. F1507 3800KV">
    </div>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Stator Size</label>
      <input type="text" name="stator_size" value="{{.StatorSize}}" placeholder="e.g. 1507">
    </div>
    <div class="form-group">
      <label>KV</label>
      <input type="number" name="kv" value="{{.KV}}" placeholder="e.g. 3800">
    </div>
    <div class="form-group">
      <label>Status</label>
      <select name="status">
        <option value="spare"   {{if eq .Status "spare"}}selected{{end}}>spare</option>
        <option value="retired" {{if eq .Status "retired"}}selected{{end}}>retired</option>
      </select>
    </div>
  </div>
  <div class="form-group">
    <label>Installed on Drone</label>
    <select name="drone_id">
      <option value="">— spare (unassigned) —</option>
      {{range .Drones}}
      <option value="{{.ID}}" {{if eq $.DroneID .ID}}selected{{end}}>{{.Label}}</option>
      {{end}}
    </select>
  </div>
  <div class="form-group">
    <label>Notes</label>
    <textarea name="notes">{{.Notes}}</textarea>
  </div>
  <div class="form-actions">
    <button class="btn btn-primary" type="submit">{{if .ID}}Save{{else}}Add Motor{{end}}</button>
    <a href="/inventory" class="btn btn-cancel">Cancel</a>
  </div>
</form>
</div>
{{end}}`

const vtxFormTmpl = `{{define "content"}}
<div class="page-header">
  <h2>{{if .ID}}Edit VTX / Air Unit{{else}}New VTX / Air Unit{{end}}</h2>
</div>
<div class="form-page">
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
<form method="POST">
  <div class="form-row">
    <div class="form-group">
      <label>Brand</label>
      <input type="text" name="brand" value="{{.Brand}}" placeholder="e.g. DJI">
    </div>
    <div class="form-group">
      <label>Name *</label>
      <input type="text" name="name" value="{{.Name}}" required autofocus placeholder="e.g. O3 Air Unit">
    </div>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>System</label>
      <input type="text" name="system" value="{{.System}}" placeholder="e.g. DJI O3">
    </div>
    <div class="form-group">
      <label>Max Power (mW)</label>
      <input type="number" name="max_power_mw" value="{{.MaxPowerMW}}" placeholder="e.g. 700">
    </div>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Resolution</label>
      <input type="text" name="resolution" value="{{.Resolution}}" placeholder="e.g. 1080p60">
    </div>
    <div class="form-group">
      <label>Weight (g)</label>
      <input type="number" name="weight_g" value="{{.WeightG}}" placeholder="e.g. 28">
    </div>
    <div class="form-group">
      <label>Status</label>
      <select name="status">
        <option value="spare"   {{if eq .Status "spare"}}selected{{end}}>spare</option>
        <option value="retired" {{if eq .Status "retired"}}selected{{end}}>retired</option>
      </select>
    </div>
  </div>
  <div class="form-group">
    <label>Notes</label>
    <textarea name="notes">{{.Notes}}</textarea>
  </div>
  <div class="form-actions">
    <button class="btn btn-primary" type="submit">{{if .ID}}Save{{else}}Add VTX{{end}}</button>
    <a href="/inventory" class="btn btn-cancel">Cancel</a>
  </div>
</form>
</div>
{{end}}`

const batteryListTmpl = `{{define "content"}}
<div class="page-header">
  <div class="page-header-left"><h2>Batteries</h2></div>
  <a href="/batteries/new" class="btn btn-primary">+ Add Battery</a>
</div>
{{if .Batteries}}
<div class="table-wrap">
<table>
<thead><tr>
  <th>Name</th><th>Brand</th><th>Cell</th><th>mAh</th><th>Cycles</th>
  <th>IR (mΩ)</th><th>Purchased</th><th>Drone</th><th>Status</th><th></th>
</tr></thead>
<tbody>
{{range .Batteries}}
<tr class="{{if eq .Status "dead"}}retired{{end}}">
  <td><strong>{{.Name}}</strong></td>
  <td class="muted">{{dash .Brand}}</td>
  <td class="muted">{{.CellCount}}S</td>
  <td class="muted">{{.CapacityMAh}}</td>
  <td>{{.CycleCount}}</td>
  <td class="muted">{{dash .InternalResistance}}</td>
  <td class="muted">{{dash .PurchaseDate}}</td>
  <td class="muted">{{dash .DroneName}}</td>
  <td><span class="badge {{badgeClass .Status}}">{{.Status}}</span></td>
  <td class="actions-cell">
    <a href="/batteries/{{.ID}}/edit" class="btn btn-sm btn-edit">Edit</a>
    <form class="inline" method="POST" action="/batteries/{{.ID}}/delete">
      <button class="btn btn-sm btn-danger" type="submit">Delete</button>
    </form>
  </td>
</tr>
{{end}}
</tbody>
</table>
</div>
{{else}}
<p class="muted">No batteries. <a href="/batteries/new">Add one.</a></p>
{{end}}
{{end}}`

const batteryFormTmpl = `{{define "content"}}
<div class="page-header">
  <h2>{{if .ID}}Edit Battery{{else}}New Battery{{end}}</h2>
</div>
<div class="form-page">
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
<form method="POST">
  <div class="form-row">
    <div class="form-group">
      <label>Name *</label>
      <input type="text" name="name" value="{{.Name}}" required autofocus placeholder="e.g. CNHL 650 #1">
    </div>
    <div class="form-group">
      <label>Brand</label>
      <input type="text" name="brand" value="{{.Brand}}" placeholder="e.g. CNHL">
    </div>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Cell Count (S) *</label>
      <input type="number" name="cell_count" value="{{.CellCount}}" required min="1" max="8" placeholder="e.g. 4">
    </div>
    <div class="form-group">
      <label>Capacity (mAh) *</label>
      <input type="number" name="capacity_mah" value="{{.CapacityMAh}}" required placeholder="e.g. 650">
    </div>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Cycle Count</label>
      <input type="number" name="cycle_count" value="{{.CycleCount}}" min="0">
    </div>
    <div class="form-group">
      <label>Internal Resistance (mΩ)</label>
      <input type="number" name="internal_resistance" value="{{.InternalResistance}}" placeholder="optional">
    </div>
    <div class="form-group">
      <label>Purchase Date</label>
      <input type="date" name="purchase_date" value="{{.PurchaseDate}}">
    </div>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Primary Drone</label>
      <select name="drone_id">
        <option value="">— unassigned —</option>
        {{range .Drones}}
        <option value="{{.ID}}" {{if eq $.DroneID .ID}}selected{{end}}>{{.Label}}</option>
        {{end}}
      </select>
    </div>
    <div class="form-group">
      <label>Status</label>
      <select name="status">
        <option value="good"     {{if eq .Status "good"}}selected{{end}}>good</option>
        <option value="degraded" {{if eq .Status "degraded"}}selected{{end}}>degraded</option>
        <option value="storage"  {{if eq .Status "storage"}}selected{{end}}>storage</option>
        <option value="dead"     {{if eq .Status "dead"}}selected{{end}}>dead</option>
      </select>
    </div>
  </div>
  <div class="form-actions">
    <button class="btn btn-primary" type="submit">{{if .ID}}Save{{else}}Add Battery{{end}}</button>
    <a href="/batteries" class="btn btn-cancel">Cancel</a>
  </div>
</form>
</div>
{{end}}`

const propListTmpl = `{{define "content"}}
<div class="page-header">
  <div class="page-header-left"><h2>Props</h2></div>
  <a href="/props/new" class="btn btn-primary">+ Add Props</a>
</div>
{{if .Propellers}}
<div class="table-wrap">
<table>
<thead><tr>
  <th>Brand</th><th>Name</th><th>Size</th><th>Pitch</th><th>Blades</th>
  <th>Material</th><th>Qty</th><th>Reorder At</th><th>Drone</th><th></th>
</tr></thead>
<tbody>
{{range .Propellers}}
<tr class="{{if .LowStock}}low-stock{{end}}">
  <td class="muted">{{dash .Brand}}</td>
  <td>{{.Name}}</td>
  <td class="muted">{{if .SizeInch}}{{.SizeInch}}"{{else}}—{{end}}</td>
  <td class="muted">{{dash .Pitch}}</td>
  <td class="muted">{{.BladeCount}}</td>
  <td class="muted">{{dash .Material}}</td>
  <td>{{.Quantity}}{{if .LowStock}} <span class="muted" title="low stock">&#9888;</span>{{end}}</td>
  <td class="muted">{{.ReorderThreshold}}</td>
  <td class="muted">{{dash .DroneName}}</td>
  <td class="actions-cell">
    <a href="/props/{{.ID}}/edit" class="btn btn-sm btn-edit">Edit</a>
    <form class="inline" method="POST" action="/props/{{.ID}}/delete">
      <button class="btn btn-sm btn-danger" type="submit">Delete</button>
    </form>
  </td>
</tr>
{{end}}
</tbody>
</table>
</div>
{{else}}
<p class="muted">No props. <a href="/props/new">Add some.</a></p>
{{end}}
{{end}}`

const propFormTmpl = `{{define "content"}}
<div class="page-header">
  <h2>{{if .ID}}Edit Props{{else}}New Props{{end}}</h2>
</div>
<div class="form-page">
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
<form method="POST">
  <div class="form-row">
    <div class="form-group">
      <label>Brand</label>
      <input type="text" name="brand" value="{{.Brand}}" placeholder="e.g. HQ">
    </div>
    <div class="form-group">
      <label>Name / Model *</label>
      <input type="text" name="name" value="{{.Name}}" required autofocus placeholder="e.g. 5x4.3x3 V1S">
    </div>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Size (inch)</label>
      <input type="number" name="size_inch" step="0.1" value="{{.SizeInch}}" placeholder="e.g. 5.1">
    </div>
    <div class="form-group">
      <label>Pitch</label>
      <input type="number" name="pitch" step="0.1" value="{{.Pitch}}" placeholder="e.g. 4.3">
    </div>
    <div class="form-group">
      <label>Blades</label>
      <input type="number" name="blade_count" value="{{.BladeCount}}" min="2" max="6" placeholder="3">
    </div>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Material</label>
      <input type="text" name="material" value="{{.Material}}" placeholder="e.g. PC">
    </div>
    <div class="form-group">
      <label>Quantity</label>
      <input type="number" name="quantity" value="{{.Quantity}}" min="0">
    </div>
    <div class="form-group">
      <label>Reorder At</label>
      <input type="number" name="reorder_threshold" value="{{.ReorderThreshold}}" min="0">
    </div>
  </div>
  <div class="form-group">
    <label>Primary Drone</label>
    <select name="drone_id">
      <option value="">— unassigned —</option>
      {{range .Drones}}
      <option value="{{.ID}}" {{if eq $.DroneID .ID}}selected{{end}}>{{.Label}}</option>
      {{end}}
    </select>
  </div>
  <div class="form-group">
    <label>Notes</label>
    <textarea name="notes">{{.Notes}}</textarea>
  </div>
  <div class="form-actions">
    <button class="btn btn-primary" type="submit">{{if .ID}}Save{{else}}Add Props{{end}}</button>
    <a href="/props" class="btn btn-cancel">Cancel</a>
  </div>
</form>
</div>
{{end}}`

const partsListTmpl = `{{define "content"}}
<div class="page-header">
  <div class="page-header-left"><h2>Spare Parts</h2></div>
  <a href="/parts/new" class="btn btn-primary">+ Add Part</a>
</div>
{{if .Parts}}
<div class="table-wrap">
<table>
<thead><tr>
  <th>Category</th><th>Name</th><th>Qty</th><th>Reorder At</th><th>Unit Price</th><th></th>
</tr></thead>
<tbody>
{{range .Parts}}
<tr class="{{if .LowStock}}low-stock{{end}}">
  <td><span class="badge badge-retired">{{.Category}}</span></td>
  <td>{{.Name}}</td>
  <td>{{.Quantity}}{{if .LowStock}} <span class="muted" title="low stock">&#9888;</span>{{end}}</td>
  <td class="muted">{{.ReorderThreshold}}</td>
  <td class="muted">{{.UnitPrice}}</td>
  <td class="actions-cell">
    <a href="/parts/{{.ID}}/edit" class="btn btn-sm btn-edit">Edit</a>
    <form class="inline" method="POST" action="/parts/{{.ID}}/delete">
      <button class="btn btn-sm btn-danger" type="submit">Delete</button>
    </form>
  </td>
</tr>
{{end}}
</tbody>
</table>
</div>
{{else}}
<p class="muted">No spare parts tracked. <a href="/parts/new">Add one.</a></p>
{{end}}
{{end}}`

const partFormTmpl = `{{define "content"}}
<div class="page-header">
  <h2>{{if .ID}}Edit Part{{else}}New Part{{end}}</h2>
</div>
<div class="form-page">
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
<form method="POST">
  <div class="form-row">
    <div class="form-group">
      <label>Category</label>
      <select name="category">
        <option value="antennas" {{if eq .Category "antennas"}}selected{{end}}>antennas</option>
        <option value="misc"     {{if eq .Category "misc"}}selected{{end}}>misc</option>
      </select>
    </div>
    <div class="form-group">
      <label>Name *</label>
      <input type="text" name="name" value="{{.Name}}" required autofocus>
    </div>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Quantity</label>
      <input type="number" name="quantity" value="{{.Quantity}}" min="0">
    </div>
    <div class="form-group">
      <label>Reorder At</label>
      <input type="number" name="reorder_threshold" value="{{.ReorderThreshold}}" min="0">
    </div>
    <div class="form-group">
      <label>Unit Price ($)</label>
      <input type="text" name="unit_price" value="{{.UnitPrice}}" placeholder="0.00">
    </div>
  </div>
  <div class="form-actions">
    <button class="btn btn-primary" type="submit">{{if .ID}}Save{{else}}Add Part{{end}}</button>
    <a href="/parts" class="btn btn-cancel">Cancel</a>
  </div>
</form>
</div>
{{end}}`

const logListTmpl = `{{define "content"}}
<div class="page-header">
  <div class="page-header-left"><h2>Flight Log</h2></div>
  <a href="/log/new" class="btn btn-primary">+ Log Session</a>
</div>
{{if .Sessions}}
<div class="table-wrap">
<table>
<thead><tr>
  <th>Date</th><th>Drone</th><th>Type</th><th>Duration</th><th>Location</th><th>Batteries</th><th>Notes</th><th></th>
</tr></thead>
<tbody>
{{range .Sessions}}
<tr>
  <td class="muted" style="white-space:nowrap">{{.SessionDate}}</td>
  <td><strong>{{.DroneName}}</strong></td>
  <td><span class="badge {{badgeClass .Type}}">{{.Type}}</span></td>
  <td class="muted">{{if gt .DurationMin 0}}{{.DurationMin}}m{{else}}—{{end}}</td>
  <td class="muted">{{dash .Location}}</td>
  <td class="muted" style="font-size:12px">{{dash .BatteryList}}</td>
  <td class="muted" style="max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{{dash .Notes}}</td>
  <td class="actions-cell">
    <a href="/log/{{.ID}}" class="btn btn-sm btn-edit">View</a>
    <a href="/log/{{.ID}}/edit" class="btn btn-sm btn-edit">Edit</a>
    <form class="inline" method="POST" action="/log/{{.ID}}/delete">
      <button class="btn btn-sm btn-danger" type="submit">Delete</button>
    </form>
  </td>
</tr>
{{end}}
</tbody>
</table>
</div>
{{else}}
<p class="muted">No sessions logged. <a href="/log/new">Log your first session.</a></p>
{{end}}
{{end}}`

const sessionFormTmpl = `{{define "content"}}
<div class="page-header">
  <h2>{{if .ID}}Edit Session{{else}}Log Session{{end}}</h2>
</div>
<div class="form-page">
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
<form method="POST">
  <div class="form-row">
    <div class="form-group">
      <label>Drone *</label>
      <select name="drone_id" required>
        <option value="">— select drone —</option>
        {{range .Drones}}
        <option value="{{.ID}}" {{if eq $.DroneID .ID}}selected{{end}}>{{.Label}}</option>
        {{end}}
      </select>
    </div>
    <div class="form-group">
      <label>Type</label>
      <select name="type">
        <option value="flight"      {{if eq .Type "flight"}}selected{{end}}>flight</option>
        <option value="maintenance" {{if eq .Type "maintenance"}}selected{{end}}>maintenance</option>
        <option value="crash"       {{if eq .Type "crash"}}selected{{end}}>crash</option>
      </select>
    </div>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Date</label>
      <input type="date" name="session_date" value="{{.SessionDate}}" required>
    </div>
    <div class="form-group">
      <label>Duration (minutes)</label>
      <input type="number" name="duration_min" value="{{.DurationMin}}" min="0">
    </div>
    <div class="form-group">
      <label>Location</label>
      <input type="text" name="location" value="{{.Location}}" placeholder="e.g. backyard">
    </div>
  </div>
  {{if .Batteries}}
  <div class="form-group">
    <label>Batteries Used</label>
    <div class="battery-checks">
      {{range .Batteries}}
      <label class="battery-check">
        <input type="checkbox" name="battery_ids" value="{{.ID}}" {{if .Checked}}checked{{end}}>
        {{.Label}}
      </label>
      {{end}}
    </div>
  </div>
  {{end}}
  <div class="form-group">
    <label>Notes</label>
    <textarea name="notes">{{.Notes}}</textarea>
  </div>
  <div class="form-actions">
    <button class="btn btn-primary" type="submit">{{if .ID}}Save{{else}}Log Session{{end}}</button>
    <a href="/log" class="btn btn-cancel">Cancel</a>
  </div>
</form>
</div>
{{end}}`

const sessionDetailTmpl = `{{define "content"}}
<div class="page-header">
  <div class="page-header-left">
    <h2>Session #{{.ID}}</h2>
    <div class="summary">{{.DroneName}} &mdash; {{.Date}}</div>
  </div>
  <div>
    <a href="/log/{{.ID}}/edit" class="btn btn-edit">Edit</a>
    <form class="inline" method="POST" action="/log/{{.ID}}/delete">
      <button class="btn btn-danger" type="submit">Delete</button>
    </form>
  </div>
</div>
<table style="max-width:500px;margin-bottom:24px">
<tr><td class="muted" style="padding:6px 12px 6px 0;width:120px">Drone</td><td>{{.DroneName}}</td></tr>
<tr><td class="muted" style="padding:6px 12px 6px 0">Type</td><td><span class="badge {{badgeClass .Type}}">{{.Type}}</span></td></tr>
<tr><td class="muted" style="padding:6px 12px 6px 0">Date</td><td>{{.Date}}</td></tr>
<tr><td class="muted" style="padding:6px 12px 6px 0">Duration</td><td>{{if gt .Duration 0}}{{.Duration}} min{{else}}—{{end}}</td></tr>
<tr><td class="muted" style="padding:6px 12px 6px 0">Location</td><td>{{dash .Location}}</td></tr>
<tr><td class="muted" style="padding:6px 12px 6px 0;vertical-align:top">Notes</td><td style="white-space:pre-wrap">{{dash .Notes}}</td></tr>
</table>

{{if .Batteries}}
<h3>Batteries Used</h3>
<div class="table-wrap" style="max-width:600px">
<table>
<thead><tr><th>Name</th><th>Cell</th><th>mAh</th><th>Cycles (after)</th><th>Status</th></tr></thead>
<tbody>
{{range .Batteries}}
<tr>
  <td>{{.Name}}</td>
  <td class="muted">{{.CellCount}}S</td>
  <td class="muted">{{.CapacityMAh}}</td>
  <td>{{.CycleCount}}</td>
  <td><span class="badge {{badgeClass .Status}}">{{.Status}}</span></td>
</tr>
{{end}}
</tbody>
</table>
</div>
{{else}}
<p class="muted">No batteries recorded for this session.</p>
{{end}}

<p style="margin-top:24px"><a href="/log">&larr; Back to log</a></p>
{{end}}`
