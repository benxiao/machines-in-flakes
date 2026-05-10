package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ---- helpers ----

func (a *App) droneOptions(r *http.Request) ([]OptionItem, error) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx, `SELECT id, name FROM drones ORDER BY name`)
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

func (a *App) frameOptions(r *http.Request) ([]OptionItem, error) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx,
		`SELECT id, CASE WHEN brand!='' THEN brand||' '||name ELSE name END
         FROM frames ORDER BY brand, name`)
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

func (a *App) fcOptions(r *http.Request) ([]OptionItem, error) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx,
		`SELECT id, CASE WHEN brand!='' THEN brand||' '||name ELSE name END
         FROM flight_controllers ORDER BY brand, name`)
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

func (a *App) escOptions(r *http.Request) ([]OptionItem, error) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx,
		`SELECT id, CASE WHEN brand!='' THEN brand||' '||name ELSE name END
         FROM escs ORDER BY brand, name`)
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

func (a *App) vtxOptions(r *http.Request) ([]OptionItem, error) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx,
		`SELECT id, CASE WHEN brand!='' THEN brand||' '||name ELSE name END
         FROM vtx_units ORDER BY brand, name`)
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

func (a *App) batteryChecks(r *http.Request, sessionID int) ([]BatteryCheck, error) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx, `
        SELECT b.id,
               CASE WHEN b.brand!='' THEN b.brand||' '||b.name ELSE b.name END
               ||' ('||b.cell_count||'S '||b.capacity_mah||'mAh)',
               EXISTS(SELECT 1 FROM session_batteries sb
                      WHERE sb.session_id=$1 AND sb.battery_id=b.id),
               COALESCE((SELECT sb.count FROM session_batteries sb
                         WHERE sb.session_id=$1 AND sb.battery_id=b.id), 1)
        FROM batteries b WHERE b.status != 'dead'
        ORDER BY b.brand, b.name`, sessionID)
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

func sessionBatteryIDs(ctx context.Context, tx pgx.Tx, sessionID int) ([]int, error) {
	rows, err := tx.Query(ctx, `SELECT battery_id FROM session_batteries WHERE session_id=$1`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func sessionDroneIDs(ctx context.Context, tx pgx.Tx, sessionID int) ([]int, error) {
	rows, err := tx.Query(ctx, `SELECT drone_id FROM session_drones WHERE session_id=$1`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
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

func setDiff(a, b []int) []int {
	bset := make(map[int]bool, len(b))
	for _, v := range b {
		bset[v] = true
	}
	var out []int
	for _, v := range a {
		if !bset[v] {
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
               TO_CHAR(d.build_date,'YYYY-MM-DD'),
               COALESCE(CASE WHEN f.brand!='' THEN f.brand||' '||f.name ELSE f.name END,''),
               COALESCE(CASE WHEN fc.brand!='' THEN fc.brand||' '||fc.name ELSE fc.name END,''),
               COALESCE(CASE WHEN e.brand!='' THEN e.brand||' '||e.name ELSE e.name END,''),
               COALESCE(CASE WHEN v.brand!='' THEN v.brand||' '||v.name ELSE v.name END,''),
               COALESCE(CASE WHEN m.brand!='' THEN m.brand||' '||m.name ELSE m.name END,''),
               d.motor_count,
               COALESCE(CASE WHEN b.brand!='' THEN b.brand||' '||b.name ELSE b.name END,''),
               d.battery_count,
               COALESCE(CASE WHEN g.brand!='' THEN g.brand||' '||g.name ELSE g.name END,''),
               COALESCE(CASE WHEN rx.brand!='' THEN rx.brand||' '||rx.name ELSE rx.name END,'')
        FROM drones d
        LEFT JOIN frames f ON f.id=d.frame_id
        LEFT JOIN flight_controllers fc ON fc.id=d.fc_id
        LEFT JOIN escs e ON e.id=d.esc_id
        LEFT JOIN vtx_units v ON v.id=d.vtx_id
        LEFT JOIN motors m ON m.id=d.motor_id
        LEFT JOIN batteries b ON b.id=d.battery_id
        LEFT JOIN gps_modules g ON g.id=d.gps_id
        LEFT JOIN radio_receivers rx ON rx.id=d.rx_id
        ORDER BY d.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	defer rows.Close()
	var drones []DroneRow
	for rows.Next() {
		var d DroneRow
		var bd *string
		if err := rows.Scan(&d.ID, &d.Name, &d.Status, &bd,
			&d.FrameName, &d.FCName, &d.ESCName, &d.VTXName,
			&d.MotorName, &d.MotorCount, &d.BatteryName, &d.BatteryCount,
			&d.GPSName, &d.RXName); err != nil {
			httpErr(w, err)
			return
		}
		if bd != nil {
			d.BuildDate = *bd
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
	ctx := r.Context()
	rows, err := a.db.Query(ctx,
		`SELECT id, CASE WHEN brand!='' THEN brand||' '||name ELSE name END FROM motors ORDER BY brand,name`)
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

func (a *App) batteryOptions(r *http.Request) ([]OptionItem, error) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx,
		`SELECT id, CASE WHEN brand!='' THEN brand||' '||name||' ('||cell_count||'S '||capacity_mah||'mAh)'
                         ELSE name||' ('||cell_count||'S '||capacity_mah||'mAh)' END
         FROM batteries ORDER BY brand, name, cell_count, capacity_mah`)
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

func (a *App) gpsOptions(r *http.Request) ([]OptionItem, error) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx,
		`SELECT id, CASE WHEN brand!='' THEN brand||' '||name ELSE name END FROM gps_modules ORDER BY brand, name`)
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

func (a *App) rxOptions(r *http.Request) ([]OptionItem, error) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx,
		`SELECT id, CASE WHEN brand!='' THEN brand||' '||name ELSE name END FROM radio_receivers ORDER BY brand, name`)
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

func (a *App) fillDroneFormOptions(r *http.Request, page *DroneFormPage) {
	page.Frames, _ = a.frameOptions(r)
	page.FCs, _ = a.fcOptions(r)
	page.ESCs, _ = a.escOptions(r)
	page.VTXs, _ = a.vtxOptions(r)
	page.Motors, _ = a.motorOptions(r)
	page.Batteries, _ = a.batteryOptions(r)
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
		bc, _ := strconv.Atoi(r.FormValue("battery_count"))
		if bc == 0 {
			bc = 1
		}
		_, err := a.db.Exec(ctx, `
            INSERT INTO drones (name,frame_id,fc_id,esc_id,vtx_id,motor_id,motor_count,battery_id,battery_count,gps_id,rx_id,status,build_date,notes)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			name,
			nullIntPtr(r.FormValue("frame_id")),
			nullIntPtr(r.FormValue("fc_id")),
			nullIntPtr(r.FormValue("esc_id")),
			nullIntPtr(r.FormValue("vtx_id")),
			nullIntPtr(r.FormValue("motor_id")),
			mc,
			nullIntPtr(r.FormValue("battery_id")),
			bc,
			nullIntPtr(r.FormValue("gps_id")),
			nullIntPtr(r.FormValue("rx_id")),
			r.FormValue("status"),
			nullDate(r.FormValue("build_date")),
			r.FormValue("notes"))
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/drones", http.StatusSeeOther)
		return
	}
	page := DroneFormPage{ActiveTab: "drones", Status: "build", BuildDate: time.Now().Format("2006-01-02")}
	a.fillDroneFormOptions(r, &page)
	render(w, "drone-form", page)
}

func (a *App) handleDroneEdit(w http.ResponseWriter, r *http.Request) {
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
			page := DroneFormPage{ActiveTab: "drones", Error: "Name is required", ID: id}
			a.fillDroneFormOptions(r, &page)
			render(w, "drone-form", page)
			return
		}
		mc, _ := strconv.Atoi(r.FormValue("motor_count"))
		if mc == 0 {
			mc = 4
		}
		bc, _ := strconv.Atoi(r.FormValue("battery_count"))
		if bc == 0 {
			bc = 1
		}
		_, err := a.db.Exec(ctx, `
            UPDATE drones SET name=$1,frame_id=$2,fc_id=$3,esc_id=$4,vtx_id=$5,
            motor_id=$6,motor_count=$7,battery_id=$8,battery_count=$9,
            gps_id=$10,rx_id=$11,status=$12,build_date=$13,notes=$14 WHERE id=$15`,
			name,
			nullIntPtr(r.FormValue("frame_id")),
			nullIntPtr(r.FormValue("fc_id")),
			nullIntPtr(r.FormValue("esc_id")),
			nullIntPtr(r.FormValue("vtx_id")),
			nullIntPtr(r.FormValue("motor_id")),
			mc,
			nullIntPtr(r.FormValue("battery_id")),
			bc,
			nullIntPtr(r.FormValue("gps_id")),
			nullIntPtr(r.FormValue("rx_id")),
			r.FormValue("status"),
			nullDate(r.FormValue("build_date")),
			r.FormValue("notes"), id)
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/drones", http.StatusSeeOther)
		return
	}
	var page DroneFormPage
	var frameID, fcID, escID, vtxID, motorID, batteryID, gpsID, rxID *int
	var motorCount, batteryCount int
	var bd *string
	err := a.db.QueryRow(ctx, `
        SELECT id,name,frame_id,fc_id,esc_id,vtx_id,motor_id,motor_count,
               battery_id,battery_count,gps_id,rx_id,
               status,TO_CHAR(build_date,'YYYY-MM-DD'),notes
        FROM drones WHERE id=$1`, id).Scan(
		&page.ID, &page.Name, &frameID, &fcID, &escID, &vtxID,
		&motorID, &motorCount, &batteryID, &batteryCount,
		&gpsID, &rxID, &page.Status, &bd, &page.Notes)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	if frameID != nil {
		page.FrameID = *frameID
	}
	if fcID != nil {
		page.FCID = *fcID
	}
	if escID != nil {
		page.ESCID = *escID
	}
	if vtxID != nil {
		page.VTXID = *vtxID
	}
	if motorID != nil {
		page.MotorID = *motorID
	}
	if batteryID != nil {
		page.BatteryID = *batteryID
	}
	if gpsID != nil {
		page.GPSID = *gpsID
	}
	if rxID != nil {
		page.RXID = *rxID
	}
	page.MotorCount = strconv.Itoa(motorCount)
	page.BatteryCount = strconv.Itoa(batteryCount)
	if bd != nil {
		page.BuildDate = *bd
	}
	page.ActiveTab = "drones"
	a.fillDroneFormOptions(r, &page)
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

// ---- Inventory ----

func (a *App) handleInventory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page := InventoryPage{ActiveTab: "inventory"}

	// Each component table has one row per unique model.
	// item_counts.count = total owned.
	// installed = count(drones) referencing this model (or sum(motor_count) for motors).
	// available = total - installed.

	rows, err := a.db.Query(ctx, `
        SELECT f.id, f.brand, f.name, f.size_mm, f.weight_g,
               COALESCE(ic.count,0),
               (SELECT COUNT(*) FROM drones d WHERE d.frame_id=f.id),
               COALESCE(ic.count,0) - (SELECT COUNT(*) FROM drones d WHERE d.frame_id=f.id),
               COALESCE(string_agg(d.name,', ' ORDER BY d.name),'')
        FROM frames f
        LEFT JOIN item_counts ic ON ic.item_type='frame' AND ic.item_id=f.id
        LEFT JOIN drones d ON d.frame_id=f.id
        GROUP BY f.id, f.brand, f.name, f.size_mm, f.weight_g, ic.count
        ORDER BY f.brand, f.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var fr FrameRow
		var sizeMM, weightG *int
		if err := rows.Scan(&fr.ID, &fr.Brand, &fr.Name, &sizeMM, &weightG,
			&fr.Total, &fr.Installed, &fr.Available, &fr.InstalledOn); err != nil {
			rows.Close()
			httpErr(w, err)
			return
		}
		if sizeMM != nil {
			fr.SizeMM = strconv.Itoa(*sizeMM)
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
        SELECT fc.id, fc.brand, fc.name, fc.mcu, fc.firmware,
               COALESCE(ic.count,0),
               (SELECT COUNT(*) FROM drones d WHERE d.fc_id=fc.id),
               COALESCE(ic.count,0) - (SELECT COUNT(*) FROM drones d WHERE d.fc_id=fc.id),
               COALESCE(string_agg(d.name,', ' ORDER BY d.name),'')
        FROM flight_controllers fc
        LEFT JOIN item_counts ic ON ic.item_type='fc' AND ic.item_id=fc.id
        LEFT JOIN drones d ON d.fc_id=fc.id
        GROUP BY fc.id, fc.brand, fc.name, fc.mcu, fc.firmware, ic.count
        ORDER BY fc.brand, fc.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var fc FCRow
		if err := rows.Scan(&fc.ID, &fc.Brand, &fc.Name, &fc.MCU, &fc.Firmware,
			&fc.Total, &fc.Installed, &fc.Available, &fc.InstalledOn); err != nil {
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
        SELECT e.id, e.brand, e.name, e.current_rating, e.cell_max,
               COALESCE(ic.count,0),
               (SELECT COUNT(*) FROM drones d WHERE d.esc_id=e.id),
               COALESCE(ic.count,0) - (SELECT COUNT(*) FROM drones d WHERE d.esc_id=e.id),
               COALESCE(string_agg(d.name,', ' ORDER BY d.name),'')
        FROM escs e
        LEFT JOIN item_counts ic ON ic.item_type='esc' AND ic.item_id=e.id
        LEFT JOIN drones d ON d.esc_id=e.id
        GROUP BY e.id, e.brand, e.name, e.current_rating, e.cell_max, ic.count
        ORDER BY e.brand, e.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var e ESCRow
		var cr, cm *int
		if err := rows.Scan(&e.ID, &e.Brand, &e.Name, &cr, &cm,
			&e.Total, &e.Installed, &e.Available, &e.InstalledOn); err != nil {
			rows.Close()
			httpErr(w, err)
			return
		}
		if cr != nil {
			e.CurrentRating = strconv.Itoa(*cr)
		}
		if cm != nil {
			e.CellMax = strconv.Itoa(*cm)
		}
		page.ESCs = append(page.ESCs, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpErr(w, err)
		return
	}

	rows, err = a.db.Query(ctx, `
        SELECT m.id, m.brand, m.name, m.stator_size, m.kv,
               COALESCE(ic.count,0),
               COALESCE((SELECT SUM(d.motor_count) FROM drones d WHERE d.motor_id=m.id),0),
               COALESCE(ic.count,0) - COALESCE((SELECT SUM(d.motor_count) FROM drones d WHERE d.motor_id=m.id),0),
               COALESCE(string_agg(d.name||' (×'||d.motor_count||')',', ' ORDER BY d.name),'')
        FROM motors m
        LEFT JOIN item_counts ic ON ic.item_type='motor' AND ic.item_id=m.id
        LEFT JOIN drones d ON d.motor_id=m.id
        GROUP BY m.id, m.brand, m.name, m.stator_size, m.kv, ic.count
        ORDER BY m.brand, m.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var m MotorRow
		var kv *int
		if err := rows.Scan(&m.ID, &m.Brand, &m.Name, &m.StatorSize, &kv,
			&m.Total, &m.Installed, &m.Available, &m.InstalledOn); err != nil {
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
        SELECT v.id, v.brand, v.name, v.system, v.max_power_mw, v.resolution,
               COALESCE(ic.count,0),
               (SELECT COUNT(*) FROM drones d WHERE d.vtx_id=v.id),
               COALESCE(ic.count,0) - (SELECT COUNT(*) FROM drones d WHERE d.vtx_id=v.id),
               COALESCE(string_agg(d.name,', ' ORDER BY d.name),'')
        FROM vtx_units v
        LEFT JOIN item_counts ic ON ic.item_type='vtx' AND ic.item_id=v.id
        LEFT JOIN drones d ON d.vtx_id=v.id
        GROUP BY v.id, v.brand, v.name, v.system, v.max_power_mw, v.resolution, ic.count
        ORDER BY v.brand, v.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var v VTXRow
		var mw *int
		if err := rows.Scan(&v.ID, &v.Brand, &v.Name, &v.System, &mw, &v.Resolution,
			&v.Total, &v.Installed, &v.Available, &v.InstalledOn); err != nil {
			rows.Close()
			httpErr(w, err)
			return
		}
		if mw != nil {
			v.MaxPowerMW = strconv.Itoa(*mw)
		}
		page.VTXs = append(page.VTXs, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpErr(w, err)
		return
	}

	rows, err = a.db.Query(ctx, `
        SELECT g.id, g.brand, g.name,
               COALESCE(ic.count,0),
               (SELECT COUNT(*) FROM drones d WHERE d.gps_id=g.id),
               COALESCE(ic.count,0) - (SELECT COUNT(*) FROM drones d WHERE d.gps_id=g.id),
               COALESCE(string_agg(d.name,', ' ORDER BY d.name),'')
        FROM gps_modules g
        LEFT JOIN item_counts ic ON ic.item_type='gps' AND ic.item_id=g.id
        LEFT JOIN drones d ON d.gps_id=g.id
        GROUP BY g.id, g.brand, g.name, ic.count
        ORDER BY g.brand, g.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var g GPSRow
		if err := rows.Scan(&g.ID, &g.Brand, &g.Name,
			&g.Total, &g.Installed, &g.Available, &g.InstalledOn); err != nil {
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
        SELECT rx.id, rx.brand, rx.name, rx.protocol,
               COALESCE(ic.count,0),
               (SELECT COUNT(*) FROM drones d WHERE d.rx_id=rx.id),
               COALESCE(ic.count,0) - (SELECT COUNT(*) FROM drones d WHERE d.rx_id=rx.id),
               COALESCE(string_agg(d.name,', ' ORDER BY d.name),'')
        FROM radio_receivers rx
        LEFT JOIN item_counts ic ON ic.item_type='rx' AND ic.item_id=rx.id
        LEFT JOIN drones d ON d.rx_id=rx.id
        GROUP BY rx.id, rx.brand, rx.name, rx.protocol, ic.count
        ORDER BY rx.brand, rx.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var rx RXRow
		if err := rows.Scan(&rx.ID, &rx.Brand, &rx.Name, &rx.Protocol,
			&rx.Total, &rx.Installed, &rx.Available, &rx.InstalledOn); err != nil {
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
			render(w, "frame-form", FrameFormPage{ActiveTab: "inventory", Error: "Name is required"})
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		var id int
		err := a.db.QueryRow(ctx,
			`INSERT INTO frames (brand,name,size_mm,weight_g,notes) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			r.FormValue("brand"), name,
			nullIntPtr(r.FormValue("size_mm")), nullIntPtr(r.FormValue("weight_g")),
			r.FormValue("notes")).Scan(&id)
		if err != nil {
			httpErr(w, err)
			return
		}
		a.db.Exec(ctx, `INSERT INTO item_counts (item_type,item_id,count) VALUES ('frame',$1,$2)`, id, qty)
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	render(w, "frame-form", FrameFormPage{ActiveTab: "inventory"})
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
			render(w, "frame-form", FrameFormPage{ActiveTab: "inventory", Error: "Name is required", ID: id})
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		_, err := a.db.Exec(ctx,
			`UPDATE frames SET brand=$1,name=$2,size_mm=$3,weight_g=$4,notes=$5 WHERE id=$6`,
			r.FormValue("brand"), name,
			nullIntPtr(r.FormValue("size_mm")), nullIntPtr(r.FormValue("weight_g")),
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
	var sizeMM, weightG *int
	var qty int
	err := a.db.QueryRow(ctx,
		`SELECT f.id,f.brand,f.name,f.size_mm,f.weight_g,f.notes,COALESCE(ic.count,0)
         FROM frames f LEFT JOIN item_counts ic ON ic.item_type='frame' AND ic.item_id=f.id
         WHERE f.id=$1`, id).
		Scan(&page.ID, &page.Brand, &page.Name, &sizeMM, &weightG, &page.Notes, &qty)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	if sizeMM != nil {
		page.SizeMM = strconv.Itoa(*sizeMM)
	}
	if weightG != nil {
		page.WeightG = strconv.Itoa(*weightG)
	}
	page.Quantity = strconv.Itoa(qty)
	page.ActiveTab = "inventory"
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
			render(w, "fc-form", FCFormPage{ActiveTab: "inventory", Error: "Name is required"})
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		var id int
		err := a.db.QueryRow(ctx,
			`INSERT INTO flight_controllers (brand,name,mcu,firmware,notes) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			r.FormValue("brand"), name, r.FormValue("mcu"), r.FormValue("firmware"),
			r.FormValue("notes")).Scan(&id)
		if err != nil {
			httpErr(w, err)
			return
		}
		a.db.Exec(ctx, `INSERT INTO item_counts (item_type,item_id,count) VALUES ('fc',$1,$2)`, id, qty)
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	render(w, "fc-form", FCFormPage{ActiveTab: "inventory"})
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
			render(w, "fc-form", FCFormPage{ActiveTab: "inventory", Error: "Name is required", ID: id})
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		_, err := a.db.Exec(ctx,
			`UPDATE flight_controllers SET brand=$1,name=$2,mcu=$3,firmware=$4,notes=$5 WHERE id=$6`,
			r.FormValue("brand"), name, r.FormValue("mcu"), r.FormValue("firmware"),
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
	var qty int
	err := a.db.QueryRow(ctx,
		`SELECT fc.id,fc.brand,fc.name,fc.mcu,fc.firmware,fc.notes,COALESCE(ic.count,0)
         FROM flight_controllers fc LEFT JOIN item_counts ic ON ic.item_type='fc' AND ic.item_id=fc.id
         WHERE fc.id=$1`, id).
		Scan(&page.ID, &page.Brand, &page.Name, &page.MCU, &page.Firmware, &page.Notes, &qty)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	page.Quantity = strconv.Itoa(qty)
	page.ActiveTab = "inventory"
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
			render(w, "esc-form", ESCFormPage{ActiveTab: "inventory", Error: "Name is required"})
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		var id int
		err := a.db.QueryRow(ctx,
			`INSERT INTO escs (brand,name,current_rating,cell_max,notes) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			r.FormValue("brand"), name,
			nullIntPtr(r.FormValue("current_rating")), nullIntPtr(r.FormValue("cell_max")),
			r.FormValue("notes")).Scan(&id)
		if err != nil {
			httpErr(w, err)
			return
		}
		a.db.Exec(ctx, `INSERT INTO item_counts (item_type,item_id,count) VALUES ('esc',$1,$2)`, id, qty)
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	render(w, "esc-form", ESCFormPage{ActiveTab: "inventory"})
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
			render(w, "esc-form", ESCFormPage{ActiveTab: "inventory", Error: "Name is required", ID: id})
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		_, err := a.db.Exec(ctx,
			`UPDATE escs SET brand=$1,name=$2,current_rating=$3,cell_max=$4,notes=$5 WHERE id=$6`,
			r.FormValue("brand"), name,
			nullIntPtr(r.FormValue("current_rating")), nullIntPtr(r.FormValue("cell_max")),
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
	var cr, cm *int
	var qty int
	err := a.db.QueryRow(ctx,
		`SELECT e.id,e.brand,e.name,e.current_rating,e.cell_max,e.notes,COALESCE(ic.count,0)
         FROM escs e LEFT JOIN item_counts ic ON ic.item_type='esc' AND ic.item_id=e.id
         WHERE e.id=$1`, id).
		Scan(&page.ID, &page.Brand, &page.Name, &cr, &cm, &page.Notes, &qty)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	if cr != nil {
		page.CurrentRating = strconv.Itoa(*cr)
	}
	if cm != nil {
		page.CellMax = strconv.Itoa(*cm)
	}
	page.Quantity = strconv.Itoa(qty)
	page.ActiveTab = "inventory"
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
			render(w, "motor-form", MotorFormPage{ActiveTab: "inventory", Error: "Name is required"})
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		var id int
		err := a.db.QueryRow(ctx,
			`INSERT INTO motors (brand,name,stator_size,kv,notes) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			r.FormValue("brand"), name, r.FormValue("stator_size"),
			nullIntPtr(r.FormValue("kv")), r.FormValue("notes")).Scan(&id)
		if err != nil {
			httpErr(w, err)
			return
		}
		a.db.Exec(ctx, `INSERT INTO item_counts (item_type,item_id,count) VALUES ('motor',$1,$2)`, id, qty)
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	render(w, "motor-form", MotorFormPage{ActiveTab: "inventory"})
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
			render(w, "motor-form", MotorFormPage{ActiveTab: "inventory", Error: "Name is required", ID: id})
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		_, err := a.db.Exec(ctx,
			`UPDATE motors SET brand=$1,name=$2,stator_size=$3,kv=$4,notes=$5 WHERE id=$6`,
			r.FormValue("brand"), name, r.FormValue("stator_size"),
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
	var kv *int
	var qty int
	err := a.db.QueryRow(ctx,
		`SELECT m.id,m.brand,m.name,m.stator_size,m.kv,m.notes,COALESCE(ic.count,0)
         FROM motors m LEFT JOIN item_counts ic ON ic.item_type='motor' AND ic.item_id=m.id
         WHERE m.id=$1`, id).
		Scan(&page.ID, &page.Brand, &page.Name, &page.StatorSize, &kv, &page.Notes, &qty)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	if kv != nil {
		page.KV = strconv.Itoa(*kv)
	}
	page.Quantity = strconv.Itoa(qty)
	page.ActiveTab = "inventory"
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
			render(w, "vtx-form", VTXFormPage{ActiveTab: "inventory", Error: "Name is required"})
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		var id int
		err := a.db.QueryRow(ctx,
			`INSERT INTO vtx_units (brand,name,system,max_power_mw,resolution,weight_g,notes) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
			r.FormValue("brand"), name, r.FormValue("system"),
			nullIntPtr(r.FormValue("max_power_mw")), r.FormValue("resolution"),
			nullIntPtr(r.FormValue("weight_g")), r.FormValue("notes")).Scan(&id)
		if err != nil {
			httpErr(w, err)
			return
		}
		a.db.Exec(ctx, `INSERT INTO item_counts (item_type,item_id,count) VALUES ('vtx',$1,$2)`, id, qty)
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	render(w, "vtx-form", VTXFormPage{ActiveTab: "inventory"})
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
			render(w, "vtx-form", VTXFormPage{ActiveTab: "inventory", Error: "Name is required", ID: id})
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		_, err := a.db.Exec(ctx,
			`UPDATE vtx_units SET brand=$1,name=$2,system=$3,max_power_mw=$4,resolution=$5,weight_g=$6,notes=$7 WHERE id=$8`,
			r.FormValue("brand"), name, r.FormValue("system"),
			nullIntPtr(r.FormValue("max_power_mw")), r.FormValue("resolution"),
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
	var mw, wg *int
	var qty int
	err := a.db.QueryRow(ctx,
		`SELECT v.id,v.brand,v.name,v.system,v.max_power_mw,v.resolution,v.weight_g,v.notes,COALESCE(ic.count,0)
         FROM vtx_units v LEFT JOIN item_counts ic ON ic.item_type='vtx' AND ic.item_id=v.id
         WHERE v.id=$1`, id).
		Scan(&page.ID, &page.Brand, &page.Name, &page.System, &mw, &page.Resolution, &wg, &page.Notes, &qty)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	if mw != nil {
		page.MaxPowerMW = strconv.Itoa(*mw)
	}
	if wg != nil {
		page.WeightG = strconv.Itoa(*wg)
	}
	page.Quantity = strconv.Itoa(qty)
	page.ActiveTab = "inventory"
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
        SELECT p.id, p.brand, p.name,
               CAST(p.size_inch AS FLOAT8), CAST(p.pitch AS FLOAT8),
               p.blade_count, p.material, p.quantity, p.reorder_threshold,
               COALESCE(d.name,'')
        FROM propellers p LEFT JOIN drones d ON d.id=p.drone_id
        ORDER BY p.brand, p.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	defer rows.Close()
	var props []PropRow
	for rows.Next() {
		var p PropRow
		var si, pitch *float64
		if err := rows.Scan(&p.ID, &p.Brand, &p.Name, &si, &pitch,
			&p.BladeCount, &p.Material, &p.Quantity, &p.ReorderThreshold, &p.DroneName); err != nil {
			httpErr(w, err)
			return
		}
		if si != nil {
			p.SizeInch = fmt.Sprintf("%.1f", *si)
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
			page.Drones, _ = a.droneOptions(r)
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
			`INSERT INTO propellers (brand,name,size_inch,pitch,blade_count,material,quantity,reorder_threshold,drone_id,notes)
             VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			r.FormValue("brand"), name,
			nullFloat64(r.FormValue("size_inch")), nullFloat64(r.FormValue("pitch")),
			bc, r.FormValue("material"), qty, rt,
			nullIntPtr(r.FormValue("drone_id")), r.FormValue("notes"))
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/props", http.StatusSeeOther)
		return
	}
	page := PropFormPage{ActiveTab: "props", BladeCount: "3"}
	page.Drones, _ = a.droneOptions(r)
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
			page.Drones, _ = a.droneOptions(r)
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
			`UPDATE propellers SET brand=$1,name=$2,size_inch=$3,pitch=$4,blade_count=$5,
             material=$6,quantity=$7,reorder_threshold=$8,drone_id=$9,notes=$10 WHERE id=$11`,
			r.FormValue("brand"), name,
			nullFloat64(r.FormValue("size_inch")), nullFloat64(r.FormValue("pitch")),
			bc, r.FormValue("material"), qty, rt,
			nullIntPtr(r.FormValue("drone_id")), r.FormValue("notes"), id)
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/props", http.StatusSeeOther)
		return
	}
	var page PropFormPage
	var si, pitch *float64
	var droneID *int
	var bladeCount, qty, rt int
	err := a.db.QueryRow(ctx,
		`SELECT id,brand,name,CAST(size_inch AS FLOAT8),CAST(pitch AS FLOAT8),blade_count,
         material,quantity,reorder_threshold,drone_id,notes FROM propellers WHERE id=$1`, id).
		Scan(&page.ID, &page.Brand, &page.Name, &si, &pitch,
			&bladeCount, &page.Material, &qty, &rt,
			&droneID, &page.Notes)
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
	if si != nil {
		page.SizeInch = fmt.Sprintf("%.1f", *si)
	}
	if pitch != nil {
		page.Pitch = fmt.Sprintf("%.1f", *pitch)
	}
	if droneID != nil {
		page.DroneID = *droneID
	}
	page.ActiveTab = "props"
	page.Drones, _ = a.droneOptions(r)
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
        SELECT b.id, b.brand, b.name, b.cell_count, b.capacity_mah, b.status,
               b.count,
               COALESCE((SELECT SUM(d.battery_count) FROM drones d WHERE d.battery_id=b.id),0),
               b.count - COALESCE((SELECT SUM(d.battery_count) FROM drones d WHERE d.battery_id=b.id),0),
               COALESCE(string_agg(d.name||' (×'||d.battery_count||')',', ' ORDER BY d.name),'')
        FROM batteries b
        LEFT JOIN drones d ON d.battery_id=b.id
        GROUP BY b.id, b.brand, b.name, b.cell_count, b.capacity_mah, b.status, b.count
        ORDER BY b.brand, b.name, b.cell_count, b.capacity_mah`)
	if err != nil {
		httpErr(w, err)
		return
	}
	defer rows.Close()
	var bats []BatteryRow
	for rows.Next() {
		var b BatteryRow
		if err := rows.Scan(&b.ID, &b.Brand, &b.Name, &b.CellCount, &b.CapacityMAh,
			&b.Status, &b.Total, &b.Installed, &b.Available, &b.InstalledOn); err != nil {
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
		cc, _ := strconv.Atoi(r.FormValue("cell_count"))
		cap, _ := strconv.Atoi(r.FormValue("capacity_mah"))
		if name == "" || cc == 0 || cap == 0 {
			render(w, "battery-form", BatteryFormPage{ActiveTab: "batteries",
				Error: "Name, cell count, and capacity are required", Status: "good"})
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		if qty == 0 {
			qty = 1
		}
		_, err := a.db.Exec(ctx,
			`INSERT INTO batteries (brand,name,cell_count,capacity_mah,count,status,notes)
             VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			r.FormValue("brand"), name, cc, cap, qty,
			r.FormValue("status"), r.FormValue("notes"))
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/batteries", http.StatusSeeOther)
		return
	}
	render(w, "battery-form", BatteryFormPage{ActiveTab: "batteries", Status: "good"})
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
		cc, _ := strconv.Atoi(r.FormValue("cell_count"))
		cap, _ := strconv.Atoi(r.FormValue("capacity_mah"))
		if name == "" || cc == 0 || cap == 0 {
			render(w, "battery-form", BatteryFormPage{ActiveTab: "batteries",
				Error: "Name, cell count, and capacity are required", ID: id, Status: "good"})
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		if qty == 0 {
			qty = 1
		}
		_, err := a.db.Exec(ctx,
			`UPDATE batteries SET brand=$1,name=$2,cell_count=$3,capacity_mah=$4,count=$5,status=$6,notes=$7 WHERE id=$8`,
			r.FormValue("brand"), name, cc, cap, qty, r.FormValue("status"), r.FormValue("notes"), id)
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/batteries", http.StatusSeeOther)
		return
	}
	var page BatteryFormPage
	var cellCount, capMAh, qty int
	err := a.db.QueryRow(ctx,
		`SELECT id,brand,name,cell_count,capacity_mah,count,status,notes FROM batteries WHERE id=$1`, id).
		Scan(&page.ID, &page.Brand, &page.Name, &cellCount, &capMAh, &qty, &page.Status, &page.Notes)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	page.CellCount = strconv.Itoa(cellCount)
	page.CapacityMAh = strconv.Itoa(capMAh)
	page.Quantity = strconv.Itoa(qty)
	page.ActiveTab = "batteries"
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
        SELECT s.id, s.type,
               TO_CHAR(s.session_date,'YYYY-MM-DD'),
               s.duration_min, s.location, s.notes,
               COALESCE((SELECT string_agg(d.name,', ' ORDER BY d.name)
                         FROM session_drones sd JOIN drones d ON d.id=sd.drone_id
                         WHERE sd.session_id=s.id),''),
               COALESCE((SELECT string_agg(b.name||' ('||b.cell_count||'S)',', ' ORDER BY b.name)
                         FROM session_batteries sb JOIN batteries b ON b.id=sb.battery_id
                         WHERE sb.session_id=s.id),'')
        FROM sessions s
        ORDER BY s.session_date DESC, s.created_at DESC`)
	if err != nil {
		httpErr(w, err)
		return
	}
	defer rows.Close()
	var sessions []SessionRow
	for rows.Next() {
		var s SessionRow
		if err := rows.Scan(&s.ID, &s.Type, &s.SessionDate,
			&s.DurationMin, &s.Location, &s.Notes, &s.DroneNames, &s.BatteryList); err != nil {
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
			`INSERT INTO sessions (type,session_date,duration_min,location,notes)
             VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			r.FormValue("type"), nullDate(r.FormValue("session_date")), dur,
			r.FormValue("location"), r.FormValue("notes")).Scan(&sessionID)
		if err != nil {
			httpErr(w, err)
			return
		}
		for _, did := range droneIDs {
			tx.Exec(ctx, `INSERT INTO session_drones (session_id,drone_id) VALUES ($1,$2)`, sessionID, did)
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
			tx.Exec(ctx, `INSERT INTO session_batteries (session_id,battery_id,count) VALUES ($1,$2,$3)`, sessionID, batID, cnt)
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
        SELECT s.id, s.type, TO_CHAR(s.session_date,'YYYY-MM-DD'),
               s.duration_min, s.location, s.notes,
               COALESCE((SELECT string_agg(d.name,', ' ORDER BY d.name)
                         FROM session_drones sd JOIN drones d ON d.id=sd.drone_id
                         WHERE sd.session_id=s.id),'')
        FROM sessions s WHERE s.id=$1`, id).
		Scan(&page.ID, &page.Type, &page.Date,
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
        SELECT b.id, b.brand, b.name, b.cell_count, b.capacity_mah, b.status, sb.count
        FROM batteries b JOIN session_batteries sb ON sb.battery_id=b.id
        WHERE sb.session_id=$1 ORDER BY b.brand, b.name`, id)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var b BatteryRow
		if err := rows.Scan(&b.ID, &b.Brand, &b.Name, &b.CellCount, &b.CapacityMAh, &b.Status, &b.Count); err != nil {
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
	rows, err = a.db.Query(ctx,
		`SELECT id, original_name, notes FROM session_videos WHERE session_id=$1 ORDER BY created_at`, id)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var v VideoRow
		if err := rows.Scan(&v.ID, &v.OriginalName, &v.Notes); err != nil {
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

		tx.Exec(ctx, `DELETE FROM session_drones WHERE session_id=$1`, id)
		for _, did := range droneIDs {
			tx.Exec(ctx, `INSERT INTO session_drones (session_id,drone_id) VALUES ($1,$2)`, id, did)
		}
		tx.Exec(ctx, `DELETE FROM session_batteries WHERE session_id=$1`, id)
		for _, bid := range newBatIDs {
			cnt, _ := strconv.Atoi(r.FormValue(fmt.Sprintf("battery_count_%d", bid)))
			if cnt < 1 {
				cnt = 1
			}
			tx.Exec(ctx, `INSERT INTO session_batteries (session_id,battery_id,count) VALUES ($1,$2,$3)`, id, bid, cnt)
		}
		_, err = tx.Exec(ctx,
			`UPDATE sessions SET type=$1,session_date=$2,duration_min=$3,location=$4,notes=$5 WHERE id=$6`,
			r.FormValue("type"), nullDate(r.FormValue("session_date")), dur,
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
		`SELECT id,type,TO_CHAR(session_date,'YYYY-MM-DD'),duration_min,location,notes FROM sessions WHERE id=$1`, id).
		Scan(&page.ID, &page.Type, &page.SessionDate, &dur, &page.Location, &page.Notes)
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
			render(w, "gps-form", GPSFormPage{ActiveTab: "inventory", Error: "Name is required"})
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		var id int
		err := a.db.QueryRow(ctx,
			`INSERT INTO gps_modules (brand,name,notes) VALUES ($1,$2,$3) RETURNING id`,
			r.FormValue("brand"), name, r.FormValue("notes")).Scan(&id)
		if err != nil {
			httpErr(w, err)
			return
		}
		a.db.Exec(ctx, `INSERT INTO item_counts (item_type,item_id,count) VALUES ('gps',$1,$2)`, id, qty)
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	render(w, "gps-form", GPSFormPage{ActiveTab: "inventory"})
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
			render(w, "gps-form", GPSFormPage{ActiveTab: "inventory", Error: "Name is required", ID: id})
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		_, err := a.db.Exec(ctx,
			`UPDATE gps_modules SET brand=$1,name=$2,notes=$3 WHERE id=$4`,
			r.FormValue("brand"), name, r.FormValue("notes"), id)
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
	var qty int
	err := a.db.QueryRow(ctx,
		`SELECT g.id,g.brand,g.name,g.notes,COALESCE(ic.count,0)
         FROM gps_modules g LEFT JOIN item_counts ic ON ic.item_type='gps' AND ic.item_id=g.id
         WHERE g.id=$1`, id).
		Scan(&page.ID, &page.Brand, &page.Name, &page.Notes, &qty)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	page.Quantity = strconv.Itoa(qty)
	page.ActiveTab = "inventory"
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
			render(w, "rx-form", RXFormPage{ActiveTab: "inventory", Error: "Name is required"})
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		var id int
		err := a.db.QueryRow(ctx,
			`INSERT INTO radio_receivers (brand,name,protocol,notes) VALUES ($1,$2,$3,$4) RETURNING id`,
			r.FormValue("brand"), name, r.FormValue("protocol"), r.FormValue("notes")).Scan(&id)
		if err != nil {
			httpErr(w, err)
			return
		}
		a.db.Exec(ctx, `INSERT INTO item_counts (item_type,item_id,count) VALUES ('rx',$1,$2)`, id, qty)
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	render(w, "rx-form", RXFormPage{ActiveTab: "inventory"})
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
			render(w, "rx-form", RXFormPage{ActiveTab: "inventory", Error: "Name is required", ID: id})
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		_, err := a.db.Exec(ctx,
			`UPDATE radio_receivers SET brand=$1,name=$2,protocol=$3,notes=$4 WHERE id=$5`,
			r.FormValue("brand"), name, r.FormValue("protocol"), r.FormValue("notes"), id)
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
	var qty int
	err := a.db.QueryRow(ctx,
		`SELECT rx.id,rx.brand,rx.name,rx.protocol,rx.notes,COALESCE(ic.count,0)
         FROM radio_receivers rx LEFT JOIN item_counts ic ON ic.item_type='rx' AND ic.item_id=rx.id
         WHERE rx.id=$1`, id).
		Scan(&page.ID, &page.Brand, &page.Name, &page.Protocol, &page.Notes, &qty)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	page.Quantity = strconv.Itoa(qty)
	page.ActiveTab = "inventory"
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
		`INSERT INTO session_videos (session_id,filename,original_name) VALUES ($1,$2,$3)`,
		id, filename, header.Filename)
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
