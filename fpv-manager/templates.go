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

type FrameOption struct {
	ID     int
	Label  string
	SizeID int
}

type DroneListPage struct {
	ActiveTab string
	Drones    []DroneRow
}

type DroneRow struct {
	ID             int
	Name           string
	SizeInch       string
	CellLabel      string
	Status         string
	Sub250g        bool
	FlightCount    int
	FirstPhotoID   int
	HasFlightToday bool
}

type DronePhotoRow struct {
	ID           int
	OriginalName string
	Notes        string
}

type DroneLogEntry struct {
	ID            int
	LoggedAt      string
	LoggedAtInput string
	Body          string
}

type DroneTimelineEntry struct {
	Kind string // "log" or "session"
	SortKey string

	// log fields
	LogID         int
	LoggedAt      string
	LoggedAtInput string
	Body          string

	// session fields
	SessionID   int
	SessionTitle string
	SessionType  string
	Date        string
	DurationMin int
	Location    string
}

type DronePage struct {
	ActiveTab    string
	ID           int
	Name         string
	Status       string
	SizeID       int
	CellID       int
	FrameID      int
	FCID         int
	ESCID        int
	VTXID        int
	MotorID      int
	MotorCount   string
	GPSID        int
	RXID         int
	BuildDate    string
	WeightG      string
	Sub250g      bool
	Notes        string
	BatteryNames string
	PropNames    string
	Batteries    []BatteryCheck
	Photos       []DronePhotoRow
	Timeline     []DroneTimelineEntry
	Sizes        []OptionItem
	Cells        []OptionItem
	Frames       []FrameOption
	FCs          []OptionItem
	ESCs         []OptionItem
	VTXs         []OptionItem
	Motors       []OptionItem
	GPSs         []OptionItem
	RXs          []OptionItem
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
	Sub250g      bool
	Notes        string
	Sizes        []OptionItem
	Cells        []OptionItem
	Frames       []FrameOption
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
	ActiveTab  string
	Error      string
	ID         int
	BrandID    int
	Name       string
	ProtocolID int
	Notes      string
	Quantity   string
	Photos     []DronePhotoRow
	Brands     []OptionItem
	Protocols  []OptionItem
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
	MCUID     int
	Firmware  string
	Notes     string
	Quantity  string
	Photos    []DronePhotoRow
	Brands    []OptionItem
	MCUs      []OptionItem
}

type ESCFormPage struct {
	ActiveTab     string
	Error         string
	ID            int
	BrandID       int
	Name          string
	CurrentRating string
	CellMaxID     int
	Notes         string
	Quantity      string
	Photos        []DronePhotoRow
	Brands        []OptionItem
	Cells         []OptionItem
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
	BrandID int
	Name    string
	WeightG string
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
	ID             int
	Title          string
	DroneNames     string
	Type           string
	SessionDate    string
	DurationMin    int
	Location       string
	Notes          string
	BatteryList    string
	IsQuickFlight  bool
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
	DroneID      int
	DroneName    string
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
	Drones     []OptionItem
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

type RadioProtocolRow struct {
	ID   int
	Name string
}

type RadioProtocolFormPage struct {
	ActiveTab string
	Error     string
	ID        int
	Name      string
}

type MCURow struct {
	ID   int
	Name string
}

type MCUFormPage struct {
	ActiveTab string
	Error     string
	ID        int
	Name      string
}

type SettingsPage struct {
	ActiveTab      string
	Brands         []BrandRow
	Sizes          []SizeRow
	Cells          []CellRow
	RadioProtocols []RadioProtocolRow
	MCUs           []MCURow
	Config         AppConfig
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
	add("drone", droneTmpl)
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
	add("radio-protocol-form", radioProtocolFormTmpl)
	add("mcu-form", mcuFormTmpl)
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
.actions-cell { white-space: nowrap; text-align: right; }
.installed-badge {
  font-size: 12px;
  color: #58a6ff;
}
.upload-form { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
#drone-name:focus { border-bottom-color: #58a6ff !important; }
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
  .drone-cols { grid-template-columns: 1fr !important; }
  #drone-name { width: 100%; min-width: 0; box-sizing: border-box; }
  .drone-motor-row { flex-wrap: wrap; }
}
`

// ---- Base template ----

const baseTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>FPV Manager v0.5</title>
<link rel="icon" type="image/svg+xml" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Cline x1='16' y1='16' x2='5' y2='5' stroke='%2358a6ff' stroke-width='2.5' stroke-linecap='round'/%3E%3Cline x1='16' y1='16' x2='27' y2='5' stroke='%2358a6ff' stroke-width='2.5' stroke-linecap='round'/%3E%3Cline x1='16' y1='16' x2='5' y2='27' stroke='%2358a6ff' stroke-width='2.5' stroke-linecap='round'/%3E%3Cline x1='16' y1='16' x2='27' y2='27' stroke='%2358a6ff' stroke-width='2.5' stroke-linecap='round'/%3E%3Ccircle cx='5' cy='5' r='4' fill='%2358a6ff'/%3E%3Ccircle cx='27' cy='5' r='4' fill='%2358a6ff'/%3E%3Ccircle cx='5' cy='27' r='4' fill='%2358a6ff'/%3E%3Ccircle cx='27' cy='27' r='4' fill='%2358a6ff'/%3E%3Ccircle cx='16' cy='16' r='3.5' fill='%2358a6ff'/%3E%3C/svg%3E">
<style>` + css + `</style>
</head>
<body>
<header><span class="logo"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="20" height="20" style="vertical-align:-4px;margin-right:6px"><line x1="16" y1="16" x2="5" y2="5" stroke="#58a6ff" stroke-width="2.5" stroke-linecap="round"/><line x1="16" y1="16" x2="27" y2="5" stroke="#58a6ff" stroke-width="2.5" stroke-linecap="round"/><line x1="16" y1="16" x2="5" y2="27" stroke="#58a6ff" stroke-width="2.5" stroke-linecap="round"/><line x1="16" y1="16" x2="27" y2="27" stroke="#58a6ff" stroke-width="2.5" stroke-linecap="round"/><circle cx="5" cy="5" r="4" fill="#58a6ff"/><circle cx="27" cy="5" r="4" fill="#58a6ff"/><circle cx="5" cy="27" r="4" fill="#58a6ff"/><circle cx="27" cy="27" r="4" fill="#58a6ff"/><circle cx="16" cy="16" r="3.5" fill="#58a6ff"/></svg>FPV Manager <span style="font-size:0.7em;opacity:0.5;font-weight:400">v0.5</span></span></header>
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
  <th></th><th>Name</th><th>Size</th><th>Cell</th><th>Status</th><th>Flights</th><th>Sub 250g</th><th></th>
</tr></thead>
<tbody>
{{range .Drones}}
<tr class="{{if eq .Status "retired"}}retired{{end}}" style="cursor:pointer" onclick="window.location='/drones/{{.ID}}'">
  <td style="width:56px;padding:6px 8px" onclick="event.stopPropagation()">
    {{if .FirstPhotoID}}
    <img src="/drone-photos/{{.FirstPhotoID}}" style="width:48px;height:48px;object-fit:cover;border-radius:4px;display:block;cursor:zoom-in" onclick="event.stopPropagation();openLightbox('/drone-photos/{{.FirstPhotoID}}')">
    {{else}}
    <div style="width:48px;height:48px;background:#21262d;border-radius:4px;display:flex;align-items:center;justify-content:center;color:#8b949e;font-size:20px">✈</div>
    {{end}}
  </td>
  <td style="font-weight:600">{{.Name}}</td>
  <td class="muted">{{if .SizeInch}}{{.SizeInch}}"{{else}}—{{end}}</td>
  <td class="muted">{{dash .CellLabel}}</td>
  <td><span class="badge {{badgeClass .Status}}">{{.Status}}</span></td>
  <td class="muted">{{if gt .FlightCount 0}}{{.FlightCount}}{{else}}—{{end}}</td>
  <td>{{if .Sub250g}}<span style="color:#3fb950;font-weight:600">✓</span>{{else}}<span class="muted">—</span>{{end}}</td>
  <td style="width:44px;padding:4px 8px;text-align:center" onclick="event.stopPropagation()">
    {{if .HasFlightToday}}
    <button class="btn btn-sm" disabled style="opacity:0.35;cursor:not-allowed" title="Already flown today">+</button>
    {{else}}
    <form class="inline" method="POST" action="/drones/{{.ID}}/quick-flight">
      <button class="btn btn-sm btn-primary" type="submit" title="Log quick flight">+</button>
    </form>
    {{end}}
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

const droneTmpl = `{{define "content"}}
<div class="page-header">
  <div class="page-header-left" style="gap:12px;align-items:center">
    <input type="text" id="drone-name" value="{{.Name}}" onchange="saveDrone()"
      style="font-size:1.4rem;font-weight:600;background:none;border:none;border-bottom:1px solid transparent;color:#c9d1d9;padding:2px 0;min-width:160px;width:auto;outline:none">
    <span class="badge {{badgeClass .Status}}">{{.Status}}</span>
  </div>
  <div style="display:flex;gap:8px;align-items:center">
    <button class="btn btn-cancel" id="toggle-detail" onclick="toggleDroneDetail()">Show drone detail</button>
    <form class="inline" method="POST" action="/drones/{{.ID}}/delete">
      <button class="btn btn-danger" type="submit">Delete</button>
    </form>
    <a href="/drones" class="btn btn-cancel">← Back</a>
  </div>
</div>

<form id="drone-form" action="/drones/{{.ID}}/save" method="POST">
<div class="section" id="drone-detail-section">
  <div class="drone-cols" style="display:grid;grid-template-columns:1fr 1fr;gap:24px;max-width:900px">
    <div>
      {{if .Photos}}
      <img src="/drone-photos/{{(index .Photos 0).ID}}" style="width:100%;border-radius:6px;object-fit:cover;max-height:260px;display:block;cursor:zoom-in;margin-bottom:16px" onclick="openLightbox('/drone-photos/{{(index .Photos 0).ID}}')">
      {{end}}
      <table style="border-collapse:collapse;width:100%">
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap;width:90px">Status</td><td>
          <select name="status" onchange="saveDrone()">
            <option value="build"     {{if eq .Status "build"}}selected{{end}}>build</option>
            <option value="flying"    {{if eq .Status "flying"}}selected{{end}}>flying</option>
            <option value="repairing" {{if eq .Status "repairing"}}selected{{end}}>repairing</option>
            <option value="retired"   {{if eq .Status "retired"}}selected{{end}}>retired</option>
          </select>
        </td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">Size</td><td>
          <select name="size_id" onchange="filterFrames();saveDrone()">
            <option value="">—</option>
            {{range .Sizes}}<option value="{{.ID}}" {{if eq $.SizeID .ID}}selected{{end}}>{{.Label}}"</option>{{end}}
          </select>
        </td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">Cell Count</td><td>
          <select name="cell_id" onchange="saveDrone()">
            <option value="">—</option>
            {{range .Cells}}<option value="{{.ID}}" {{if eq $.CellID .ID}}selected{{end}}>{{.Label}}</option>{{end}}
          </select>
        </td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">Weight (g)</td><td>
          <input type="number" name="weight_g" value="{{.WeightG}}" min="0" placeholder="e.g. 250" onchange="saveDrone()" style="width:100px">
        </td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">Build Date</td><td>
          <input type="date" name="build_date" value="{{.BuildDate}}" onchange="saveDrone()">
        </td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">Sub 250g</td><td>
          <input type="checkbox" name="sub_250g" {{if .Sub250g}}checked{{end}} onchange="saveDrone()">
        </td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap;vertical-align:top;padding-top:8px">Batteries</td><td>
          {{if .Batteries}}
          <div class="battery-checks" id="bat-checks">
            {{range .Batteries}}
            <label class="battery-check">
              <input type="checkbox" name="battery_ids" value="{{.ID}}" {{if .Checked}}checked{{end}} onchange="saveBatteries()">
              {{.Label}}
            </label>
            {{end}}
          </div>
          {{else}}<span class="muted">—</span>{{end}}
        </td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">Props</td><td><span class="muted">{{dash .PropNames}}</span></td></tr>
      </table>
    </div>
    <div>
      <table style="border-collapse:collapse;width:100%">
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap;width:90px">Frame</td><td>
          <select name="frame_id" onchange="saveDrone()">
            <option value="">— none —</option>
            {{range .Frames}}<option value="{{.ID}}" data-size-id="{{.SizeID}}" {{if eq $.FrameID .ID}}selected{{end}}>{{.Label}}</option>{{end}}
          </select>
        </td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">FC</td><td>
          <select name="fc_id" onchange="saveDrone()">
            <option value="">— none —</option>
            {{range .FCs}}<option value="{{.ID}}" {{if eq $.FCID .ID}}selected{{end}}>{{.Label}}</option>{{end}}
          </select>
        </td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">ESC</td><td>
          <select name="esc_id" onchange="saveDrone()">
            <option value="">— none —</option>
            {{range .ESCs}}<option value="{{.ID}}" {{if eq $.ESCID .ID}}selected{{end}}>{{.Label}}</option>{{end}}
          </select>
        </td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">VTX</td><td>
          <select name="vtx_id" onchange="saveDrone()">
            <option value="">— none —</option>
            {{range .VTXs}}<option value="{{.ID}}" {{if eq $.VTXID .ID}}selected{{end}}>{{.Label}}</option>{{end}}
          </select>
        </td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">Motors</td><td><div class="drone-motor-row" style="display:flex;gap:8px;align-items:center">
          <select name="motor_id" onchange="saveDrone()">
            <option value="">— none —</option>
            {{range .Motors}}<option value="{{.ID}}" {{if eq $.MotorID .ID}}selected{{end}}>{{.Label}}</option>{{end}}
          </select>
          <input type="number" name="motor_count" value="{{if .MotorCount}}{{.MotorCount}}{{else}}4{{end}}" min="1" onchange="saveDrone()" style="width:60px" title="Motor count">
        </div></td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">GPS</td><td>
          <select name="gps_id" onchange="saveDrone()">
            <option value="">— none —</option>
            {{range .GPSs}}<option value="{{.ID}}" {{if eq $.GPSID .ID}}selected{{end}}>{{.Label}}</option>{{end}}
          </select>
        </td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap">RX</td><td>
          <select name="rx_id" onchange="saveDrone()">
            <option value="">— none —</option>
            {{range .RXs}}<option value="{{.ID}}" {{if eq $.RXID .ID}}selected{{end}}>{{.Label}}</option>{{end}}
          </select>
        </td></tr>
        <tr><td style="padding:5px 12px 5px 0;color:#8b949e;white-space:nowrap;vertical-align:top;padding-top:8px">Notes</td><td>
          <textarea name="notes" rows="4" style="width:100%;resize:vertical" onchange="saveDrone()">{{.Notes}}</textarea>
        </td></tr>
      </table>
    </div>
  </div>
</div>
</form>

<div id="lightbox" style="display:none;position:fixed;inset:0;background:rgba(0,0,0,.85);z-index:9999;align-items:center;justify-content:center" onclick="closeLightbox()">
  <img id="lightbox-img" src="" style="max-width:90vw;max-height:90vh;border-radius:8px;object-fit:contain;box-shadow:0 8px 40px rgba(0,0,0,.6)">
</div>
<script>
(function(){
  var show=localStorage.getItem('showDroneDetail')==='1';
  function apply(){
    var s=document.getElementById('drone-detail-section');
    var b=document.getElementById('toggle-detail');
    if(s)s.style.display=show?'':'none';
    if(b)b.textContent=show?'Hide drone detail':'Show drone detail';
  }
  window.toggleDroneDetail=function(){show=!show;localStorage.setItem('showDroneDetail',show?'1':'0');apply();};
  apply();
})();
var _saveDroneTimer = null;
function saveDrone() {
  clearTimeout(_saveDroneTimer);
  _saveDroneTimer = setTimeout(function() {
    var form = document.getElementById('drone-form');
    var body = new URLSearchParams(new FormData(form));
    body.set('name', document.getElementById('drone-name').value);
    fetch(form.action, {method:'POST', body:body}).catch(function(e) { console.error('save failed', e); });
  }, 300);
}
function saveBatteries() {
  var checks = document.querySelectorAll('#bat-checks input[type=checkbox]');
  var body = new URLSearchParams();
  checks.forEach(function(cb) { if (cb.checked) body.append('battery_ids', cb.value); });
  fetch('/drones/{{.ID}}/batteries', {method:'POST', body:body}).catch(function(e) { console.error('save failed', e); });
}
function filterFrames() {
  var sizeSelect = document.querySelector('select[name="size_id"]');
  var frameSelect = document.querySelector('select[name="frame_id"]');
  if (!sizeSelect || !frameSelect) return;
  var sizeVal = sizeSelect.value;
  Array.prototype.forEach.call(frameSelect.options, function(opt) {
    if (!opt.value) return;
    var frameSizeId = opt.getAttribute('data-size-id') || '0';
    var hide = sizeVal && frameSizeId !== '0' && frameSizeId !== sizeVal;
    opt.hidden = hide;
    if (hide && opt.selected) { opt.selected = false; frameSelect.value = ''; }
  });
}
function openLightbox(src){var lb=document.getElementById('lightbox');document.getElementById('lightbox-img').src=src;lb.style.display='flex';}
function closeLightbox(){document.getElementById('lightbox').style.display='none';}
document.addEventListener('keydown',function(e){if(e.key==='Escape')closeLightbox();});
filterFrames();
</script>

{{if .Photos}}
<div class="section">
<h3 style="margin-bottom:12px">Photos</h3>
<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(180px,1fr));gap:16px;max-width:900px;margin-bottom:16px">
{{range .Photos}}
<div style="background:#161b22;border:1px solid #30363d;border-radius:6px;padding:10px">
  <img src="/drone-photos/{{.ID}}" style="width:100%;border-radius:4px;display:block;max-height:160px;object-fit:cover;cursor:zoom-in" onclick="openLightbox('/drone-photos/{{.ID}}')">
  <form method="POST" action="/drone-photos/{{.ID}}/delete" style="margin-top:6px">
    <button class="btn btn-sm btn-danger" type="submit">Delete</button>
  </form>
</div>
{{end}}
</div>
{{end}}
<div class="section" style="padding-top:0">
<form method="POST" action="/drones/{{.ID}}/photos" enctype="multipart/form-data" class="upload-form">
  <label class="btn btn-primary">Upload Photo<input type="file" name="photo" accept="image/*" style="display:none" onchange="this.closest('form').requestSubmit()"></label>
</form>
</div>

<div class="section">
  <h3 style="margin-bottom:12px">Add Log Entry</h3>
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
      if(el&&!el.value){var n=new Date();n.setMinutes(n.getMinutes()-n.getTimezoneOffset());el.value=n.toISOString().slice(0,16);}
    })();
  </script>
</div>

<div class="section">
{{if .Timeline}}
<div style="display:flex;flex-direction:column;gap:10px;max-width:700px">
{{range .Timeline}}
{{if eq .Kind "session"}}
<div style="background:#161b22;border:1px solid #30363d;border-radius:6px;padding:12px 16px;display:flex;gap:12px;align-items:baseline">
  <div style="flex:0 0 auto;color:#8b949e;font-size:13px;white-space:nowrap">{{.Date}}</div>
  <div style="flex:0 0 auto"><span class="badge {{if eq .SessionType "crash"}}badge-danger{{else if eq .SessionType "maintenance"}}badge-warning{{else}}badge-ok{{end}}">{{.SessionType}}</span></div>
  <div style="flex:1;min-width:0">
    <a href="/log/{{.SessionID}}" style="color:#c9d1d9;text-decoration:none;font-weight:500">{{if .SessionTitle}}{{.SessionTitle}}{{else}}<span class="muted">Untitled session</span>{{end}}</a>
    {{if .Location}}<span class="muted" style="font-size:13px;margin-left:8px">@ {{.Location}}</span>{{end}}
  </div>
  {{if gt .DurationMin 0}}<div style="flex:0 0 auto;color:#8b949e;font-size:13px">{{.DurationMin}} min</div>{{end}}
</div>
{{else}}
<div style="background:#161b22;border:1px solid #30363d;border-radius:6px;padding:14px 16px">
  <div class="dle-entry-row" style="display:flex;gap:12px;align-items:flex-start">
    <div class="dle-view" style="display:flex;gap:12px;align-items:flex-start;flex:1;min-width:0">
      <div style="flex:0 0 auto;color:#8b949e;font-size:13px;padding-top:2px;white-space:nowrap">{{.LoggedAt}}</div>
      <div style="flex:1;white-space:pre-wrap;word-break:break-word;min-width:0">{{.Body}}</div>
    </div>
    <form class="dle-form" method="POST" action="/drone-log/{{.LogID}}/edit" style="display:none;flex:1;gap:10px;flex-direction:column;min-width:0">
      <div style="display:flex;gap:10px;align-items:flex-start;flex-wrap:wrap">
        <input type="datetime-local" name="logged_at" value="{{.LoggedAtInput}}" style="flex:0 0 auto;font-size:13px">
        <textarea name="body" rows="3" style="flex:1;min-width:200px;resize:vertical;font-size:14px">{{.Body}}</textarea>
      </div>
    </form>
    <div class="dle-entry-btns" style="display:flex;gap:6px;flex-shrink:0">
      <button class="btn btn-sm btn-edit dle-edit-btn" onclick="dleEdit(this)">Edit</button>
      <form method="POST" action="/drone-log/{{.LogID}}/delete" style="display:inline">
        <button class="btn btn-sm btn-danger" type="submit">Delete</button>
      </form>
    </div>
  </div>
</div>
{{end}}
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
    btn.onclick = function() {
      var body = new URLSearchParams(new FormData(form));
      fetch(form.action, {method:'POST', body:body}).then(function() {
        var ta = form.querySelector('textarea');
        view.querySelector('div:last-child').textContent = ta ? ta.value : '';
        view.style.display = 'flex';
        form.style.display = 'none';
        btn.textContent = 'Edit';
        btn.onclick = null;
      });
    };
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
    <div class="form-group" style="justify-content:flex-end;padding-bottom:6px">
      <label style="display:flex;align-items:center;gap:8px;cursor:pointer">
        <input type="checkbox" name="sub_250g" {{if .Sub250g}}checked{{end}}>
        Sub 250g
      </label>
    </div>
  </div>
  <div class="form-row">
    <div class="form-group">
      <label>Frame</label>
      <select name="frame_id">
        <option value="">— none —</option>
        {{range .Frames}}
        <option value="{{.ID}}" data-size-id="{{.SizeID}}" {{if eq $.FrameID .ID}}selected{{end}}>{{.Label}}</option>
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
  <label class="btn btn-primary">Upload Photo<input type="file" name="photo" accept="image/*" style="display:none" onchange="this.closest('form').requestSubmit()"></label>
</form>
{{end}}
</div>
<script>
(function() {
  var sizeSelect = document.querySelector('select[name="size_id"]');
  var frameSelect = document.querySelector('select[name="frame_id"]');
  if (!sizeSelect || !frameSelect) return;
  function filterFrames() {
    var sizeVal = sizeSelect.value;
    Array.prototype.forEach.call(frameSelect.options, function(opt) {
      if (!opt.value) return;
      var frameSizeId = opt.getAttribute('data-size-id') || '0';
      var hide = sizeVal && frameSizeId !== '0' && frameSizeId !== sizeVal;
      opt.hidden = hide;
      if (hide && opt.selected) { opt.selected = false; frameSelect.value = ''; }
    });
  }
  sizeSelect.addEventListener('change', filterFrames);
  filterFrames();
})();
</script>
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
  <tr style="cursor:pointer" onclick="window.location='/frames/{{.ID}}/edit'">
    <td style="width:40px;padding:4px 6px" onclick="event.stopPropagation()">{{if .FirstPhotoID}}<img src="/frame-photos/{{.FirstPhotoID}}" style="width:36px;height:36px;object-fit:cover;border-radius:3px;display:block;cursor:zoom-in" onclick="event.stopPropagation();openLightbox('/frame-photos/{{.FirstPhotoID}}')">{{else}}<svg width="36" height="36" viewBox="0 0 36 36" style="border-radius:3px;background:#21262d;display:block"><g stroke="#8b949e" stroke-linecap="round"><line x1="8" y1="8" x2="28" y2="28" stroke-width="1.5"/><line x1="28" y1="8" x2="8" y2="28" stroke-width="1.5"/><circle cx="8" cy="8" r="3.5" fill="#8b949e"/><circle cx="28" cy="8" r="3.5" fill="#8b949e"/><circle cx="8" cy="28" r="3.5" fill="#8b949e"/><circle cx="28" cy="28" r="3.5" fill="#8b949e"/><circle cx="18" cy="18" r="4" fill="none" stroke-width="1.5"/></g></svg>{{end}}</td>
    <td class="muted">{{dash .Brand}}</td>
    <td>{{.Name}}</td>
    <td class="muted">{{if .SizeInch}}{{.SizeInch}}"{{else}}—{{end}}</td>
    <td class="muted">{{.Total}}</td>
    <td class="muted">{{.Installed}}</td>
    <td>{{if gt .Available 0}}<span style="color:#3fb950;font-weight:500">{{.Available}}</span>{{else}}<span class="muted">0</span>{{end}}</td>
    <td>{{if .InstalledOn}}<span class="installed-badge">{{.InstalledOn}}</span>{{else}}<span class="muted">—</span>{{end}}</td>
    <td class="actions-cell" onclick="event.stopPropagation()">
      <form class="inline" method="POST" action="/frames/{{.ID}}/adjust">
        <input type="number" name="count" placeholder="±" style="width:46px;padding:2px 4px;vertical-align:middle">
        <button class="btn btn-sm btn-edit" type="submit">Apply</button>
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
  <tr style="cursor:pointer" onclick="window.location='/fcs/{{.ID}}/edit'">
    <td style="width:40px;padding:4px 6px" onclick="event.stopPropagation()">{{if .FirstPhotoID}}<img src="/fc-photos/{{.FirstPhotoID}}" style="width:36px;height:36px;object-fit:cover;border-radius:3px;display:block;cursor:zoom-in" onclick="event.stopPropagation();openLightbox('/fc-photos/{{.FirstPhotoID}}')">{{else}}<svg width="36" height="36" viewBox="0 0 36 36" style="border-radius:3px;background:#21262d;display:block"><g stroke="#8b949e"><rect x="10" y="10" width="16" height="16" rx="2" stroke-width="1.5" fill="none"/><rect x="14" y="14" width="8" height="8" rx="1" fill="#8b949e" opacity="0.5"/><line x1="14" y1="10" x2="14" y2="7" stroke-width="1.5" stroke-linecap="round"/><line x1="18" y1="10" x2="18" y2="7" stroke-width="1.5" stroke-linecap="round"/><line x1="22" y1="10" x2="22" y2="7" stroke-width="1.5" stroke-linecap="round"/><line x1="14" y1="26" x2="14" y2="29" stroke-width="1.5" stroke-linecap="round"/><line x1="18" y1="26" x2="18" y2="29" stroke-width="1.5" stroke-linecap="round"/><line x1="22" y1="26" x2="22" y2="29" stroke-width="1.5" stroke-linecap="round"/></g></svg>{{end}}</td>
    <td class="muted">{{dash .Brand}}</td>
    <td>{{.Name}}</td>
    <td class="muted">{{dash .MCU}}</td>
    <td class="muted">{{dash .Firmware}}</td>
    <td class="muted">{{.Total}}</td>
    <td class="muted">{{.Installed}}</td>
    <td>{{if gt .Available 0}}<span style="color:#3fb950;font-weight:500">{{.Available}}</span>{{else}}<span class="muted">0</span>{{end}}</td>
    <td>{{if .InstalledOn}}<span class="installed-badge">{{.InstalledOn}}</span>{{else}}<span class="muted">—</span>{{end}}</td>
    <td class="actions-cell" onclick="event.stopPropagation()">
      <form class="inline" method="POST" action="/fcs/{{.ID}}/adjust">
        <input type="number" name="count" placeholder="±" style="width:46px;padding:2px 4px;vertical-align:middle">
        <button class="btn btn-sm btn-edit" type="submit">Apply</button>
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
  <tr style="cursor:pointer" onclick="window.location='/escs/{{.ID}}/edit'">
    <td style="width:40px;padding:4px 6px" onclick="event.stopPropagation()">{{if .FirstPhotoID}}<img src="/esc-photos/{{.FirstPhotoID}}" style="width:36px;height:36px;object-fit:cover;border-radius:3px;display:block;cursor:zoom-in" onclick="event.stopPropagation();openLightbox('/esc-photos/{{.FirstPhotoID}}')">{{else}}<svg width="36" height="36" viewBox="0 0 36 36" style="border-radius:3px;background:#21262d;display:block"><g stroke="#8b949e"><rect x="5" y="12" width="26" height="12" rx="2" stroke-width="1.5" fill="none"/><line x1="11" y1="12" x2="11" y2="24" stroke-width="1" opacity="0.5"/><line x1="17" y1="12" x2="17" y2="24" stroke-width="1" opacity="0.5"/><line x1="23" y1="12" x2="23" y2="24" stroke-width="1" opacity="0.5"/><line x1="5" y1="18" x2="31" y2="18" stroke-width="1" opacity="0.3"/></g></svg>{{end}}</td>
    <td class="muted">{{dash .Brand}}</td>
    <td>{{.Name}}</td>
    <td class="muted">{{if .CurrentRating}}{{.CurrentRating}}A{{else}}—{{end}}</td>
    <td class="muted">{{if .CellMax}}{{.CellMax}}{{else}}—{{end}}</td>
    <td class="muted">{{.Total}}</td>
    <td class="muted">{{.Installed}}</td>
    <td>{{if gt .Available 0}}<span style="color:#3fb950;font-weight:500">{{.Available}}</span>{{else}}<span class="muted">0</span>{{end}}</td>
    <td>{{if .InstalledOn}}<span class="installed-badge">{{.InstalledOn}}</span>{{else}}<span class="muted">—</span>{{end}}</td>
    <td class="actions-cell" onclick="event.stopPropagation()">
      <form class="inline" method="POST" action="/escs/{{.ID}}/adjust">
        <input type="number" name="count" placeholder="±" style="width:46px;padding:2px 4px;vertical-align:middle">
        <button class="btn btn-sm btn-edit" type="submit">Apply</button>
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
  <tr style="cursor:pointer" onclick="window.location='/motors/{{.ID}}/edit'">
    <td style="width:40px;padding:4px 6px" onclick="event.stopPropagation()">{{if .FirstPhotoID}}<img src="/motor-photos/{{.FirstPhotoID}}" style="width:36px;height:36px;object-fit:cover;border-radius:3px;display:block;cursor:zoom-in" onclick="event.stopPropagation();openLightbox('/motor-photos/{{.FirstPhotoID}}')">{{else}}<svg width="36" height="36" viewBox="0 0 36 36" style="border-radius:3px;background:#21262d;display:block"><g stroke="#8b949e" stroke-width="1.5" fill="none"><circle cx="18" cy="18" r="12"/><circle cx="18" cy="18" r="6"/><circle cx="18" cy="18" r="2" fill="#8b949e" stroke="none"/></g></svg>{{end}}</td>
    <td class="muted">{{dash .Brand}}</td>
    <td>{{.Name}}</td>
    <td class="muted">{{dash .StatorSize}}</td>
    <td class="muted">{{if .KV}}{{.KV}}kv{{else}}—{{end}}</td>
    <td class="muted">{{.Total}}</td>
    <td class="muted">{{.Installed}}</td>
    <td>{{if gt .Available 0}}<span style="color:#3fb950;font-weight:500">{{.Available}}</span>{{else}}<span class="muted">0</span>{{end}}</td>
    <td>{{if .InstalledOn}}<span class="installed-badge">{{.InstalledOn}}</span>{{else}}<span class="muted">—</span>{{end}}</td>
    <td class="actions-cell" onclick="event.stopPropagation()">
      <form class="inline" method="POST" action="/motors/{{.ID}}/adjust">
        <input type="number" name="count" placeholder="±" style="width:46px;padding:2px 4px;vertical-align:middle">
        <button class="btn btn-sm btn-edit" type="submit">Apply</button>
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
  <thead><tr><th></th><th>Brand</th><th>Name</th><th>Owned</th><th>Installed</th><th>Avail.</th><th>Installed On</th><th></th></tr></thead>
  <tbody>
  {{range .VTXs}}
  <tr style="cursor:pointer" onclick="window.location='/vtx/{{.ID}}/edit'">
    <td style="width:40px;padding:4px 6px" onclick="event.stopPropagation()">{{if .FirstPhotoID}}<img src="/vtx-photos/{{.FirstPhotoID}}" style="width:36px;height:36px;object-fit:cover;border-radius:3px;display:block;cursor:zoom-in" onclick="event.stopPropagation();openLightbox('/vtx-photos/{{.FirstPhotoID}}')">{{else}}<svg width="36" height="36" viewBox="0 0 36 36" style="border-radius:3px;background:#21262d;display:block"><g stroke="#8b949e" stroke-linecap="round" fill="none"><line x1="18" y1="10" x2="18" y2="27" stroke-width="2"/><circle cx="18" cy="27" r="2.5" fill="#8b949e" stroke="none"/><path d="M12 17 Q18 13 24 17" stroke-width="1.5"/><path d="M8 12 Q18 7 28 12" stroke-width="1.5"/></g></svg>{{end}}</td>
    <td class="muted">{{dash .Brand}}</td>
    <td>{{.Name}}</td>
    <td class="muted">{{.Total}}</td>
    <td class="muted">{{.Installed}}</td>
    <td>{{if gt .Available 0}}<span style="color:#3fb950;font-weight:500">{{.Available}}</span>{{else}}<span class="muted">0</span>{{end}}</td>
    <td>{{if .InstalledOn}}<span class="installed-badge">{{.InstalledOn}}</span>{{else}}<span class="muted">—</span>{{end}}</td>
    <td class="actions-cell" onclick="event.stopPropagation()">
      <form class="inline" method="POST" action="/vtx/{{.ID}}/adjust">
        <input type="number" name="count" placeholder="±" style="width:46px;padding:2px 4px;vertical-align:middle">
        <button class="btn btn-sm btn-edit" type="submit">Apply</button>
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
  <tr style="cursor:pointer" onclick="window.location='/gps/{{.ID}}/edit'">
    <td style="width:40px;padding:4px 6px" onclick="event.stopPropagation()">{{if .FirstPhotoID}}<img src="/gps-photos/{{.FirstPhotoID}}" style="width:36px;height:36px;object-fit:cover;border-radius:3px;display:block;cursor:zoom-in" onclick="event.stopPropagation();openLightbox('/gps-photos/{{.FirstPhotoID}}')">{{else}}<svg width="36" height="36" viewBox="0 0 36 36" style="border-radius:3px;background:#21262d;display:block"><g stroke="#8b949e" stroke-width="1.5"><path d="M18 6C12.5 6 8 10.5 8 16C8 22.5 18 30 18 30C18 30 28 22.5 28 16C28 10.5 23.5 6 18 6Z" fill="none"/><circle cx="18" cy="16" r="3.5" fill="#8b949e" stroke="none"/></g></svg>{{end}}</td>
    <td class="muted">{{dash .Brand}}</td>
    <td>{{.Name}}</td>
    <td class="muted">{{.Total}}</td>
    <td class="muted">{{.Installed}}</td>
    <td>{{if gt .Available 0}}<span style="color:#3fb950;font-weight:500">{{.Available}}</span>{{else}}<span class="muted">0</span>{{end}}</td>
    <td>{{if .InstalledOn}}<span class="installed-badge">{{.InstalledOn}}</span>{{else}}<span class="muted">—</span>{{end}}</td>
    <td class="actions-cell" onclick="event.stopPropagation()">
      <form class="inline" method="POST" action="/gps/{{.ID}}/adjust">
        <input type="number" name="count" placeholder="±" style="width:46px;padding:2px 4px;vertical-align:middle">
        <button class="btn btn-sm btn-edit" type="submit">Apply</button>
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
  <tr style="cursor:pointer" onclick="window.location='/rx/{{.ID}}/edit'">
    <td style="width:40px;padding:4px 6px" onclick="event.stopPropagation()">{{if .FirstPhotoID}}<img src="/rx-photos/{{.FirstPhotoID}}" style="width:36px;height:36px;object-fit:cover;border-radius:3px;display:block;cursor:zoom-in" onclick="event.stopPropagation();openLightbox('/rx-photos/{{.FirstPhotoID}}')">{{else}}<svg width="36" height="36" viewBox="0 0 36 36" style="border-radius:3px;background:#21262d;display:block"><g stroke="#8b949e" stroke-linecap="round" fill="none"><line x1="18" y1="13" x2="18" y2="29" stroke-width="2"/><circle cx="18" cy="9.5" r="3" fill="#8b949e" stroke="none"/><path d="M11 18 Q18 22 25 18" stroke-width="1.5"/><path d="M8 14 Q18 19 28 14" stroke-width="1.5" opacity="0.5"/></g></svg>{{end}}</td>
    <td class="muted">{{dash .Brand}}</td>
    <td>{{.Name}}</td>
    <td class="muted">{{dash .Protocol}}</td>
    <td class="muted">{{.Total}}</td>
    <td class="muted">{{.Installed}}</td>
    <td>{{if gt .Available 0}}<span style="color:#3fb950;font-weight:500">{{.Available}}</span>{{else}}<span class="muted">0</span>{{end}}</td>
    <td>{{if .InstalledOn}}<span class="installed-badge">{{.InstalledOn}}</span>{{else}}<span class="muted">—</span>{{end}}</td>
    <td class="actions-cell" onclick="event.stopPropagation()">
      <form class="inline" method="POST" action="/rx/{{.ID}}/adjust">
        <input type="number" name="count" placeholder="±" style="width:46px;padding:2px 4px;vertical-align:middle">
        <button class="btn btn-sm btn-edit" type="submit">Apply</button>
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
    {{if .ID}}<form class="inline" method="POST" action="/frames/{{.ID}}/delete"><button class="btn btn-danger" type="submit">Delete</button></form>{{end}}
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
  <label class="btn btn-primary">Upload Photo<input type="file" name="photo" accept="image/*" style="display:none" onchange="this.closest('form').requestSubmit()"></label>
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
      <select name="mcu_id">
        <option value="">— none —</option>
        {{range .MCUs}}<option value="{{.ID}}" {{if eq $.MCUID .ID}}selected{{end}}>{{.Label}}</option>{{end}}
      </select>
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
    {{if .ID}}<form class="inline" method="POST" action="/fcs/{{.ID}}/delete"><button class="btn btn-danger" type="submit">Delete</button></form>{{end}}
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
  <label class="btn btn-primary">Upload Photo<input type="file" name="photo" accept="image/*" style="display:none" onchange="this.closest('form').requestSubmit()"></label>
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
      <label>Max Cell</label>
      <select name="cell_max_id">
        <option value="">— none —</option>
        {{range .Cells}}<option value="{{.ID}}" {{if eq $.CellMaxID .ID}}selected{{end}}>{{.Label}}</option>{{end}}
      </select>
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
    {{if .ID}}<form class="inline" method="POST" action="/escs/{{.ID}}/delete"><button class="btn btn-danger" type="submit">Delete</button></form>{{end}}
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
  <label class="btn btn-primary">Upload Photo<input type="file" name="photo" accept="image/*" style="display:none" onchange="this.closest('form').requestSubmit()"></label>
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
    {{if .ID}}<form class="inline" method="POST" action="/motors/{{.ID}}/delete"><button class="btn btn-danger" type="submit">Delete</button></form>{{end}}
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
  <label class="btn btn-primary">Upload Photo<input type="file" name="photo" accept="image/*" style="display:none" onchange="this.closest('form').requestSubmit()"></label>
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
    {{if .ID}}<form class="inline" method="POST" action="/vtx/{{.ID}}/delete"><button class="btn btn-danger" type="submit">Delete</button></form>{{end}}
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
  <label class="btn btn-primary">Upload Photo<input type="file" name="photo" accept="image/*" style="display:none" onchange="this.closest('form').requestSubmit()"></label>
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
    {{if .ID}}<form class="inline" method="POST" action="/gps/{{.ID}}/delete"><button class="btn btn-danger" type="submit">Delete</button></form>{{end}}
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
  <label class="btn btn-primary">Upload Photo<input type="file" name="photo" accept="image/*" style="display:none" onchange="this.closest('form').requestSubmit()"></label>
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
      <select name="protocol_id">
        <option value="">— none —</option>
        {{range .Protocols}}<option value="{{.ID}}" {{if eq $.ProtocolID .ID}}selected{{end}}>{{.Label}}</option>{{end}}
      </select>
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
    {{if .ID}}<form class="inline" method="POST" action="/rx/{{.ID}}/delete"><button class="btn btn-danger" type="submit">Delete</button></form>{{end}}
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
  <label class="btn btn-primary">Upload Photo<input type="file" name="photo" accept="image/*" style="display:none" onchange="this.closest('form').requestSubmit()"></label>
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
<tr style="cursor:pointer" onclick="window.location='/batteries/{{.ID}}/edit'">
  <td style="width:40px;padding:4px 6px" onclick="event.stopPropagation()">{{if .FirstPhotoID}}<img src="/battery-photos/{{.FirstPhotoID}}" style="width:36px;height:36px;object-fit:cover;border-radius:3px;display:block;cursor:zoom-in" onclick="event.stopPropagation();openLightbox('/battery-photos/{{.FirstPhotoID}}')">{{else}}<svg width="36" height="36" viewBox="0 0 36 36" style="border-radius:3px;background:#21262d;display:block"><g stroke="#8b949e"><rect x="4" y="12" width="24" height="12" rx="2" stroke-width="1.5" fill="none"/><rect x="6" y="15" width="10" height="6" rx="1" fill="#8b949e" opacity="0.5" stroke="none"/><line x1="28" y1="15" x2="28" y2="21" stroke-width="2" stroke-linecap="round"/><line x1="31" y1="16.5" x2="31" y2="19.5" stroke-width="1.5" stroke-linecap="round" opacity="0.5"/></g></svg>{{end}}</td>
  <td class="muted">{{dash .Brand}}</td>
  <td><strong>{{.Name}}</strong></td>
  <td class="muted">{{dash .CellLabel}}</td>
  <td class="muted">{{.CapacityMAh}}</td>
  <td class="muted">{{if .WeightG}}{{.WeightG}}g{{else}}—{{end}}</td>
  <td class="muted">{{.Total}}</td>
  <td>{{if .AssignedTo}}<span class="installed-badge">{{.AssignedTo}}</span>{{else}}<span class="muted">—</span>{{end}}</td>
  <td class="actions-cell" onclick="event.stopPropagation()">
    <form class="inline" method="POST" action="/batteries/{{.ID}}/adjust">
      <input type="number" name="count" placeholder="±" style="width:46px;padding:2px 4px;vertical-align:middle">
      <button class="btn btn-sm btn-edit" type="submit">Apply</button>
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
    {{if .ID}}<form class="inline" method="POST" action="/batteries/{{.ID}}/delete"><button class="btn btn-danger" type="submit">Delete</button></form>{{end}}
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
  <label class="btn btn-primary">Upload Photo<input type="file" name="photo" accept="image/*" style="display:none" onchange="this.closest('form').requestSubmit()"></label>
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
  <th>Qty</th><th>Reorder At</th><th>Drone</th>
</tr></thead>
<tbody>
{{range .Propellers}}
<tr class="{{if .LowStock}}low-stock{{end}}" style="cursor:pointer" onclick="window.location='/props/{{.ID}}/edit'">
  <td style="width:40px;padding:4px 6px" onclick="event.stopPropagation()">{{if .FirstPhotoID}}<img src="/prop-photos/{{.FirstPhotoID}}" style="width:36px;height:36px;object-fit:cover;border-radius:3px;display:block;cursor:zoom-in" onclick="event.stopPropagation();openLightbox('/prop-photos/{{.FirstPhotoID}}')">{{else}}<svg width="36" height="36" viewBox="0 0 36 36" style="border-radius:3px;background:#21262d;display:block"><g fill="#8b949e" opacity="0.7"><ellipse cx="13" cy="13" rx="8" ry="4" transform="rotate(-45 13 13)"/><ellipse cx="23" cy="23" rx="8" ry="4" transform="rotate(-45 23 23)"/></g><circle cx="18" cy="18" r="3" fill="#21262d" stroke="#8b949e" stroke-width="1.5"/></svg>{{end}}</td>
  <td class="muted">{{dash .Brand}}</td>
  <td>{{.Name}}</td>
  <td class="muted">{{if .SizeInch}}{{.SizeInch}}"{{else}}—{{end}}</td>
  <td class="muted">{{dash .Pitch}}</td>
  <td class="muted">{{.BladeCount}}</td>
  <td>{{.Quantity}}{{if .LowStock}} <span class="muted" title="low stock">&#9888;</span>{{end}}</td>
  <td class="muted">{{.ReorderThreshold}}</td>
  <td class="muted">{{dash .DroneNames}}</td>
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
    {{if .ID}}<form class="inline" method="POST" action="/props/{{.ID}}/delete"><button class="btn btn-danger" type="submit">Delete</button></form>{{end}}
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
  <label class="btn btn-primary">Upload Photo<input type="file" name="photo" accept="image/*" style="display:none" onchange="this.closest('form').requestSubmit()"></label>
</form>
{{end}}
</div>
{{end}}`

const logListTmpl = `{{define "content"}}
<div class="page-header">
  <div class="page-header-left"><h2>Flight Log</h2></div>
  <div style="display:flex;gap:8px">
    <button class="btn btn-cancel" id="toggle-quick" onclick="toggleQuickFlights()">Show quick flights</button>
    <a href="/log/new" class="btn btn-primary">+ Log Session</a>
  </div>
</div>
{{if .Sessions}}
<div class="table-wrap">
<table>
<thead><tr>
  <th>Date</th><th>Title / Notes</th><th>Drone</th><th>Type</th><th>Duration</th><th>Location</th><th>Batteries</th><th></th>
</tr></thead>
<tbody>
{{range .Sessions}}
<tr {{if .IsQuickFlight}}class="quick-flight-row"{{end}} style="cursor:pointer" onclick="window.location='/log/{{.ID}}'">
  <td class="muted" style="white-space:nowrap">{{.SessionDate}}</td>
  <td style="max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{{if .Title}}<strong>{{.Title}}</strong>{{else}}<span class="muted">{{dash .Notes}}</span>{{end}}</td>
  <td><strong>{{.DroneNames}}</strong></td>
  <td><span class="badge {{badgeClass .Type}}">{{.Type}}</span></td>
  <td class="muted">{{if gt .DurationMin 0}}{{.DurationMin}}m{{else}}—{{end}}</td>
  <td class="muted">{{dash .Location}}</td>
  <td class="muted" style="font-size:12px">{{dash .BatteryList}}</td>
  <td class="actions-cell" onclick="event.stopPropagation()">
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
<script>
(function(){
  var show = localStorage.getItem('showQuickFlights')==='1';
  function apply(){
    var rows=document.querySelectorAll('tr.quick-flight-row');
    rows.forEach(function(r){r.style.display=show?'':'none';});
    var btn=document.getElementById('toggle-quick');
    if(btn)btn.textContent=show?'Hide quick flights':'Show quick flights';
  }
  window.toggleQuickFlights=function(){show=!show;localStorage.setItem('showQuickFlights',show?'1':'0');apply();};
  apply();
})();
</script>
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
  {{if $.Drones}}
  <div style="margin-top:8px">
    {{$vid := .}}
    <select name="drone_id" style="font-size:13px;max-width:260px" onchange="saveDrone({{.ID}},this.value)">
      <option value="">— no drone —</option>
      {{range $.Drones}}<option value="{{.ID}}" {{if eq .ID $vid.DroneID}}selected{{end}}>{{.Label}}</option>{{end}}
    </select>
  </div>
  {{end}}
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
  <label class="btn btn-primary">Upload Video<input type="file" name="video" accept="video/*" style="display:none" onchange="this.closest('form').requestSubmit()"></label>
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
  <label class="btn btn-primary">Upload Photo<input type="file" name="photo" accept="image/*" style="display:none" onchange="this.closest('form').requestSubmit()"></label>
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
    btn.onclick = function() {
      var body = new URLSearchParams(new FormData(form));
      fetch(form.action, {method:'POST', body:body}).then(function() {
        var ta = form.querySelector('textarea');
        var newText = ta ? ta.value : '';
        view.innerHTML = newText ? newText : '<span style="opacity:.5">No note</span>';
        view.style.display = '';
        form.style.display = 'none';
        btn.textContent = 'Edit';
        btn.onclick = null;
      });
    };
  }
}
function saveDrone(videoID, droneID) {
  var body = new URLSearchParams();
  body.append('drone_id', droneID);
  fetch('/videos/'+videoID+'/drone', {method:'POST', body:body});
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
  <th>Name</th><th>Type</th><th>Address</th><th>Notes</th>
</tr></thead>
<tbody>
{{range .Places}}
<tr style="cursor:pointer" onclick="window.location='/places/{{.ID}}/edit'">
  <td><strong>{{.Name}}</strong></td>
  <td class="muted">{{.PlaceType}}</td>
  <td class="muted">{{dash .Address}}</td>
  <td class="muted" style="max-width:220px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{{dash .Notes}}</td>
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
    {{if .ID}}<form class="inline" method="POST" action="/places/{{.ID}}/delete"><button class="btn btn-danger" type="submit">Delete</button></form>{{end}}
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
  <div class="section-header"><h3>Transcode</h3></div>
  <form id="config-form" action="/settings/config" method="POST" style="max-width:480px">
    <table style="border-collapse:collapse;width:100%">
      <tr><td style="padding:6px 12px 6px 0;color:#8b949e;white-space:nowrap;width:140px">CRF (0–51)</td><td>
        <input type="number" name="ffmpeg_crf" value="{{.Config.FFmpegCRF}}" min="0" max="51" style="width:80px" onchange="saveConfig()">
        <span class="muted" style="font-size:12px;margin-left:8px">lower = better quality</span>
      </td></tr>
      <tr><td style="padding:6px 12px 6px 0;color:#8b949e;white-space:nowrap">Preset</td><td>
        <select name="ffmpeg_preset" onchange="saveConfig()">
          <option value="ultrafast" {{if eq .Config.FFmpegPreset "ultrafast"}}selected{{end}}>ultrafast</option>
          <option value="superfast" {{if eq .Config.FFmpegPreset "superfast"}}selected{{end}}>superfast</option>
          <option value="veryfast"  {{if eq .Config.FFmpegPreset "veryfast"}}selected{{end}}>veryfast</option>
          <option value="faster"    {{if eq .Config.FFmpegPreset "faster"}}selected{{end}}>faster</option>
          <option value="fast"      {{if eq .Config.FFmpegPreset "fast"}}selected{{end}}>fast</option>
          <option value="medium"    {{if eq .Config.FFmpegPreset "medium"}}selected{{end}}>medium</option>
          <option value="slow"      {{if eq .Config.FFmpegPreset "slow"}}selected{{end}}>slow</option>
          <option value="slower"    {{if eq .Config.FFmpegPreset "slower"}}selected{{end}}>slower</option>
          <option value="veryslow"  {{if eq .Config.FFmpegPreset "veryslow"}}selected{{end}}>veryslow</option>
        </select>
      </td></tr>
      <tr><td style="padding:6px 12px 6px 0;color:#8b949e;white-space:nowrap">Max width (px)</td><td>
        <input type="number" name="video_max_width" value="{{.Config.VideoMaxWidth}}" min="480" step="16" style="width:100px" onchange="saveConfig()">
      </td></tr>
      <tr><td style="padding:6px 12px 6px 0;color:#8b949e;white-space:nowrap">Bitrate (kbps)</td><td>
        <input type="number" name="video_bitrate_k" value="{{.Config.VideoBitrateK}}" min="500" step="100" style="width:100px" onchange="saveConfig()">
      </td></tr>
      <tr><td style="padding:6px 12px 6px 0;color:#8b949e;white-space:nowrap">HLS segment (s)</td><td>
        <input type="number" name="hls_segment_sec" value="{{.Config.HLSSegmentSec}}" min="2" max="30" style="width:80px" onchange="saveConfig()">
      </td></tr>
    </table>
  </form>
<script>
var _saveConfigTimer = null;
function saveConfig() {
  clearTimeout(_saveConfigTimer);
  _saveConfigTimer = setTimeout(function() {
    var form = document.getElementById('config-form');
    fetch(form.action, {method:'POST', body:new URLSearchParams(new FormData(form))}).catch(function(e){console.error('config save failed',e);});
  }, 300);
}
</script>
</div>

<div class="section">
  <div class="section-header">
    <h3>Brands</h3>
    <a href="/brands/new" class="btn btn-sm btn-primary">+ Add</a>
  </div>
  {{if .Brands}}
  <div class="table-wrap"><table>
  <thead><tr><th>Name</th></tr></thead>
  <tbody>
  {{range .Brands}}<tr style="cursor:pointer" onclick="window.location='/brands/{{.ID}}/edit'">
    <td><strong>{{.Name}}</strong></td>
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
  <thead><tr><th>Label</th></tr></thead>
  <tbody>
  {{range .Sizes}}<tr style="cursor:pointer" onclick="window.location='/sizes/{{.ID}}/edit'">
    <td><strong>{{.Label}}"</strong></td>
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
  <thead><tr><th>Label</th></tr></thead>
  <tbody>
  {{range .Cells}}<tr style="cursor:pointer" onclick="window.location='/cells/{{.ID}}/edit'">
    <td><strong>{{.Label}}</strong></td>
  </tr>{{end}}
  </tbody></table></div>
  {{else}}<p class="muted">No cells yet. <a href="/cells/new">Add one.</a></p>{{end}}
</div>

<div class="section">
  <div class="section-header">
    <h3>Radio Protocols</h3>
    <a href="/radio-protocols/new" class="btn btn-sm btn-primary">+ Add</a>
  </div>
  {{if .RadioProtocols}}
  <div class="table-wrap"><table>
  <thead><tr><th>Name</th></tr></thead>
  <tbody>
  {{range .RadioProtocols}}<tr style="cursor:pointer" onclick="window.location='/radio-protocols/{{.ID}}/edit'">
    <td><strong>{{.Name}}</strong></td>
  </tr>{{end}}
  </tbody></table></div>
  {{else}}<p class="muted">No radio protocols yet. <a href="/radio-protocols/new">Add one.</a></p>{{end}}
</div>

<div class="section">
  <div class="section-header">
    <h3>MCUs</h3>
    <a href="/mcus/new" class="btn btn-sm btn-primary">+ Add</a>
  </div>
  {{if .MCUs}}
  <div class="table-wrap"><table>
  <thead><tr><th>Name</th></tr></thead>
  <tbody>
  {{range .MCUs}}<tr style="cursor:pointer" onclick="window.location='/mcus/{{.ID}}/edit'">
    <td><strong>{{.Name}}</strong></td>
  </tr>{{end}}
  </tbody></table></div>
  {{else}}<p class="muted">No MCUs yet. <a href="/mcus/new">Add one.</a></p>{{end}}
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
    {{if .ID}}<form class="inline" method="POST" action="/brands/{{.ID}}/delete"><button class="btn btn-danger" type="submit">Delete</button></form>{{end}}
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
    {{if .ID}}<form class="inline" method="POST" action="/cells/{{.ID}}/delete"><button class="btn btn-danger" type="submit">Delete</button></form>{{end}}
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
    {{if .ID}}<form class="inline" method="POST" action="/sizes/{{.ID}}/delete"><button class="btn btn-danger" type="submit">Delete</button></form>{{end}}
  </div>
</form>
</div>
{{end}}`

const mcuFormTmpl = `{{define "content"}}
<div class="page-header">
  <h2>{{if .ID}}Edit MCU{{else}}New MCU{{end}}</h2>
</div>
<div class="form-page">
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
<form method="POST">
  <div class="form-group">
    <label>Name *</label>
    <input type="text" name="name" value="{{.Name}}" required autofocus placeholder="e.g. F405">
  </div>
  <div class="form-actions">
    <button class="btn btn-primary" type="submit">{{if .ID}}Save{{else}}Add MCU{{end}}</button>
    <a href="/settings" class="btn btn-cancel">Cancel</a>
    {{if .ID}}<form class="inline" method="POST" action="/mcus/{{.ID}}/delete"><button class="btn btn-danger" type="submit">Delete</button></form>{{end}}
  </div>
</form>
</div>
{{end}}`

const radioProtocolFormTmpl = `{{define "content"}}
<div class="page-header">
  <h2>{{if .ID}}Edit Radio Protocol{{else}}New Radio Protocol{{end}}</h2>
</div>
<div class="form-page">
{{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
<form method="POST">
  <div class="form-group">
    <label>Name *</label>
    <input type="text" name="name" value="{{.Name}}" required autofocus placeholder="e.g. ELRS 2.4GHz">
  </div>
  <div class="form-actions">
    <button class="btn btn-primary" type="submit">{{if .ID}}Save{{else}}Add Protocol{{end}}</button>
    <a href="/settings" class="btn btn-cancel">Cancel</a>
    {{if .ID}}<form class="inline" method="POST" action="/radio-protocols/{{.ID}}/delete"><button class="btn btn-danger" type="submit">Delete</button></form>{{end}}
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
