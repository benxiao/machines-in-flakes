package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const quickFlightLabel = "Quick flight"

type AppConfig struct {
	FFmpegCRF      int
	FFmpegPreset   string
	VideoMaxWidth  int
	VideoBitrateK  int
	HLSSegmentSec  int
}

func (a *App) loadConfig(ctx context.Context) AppConfig {
	cfg := AppConfig{FFmpegCRF: 23, FFmpegPreset: "fast", VideoMaxWidth: 1280, VideoBitrateK: 3000, HLSSegmentSec: 6}
	a.db.QueryRow(ctx, `SELECT ffmpeg_crf, ffmpeg_preset, video_max_width, video_bitrate_k, hls_segment_sec FROM app_config WHERE id=1`).
		Scan(&cfg.FFmpegCRF, &cfg.FFmpegPreset, &cfg.VideoMaxWidth, &cfg.VideoBitrateK, &cfg.HLSSegmentSec)
	return cfg
}

// ---- helpers ----

func (a *App) queryOptions(r *http.Request, sql string) ([]OptionItem, error) {
	rows, err := a.db.Query(r.Context(), sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var opts []OptionItem
	for rows.Next() {
		var o OptionItem
		if err := rows.Scan(&o.ID, &o.Label); err != nil {
			return nil, err
		}
		opts = append(opts, o)
	}
	return opts, rows.Err()
}

func (a *App) droneChecks(r *http.Request, sessionID int) ([]DroneCheck, error) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx, `
        SELECT d.id, d.name,
               EXISTS(SELECT 1 FROM session_drones sd WHERE sd.session_id=$1 AND sd.drone_id=d.id)
        FROM drones d ORDER BY d.name`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var checks []DroneCheck
	for rows.Next() {
		var c DroneCheck
		if err := rows.Scan(&c.ID, &c.Label, &c.Checked); err != nil {
			return nil, err
		}
		checks = append(checks, c)
	}
	return checks, rows.Err()
}

func (a *App) frameOptions(r *http.Request) ([]FrameOption, error) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx, `SELECT f.id, COALESCE(b.name||' ','')||f.name, COALESCE(f.size_id,0)
         FROM frames f LEFT JOIN brands b ON b.id=f.brand_id ORDER BY b.name, f.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FrameOption
	for rows.Next() {
		var fo FrameOption
		if err := rows.Scan(&fo.ID, &fo.Label, &fo.SizeID); err != nil {
			return nil, err
		}
		out = append(out, fo)
	}
	return out, nil
}

func (a *App) fcOptions(r *http.Request) ([]OptionItem, error) {
	return a.queryOptions(r, `SELECT fc.id, COALESCE(b.name||' ','')|| fc.name
         FROM flight_controllers fc LEFT JOIN brands b ON b.id=fc.brand_id ORDER BY b.name, fc.name`)
}

func (a *App) escOptions(r *http.Request) ([]OptionItem, error) {
	return a.queryOptions(r, `SELECT e.id, COALESCE(b.name||' ','')|| e.name
         FROM escs e LEFT JOIN brands b ON b.id=e.brand_id ORDER BY b.name, e.name`)
}

func (a *App) vtxOptions(r *http.Request) ([]OptionItem, error) {
	return a.queryOptions(r, `SELECT v.id, COALESCE(b.name||' ','')|| v.name
         FROM vtx_units v LEFT JOIN brands b ON b.id=v.brand_id ORDER BY b.name, v.name`)
}

func (a *App) dronePropChecks(r *http.Request, droneID int) ([]PropCheck, error) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx, `
        SELECT p.id,
               COALESCE(br.name||' ','')|| p.name||
               CASE WHEN sz.label IS NOT NULL THEN ' '||sz.label||'"' ELSE '' END,
               EXISTS(SELECT 1 FROM drone_props dp WHERE dp.drone_id=$1 AND dp.prop_id=p.id)
        FROM propellers p
        LEFT JOIN brands br ON br.id=p.brand_id
        LEFT JOIN sizes sz ON sz.id=p.size_id
        JOIN (SELECT size_id FROM drones WHERE id=$1) dd ON (p.size_id=dd.size_id OR dd.size_id IS NULL)
        ORDER BY br.name, p.name`, droneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var checks []PropCheck
	for rows.Next() {
		var c PropCheck
		if err := rows.Scan(&c.ID, &c.Label, &c.Checked); err != nil {
			return nil, err
		}
		checks = append(checks, c)
	}
	return checks, rows.Err()
}

func (a *App) droneBatteryChecks(r *http.Request, droneID int) ([]BatteryCheck, error) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx, `
        SELECT b.id,
               COALESCE(br.name||' ','')|| b.name||' ('||COALESCE(c.label,'?')||' '||b.capacity_mah||'mAh)',
               EXISTS(SELECT 1 FROM drone_batteries db WHERE db.drone_id=$1 AND db.battery_id=b.id)
        FROM batteries b
        LEFT JOIN brands br ON br.id=b.brand_id
        LEFT JOIN cells c ON c.id=b.cell_id
        JOIN (SELECT cell_id FROM drones WHERE id=$1) dd ON (b.cell_id=dd.cell_id OR dd.cell_id IS NULL)
        ORDER BY br.name, b.name`, droneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var checks []BatteryCheck
	for rows.Next() {
		var c BatteryCheck
		if err := rows.Scan(&c.ID, &c.Label, &c.Checked); err != nil {
			return nil, err
		}
		checks = append(checks, c)
	}
	return checks, rows.Err()
}

func (a *App) batteryChecks(r *http.Request, sessionID int) ([]BatteryCheck, error) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx, `
        SELECT b.id,
               COALESCE(br.name||' ','')|| b.name||' ('||COALESCE(c.label,'?')||' '||b.capacity_mah||'mAh)',
               EXISTS(SELECT 1 FROM session_batteries sb
                      WHERE sb.session_id=$1 AND sb.battery_id=b.id),
               COALESCE((SELECT sb.count FROM session_batteries sb
                         WHERE sb.session_id=$1 AND sb.battery_id=b.id), 1)
        FROM batteries b
        LEFT JOIN brands br ON br.id=b.brand_id
        LEFT JOIN cells c ON c.id=b.cell_id
        ORDER BY br.name, b.name`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var checks []BatteryCheck
	for rows.Next() {
		var c BatteryCheck
		if err := rows.Scan(&c.ID, &c.Label, &c.Checked, &c.Count); err != nil {
			return nil, err
		}
		checks = append(checks, c)
	}
	return checks, rows.Err()
}

func parseIntList(ss []string) []int {
	var out []int
	for _, s := range ss {
		v, err := strconv.Atoi(s)
		if err == nil && v > 0 {
			out = append(out, v)
		}
	}
	return out
}

// ---- Drones ----

func (a *App) handleDrones(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx, `
        SELECT d.id, d.name, d.status,
               COALESCE(sz.label,''), COALESCE(c.label,''),
               d.sub_250g,
               fl.flight_count, COALESCE(dp.id,0), fl.has_today
        FROM drones d
        LEFT JOIN sizes sz ON sz.id=d.size_id
        LEFT JOIN cells c ON c.id=d.cell_id
        LEFT JOIN LATERAL (
            SELECT id FROM drone_photos WHERE drone_id=d.id ORDER BY created_at LIMIT 1
        ) dp ON true
        LEFT JOIN LATERAL (
            SELECT COUNT(*)::int AS flight_count,
                   COALESCE(bool_or(s.session_date=CURRENT_DATE), false) AS has_today
            FROM session_drones sd
            JOIN sessions s ON s.id=sd.session_id
            WHERE sd.drone_id=d.id AND s.type='flight'
        ) fl ON true
        ORDER BY d.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	defer rows.Close()
	var drones []DroneRow
	for rows.Next() {
		var d DroneRow
		if err := rows.Scan(&d.ID, &d.Name, &d.Status,
			&d.SizeInch, &d.CellLabel, &d.Sub250g, &d.FlightCount, &d.FirstPhotoID, &d.HasFlightToday); err != nil {
			httpErr(w, err)
			return
		}
		drones = append(drones, d)
	}
	if err := rows.Err(); err != nil {
		httpErr(w, err)
		return
	}
	render(w, "drone-list", DroneListPage{ActiveTab: "drones", Drones: drones})
}

func (a *App) motorOptions(r *http.Request) ([]OptionItem, error) {
	return a.queryOptions(r, `SELECT m.id, COALESCE(b.name||' ','')|| m.name FROM motors m LEFT JOIN brands b ON b.id=m.brand_id ORDER BY b.name,m.name`)
}


func (a *App) placeOptions(r *http.Request) ([]OptionItem, error) {
	return a.queryOptions(r, `SELECT id, name FROM places ORDER BY name`)
}

func (a *App) brandOptions(r *http.Request) ([]OptionItem, error) {
	return a.queryOptions(r, `SELECT id, name FROM brands ORDER BY name`)
}

func (a *App) gpsOptions(r *http.Request) ([]OptionItem, error) {
	return a.queryOptions(r, `SELECT g.id, COALESCE(b.name||' ','')|| g.name FROM gps_modules g LEFT JOIN brands b ON b.id=g.brand_id ORDER BY b.name, g.name`)
}

func (a *App) radioProtocolOptions(r *http.Request) ([]OptionItem, error) {
	return a.queryOptions(r, `SELECT id, name FROM radio_protocols ORDER BY name`)
}

func (a *App) mcuOptions(r *http.Request) ([]OptionItem, error) {
	return a.queryOptions(r, `SELECT id, name FROM mcus ORDER BY name`)
}

func (a *App) rxOptions(r *http.Request) ([]OptionItem, error) {
	return a.queryOptions(r, `SELECT rx.id, COALESCE(b.name||' ','')|| rx.name FROM radio_receivers rx LEFT JOIN brands b ON b.id=rx.brand_id ORDER BY b.name, rx.name`)
}

func (a *App) fillDroneFormOptions(r *http.Request, page *DroneFormPage) {
	page.Sizes, _ = a.sizeOptions(r)
	page.Cells, _ = a.cellOptions(r)
	page.Frames, _ = a.frameOptions(r)
	page.FCs, _ = a.fcOptions(r)
	page.ESCs, _ = a.escOptions(r)
	page.VTXs, _ = a.vtxOptions(r)
	page.Motors, _ = a.motorOptions(r)
	page.GPSs, _ = a.gpsOptions(r)
	page.RXs, _ = a.rxOptions(r)
}

func (a *App) fillDronePageOptions(r *http.Request, page *DronePage) {
	page.Sizes, _ = a.sizeOptions(r)
	page.Cells, _ = a.cellOptions(r)
	page.Frames, _ = a.frameOptions(r)
	page.FCs, _ = a.fcOptions(r)
	page.ESCs, _ = a.escOptions(r)
	page.VTXs, _ = a.vtxOptions(r)
	page.Motors, _ = a.motorOptions(r)
	page.GPSs, _ = a.gpsOptions(r)
	page.RXs, _ = a.rxOptions(r)
}

func (a *App) handleDroneNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			page := DroneFormPage{ActiveTab: "drones", Error: "Name is required", Status: "build"}
			a.fillDroneFormOptions(r, &page)
			render(w, "drone-form", page)
			return
		}
		mc, _ := strconv.Atoi(r.FormValue("motor_count"))
		if mc == 0 {
			mc = 4
		}
		var droneID int
		err := a.db.QueryRow(ctx, `
            INSERT INTO drones (name,frame_id,fc_id,esc_id,vtx_id,motor_id,motor_count,size_id,cell_id,gps_id,rx_id,status,build_date,weight_g,sub_250g,notes)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16) RETURNING id`,
			name,
			nullIntPtr(r.FormValue("frame_id")),
			nullIntPtr(r.FormValue("fc_id")),
			nullIntPtr(r.FormValue("esc_id")),
			nullIntPtr(r.FormValue("vtx_id")),
			nullIntPtr(r.FormValue("motor_id")),
			mc,
			nullIntPtr(r.FormValue("size_id")),
			nullIntPtr(r.FormValue("cell_id")),
			nullIntPtr(r.FormValue("gps_id")),
			nullIntPtr(r.FormValue("rx_id")),
			r.FormValue("status"),
			nullDate(r.FormValue("build_date")),
			nullIntPtr(r.FormValue("weight_g")),
			r.FormValue("sub_250g") == "on",
			r.FormValue("notes")).Scan(&droneID)
		if err != nil {
			httpErr(w, err)
			return
		}
		a.updateDroneBatteries(ctx, droneID, r.Form["battery_ids"])
		for _, pidStr := range r.Form["prop_ids"] {
			pid, _ := strconv.Atoi(pidStr)
			if pid > 0 {
				a.db.Exec(ctx, `INSERT INTO drone_props (drone_id,prop_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, droneID, pid)
			}
		}
		http.Redirect(w, r, "/drones", http.StatusSeeOther)
		return
	}
	page := DroneFormPage{ActiveTab: "drones", Status: "build", BuildDate: time.Now().Format("2006-01-02")}
	a.fillDroneFormOptions(r, &page)
	page.Batteries, _ = a.droneBatteryChecks(r, 0)
	page.Props, _ = a.dronePropChecks(r, 0)
	render(w, "drone-form", page)
}

func (a *App) handleDroneEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodGet {
		http.Redirect(w, r, fmt.Sprintf("/drones/%d", id), http.StatusSeeOther)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			page := DroneFormPage{ActiveTab: "drones", Error: "Name is required", ID: id}
			a.fillDroneFormOptions(r, &page)
			render(w, "drone-form", page)
			return
		}
		mc, _ := strconv.Atoi(r.FormValue("motor_count"))
		if mc == 0 {
			mc = 4
		}
		_, err := a.db.Exec(ctx, `
            UPDATE drones SET name=$1,frame_id=$2,fc_id=$3,esc_id=$4,vtx_id=$5,
            motor_id=$6,motor_count=$7,size_id=$8,cell_id=$9,
            gps_id=$10,rx_id=$11,status=$12,build_date=$13,weight_g=$14,sub_250g=$15,notes=$16 WHERE id=$17`,
			name,
			nullIntPtr(r.FormValue("frame_id")),
			nullIntPtr(r.FormValue("fc_id")),
			nullIntPtr(r.FormValue("esc_id")),
			nullIntPtr(r.FormValue("vtx_id")),
			nullIntPtr(r.FormValue("motor_id")),
			mc,
			nullIntPtr(r.FormValue("size_id")),
			nullIntPtr(r.FormValue("cell_id")),
			nullIntPtr(r.FormValue("gps_id")),
			nullIntPtr(r.FormValue("rx_id")),
			r.FormValue("status"),
			nullDate(r.FormValue("build_date")),
			nullIntPtr(r.FormValue("weight_g")),
			r.FormValue("sub_250g") == "on",
			r.FormValue("notes"), id)
		if err != nil {
			httpErr(w, err)
			return
		}
		a.updateDroneBatteries(ctx, id, r.Form["battery_ids"])
		newPropIDs := parseIntList(r.Form["prop_ids"])
		a.db.Exec(ctx, `DELETE FROM drone_props WHERE drone_id=$1`, id)
		for _, pid := range newPropIDs {
			a.db.Exec(ctx, `INSERT INTO drone_props (drone_id,prop_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, id, pid)
		}
		http.Redirect(w, r, "/drones", http.StatusSeeOther)
		return
	}
	var page DroneFormPage
	var frameID, fcID, escID, vtxID, motorID, sizeID, cellID, gpsID, rxID *int
	var motorCount int
	var bd *string
	var weightG *int
	err := a.db.QueryRow(ctx, `
        SELECT id,name,frame_id,fc_id,esc_id,vtx_id,motor_id,motor_count,size_id,cell_id,
               gps_id,rx_id,
               status,TO_CHAR(build_date,'YYYY-MM-DD'),weight_g,sub_250g,notes
        FROM drones WHERE id=$1`, id).Scan(
		&page.ID, &page.Name, &frameID, &fcID, &escID, &vtxID,
		&motorID, &motorCount, &sizeID, &cellID,
		&gpsID, &rxID, &page.Status, &bd, &weightG, &page.Sub250g, &page.Notes)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	page.FrameID = derefInt(frameID)
	page.FCID = derefInt(fcID)
	page.ESCID = derefInt(escID)
	page.VTXID = derefInt(vtxID)
	page.MotorID = derefInt(motorID)
	page.SizeID = derefInt(sizeID)
	page.CellID = derefInt(cellID)
	page.GPSID = derefInt(gpsID)
	page.RXID = derefInt(rxID)
	page.MotorCount = strconv.Itoa(motorCount)
	if bd != nil {
		page.BuildDate = *bd
	}
	if weightG != nil {
		page.WeightG = strconv.Itoa(*weightG)
	}
	photoRows, err := a.db.Query(ctx,
		`SELECT id, original_name, notes FROM drone_photos WHERE drone_id=$1 ORDER BY created_at`, id)
	if err != nil {
		httpErr(w, err)
		return
	}
	for photoRows.Next() {
		var p DronePhotoRow
		if err := photoRows.Scan(&p.ID, &p.OriginalName, &p.Notes); err != nil {
			photoRows.Close()
			httpErr(w, err)
			return
		}
		page.Photos = append(page.Photos, p)
	}
	photoRows.Close()
	if err := photoRows.Err(); err != nil {
		httpErr(w, err)
		return
	}
	page.ActiveTab = "drones"
	a.fillDroneFormOptions(r, &page)
	page.Batteries, _ = a.droneBatteryChecks(r, id)
	page.Props, _ = a.dronePropChecks(r, id)
	render(w, "drone-form", page)
}

func (a *App) handleDroneDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(r.Context(), `DELETE FROM drones WHERE id=$1`, id)
	http.Redirect(w, r, "/drones", http.StatusSeeOther)
}

func (a *App) handleQuickFlight(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	tx, err := a.db.Begin(ctx)
	if err != nil {
		httpErr(w, err)
		return
	}
	defer tx.Rollback(ctx)
	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM session_drones sd
			JOIN sessions s ON s.id=sd.session_id
			WHERE sd.drone_id=$1 AND s.type='flight' AND s.session_date=CURRENT_DATE
		)`, id).Scan(&exists); err != nil {
		httpErr(w, err)
		return
	}
	if exists {
		http.Redirect(w, r, "/drones", http.StatusSeeOther)
		return
	}
	var sessionID int
	if err := tx.QueryRow(ctx,
		`INSERT INTO sessions (type,session_date,duration_min,location,notes)
		 VALUES ('flight',CURRENT_DATE,0,'',$1) RETURNING id`, quickFlightLabel,
	).Scan(&sessionID); err != nil {
		httpErr(w, err)
		return
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO session_drones (session_id,drone_id) VALUES ($1,$2)`, sessionID, id); err != nil {
		httpErr(w, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		httpErr(w, err)
		return
	}
	http.Redirect(w, r, "/drones", http.StatusSeeOther)
}

func (a *App) handleDronePhotoUpload(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoUpload(w, r, "drone_photos", "drone_id", "/drones/%d/edit")
}
func (a *App) handleDronePhotoServe(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoServe(w, r, "drone_photos")
}
func (a *App) handleDronePhotoDelete(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoDelete(w, r, "drone_photos", "drone_id", "/drones/%d/edit")
}
func (a *App) handleDronePhotoNote(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoNote(w, r, "drone_photos", "drone_id", "/drones/%d/edit")
}

// ---- Drone log ----

func (a *App) handleDroneDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	var page DronePage
	var frameID, fcID, escID, vtxID, motorID, sizeID, cellID, gpsID, rxID *int
	var motorCount int
	var bd *string
	var weightGInt *int
	err := a.db.QueryRow(ctx, `
        SELECT id,name,status,size_id,cell_id,frame_id,fc_id,esc_id,vtx_id,motor_id,motor_count,
               gps_id,rx_id,TO_CHAR(build_date,'YYYY-MM-DD'),weight_g,sub_250g,notes
        FROM drones WHERE id=$1`, id).Scan(
		&page.ID, &page.Name, &page.Status,
		&sizeID, &cellID, &frameID, &fcID, &escID, &vtxID, &motorID, &motorCount,
		&gpsID, &rxID, &bd, &weightGInt, &page.Sub250g, &page.Notes)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	page.SizeID = derefInt(sizeID)
	page.CellID = derefInt(cellID)
	page.FrameID = derefInt(frameID)
	page.FCID = derefInt(fcID)
	page.ESCID = derefInt(escID)
	page.VTXID = derefInt(vtxID)
	page.MotorID = derefInt(motorID)
	page.MotorCount = strconv.Itoa(motorCount)
	page.GPSID = derefInt(gpsID)
	page.RXID = derefInt(rxID)
	if bd != nil {
		page.BuildDate = *bd
	}
	if weightGInt != nil {
		page.WeightG = strconv.Itoa(*weightGInt)
	}

	batRows, _ := a.db.Query(ctx, `
        SELECT COALESCE(br.name||' ','')|| b.name||' ('||COALESCE(c2.label,'?')||' '||b.capacity_mah||'mAh)'
        FROM drone_batteries db2
        JOIN batteries b ON b.id=db2.battery_id
        LEFT JOIN brands br ON br.id=b.brand_id
        LEFT JOIN cells c2 ON c2.id=b.cell_id
        WHERE db2.drone_id=$1 ORDER BY b.name`, id)
	var batNames []string
	if batRows != nil {
		for batRows.Next() {
			var s string
			batRows.Scan(&s)
			batNames = append(batNames, s)
		}
		batRows.Close()
	}
	page.BatteryNames = strings.Join(batNames, ", ")

	propRows, _ := a.db.Query(ctx, `
        SELECT COALESCE(br.name||' ','')|| p.name||COALESCE(' '||sz2.label||'"','')
        FROM drone_props dp2
        JOIN propellers p ON p.id=dp2.prop_id
        LEFT JOIN brands br ON br.id=p.brand_id
        LEFT JOIN sizes sz2 ON sz2.id=p.size_id
        WHERE dp2.drone_id=$1 ORDER BY p.name`, id)
	var propNames []string
	if propRows != nil {
		for propRows.Next() {
			var s string
			propRows.Scan(&s)
			propNames = append(propNames, s)
		}
		propRows.Close()
	}
	page.PropNames = strings.Join(propNames, ", ")

	page.Batteries, _ = a.droneBatteryChecks(r, id)
	page.Photos = a.fetchItemPhotos(ctx, "drone_photos", "drone_id", id)

	logRows, err := a.db.Query(ctx,
		`SELECT id, logged_at, body FROM drone_log_entries WHERE drone_id=$1`, id)
	if err != nil {
		httpErr(w, err)
		return
	}
	for logRows.Next() {
		var e DroneTimelineEntry
		var t time.Time
		if logRows.Scan(&e.LogID, &t, &e.Body) == nil {
			lt := t.Local()
			e.Kind = "log"
			e.SortKey = lt.Format("2006-01-02 15:04:05")
			e.LoggedAt = lt.Format("2006-01-02 15:04")
			e.LoggedAtInput = lt.Format("2006-01-02T15:04")
			page.Timeline = append(page.Timeline, e)
		}
	}
	logRows.Close()

	sessRows, _ := a.db.Query(ctx, `
        SELECT s.id, s.title, s.type, TO_CHAR(s.session_date,'YYYY-MM-DD'),
               s.duration_min, s.location
        FROM sessions s
        JOIN session_drones sd ON sd.session_id=s.id
        WHERE sd.drone_id=$1`, id)
	if sessRows != nil {
		for sessRows.Next() {
			var e DroneTimelineEntry
			e.Kind = "session"
			sessRows.Scan(&e.SessionID, &e.SessionTitle, &e.SessionType, &e.Date, &e.DurationMin, &e.Location)
			e.SortKey = e.Date + " 00:00:00"
			page.Timeline = append(page.Timeline, e)
		}
		sessRows.Close()
	}
	sort.Slice(page.Timeline, func(i, j int) bool {
		return page.Timeline[i].SortKey > page.Timeline[j].SortKey
	})

	a.fillDronePageOptions(r, &page)
	page.ActiveTab = "drones"
	render(w, "drone", page)
}

func (a *App) updateDroneBatteries(ctx context.Context, droneID int, bidStrs []string) {
	a.db.Exec(ctx, `DELETE FROM drone_batteries WHERE drone_id=$1`, droneID)
	for _, s := range bidStrs {
		bid, _ := strconv.Atoi(s)
		if bid > 0 {
			a.db.Exec(ctx, `INSERT INTO drone_batteries (drone_id,battery_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, droneID, bid)
		}
	}
}

func (a *App) handleDroneBatteries(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		httpErr(w, err)
		return
	}
	a.updateDroneBatteries(r.Context(), id, r.Form["battery_ids"])
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleDroneSave(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		httpErr(w, err)
		return
	}
	mc, _ := strconv.Atoi(r.FormValue("motor_count"))
	if mc == 0 {
		mc = 4
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	a.db.Exec(r.Context(), `
        UPDATE drones SET name=$1,frame_id=$2,fc_id=$3,esc_id=$4,vtx_id=$5,
        motor_id=$6,motor_count=$7,size_id=$8,cell_id=$9,
        gps_id=$10,rx_id=$11,status=$12,build_date=$13,weight_g=$14,sub_250g=$15,notes=$16
        WHERE id=$17`,
		name,
		nullIntPtr(r.FormValue("frame_id")),
		nullIntPtr(r.FormValue("fc_id")),
		nullIntPtr(r.FormValue("esc_id")),
		nullIntPtr(r.FormValue("vtx_id")),
		nullIntPtr(r.FormValue("motor_id")),
		mc,
		nullIntPtr(r.FormValue("size_id")),
		nullIntPtr(r.FormValue("cell_id")),
		nullIntPtr(r.FormValue("gps_id")),
		nullIntPtr(r.FormValue("rx_id")),
		r.FormValue("status"),
		nullDate(r.FormValue("build_date")),
		nullIntPtr(r.FormValue("weight_g")),
		r.FormValue("sub_250g") == "on",
		r.FormValue("notes"),
		id)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleDroneLogAdd(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		httpErr(w, err)
		return
	}
	loggedAt := time.Now()
	if s := r.FormValue("logged_at"); s != "" {
		if t, err := time.ParseInLocation("2006-01-02T15:04", s, time.Local); err == nil {
			loggedAt = t
		}
	}
	body := strings.TrimSpace(r.FormValue("body"))
	if body != "" {
		a.db.Exec(r.Context(),
			`INSERT INTO drone_log_entries (drone_id, logged_at, body) VALUES ($1, $2, $3)`,
			id, loggedAt, body)
	}
	http.Redirect(w, r, fmt.Sprintf("/drones/%d", id), http.StatusSeeOther)
}

func (a *App) handleDroneLogDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	var droneID int
	err := a.db.QueryRow(ctx, `SELECT drone_id FROM drone_log_entries WHERE id=$1`, id).Scan(&droneID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(ctx, `DELETE FROM drone_log_entries WHERE id=$1`, id)
	http.Redirect(w, r, fmt.Sprintf("/drones/%d", droneID), http.StatusSeeOther)
}

func (a *App) handleDroneLogEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		httpErr(w, err)
		return
	}
	ctx := r.Context()
	var droneID int
	err := a.db.QueryRow(ctx, `SELECT drone_id FROM drone_log_entries WHERE id=$1`, id).Scan(&droneID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	loggedAt := time.Now()
	if s := r.FormValue("logged_at"); s != "" {
		if t, err := time.ParseInLocation("2006-01-02T15:04", s, time.Local); err == nil {
			loggedAt = t
		}
	}
	body := strings.TrimSpace(r.FormValue("body"))
	a.db.Exec(ctx, `UPDATE drone_log_entries SET logged_at=$1, body=$2 WHERE id=$3`, loggedAt, body, id)
	http.Redirect(w, r, fmt.Sprintf("/drones/%d", droneID), http.StatusSeeOther)
}

// ---- Item photo helpers ----

func (a *App) itemPhotoUpload(w http.ResponseWriter, r *http.Request, table, fkCol, redirectFmt string) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	if err := r.ParseMultipartForm(256 << 20); err != nil {
		httpErr(w, err)
		return
	}
	file, header, err := r.FormFile("photo")
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf(redirectFmt, id), http.StatusSeeOther)
		return
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	dst := filepath.Join(a.videoDir, filename)
	out, err := os.Create(dst)
	if err != nil {
		httpErr(w, err)
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		os.Remove(dst)
		httpErr(w, err)
		return
	}
	out.Close()

	_, err = a.db.Exec(ctx,
		`INSERT INTO `+table+` (`+fkCol+`,filename,original_name) VALUES ($1,$2,$3)`,
		id, filename, header.Filename)
	if err != nil {
		os.Remove(dst)
		httpErr(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf(redirectFmt, id), http.StatusSeeOther)
}

func (a *App) itemPhotoServe(w http.ResponseWriter, r *http.Request, table string) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var filename string
	err := a.db.QueryRow(r.Context(), `SELECT filename FROM `+table+` WHERE id=$1`, id).Scan(&filename)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(a.videoDir, filename))
}

func (a *App) itemPhotoDelete(w http.ResponseWriter, r *http.Request, table, fkCol, redirectFmt string) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	var filename string
	var itemID int
	err := a.db.QueryRow(ctx,
		`SELECT `+fkCol+`,filename FROM `+table+` WHERE id=$1`, id).Scan(&itemID, &filename)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(ctx, `DELETE FROM `+table+` WHERE id=$1`, id)
	os.Remove(filepath.Join(a.videoDir, filename))
	http.Redirect(w, r, fmt.Sprintf(redirectFmt, itemID), http.StatusSeeOther)
}

func (a *App) itemPhotoNote(w http.ResponseWriter, r *http.Request, table, fkCol, redirectFmt string) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		httpErr(w, err)
		return
	}
	var itemID int
	err := a.db.QueryRow(ctx, `SELECT `+fkCol+` FROM `+table+` WHERE id=$1`, id).Scan(&itemID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(ctx, `UPDATE `+table+` SET notes=$1 WHERE id=$2`, r.FormValue("notes"), id)
	http.Redirect(w, r, fmt.Sprintf(redirectFmt, itemID), http.StatusSeeOther)
}

// ---- Frame photos ----

func (a *App) handleFramePhotoUpload(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoUpload(w, r, "frame_photos", "frame_id", "/frames/%d/edit")
}
func (a *App) handleFramePhotoServe(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoServe(w, r, "frame_photos")
}
func (a *App) handleFramePhotoDelete(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoDelete(w, r, "frame_photos", "frame_id", "/frames/%d/edit")
}
func (a *App) handleFramePhotoNote(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoNote(w, r, "frame_photos", "frame_id", "/frames/%d/edit")
}

// ---- FC photos ----

func (a *App) handleFCPhotoUpload(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoUpload(w, r, "fc_photos", "fc_id", "/fcs/%d/edit")
}
func (a *App) handleFCPhotoServe(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoServe(w, r, "fc_photos")
}
func (a *App) handleFCPhotoDelete(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoDelete(w, r, "fc_photos", "fc_id", "/fcs/%d/edit")
}
func (a *App) handleFCPhotoNote(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoNote(w, r, "fc_photos", "fc_id", "/fcs/%d/edit")
}

// ---- ESC photos ----

func (a *App) handleESCPhotoUpload(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoUpload(w, r, "esc_photos", "esc_id", "/escs/%d/edit")
}
func (a *App) handleESCPhotoServe(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoServe(w, r, "esc_photos")
}
func (a *App) handleESCPhotoDelete(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoDelete(w, r, "esc_photos", "esc_id", "/escs/%d/edit")
}
func (a *App) handleESCPhotoNote(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoNote(w, r, "esc_photos", "esc_id", "/escs/%d/edit")
}

// ---- Motor photos ----

func (a *App) handleMotorPhotoUpload(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoUpload(w, r, "motor_photos", "motor_id", "/motors/%d/edit")
}
func (a *App) handleMotorPhotoServe(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoServe(w, r, "motor_photos")
}
func (a *App) handleMotorPhotoDelete(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoDelete(w, r, "motor_photos", "motor_id", "/motors/%d/edit")
}
func (a *App) handleMotorPhotoNote(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoNote(w, r, "motor_photos", "motor_id", "/motors/%d/edit")
}

// ---- VTX photos ----

func (a *App) handleVTXPhotoUpload(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoUpload(w, r, "vtx_photos", "vtx_id", "/vtx/%d/edit")
}
func (a *App) handleVTXPhotoServe(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoServe(w, r, "vtx_photos")
}
func (a *App) handleVTXPhotoDelete(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoDelete(w, r, "vtx_photos", "vtx_id", "/vtx/%d/edit")
}
func (a *App) handleVTXPhotoNote(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoNote(w, r, "vtx_photos", "vtx_id", "/vtx/%d/edit")
}

// ---- GPS photos ----

func (a *App) handleGPSPhotoUpload(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoUpload(w, r, "gps_photos", "gps_id", "/gps/%d/edit")
}
func (a *App) handleGPSPhotoServe(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoServe(w, r, "gps_photos")
}
func (a *App) handleGPSPhotoDelete(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoDelete(w, r, "gps_photos", "gps_id", "/gps/%d/edit")
}
func (a *App) handleGPSPhotoNote(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoNote(w, r, "gps_photos", "gps_id", "/gps/%d/edit")
}

// ---- RX photos ----

func (a *App) handleRXPhotoUpload(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoUpload(w, r, "rx_photos", "rx_id", "/rx/%d/edit")
}
func (a *App) handleRXPhotoServe(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoServe(w, r, "rx_photos")
}
func (a *App) handleRXPhotoDelete(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoDelete(w, r, "rx_photos", "rx_id", "/rx/%d/edit")
}
func (a *App) handleRXPhotoNote(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoNote(w, r, "rx_photos", "rx_id", "/rx/%d/edit")
}

// ---- Battery photos ----

func (a *App) handleBatteryPhotoUpload(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoUpload(w, r, "battery_photos", "battery_id", "/batteries/%d/edit")
}
func (a *App) handleBatteryPhotoServe(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoServe(w, r, "battery_photos")
}
func (a *App) handleBatteryPhotoDelete(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoDelete(w, r, "battery_photos", "battery_id", "/batteries/%d/edit")
}
func (a *App) handleBatteryPhotoNote(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoNote(w, r, "battery_photos", "battery_id", "/batteries/%d/edit")
}

// ---- Prop photos ----

func (a *App) handlePropPhotoUpload(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoUpload(w, r, "prop_photos", "prop_id", "/props/%d/edit")
}
func (a *App) handlePropPhotoServe(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoServe(w, r, "prop_photos")
}
func (a *App) handlePropPhotoDelete(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoDelete(w, r, "prop_photos", "prop_id", "/props/%d/edit")
}
func (a *App) handlePropPhotoNote(w http.ResponseWriter, r *http.Request) {
	a.itemPhotoNote(w, r, "prop_photos", "prop_id", "/props/%d/edit")
}

func (a *App) fetchItemPhotos(ctx context.Context, table, fkCol string, id int) []DronePhotoRow {
	rows, err := a.db.Query(ctx,
		`SELECT id, original_name, notes FROM `+table+` WHERE `+fkCol+`=$1 ORDER BY created_at`, id)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var photos []DronePhotoRow
	for rows.Next() {
		var p DronePhotoRow
		if rows.Scan(&p.ID, &p.OriginalName, &p.Notes) == nil {
			photos = append(photos, p)
		}
	}
	return photos
}

// ---- Inventory ----

func (a *App) handleInventory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page := InventoryPage{ActiveTab: "inventory"}

	// Each component table has one row per unique model.
	// item_counts.count = total owned.
	// installed = count(drones) referencing this model (or sum(motor_count) for motors).
	// available = total - installed.

	rows, err := a.db.Query(ctx, `
        SELECT f.id, COALESCE(br.name,''), f.name, COALESCE(sz.label,''), f.weight_g,
               COALESCE(ic.count,0),
               COUNT(d.id)::int,
               COALESCE(ic.count,0) - COUNT(d.id)::int,
               COALESCE(string_agg(d.name,', ' ORDER BY d.name),''),
               COALESCE(fp.id,0)
        FROM frames f
        LEFT JOIN brands br ON br.id=f.brand_id
        LEFT JOIN sizes sz ON sz.id=f.size_id
        LEFT JOIN item_counts ic ON ic.item_type='frame' AND ic.item_id=f.id
        LEFT JOIN drones d ON d.frame_id=f.id
        LEFT JOIN LATERAL (SELECT id FROM frame_photos WHERE frame_id=f.id ORDER BY created_at LIMIT 1) fp ON true
        GROUP BY f.id, br.name, f.name, sz.label, f.weight_g, ic.count, fp.id
        ORDER BY br.name, f.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var fr FrameRow
		var weightG *int
		if err := rows.Scan(&fr.ID, &fr.Brand, &fr.Name, &fr.SizeInch, &weightG,
			&fr.Total, &fr.Installed, &fr.Available, &fr.InstalledOn, &fr.FirstPhotoID); err != nil {
			rows.Close()
			httpErr(w, err)
			return
		}
		if weightG != nil {
			fr.WeightG = strconv.Itoa(*weightG)
		}
		page.Frames = append(page.Frames, fr)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpErr(w, err)
		return
	}

	rows, err = a.db.Query(ctx, `
        SELECT fc.id, COALESCE(br.name,''), fc.name, COALESCE(m.name,''), fc.firmware,
               COALESCE(ic.count,0),
               COUNT(d.id)::int,
               COALESCE(ic.count,0) - COUNT(d.id)::int,
               COALESCE(string_agg(d.name,', ' ORDER BY d.name),''),
               COALESCE(fcp.id,0)
        FROM flight_controllers fc
        LEFT JOIN brands br ON br.id=fc.brand_id
        LEFT JOIN mcus m ON m.id=fc.mcu_id
        LEFT JOIN item_counts ic ON ic.item_type='fc' AND ic.item_id=fc.id
        LEFT JOIN drones d ON d.fc_id=fc.id
        LEFT JOIN LATERAL (SELECT id FROM fc_photos WHERE fc_id=fc.id ORDER BY created_at LIMIT 1) fcp ON true
        GROUP BY fc.id, br.name, fc.name, m.name, fc.firmware, ic.count, fcp.id
        ORDER BY br.name, fc.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var fc FCRow
		if err := rows.Scan(&fc.ID, &fc.Brand, &fc.Name, &fc.MCU, &fc.Firmware,
			&fc.Total, &fc.Installed, &fc.Available, &fc.InstalledOn, &fc.FirstPhotoID); err != nil {
			rows.Close()
			httpErr(w, err)
			return
		}
		page.FCs = append(page.FCs, fc)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpErr(w, err)
		return
	}

	rows, err = a.db.Query(ctx, `
        SELECT e.id, COALESCE(br.name,''), e.name, e.current_rating, COALESCE(c.label,''),
               COALESCE(ic.count,0),
               COUNT(d.id)::int,
               COALESCE(ic.count,0) - COUNT(d.id)::int,
               COALESCE(string_agg(d.name,', ' ORDER BY d.name),''),
               COALESCE(ep.id,0)
        FROM escs e
        LEFT JOIN brands br ON br.id=e.brand_id
        LEFT JOIN cells c ON c.id=e.cell_max_id
        LEFT JOIN item_counts ic ON ic.item_type='esc' AND ic.item_id=e.id
        LEFT JOIN drones d ON d.esc_id=e.id
        LEFT JOIN LATERAL (SELECT id FROM esc_photos WHERE esc_id=e.id ORDER BY created_at LIMIT 1) ep ON true
        GROUP BY e.id, br.name, e.name, e.current_rating, c.label, ic.count, ep.id
        ORDER BY br.name, e.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var e ESCRow
		var cr *int
		if err := rows.Scan(&e.ID, &e.Brand, &e.Name, &cr, &e.CellMax,
			&e.Total, &e.Installed, &e.Available, &e.InstalledOn, &e.FirstPhotoID); err != nil {
			rows.Close()
			httpErr(w, err)
			return
		}
		if cr != nil {
			e.CurrentRating = strconv.Itoa(*cr)
		}
		page.ESCs = append(page.ESCs, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpErr(w, err)
		return
	}

	rows, err = a.db.Query(ctx, `
        SELECT m.id, COALESCE(br.name,''), m.name, m.stator_size, m.kv,
               COALESCE(ic.count,0),
               COALESCE(SUM(d.motor_count),0)::int,
               COALESCE(ic.count,0) - COALESCE(SUM(d.motor_count),0)::int,
               COALESCE(string_agg(d.name||' (×'||d.motor_count||')',', ' ORDER BY d.name),''),
               COALESCE(mp.id,0)
        FROM motors m
        LEFT JOIN brands br ON br.id=m.brand_id
        LEFT JOIN item_counts ic ON ic.item_type='motor' AND ic.item_id=m.id
        LEFT JOIN drones d ON d.motor_id=m.id
        LEFT JOIN LATERAL (SELECT id FROM motor_photos WHERE motor_id=m.id ORDER BY created_at LIMIT 1) mp ON true
        GROUP BY m.id, br.name, m.name, m.stator_size, m.kv, ic.count, mp.id
        ORDER BY br.name, m.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var m MotorRow
		var kv *int
		if err := rows.Scan(&m.ID, &m.Brand, &m.Name, &m.StatorSize, &kv,
			&m.Total, &m.Installed, &m.Available, &m.InstalledOn, &m.FirstPhotoID); err != nil {
			rows.Close()
			httpErr(w, err)
			return
		}
		if kv != nil {
			m.KV = strconv.Itoa(*kv)
		}
		page.Motors = append(page.Motors, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpErr(w, err)
		return
	}

	rows, err = a.db.Query(ctx, `
        SELECT v.id, COALESCE(br.name,''), v.name,
               COALESCE(ic.count,0),
               COUNT(d.id)::int,
               COALESCE(ic.count,0) - COUNT(d.id)::int,
               COALESCE(string_agg(d.name,', ' ORDER BY d.name),''),
               COALESCE(vp.id,0)
        FROM vtx_units v
        LEFT JOIN brands br ON br.id=v.brand_id
        LEFT JOIN item_counts ic ON ic.item_type='vtx' AND ic.item_id=v.id
        LEFT JOIN drones d ON d.vtx_id=v.id
        LEFT JOIN LATERAL (SELECT id FROM vtx_photos WHERE vtx_id=v.id ORDER BY created_at LIMIT 1) vp ON true
        GROUP BY v.id, br.name, v.name, ic.count, vp.id
        ORDER BY br.name, v.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var v VTXRow
		if err := rows.Scan(&v.ID, &v.Brand, &v.Name,
			&v.Total, &v.Installed, &v.Available, &v.InstalledOn, &v.FirstPhotoID); err != nil {
			rows.Close()
			httpErr(w, err)
			return
		}
		page.VTXs = append(page.VTXs, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpErr(w, err)
		return
	}

	rows, err = a.db.Query(ctx, `
        SELECT g.id, COALESCE(br.name,''), g.name,
               COALESCE(ic.count,0),
               COUNT(d.id)::int,
               COALESCE(ic.count,0) - COUNT(d.id)::int,
               COALESCE(string_agg(d.name,', ' ORDER BY d.name),''),
               COALESCE(gp.id,0)
        FROM gps_modules g
        LEFT JOIN brands br ON br.id=g.brand_id
        LEFT JOIN item_counts ic ON ic.item_type='gps' AND ic.item_id=g.id
        LEFT JOIN drones d ON d.gps_id=g.id
        LEFT JOIN LATERAL (SELECT id FROM gps_photos WHERE gps_id=g.id ORDER BY created_at LIMIT 1) gp ON true
        GROUP BY g.id, br.name, g.name, ic.count, gp.id
        ORDER BY br.name, g.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var g GPSRow
		if err := rows.Scan(&g.ID, &g.Brand, &g.Name,
			&g.Total, &g.Installed, &g.Available, &g.InstalledOn, &g.FirstPhotoID); err != nil {
			rows.Close()
			httpErr(w, err)
			return
		}
		page.GPSs = append(page.GPSs, g)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpErr(w, err)
		return
	}

	rows, err = a.db.Query(ctx, `
        SELECT rx.id, COALESCE(br.name,''), rx.name, COALESCE(rp.name,''),
               COALESCE(ic.count,0),
               COUNT(d.id)::int,
               COALESCE(ic.count,0) - COUNT(d.id)::int,
               COALESCE(string_agg(d.name,', ' ORDER BY d.name),''),
               COALESCE(rxp.id,0)
        FROM radio_receivers rx
        LEFT JOIN brands br ON br.id=rx.brand_id
        LEFT JOIN radio_protocols rp ON rp.id=rx.protocol_id
        LEFT JOIN item_counts ic ON ic.item_type='rx' AND ic.item_id=rx.id
        LEFT JOIN drones d ON d.rx_id=rx.id
        LEFT JOIN LATERAL (SELECT id FROM rx_photos WHERE rx_id=rx.id ORDER BY created_at LIMIT 1) rxp ON true
        GROUP BY rx.id, br.name, rx.name, rp.name, ic.count, rxp.id
        ORDER BY br.name, rx.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var rx RXRow
		if err := rows.Scan(&rx.ID, &rx.Brand, &rx.Name, &rx.Protocol,
			&rx.Total, &rx.Installed, &rx.Available, &rx.InstalledOn, &rx.FirstPhotoID); err != nil {
			rows.Close()
			httpErr(w, err)
			return
		}
		page.RXs = append(page.RXs, rx)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpErr(w, err)
		return
	}

	render(w, "inventory", page)
}

// ---- Frames ----

func (a *App) handleFrameNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			page := FrameFormPage{ActiveTab: "inventory", Error: "Name is required"}
			page.Brands, _ = a.brandOptions(r)
			render(w, "frame-form", page)
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		var id int
		err := a.db.QueryRow(ctx,
			`INSERT INTO frames (brand_id,name,size_id,weight_g,notes) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			nullIntPtr(r.FormValue("brand_id")), name,
			nullIntPtr(r.FormValue("size_id")), nullIntPtr(r.FormValue("weight_g")),
			r.FormValue("notes")).Scan(&id)
		if err != nil {
			httpErr(w, err)
			return
		}
		a.db.Exec(ctx, `INSERT INTO item_counts (item_type,item_id,count) VALUES ('frame',$1,$2)`, id, qty)
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	page := FrameFormPage{ActiveTab: "inventory"}
	page.Brands, _ = a.brandOptions(r)
	page.Sizes, _ = a.sizeOptions(r)
	render(w, "frame-form", page)
}

func (a *App) handleFrameEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			page := FrameFormPage{ActiveTab: "inventory", Error: "Name is required", ID: id}
			page.Brands, _ = a.brandOptions(r)
			render(w, "frame-form", page)
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		_, err := a.db.Exec(ctx,
			`UPDATE frames SET brand_id=$1,name=$2,size_id=$3,weight_g=$4,notes=$5 WHERE id=$6`,
			nullIntPtr(r.FormValue("brand_id")), name,
			nullIntPtr(r.FormValue("size_id")), nullIntPtr(r.FormValue("weight_g")),
			r.FormValue("notes"), id)
		if err != nil {
			httpErr(w, err)
			return
		}
		a.db.Exec(ctx,
			`INSERT INTO item_counts (item_type,item_id,count) VALUES ('frame',$1,$2)
             ON CONFLICT (item_type,item_id) DO UPDATE SET count=$2`, id, qty)
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	var page FrameFormPage
	var brandID, sizeID *int
	var weightG *int
	var qty int
	err := a.db.QueryRow(ctx,
		`SELECT f.id,f.brand_id,f.name,f.size_id,f.weight_g,f.notes,COALESCE(ic.count,0)
         FROM frames f LEFT JOIN item_counts ic ON ic.item_type='frame' AND ic.item_id=f.id
         WHERE f.id=$1`, id).
		Scan(&page.ID, &brandID, &page.Name, &sizeID, &weightG, &page.Notes, &qty)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	page.BrandID = derefInt(brandID)
	page.SizeID = derefInt(sizeID)
	if weightG != nil {
		page.WeightG = strconv.Itoa(*weightG)
	}
	page.Quantity = strconv.Itoa(qty)
	page.ActiveTab = "inventory"
	page.Brands, _ = a.brandOptions(r)
	page.Sizes, _ = a.sizeOptions(r)
	page.Photos = a.fetchItemPhotos(ctx, "frame_photos", "frame_id", id)
	render(w, "frame-form", page)
}

func (a *App) handleFrameDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(ctx, `DELETE FROM item_counts WHERE item_type='frame' AND item_id=$1`, id)
	a.db.Exec(ctx, `DELETE FROM frames WHERE id=$1`, id)
	http.Redirect(w, r, "/inventory", http.StatusSeeOther)
}

// ---- Flight Controllers ----

func (a *App) handleFCNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			page := FCFormPage{ActiveTab: "inventory", Error: "Name is required"}
			page.Brands, _ = a.brandOptions(r)
			render(w, "fc-form", page)
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		var id int
		err := a.db.QueryRow(ctx,
			`INSERT INTO flight_controllers (brand_id,name,mcu_id,firmware,notes) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			nullIntPtr(r.FormValue("brand_id")), name, nullIntPtr(r.FormValue("mcu_id")), r.FormValue("firmware"),
			r.FormValue("notes")).Scan(&id)
		if err != nil {
			httpErr(w, err)
			return
		}
		a.db.Exec(ctx, `INSERT INTO item_counts (item_type,item_id,count) VALUES ('fc',$1,$2)`, id, qty)
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	page := FCFormPage{ActiveTab: "inventory"}
	page.Brands, _ = a.brandOptions(r)
	page.MCUs, _ = a.mcuOptions(r)
	render(w, "fc-form", page)
}

func (a *App) handleFCEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			page := FCFormPage{ActiveTab: "inventory", Error: "Name is required", ID: id}
			page.Brands, _ = a.brandOptions(r)
			render(w, "fc-form", page)
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		_, err := a.db.Exec(ctx,
			`UPDATE flight_controllers SET brand_id=$1,name=$2,mcu_id=$3,firmware=$4,notes=$5 WHERE id=$6`,
			nullIntPtr(r.FormValue("brand_id")), name, nullIntPtr(r.FormValue("mcu_id")), r.FormValue("firmware"),
			r.FormValue("notes"), id)
		if err != nil {
			httpErr(w, err)
			return
		}
		a.db.Exec(ctx,
			`INSERT INTO item_counts (item_type,item_id,count) VALUES ('fc',$1,$2)
             ON CONFLICT (item_type,item_id) DO UPDATE SET count=$2`, id, qty)
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	var page FCFormPage
	var brandID, mcuID *int
	var qty int
	err := a.db.QueryRow(ctx,
		`SELECT fc.id,fc.brand_id,fc.name,fc.mcu_id,fc.firmware,fc.notes,COALESCE(ic.count,0)
         FROM flight_controllers fc LEFT JOIN item_counts ic ON ic.item_type='fc' AND ic.item_id=fc.id
         WHERE fc.id=$1`, id).
		Scan(&page.ID, &brandID, &page.Name, &mcuID, &page.Firmware, &page.Notes, &qty)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	page.BrandID = derefInt(brandID)
	page.MCUID = derefInt(mcuID)
	page.Quantity = strconv.Itoa(qty)
	page.ActiveTab = "inventory"
	page.Brands, _ = a.brandOptions(r)
	page.MCUs, _ = a.mcuOptions(r)
	page.Photos = a.fetchItemPhotos(ctx, "fc_photos", "fc_id", id)
	render(w, "fc-form", page)
}

func (a *App) handleFCDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(ctx, `DELETE FROM item_counts WHERE item_type='fc' AND item_id=$1`, id)
	a.db.Exec(ctx, `DELETE FROM flight_controllers WHERE id=$1`, id)
	http.Redirect(w, r, "/inventory", http.StatusSeeOther)
}

// ---- ESCs ----

func (a *App) handleESCNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			page := ESCFormPage{ActiveTab: "inventory", Error: "Name is required"}
			page.Brands, _ = a.brandOptions(r)
			render(w, "esc-form", page)
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		var id int
		err := a.db.QueryRow(ctx,
			`INSERT INTO escs (brand_id,name,current_rating,cell_max_id,notes) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			nullIntPtr(r.FormValue("brand_id")), name,
			nullIntPtr(r.FormValue("current_rating")), nullIntPtr(r.FormValue("cell_max_id")),
			r.FormValue("notes")).Scan(&id)
		if err != nil {
			httpErr(w, err)
			return
		}
		a.db.Exec(ctx, `INSERT INTO item_counts (item_type,item_id,count) VALUES ('esc',$1,$2)`, id, qty)
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	page := ESCFormPage{ActiveTab: "inventory"}
	page.Brands, _ = a.brandOptions(r)
	page.Cells, _ = a.cellOptions(r)
	render(w, "esc-form", page)
}

func (a *App) handleESCEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			page := ESCFormPage{ActiveTab: "inventory", Error: "Name is required", ID: id}
			page.Brands, _ = a.brandOptions(r)
			render(w, "esc-form", page)
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		_, err := a.db.Exec(ctx,
			`UPDATE escs SET brand_id=$1,name=$2,current_rating=$3,cell_max_id=$4,notes=$5 WHERE id=$6`,
			nullIntPtr(r.FormValue("brand_id")), name,
			nullIntPtr(r.FormValue("current_rating")), nullIntPtr(r.FormValue("cell_max_id")),
			r.FormValue("notes"), id)
		if err != nil {
			httpErr(w, err)
			return
		}
		a.db.Exec(ctx,
			`INSERT INTO item_counts (item_type,item_id,count) VALUES ('esc',$1,$2)
             ON CONFLICT (item_type,item_id) DO UPDATE SET count=$2`, id, qty)
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	var page ESCFormPage
	var brandID, cellMaxID *int
	var cr *int
	var qty int
	err := a.db.QueryRow(ctx,
		`SELECT e.id,e.brand_id,e.name,e.current_rating,e.cell_max_id,e.notes,COALESCE(ic.count,0)
         FROM escs e LEFT JOIN item_counts ic ON ic.item_type='esc' AND ic.item_id=e.id
         WHERE e.id=$1`, id).
		Scan(&page.ID, &brandID, &page.Name, &cr, &cellMaxID, &page.Notes, &qty)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	page.BrandID = derefInt(brandID)
	page.CellMaxID = derefInt(cellMaxID)
	if cr != nil {
		page.CurrentRating = strconv.Itoa(*cr)
	}
	page.Quantity = strconv.Itoa(qty)
	page.ActiveTab = "inventory"
	page.Brands, _ = a.brandOptions(r)
	page.Cells, _ = a.cellOptions(r)
	page.Photos = a.fetchItemPhotos(ctx, "esc_photos", "esc_id", id)
	render(w, "esc-form", page)
}

func (a *App) handleESCDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(ctx, `DELETE FROM item_counts WHERE item_type='esc' AND item_id=$1`, id)
	a.db.Exec(ctx, `DELETE FROM escs WHERE id=$1`, id)
	http.Redirect(w, r, "/inventory", http.StatusSeeOther)
}

// ---- Motors ----

func (a *App) handleMotorNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			page := MotorFormPage{ActiveTab: "inventory", Error: "Name is required"}
			page.Brands, _ = a.brandOptions(r)
			render(w, "motor-form", page)
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		var id int
		err := a.db.QueryRow(ctx,
			`INSERT INTO motors (brand_id,name,stator_size,kv,notes) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			nullIntPtr(r.FormValue("brand_id")), name, r.FormValue("stator_size"),
			nullIntPtr(r.FormValue("kv")), r.FormValue("notes")).Scan(&id)
		if err != nil {
			httpErr(w, err)
			return
		}
		a.db.Exec(ctx, `INSERT INTO item_counts (item_type,item_id,count) VALUES ('motor',$1,$2)`, id, qty)
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	page := MotorFormPage{ActiveTab: "inventory"}
	page.Brands, _ = a.brandOptions(r)
	render(w, "motor-form", page)
}

func (a *App) handleMotorEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			page := MotorFormPage{ActiveTab: "inventory", Error: "Name is required", ID: id}
			page.Brands, _ = a.brandOptions(r)
			render(w, "motor-form", page)
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		_, err := a.db.Exec(ctx,
			`UPDATE motors SET brand_id=$1,name=$2,stator_size=$3,kv=$4,notes=$5 WHERE id=$6`,
			nullIntPtr(r.FormValue("brand_id")), name, r.FormValue("stator_size"),
			nullIntPtr(r.FormValue("kv")), r.FormValue("notes"), id)
		if err != nil {
			httpErr(w, err)
			return
		}
		a.db.Exec(ctx,
			`INSERT INTO item_counts (item_type,item_id,count) VALUES ('motor',$1,$2)
             ON CONFLICT (item_type,item_id) DO UPDATE SET count=$2`, id, qty)
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	var page MotorFormPage
	var brandID *int
	var kv *int
	var qty int
	err := a.db.QueryRow(ctx,
		`SELECT m.id,m.brand_id,m.name,m.stator_size,m.kv,m.notes,COALESCE(ic.count,0)
         FROM motors m LEFT JOIN item_counts ic ON ic.item_type='motor' AND ic.item_id=m.id
         WHERE m.id=$1`, id).
		Scan(&page.ID, &brandID, &page.Name, &page.StatorSize, &kv, &page.Notes, &qty)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	page.BrandID = derefInt(brandID)
	if kv != nil {
		page.KV = strconv.Itoa(*kv)
	}
	page.Quantity = strconv.Itoa(qty)
	page.ActiveTab = "inventory"
	page.Brands, _ = a.brandOptions(r)
	page.Photos = a.fetchItemPhotos(ctx, "motor_photos", "motor_id", id)
	render(w, "motor-form", page)
}

func (a *App) handleMotorDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(ctx, `DELETE FROM item_counts WHERE item_type='motor' AND item_id=$1`, id)
	a.db.Exec(ctx, `DELETE FROM motors WHERE id=$1`, id)
	http.Redirect(w, r, "/inventory", http.StatusSeeOther)
}

// ---- VTX ----

func (a *App) handleVTXNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			page := VTXFormPage{ActiveTab: "inventory", Error: "Name is required"}
			page.Brands, _ = a.brandOptions(r)
			render(w, "vtx-form", page)
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		var id int
		err := a.db.QueryRow(ctx,
			`INSERT INTO vtx_units (brand_id,name,weight_g,notes) VALUES ($1,$2,$3,$4) RETURNING id`,
			nullIntPtr(r.FormValue("brand_id")), name,
			nullIntPtr(r.FormValue("weight_g")), r.FormValue("notes")).Scan(&id)
		if err != nil {
			httpErr(w, err)
			return
		}
		a.db.Exec(ctx, `INSERT INTO item_counts (item_type,item_id,count) VALUES ('vtx',$1,$2)`, id, qty)
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	page := VTXFormPage{ActiveTab: "inventory"}
	page.Brands, _ = a.brandOptions(r)
	render(w, "vtx-form", page)
}

func (a *App) handleVTXEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			page := VTXFormPage{ActiveTab: "inventory", Error: "Name is required", ID: id}
			page.Brands, _ = a.brandOptions(r)
			render(w, "vtx-form", page)
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		_, err := a.db.Exec(ctx,
			`UPDATE vtx_units SET brand_id=$1,name=$2,weight_g=$3,notes=$4 WHERE id=$5`,
			nullIntPtr(r.FormValue("brand_id")), name,
			nullIntPtr(r.FormValue("weight_g")), r.FormValue("notes"), id)
		if err != nil {
			httpErr(w, err)
			return
		}
		a.db.Exec(ctx,
			`INSERT INTO item_counts (item_type,item_id,count) VALUES ('vtx',$1,$2)
             ON CONFLICT (item_type,item_id) DO UPDATE SET count=$2`, id, qty)
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	var page VTXFormPage
	var brandID, wg *int
	var qty int
	err := a.db.QueryRow(ctx,
		`SELECT v.id,v.brand_id,v.name,v.weight_g,v.notes,COALESCE(ic.count,0)
         FROM vtx_units v LEFT JOIN item_counts ic ON ic.item_type='vtx' AND ic.item_id=v.id
         WHERE v.id=$1`, id).
		Scan(&page.ID, &brandID, &page.Name, &wg, &page.Notes, &qty)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	page.BrandID = derefInt(brandID)
	if wg != nil {
		page.WeightG = strconv.Itoa(*wg)
	}
	page.Quantity = strconv.Itoa(qty)
	page.ActiveTab = "inventory"
	page.Brands, _ = a.brandOptions(r)
	page.Photos = a.fetchItemPhotos(ctx, "vtx_photos", "vtx_id", id)
	render(w, "vtx-form", page)
}

func (a *App) handleVTXDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(ctx, `DELETE FROM item_counts WHERE item_type='vtx' AND item_id=$1`, id)
	a.db.Exec(ctx, `DELETE FROM vtx_units WHERE id=$1`, id)
	http.Redirect(w, r, "/inventory", http.StatusSeeOther)
}

// ---- Propellers ----

func (a *App) handleProps(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx, `
        SELECT p.id, COALESCE(br.name,''), p.name,
               COALESCE(sz.label,''), CAST(p.pitch AS FLOAT8),
               p.blade_count, p.quantity, p.reorder_threshold,
               COALESCE(string_agg(d.name, ', ' ORDER BY d.name),''), COALESCE(pp.id,0)
        FROM propellers p
        LEFT JOIN brands br ON br.id=p.brand_id
        LEFT JOIN sizes sz ON sz.id=p.size_id
        LEFT JOIN drone_props dp ON dp.prop_id=p.id
        LEFT JOIN drones d ON d.id=dp.drone_id
        LEFT JOIN LATERAL (SELECT id FROM prop_photos WHERE prop_id=p.id ORDER BY created_at LIMIT 1) pp ON true
        GROUP BY p.id, br.name, p.name, sz.label, p.pitch, p.blade_count, p.quantity, p.reorder_threshold, pp.id
        ORDER BY br.name, p.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	defer rows.Close()
	var props []PropRow
	for rows.Next() {
		var p PropRow
		var pitch *float64
		if err := rows.Scan(&p.ID, &p.Brand, &p.Name, &p.SizeInch, &pitch,
			&p.BladeCount, &p.Quantity, &p.ReorderThreshold, &p.DroneNames, &p.FirstPhotoID); err != nil {
			httpErr(w, err)
			return
		}
		if pitch != nil {
			p.Pitch = fmt.Sprintf("%.1f", *pitch)
		}
		p.LowStock = p.ReorderThreshold > 0 && p.Quantity <= p.ReorderThreshold
		props = append(props, p)
	}
	if err := rows.Err(); err != nil {
		httpErr(w, err)
		return
	}
	render(w, "prop-list", PropListPage{ActiveTab: "props", Propellers: props})
}

func (a *App) handlePropNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			page := PropFormPage{ActiveTab: "props", Error: "Name is required", BladeCount: "3"}
			page.Brands, _ = a.brandOptions(r)
			page.Sizes, _ = a.sizeOptions(r)
			render(w, "prop-form", page)
			return
		}
		bc, _ := strconv.Atoi(r.FormValue("blade_count"))
		if bc == 0 {
			bc = 3
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		rt, _ := strconv.Atoi(r.FormValue("reorder_threshold"))
		_, err := a.db.Exec(ctx,
			`INSERT INTO propellers (brand_id,name,size_id,pitch,blade_count,quantity,reorder_threshold,notes)
             VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			nullIntPtr(r.FormValue("brand_id")), name,
			nullIntPtr(r.FormValue("size_id")), nullFloat64(r.FormValue("pitch")),
			bc, qty, rt, r.FormValue("notes"))
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/props", http.StatusSeeOther)
		return
	}
	page := PropFormPage{ActiveTab: "props", BladeCount: "3"}
	page.Brands, _ = a.brandOptions(r)
	page.Sizes, _ = a.sizeOptions(r)
	render(w, "prop-form", page)
}

func (a *App) handlePropEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			page := PropFormPage{ActiveTab: "props", Error: "Name is required", ID: id}
			page.Brands, _ = a.brandOptions(r)
			render(w, "prop-form", page)
			return
		}
		bc, _ := strconv.Atoi(r.FormValue("blade_count"))
		if bc == 0 {
			bc = 3
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		rt, _ := strconv.Atoi(r.FormValue("reorder_threshold"))
		_, err := a.db.Exec(ctx,
			`UPDATE propellers SET brand_id=$1,name=$2,size_id=$3,pitch=$4,blade_count=$5,
             quantity=$6,reorder_threshold=$7,notes=$8 WHERE id=$9`,
			nullIntPtr(r.FormValue("brand_id")), name,
			nullIntPtr(r.FormValue("size_id")), nullFloat64(r.FormValue("pitch")),
			bc, qty, rt, r.FormValue("notes"), id)
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/props", http.StatusSeeOther)
		return
	}
	var page PropFormPage
	var brandID, sizeID *int
	var pitch *float64
	var bladeCount, qty, rt int
	err := a.db.QueryRow(ctx,
		`SELECT id,brand_id,name,size_id,CAST(pitch AS FLOAT8),blade_count,
         quantity,reorder_threshold,notes FROM propellers WHERE id=$1`, id).
		Scan(&page.ID, &brandID, &page.Name, &sizeID, &pitch,
			&bladeCount, &qty, &rt, &page.Notes)
	page.BladeCount = strconv.Itoa(bladeCount)
	page.Quantity = strconv.Itoa(qty)
	page.ReorderThreshold = strconv.Itoa(rt)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	page.BrandID = derefInt(brandID)
	page.SizeID = derefInt(sizeID)
	if pitch != nil {
		page.Pitch = fmt.Sprintf("%.1f", *pitch)
	}
	page.ActiveTab = "props"
	page.Brands, _ = a.brandOptions(r)
	page.Sizes, _ = a.sizeOptions(r)
	page.Photos = a.fetchItemPhotos(ctx, "prop_photos", "prop_id", id)
	render(w, "prop-form", page)
}

func (a *App) handlePropDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(r.Context(), `DELETE FROM propellers WHERE id=$1`, id)
	http.Redirect(w, r, "/props", http.StatusSeeOther)
}

// ---- Batteries ----

func (a *App) handleBatteries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx, `
        SELECT b.id, COALESCE(br.name,''), b.name, COALESCE(c.label,''), b.capacity_mah, b.weight_g,
               b.count,
               COALESCE(string_agg(d.name, ', ' ORDER BY d.name), ''),
               COALESCE(bp.id,0)
        FROM batteries b
        LEFT JOIN brands br ON br.id=b.brand_id
        LEFT JOIN cells c ON c.id=b.cell_id
        LEFT JOIN drone_batteries db ON db.battery_id=b.id
        LEFT JOIN drones d ON d.id=db.drone_id
        LEFT JOIN LATERAL (SELECT id FROM battery_photos WHERE battery_id=b.id ORDER BY created_at LIMIT 1) bp ON true
        GROUP BY b.id, br.name, b.name, c.label, b.capacity_mah, b.weight_g, b.count, bp.id
        ORDER BY br.name, b.name, c.label, b.capacity_mah`)
	if err != nil {
		httpErr(w, err)
		return
	}
	defer rows.Close()
	var bats []BatteryRow
	for rows.Next() {
		var b BatteryRow
		if err := rows.Scan(&b.ID, &b.Brand, &b.Name, &b.CellLabel, &b.CapacityMAh, &b.WeightG,
			&b.Total, &b.AssignedTo, &b.FirstPhotoID); err != nil {
			httpErr(w, err)
			return
		}
		bats = append(bats, b)
	}
	if err := rows.Err(); err != nil {
		httpErr(w, err)
		return
	}
	render(w, "battery-list", BatteryListPage{ActiveTab: "batteries", Batteries: bats})
}

func (a *App) handleBatteryNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		cap, _ := strconv.Atoi(r.FormValue("capacity_mah"))
		if name == "" || cap == 0 {
			page := BatteryFormPage{ActiveTab: "batteries", Error: "Name and capacity are required"}
			page.Brands, _ = a.brandOptions(r)
			page.Cells, _ = a.cellOptions(r)
			render(w, "battery-form", page)
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		if qty == 0 {
			qty = 1
		}
		_, err := a.db.Exec(ctx,
			`INSERT INTO batteries (brand_id,name,cell_id,capacity_mah,weight_g,count,notes)
             VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			nullIntPtr(r.FormValue("brand_id")), name,
			nullIntPtr(r.FormValue("cell_id")), cap,
			nullIntPtr(r.FormValue("weight_g")), qty, r.FormValue("notes"))
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/batteries", http.StatusSeeOther)
		return
	}
	page := BatteryFormPage{ActiveTab: "batteries"}
	page.Brands, _ = a.brandOptions(r)
	page.Cells, _ = a.cellOptions(r)
	render(w, "battery-form", page)
}

func (a *App) handleBatteryEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		cap, _ := strconv.Atoi(r.FormValue("capacity_mah"))
		if name == "" || cap == 0 {
			page := BatteryFormPage{ActiveTab: "batteries", Error: "Name and capacity are required", ID: id}
			page.Brands, _ = a.brandOptions(r)
			page.Cells, _ = a.cellOptions(r)
			render(w, "battery-form", page)
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		if qty == 0 {
			qty = 1
		}
		_, err := a.db.Exec(ctx,
			`UPDATE batteries SET brand_id=$1,name=$2,cell_id=$3,capacity_mah=$4,weight_g=$5,count=$6,notes=$7 WHERE id=$8`,
			nullIntPtr(r.FormValue("brand_id")), name,
			nullIntPtr(r.FormValue("cell_id")), cap,
			nullIntPtr(r.FormValue("weight_g")), qty, r.FormValue("notes"), id)
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/batteries", http.StatusSeeOther)
		return
	}
	var page BatteryFormPage
	var capMAh, qty int
	var brandID, cellID, weightG *int
	err := a.db.QueryRow(ctx,
		`SELECT id,brand_id,name,cell_id,capacity_mah,weight_g,count,notes FROM batteries WHERE id=$1`, id).
		Scan(&page.ID, &brandID, &page.Name, &cellID, &capMAh, &weightG, &qty, &page.Notes)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	page.BrandID = derefInt(brandID)
	page.CellID = derefInt(cellID)
	page.CapacityMAh = strconv.Itoa(capMAh)
	if weightG != nil {
		page.WeightG = strconv.Itoa(*weightG)
	}
	page.Quantity = strconv.Itoa(qty)
	page.ActiveTab = "batteries"
	page.Brands, _ = a.brandOptions(r)
	page.Cells, _ = a.cellOptions(r)
	page.Photos = a.fetchItemPhotos(ctx, "battery_photos", "battery_id", id)
	render(w, "battery-form", page)
}

func (a *App) handleBatteryDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(r.Context(), `DELETE FROM batteries WHERE id=$1`, id)
	http.Redirect(w, r, "/batteries", http.StatusSeeOther)
}

func (a *App) handleBatteryAdjust(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	n, ok := adjustCount(r)
	if !ok {
		http.Redirect(w, r, "/batteries", http.StatusSeeOther)
		return
	}
	a.db.Exec(ctx, `UPDATE batteries SET count=GREATEST(0,count+$1) WHERE id=$2`, n, id)
	http.Redirect(w, r, "/batteries", http.StatusSeeOther)
}

// ---- Sessions / Log ----

func (a *App) handleLog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx, `
        SELECT s.id, s.title, s.type,
               TO_CHAR(s.session_date,'YYYY-MM-DD'),
               s.duration_min, s.location, s.notes,
               COALESCE((SELECT string_agg(d.name,', ' ORDER BY d.name)
                         FROM session_drones sd JOIN drones d ON d.id=sd.drone_id
                         WHERE sd.session_id=s.id),''),
               COALESCE((SELECT string_agg(b.name||' ('||COALESCE(c.label,'?')||')',', ' ORDER BY b.name)
                         FROM session_batteries sb JOIN batteries b ON b.id=sb.battery_id
                         LEFT JOIN cells c ON c.id=b.cell_id
                         WHERE sb.session_id=s.id),''),
               (s.notes=$1 AND s.title='')
        FROM sessions s
        ORDER BY s.session_date DESC, s.created_at DESC`, quickFlightLabel)
	if err != nil {
		httpErr(w, err)
		return
	}
	defer rows.Close()
	var sessions []SessionRow
	for rows.Next() {
		var s SessionRow
		if err := rows.Scan(&s.ID, &s.Title, &s.Type, &s.SessionDate,
			&s.DurationMin, &s.Location, &s.Notes, &s.DroneNames, &s.BatteryList, &s.IsQuickFlight); err != nil {
			httpErr(w, err)
			return
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		httpErr(w, err)
		return
	}
	render(w, "log-list", LogListPage{ActiveTab: "log", Sessions: sessions})
}

func (a *App) handleSessionNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		droneIDs := parseIntList(r.Form["drone_ids"])
		if len(droneIDs) == 0 {
			page := SessionFormPage{ActiveTab: "log", Error: "Select at least one drone", Type: "flight"}
			page.Drones, _ = a.droneChecks(r, 0)
			page.Batteries, _ = a.batteryChecks(r, 0)
			page.Places, _ = a.placeOptions(r)
			render(w, "session-form", page)
			return
		}
		dur, _ := strconv.Atoi(r.FormValue("duration_min"))

		tx, err := a.db.Begin(ctx)
		if err != nil {
			httpErr(w, err)
			return
		}
		defer tx.Rollback(ctx)

		var sessionID int
		err = tx.QueryRow(ctx,
			`INSERT INTO sessions (title,type,session_date,duration_min,location,notes)
             VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
			r.FormValue("title"), r.FormValue("type"), nullDate(r.FormValue("session_date")), dur,
			r.FormValue("location"), r.FormValue("notes")).Scan(&sessionID)
		if err != nil {
			httpErr(w, err)
			return
		}
		for _, did := range droneIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO session_drones (session_id,drone_id) VALUES ($1,$2)`, sessionID, did); err != nil {
				httpErr(w, err)
				return
			}
		}
		for _, batIDStr := range r.Form["battery_ids"] {
			batID, err := strconv.Atoi(batIDStr)
			if err != nil {
				continue
			}
			cnt, _ := strconv.Atoi(r.FormValue(fmt.Sprintf("battery_count_%d", batID)))
			if cnt < 1 {
				cnt = 1
			}
			if _, err := tx.Exec(ctx, `INSERT INTO session_batteries (session_id,battery_id,count) VALUES ($1,$2,$3)`, sessionID, batID, cnt); err != nil {
				httpErr(w, err)
				return
			}
		}
		if err := tx.Commit(ctx); err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/log", http.StatusSeeOther)
		return
	}
	page := SessionFormPage{ActiveTab: "log", Type: "flight", SessionDate: time.Now().Format("2006-01-02")}
	page.Drones, _ = a.droneChecks(r, 0)
	page.Batteries, _ = a.batteryChecks(r, 0)
	page.Places, _ = a.placeOptions(r)
	render(w, "session-form", page)
}

func (a *App) handleSessionDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var page SessionDetailPage
	err := a.db.QueryRow(ctx, `
        SELECT s.id, s.title, s.type, TO_CHAR(s.session_date,'YYYY-MM-DD'),
               s.duration_min, s.location, s.notes,
               COALESCE((SELECT string_agg(d.name,', ' ORDER BY d.name)
                         FROM session_drones sd JOIN drones d ON d.id=sd.drone_id
                         WHERE sd.session_id=s.id),'')
        FROM sessions s WHERE s.id=$1`, id).
		Scan(&page.ID, &page.Title, &page.Type, &page.Date,
			&page.Duration, &page.Location, &page.Notes, &page.DroneNames)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	rows, err := a.db.Query(ctx, `
        SELECT b.id, COALESCE(br.name,''), b.name, COALESCE(c.label,''), b.capacity_mah, sb.count
        FROM batteries b JOIN session_batteries sb ON sb.battery_id=b.id
        LEFT JOIN brands br ON br.id=b.brand_id
        LEFT JOIN cells c ON c.id=b.cell_id
        WHERE sb.session_id=$1 ORDER BY br.name, b.name`, id)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var b BatteryRow
		if err := rows.Scan(&b.ID, &b.Brand, &b.Name, &b.CellLabel, &b.CapacityMAh, &b.Count); err != nil {
			rows.Close()
			httpErr(w, err)
			return
		}
		page.Batteries = append(page.Batteries, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpErr(w, err)
		return
	}
	// session drones for video drone picker
	droneOptRows, _ := a.db.Query(ctx,
		`SELECT d.id, d.name FROM drones d JOIN session_drones sd ON sd.drone_id=d.id
         WHERE sd.session_id=$1 ORDER BY d.name`, id)
	if droneOptRows != nil {
		for droneOptRows.Next() {
			var o OptionItem
			droneOptRows.Scan(&o.ID, &o.Label)
			page.Drones = append(page.Drones, o)
		}
		droneOptRows.Close()
	}

	rows, err = a.db.Query(ctx,
		`SELECT sv.id, sv.original_name, sv.notes, COALESCE(sv.drone_id,0), COALESCE(d.name,'')
         FROM session_videos sv LEFT JOIN drones d ON d.id=sv.drone_id
         WHERE sv.session_id=$1 ORDER BY sv.created_at`, id)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var v VideoRow
		if err := rows.Scan(&v.ID, &v.OriginalName, &v.Notes, &v.DroneID, &v.DroneName); err != nil {
			rows.Close()
			httpErr(w, err)
			return
		}
		page.Videos = append(page.Videos, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpErr(w, err)
		return
	}
	rows, err = a.db.Query(ctx,
		`SELECT id, original_name, notes FROM session_photos WHERE session_id=$1 ORDER BY created_at`, id)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var p PhotoRow
		if err := rows.Scan(&p.ID, &p.OriginalName, &p.Notes); err != nil {
			rows.Close()
			httpErr(w, err)
			return
		}
		page.Photos = append(page.Photos, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpErr(w, err)
		return
	}
	page.ActiveTab = "log"
	render(w, "session-detail", page)
}

func (a *App) handleSessionEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		droneIDs := parseIntList(r.Form["drone_ids"])
		if len(droneIDs) == 0 {
			page := SessionFormPage{ActiveTab: "log", Error: "Select at least one drone", ID: id}
			page.Drones, _ = a.droneChecks(r, id)
			page.Batteries, _ = a.batteryChecks(r, id)
			page.Places, _ = a.placeOptions(r)
			render(w, "session-form", page)
			return
		}
		dur, _ := strconv.Atoi(r.FormValue("duration_min"))
		newBatIDs := parseIntList(r.Form["battery_ids"])

		tx, err := a.db.Begin(ctx)
		if err != nil {
			httpErr(w, err)
			return
		}
		defer tx.Rollback(ctx)

		if _, err := tx.Exec(ctx, `DELETE FROM session_drones WHERE session_id=$1`, id); err != nil {
			httpErr(w, err)
			return
		}
		for _, did := range droneIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO session_drones (session_id,drone_id) VALUES ($1,$2)`, id, did); err != nil {
				httpErr(w, err)
				return
			}
		}
		if _, err := tx.Exec(ctx, `DELETE FROM session_batteries WHERE session_id=$1`, id); err != nil {
			httpErr(w, err)
			return
		}
		for _, bid := range newBatIDs {
			cnt, _ := strconv.Atoi(r.FormValue(fmt.Sprintf("battery_count_%d", bid)))
			if cnt < 1 {
				cnt = 1
			}
			if _, err := tx.Exec(ctx, `INSERT INTO session_batteries (session_id,battery_id,count) VALUES ($1,$2,$3)`, id, bid, cnt); err != nil {
				httpErr(w, err)
				return
			}
		}
		_, err = tx.Exec(ctx,
			`UPDATE sessions SET title=$1,type=$2,session_date=$3,duration_min=$4,location=$5,notes=$6 WHERE id=$7`,
			r.FormValue("title"), r.FormValue("type"), nullDate(r.FormValue("session_date")), dur,
			r.FormValue("location"), r.FormValue("notes"), id)
		if err != nil {
			httpErr(w, err)
			return
		}
		if err := tx.Commit(ctx); err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/log/%d", id), http.StatusSeeOther)
		return
	}
	var page SessionFormPage
	var dur int
	err := a.db.QueryRow(ctx,
		`SELECT id,title,type,TO_CHAR(session_date,'YYYY-MM-DD'),duration_min,location,notes FROM sessions WHERE id=$1`, id).
		Scan(&page.ID, &page.Title, &page.Type, &page.SessionDate, &dur, &page.Location, &page.Notes)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	page.DurationMin = strconv.Itoa(dur)
	page.ActiveTab = "log"
	page.Drones, _ = a.droneChecks(r, id)
	page.Batteries, _ = a.batteryChecks(r, id)
	page.Places, _ = a.placeOptions(r)
	render(w, "session-form", page)
}

func (a *App) handleSessionDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	tx, err := a.db.Begin(ctx)
	if err != nil {
		httpErr(w, err)
		return
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE id=$1`, id); err != nil {
		httpErr(w, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		httpErr(w, err)
		return
	}
	http.Redirect(w, r, "/log", http.StatusSeeOther)
}

// ---- GPS Modules ----

func (a *App) handleGPSNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			page := GPSFormPage{ActiveTab: "inventory", Error: "Name is required"}
			page.Brands, _ = a.brandOptions(r)
			render(w, "gps-form", page)
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		var id int
		err := a.db.QueryRow(ctx,
			`INSERT INTO gps_modules (brand_id,name,notes) VALUES ($1,$2,$3) RETURNING id`,
			nullIntPtr(r.FormValue("brand_id")), name, r.FormValue("notes")).Scan(&id)
		if err != nil {
			httpErr(w, err)
			return
		}
		a.db.Exec(ctx, `INSERT INTO item_counts (item_type,item_id,count) VALUES ('gps',$1,$2)`, id, qty)
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	page := GPSFormPage{ActiveTab: "inventory"}
	page.Brands, _ = a.brandOptions(r)
	render(w, "gps-form", page)
}

func (a *App) handleGPSEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			page := GPSFormPage{ActiveTab: "inventory", Error: "Name is required", ID: id}
			page.Brands, _ = a.brandOptions(r)
			render(w, "gps-form", page)
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		_, err := a.db.Exec(ctx,
			`UPDATE gps_modules SET brand_id=$1,name=$2,notes=$3 WHERE id=$4`,
			nullIntPtr(r.FormValue("brand_id")), name, r.FormValue("notes"), id)
		if err != nil {
			httpErr(w, err)
			return
		}
		a.db.Exec(ctx,
			`INSERT INTO item_counts (item_type,item_id,count) VALUES ('gps',$1,$2)
             ON CONFLICT (item_type,item_id) DO UPDATE SET count=$2`, id, qty)
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	var page GPSFormPage
	var brandID *int
	var qty int
	err := a.db.QueryRow(ctx,
		`SELECT g.id,g.brand_id,g.name,g.notes,COALESCE(ic.count,0)
         FROM gps_modules g LEFT JOIN item_counts ic ON ic.item_type='gps' AND ic.item_id=g.id
         WHERE g.id=$1`, id).
		Scan(&page.ID, &brandID, &page.Name, &page.Notes, &qty)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	page.BrandID = derefInt(brandID)
	page.Quantity = strconv.Itoa(qty)
	page.ActiveTab = "inventory"
	page.Brands, _ = a.brandOptions(r)
	page.Photos = a.fetchItemPhotos(ctx, "gps_photos", "gps_id", id)
	render(w, "gps-form", page)
}

func (a *App) handleGPSDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(ctx, `DELETE FROM item_counts WHERE item_type='gps' AND item_id=$1`, id)
	a.db.Exec(ctx, `DELETE FROM gps_modules WHERE id=$1`, id)
	http.Redirect(w, r, "/inventory", http.StatusSeeOther)
}

// ---- Radio Receivers ----

func (a *App) handleRXNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			page := RXFormPage{ActiveTab: "inventory", Error: "Name is required"}
			page.Brands, _ = a.brandOptions(r)
			render(w, "rx-form", page)
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		var id int
		err := a.db.QueryRow(ctx,
			`INSERT INTO radio_receivers (brand_id,name,protocol_id,notes) VALUES ($1,$2,$3,$4) RETURNING id`,
			nullIntPtr(r.FormValue("brand_id")), name, nullIntPtr(r.FormValue("protocol_id")), r.FormValue("notes")).Scan(&id)
		if err != nil {
			httpErr(w, err)
			return
		}
		a.db.Exec(ctx, `INSERT INTO item_counts (item_type,item_id,count) VALUES ('rx',$1,$2)`, id, qty)
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	page := RXFormPage{ActiveTab: "inventory"}
	page.Brands, _ = a.brandOptions(r)
	page.Protocols, _ = a.radioProtocolOptions(r)
	render(w, "rx-form", page)
}

func (a *App) handleRXEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			page := RXFormPage{ActiveTab: "inventory", Error: "Name is required", ID: id}
			page.Brands, _ = a.brandOptions(r)
			render(w, "rx-form", page)
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		_, err := a.db.Exec(ctx,
			`UPDATE radio_receivers SET brand_id=$1,name=$2,protocol_id=$3,notes=$4 WHERE id=$5`,
			nullIntPtr(r.FormValue("brand_id")), name, nullIntPtr(r.FormValue("protocol_id")), r.FormValue("notes"), id)
		if err != nil {
			httpErr(w, err)
			return
		}
		a.db.Exec(ctx,
			`INSERT INTO item_counts (item_type,item_id,count) VALUES ('rx',$1,$2)
             ON CONFLICT (item_type,item_id) DO UPDATE SET count=$2`, id, qty)
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	var page RXFormPage
	var brandID, protocolID *int
	var qty int
	err := a.db.QueryRow(ctx,
		`SELECT rx.id,rx.brand_id,rx.name,rx.protocol_id,rx.notes,COALESCE(ic.count,0)
         FROM radio_receivers rx LEFT JOIN item_counts ic ON ic.item_type='rx' AND ic.item_id=rx.id
         WHERE rx.id=$1`, id).
		Scan(&page.ID, &brandID, &page.Name, &protocolID, &page.Notes, &qty)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	page.BrandID = derefInt(brandID)
	page.ProtocolID = derefInt(protocolID)
	page.Quantity = strconv.Itoa(qty)
	page.ActiveTab = "inventory"
	page.Brands, _ = a.brandOptions(r)
	page.Protocols, _ = a.radioProtocolOptions(r)
	page.Photos = a.fetchItemPhotos(ctx, "rx_photos", "rx_id", id)
	render(w, "rx-form", page)
}

func (a *App) handleRXDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(ctx, `DELETE FROM item_counts WHERE item_type='rx' AND item_id=$1`, id)
	a.db.Exec(ctx, `DELETE FROM radio_receivers WHERE id=$1`, id)
	http.Redirect(w, r, "/inventory", http.StatusSeeOther)
}

// ---- Adjust handlers — update item_counts.count by delta ----

func adjustCount(r *http.Request) (int, bool) {
	if err := r.ParseForm(); err != nil {
		return 0, false
	}
	n, err := strconv.Atoi(r.FormValue("count"))
	return n, err == nil && n != 0
}

func (a *App) adjustItemCount(w http.ResponseWriter, r *http.Request, itemType string) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	n, ok := adjustCount(r)
	if !ok {
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	_, err := a.db.Exec(ctx, `
        INSERT INTO item_counts (item_type,item_id,count) VALUES ($1,$2,GREATEST(0,$3))
        ON CONFLICT (item_type,item_id) DO UPDATE
        SET count = GREATEST(0, item_counts.count + $3)`, itemType, id, n)
	if err != nil {
		httpErr(w, err)
		return
	}
	http.Redirect(w, r, "/inventory", http.StatusSeeOther)
}

func (a *App) handleFrameAdjust(w http.ResponseWriter, r *http.Request) {
	a.adjustItemCount(w, r, "frame")
}
func (a *App) handleFCAdjust(w http.ResponseWriter, r *http.Request) {
	a.adjustItemCount(w, r, "fc")
}
func (a *App) handleESCAdjust(w http.ResponseWriter, r *http.Request) {
	a.adjustItemCount(w, r, "esc")
}
func (a *App) handleMotorAdjust(w http.ResponseWriter, r *http.Request) {
	a.adjustItemCount(w, r, "motor")
}
func (a *App) handleVTXAdjust(w http.ResponseWriter, r *http.Request) {
	a.adjustItemCount(w, r, "vtx")
}
func (a *App) handleGPSAdjust(w http.ResponseWriter, r *http.Request) {
	a.adjustItemCount(w, r, "gps")
}
func (a *App) handleRXAdjust(w http.ResponseWriter, r *http.Request) {
	a.adjustItemCount(w, r, "rx")
}

// ---- Videos ----

func (a *App) handleVideoUpload(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	if err := r.ParseMultipartForm(4 << 30); err != nil {
		httpErr(w, err)
		return
	}
	file, header, err := r.FormFile("video")
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/log/%d", id), http.StatusSeeOther)
		return
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	dst := filepath.Join(a.videoDir, filename)
	out, err := os.Create(dst)
	if err != nil {
		httpErr(w, err)
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		os.Remove(dst)
		httpErr(w, err)
		return
	}
	out.Close()

	_, err = a.db.Exec(ctx,
		`INSERT INTO session_videos (session_id,filename,original_name,drone_id) VALUES ($1,$2,$3,$4)`,
		id, filename, header.Filename, nullIntPtr(r.FormValue("drone_id")))
	if err != nil {
		os.Remove(dst)
		httpErr(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/log/%d", id), http.StatusSeeOther)
}

func (a *App) handleVideoServe(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var filename string
	err := a.db.QueryRow(r.Context(), `SELECT filename FROM session_videos WHERE id=$1`, id).Scan(&filename)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(a.videoDir, filename))
}

func (a *App) handleMobilePlaylist(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if a.ffmpegPath == "" {
		http.Error(w, "transcoding not configured", http.StatusServiceUnavailable)
		return
	}
	var filename string
	err := a.db.QueryRow(r.Context(), `SELECT filename FROM session_videos WHERE id=$1`, id).Scan(&filename)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	srcPath := filepath.Join(a.videoDir, filename)
	ffprobePath := filepath.Join(filepath.Dir(a.ffmpegPath), "ffprobe")
	out, err := exec.CommandContext(r.Context(), ffprobePath,
		"-v", "quiet", "-show_entries", "format=duration", "-of", "csv=p=0", srcPath,
	).Output()
	if err != nil {
		http.Error(w, "could not probe video", http.StatusInternalServerError)
		return
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || duration <= 0 {
		http.Error(w, "invalid duration", http.StatusInternalServerError)
		return
	}

	cfg := a.loadConfig(r.Context())
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	b.WriteString("#EXT-X-VERSION:3\n")
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n", cfg.HLSSegmentSec)
	b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	b.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	fullSegments := int(duration) / cfg.HLSSegmentSec
	lastDur := duration - float64(fullSegments*cfg.HLSSegmentSec)
	for i := range fullSegments {
		if i > 0 {
			b.WriteString("#EXT-X-DISCONTINUITY\n")
		}
		fmt.Fprintf(&b, "#EXTINF:%d.000,\nmobile/%d.ts\n", cfg.HLSSegmentSec, i)
	}
	if lastDur > 0.05 {
		if fullSegments > 0 {
			b.WriteString("#EXT-X-DISCONTINUITY\n")
		}
		fmt.Fprintf(&b, "#EXTINF:%.3f,\nmobile/%d.ts\n", lastDur, fullSegments)
	}
	b.WriteString("#EXT-X-ENDLIST\n")

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Write([]byte(b.String()))
}

func (a *App) handleMobileSegment(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if a.ffmpegPath == "" {
		http.Error(w, "transcoding not configured", http.StatusServiceUnavailable)
		return
	}
	name := r.PathValue("name")
	if !strings.HasSuffix(name, ".ts") {
		http.NotFound(w, r)
		return
	}
	n, err := strconv.Atoi(strings.TrimSuffix(name, ".ts"))
	if err != nil || n < 0 {
		http.NotFound(w, r)
		return
	}
	var filename string
	err = a.db.QueryRow(r.Context(), `SELECT filename FROM session_videos WHERE id=$1`, id).Scan(&filename)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	srcPath := filepath.Join(a.videoDir, filename)
	cfg := a.loadConfig(r.Context())
	startSec := n * cfg.HLSSegmentSec
	bitrateStr := strconv.Itoa(cfg.VideoBitrateK) + "k"

	cmd := exec.CommandContext(r.Context(), a.ffmpegPath,
		"-y",
		"-ss", strconv.Itoa(startSec),
		"-i", srcPath,
		"-t", strconv.Itoa(cfg.HLSSegmentSec),
		"-c:v", "libx264", "-crf", strconv.Itoa(cfg.FFmpegCRF), "-preset", cfg.FFmpegPreset,
		"-profile:v", "main", "-level", "4.0",
		"-pix_fmt", "yuv420p",
		"-bf", "0",
		"-vf", fmt.Sprintf("scale='min(%d,iw)':-2", cfg.VideoMaxWidth),
		"-b:v", bitrateStr, "-maxrate", bitrateStr, "-bufsize", strconv.Itoa(cfg.VideoBitrateK*2)+"k",
		"-c:a", "aac", "-b:a", "128k",
		"-muxdelay", "0", "-muxpreload", "0",
		"-f", "mpegts", "pipe:1",
	)
	var stderr bytes.Buffer
	cmd.Stdout = w
	cmd.Stderr = &stderr
	w.Header().Set("Content-Type", "video/mp2t")
	if err := cmd.Run(); err != nil {
		log.Printf("transcode segment %d/%d: %v\n%s", id, n, err, stderr.String())
	}
}

func (a *App) handleVideoDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	var filename string
	var sessionID int
	err := a.db.QueryRow(ctx,
		`SELECT session_id,filename FROM session_videos WHERE id=$1`, id).Scan(&sessionID, &filename)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(ctx, `DELETE FROM session_videos WHERE id=$1`, id)
	os.Remove(filepath.Join(a.videoDir, filename))
	http.Redirect(w, r, fmt.Sprintf("/log/%d", sessionID), http.StatusSeeOther)
}

func (a *App) handleVideoNote(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		httpErr(w, err)
		return
	}
	var sessionID int
	err := a.db.QueryRow(ctx, `SELECT session_id FROM session_videos WHERE id=$1`, id).Scan(&sessionID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(ctx, `UPDATE session_videos SET notes=$1 WHERE id=$2`, r.FormValue("notes"), id)
	http.Redirect(w, r, fmt.Sprintf("/log/%d", sessionID), http.StatusSeeOther)
}

func (a *App) handleVideoDrone(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var sessionID int
	err := a.db.QueryRow(ctx, `SELECT session_id FROM session_videos WHERE id=$1`, id).Scan(&sessionID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(ctx, `UPDATE session_videos SET drone_id=$1 WHERE id=$2`, nullIntPtr(r.FormValue("drone_id")), id)
	http.Redirect(w, r, fmt.Sprintf("/log/%d", sessionID), http.StatusSeeOther)
}

func (a *App) handlePhotoUpload(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	if err := r.ParseMultipartForm(256 << 20); err != nil {
		httpErr(w, err)
		return
	}
	file, header, err := r.FormFile("photo")
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/log/%d", id), http.StatusSeeOther)
		return
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	dst := filepath.Join(a.videoDir, filename)
	out, err := os.Create(dst)
	if err != nil {
		httpErr(w, err)
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		os.Remove(dst)
		httpErr(w, err)
		return
	}
	out.Close()

	_, err = a.db.Exec(ctx,
		`INSERT INTO session_photos (session_id,filename,original_name) VALUES ($1,$2,$3)`,
		id, filename, header.Filename)
	if err != nil {
		os.Remove(dst)
		httpErr(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/log/%d", id), http.StatusSeeOther)
}

func (a *App) handlePhotoServe(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var filename string
	err := a.db.QueryRow(r.Context(), `SELECT filename FROM session_photos WHERE id=$1`, id).Scan(&filename)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(a.videoDir, filename))
}

func (a *App) handlePhotoDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	var filename string
	var sessionID int
	err := a.db.QueryRow(ctx,
		`SELECT session_id,filename FROM session_photos WHERE id=$1`, id).Scan(&sessionID, &filename)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(ctx, `DELETE FROM session_photos WHERE id=$1`, id)
	os.Remove(filepath.Join(a.videoDir, filename))
	http.Redirect(w, r, fmt.Sprintf("/log/%d", sessionID), http.StatusSeeOther)
}

func (a *App) handlePhotoNote(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		httpErr(w, err)
		return
	}
	var sessionID int
	err := a.db.QueryRow(ctx, `SELECT session_id FROM session_photos WHERE id=$1`, id).Scan(&sessionID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(ctx, `UPDATE session_photos SET notes=$1 WHERE id=$2`, r.FormValue("notes"), id)
	http.Redirect(w, r, fmt.Sprintf("/log/%d", sessionID), http.StatusSeeOther)
}

// ---- Places ----

func geocodeAddress(address string) (lat, lng *float64) {
	req, err := http.NewRequest("GET",
		"https://nominatim.openstreetmap.org/search?format=json&limit=1&q="+url.QueryEscape(address),
		nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "FPV-Manager/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var results []struct {
		Lat string `json:"lat"`
		Lon string `json:"lon"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil || len(results) == 0 {
		return
	}
	la, err := strconv.ParseFloat(results[0].Lat, 64)
	if err != nil {
		return
	}
	lo, err := strconv.ParseFloat(results[0].Lon, 64)
	if err != nil {
		return
	}
	return &la, &lo
}

func (a *App) handlePlaces(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx,
		`SELECT id, name, address, lat, lng, place_type, notes FROM places ORDER BY name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	defer rows.Close()
	var places []PlaceRow
	for rows.Next() {
		var p PlaceRow
		if err := rows.Scan(&p.ID, &p.Name, &p.Address, &p.Lat, &p.Lng, &p.PlaceType, &p.Notes); err != nil {
			httpErr(w, err)
			return
		}
		if p.Lat != nil && p.Lng != nil {
			p.HasCoords = true
			p.LatStr = strconv.FormatFloat(*p.Lat, 'f', 7, 64)
			p.LngStr = strconv.FormatFloat(*p.Lng, 'f', 7, 64)
		}
		places = append(places, p)
	}
	if err := rows.Err(); err != nil {
		httpErr(w, err)
		return
	}
	render(w, "place-list", PlaceListPage{ActiveTab: "places", Places: places})
}

func (a *App) handlePlaceNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		address := strings.TrimSpace(r.FormValue("address"))
		if name == "" || address == "" {
			render(w, "place-form", PlaceFormPage{
				ActiveTab: "places",
				Error:     "Name and address are required",
				Name:      name, Address: address,
				PlaceType: r.FormValue("place_type"),
				Notes:     r.FormValue("notes"),
			})
			return
		}
		lat, lng := geocodeAddress(address)
		var id int
		err := a.db.QueryRow(ctx,
			`INSERT INTO places (name,address,lat,lng,place_type,notes)
             VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
			name, address, lat, lng, r.FormValue("place_type"), r.FormValue("notes")).Scan(&id)
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/places/%d", id), http.StatusSeeOther)
		return
	}
	render(w, "place-form", PlaceFormPage{ActiveTab: "places", PlaceType: "field"})
}

func (a *App) handlePlaceDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var p PlaceRow
	err := a.db.QueryRow(r.Context(),
		`SELECT id, name, address, lat, lng, place_type, notes FROM places WHERE id=$1`, id).
		Scan(&p.ID, &p.Name, &p.Address, &p.Lat, &p.Lng, &p.PlaceType, &p.Notes)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	if p.Lat != nil && p.Lng != nil {
		p.HasCoords = true
		p.LatStr = strconv.FormatFloat(*p.Lat, 'f', 7, 64)
		p.LngStr = strconv.FormatFloat(*p.Lng, 'f', 7, 64)
	}
	render(w, "place-detail", PlaceDetailPage{ActiveTab: "places", PlaceRow: p})
}

func (a *App) handlePlaceEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		address := strings.TrimSpace(r.FormValue("address"))
		if name == "" || address == "" {
			render(w, "place-form", PlaceFormPage{
				ActiveTab: "places",
				Error:     "Name and address are required",
				ID:        id, Name: name, Address: address,
				PlaceType: r.FormValue("place_type"),
				Notes:     r.FormValue("notes"),
			})
			return
		}
		lat, lng := geocodeAddress(address)
		_, err := a.db.Exec(ctx,
			`UPDATE places SET name=$1,address=$2,lat=$3,lng=$4,place_type=$5,notes=$6 WHERE id=$7`,
			name, address, lat, lng, r.FormValue("place_type"), r.FormValue("notes"), id)
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/places/%d", id), http.StatusSeeOther)
		return
	}
	var p PlaceFormPage
	err := a.db.QueryRow(ctx,
		`SELECT id, name, address, place_type, notes FROM places WHERE id=$1`, id).
		Scan(&p.ID, &p.Name, &p.Address, &p.PlaceType, &p.Notes)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	p.ActiveTab = "places"
	render(w, "place-form", p)
}

func (a *App) handlePlaceDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(r.Context(), `DELETE FROM places WHERE id=$1`, id)
	http.Redirect(w, r, "/places", http.StatusSeeOther)
}

// ---- Brands ----

func (a *App) handleBrandNew(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			render(w, "brand-form", BrandFormPage{ActiveTab: "settings", Error: "Name is required"})
			return
		}
		_, err := a.db.Exec(r.Context(), `INSERT INTO brands (name) VALUES ($1)`, name)
		if err != nil {
			render(w, "brand-form", BrandFormPage{ActiveTab: "settings", Error: "Brand name already exists"})
			return
		}
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	render(w, "brand-form", BrandFormPage{ActiveTab: "settings"})
}

func (a *App) handleBrandEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			render(w, "brand-form", BrandFormPage{ActiveTab: "settings", Error: "Name is required", ID: id})
			return
		}
		_, err := a.db.Exec(r.Context(), `UPDATE brands SET name=$1 WHERE id=$2`, name, id)
		if err != nil {
			render(w, "brand-form", BrandFormPage{ActiveTab: "settings", Error: "Brand name already exists", ID: id, Name: name})
			return
		}
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	var page BrandFormPage
	err := a.db.QueryRow(r.Context(), `SELECT id, name FROM brands WHERE id=$1`, id).
		Scan(&page.ID, &page.Name)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	page.ActiveTab = "settings"
	render(w, "brand-form", page)
}

func (a *App) handleBrandDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if _, err := a.db.Exec(r.Context(), `DELETE FROM brands WHERE id=$1`, id); err != nil {
		httpErr(w, err)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// ---- Settings ----

func (a *App) handleSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page := SettingsPage{ActiveTab: "settings"}
	brandRows, _ := a.db.Query(ctx, `SELECT id, name FROM brands ORDER BY name`)
	if brandRows != nil {
		for brandRows.Next() {
			var b BrandRow
			brandRows.Scan(&b.ID, &b.Name)
			page.Brands = append(page.Brands, b)
		}
		brandRows.Close()
	}
	sizeRows, _ := a.db.Query(ctx, `SELECT id, label FROM sizes ORDER BY label::FLOAT`)
	if sizeRows != nil {
		for sizeRows.Next() {
			var s SizeRow
			sizeRows.Scan(&s.ID, &s.Label)
			page.Sizes = append(page.Sizes, s)
		}
		sizeRows.Close()
	}
	cellRows, _ := a.db.Query(ctx, `SELECT id, label FROM cells ORDER BY id`)
	if cellRows != nil {
		for cellRows.Next() {
			var c CellRow
			cellRows.Scan(&c.ID, &c.Label)
			page.Cells = append(page.Cells, c)
		}
		cellRows.Close()
	}
	rpRows, _ := a.db.Query(ctx, `SELECT id, name FROM radio_protocols ORDER BY name`)
	if rpRows != nil {
		for rpRows.Next() {
			var rp RadioProtocolRow
			rpRows.Scan(&rp.ID, &rp.Name)
			page.RadioProtocols = append(page.RadioProtocols, rp)
		}
		rpRows.Close()
	}
	mcuRows, _ := a.db.Query(ctx, `SELECT id, name FROM mcus ORDER BY name`)
	if mcuRows != nil {
		for mcuRows.Next() {
			var m MCURow
			mcuRows.Scan(&m.ID, &m.Name)
			page.MCUs = append(page.MCUs, m)
		}
		mcuRows.Close()
	}
	page.Config = a.loadConfig(ctx)
	render(w, "settings", page)
}

func (a *App) handleConfigSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		httpErr(w, err)
		return
	}
	crf, _ := strconv.Atoi(r.FormValue("ffmpeg_crf"))
	if crf < 0 {
		crf = 0
	} else if crf > 51 {
		crf = 51
	}
	preset := r.FormValue("ffmpeg_preset")
	validPresets := map[string]bool{"ultrafast": true, "superfast": true, "veryfast": true, "faster": true, "fast": true, "medium": true, "slow": true, "slower": true, "veryslow": true}
	if !validPresets[preset] {
		preset = "fast"
	}
	maxW, _ := strconv.Atoi(r.FormValue("video_max_width"))
	if maxW < 480 {
		maxW = 480
	}
	bitrateK, _ := strconv.Atoi(r.FormValue("video_bitrate_k"))
	if bitrateK < 500 {
		bitrateK = 500
	}
	hlsSec, _ := strconv.Atoi(r.FormValue("hls_segment_sec"))
	if hlsSec < 2 {
		hlsSec = 2
	} else if hlsSec > 30 {
		hlsSec = 30
	}
	a.db.Exec(r.Context(), `
		INSERT INTO app_config (id, ffmpeg_crf, ffmpeg_preset, video_max_width, video_bitrate_k, hls_segment_sec)
		VALUES (1, $1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			ffmpeg_crf=$1, ffmpeg_preset=$2, video_max_width=$3, video_bitrate_k=$4, hls_segment_sec=$5`,
		crf, preset, maxW, bitrateK, hlsSec)
	w.WriteHeader(http.StatusNoContent)
}

// ---- Radio Protocols ----

func (a *App) handleRadioProtocolNew(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			render(w, "radio-protocol-form", RadioProtocolFormPage{ActiveTab: "settings", Error: "Name is required"})
			return
		}
		_, err := a.db.Exec(r.Context(), `INSERT INTO radio_protocols (name) VALUES ($1)`, name)
		if err != nil {
			render(w, "radio-protocol-form", RadioProtocolFormPage{ActiveTab: "settings", Error: "Protocol already exists"})
			return
		}
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	render(w, "radio-protocol-form", RadioProtocolFormPage{ActiveTab: "settings"})
}

func (a *App) handleRadioProtocolEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			render(w, "radio-protocol-form", RadioProtocolFormPage{ActiveTab: "settings", Error: "Name is required", ID: id})
			return
		}
		_, err := a.db.Exec(r.Context(), `UPDATE radio_protocols SET name=$1 WHERE id=$2`, name, id)
		if err != nil {
			render(w, "radio-protocol-form", RadioProtocolFormPage{ActiveTab: "settings", Error: "Protocol already exists", ID: id, Name: name})
			return
		}
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	var page RadioProtocolFormPage
	err := a.db.QueryRow(r.Context(), `SELECT id, name FROM radio_protocols WHERE id=$1`, id).
		Scan(&page.ID, &page.Name)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	page.ActiveTab = "settings"
	render(w, "radio-protocol-form", page)
}

func (a *App) handleRadioProtocolDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if _, err := a.db.Exec(r.Context(), `DELETE FROM radio_protocols WHERE id=$1`, id); err != nil {
		httpErr(w, err)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// ---- MCUs ----

func (a *App) handleMCUNew(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			render(w, "mcu-form", MCUFormPage{ActiveTab: "settings", Error: "Name is required"})
			return
		}
		_, err := a.db.Exec(r.Context(), `INSERT INTO mcus (name) VALUES ($1)`, name)
		if err != nil {
			render(w, "mcu-form", MCUFormPage{ActiveTab: "settings", Error: "MCU already exists"})
			return
		}
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	render(w, "mcu-form", MCUFormPage{ActiveTab: "settings"})
}

func (a *App) handleMCUEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			render(w, "mcu-form", MCUFormPage{ActiveTab: "settings", Error: "Name is required", ID: id})
			return
		}
		_, err := a.db.Exec(r.Context(), `UPDATE mcus SET name=$1 WHERE id=$2`, name, id)
		if err != nil {
			render(w, "mcu-form", MCUFormPage{ActiveTab: "settings", Error: "MCU already exists", ID: id, Name: name})
			return
		}
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	var page MCUFormPage
	err := a.db.QueryRow(r.Context(), `SELECT id, name FROM mcus WHERE id=$1`, id).
		Scan(&page.ID, &page.Name)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	page.ActiveTab = "settings"
	render(w, "mcu-form", page)
}

func (a *App) handleMCUDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if _, err := a.db.Exec(r.Context(), `DELETE FROM mcus WHERE id=$1`, id); err != nil {
		httpErr(w, err)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// ---- Sizes ----

func (a *App) sizeOptions(r *http.Request) ([]OptionItem, error) {
	return a.queryOptions(r, `SELECT id, label FROM sizes ORDER BY label::FLOAT`)
}

func (a *App) handleSizeNew(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		label := strings.TrimSpace(r.FormValue("label"))
		if label == "" {
			render(w, "size-form", SizeFormPage{ActiveTab: "settings", Error: "Label is required"})
			return
		}
		_, err := a.db.Exec(r.Context(), `INSERT INTO sizes (label) VALUES ($1)`, label)
		if err != nil {
			render(w, "size-form", SizeFormPage{ActiveTab: "settings", Error: "Size already exists"})
			return
		}
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	render(w, "size-form", SizeFormPage{ActiveTab: "settings"})
}

func (a *App) handleSizeEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		label := strings.TrimSpace(r.FormValue("label"))
		if label == "" {
			render(w, "size-form", SizeFormPage{ActiveTab: "settings", Error: "Label is required", ID: id})
			return
		}
		_, err := a.db.Exec(r.Context(), `UPDATE sizes SET label=$1 WHERE id=$2`, label, id)
		if err != nil {
			render(w, "size-form", SizeFormPage{ActiveTab: "settings", Error: "Size already exists", ID: id, Label: label})
			return
		}
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	var page SizeFormPage
	err := a.db.QueryRow(r.Context(), `SELECT id, label FROM sizes WHERE id=$1`, id).
		Scan(&page.ID, &page.Label)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	page.ActiveTab = "settings"
	render(w, "size-form", page)
}

func (a *App) handleSizeDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if _, err := a.db.Exec(r.Context(), `DELETE FROM sizes WHERE id=$1`, id); err != nil {
		httpErr(w, err)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// ---- Cells ----

func (a *App) cellOptions(r *http.Request) ([]OptionItem, error) {
	return a.queryOptions(r, `SELECT id, label FROM cells ORDER BY id`)
}

func (a *App) handleCellNew(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		label := strings.TrimSpace(r.FormValue("label"))
		if label == "" {
			render(w, "cell-form", CellFormPage{ActiveTab: "settings", Error: "Label is required"})
			return
		}
		_, err := a.db.Exec(r.Context(), `INSERT INTO cells (label) VALUES ($1)`, label)
		if err != nil {
			render(w, "cell-form", CellFormPage{ActiveTab: "settings", Error: "Cell already exists"})
			return
		}
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	render(w, "cell-form", CellFormPage{ActiveTab: "settings"})
}

func (a *App) handleCellEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		label := strings.TrimSpace(r.FormValue("label"))
		if label == "" {
			render(w, "cell-form", CellFormPage{ActiveTab: "settings", Error: "Label is required", ID: id})
			return
		}
		_, err := a.db.Exec(r.Context(), `UPDATE cells SET label=$1 WHERE id=$2`, label, id)
		if err != nil {
			render(w, "cell-form", CellFormPage{ActiveTab: "settings", Error: "Cell already exists", ID: id, Label: label})
			return
		}
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	var page CellFormPage
	err := a.db.QueryRow(r.Context(), `SELECT id, label FROM cells WHERE id=$1`, id).
		Scan(&page.ID, &page.Label)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	page.ActiveTab = "settings"
	render(w, "cell-form", page)
}

func (a *App) handleCellDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if _, err := a.db.Exec(r.Context(), `DELETE FROM cells WHERE id=$1`, id); err != nil {
		httpErr(w, err)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// ---- Weather ----

const (
	bomDailyURL  = "https://api.weather.bom.gov.au/v1/locations/r1r0fup/forecasts/daily"
	bomHourlyURL = "https://api.weather.bom.gov.au/v1/locations/r1r0fu/forecasts/hourly"
)

func bomFetch(rawURL string) ([]byte, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "fpv-manager/1.0")
	c := &http.Client{Timeout: 10 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func flyRating(windKmh, gustKmh, rainChance int) string {
	if windKmh > 35 || gustKmh > 50 || rainChance > 65 {
		return "bad"
	}
	if windKmh > 20 || gustKmh > 30 || rainChance > 35 {
		return "caution"
	}
	return "good"
}

func windColor(kmh int) string {
	if kmh >= 35 {
		return "#f85149"
	}
	if kmh >= 20 {
		return "#d29922"
	}
	return "#3fb950"
}

// rainGuides are fixed threshold lines for the rain chart (35% and 65% rain chance).
var rainGuides = []ChartGuide{
	{TopPct: 35, Label: "65", Color: "rgba(248,81,73,0.55)"},
	{TopPct: 65, Label: "35", Color: "rgba(210,153,34,0.55)"},
}

func (a *App) handleWeather(w http.ResponseWriter, r *http.Request) {
	page := WeatherPage{ActiveTab: "weather"}

	// Fetch both in parallel via goroutines
	type result struct {
		data []byte
		err  error
	}
	dailyCh := make(chan result, 1)
	hourlyCh := make(chan result, 1)
	go func() { b, err := bomFetch(bomDailyURL); dailyCh <- result{b, err} }()
	go func() { b, err := bomFetch(bomHourlyURL); hourlyCh <- result{b, err} }()
	dailyRes := <-dailyCh
	hourlyRes := <-hourlyCh

	if dailyRes.err != nil {
		page.Error = "Could not fetch forecast: " + dailyRes.err.Error()
		render(w, "weather", page)
		return
	}

	// Parse daily
	var daily struct {
		Data []struct {
			Date         string `json:"date"`
			TempMax      *int   `json:"temp_max"`
			TempMin      *int   `json:"temp_min"`
			ShortText    string `json:"short_text"`
			ExtendedText string `json:"extended_text"`
			IconDesc     string `json:"icon_descriptor"`
			Rain         struct {
				Chance int `json:"chance"`
				Amount struct {
					Max *float64 `json:"max"`
				} `json:"amount"`
			} `json:"rain"`
		} `json:"data"`
	}
	if err := json.Unmarshal(dailyRes.data, &daily); err != nil {
		page.Error = "Could not parse daily forecast"
		render(w, "weather", page)
		return
	}

	// Parse hourly — group by local date (AEST = UTC+10)
	aest := time.FixedZone("AEST", 10*60*60)
	type windAgg struct {
		maxWind int
		maxGust int
		dir     string
	}
	dayWind := map[string]windAgg{}

	type rawHr struct{ hour, wind, rain int }
	dayRawHours := map[string][]rawHr{}

	if hourlyRes.err == nil {
		var hourly struct {
			Data []struct {
				Time string `json:"time"`
				Rain struct {
					Chance int `json:"chance"`
				} `json:"rain"`
				Wind struct {
					SpeedKmh int    `json:"speed_kilometre"`
					GustKmh  int    `json:"gust_speed_kilometre"`
					Dir      string `json:"direction"`
				} `json:"wind"`
			} `json:"data"`
		}
		if json.Unmarshal(hourlyRes.data, &hourly) == nil {
			for _, h := range hourly.Data {
				t, err := time.Parse(time.RFC3339, h.Time)
				if err != nil {
					continue
				}
				tLocal := t.In(aest)
				dateKey := tLocal.Format("2006-01-02")

				agg := dayWind[dateKey]
				if h.Wind.SpeedKmh > agg.maxWind {
					agg.maxWind = h.Wind.SpeedKmh
					agg.dir = h.Wind.Dir
				}
				if h.Wind.GustKmh > agg.maxGust {
					agg.maxGust = h.Wind.GustKmh
				}
				dayWind[dateKey] = agg

				if hr := tLocal.Hour(); hr >= 8 && hr <= 17 {
					dayRawHours[dateKey] = append(dayRawHours[dateKey], rawHr{hr, h.Wind.SpeedKmh, h.Rain.Chance})
				}
			}
		}
	}


	now := time.Now().In(aest)
	todayKey := now.Format("2006-01-02")

	for _, d := range daily.Data {
		t, _ := time.Parse(time.RFC3339, d.Date)
		tLocal := t.In(aest)
		dateKey := tLocal.Format("2006-01-02")
		if dateKey == todayKey && now.Hour() > 17 {
			continue
		}

		rainMaxStr := "—"
		if d.Rain.Amount.Max != nil {
			rainMaxStr = fmt.Sprintf("%.0fmm", *d.Rain.Amount.Max)
		}

		tempMax, tempMin := 0, 0
		if d.TempMax != nil {
			tempMax = *d.TempMax
		}
		if d.TempMin != nil {
			tempMin = *d.TempMin
		}

		agg := dayWind[dateKey]
		wd := WeatherDay{
			Date:         tLocal.Format("Mon 02 Jan"),
			RainChance:   d.Rain.Chance,
			RainMaxMm:    rainMaxStr,
			WindSpeedKmh: agg.maxWind,
			GustSpeedKmh: agg.maxGust,
			WindDir:      agg.dir,
			TempMax:      tempMax,
			TempMin:      tempMin,
			ShortText:    d.ShortText,
			ExtendedText: d.ExtendedText,
			IconDesc:     d.IconDesc,
			HasWind:      agg.maxWind > 0,
		}
		wd.FlyRating = flyRating(wd.WindSpeedKmh, wd.GustSpeedKmh, wd.RainChance)
		rawHrs := dayRawHours[dateKey]
		maxWind := 1 // start at 1 to avoid division by zero
		for _, rh := range rawHrs {
			maxWind = max(maxWind, rh.wind)
		}

		// Ceiling is 20% above peak, rounded up to nearest 5
		ceiling := max(((maxWind*6/5)+4)/5*5, 5)

		for _, rh := range rawHrs {
			wd.DayHours = append(wd.DayHours, DayHour{
				Label:    strconv.Itoa(rh.hour),
				WindBarH: rh.wind * 100 / ceiling,
				RainBarH: rh.rain,
				WindFill: windColor(rh.wind),
			})
		}

		wd.WindMaxLabel = strconv.Itoa(ceiling)
		step := 10
		if ceiling <= 15 {
			step = 5
		} else if ceiling > 50 {
			step = 15
		}
		for v := step; v < ceiling; v += step {
			topPct := 100 - v*100/ceiling
			color := "rgba(255,255,255,0.07)"
			switch v {
			case 20:
				color = "rgba(210,153,34,0.5)"
			case 35:
				color = "rgba(248,81,73,0.5)"
			}
			wd.WindGuides = append(wd.WindGuides, ChartGuide{TopPct: topPct, Label: strconv.Itoa(v), Color: color})
		}

		wd.RainGuides = rainGuides

		page.Days = append(page.Days, wd)
	}

	page.FetchedAt = time.Now().In(aest).Format("02 Jan 2006 15:04 AEST")
	render(w, "weather", page)
}
