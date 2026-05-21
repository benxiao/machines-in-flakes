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
	SizeInch     string
	CellLabel    string
	Status       string
	WeightG      *int
	FirstPhotoID int
}

type DronePhotoRow struct {
	ID           int
	OriginalName string
	Notes        string
}

type DroneLogEntry struct {
	ID           int
	LoggedAt     string
	LoggedAtInput string
	Body         string
}

type DroneDetailPage struct {
	ActiveTab    string
	ID           int
	Name         string
	Status       string
	SizeInch     string
	CellLabel    string
	WeightG      *int
	BuildDate    string
	Notes        string
	FrameName    string
	FCName       string
	ESCName      string
	VTXName      string
	MotorName    string
	MotorCount   int
	BatteryNames string
	GPSName      string
	RXName       string
	PropNames    string
	Photos       []DronePhotoRow
	Entries      []DroneLogEntry
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
	GPSID        int
	RXID         int
	Status       string
	BuildDate    string
	SizeID       int
	CellID       int
	WeightG      string
	Notes        string
	Sizes        []OptionItem
	Cells        []OptionItem
	Frames       []OptionItem
	FCs          []OptionItem
	ESCs         []OptionItem
	VTXs         []OptionItem
	Motors       []OptionItem
	GPSs         []OptionItem
	RXs          []OptionItem
	Batteries    []BatteryCheck
	Props        []PropCheck
	Photos       []DronePhotoRow
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
	ID           int
	Brand        string
	Name         string
	Total        int
	Installed    int
	Available    int
	InstalledOn  string
	FirstPhotoID int
}

type RXRow struct {
	ID           int
	Brand        string
	Name         string
	Protocol     string
	Total        int
	Installed    int
	Available    int
	InstalledOn  string
	FirstPhotoID int
}

type GPSFormPage struct {
	ActiveTab string
	Error     string
	ID        int
	BrandID   int
	Name      string
	Notes     string
	Quantity  string
	Photos    []DronePhotoRow
	Brands    []OptionItem
}

type RXFormPage struct {
	ActiveTab string
	Error     string
	ID        int
	BrandID   int
	Name      string
	Protocol  string
	Notes     string
	Quantity  string
	Photos    []DronePhotoRow
	Brands    []OptionItem
}

type FrameRow struct {
	ID           int
	Brand        string
	Name         string
	SizeInch     string
	WeightG      string
	Total        int
	Installed    int
	Available    int
	InstalledOn  string
	FirstPhotoID int
}

type FCRow struct {
	ID           int
	Brand        string
	Name         string
	MCU          string
	Firmware     string
	Total        int
	Installed    int
	Available    int
	InstalledOn  string
	FirstPhotoID int
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
	FirstPhotoID  int
}

type MotorRow struct {
	ID           int
	Brand        string
	Name         string
	StatorSize   string
	KV           string
	Total        int
	Installed    int
	Available    int
	InstalledOn  string
	FirstPhotoID int
}

type VTXRow struct {
	ID           int
	Brand        string
	Name         string
	System       string
	MaxPowerMW   string
	Resolution   string
	Total        int
	Installed    int
	Available    int
	InstalledOn  string
	FirstPhotoID int
}

type FrameFormPage struct {
	ActiveTab string
	Error     string
	ID        int
	BrandID   int
	SizeID    int
	Name      string
	WeightG   string
	Notes     string
	Quantity  string
	Photos    []DronePhotoRow
	Brands    []OptionItem
	Sizes     []OptionItem
}

type FCFormPage struct {
	ActiveTab string
	Error     string
	ID        int
	BrandID   int
	Name      string
	MCU       string
	Firmware  string
	Notes     string
	Quantity  string
	Photos    []DronePhotoRow
	Brands    []OptionItem
}

type ESCFormPage struct {
	ActiveTab     string
	Error         string
	ID            int
	BrandID       int
	Name          string
	CurrentRating string
	CellMax       string
	Notes         string
	Quantity      string
	Photos        []DronePhotoRow
	Brands        []OptionItem
}

type MotorFormPage struct {
	ActiveTab  string
	Error      string
	ID         int
	BrandID    int
	Name       string
	StatorSize string
	KV         string
	Notes      string
	Quantity   string
	Photos     []DronePhotoRow
	Brands     []OptionItem
}

type VTXFormPage struct {
	ActiveTab  string
	Error      string
	ID         int
	BrandID    int
	Name       string
	System     string
	MaxPowerMW string
	Resolution string
	WeightG    string
	Notes      string
	Quantity   string
	Photos     []DronePhotoRow
	Brands     []OptionItem
}

type BatteryListPage struct {
	ActiveTab string
	Batteries []BatteryRow
}

type BatteryRow struct {
	ID           int
	Brand        string
	Name         string
	CellLabel    string
	CapacityMAh  int
	WeightG      *int
	Total        int
	AssignedTo   string
	Count        int
	FirstPhotoID int
}

type BatteryFormPage struct {
	ActiveTab   string
	Error       string
	ID          int
	BrandID     int
	CellID      int
	Name        string
	CapacityMAh string
	WeightG     string
	Quantity    string
	Notes       string
	Photos      []DronePhotoRow
	Brands      []OptionItem
	Cells       []OptionItem
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
	DroneNames       string
	LowStock         bool
	FirstPhotoID     int
}

type PropCheck struct {
	ID      int
	Label   string
	Checked bool
}

type PropFormPage struct {
	ActiveTab        string
	Error            string
	ID               int
	BrandID          int
	SizeID           int
	Name             string
	Pitch            string
	BladeCount       string
	Material         string
	Quantity         string
	ReorderThreshold string
	Notes            string
	Photos           []DronePhotoRow
	Brands           []OptionItem
	Sizes            []OptionItem
}

type LogListPage struct {
	ActiveTab string
	Sessions  []SessionRow
}

type SessionRow struct {
	ID          int
	Title       string
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
	Count   int
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
	Title       string
	Type        string
	SessionDate string
	DurationMin string
	Location    string
	Notes       string
	Drones      []DroneCheck
	Batteries   []BatteryCheck
	Places      []OptionItem
}

type SessionDetailPage struct {
	ActiveTab  string
	ID         int
	Title      string
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

type PlaceListPage struct {
	ActiveTab string
	Places    []PlaceRow
}

type PlaceRow struct {
	ID        int
	Name      string
	Address   string
	PlaceType string
	Notes     string
	Lat       *float64
	Lng       *float64
	HasCoords bool
	LatStr    string
	LngStr    string
}

type PlaceDetailPage struct {
	ActiveTab string
	PlaceRow
}

type PlaceFormPage struct {
	ActiveTab string
	Error     string
	ID        int
	Name      string
	Address   string
	PlaceType string
	Notes     string
	Lat       *float64
	Lng       *float64
}

type CellRow struct {
	ID    int
	Label string
}

type SettingsPage struct {
	ActiveTab string
	Brands    []BrandRow
	Sizes     []SizeRow
	Cells     []CellRow
}

type CellFormPage struct {
	ActiveTab string
	Error     string
	ID        int
	Label     string
}

type SizeRow struct {
	ID    int
	Label string
}

type SizeFormPage struct {
	ActiveTab string
	Error     string
	ID        int
	Label     string
}

type BrandRow struct {
	ID   int
	Name string
}

type BrandFormPage struct {
	ActiveTab string
	Error     string
	ID        int
	Name      string
}

type WeatherDay struct {
	Date         string
	RainChance   int
	RainMaxMm    string
	WindSpeedKmh int
	GustSpeedKmh int
	WindDir      string
	TempMax      int
	TempMin      int
	ShortText    string
	ExtendedText string
	IconDesc     string
	HasWind      bool
	FlyRating    string // "good", "caution", "bad"
	DayHours     []DayHour
	WindMaxLabel string
	WindGuides   []ChartGuide
	RainGuides   []ChartGuide
}

type DayHour struct {
	Label    string // "8" … "17"
	WindBarH int    // 0–100 (% of chart height)
	RainBarH int    // 0–100 (% of chart height)
	WindFill string // hex color based on speed
}

type ChartGuide struct {
	TopPct int    // 0 = chart top, 100 = chart bottom
	Label  string // value shown on Y-axis
	Color  string // CSS color for the dashed guideline
}

type WeatherPage struct {
	ActiveTab string
	Days      []WeatherDay
	FetchedAt string
	Error     string
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
		case "repairing":
			return "badge-repairing"
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
	add("drone-detail", droneDetailTmpl)
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
	add("place-list", placeListTmpl)
	add("place-form", placeFormTmpl)
	add("place-detail", placeDetailTmpl)
	add("settings", settingsTmpl)
	add("brand-form", brandFormTmpl)
	add("size-form", sizeFormTmpl)
	add("cell-form", cellFormTmpl)
	add("weather", weatherTmpl)
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
.badge-repairing  { background: rgba(248,81,73,0.15);  color: #f85149; border: 1px solid rgba(248,81,73,0.4); }
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
.table-wrap { border: 1px solid #30363d; border-radius: 6px; overflow-x: auto; }
.form-page { max-width: 580px; }
.form-group { margin-bottom: 16px; }
label { display: block; font-size: 13px; color: #8b949e; margin-bottom: 4px; }
input[type=text], input[type=number], input[type=date], input[type=datetime-local], select, textarea {
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
.upload-form { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
@media (max-width: 640px) {
  main { padding: 12px; }
  header { padding: 10px 16px; }
  nav { overflow-x: auto; -webkit-overflow-scrolling: touch; padding: 0 12px; }
  nav a { white-space: nowrap; padding: 10px 10px; font-size: 13px; }
  .page-header { flex-direction: column; align-items: flex-start; gap: 10px; }
  .section-header { flex-wrap: wrap; gap: 8px; }
  .form-row { flex-direction: column; gap: 0; }
  .form-page { max-width: 100%; }
  .form-actions { flex-wrap: wrap; }
  .form-row .form-group[style*="max-width"] { max-width: 100% !important; }
  .btn { min-height: 44px; padding: 10px 16px; }
  .btn-sm { min-height: 36px; padding: 6px 12px; font-size: 13px; }
  .dle-entry-row { flex-direction: column !important; }
  .dle-entry-btns { align-self: flex-end; }
  .media-card-header { flex-wrap: wrap; gap: 8px; }
  .upload-form input[type=file] { width: 100%; }
}
`

// ---- Base template ----

const baseTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>FPV Manager v0.3</title>
<link rel="icon" type="image/svg+xml" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Cline x1='16' y1='16' x2='5' y2='5' stroke='%2358a6ff' stroke-width='2.5' stroke-linecap='round'/%3E%3Cline x1='16' y1='16' x2='27' y2='5' stroke='%2358a6ff' stroke-width='2.5' stroke-linecap='round'/%3E%3Cline x1='16' y1='16' x2='5' y2='27' stroke='%2358a6ff' stroke-width='2.5' stroke-linecap='round'/%3E%3Cline x1='16' y1='16' x2='27' y2='27' stroke='%2358a6ff' stroke-width='2.5' stroke-linecap='round'/%3E%3Ccircle cx='5' cy='5' r='4' fill='%2358a6ff'/%3E%3Ccircle cx='27' cy='5' r='4' fill='%2358a6ff'/%3E%3Ccircle cx='5' cy='27' r='4' fill='%2358a6ff'/%3E%3Ccircle cx='27' cy='27' r='4' fill='%2358a6ff'/%3E%3Ccircle cx='16' cy='16' r='3.5' fill='%2358a6ff'/%3E%3C/svg%3E">
<style>` + css + `</style>
</head>
<body>
<header><span class="logo"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="20" height="20" style="vertical-align:-4px;margin-right:6px"><line x1="16" y1="16" x2="5" y2="5" stroke="#58a6ff" stroke-width="2.5" stroke-linecap="round"/><line x1="16" y1="16" x2="27" y2="5" stroke="#58a6ff" stroke-width="2.5" stroke-linecap="round"/><line x1="16" y1="16" x2="5" y2="27" stroke="#58a6ff" stroke-width="2.5" stroke-linecap="round"/><line x1="16" y1="16" x2="27" y2="27" stroke="#58a6ff" stroke-width="2.5" stroke-linecap="round"/><circle cx="5" cy="5" r="4" fill="#58a6ff"/><circle cx="27" cy="5" r="4" fill="#58a6ff"/><circle cx="5" cy="27" r="4" fill="#58a6ff"/><circle cx="27" cy="27" r="4" fill="#58a6ff"/><circle cx="16" cy="16" r="3.5" fill="#58a6ff"/></svg>FPV Manager <span style="font-size:0.7em;opacity:0.5;font-weight:400">v0.3</span></span></header>
<nav>
  <a href="/drones"    {{if eq .ActiveTab "drones"}}class="active"{{end}}>Drones</a>
  <a href="/inventory" {{if eq .ActiveTab "inventory"}}class="active"{{end}}>Inventory</a>
  <a href="/props"     {{if eq .ActiveTab "props"}}class="active"{{end}}>Props</a>
  <a href="/batteries" {{if eq .ActiveTab "batteries"}}class="active"{{end}}>Batteries</a>
  <a href="/log"       {{if eq .ActiveTab "log"}}class="active"{{end}}>Log</a>
  <a href="/places"    {{if eq .ActiveTab "places"}}class="active"{{end}}>Places</a>
  <a href="/settings"  {{if eq .ActiveTab "settings"}}class="active"{{end}}>Settings</a>
  <a href="/weather"   {{if eq .ActiveTab "weather"}}class="active"{{end}}>Weather</a>
</nav>
<main>
{{template "content" .}}
</main>
<script>
document.addEventListener('submit', function(e) {
  var action = e.target.getAttribute('action') || '';
  if (action.indexOf('/delete') !== -1) {
    if (!confirm('Are you sure you want to delete this? This cannot be undone.')) {
      e.preventDefault();
    }
  }
});
</script>
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
  <th></th><th>Name</th><th>Size</th><th>Cell</th><th>Status</th><th>Weight</th><th></th>
</tr></thead>
<tbody>
{{range .Drones}}
<tr class="{{if eq .Status "retired"}}retired{{end}}">
  <td style="width:56px;padding:6px 8px">
    {{if .FirstPhotoID}}
    <img src="/drone-photos/{{.FirstPhotoID}}" style="width:48px;height:48px;object-fit:cover;border-radius:4px;display:block;cursor:zoom-in" onclick="openLightbox('/drone-photos/{{.FirstPhotoID}}')">
    {{else}}
    <div style="width:48px;height:48px;background:#21262d;border-radius:4px;display:flex;align-items:center;justify-content:center;color:#8b949e;font-size:20px">✈</div>
    {{end}}
  </td>
  <td><a href="/drones/{{.ID}}" style="font-weight:600;color:#c9d1d9;text-decoration:none">{{.Name}}</a></td>
  <td class="muted">{{if .SizeInch}}{{.SizeInch}}"{{else}}—{{end}}</td>
  <td class="muted">{{dash .CellLabel}}</td>
  <td><span class="badge {{badgeClass .Status}}">{{.Status}}</span></td>
  <td class="muted">{{if .WeightG}}{{.WeightG}}g{{else}}—{{end}}</td>
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

<div id="lightbox" style="display:none;position:fixed;inset:0;background:rgba(0,0,0,.85);z-index:9999;align-items:center;justify-content:center" onclick="closeLightbox()">
  <img id="lightbox-img" src="" style="max-width:90vw;max-height:90vh;border-radius:8px;object-fit:contain;box-shadow:0 8px 40px rgba(0,0,0,.6)">
</div>
<script>
function openLightbox(src){var lb=document.getElementById('lightbox');document.getElementById('lightbox-img').src=src;lb.style.display='flex';}
function closeLightbox(){document.getElementById('lightbox').style.display='none';}
document.addEventListener('keydown',function(e){if(e.key==='Escape')closeLightbox();});
</script>
{{end}}`

const droneDetailTmpl = `{{define "content"}}
<div class="page-header">
  <div class="page-header-left">
    <h2>{{.Name}}</h2>
    <span class="badge {{badgeClass .Status}}" style="margin-left:10px">{{.Status}}</span>
  </div>
  <div style="display:flex;gap:8px;align-items:center">
    <a href="/drones/{{.ID}}/edit" class="btn btn-edit">Edit</a>
    <form class="inline" method="POST" action="/drones/{{.ID}}/delete">
      <button class="btn btn-danger" type="submit">Delete</button>
    </form>
    <a href="/drones" class="btn btn-cancel">← Back</a>
  </div>
</div>

<div class="section">
  <div style="display:grid;grid-template-columns:1fr 1fr;gap:24px;max-width:900px">
    <div>
      {{if .Photos}}
      <img src="/drone-photos/{{(index .Photos 0).ID}}" style="width:100%;border-radius:6px;object-fit:cover;max-height:260px;display:block;cursor:zoom-in;margin-bottom:16px" onclick="openLightbox('/drone-photos/{{(index .Photos 0).ID}}')">
      {{end}}
      <table style="border-collapse:collapse;width:100%">
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">Size</td><td>{{if .SizeInch}}{{.SizeInch}}"{{else}}<span class="muted">—</span>{{end}}</td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">Cell Count</td><td>{{if .CellLabel}}{{.CellLabel}}{{else}}<span class="muted">—</span>{{end}}</td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">Weight</td><td>{{if .WeightG}}{{.WeightG}}g{{else}}<span class="muted">—</span>{{end}}</td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">Build Date</td><td>{{if .BuildDate}}{{.BuildDate}}{{else}}<span class="muted">—</span>{{end}}</td></tr>
        {{if .Notes}}<tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap;vertical-align:top">Notes</td><td style="white-space:pre-wrap">{{.Notes}}</td></tr>{{end}}
      </table>
      {{if gt (len .Photos) 1}}
      <div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(80px,1fr));gap:8px;margin-top:12px">
        {{range $i,$p := .Photos}}{{if gt $i 0}}
        <img src="/drone-photos/{{$p.ID}}" style="width:100%;height:80px;object-fit:cover;border-radius:4px;cursor:zoom-in" onclick="openLightbox('/drone-photos/{{$p.ID}}')">
        {{end}}{{end}}
      </div>
      {{end}}
    </div>
    <div>
      <table style="border-collapse:collapse;width:100%">
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">Frame</td><td>{{dash .FrameName}}</td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">FC</td><td>{{dash .FCName}}</td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">ESC</td><td>{{dash .ESCName}}</td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">VTX</td><td>{{dash .VTXName}}</td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">Motors</td><td>{{if .MotorName}}{{.MotorName}} ×{{.MotorCount}}{{else}}<span class="muted">—</span>{{end}}</td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">Batteries</td><td>{{dash .BatteryNames}}</td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">GPS</td><td>{{dash .GPSName}}</td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">RX</td><td>{{dash .RXName}}</td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">Props</td><td>{{dash .PropNames}}</td></tr>
      </table>
    </div>
  </div>
</div>

<div id="lightbox" style="display:none;position:fixed;inset:0;background:rgba(0,0,0,.85);z-index:9999;align-items:center;justify-content:center" onclick="closeLightbox()">
  <img id="lightbox-img" src="" style="max-width:90vw;max-height:90vh;border-radius:8px;object-fit:contain;box-shadow:0 8px 40px rgba(0,0,0,.6)">
</div>
<script>
function openLightbox(src){var lb=document.getElementById('lightbox');document.getElementById('lightbox-img').src=src;lb.style.display='flex';}
function closeLightbox(){document.getElementById('lightbox').style.display='none';}
document.addEventListener('keydown',function(e){if(e.key==='Escape')closeLightbox();});
</script>

<div class="section">
  <h3 style="margin-bottom:12px">Add Entry</h3>
  <form method="POST" action="/drones/{{.ID}}/log" style="display:flex;flex-direction:column;gap:10px;max-width:600px">
    <div style="display:flex;gap:12px;align-items:flex-start;flex-wrap:wrap">
      <div class="form-group" style="flex:0 0 auto">
        <label>Date &amp; Time</label>
        <input type="datetime-local" name="logged_at" id="log_at" style="width:200px">
      </div>
      <div class="form-group" style="flex:1 1 300px">
        <label>Note</label>
        <textarea name="body" rows="3" placeholder="e.g. replaced motor, adjusted PID, tuned rates…" style="resize:vertical"></textarea>
      </div>
    </div>
    <div><button class="btn btn-primary" type="submit">Add</button></div>
  </form>
  <script>
    (function(){
      var el=document.getElementById('log_at');
      if(el&&!el.value){
        var n=new Date();
        n.setMinutes(n.getMinutes()-n.getTimezoneOffset());
        el.value=n.toISOString().slice(0,16);
      }
    })();
  </script>
</div>

<div class="section">
{{if .Entries}}
<div style="display:flex;flex-direction:column;gap:10px;max-width:700px">
{{range .Entries}}
<div style="background:#161b22;border:1px solid #30363d;border-radius:6px;padding:14px 16px">
  <div class="dle-entry-row" style="display:flex;gap:12px;align-items:flex-start">
    <!-- view mode -->
    <div class="dle-view" style="display:flex;gap:12px;align-items:flex-start;flex:1;min-width:0">
      <div style="flex:0 0 auto;color:#8b949e;font-size:13px;padding-top:2px;white-space:nowrap">{{.LoggedAt}}</div>
      <div style="flex:1;white-space:pre-wrap;word-break:break-word;min-width:0">{{.Body}}</div>
    </div>
    <!-- edit mode (hidden) -->
    <form class="dle-form" method="POST" action="/drone-log/{{.ID}}/edit" style="display:none;flex:1;gap:10px;flex-direction:column;min-width:0">
      <div style="display:flex;gap:10px;align-items:flex-start;flex-wrap:wrap">
        <input type="datetime-local" name="logged_at" value="{{.LoggedAtInput}}" style="flex:0 0 auto;font-size:13px">
        <textarea name="body" rows="3" style="flex:1;min-width:200px;resize:vertical;font-size:14px">{{.Body}}</textarea>
      </div>
    </form>
    <!-- buttons -->
    <div class="dle-entry-btns" style="display:flex;gap:6px;flex-shrink:0">
      <button class="btn btn-sm btn-edit dle-edit-btn" onclick="dleEdit(this)">Edit</button>
      <form method="POST" action="/drone-log/{{.ID}}/delete" style="display:inline">
        <button class="btn btn-sm btn-danger" type="submit">Delete</button>
      </form>
    </div>
  </div>
</div>
{{end}}
<script>
function dleEdit(btn) {
  var card = btn.closest('div[style*="background:#161b22"]');
  var view = card.querySelector('.dle-view');
  var form = card.querySelector('.dle-form');
  if (btn.textContent === 'Edit') {
    view.style.display = 'none';
    form.style.display = 'flex';
    btn.textContent = 'Save';
    btn.onclick = function(){ form.submit(); };
  }
}
</script>
</div>
{{else}}
<p class="muted">No log entries yet.</p>
{{end}}
</div>
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
        <option value="repairing" {{if eq .Status "repairing"}}selected{{end}}>repairing</option>
      </select>
    </div>
    <div class="form-group">
      <label>Build Date</label>
      <input type="date" name="build_date" value="{{.BuildDate}}">
    </div>
    <div class="form-group">
      <label>Size (in)</label>
      <select name="size_id">
        <option value="">—</option>
        {{range .Sizes}}<option value="{{.ID}}" {{if eq $.SizeID .ID}}selected{{end}}>{{.Label}}"</option>{{end}}
      </select>
    </div>
    <div class="form-group">
      <label>Cell Count</label>
      <select name="cell_id">
        <option value="">—</option>
        {{range .Cells}}<option value="{{.ID}}" {{if eq $.CellID .ID}}selected{{end}}>{{.Label}}</option>{{end}}
      </select>
    </div>
    <div class="form-group" style="max-width:120px">
      <label>Weight (g)</label>
      <input type="number" name="weight_g" value="{{.WeightG}}" placeholder="e.g. 250" min="0">
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
  {{if .Batteries}}
  <div class="form-group">
    <label>Batteries</label>
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
  {{if .Props}}
  <div class="form-group">
    <label>Props</label>
    <div class="battery-checks">
      {{range .Props}}
      <label class="battery-check">
        <input type="checkbox" name="prop_ids" value="{{.ID}}" {{if .Checked}}checked{{end}}>
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
    <button class="btn btn-primary" type="submit">{{if .ID}}Save Changes{{else}}Create Drone{{end}}</button>
    <a href="/drones" class="btn btn-cancel">Cancel</a>
  </div>
</form>

{{if .ID}}
<h3 style="margin-top:32px">Photos</h3>
{{if .Photos}}
<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:16px;max-width:700px;margin-bottom:16px">
{{range .Photos}}
<div style="background:#161b22;border:1px solid #30363d;border-radius:6px;padding:10px">
  <img src="/drone-photos/{{.ID}}" style="width:100%;border-radius:4px;display:block;max-height:200px;object-fit:cover">
  <form method="POST" action="/drone-photos/{{.ID}}/note" style="margin-top:8px;display:flex;gap:6px;align-items:flex-start">
    <textarea name="notes" rows="2" style="flex:1;resize:vertical;font-size:13px" placeholder="Add a note…">{{.Notes}}</textarea>
    <button class="btn btn-sm btn-edit" type="submit">Save</button>
  </form>
  <form method="POST" action="/drone-photos/{{.ID}}/delete" style="margin-top:6px">
    <button class="btn btn-sm btn-danger" type="submit">Delete</button>
  </form>
</div>
{{end}}
</div>
{{else}}
<p class="muted" style="margin-bottom:8px">No photos yet.</p>
{{end}}
<form method="POST" action="/drones/{{.ID}}/photos" enctype="multipart/form-data" class="upload-form">
  <input type="file" name="photo" accept="image/*" style="color:#c9d1d9;font-size:13px">
  <button class="btn btn-primary" type="submit">Upload Photo</button>
</form>
{{end}}
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
  <thead><tr><th></th><th>Brand</th><th>Name</th><th>Size</th><th>Owned</th><th>Installed</th><th>Avail.</th><th>Installed On</th><th></th></tr></thead>
  <tbody>
  {{range .Frames}}
  <tr>
    <td style="width:40px;padding:4px 6px">{{if .FirstPhotoID}}<img src="/frame-photos/{{.FirstPhotoID}}" style="width:36px;height:36px;object-fit:cover;border-radius:3px;display:block;cursor:zoom-in" onclick="openLightbox('/frame-photos/{{.FirstPhotoID}}')">{{end}}</td>
    <td class="muted">{{dash .Brand}}</td>
    <td>{{.Name}}</td>
    <td class="muted">{{if .SizeInch}}{{.SizeInch}}"{{else}}—{{end}}</td>
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
  <thead><tr><th></th><th>Brand</th><th>Name</th><th>MCU</th><th>Firmware</th><th>Owned</th><th>Installed</th><th>Avail.</th><th>Installed On</th><th></th></tr></thead>
  <tbody>
  {{range .FCs}}
  <tr>
    <td style="width:40px;padding:4px 6px">{{if .FirstPhotoID}}<img src="/fc-photos/{{.FirstPhotoID}}" style="width:36px;height:36px;object-fit:cover;border-radius:3px;display:block;cursor:zoom-in" onclick="openLightbox('/fc-photos/{{.FirstPhotoID}}')">{{end}}</td>
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
  <thead><tr><th></th><th>Brand</th><th>Name</th><th>Current</th><th>Max Cell</th><th>Owned</th><th>Installed</th><th>Avail.</th><th>Installed On</th><th></th></tr></thead>
  <tbody>
  {{range .ESCs}}
  <tr>
    <td style="width:40px;padding:4px 6px">{{if .FirstPhotoID}}<img src="/esc-photos/{{.FirstPhotoID}}" style="width:36px;height:36px;object-fit:cover;border-radius:3px;display:block;cursor:zoom-in" onclick="openLightbox('/esc-photos/{{.FirstPhotoID}}')">{{end}}</td>
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
  <thead><tr><th></th><th>Brand</th><th>Name</th><th>Stator</th><th>KV</th><th>Owned</th><th>Installed</th><th>Avail.</th><th>Installed On</th><th></th></tr></thead>
  <tbody>
  {{range .Motors}}
  <tr>
    <td style="width:40px;padding:4px 6px">{{if .FirstPhotoID}}<img src="/motor-photos/{{.FirstPhotoID}}" style="width:36px;height:36px;object-fit:cover;border-radius:3px;display:block;cursor:zoom-in" onclick="openLightbox('/motor-photos/{{.FirstPhotoID}}')">{{end}}</td>
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
  <thead><tr><th></th><th>Brand</th><th>Name</th><th>System</th><th>Max Power</th><th>Resolution</th><th>Owned</th><th>Installed</th><th>Avail.</th><th>Installed On</th><th></th></tr></thead>
  <tbody>
  {{range .VTXs}}
  <tr>
    <td style="width:40px;padding:4px 6px">{{if .FirstPhotoID}}<img src="/vtx-photos/{{.FirstPhotoID}}" style="width:36px;height:36px;object-fit:cover;border-radius:3px;display:block;cursor:zoom-in" onclick="openLightbox('/vtx-photos/{{.FirstPhotoID}}')">{{end}}</td>
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
  <thead><tr><th></th><th>Brand</th><th>Name</th><th>Owned</th><th>Installed</th><th>Avail.</th><th>Installed On</th><th></th></tr></thead>
  <tbody>
  {{range .GPSs}}
  <tr>
    <td style="width:40px;padding:4px 6px">{{if .FirstPhotoID}}<img src="/gps-photos/{{.FirstPhotoID}}" style="width:36px;height:36px;object-fit:cover;border-radius:3px;display:block;cursor:zoom-in" onclick="openLightbox('/gps-photos/{{.FirstPhotoID}}')">{{end}}</td>
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
  <thead><tr><th></th><th>Brand</th><th>Name</th><th>Protocol</th><th>Owned</th><th>Installed</th><th>Avail.</th><th>Installed On</th><th></th></tr></thead>
  <tbody>
  {{range .RXs}}
  <tr>
    <td style="width:40px;padding:4px 6px">{{if .FirstPhotoID}}<img src="/rx-photos/{{.FirstPhotoID}}" style="width:36px;height:36px;object-fit:cover;border-radius:3px;display:block;cursor:zoom-in" onclick="openLightbox('/rx-photos/{{.FirstPhotoID}}')">{{end}}</td>
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
      <select name="brand_id">
        <option value="0">— no brand —</option>
        {{range .Brands}}<option value="{{.ID}}" {{if eq $.BrandID .ID}}selected{{end}}>{{.Label}}</option>{{end}}
      </select>
    </div>
    <div class="form-group">
      <label>Name *</label>
      <input type="text" name="name" value="{{.Name}}" required autofocus placeholder="e.g. Nazgul5 V3">
    </div>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Size (in)</label>
      <select name="size_id">
        <option value="">—</option>
        {{range .Sizes}}<option value="{{.ID}}" {{if eq $.SizeID .ID}}selected{{end}}>{{.Label}}"</option>{{end}}
      </select>
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
{{if .ID}}
<h3 style="margin-top:32px">Photos</h3>
{{if .Photos}}
<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:16px;max-width:700px;margin-bottom:16px">
{{range .Photos}}
<div style="background:#161b22;border:1px solid #30363d;border-radius:6px;padding:10px">
  <img src="/frame-photos/{{.ID}}" style="width:100%;border-radius:4px;display:block;max-height:200px;object-fit:cover">
  <form method="POST" action="/frame-photos/{{.ID}}/note" style="margin-top:8px;display:flex;gap:6px;align-items:flex-start">
    <textarea name="notes" rows="2" style="flex:1;resize:vertical;font-size:13px" placeholder="Add a note…">{{.Notes}}</textarea>
    <button class="btn btn-sm btn-edit" type="submit">Save</button>
  </form>
  <form method="POST" action="/frame-photos/{{.ID}}/delete" style="margin-top:6px">
    <button class="btn btn-sm btn-danger" type="submit">Delete</button>
  </form>
</div>
{{end}}
</div>
{{else}}
<p class="muted" style="margin-bottom:8px">No photos yet.</p>
{{end}}
<form method="POST" action="/frames/{{.ID}}/photos" enctype="multipart/form-data" class="upload-form">
  <input type="file" name="photo" accept="image/*" style="color:#c9d1d9;font-size:13px">
  <button class="btn btn-primary" type="submit">Upload Photo</button>
</form>
{{end}}
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
      <select name="brand_id">
        <option value="0">— no brand —</option>
        {{range .Brands}}<option value="{{.ID}}" {{if eq $.BrandID .ID}}selected{{end}}>{{.Label}}</option>{{end}}
      </select>
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
{{if .ID}}
<h3 style="margin-top:32px">Photos</h3>
{{if .Photos}}
<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:16px;max-width:700px;margin-bottom:16px">
{{range .Photos}}
<div style="background:#161b22;border:1px solid #30363d;border-radius:6px;padding:10px">
  <img src="/fc-photos/{{.ID}}" style="width:100%;border-radius:4px;display:block;max-height:200px;object-fit:cover">
  <form method="POST" action="/fc-photos/{{.ID}}/note" style="margin-top:8px;display:flex;gap:6px;align-items:flex-start">
    <textarea name="notes" rows="2" style="flex:1;resize:vertical;font-size:13px" placeholder="Add a note…">{{.Notes}}</textarea>
    <button class="btn btn-sm btn-edit" type="submit">Save</button>
  </form>
  <form method="POST" action="/fc-photos/{{.ID}}/delete" style="margin-top:6px">
    <button class="btn btn-sm btn-danger" type="submit">Delete</button>
  </form>
</div>
{{end}}
</div>
{{else}}
<p class="muted" style="margin-bottom:8px">No photos yet.</p>
{{end}}
<form method="POST" action="/fcs/{{.ID}}/photos" enctype="multipart/form-data" class="upload-form">
  <input type="file" name="photo" accept="image/*" style="color:#c9d1d9;font-size:13px">
  <button class="btn btn-primary" type="submit">Upload Photo</button>
</form>
{{end}}
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
      <select name="brand_id">
        <option value="0">— no brand —</option>
        {{range .Brands}}<option value="{{.ID}}" {{if eq $.BrandID .ID}}selected{{end}}>{{.Label}}</option>{{end}}
      </select>
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
{{if .ID}}
<h3 style="margin-top:32px">Photos</h3>
{{if .Photos}}
<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:16px;max-width:700px;margin-bottom:16px">
{{range .Photos}}
<div style="background:#161b22;border:1px solid #30363d;border-radius:6px;padding:10px">
  <img src="/esc-photos/{{.ID}}" style="width:100%;border-radius:4px;display:block;max-height:200px;object-fit:cover">
  <form method="POST" action="/esc-photos/{{.ID}}/note" style="margin-top:8px;display:flex;gap:6px;align-items:flex-start">
    <textarea name="notes" rows="2" style="flex:1;resize:vertical;font-size:13px" placeholder="Add a note…">{{.Notes}}</textarea>
    <button class="btn btn-sm btn-edit" type="submit">Save</button>
  </form>
  <form method="POST" action="/esc-photos/{{.ID}}/delete" style="margin-top:6px">
    <button class="btn btn-sm btn-danger" type="submit">Delete</button>
  </form>
</div>
{{end}}
</div>
{{else}}
<p class="muted" style="margin-bottom:8px">No photos yet.</p>
{{end}}
<form method="POST" action="/escs/{{.ID}}/photos" enctype="multipart/form-data" class="upload-form">
  <input type="file" name="photo" accept="image/*" style="color:#c9d1d9;font-size:13px">
  <button class="btn btn-primary" type="submit">Upload Photo</button>
</form>
{{end}}
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
      <select name="brand_id">
        <option value="0">— no brand —</option>
        {{range .Brands}}<option value="{{.ID}}" {{if eq $.BrandID .ID}}selected{{end}}>{{.Label}}</option>{{end}}
      </select>
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
{{if .ID}}
<h3 style="margin-top:32px">Photos</h3>
{{if .Photos}}
<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:16px;max-width:700px;margin-bottom:16px">
{{range .Photos}}
<div style="background:#161b22;border:1px solid #30363d;border-radius:6px;padding:10px">
  <img src="/motor-photos/{{.ID}}" style="width:100%;border-radius:4px;display:block;max-height:200px;object-fit:cover">
  <form method="POST" action="/motor-photos/{{.ID}}/note" style="margin-top:8px;display:flex;gap:6px;align-items:flex-start">
    <textarea name="notes" rows="2" style="flex:1;resize:vertical;font-size:13px" placeholder="Add a note…">{{.Notes}}</textarea>
    <button class="btn btn-sm btn-edit" type="submit">Save</button>
  </form>
  <form method="POST" action="/motor-photos/{{.ID}}/delete" style="margin-top:6px">
    <button class="btn btn-sm btn-danger" type="submit">Delete</button>
  </form>
</div>
{{end}}
</div>
{{else}}
<p class="muted" style="margin-bottom:8px">No photos yet.</p>
{{end}}
<form method="POST" action="/motors/{{.ID}}/photos" enctype="multipart/form-data" class="upload-form">
  <input type="file" name="photo" accept="image/*" style="color:#c9d1d9;font-size:13px">
  <button class="btn btn-primary" type="submit">Upload Photo</button>
</form>
{{end}}
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
      <select name="brand_id">
        <option value="0">— no brand —</option>
        {{range .Brands}}<option value="{{.ID}}" {{if eq $.BrandID .ID}}selected{{end}}>{{.Label}}</option>{{end}}
      </select>
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
{{if .ID}}
<h3 style="margin-top:32px">Photos</h3>
{{if .Photos}}
<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:16px;max-width:700px;margin-bottom:16px">
{{range .Photos}}
<div style="background:#161b22;border:1px solid #30363d;border-radius:6px;padding:10px">
  <img src="/vtx-photos/{{.ID}}" style="width:100%;border-radius:4px;display:block;max-height:200px;object-fit:cover">
  <form method="POST" action="/vtx-photos/{{.ID}}/note" style="margin-top:8px;display:flex;gap:6px;align-items:flex-start">
    <textarea name="notes" rows="2" style="flex:1;resize:vertical;font-size:13px" placeholder="Add a note…">{{.Notes}}</textarea>
    <button class="btn btn-sm btn-edit" type="submit">Save</button>
  </form>
  <form method="POST" action="/vtx-photos/{{.ID}}/delete" style="margin-top:6px">
    <button class="btn btn-sm btn-danger" type="submit">Delete</button>
  </form>
</div>
{{end}}
</div>
{{else}}
<p class="muted" style="margin-bottom:8px">No photos yet.</p>
{{end}}
<form method="POST" action="/vtx/{{.ID}}/photos" enctype="multipart/form-data" class="upload-form">
  <input type="file" name="photo" accept="image/*" style="color:#c9d1d9;font-size:13px">
  <button class="btn btn-primary" type="submit">Upload Photo</button>
</form>
{{end}}
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
      <select name="brand_id">
        <option value="0">— no brand —</option>
        {{range .Brands}}<option value="{{.ID}}" {{if eq $.BrandID .ID}}selected{{end}}>{{.Label}}</option>{{end}}
      </select>
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
{{if .ID}}
<h3 style="margin-top:32px">Photos</h3>
{{if .Photos}}
<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:16px;max-width:700px;margin-bottom:16px">
{{range .Photos}}
<div style="background:#161b22;border:1px solid #30363d;border-radius:6px;padding:10px">
  <img src="/gps-photos/{{.ID}}" style="width:100%;border-radius:4px;display:block;max-height:200px;object-fit:cover">
  <form method="POST" action="/gps-photos/{{.ID}}/note" style="margin-top:8px;display:flex;gap:6px;align-items:flex-start">
    <textarea name="notes" rows="2" style="flex:1;resize:vertical;font-size:13px" placeholder="Add a note…">{{.Notes}}</textarea>
    <button class="btn btn-sm btn-edit" type="submit">Save</button>
  </form>
  <form method="POST" action="/gps-photos/{{.ID}}/delete" style="margin-top:6px">
    <button class="btn btn-sm btn-danger" type="submit">Delete</button>
  </form>
</div>
{{end}}
</div>
{{else}}
<p class="muted" style="margin-bottom:8px">No photos yet.</p>
{{end}}
<form method="POST" action="/gps/{{.ID}}/photos" enctype="multipart/form-data" class="upload-form">
  <input type="file" name="photo" accept="image/*" style="color:#c9d1d9;font-size:13px">
  <button class="btn btn-primary" type="submit">Upload Photo</button>
</form>
{{end}}
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
      <select name="brand_id">
        <option value="0">— no brand —</option>
        {{range .Brands}}<option value="{{.ID}}" {{if eq $.BrandID .ID}}selected{{end}}>{{.Label}}</option>{{end}}
      </select>
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
{{if .ID}}
<h3 style="margin-top:32px">Photos</h3>
{{if .Photos}}
<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:16px;max-width:700px;margin-bottom:16px">
{{range .Photos}}
<div style="background:#161b22;border:1px solid #30363d;border-radius:6px;padding:10px">
  <img src="/rx-photos/{{.ID}}" style="width:100%;border-radius:4px;display:block;max-height:200px;object-fit:cover">
  <form method="POST" action="/rx-photos/{{.ID}}/note" style="margin-top:8px;display:flex;gap:6px;align-items:flex-start">
    <textarea name="notes" rows="2" style="flex:1;resize:vertical;font-size:13px" placeholder="Add a note…">{{.Notes}}</textarea>
    <button class="btn btn-sm btn-edit" type="submit">Save</button>
  </form>
  <form method="POST" action="/rx-photos/{{.ID}}/delete" style="margin-top:6px">
    <button class="btn btn-sm btn-danger" type="submit">Delete</button>
  </form>
</div>
{{end}}
</div>
{{else}}
<p class="muted" style="margin-bottom:8px">No photos yet.</p>
{{end}}
<form method="POST" action="/rx/{{.ID}}/photos" enctype="multipart/form-data" class="upload-form">
  <input type="file" name="photo" accept="image/*" style="color:#c9d1d9;font-size:13px">
  <button class="btn btn-primary" type="submit">Upload Photo</button>
</form>
{{end}}
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
  <th></th><th>Brand</th><th>Name</th><th>Cell</th><th>mAh</th><th>Weight</th><th>Owned</th><th>Assigned To</th><th></th>
</tr></thead>
<tbody>
{{range .Batteries}}
<tr>
  <td style="width:40px;padding:4px 6px">{{if .FirstPhotoID}}<img src="/battery-photos/{{.FirstPhotoID}}" style="width:36px;height:36px;object-fit:cover;border-radius:3px;display:block;cursor:zoom-in" onclick="openLightbox('/battery-photos/{{.FirstPhotoID}}')">{{end}}</td>
  <td class="muted">{{dash .Brand}}</td>
  <td><strong>{{.Name}}</strong></td>
  <td class="muted">{{dash .CellLabel}}</td>
  <td class="muted">{{.CapacityMAh}}</td>
  <td class="muted">{{if .WeightG}}{{.WeightG}}g{{else}}—{{end}}</td>
  <td class="muted">{{.Total}}</td>
  <td>{{if .AssignedTo}}<span class="installed-badge">{{.AssignedTo}}</span>{{else}}<span class="muted">—</span>{{end}}</td>
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
      <select name="brand_id">
        <option value="0">— no brand —</option>
        {{range .Brands}}<option value="{{.ID}}" {{if eq $.BrandID .ID}}selected{{end}}>{{.Label}}</option>{{end}}
      </select>
    </div>
    <div class="form-group">
      <label>Name *</label>
      <input type="text" name="name" value="{{.Name}}" required autofocus placeholder="e.g. 650mAh 4S">
    </div>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Cell Count *</label>
      <select name="cell_id" required>
        <option value="">—</option>
        {{range .Cells}}<option value="{{.ID}}" {{if eq $.CellID .ID}}selected{{end}}>{{.Label}}</option>{{end}}
      </select>
    </div>
    <div class="form-group">
      <label>Capacity (mAh) *</label>
      <input type="number" name="capacity_mah" value="{{.CapacityMAh}}" required placeholder="e.g. 650">
    </div>
    <div class="form-group" style="max-width:110px">
      <label>Weight (g)</label>
      <input type="number" name="weight_g" value="{{.WeightG}}" placeholder="e.g. 95" min="0">
    </div>
    <div class="form-group" style="max-width:100px">
      <label>Quantity owned</label>
      <input type="number" name="quantity" value="{{if .Quantity}}{{.Quantity}}{{else}}1{{end}}" min="0">
    </div>
  </div>
  <div class="form-group">
    <label>Notes</label>
    <input type="text" name="notes" value="{{.Notes}}" placeholder="optional">
  </div>
  <div class="form-actions">
    <button class="btn btn-primary" type="submit">{{if .ID}}Save{{else}}Add Battery{{end}}</button>
    <a href="/batteries" class="btn btn-cancel">Cancel</a>
  </div>
</form>
{{if .ID}}
<h3 style="margin-top:32px">Photos</h3>
{{if .Photos}}
<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:16px;max-width:700px;margin-bottom:16px">
{{range .Photos}}
<div style="background:#161b22;border:1px solid #30363d;border-radius:6px;padding:10px">
  <img src="/battery-photos/{{.ID}}" style="width:100%;border-radius:4px;display:block;max-height:200px;object-fit:cover">
  <form method="POST" action="/battery-photos/{{.ID}}/note" style="margin-top:8px;display:flex;gap:6px;align-items:flex-start">
    <textarea name="notes" rows="2" style="flex:1;resize:vertical;font-size:13px" placeholder="Add a note…">{{.Notes}}</textarea>
    <button class="btn btn-sm btn-edit" type="submit">Save</button>
  </form>
  <form method="POST" action="/battery-photos/{{.ID}}/delete" style="margin-top:6px">
    <button class="btn btn-sm btn-danger" type="submit">Delete</button>
  </form>
</div>
{{end}}
</div>
{{else}}
<p class="muted" style="margin-bottom:8px">No photos yet.</p>
{{end}}
<form method="POST" action="/batteries/{{.ID}}/photos" enctype="multipart/form-data" class="upload-form">
  <input type="file" name="photo" accept="image/*" style="color:#c9d1d9;font-size:13px">
  <button class="btn btn-primary" type="submit">Upload Photo</button>
</form>
{{end}}
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
  <th></th><th>Brand</th><th>Name</th><th>Size</th><th>Pitch</th><th>Blades</th>
  <th>Material</th><th>Qty</th><th>Reorder At</th><th>Drone</th><th></th>
</tr></thead>
<tbody>
{{range .Propellers}}
<tr class="{{if .LowStock}}low-stock{{end}}">
  <td style="width:40px;padding:4px 6px">{{if .FirstPhotoID}}<img src="/prop-photos/{{.FirstPhotoID}}" style="width:36px;height:36px;object-fit:cover;border-radius:3px;display:block;cursor:zoom-in" onclick="openLightbox('/prop-photos/{{.FirstPhotoID}}')">{{end}}</td>
  <td class="muted">{{dash .Brand}}</td>
  <td>{{.Name}}</td>
  <td class="muted">{{if .SizeInch}}{{.SizeInch}}"{{else}}—{{end}}</td>
  <td class="muted">{{dash .Pitch}}</td>
  <td class="muted">{{.BladeCount}}</td>
  <td class="muted">{{dash .Material}}</td>
  <td>{{.Quantity}}{{if .LowStock}} <span class="muted" title="low stock">&#9888;</span>{{end}}</td>
  <td class="muted">{{.ReorderThreshold}}</td>
  <td class="muted">{{dash .DroneNames}}</td>
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
      <select name="brand_id">
        <option value="0">— no brand —</option>
        {{range .Brands}}<option value="{{.ID}}" {{if eq $.BrandID .ID}}selected{{end}}>{{.Label}}</option>{{end}}
      </select>
    </div>
    <div class="form-group">
      <label>Name / Model *</label>
      <input type="text" name="name" value="{{.Name}}" required autofocus placeholder="e.g. 5x4.3x3 V1S">
    </div>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Size (in)</label>
      <select name="size_id">
        <option value="">—</option>
        {{range .Sizes}}<option value="{{.ID}}" {{if eq $.SizeID .ID}}selected{{end}}>{{.Label}}"</option>{{end}}
      </select>
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
    <label>Notes</label>
    <textarea name="notes">{{.Notes}}</textarea>
  </div>
  <div class="form-actions">
    <button class="btn btn-primary" type="submit">{{if .ID}}Save{{else}}Add Props{{end}}</button>
    <a href="/props" class="btn btn-cancel">Cancel</a>
  </div>
</form>
{{if .ID}}
<h3 style="margin-top:32px">Photos</h3>
{{if .Photos}}
<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:16px;max-width:700px;margin-bottom:16px">
{{range .Photos}}
<div style="background:#161b22;border:1px solid #30363d;border-radius:6px;padding:10px">
  <img src="/prop-photos/{{.ID}}" style="width:100%;border-radius:4px;display:block;max-height:200px;object-fit:cover">
  <form method="POST" action="/prop-photos/{{.ID}}/note" style="margin-top:8px;display:flex;gap:6px;align-items:flex-start">
    <textarea name="notes" rows="2" style="flex:1;resize:vertical;font-size:13px" placeholder="Add a note…">{{.Notes}}</textarea>
    <button class="btn btn-sm btn-edit" type="submit">Save</button>
  </form>
  <form method="POST" action="/prop-photos/{{.ID}}/delete" style="margin-top:6px">
    <button class="btn btn-sm btn-danger" type="submit">Delete</button>
  </form>
</div>
{{end}}
</div>
{{else}}
<p class="muted" style="margin-bottom:8px">No photos yet.</p>
{{end}}
<form method="POST" action="/props/{{.ID}}/photos" enctype="multipart/form-data" class="upload-form">
  <input type="file" name="photo" accept="image/*" style="color:#c9d1d9;font-size:13px">
  <button class="btn btn-primary" type="submit">Upload Photo</button>
</form>
{{end}}
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
  <th>Date</th><th>Title / Notes</th><th>Drone</th><th>Type</th><th>Duration</th><th>Location</th><th>Batteries</th><th></th>
</tr></thead>
<tbody>
{{range .Sessions}}
<tr>
  <td class="muted" style="white-space:nowrap">{{.SessionDate}}</td>
  <td style="max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{{if .Title}}<strong>{{.Title}}</strong>{{else}}<span class="muted">{{dash .Notes}}</span>{{end}}</td>
  <td><strong>{{.DroneNames}}</strong></td>
  <td><span class="badge {{badgeClass .Type}}">{{.Type}}</span></td>
  <td class="muted">{{if gt .DurationMin 0}}{{.DurationMin}}m{{else}}—{{end}}</td>
  <td class="muted">{{dash .Location}}</td>
  <td class="muted" style="font-size:12px">{{dash .BatteryList}}</td>
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
    <label>Title</label>
    <input type="text" name="title" value="{{.Title}}" placeholder="e.g. Golden hour at the park">
  </div>
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
      {{if .Places}}
      <select id="loc-sel" onchange="locSelChange(this)">
        <option value="">— none —</option>
        {{range .Places}}<option value="{{.Label}}">{{.Label}}</option>{{end}}
        <option value="__other__">Other…</option>
      </select>
      {{end}}
      <input type="text" id="loc-txt" name="location" value="{{.Location}}" style="margin-top:6px" placeholder="Type a location…">
      {{if .Places}}
      <script>
      (function(){
        var sel=document.getElementById('loc-sel'),txt=document.getElementById('loc-txt');
        function syncSel(){
          var v=txt.value;
          var found=[].some.call(sel.options,function(o){return o.value===v&&o.value!==''&&o.value!=='__other__';});
          sel.value=found?v:'__other__';
        }
        syncSel();
        txt.style.display=sel.value==='__other__'?'':'none';
        window.locSelChange=function(s){
          if(s.value!=='__other__'){txt.value=s.value;}
          txt.style.display=s.value==='__other__'?'':'none';
          if(s.value==='__other__')txt.focus();
        };
      })();
      </script>
      {{end}}
    </div>
  </div>
  {{if .Batteries}}
  <div class="form-group">
    <label>Batteries Used</label>
    <div style="display:flex;flex-direction:column;gap:6px">
      {{range .Batteries}}
      <div style="display:flex;align-items:center;gap:8px">
        <label class="battery-check" style="margin:0">
          <input type="checkbox" name="battery_ids" value="{{.ID}}" {{if .Checked}}checked{{end}}>
          {{.Label}}
        </label>
        <input type="number" name="battery_count_{{.ID}}" value="{{if gt .Count 0}}{{.Count}}{{else}}1{{end}}" min="1" style="width:52px;padding:2px 6px">
      </div>
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
    <h2>{{if .Title}}{{.Title}}{{else}}Session #{{.ID}}{{end}}</h2>
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
<thead><tr><th>Brand</th><th>Name</th><th>Cell</th><th>mAh</th><th>Qty</th></tr></thead>
<tbody>
{{range .Batteries}}
<tr>
  <td class="muted">{{dash .Brand}}</td>
  <td>{{.Name}}</td>
  <td class="muted">{{dash .CellLabel}}</td>
  <td class="muted">{{.CapacityMAh}}</td>
  <td><strong>{{.Count}}</strong></td>
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
  <div class="media-card-header" style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
    <span class="muted" style="font-size:13px;word-break:break-all">{{.OriginalName}}</span>
    <div style="display:flex;gap:6px;align-items:center">
      <button type="button" class="btn btn-sm" onclick="toggleOriginal(this,{{.ID}})">Original</button>
      <form class="inline" method="POST" action="/videos/{{.ID}}/delete">
        <button class="btn btn-sm btn-danger" type="submit">Delete</button>
      </form>
    </div>
  </div>
  <video controls style="width:100%;border-radius:4px;max-height:480px;background:#000" data-mobile="/videos/{{.ID}}/mobile.m3u8" data-original="/videos/{{.ID}}">
  </video>
  <div style="margin-top:10px;display:flex;gap:8px;align-items:flex-start">
    <div class="vnote-view" style="flex:1;font-size:13px;color:#8b949e;min-height:1.4em;white-space:pre-wrap;word-break:break-word">{{if .Notes}}{{.Notes}}{{else}}<span style="opacity:.5">No note</span>{{end}}</div>
    <form class="vnote-form" method="POST" action="/videos/{{.ID}}/note" style="display:none;flex:1;gap:6px;align-items:flex-start">
      <textarea name="notes" rows="2" style="flex:1;resize:vertical;width:100%">{{.Notes}}</textarea>
    </form>
    <button class="btn btn-sm btn-edit vnote-btn" onclick="vnoteEdit(this)">Edit</button>
  </div>
</div>
{{end}}
</div>
{{else}}
<p class="muted" style="margin-bottom:8px">No videos yet.</p>
{{end}}
<form id="video-upload-form" method="POST" action="/log/{{.ID}}/videos" enctype="multipart/form-data" class="upload-form" style="margin-bottom:8px">
  <input type="file" name="video" accept="video/*" style="color:#c9d1d9;font-size:13px">
  <button class="btn btn-primary" type="submit">Upload Video</button>
</form>
<div id="video-upload-progress" style="display:none;margin-bottom:24px;max-width:860px">
  <div style="background:#30363d;border-radius:4px;height:8px;overflow:hidden">
    <div id="video-upload-bar" style="background:#58a6ff;height:100%;width:0%;transition:width .15s"></div>
  </div>
  <div id="video-upload-label" style="font-size:12px;color:#8b949e;margin-top:4px">Uploading…</div>
</div>

<h3>Photos</h3>
{{if .Photos}}
<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(260px,1fr));gap:16px;max-width:860px;margin-bottom:16px">
{{range .Photos}}
<div style="background:#161b22;border:1px solid #30363d;border-radius:6px;padding:10px">
  <img src="/photos/{{.ID}}" style="width:100%;border-radius:4px;display:block;max-height:300px;object-fit:cover;cursor:zoom-in" onclick="openLightbox('/photos/{{.ID}}')">
  <div style="margin-top:8px;display:flex;gap:8px;align-items:flex-start">
    <div class="vnote-view" style="flex:1;font-size:13px;color:#8b949e;min-height:1.4em;white-space:pre-wrap;word-break:break-word">{{if .Notes}}{{.Notes}}{{else}}<span style="opacity:.5">No note</span>{{end}}</div>
    <form class="vnote-form" method="POST" action="/photos/{{.ID}}/note" style="display:none;flex:1;gap:6px;align-items:flex-start">
      <textarea name="notes" rows="2" style="flex:1;resize:vertical;width:100%">{{.Notes}}</textarea>
    </form>
    <button class="btn btn-sm btn-edit vnote-btn" onclick="vnoteEdit(this)">Edit</button>
  </div>
  <form method="POST" action="/photos/{{.ID}}/delete" style="margin-top:6px">
    <button class="btn btn-sm btn-danger" type="submit">Delete</button>
  </form>
</div>
{{end}}
</div>
{{else}}
<p class="muted" style="margin-bottom:8px">No photos yet.</p>
{{end}}
<form method="POST" action="/log/{{.ID}}/photos" enctype="multipart/form-data" class="upload-form" style="margin-bottom:32px">
  <input type="file" name="photo" accept="image/*" style="color:#c9d1d9;font-size:13px">
  <button class="btn btn-primary" type="submit">Upload Photo</button>
</form>

<p><a href="/log">&larr; Back to log</a></p>
<script src="https://cdn.jsdelivr.net/npm/hls.js@1.4"></script>
<script>
function attachVideo(video, src) {
  if (video.hlsInstance) { video.hlsInstance.destroy(); video.hlsInstance = null; }
  var isHls = src.indexOf('.m3u8') !== -1;
  if (isHls && typeof Hls !== 'undefined' && Hls.isSupported()) {
    var hls = new Hls();
    hls.loadSource(src);
    hls.attachMedia(video);
    video.hlsInstance = hls;
  } else if (isHls && video.canPlayType('application/vnd.apple.mpegurl')) {
    video.src = src;
    video.load();
  } else {
    video.src = src;
    video.load();
  }
}
function toggleOriginal(btn, id) {
  var video = btn.closest('.media-card-header').nextElementSibling;
  var showOriginal = btn.textContent === 'Original';
  attachVideo(video, showOriginal ? video.dataset.original : video.dataset.mobile);
  btn.textContent = showOriginal ? 'Mobile' : 'Original';
  video.play().catch(function(){});
}
document.querySelectorAll('video[data-mobile]').forEach(function(v){ attachVideo(v, v.dataset.mobile); });
document.getElementById('video-upload-form').addEventListener('submit', function(e) {
  e.preventDefault();
  var form = this;
  var bar = document.getElementById('video-upload-bar');
  var label = document.getElementById('video-upload-label');
  var progress = document.getElementById('video-upload-progress');
  var btn = form.querySelector('button[type=submit]');
  var xhr = new XMLHttpRequest();
  progress.style.display = 'block';
  btn.disabled = true;
  xhr.upload.onprogress = function(e) {
    if (e.lengthComputable) {
      var pct = Math.round(e.loaded / e.total * 100);
      bar.style.width = pct + '%';
      label.textContent = 'Uploading… ' + pct + '%';
    }
  };
  xhr.onload = function() {
    if (xhr.status < 400) {
      bar.style.width = '100%';
      label.textContent = 'Done!';
      window.location.reload();
    } else {
      label.textContent = 'Upload failed.';
      btn.disabled = false;
    }
  };
  xhr.onerror = function() { label.textContent = 'Upload failed.'; btn.disabled = false; };
  xhr.open('POST', form.action);
  xhr.send(new FormData(form));
});
function vnoteEdit(btn) {
  var wrap = btn.parentElement;
  var view = wrap.querySelector('.vnote-view');
  var form = wrap.querySelector('.vnote-form');
  if (btn.textContent === 'Edit') {
    view.style.display = 'none';
    form.style.display = 'flex';
    form.querySelector('textarea').focus();
    btn.textContent = 'Save';
    btn.onclick = function(){ form.submit(); };
  }
}
function openLightbox(src){var lb=document.getElementById('lightbox');document.getElementById('lightbox-img').src=src;lb.style.display='flex';}
function closeLightbox(){document.getElementById('lightbox').style.display='none';}
document.addEventListener('keydown',function(e){if(e.key==='Escape')closeLightbox();});
</script>
<div id="lightbox" style="display:none;position:fixed;inset:0;background:rgba(0,0,0,.85);z-index:9999;align-items:center;justify-content:center" onclick="closeLightbox()">
  <img id="lightbox-img" src="" style="max-width:90vw;max-height:90vh;border-radius:8px;object-fit:contain;box-shadow:0 8px 40px rgba(0,0,0,.6)">
</div>
{{end}}`

const placeListTmpl = `{{define "content"}}
<div class="page-header">
  <div class="page-header-left"><h2>Places</h2></div>
  <a href="/places/new" class="btn btn-primary">+ Add Place</a>
</div>
{{if .Places}}
<div class="table-wrap">
<table>
<thead><tr>
  <th>Name</th><th>Type</th><th>Address</th><th>Notes</th><th></th>
</tr></thead>
<tbody>
{{range .Places}}
<tr>
  <td><strong>{{.Name}}</strong></td>
  <td class="muted">{{.PlaceType}}</td>
  <td class="muted">{{dash .Address}}</td>
  <td class="muted" style="max-width:220px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{{dash .Notes}}</td>
  <td class="actions-cell">
    <a href="/places/{{.ID}}" class="btn btn-sm btn-edit">View</a>
    <a href="/places/{{.ID}}/edit" class="btn btn-sm btn-edit">Edit</a>
    <form class="inline" method="POST" action="/places/{{.ID}}/delete">
      <button class="btn btn-sm btn-danger" type="submit">Delete</button>
    </form>
  </td>
</tr>
{{end}}
</tbody>
</table>
</div>
<link rel="stylesheet" href="https://unpkg.com/leaflet@1.9.4/dist/leaflet.css">
<script src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js"></script>
<div id="places-map" style="height:360px;border-radius:6px;border:1px solid #30363d;margin-top:24px"></div>
<script>
(function(){
  var map = L.map('places-map');
  L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png',{
    attribution:'&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
  }).addTo(map);
  var group = L.featureGroup().addTo(map);
  {{range .Places}}{{if .HasCoords}}
  L.marker([{{.LatStr}},{{.LngStr}}]).addTo(group)
    .bindPopup('<a href="/places/{{.ID}}"><strong>{{.Name}}</strong></a><br><span style="color:#8b949e">{{.PlaceType}}</span>');
  {{end}}{{end}}
  if (group.getLayers().length > 0) {
    map.fitBounds(group.getBounds().pad(0.2));
  } else {
    map.setView([20,0],2);
  }
})();
</script>
{{else}}
<p class="muted">No places yet. <a href="/places/new">Add your first flying spot.</a></p>
{{end}}
{{end}}`

const placeFormTmpl = `{{define "content"}}
<div class="page-header">
  <h2>{{if .ID}}Edit Place{{else}}New Place{{end}}</h2>
</div>
<div class="form-page">
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
<form method="POST">
  <div class="form-group">
    <label>Name *</label>
    <input type="text" name="name" value="{{.Name}}" required autofocus placeholder="e.g. Riverside Park Field">
  </div>
  <div class="form-group">
    <label>Address *</label>
    <input type="text" name="address" value="{{.Address}}" required placeholder="e.g. 123 Main St, Springfield, CA">
  </div>
  <div class="form-group" style="max-width:200px">
    <label>Type</label>
    <select name="place_type">
      <option value="field"    {{if eq .PlaceType "field"}}selected{{end}}>field</option>
      <option value="park"     {{if eq .PlaceType "park"}}selected{{end}}>park</option>
      <option value="backyard" {{if eq .PlaceType "backyard"}}selected{{end}}>backyard</option>
      <option value="rooftop"  {{if eq .PlaceType "rooftop"}}selected{{end}}>rooftop</option>
      <option value="other"    {{if eq .PlaceType "other"}}selected{{end}}>other</option>
    </select>
  </div>
  <div class="form-group">
    <label>Notes</label>
    <textarea name="notes" placeholder="e.g. No flying after sunset, LAANC authorization required">{{.Notes}}</textarea>
  </div>
  <div class="form-actions">
    <button class="btn btn-primary" type="submit">{{if .ID}}Save Changes{{else}}Add Place{{end}}</button>
    <a href="/places" class="btn btn-cancel">Cancel</a>
  </div>
</form>
<p class="muted" style="margin-top:16px;font-size:12px">The address will be geocoded automatically to show a map pin.</p>
</div>
{{end}}`

const placeDetailTmpl = `{{define "content"}}
<div class="page-header">
  <div class="page-header-left">
    <h2>{{.Name}}</h2>
    <div class="summary">{{.PlaceType}} &mdash; {{.Address}}</div>
  </div>
  <div>
    <a href="/places/{{.ID}}/edit" class="btn btn-edit">Edit</a>
    <form class="inline" method="POST" action="/places/{{.ID}}/delete">
      <button class="btn btn-danger" type="submit">Delete</button>
    </form>
  </div>
</div>

<table style="max-width:500px;margin-bottom:24px">
<tr><td class="muted" style="padding:6px 12px 6px 0;width:100px">Type</td><td>{{.PlaceType}}</td></tr>
<tr><td class="muted" style="padding:6px 12px 6px 0">Address</td><td>{{dash .Address}}</td></tr>
{{if .HasCoords}}
<tr><td class="muted" style="padding:6px 12px 6px 0">Coordinates</td><td class="muted" style="font-size:12px">{{.LatStr}}, {{.LngStr}}</td></tr>
{{end}}
<tr><td class="muted" style="padding:6px 12px 6px 0;vertical-align:top">Notes</td><td style="white-space:pre-wrap">{{dash .Notes}}</td></tr>
</table>

{{if .HasCoords}}
<link rel="stylesheet" href="https://unpkg.com/leaflet@1.9.4/dist/leaflet.css">
<script src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js"></script>
<div id="map" style="height:420px;border-radius:6px;border:1px solid #30363d;margin-bottom:24px"></div>
<script>
(function(){
  var lat={{.LatStr}}, lng={{.LngStr}};
  var map=L.map('map').setView([lat,lng],15);
  L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png',{
    attribution:'&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
  }).addTo(map);
  L.marker([lat,lng]).addTo(map).bindPopup("{{.Name}}").openPopup();
})();
</script>
{{else}}
<div style="background:#161b22;border:1px solid #30363d;border-radius:6px;padding:20px;color:#8b949e;font-size:13px;margin-bottom:24px">
  No map coordinates available. <a href="/places/{{.ID}}/edit">Edit the place</a> to re-save and retry geocoding.
</div>
{{end}}

<p><a href="/places">&larr; Back to places</a></p>
{{end}}`

const settingsTmpl = `{{define "content"}}
<div class="page-header"><h2>Settings</h2></div>

<div class="section">
  <div class="section-header">
    <h3>Brands</h3>
    <a href="/brands/new" class="btn btn-sm btn-primary">+ Add</a>
  </div>
  {{if .Brands}}
  <div class="table-wrap"><table>
  <thead><tr><th>Name</th><th></th></tr></thead>
  <tbody>
  {{range .Brands}}<tr>
    <td><strong>{{.Name}}</strong></td>
    <td class="actions-cell">
      <a href="/brands/{{.ID}}/edit" class="btn btn-sm btn-edit">Edit</a>
      <form class="inline" method="POST" action="/brands/{{.ID}}/delete">
        <button class="btn btn-sm btn-danger" type="submit">Delete</button>
      </form>
    </td>
  </tr>{{end}}
  </tbody></table></div>
  {{else}}<p class="muted">No brands yet. <a href="/brands/new">Add one.</a></p>{{end}}
</div>

<div class="section">
  <div class="section-header">
    <h3>Sizes</h3>
    <a href="/sizes/new" class="btn btn-sm btn-primary">+ Add</a>
  </div>
  {{if .Sizes}}
  <div class="table-wrap"><table>
  <thead><tr><th>Label</th><th></th></tr></thead>
  <tbody>
  {{range .Sizes}}<tr>
    <td><strong>{{.Label}}"</strong></td>
    <td class="actions-cell">
      <a href="/sizes/{{.ID}}/edit" class="btn btn-sm btn-edit">Edit</a>
      <form class="inline" method="POST" action="/sizes/{{.ID}}/delete">
        <button class="btn btn-sm btn-danger" type="submit">Delete</button>
      </form>
    </td>
  </tr>{{end}}
  </tbody></table></div>
  {{else}}<p class="muted">No sizes yet. <a href="/sizes/new">Add one.</a></p>{{end}}
</div>

<div class="section">
  <div class="section-header">
    <h3>Cells</h3>
    <a href="/cells/new" class="btn btn-sm btn-primary">+ Add</a>
  </div>
  {{if .Cells}}
  <div class="table-wrap"><table>
  <thead><tr><th>Label</th><th></th></tr></thead>
  <tbody>
  {{range .Cells}}<tr>
    <td><strong>{{.Label}}</strong></td>
    <td class="actions-cell">
      <a href="/cells/{{.ID}}/edit" class="btn btn-sm btn-edit">Edit</a>
      <form class="inline" method="POST" action="/cells/{{.ID}}/delete">
        <button class="btn btn-sm btn-danger" type="submit">Delete</button>
      </form>
    </td>
  </tr>{{end}}
  </tbody></table></div>
  {{else}}<p class="muted">No cells yet. <a href="/cells/new">Add one.</a></p>{{end}}
</div>
{{end}}`

const brandFormTmpl = `{{define "content"}}
<div class="page-header">
  <h2>{{if .ID}}Edit Brand{{else}}New Brand{{end}}</h2>
</div>
<div class="form-page">
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
<form method="POST">
  <div class="form-group">
    <label>Name *</label>
    <input type="text" name="name" value="{{.Name}}" required autofocus placeholder="e.g. iFlight">
  </div>
  <div class="form-actions">
    <button class="btn btn-primary" type="submit">{{if .ID}}Save{{else}}Add Brand{{end}}</button>
    <a href="/settings" class="btn btn-cancel">Cancel</a>
  </div>
</form>
</div>
{{end}}`

const cellFormTmpl = `{{define "content"}}
<div class="page-header">
  <h2>{{if .ID}}Edit Cell{{else}}New Cell{{end}}</h2>
</div>
<div class="form-page">
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
<form method="POST">
  <div class="form-group">
    <label>Label *</label>
    <input type="text" name="label" value="{{.Label}}" required autofocus placeholder="e.g. 4S">
  </div>
  <div class="form-actions">
    <button class="btn btn-primary" type="submit">{{if .ID}}Save{{else}}Add{{end}}</button>
    <a href="/settings" class="btn btn-cancel">Cancel</a>
  </div>
</form>
</div>
{{end}}`

const sizeFormTmpl = `{{define "content"}}
<div class="page-header">
  <h2>{{if .ID}}Edit Size{{else}}New Size{{end}}</h2>
</div>
<div class="form-page">
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
<form method="POST">
  <div class="form-group">
    <label>Label (inches) *</label>
    <input type="text" name="label" value="{{.Label}}" required autofocus placeholder="e.g. 5">
  </div>
  <div class="form-actions">
    <button class="btn btn-primary" type="submit">{{if .ID}}Save{{else}}Add Size{{end}}</button>
    <a href="/settings" class="btn btn-cancel">Cancel</a>
  </div>
</form>
</div>
{{end}}`

const weatherTmpl = `{{define "content"}}
<style>
.wx-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 12px; margin-bottom: 32px; }
.wx-card {
  background: #161b22;
  border: 1px solid #30363d;
  border-radius: 8px;
  padding: 16px;
}
.wx-card-top { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 10px; }
.wx-date { font-size: 13px; font-weight: 600; color: #8b949e; text-transform: uppercase; letter-spacing: 0.5px; }
.wx-fly { display: inline-block; padding: 2px 8px; border-radius: 10px; font-size: 11px; font-weight: 600; }
.wx-fly-good    { background: rgba(63,185,80,0.15);  color: #3fb950; border: 1px solid rgba(63,185,80,0.4); }
.wx-fly-caution { background: rgba(210,153,34,0.15); color: #d29922; border: 1px solid rgba(210,153,34,0.4); }
.wx-fly-bad     { background: rgba(248,81,73,0.15);  color: #f85149; border: 1px solid rgba(248,81,73,0.4); }
.wx-desc { font-size: 14px; color: #c9d1d9; margin-bottom: 12px; }
.wx-stats { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
.wx-stat { background: #0d1117; border-radius: 6px; padding: 8px 10px; }
.wx-stat-label { font-size: 10px; color: #8b949e; text-transform: uppercase; letter-spacing: 0.4px; margin-bottom: 2px; }
.wx-stat-value { font-size: 15px; font-weight: 600; color: #f0f6fc; }
.wx-stat-sub { font-size: 11px; color: #8b949e; margin-top: 1px; }
.rain-bar-wrap { height: 4px; background: #21262d; border-radius: 2px; margin-top: 6px; }
.rain-bar { height: 4px; border-radius: 2px; }
.rain-low  { background: #3fb950; }
.rain-med  { background: #d29922; }
.rain-high { background: #f85149; }
.wind-ok      { color: #3fb950; }
.wind-caution { color: #d29922; }
.wind-bad     { color: #f85149; }
.wx-extended { font-size: 12px; color: #8b949e; margin-top: 10px; border-top: 1px solid #21262d; padding-top: 8px; line-height: 1.5; }
.hourly-section { margin-top: 8px; }
.hourly-table th, .hourly-table td { padding: 7px 10px; }
.hour-night { background: rgba(88,166,255,0.04); }
.hour-good  { background: rgba(63,185,80,0.05); }
.wx-chart-section { margin-top: 10px; padding-top: 10px; border-top: 1px solid #21262d; display: flex; flex-direction: column; gap: 6px; }
.wx-chart-title { font-size: 10px; color: #8b949e; text-transform: uppercase; letter-spacing: 0.4px; margin-bottom: 2px; }
.wx-chart-wrap { display: flex; gap: 4px; }
.wx-yaxis { width: 22px; position: relative; height: 48px; flex-shrink: 0; }
.wx-yaxis-max, .wx-yaxis-min { position: absolute; right: 0; font-size: 8px; color: #8b949e; line-height: 1; }
.wx-yaxis-max { top: 0; }
.wx-yaxis-min { bottom: 0; }
.wx-yaxis-label { position: absolute; right: 0; font-size: 8px; color: #8b949e; transform: translateY(-50%); line-height: 1; }
.wx-bars-wrap { position: relative; height: 48px; }
.wx-guideline { position: absolute; left: 0; right: 0; border-top: 1px dashed; z-index: 1; pointer-events: none; }
.wx-chart { display: flex; align-items: stretch; height: 100%; gap: 2px; position: relative; z-index: 2; }
.wx-chart-col { flex: 1; min-width: 0; display: flex; align-items: flex-end; }
.wx-bar { width: 100%; min-height: 2px; border-radius: 2px 2px 0 0; }
.wx-hours-row { display: flex; gap: 2px; margin-top: 2px; }
.wx-hour-cell { flex: 1; font-size: 8px; color: #8b949e; text-align: center; min-width: 0; overflow: hidden; }
</style>

<div class="page-header">
  <div>
    <h2>Melbourne Weather</h2>
    <div class="summary">7-day forecast · Rain &amp; wind focus · Source: Bureau of Meteorology</div>
  </div>
  <div style="display:flex;align-items:center;gap:12px">
    <span class="muted" style="font-size:12px">Updated {{.FetchedAt}}</span>
    <a href="/weather" class="btn btn-edit btn-sm">Refresh</a>
  </div>
</div>

{{if .Error}}
<div class="error-box">{{.Error}}</div>
{{else}}

<div style="display:flex;gap:12px;margin-bottom:16px;font-size:12px;color:#8b949e;align-items:center">
  <span>FPV rating:</span>
  <span class="wx-fly wx-fly-good">Fly</span> &lt;20 km/h wind, &lt;35% rain
  <span class="wx-fly wx-fly-caution">Caution</span> 20–35 km/h or 35–65% rain
  <span class="wx-fly wx-fly-bad">No-fly</span> &gt;35 km/h or &gt;65% rain
</div>

<div class="wx-grid">
{{range .Days}}
<div class="wx-card">
  <div class="wx-card-top">
    <div class="wx-date">{{.Date}}</div>
    <span class="wx-fly wx-fly-{{.FlyRating}}">{{if eq .FlyRating "good"}}Fly{{else if eq .FlyRating "caution"}}Caution{{else}}No-fly{{end}}</span>
  </div>
  <div class="wx-desc">{{.ShortText}} <span class="muted">{{.TempMin}}–{{.TempMax}}°C</span></div>
  <div class="wx-stats">
    <div class="wx-stat">
      <div class="wx-stat-label">Rain chance</div>
      <div class="wx-stat-value {{if gt .RainChance 65}}wind-bad{{else if gt .RainChance 35}}wind-caution{{else}}wind-ok{{end}}">{{.RainChance}}%</div>
      <div class="rain-bar-wrap"><div class="rain-bar {{if gt .RainChance 65}}rain-high{{else if gt .RainChance 35}}rain-med{{else}}rain-low{{end}}" style="width:{{.RainChance}}%"></div></div>
    </div>
    <div class="wx-stat">
      <div class="wx-stat-label">Max rain</div>
      <div class="wx-stat-value">{{.RainMaxMm}}</div>
    </div>
    <div class="wx-stat">
      <div class="wx-stat-label">Wind (max)</div>
      {{if .HasWind}}
      <div class="wx-stat-value {{if gt .WindSpeedKmh 35}}wind-bad{{else if gt .WindSpeedKmh 20}}wind-caution{{else}}wind-ok{{end}}">{{.WindSpeedKmh}} km/h</div>
      <div class="wx-stat-sub">{{.WindDir}}</div>
      {{else}}
      <div class="wx-stat-value muted">—</div>
      {{end}}
    </div>
    <div class="wx-stat">
      <div class="wx-stat-label">Gust (max)</div>
      {{if .HasWind}}
      <div class="wx-stat-value {{if gt .GustSpeedKmh 50}}wind-bad{{else if gt .GustSpeedKmh 30}}wind-caution{{else}}wind-ok{{end}}">{{.GustSpeedKmh}} km/h</div>
      {{else}}
      <div class="wx-stat-value muted">—</div>
      {{end}}
    </div>
  </div>
  {{if .ExtendedText}}
  <div class="wx-extended">{{.ExtendedText}}</div>
  {{end}}
  {{if .DayHours}}
  <div class="wx-chart-section">
    <div class="wx-chart-title">Wind km/h</div>
    <div class="wx-chart-wrap">
      <div class="wx-yaxis">
        <span class="wx-yaxis-max">{{.WindMaxLabel}}</span>
        {{range .WindGuides}}<span class="wx-yaxis-label" style="top:{{.TopPct}}%">{{.Label}}</span>{{end}}
        <span class="wx-yaxis-min">0</span>
      </div>
      <div style="flex:1">
        <div class="wx-bars-wrap">
          {{range .WindGuides}}<div class="wx-guideline" style="top:{{.TopPct}}%;border-top-color:{{.Color}}"></div>{{end}}
          <div class="wx-chart">{{range .DayHours}}<div class="wx-chart-col"><div class="wx-bar" style="height:{{.WindBarH}}%;background:{{.WindFill}}"></div></div>{{end}}</div>
        </div>
        <div class="wx-hours-row">{{range .DayHours}}<div class="wx-hour-cell">{{.Label}}</div>{{end}}</div>
      </div>
    </div>
    <div class="wx-chart-title">Rain %</div>
    <div class="wx-chart-wrap">
      <div class="wx-yaxis">
        <span class="wx-yaxis-max">100</span>
        {{range .RainGuides}}<span class="wx-yaxis-label" style="top:{{.TopPct}}%">{{.Label}}</span>{{end}}
        <span class="wx-yaxis-min">0</span>
      </div>
      <div style="flex:1">
        <div class="wx-bars-wrap">
          {{range .RainGuides}}<div class="wx-guideline" style="top:{{.TopPct}}%;border-top-color:{{.Color}}"></div>{{end}}
          <div class="wx-chart">{{range .DayHours}}<div class="wx-chart-col"><div class="wx-bar" style="height:{{.RainBarH}}%;background:#4d9de0"></div></div>{{end}}</div>
        </div>
        <div class="wx-hours-row">{{range .DayHours}}<div class="wx-hour-cell">{{.Label}}</div>{{end}}</div>
      </div>
    </div>
  </div>
  {{end}}
</div>
{{end}}
</div>

{{end}}
{{end}}`
