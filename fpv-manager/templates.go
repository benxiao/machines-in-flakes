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
	ID           int
	Name         string
	FrameName    string
	FCName       string
	ESCName      string
	VTXName      string
	MotorName    string
	MotorCount   int
	BatteryName  string
	BatteryCount int
	GPSName      string
	RXName       string
	Status       string
	BuildDate    string
}

type DroneFormPage struct {
	ActiveTab    string
	Error        string
	ID           int
	Name         string
	FrameID      int
	FCID         int
	ESCID        int
	VTXID        int
	MotorID      int
	MotorCount   string
	BatteryID    int
	BatteryCount string
	GPSID        int
	RXID         int
	Status       string
	BuildDate    string
	Notes        string
	Frames       []OptionItem
	FCs          []OptionItem
	ESCs         []OptionItem
	VTXs         []OptionItem
	Motors       []OptionItem
	Batteries    []OptionItem
	GPSs         []OptionItem
	RXs          []OptionItem
}

type InventoryPage struct {
	ActiveTab string
	Frames    []FrameRow
	FCs       []FCRow
	ESCs      []ESCRow
	Motors    []MotorRow
	VTXs      []VTXRow
	GPSs      []GPSRow
	RXs       []RXRow
}

type GPSRow struct {
	ID          int
	Brand       string
	Name        string
	Total       int
	Installed   int
	Available   int
	InstalledOn string
}

type RXRow struct {
	ID          int
	Brand       string
	Name        string
	Protocol    string
	Total       int
	Installed   int
	Available   int
	InstalledOn string
}

type GPSFormPage struct {
	ActiveTab string
	Error     string
	ID        int
	Brand     string
	Name      string
	Notes     string
	Quantity  string
}

type RXFormPage struct {
	ActiveTab string
	Error     string
	ID        int
	Brand     string
	Name      string
	Protocol  string
	Notes     string
	Quantity  string
}

type FrameRow struct {
	ID          int
	Brand       string
	Name        string
	SizeMM      string
	WeightG     string
	Total       int
	Installed   int
	Available   int
	InstalledOn string
}

type FCRow struct {
	ID          int
	Brand       string
	Name        string
	MCU         string
	Firmware    string
	Total       int
	Installed   int
	Available   int
	InstalledOn string
}

type ESCRow struct {
	ID            int
	Brand         string
	Name          string
	CurrentRating string
	CellMax       string
	Total         int
	Installed     int
	Available     int
	InstalledOn   string
}

type MotorRow struct {
	ID          int
	Brand       string
	Name        string
	StatorSize  string
	KV          string
	Total       int
	Installed   int
	Available   int
	InstalledOn string
}

type VTXRow struct {
	ID          int
	Brand       string
	Name        string
	System      string
	MaxPowerMW  string
	Resolution  string
	Total       int
	Installed   int
	Available   int
	InstalledOn string
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
	Quantity  string
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
	Quantity  string
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
	Quantity      string
}

type MotorFormPage struct {
	ActiveTab  string
	Error      string
	ID         int
	Brand      string
	Name       string
	StatorSize string
	KV         string
	Notes      string
	Quantity   string
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
	Quantity   string
}

type BatteryListPage struct {
	ActiveTab string
	Batteries []BatteryRow
}

type BatteryRow struct {
	ID          int
	Brand       string
	Name        string
	CellCount   int
	CapacityMAh int
	Total       int
	Installed   int
	Available   int
	InstalledOn string
	Status      string
}

type BatteryFormPage struct {
	ActiveTab   string
	Error       string
	ID          int
	Brand       string
	Name        string
	CellCount   string
	CapacityMAh string
	Quantity    string
	Status      string
	Notes       string
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

type LogListPage struct {
	ActiveTab string
	Sessions  []SessionRow
}

type SessionRow struct {
	ID          int
	DroneNames  string
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

type DroneCheck struct {
	ID      int
	Label   string
	Checked bool
}

type VideoRow struct {
	ID           int
	OriginalName string
	Notes        string
}

type PhotoRow struct {
	ID           int
	OriginalName string
	Notes        string
}

type SessionFormPage struct {
	ActiveTab   string
	Error       string
	ID          int
	Type        string
	SessionDate string
	DurationMin string
	Location    string
	Notes       string
	Drones      []DroneCheck
	Batteries   []BatteryCheck
}

type SessionDetailPage struct {
	ActiveTab  string
	ID         int
	DroneNames string
	Type       string
	Date       string
	Duration   int
	Location   string
	Notes      string
	Batteries  []BatteryRow
	Videos     []VideoRow
	Photos     []PhotoRow
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
	add("gps-form", gpsFormTmpl)
	add("rx-form", rxFormTmpl)
	add("battery-list", batteryListTmpl)
	add("battery-form", batteryFormTmpl)
	add("prop-list", propListTmpl)
	add("prop-form", propFormTmpl)
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
  <th>Motors</th><th>Batteries</th><th>GPS</th><th>RX</th><th>Status</th><th>Build Date</th><th></th>
</tr></thead>
<tbody>
{{range .Drones}}
<tr class="{{if eq .Status "retired"}}retired{{end}}">
  <td><strong>{{.Name}}</strong></td>
  <td class="muted">{{dash .FrameName}}</td>
  <td class="muted">{{dash .FCName}}</td>
  <td class="muted">{{dash .ESCName}}</td>
  <td class="muted">{{dash .VTXName}}</td>
  <td class="muted">{{if .MotorName}}{{.MotorName}} ×{{.MotorCount}}{{else}}—{{end}}</td>
  <td class="muted">{{if .BatteryName}}{{.BatteryName}} ×{{.BatteryCount}}{{else}}—{{end}}</td>
  <td class="muted">{{dash .GPSName}}</td>
  <td class="muted">{{dash .RXName}}</td>
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
  <div class="form-row">
    <div class="form-group">
      <label>Motors</label>
      <select name="motor_id">
        <option value="">— none —</option>
        {{range .Motors}}
        <option value="{{.ID}}" {{if eq $.MotorID .ID}}selected{{end}}>{{.Label}}</option>
        {{end}}
      </select>
    </div>
    <div class="form-group" style="max-width:100px">
      <label>Motor count</label>
      <input type="number" name="motor_count" value="{{if .MotorCount}}{{.MotorCount}}{{else}}4{{end}}" min="1">
    </div>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Batteries</label>
      <select name="battery_id">
        <option value="">— none —</option>
        {{range .Batteries}}
        <option value="{{.ID}}" {{if eq $.BatteryID .ID}}selected{{end}}>{{.Label}}</option>
        {{end}}
      </select>
    </div>
    <div class="form-group" style="max-width:100px">
      <label>Battery count</label>
      <input type="number" name="battery_count" value="{{if .BatteryCount}}{{.BatteryCount}}{{else}}1{{end}}" min="1">
    </div>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>GPS Module</label>
      <select name="gps_id">
        <option value="">— none —</option>
        {{range .GPSs}}
        <option value="{{.ID}}" {{if eq $.GPSID .ID}}selected{{end}}>{{.Label}}</option>
        {{end}}
      </select>
    </div>
    <div class="form-group">
      <label>Radio Receiver</label>
      <select name="rx_id">
        <option value="">— none —</option>
        {{range .RXs}}
        <option value="{{.ID}}" {{if eq $.RXID .ID}}selected{{end}}>{{.Label}}</option>
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
  <thead><tr><th>Brand</th><th>Name</th><th>Size</th><th>Weight</th><th>Owned</th><th>Installed</th><th>Avail.</th><th>Installed On</th><th></th></tr></thead>
  <tbody>
  {{range .Frames}}
  <tr>
    <td class="muted">{{dash .Brand}}</td>
    <td>{{.Name}}</td>
    <td class="muted">{{if .SizeMM}}{{.SizeMM}}mm{{else}}—{{end}}</td>
    <td class="muted">{{if .WeightG}}{{.WeightG}}g{{else}}—{{end}}</td>
    <td class="muted">{{.Total}}</td>
    <td class="muted">{{.Installed}}</td>
    <td>{{if gt .Available 0}}<span style="color:#3fb950;font-weight:500">{{.Available}}</span>{{else}}<span class="muted">0</span>{{end}}</td>
    <td>{{if .InstalledOn}}<span class="installed-badge">{{.InstalledOn}}</span>{{else}}<span class="muted">—</span>{{end}}</td>
    <td class="actions-cell">
      <a href="/frames/{{.ID}}/edit" class="btn btn-sm btn-edit">Edit</a>
      <form class="inline" method="POST" action="/frames/{{.ID}}/adjust">
        <input type="number" name="count" placeholder="±" style="width:46px;padding:2px 4px;vertical-align:middle">
        <button class="btn btn-sm btn-edit" type="submit">Apply</button>
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
  <thead><tr><th>Brand</th><th>Name</th><th>MCU</th><th>Firmware</th><th>Owned</th><th>Installed</th><th>Avail.</th><th>Installed On</th><th></th></tr></thead>
  <tbody>
  {{range .FCs}}
  <tr>
    <td class="muted">{{dash .Brand}}</td>
    <td>{{.Name}}</td>
    <td class="muted">{{dash .MCU}}</td>
    <td class="muted">{{dash .Firmware}}</td>
    <td class="muted">{{.Total}}</td>
    <td class="muted">{{.Installed}}</td>
    <td>{{if gt .Available 0}}<span style="color:#3fb950;font-weight:500">{{.Available}}</span>{{else}}<span class="muted">0</span>{{end}}</td>
    <td>{{if .InstalledOn}}<span class="installed-badge">{{.InstalledOn}}</span>{{else}}<span class="muted">—</span>{{end}}</td>
    <td class="actions-cell">
      <a href="/fcs/{{.ID}}/edit" class="btn btn-sm btn-edit">Edit</a>
      <form class="inline" method="POST" action="/fcs/{{.ID}}/adjust">
        <input type="number" name="count" placeholder="±" style="width:46px;padding:2px 4px;vertical-align:middle">
        <button class="btn btn-sm btn-edit" type="submit">Apply</button>
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
  <thead><tr><th>Brand</th><th>Name</th><th>Current</th><th>Max Cell</th><th>Owned</th><th>Installed</th><th>Avail.</th><th>Installed On</th><th></th></tr></thead>
  <tbody>
  {{range .ESCs}}
  <tr>
    <td class="muted">{{dash .Brand}}</td>
    <td>{{.Name}}</td>
    <td class="muted">{{if .CurrentRating}}{{.CurrentRating}}A{{else}}—{{end}}</td>
    <td class="muted">{{if .CellMax}}{{.CellMax}}S{{else}}—{{end}}</td>
    <td class="muted">{{.Total}}</td>
    <td class="muted">{{.Installed}}</td>
    <td>{{if gt .Available 0}}<span style="color:#3fb950;font-weight:500">{{.Available}}</span>{{else}}<span class="muted">0</span>{{end}}</td>
    <td>{{if .InstalledOn}}<span class="installed-badge">{{.InstalledOn}}</span>{{else}}<span class="muted">—</span>{{end}}</td>
    <td class="actions-cell">
      <a href="/escs/{{.ID}}/edit" class="btn btn-sm btn-edit">Edit</a>
      <form class="inline" method="POST" action="/escs/{{.ID}}/adjust">
        <input type="number" name="count" placeholder="±" style="width:46px;padding:2px 4px;vertical-align:middle">
        <button class="btn btn-sm btn-edit" type="submit">Apply</button>
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
  <thead><tr><th>Brand</th><th>Name</th><th>Stator</th><th>KV</th><th>Owned</th><th>Installed</th><th>Avail.</th><th>Installed On</th><th></th></tr></thead>
  <tbody>
  {{range .Motors}}
  <tr>
    <td class="muted">{{dash .Brand}}</td>
    <td>{{.Name}}</td>
    <td class="muted">{{dash .StatorSize}}</td>
    <td class="muted">{{if .KV}}{{.KV}}kv{{else}}—{{end}}</td>
    <td class="muted">{{.Total}}</td>
    <td class="muted">{{.Installed}}</td>
    <td>{{if gt .Available 0}}<span style="color:#3fb950;font-weight:500">{{.Available}}</span>{{else}}<span class="muted">0</span>{{end}}</td>
    <td>{{if .InstalledOn}}<span class="installed-badge">{{.InstalledOn}}</span>{{else}}<span class="muted">—</span>{{end}}</td>
    <td class="actions-cell">
      <a href="/motors/{{.ID}}/edit" class="btn btn-sm btn-edit">Edit</a>
      <form class="inline" method="POST" action="/motors/{{.ID}}/adjust">
        <input type="number" name="count" placeholder="±" style="width:46px;padding:2px 4px;vertical-align:middle">
        <button class="btn btn-sm btn-edit" type="submit">Apply</button>
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
  <thead><tr><th>Brand</th><th>Name</th><th>System</th><th>Max Power</th><th>Resolution</th><th>Owned</th><th>Installed</th><th>Avail.</th><th>Installed On</th><th></th></tr></thead>
  <tbody>
  {{range .VTXs}}
  <tr>
    <td class="muted">{{dash .Brand}}</td>
    <td>{{.Name}}</td>
    <td class="muted">{{dash .System}}</td>
    <td class="muted">{{if .MaxPowerMW}}{{.MaxPowerMW}}mW{{else}}—{{end}}</td>
    <td class="muted">{{dash .Resolution}}</td>
    <td class="muted">{{.Total}}</td>
    <td class="muted">{{.Installed}}</td>
    <td>{{if gt .Available 0}}<span style="color:#3fb950;font-weight:500">{{.Available}}</span>{{else}}<span class="muted">0</span>{{end}}</td>
    <td>{{if .InstalledOn}}<span class="installed-badge">{{.InstalledOn}}</span>{{else}}<span class="muted">—</span>{{end}}</td>
    <td class="actions-cell">
      <a href="/vtx/{{.ID}}/edit" class="btn btn-sm btn-edit">Edit</a>
      <form class="inline" method="POST" action="/vtx/{{.ID}}/adjust">
        <input type="number" name="count" placeholder="±" style="width:46px;padding:2px 4px;vertical-align:middle">
        <button class="btn btn-sm btn-edit" type="submit">Apply</button>
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

<div class="section">
  <div class="section-header">
    <h3>GPS Modules</h3>
    <a href="/gps/new" class="btn btn-sm btn-primary">+ Add GPS</a>
  </div>
  {{if .GPSs}}
  <div class="table-wrap">
  <table>
  <thead><tr><th>Brand</th><th>Name</th><th>Owned</th><th>Installed</th><th>Avail.</th><th>Installed On</th><th></th></tr></thead>
  <tbody>
  {{range .GPSs}}
  <tr>
    <td class="muted">{{dash .Brand}}</td>
    <td>{{.Name}}</td>
    <td class="muted">{{.Total}}</td>
    <td class="muted">{{.Installed}}</td>
    <td>{{if gt .Available 0}}<span style="color:#3fb950;font-weight:500">{{.Available}}</span>{{else}}<span class="muted">0</span>{{end}}</td>
    <td>{{if .InstalledOn}}<span class="installed-badge">{{.InstalledOn}}</span>{{else}}<span class="muted">—</span>{{end}}</td>
    <td class="actions-cell">
      <a href="/gps/{{.ID}}/edit" class="btn btn-sm btn-edit">Edit</a>
      <form class="inline" method="POST" action="/gps/{{.ID}}/adjust">
        <input type="number" name="count" placeholder="±" style="width:46px;padding:2px 4px;vertical-align:middle">
        <button class="btn btn-sm btn-edit" type="submit">Apply</button>
      </form>
      <form class="inline" method="POST" action="/gps/{{.ID}}/delete">
        <button class="btn btn-sm btn-danger" type="submit">Delete</button>
      </form>
    </td>
  </tr>
  {{end}}
  </tbody></table></div>
  {{else}}<p class="muted">No GPS modules. <a href="/gps/new">Add one.</a></p>{{end}}
</div>

<div class="section">
  <div class="section-header">
    <h3>Radio Receivers</h3>
    <a href="/rx/new" class="btn btn-sm btn-primary">+ Add RX</a>
  </div>
  {{if .RXs}}
  <div class="table-wrap">
  <table>
  <thead><tr><th>Brand</th><th>Name</th><th>Protocol</th><th>Owned</th><th>Installed</th><th>Avail.</th><th>Installed On</th><th></th></tr></thead>
  <tbody>
  {{range .RXs}}
  <tr>
    <td class="muted">{{dash .Brand}}</td>
    <td>{{.Name}}</td>
    <td class="muted">{{dash .Protocol}}</td>
    <td class="muted">{{.Total}}</td>
    <td class="muted">{{.Installed}}</td>
    <td>{{if gt .Available 0}}<span style="color:#3fb950;font-weight:500">{{.Available}}</span>{{else}}<span class="muted">0</span>{{end}}</td>
    <td>{{if .InstalledOn}}<span class="installed-badge">{{.InstalledOn}}</span>{{else}}<span class="muted">—</span>{{end}}</td>
    <td class="actions-cell">
      <a href="/rx/{{.ID}}/edit" class="btn btn-sm btn-edit">Edit</a>
      <form class="inline" method="POST" action="/rx/{{.ID}}/adjust">
        <input type="number" name="count" placeholder="±" style="width:46px;padding:2px 4px;vertical-align:middle">
        <button class="btn btn-sm btn-edit" type="submit">Apply</button>
      </form>
      <form class="inline" method="POST" action="/rx/{{.ID}}/delete">
        <button class="btn btn-sm btn-danger" type="submit">Delete</button>
      </form>
    </td>
  </tr>
  {{end}}
  </tbody></table></div>
  {{else}}<p class="muted">No radio receivers. <a href="/rx/new">Add one.</a></p>{{end}}
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
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Notes</label>
      <textarea name="notes">{{.Notes}}</textarea>
    </div>
    <div class="form-group" style="max-width:100px">
      <label>Quantity owned</label>
      <input type="number" name="quantity" value="{{if .Quantity}}{{.Quantity}}{{else}}1{{end}}" min="0">
    </div>
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
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Notes</label>
      <textarea name="notes">{{.Notes}}</textarea>
    </div>
    <div class="form-group" style="max-width:100px">
      <label>Quantity owned</label>
      <input type="number" name="quantity" value="{{if .Quantity}}{{.Quantity}}{{else}}1{{end}}" min="0">
    </div>
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
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Notes</label>
      <textarea name="notes">{{.Notes}}</textarea>
    </div>
    <div class="form-group" style="max-width:100px">
      <label>Quantity owned</label>
      <input type="number" name="quantity" value="{{if .Quantity}}{{.Quantity}}{{else}}1{{end}}" min="0">
    </div>
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
    <div class="form-group" style="max-width:100px">
      <label>Quantity owned</label>
      <input type="number" name="quantity" value="{{if .Quantity}}{{.Quantity}}{{else}}1{{end}}" min="0">
    </div>
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
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Notes</label>
      <textarea name="notes">{{.Notes}}</textarea>
    </div>
    <div class="form-group" style="max-width:100px">
      <label>Quantity owned</label>
      <input type="number" name="quantity" value="{{if .Quantity}}{{.Quantity}}{{else}}1{{end}}" min="0">
    </div>
  </div>
  <div class="form-actions">
    <button class="btn btn-primary" type="submit">{{if .ID}}Save{{else}}Add VTX{{end}}</button>
    <a href="/inventory" class="btn btn-cancel">Cancel</a>
  </div>
</form>
</div>
{{end}}`

const gpsFormTmpl = `{{define "content"}}
<div class="page-header">
  <h2>{{if .ID}}Edit GPS Module{{else}}New GPS Module{{end}}</h2>
</div>
<div class="form-page">
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
<form method="POST">
  <div class="form-row">
    <div class="form-group">
      <label>Brand</label>
      <input type="text" name="brand" value="{{.Brand}}" placeholder="e.g. Beitian">
    </div>
    <div class="form-group">
      <label>Name *</label>
      <input type="text" name="name" value="{{.Name}}" required autofocus placeholder="e.g. BN-880">
    </div>
    <div class="form-group" style="max-width:100px">
      <label>Quantity owned</label>
      <input type="number" name="quantity" value="{{if .Quantity}}{{.Quantity}}{{else}}1{{end}}" min="0">
    </div>
  </div>
  <div class="form-group">
    <label>Notes</label>
    <textarea name="notes">{{.Notes}}</textarea>
  </div>
  <div class="form-actions">
    <button class="btn btn-primary" type="submit">{{if .ID}}Save{{else}}Add GPS{{end}}</button>
    <a href="/inventory" class="btn btn-cancel">Cancel</a>
  </div>
</form>
</div>
{{end}}`

const rxFormTmpl = `{{define "content"}}
<div class="page-header">
  <h2>{{if .ID}}Edit Radio Receiver{{else}}New Radio Receiver{{end}}</h2>
</div>
<div class="form-page">
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
<form method="POST">
  <div class="form-row">
    <div class="form-group">
      <label>Brand</label>
      <input type="text" name="brand" value="{{.Brand}}" placeholder="e.g. ExpressLRS">
    </div>
    <div class="form-group">
      <label>Name *</label>
      <input type="text" name="name" value="{{.Name}}" required autofocus placeholder="e.g. EP1 Nano">
    </div>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Protocol</label>
      <input type="text" name="protocol" value="{{.Protocol}}" placeholder="e.g. ELRS 2.4GHz">
    </div>
    <div class="form-group" style="max-width:100px">
      <label>Quantity owned</label>
      <input type="number" name="quantity" value="{{if .Quantity}}{{.Quantity}}{{else}}1{{end}}" min="0">
    </div>
  </div>
  <div class="form-group">
    <label>Notes</label>
    <textarea name="notes">{{.Notes}}</textarea>
  </div>
  <div class="form-actions">
    <button class="btn btn-primary" type="submit">{{if .ID}}Save{{else}}Add RX{{end}}</button>
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
  <th>Brand</th><th>Name</th><th>Cell</th><th>mAh</th><th>Owned</th><th>Installed</th><th>Avail.</th><th>Status</th><th>Installed On</th><th></th>
</tr></thead>
<tbody>
{{range .Batteries}}
<tr class="{{if eq .Status "dead"}}retired{{end}}">
  <td class="muted">{{dash .Brand}}</td>
  <td><strong>{{.Name}}</strong></td>
  <td class="muted">{{.CellCount}}S</td>
  <td class="muted">{{.CapacityMAh}}</td>
  <td class="muted">{{.Total}}</td>
  <td class="muted">{{.Installed}}</td>
  <td>{{if gt .Available 0}}<span style="color:#3fb950;font-weight:500">{{.Available}}</span>{{else}}<span class="muted">0</span>{{end}}</td>
  <td><span class="badge {{badgeClass .Status}}">{{.Status}}</span></td>
  <td>{{if .InstalledOn}}<span class="installed-badge">{{.InstalledOn}}</span>{{else}}<span class="muted">—</span>{{end}}</td>
  <td class="actions-cell">
    <a href="/batteries/{{.ID}}/edit" class="btn btn-sm btn-edit">Edit</a>
    <form class="inline" method="POST" action="/batteries/{{.ID}}/adjust">
      <input type="number" name="count" placeholder="±" style="width:46px;padding:2px 4px;vertical-align:middle">
      <button class="btn btn-sm btn-edit" type="submit">Apply</button>
    </form>
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
      <label>Brand</label>
      <input type="text" name="brand" value="{{.Brand}}" placeholder="e.g. CNHL">
    </div>
    <div class="form-group">
      <label>Name *</label>
      <input type="text" name="name" value="{{.Name}}" required autofocus placeholder="e.g. 650mAh 4S">
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
    <div class="form-group" style="max-width:100px">
      <label>Quantity owned</label>
      <input type="number" name="quantity" value="{{if .Quantity}}{{.Quantity}}{{else}}1{{end}}" min="0">
    </div>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Status</label>
      <select name="status">
        <option value="good"     {{if eq .Status "good"}}selected{{end}}>good</option>
        <option value="degraded" {{if eq .Status "degraded"}}selected{{end}}>degraded</option>
        <option value="storage"  {{if eq .Status "storage"}}selected{{end}}>storage</option>
        <option value="dead"     {{if eq .Status "dead"}}selected{{end}}>dead</option>
      </select>
    </div>
    <div class="form-group">
      <label>Notes</label>
      <input type="text" name="notes" value="{{.Notes}}" placeholder="optional">
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
  <td><strong>{{.DroneNames}}</strong></td>
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
  <div class="form-group">
    <label>Drones * <span class="muted" style="font-weight:normal;font-size:12px">(hold Ctrl/Cmd for multiple)</span></label>
    <select name="drone_ids" multiple size="8" style="height:auto">
      {{range .Drones}}
      <option value="{{.ID}}" {{if .Checked}}selected{{end}}>{{.Label}}</option>
      {{end}}
    </select>
  </div>
  <div class="form-row">
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
    <div class="summary">{{.DroneNames}} &mdash; {{.Date}}</div>
  </div>
  <div>
    <a href="/log/{{.ID}}/edit" class="btn btn-edit">Edit</a>
    <form class="inline" method="POST" action="/log/{{.ID}}/delete">
      <button class="btn btn-danger" type="submit">Delete</button>
    </form>
  </div>
</div>
<table style="max-width:500px;margin-bottom:24px">
<tr><td class="muted" style="padding:6px 12px 6px 0;width:120px">Drones</td><td>{{.DroneNames}}</td></tr>
<tr><td class="muted" style="padding:6px 12px 6px 0">Type</td><td><span class="badge {{badgeClass .Type}}">{{.Type}}</span></td></tr>
<tr><td class="muted" style="padding:6px 12px 6px 0">Date</td><td>{{.Date}}</td></tr>
<tr><td class="muted" style="padding:6px 12px 6px 0">Duration</td><td>{{if gt .Duration 0}}{{.Duration}} min{{else}}—{{end}}</td></tr>
<tr><td class="muted" style="padding:6px 12px 6px 0">Location</td><td>{{dash .Location}}</td></tr>
<tr><td class="muted" style="padding:6px 12px 6px 0;vertical-align:top">Notes</td><td style="white-space:pre-wrap">{{dash .Notes}}</td></tr>
</table>

{{if .Batteries}}
<h3>Batteries Used</h3>
<div class="table-wrap" style="max-width:600px;margin-bottom:24px">
<table>
<thead><tr><th>Brand</th><th>Name</th><th>Cell</th><th>mAh</th><th>Status</th></tr></thead>
<tbody>
{{range .Batteries}}
<tr>
  <td class="muted">{{dash .Brand}}</td>
  <td>{{.Name}}</td>
  <td class="muted">{{.CellCount}}S</td>
  <td class="muted">{{.CapacityMAh}}</td>
  <td><span class="badge {{badgeClass .Status}}">{{.Status}}</span></td>
</tr>
{{end}}
</tbody>
</table>
</div>
{{end}}

<h3>Videos</h3>
{{if .Videos}}
<div style="display:flex;flex-direction:column;gap:20px;max-width:860px;margin-bottom:16px">
{{range .Videos}}
<div style="background:#161b22;border:1px solid #30363d;border-radius:6px;padding:12px">
  <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
    <span class="muted" style="font-size:13px">{{.OriginalName}}</span>
    <form class="inline" method="POST" action="/videos/{{.ID}}/delete">
      <button class="btn btn-sm btn-danger" type="submit">Delete</button>
    </form>
  </div>
  <video controls style="width:100%;border-radius:4px;max-height:480px;background:#000">
    <source src="/videos/{{.ID}}">
  </video>
  <form method="POST" action="/videos/{{.ID}}/note" style="margin-top:10px;display:flex;gap:8px;align-items:flex-start">
    <textarea name="notes" rows="2" style="flex:1;resize:vertical" placeholder="Add a note…">{{.Notes}}</textarea>
    <button class="btn btn-sm btn-edit" type="submit">Save</button>
  </form>
</div>
{{end}}
</div>
{{else}}
<p class="muted" style="margin-bottom:8px">No videos yet.</p>
{{end}}
<form method="POST" action="/log/{{.ID}}/videos" enctype="multipart/form-data" style="display:flex;gap:8px;align-items:center;margin-bottom:32px">
  <input type="file" name="video" accept="video/*" style="color:#c9d1d9;font-size:13px">
  <button class="btn btn-primary" type="submit">Upload Video</button>
</form>

<h3>Photos</h3>
{{if .Photos}}
<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(260px,1fr));gap:16px;max-width:860px;margin-bottom:16px">
{{range .Photos}}
<div style="background:#161b22;border:1px solid #30363d;border-radius:6px;padding:10px">
  <img src="/photos/{{.ID}}" style="width:100%;border-radius:4px;display:block;max-height:300px;object-fit:cover">
  <form method="POST" action="/photos/{{.ID}}/note" style="margin-top:8px;display:flex;gap:6px;align-items:flex-start">
    <textarea name="notes" rows="2" style="flex:1;resize:vertical;font-size:13px" placeholder="Add a note…">{{.Notes}}</textarea>
    <button class="btn btn-sm btn-edit" type="submit">Save</button>
  </form>
  <form method="POST" action="/photos/{{.ID}}/delete" style="margin-top:6px">
    <button class="btn btn-sm btn-danger" type="submit">Delete</button>
  </form>
</div>
{{end}}
</div>
{{else}}
<p class="muted" style="margin-bottom:8px">No photos yet.</p>
{{end}}
<form method="POST" action="/log/{{.ID}}/photos" enctype="multipart/form-data" style="display:flex;gap:8px;align-items:center;margin-bottom:32px">
  <input type="file" name="photo" accept="image/*" style="color:#c9d1d9;font-size:13px">
  <button class="btn btn-primary" type="submit">Upload Photo</button>
</form>

<p><a href="/log">&larr; Back to log</a></p>
{{end}}`
