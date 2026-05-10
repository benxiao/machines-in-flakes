package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

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

func (a *App) frameOptions(r *http.Request, currentID int) ([]OptionItem, error) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx,
		`SELECT id, CASE WHEN brand!='' THEN brand||' '||name ELSE name END
         FROM frames WHERE status='spare' OR id=$1 ORDER BY brand,name`, currentID)
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

func (a *App) fcOptions(r *http.Request, currentID int) ([]OptionItem, error) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx,
		`SELECT id, CASE WHEN brand!='' THEN brand||' '||name ELSE name END
         FROM flight_controllers WHERE status='spare' OR id=$1 ORDER BY brand,name`, currentID)
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

func (a *App) escOptions(r *http.Request, currentID int) ([]OptionItem, error) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx,
		`SELECT id, CASE WHEN brand!='' THEN brand||' '||name ELSE name END
         FROM escs WHERE status='spare' OR id=$1 ORDER BY brand,name`, currentID)
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

func (a *App) vtxOptions(r *http.Request, currentID int) ([]OptionItem, error) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx,
		`SELECT id, CASE WHEN brand!='' THEN brand||' '||name ELSE name END
         FROM vtx_units WHERE status='spare' OR id=$1 ORDER BY brand,name`, currentID)
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
               b.name||' ('||b.cell_count||'S '||b.capacity_mah||'mAh, '||b.cycle_count||' cyc)',
               EXISTS(SELECT 1 FROM session_batteries sb
                      WHERE sb.session_id=$1 AND sb.battery_id=b.id)
        FROM batteries b WHERE b.status != 'dead'
        ORDER BY b.name`, sessionID)
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
               (SELECT COUNT(*) FROM motors m WHERE m.drone_id=d.id)
        FROM drones d
        LEFT JOIN frames f ON f.id=d.frame_id
        LEFT JOIN flight_controllers fc ON fc.id=d.fc_id
        LEFT JOIN escs e ON e.id=d.esc_id
        LEFT JOIN vtx_units v ON v.id=d.vtx_id
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
		var mc int64
		if err := rows.Scan(&d.ID, &d.Name, &d.Status, &bd,
			&d.FrameName, &d.FCName, &d.ESCName, &d.VTXName, &mc); err != nil {
			httpErr(w, err)
			return
		}
		d.Motors = int(mc)
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

func (a *App) fillDroneFormOptions(r *http.Request, page *DroneFormPage) {
	page.Frames, _ = a.frameOptions(r, page.FrameID)
	page.FCs, _ = a.fcOptions(r, page.FCID)
	page.ESCs, _ = a.escOptions(r, page.ESCID)
	page.VTXs, _ = a.vtxOptions(r, page.VTXID)
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
		_, err := a.db.Exec(ctx, `
            INSERT INTO drones (name,frame_id,fc_id,esc_id,vtx_id,status,build_date,notes)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			name,
			nullIntPtr(r.FormValue("frame_id")),
			nullIntPtr(r.FormValue("fc_id")),
			nullIntPtr(r.FormValue("esc_id")),
			nullIntPtr(r.FormValue("vtx_id")),
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
	page := DroneFormPage{ActiveTab: "drones", Status: "build"}
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
		_, err := a.db.Exec(ctx, `
            UPDATE drones SET name=$1,frame_id=$2,fc_id=$3,esc_id=$4,vtx_id=$5,
            status=$6,build_date=$7,notes=$8 WHERE id=$9`,
			name,
			nullIntPtr(r.FormValue("frame_id")),
			nullIntPtr(r.FormValue("fc_id")),
			nullIntPtr(r.FormValue("esc_id")),
			nullIntPtr(r.FormValue("vtx_id")),
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
	var frameID, fcID, escID, vtxID *int
	var bd *string
	err := a.db.QueryRow(ctx, `
        SELECT id,name,frame_id,fc_id,esc_id,vtx_id,status,
               TO_CHAR(build_date,'YYYY-MM-DD'),notes
        FROM drones WHERE id=$1`, id).Scan(
		&page.ID, &page.Name, &frameID, &fcID, &escID, &vtxID,
		&page.Status, &bd, &page.Notes)
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

	rows, err := a.db.Query(ctx, `
        SELECT f.id, f.brand, f.name, f.size_mm, f.weight_g, f.status,
               COALESCE(d.name,''),
               (SELECT COUNT(*) FROM frames f2
                WHERE f2.brand=f.brand AND f2.name=f.name
                AND f2.status='spare'
                AND NOT EXISTS (SELECT 1 FROM drones d2 WHERE d2.frame_id=f2.id))
        FROM frames f LEFT JOIN drones d ON d.frame_id=f.id
        ORDER BY (f.status='retired'), d.name NULLS LAST, f.brand, f.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var fr FrameRow
		var sizeMM, weightG *int
		if err := rows.Scan(&fr.ID, &fr.Brand, &fr.Name, &sizeMM, &weightG, &fr.Status, &fr.InstalledOn, &fr.Available); err != nil {
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
        SELECT fc.id, fc.brand, fc.name, fc.mcu, fc.firmware, fc.status,
               COALESCE(d.name,''),
               (SELECT COUNT(*) FROM flight_controllers fc2
                WHERE fc2.brand=fc.brand AND fc2.name=fc.name
                AND fc2.status='spare'
                AND NOT EXISTS (SELECT 1 FROM drones d2 WHERE d2.fc_id=fc2.id))
        FROM flight_controllers fc LEFT JOIN drones d ON d.fc_id=fc.id
        ORDER BY (fc.status='retired'), d.name NULLS LAST, fc.brand, fc.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var fc FCRow
		if err := rows.Scan(&fc.ID, &fc.Brand, &fc.Name, &fc.MCU, &fc.Firmware, &fc.Status, &fc.InstalledOn, &fc.Available); err != nil {
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
        SELECT e.id, e.brand, e.name, e.current_rating, e.cell_max, e.status,
               COALESCE(d.name,''),
               (SELECT COUNT(*) FROM escs e2
                WHERE e2.brand=e.brand AND e2.name=e.name
                AND e2.status='spare'
                AND NOT EXISTS (SELECT 1 FROM drones d2 WHERE d2.esc_id=e2.id))
        FROM escs e LEFT JOIN drones d ON d.esc_id=e.id
        ORDER BY (e.status='retired'), d.name NULLS LAST, e.brand, e.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var e ESCRow
		var cr, cm *int
		if err := rows.Scan(&e.ID, &e.Brand, &e.Name, &cr, &cm, &e.Status, &e.InstalledOn, &e.Available); err != nil {
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
        SELECT m.id, m.brand, m.name, m.stator_size, m.kv, m.status,
               COALESCE(d.name,''),
               (SELECT COUNT(*) FROM motors m2
                WHERE m2.brand=m.brand AND m2.name=m.name
                AND m2.status='spare' AND m2.drone_id IS NULL)
        FROM motors m LEFT JOIN drones d ON d.id=m.drone_id
        ORDER BY (m.status='retired'), d.name NULLS LAST, m.brand, m.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var m MotorRow
		var kv *int
		if err := rows.Scan(&m.ID, &m.Brand, &m.Name, &m.StatorSize, &kv, &m.Status, &m.InstalledOn, &m.Available); err != nil {
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
        SELECT v.id, v.brand, v.name, v.system, v.max_power_mw, v.resolution, v.status,
               COALESCE(d.name,''),
               (SELECT COUNT(*) FROM vtx_units v2
                WHERE v2.brand=v.brand AND v2.name=v.name
                AND v2.status='spare'
                AND NOT EXISTS (SELECT 1 FROM drones d2 WHERE d2.vtx_id=v2.id))
        FROM vtx_units v LEFT JOIN drones d ON d.vtx_id=v.id
        ORDER BY (v.status='retired'), d.name NULLS LAST, v.brand, v.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	for rows.Next() {
		var v VTXRow
		var mw *int
		if err := rows.Scan(&v.ID, &v.Brand, &v.Name, &v.System, &mw, &v.Resolution, &v.Status, &v.InstalledOn, &v.Available); err != nil {
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
			render(w, "frame-form", FrameFormPage{ActiveTab: "inventory", Error: "Name is required", Status: "spare"})
			return
		}
		_, err := a.db.Exec(ctx,
			`INSERT INTO frames (brand,name,size_mm,weight_g,notes,status) VALUES ($1,$2,$3,$4,$5,$6)`,
			r.FormValue("brand"), name,
			nullIntPtr(r.FormValue("size_mm")), nullIntPtr(r.FormValue("weight_g")),
			r.FormValue("notes"), r.FormValue("status"))
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	render(w, "frame-form", FrameFormPage{ActiveTab: "inventory", Status: "spare"})
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
		_, err := a.db.Exec(ctx,
			`UPDATE frames SET brand=$1,name=$2,size_mm=$3,weight_g=$4,notes=$5,status=$6 WHERE id=$7`,
			r.FormValue("brand"), name,
			nullIntPtr(r.FormValue("size_mm")), nullIntPtr(r.FormValue("weight_g")),
			r.FormValue("notes"), r.FormValue("status"), id)
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	var page FrameFormPage
	var sizeMM, weightG *int
	err := a.db.QueryRow(ctx, `SELECT id,brand,name,size_mm,weight_g,notes,status FROM frames WHERE id=$1`, id).
		Scan(&page.ID, &page.Brand, &page.Name, &sizeMM, &weightG, &page.Notes, &page.Status)
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
	page.ActiveTab = "inventory"
	render(w, "frame-form", page)
}

func (a *App) handleFrameDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(r.Context(), `DELETE FROM frames WHERE id=$1`, id)
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
			render(w, "fc-form", FCFormPage{ActiveTab: "inventory", Error: "Name is required", Status: "spare"})
			return
		}
		_, err := a.db.Exec(ctx,
			`INSERT INTO flight_controllers (brand,name,mcu,firmware,notes,status) VALUES ($1,$2,$3,$4,$5,$6)`,
			r.FormValue("brand"), name, r.FormValue("mcu"), r.FormValue("firmware"),
			r.FormValue("notes"), r.FormValue("status"))
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	render(w, "fc-form", FCFormPage{ActiveTab: "inventory", Status: "spare"})
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
		_, err := a.db.Exec(ctx,
			`UPDATE flight_controllers SET brand=$1,name=$2,mcu=$3,firmware=$4,notes=$5,status=$6 WHERE id=$7`,
			r.FormValue("brand"), name, r.FormValue("mcu"), r.FormValue("firmware"),
			r.FormValue("notes"), r.FormValue("status"), id)
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	var page FCFormPage
	err := a.db.QueryRow(ctx, `SELECT id,brand,name,mcu,firmware,notes,status FROM flight_controllers WHERE id=$1`, id).
		Scan(&page.ID, &page.Brand, &page.Name, &page.MCU, &page.Firmware, &page.Notes, &page.Status)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	page.ActiveTab = "inventory"
	render(w, "fc-form", page)
}

func (a *App) handleFCDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(r.Context(), `DELETE FROM flight_controllers WHERE id=$1`, id)
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
			render(w, "esc-form", ESCFormPage{ActiveTab: "inventory", Error: "Name is required", Status: "spare"})
			return
		}
		_, err := a.db.Exec(ctx,
			`INSERT INTO escs (brand,name,current_rating,cell_max,notes,status) VALUES ($1,$2,$3,$4,$5,$6)`,
			r.FormValue("brand"), name,
			nullIntPtr(r.FormValue("current_rating")), nullIntPtr(r.FormValue("cell_max")),
			r.FormValue("notes"), r.FormValue("status"))
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	render(w, "esc-form", ESCFormPage{ActiveTab: "inventory", Status: "spare"})
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
		_, err := a.db.Exec(ctx,
			`UPDATE escs SET brand=$1,name=$2,current_rating=$3,cell_max=$4,notes=$5,status=$6 WHERE id=$7`,
			r.FormValue("brand"), name,
			nullIntPtr(r.FormValue("current_rating")), nullIntPtr(r.FormValue("cell_max")),
			r.FormValue("notes"), r.FormValue("status"), id)
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	var page ESCFormPage
	var cr, cm *int
	err := a.db.QueryRow(ctx, `SELECT id,brand,name,current_rating,cell_max,notes,status FROM escs WHERE id=$1`, id).
		Scan(&page.ID, &page.Brand, &page.Name, &cr, &cm, &page.Notes, &page.Status)
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
	page.ActiveTab = "inventory"
	render(w, "esc-form", page)
}

func (a *App) handleESCDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(r.Context(), `DELETE FROM escs WHERE id=$1`, id)
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
			page := MotorFormPage{ActiveTab: "inventory", Error: "Name is required", Status: "spare"}
			page.Drones, _ = a.droneOptions(r)
			render(w, "motor-form", page)
			return
		}
		_, err := a.db.Exec(ctx,
			`INSERT INTO motors (brand,name,stator_size,kv,drone_id,notes,status) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			r.FormValue("brand"), name, r.FormValue("stator_size"),
			nullIntPtr(r.FormValue("kv")), nullIntPtr(r.FormValue("drone_id")),
			r.FormValue("notes"), r.FormValue("status"))
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	page := MotorFormPage{ActiveTab: "inventory", Status: "spare"}
	page.Drones, _ = a.droneOptions(r)
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
			page := MotorFormPage{ActiveTab: "inventory", Error: "Name is required", ID: id, Status: "spare"}
			page.Drones, _ = a.droneOptions(r)
			render(w, "motor-form", page)
			return
		}
		_, err := a.db.Exec(ctx,
			`UPDATE motors SET brand=$1,name=$2,stator_size=$3,kv=$4,drone_id=$5,notes=$6,status=$7 WHERE id=$8`,
			r.FormValue("brand"), name, r.FormValue("stator_size"),
			nullIntPtr(r.FormValue("kv")), nullIntPtr(r.FormValue("drone_id")),
			r.FormValue("notes"), r.FormValue("status"), id)
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	var page MotorFormPage
	var kv, droneID *int
	err := a.db.QueryRow(ctx, `SELECT id,brand,name,stator_size,kv,drone_id,notes,status FROM motors WHERE id=$1`, id).
		Scan(&page.ID, &page.Brand, &page.Name, &page.StatorSize, &kv, &droneID, &page.Notes, &page.Status)
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
	if droneID != nil {
		page.DroneID = *droneID
	}
	page.ActiveTab = "inventory"
	page.Drones, _ = a.droneOptions(r)
	render(w, "motor-form", page)
}

func (a *App) handleMotorDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(r.Context(), `DELETE FROM motors WHERE id=$1`, id)
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
			render(w, "vtx-form", VTXFormPage{ActiveTab: "inventory", Error: "Name is required", Status: "spare"})
			return
		}
		_, err := a.db.Exec(ctx,
			`INSERT INTO vtx_units (brand,name,system,max_power_mw,resolution,weight_g,notes,status) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			r.FormValue("brand"), name, r.FormValue("system"),
			nullIntPtr(r.FormValue("max_power_mw")), r.FormValue("resolution"),
			nullIntPtr(r.FormValue("weight_g")), r.FormValue("notes"), r.FormValue("status"))
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	render(w, "vtx-form", VTXFormPage{ActiveTab: "inventory", Status: "spare"})
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
		_, err := a.db.Exec(ctx,
			`UPDATE vtx_units SET brand=$1,name=$2,system=$3,max_power_mw=$4,resolution=$5,weight_g=$6,notes=$7,status=$8 WHERE id=$9`,
			r.FormValue("brand"), name, r.FormValue("system"),
			nullIntPtr(r.FormValue("max_power_mw")), r.FormValue("resolution"),
			nullIntPtr(r.FormValue("weight_g")), r.FormValue("notes"), r.FormValue("status"), id)
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/inventory", http.StatusSeeOther)
		return
	}
	var page VTXFormPage
	var mw, wg *int
	err := a.db.QueryRow(ctx, `SELECT id,brand,name,system,max_power_mw,resolution,weight_g,notes,status FROM vtx_units WHERE id=$1`, id).
		Scan(&page.ID, &page.Brand, &page.Name, &page.System, &mw, &page.Resolution, &wg, &page.Notes, &page.Status)
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
	page.ActiveTab = "inventory"
	render(w, "vtx-form", page)
}

func (a *App) handleVTXDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(r.Context(), `DELETE FROM vtx_units WHERE id=$1`, id)
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
        SELECT b.id, b.name, b.brand, b.cell_count, b.capacity_mah,
               b.cycle_count, b.internal_resistance,
               TO_CHAR(b.purchase_date,'YYYY-MM-DD'), COALESCE(d.name,''), b.status
        FROM batteries b LEFT JOIN drones d ON d.id=b.drone_id
        ORDER BY b.name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	defer rows.Close()
	var bats []BatteryRow
	for rows.Next() {
		var b BatteryRow
		var ir *int
		var pd *string
		if err := rows.Scan(&b.ID, &b.Name, &b.Brand, &b.CellCount, &b.CapacityMAh,
			&b.CycleCount, &ir, &pd, &b.DroneName, &b.Status); err != nil {
			httpErr(w, err)
			return
		}
		if ir != nil {
			b.InternalResistance = strconv.Itoa(*ir)
		}
		if pd != nil {
			b.PurchaseDate = *pd
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
			page := BatteryFormPage{ActiveTab: "batteries",
				Error: "Name, cell count, and capacity are required", Status: "good"}
			page.Drones, _ = a.droneOptions(r)
			render(w, "battery-form", page)
			return
		}
		cycles, _ := strconv.Atoi(r.FormValue("cycle_count"))
		_, err := a.db.Exec(ctx,
			`INSERT INTO batteries (name,brand,cell_count,capacity_mah,cycle_count,
             internal_resistance,purchase_date,drone_id,status) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			name, r.FormValue("brand"), cc, cap, cycles,
			nullIntPtr(r.FormValue("internal_resistance")),
			nullDate(r.FormValue("purchase_date")),
			nullIntPtr(r.FormValue("drone_id")), r.FormValue("status"))
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/batteries", http.StatusSeeOther)
		return
	}
	page := BatteryFormPage{ActiveTab: "batteries", Status: "good"}
	page.Drones, _ = a.droneOptions(r)
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
		cc, _ := strconv.Atoi(r.FormValue("cell_count"))
		cap, _ := strconv.Atoi(r.FormValue("capacity_mah"))
		if name == "" || cc == 0 || cap == 0 {
			page := BatteryFormPage{ActiveTab: "batteries",
				Error: "Name, cell count, and capacity are required", ID: id, Status: "good"}
			page.Drones, _ = a.droneOptions(r)
			render(w, "battery-form", page)
			return
		}
		cycles, _ := strconv.Atoi(r.FormValue("cycle_count"))
		_, err := a.db.Exec(ctx,
			`UPDATE batteries SET name=$1,brand=$2,cell_count=$3,capacity_mah=$4,cycle_count=$5,
             internal_resistance=$6,purchase_date=$7,drone_id=$8,status=$9 WHERE id=$10`,
			name, r.FormValue("brand"), cc, cap, cycles,
			nullIntPtr(r.FormValue("internal_resistance")),
			nullDate(r.FormValue("purchase_date")),
			nullIntPtr(r.FormValue("drone_id")), r.FormValue("status"), id)
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/batteries", http.StatusSeeOther)
		return
	}
	var page BatteryFormPage
	var ir *int
	var pd *string
	var droneID *int
	var cellCount, capMAh, cycleCount int
	err := a.db.QueryRow(ctx,
		`SELECT id,name,brand,cell_count,capacity_mah,cycle_count,internal_resistance,
         TO_CHAR(purchase_date,'YYYY-MM-DD'),drone_id,status FROM batteries WHERE id=$1`, id).
		Scan(&page.ID, &page.Name, &page.Brand, &cellCount, &capMAh,
			&cycleCount, &ir, &pd, &droneID, &page.Status)
	page.CellCount = strconv.Itoa(cellCount)
	page.CapacityMAh = strconv.Itoa(capMAh)
	page.CycleCount = strconv.Itoa(cycleCount)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	if ir != nil {
		page.InternalResistance = strconv.Itoa(*ir)
	}
	if pd != nil {
		page.PurchaseDate = *pd
	}
	if droneID != nil {
		page.DroneID = *droneID
	}
	page.ActiveTab = "batteries"
	page.Drones, _ = a.droneOptions(r)
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

// ---- Spare Parts ----

func (a *App) handleParts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx,
		`SELECT id,category,name,quantity,reorder_threshold,unit_price_cents FROM spare_parts ORDER BY category,name`)
	if err != nil {
		httpErr(w, err)
		return
	}
	defer rows.Close()
	var parts []PartRow
	for rows.Next() {
		var p PartRow
		var cents int
		if err := rows.Scan(&p.ID, &p.Category, &p.Name, &p.Quantity, &p.ReorderThreshold, &cents); err != nil {
			httpErr(w, err)
			return
		}
		p.UnitPrice = fmt.Sprintf("$%.2f", float64(cents)/100)
		p.LowStock = p.ReorderThreshold > 0 && p.Quantity <= p.ReorderThreshold
		parts = append(parts, p)
	}
	if err := rows.Err(); err != nil {
		httpErr(w, err)
		return
	}
	render(w, "parts-list", PartsListPage{ActiveTab: "parts", Parts: parts})
}

func (a *App) handlePartNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		cat := r.FormValue("category")
		if name == "" {
			render(w, "part-form", PartFormPage{ActiveTab: "parts", Error: "Name is required", Category: cat})
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		rt, _ := strconv.Atoi(r.FormValue("reorder_threshold"))
		_, err := a.db.Exec(ctx,
			`INSERT INTO spare_parts (category,name,quantity,reorder_threshold,unit_price_cents) VALUES ($1,$2,$3,$4,$5)`,
			cat, name, qty, rt, parsePrice(r.FormValue("unit_price")))
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/parts", http.StatusSeeOther)
		return
	}
	render(w, "part-form", PartFormPage{ActiveTab: "parts", Category: "antennas"})
}

func (a *App) handlePartEdit(w http.ResponseWriter, r *http.Request) {
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
		cat := r.FormValue("category")
		if name == "" {
			render(w, "part-form", PartFormPage{ActiveTab: "parts", Error: "Name is required", ID: id, Category: cat})
			return
		}
		qty, _ := strconv.Atoi(r.FormValue("quantity"))
		rt, _ := strconv.Atoi(r.FormValue("reorder_threshold"))
		_, err := a.db.Exec(ctx,
			`UPDATE spare_parts SET category=$1,name=$2,quantity=$3,reorder_threshold=$4,unit_price_cents=$5 WHERE id=$6`,
			cat, name, qty, rt, parsePrice(r.FormValue("unit_price")), id)
		if err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/parts", http.StatusSeeOther)
		return
	}
	var page PartFormPage
	var cents, qty, rt int
	err := a.db.QueryRow(ctx,
		`SELECT id,category,name,quantity,reorder_threshold,unit_price_cents FROM spare_parts WHERE id=$1`, id).
		Scan(&page.ID, &page.Category, &page.Name, &qty, &rt, &cents)
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
	page.UnitPrice = fmt.Sprintf("%.2f", float64(cents)/100)
	page.ActiveTab = "parts"
	render(w, "part-form", page)
}

func (a *App) handlePartDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(r.Context(), `DELETE FROM spare_parts WHERE id=$1`, id)
	http.Redirect(w, r, "/parts", http.StatusSeeOther)
}

// ---- Sessions / Log ----

func (a *App) handleLog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := a.db.Query(ctx, `
        SELECT s.id, d.name, s.type,
               TO_CHAR(s.session_date,'YYYY-MM-DD'),
               s.duration_min, s.location, s.notes,
               COALESCE(string_agg(b.name||' ('||b.cell_count||'S)', ', ' ORDER BY b.name),'')
        FROM sessions s
        JOIN drones d ON d.id=s.drone_id
        LEFT JOIN session_batteries sb ON sb.session_id=s.id
        LEFT JOIN batteries b ON b.id=sb.battery_id
        GROUP BY s.id, d.name, s.type, s.session_date, s.duration_min, s.location, s.notes, s.created_at
        ORDER BY s.session_date DESC, s.created_at DESC`)
	if err != nil {
		httpErr(w, err)
		return
	}
	defer rows.Close()
	var sessions []SessionRow
	for rows.Next() {
		var s SessionRow
		if err := rows.Scan(&s.ID, &s.DroneName, &s.Type, &s.SessionDate,
			&s.DurationMin, &s.Location, &s.Notes, &s.BatteryList); err != nil {
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
		droneID, err := strconv.Atoi(r.FormValue("drone_id"))
		if err != nil || droneID == 0 {
			page := SessionFormPage{ActiveTab: "log", Error: "Select a drone", Type: "flight"}
			page.Drones, _ = a.droneOptions(r)
			page.Batteries, _ = a.batteryChecks(r, 0)
			render(w, "session-form", page)
			return
		}
		dur, _ := strconv.Atoi(r.FormValue("duration_min"))
		sd := r.FormValue("session_date")

		tx, err := a.db.Begin(ctx)
		if err != nil {
			httpErr(w, err)
			return
		}
		defer tx.Rollback(ctx)

		var sessionID int
		err = tx.QueryRow(ctx,
			`INSERT INTO sessions (drone_id,type,session_date,duration_min,location,notes)
             VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
			droneID, r.FormValue("type"), nullDate(sd), dur,
			r.FormValue("location"), r.FormValue("notes")).Scan(&sessionID)
		if err != nil {
			httpErr(w, err)
			return
		}
		for _, batIDStr := range r.Form["battery_ids"] {
			batID, err := strconv.Atoi(batIDStr)
			if err != nil {
				continue
			}
			tx.Exec(ctx, `INSERT INTO session_batteries (session_id,battery_id) VALUES ($1,$2)`, sessionID, batID)
			tx.Exec(ctx, `UPDATE batteries SET cycle_count=cycle_count+1 WHERE id=$1`, batID)
		}
		if err := tx.Commit(ctx); err != nil {
			httpErr(w, err)
			return
		}
		http.Redirect(w, r, "/log", http.StatusSeeOther)
		return
	}
	page := SessionFormPage{ActiveTab: "log", Type: "flight"}
	page.Drones, _ = a.droneOptions(r)
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
        SELECT s.id, d.name, s.type, TO_CHAR(s.session_date,'YYYY-MM-DD'),
               s.duration_min, s.location, s.notes
        FROM sessions s JOIN drones d ON d.id=s.drone_id WHERE s.id=$1`, id).
		Scan(&page.ID, &page.DroneName, &page.Type, &page.Date,
			&page.Duration, &page.Location, &page.Notes)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	rows, err := a.db.Query(ctx, `
        SELECT b.id, b.name, b.brand, b.cell_count, b.capacity_mah, b.cycle_count, b.status
        FROM batteries b JOIN session_batteries sb ON sb.battery_id=b.id
        WHERE sb.session_id=$1 ORDER BY b.name`, id)
	if err != nil {
		httpErr(w, err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var b BatteryRow
		if err := rows.Scan(&b.ID, &b.Name, &b.Brand, &b.CellCount, &b.CapacityMAh, &b.CycleCount, &b.Status); err != nil {
			httpErr(w, err)
			return
		}
		page.Batteries = append(page.Batteries, b)
	}
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
		droneID, err := strconv.Atoi(r.FormValue("drone_id"))
		if err != nil || droneID == 0 {
			page := SessionFormPage{ActiveTab: "log", Error: "Select a drone", ID: id}
			page.Drones, _ = a.droneOptions(r)
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

		oldBatIDs, err := sessionBatteryIDs(ctx, tx, id)
		if err != nil {
			httpErr(w, err)
			return
		}

		for _, bid := range setDiff(newBatIDs, oldBatIDs) {
			tx.Exec(ctx, `UPDATE batteries SET cycle_count=cycle_count+1 WHERE id=$1`, bid)
		}
		for _, bid := range setDiff(oldBatIDs, newBatIDs) {
			tx.Exec(ctx, `UPDATE batteries SET cycle_count=GREATEST(0,cycle_count-1) WHERE id=$1`, bid)
		}

		tx.Exec(ctx, `DELETE FROM session_batteries WHERE session_id=$1`, id)
		for _, bid := range newBatIDs {
			tx.Exec(ctx, `INSERT INTO session_batteries (session_id,battery_id) VALUES ($1,$2)`, id, bid)
		}

		_, err = tx.Exec(ctx,
			`UPDATE sessions SET drone_id=$1,type=$2,session_date=$3,duration_min=$4,location=$5,notes=$6 WHERE id=$7`,
			droneID, r.FormValue("type"), nullDate(r.FormValue("session_date")), dur,
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
	var droneID, dur int
	err := a.db.QueryRow(ctx,
		`SELECT id,drone_id,type,TO_CHAR(session_date,'YYYY-MM-DD'),duration_min,location,notes FROM sessions WHERE id=$1`, id).
		Scan(&page.ID, &droneID, &page.Type, &page.SessionDate, &dur, &page.Location, &page.Notes)
	page.DurationMin = strconv.Itoa(dur)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	page.DroneID = droneID
	page.ActiveTab = "log"
	page.Drones, _ = a.droneOptions(r)
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

	tx.Exec(ctx,
		`UPDATE batteries SET cycle_count=GREATEST(0,cycle_count-1)
         WHERE id IN (SELECT battery_id FROM session_batteries WHERE session_id=$1)`, id)
	tx.Exec(ctx, `DELETE FROM sessions WHERE id=$1`, id)

	if err := tx.Commit(ctx); err != nil {
		httpErr(w, err)
		return
	}
	http.Redirect(w, r, "/log", http.StatusSeeOther)
}

// ---- Adjust handlers (add or remove spare items) ----
// Positive count: insert N spare copies (cloned from this item).
// Negative count: delete abs(N) spare units of the same brand+name.

func adjustCount(r *http.Request) (int, bool) {
	if err := r.ParseForm(); err != nil {
		return 0, false
	}
	n, err := strconv.Atoi(r.FormValue("count"))
	return n, err == nil && n != 0
}

func (a *App) handleFrameAdjust(w http.ResponseWriter, r *http.Request) {
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
	var err error
	if n > 0 {
		_, err = a.db.Exec(ctx, `
            INSERT INTO frames (brand,name,size_mm,weight_g,notes,status)
            SELECT brand,name,size_mm,weight_g,notes,'spare'
            FROM frames, generate_series(1,$2) WHERE frames.id=$1`, id, n)
	} else {
		_, err = a.db.Exec(ctx, `
            DELETE FROM frames WHERE id IN (
                SELECT f2.id FROM frames f
                JOIN frames f2 ON f2.brand=f.brand AND f2.name=f.name
                WHERE f.id=$1 AND f2.status='spare'
                AND NOT EXISTS (SELECT 1 FROM drones d WHERE d.frame_id=f2.id)
                ORDER BY f2.id LIMIT $2)`, id, -n)
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	http.Redirect(w, r, "/inventory", http.StatusSeeOther)
}

func (a *App) handleFCAdjust(w http.ResponseWriter, r *http.Request) {
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
	var err error
	if n > 0 {
		_, err = a.db.Exec(ctx, `
            INSERT INTO flight_controllers (brand,name,mcu,firmware,notes,status)
            SELECT brand,name,mcu,firmware,notes,'spare'
            FROM flight_controllers, generate_series(1,$2) WHERE flight_controllers.id=$1`, id, n)
	} else {
		_, err = a.db.Exec(ctx, `
            DELETE FROM flight_controllers WHERE id IN (
                SELECT fc2.id FROM flight_controllers fc
                JOIN flight_controllers fc2 ON fc2.brand=fc.brand AND fc2.name=fc.name
                WHERE fc.id=$1 AND fc2.status='spare'
                AND NOT EXISTS (SELECT 1 FROM drones d WHERE d.fc_id=fc2.id)
                ORDER BY fc2.id LIMIT $2)`, id, -n)
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	http.Redirect(w, r, "/inventory", http.StatusSeeOther)
}

func (a *App) handleESCAdjust(w http.ResponseWriter, r *http.Request) {
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
	var err error
	if n > 0 {
		_, err = a.db.Exec(ctx, `
            INSERT INTO escs (brand,name,current_rating,cell_max,notes,status)
            SELECT brand,name,current_rating,cell_max,notes,'spare'
            FROM escs, generate_series(1,$2) WHERE escs.id=$1`, id, n)
	} else {
		_, err = a.db.Exec(ctx, `
            DELETE FROM escs WHERE id IN (
                SELECT e2.id FROM escs e
                JOIN escs e2 ON e2.brand=e.brand AND e2.name=e.name
                WHERE e.id=$1 AND e2.status='spare'
                AND NOT EXISTS (SELECT 1 FROM drones d WHERE d.esc_id=e2.id)
                ORDER BY e2.id LIMIT $2)`, id, -n)
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	http.Redirect(w, r, "/inventory", http.StatusSeeOther)
}

func (a *App) handleMotorAdjust(w http.ResponseWriter, r *http.Request) {
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
	var err error
	if n > 0 {
		_, err = a.db.Exec(ctx, `
            INSERT INTO motors (brand,name,stator_size,kv,notes,status)
            SELECT brand,name,stator_size,kv,notes,'spare'
            FROM motors, generate_series(1,$2) WHERE motors.id=$1`, id, n)
	} else {
		_, err = a.db.Exec(ctx, `
            DELETE FROM motors WHERE id IN (
                SELECT m2.id FROM motors m
                JOIN motors m2 ON m2.brand=m.brand AND m2.name=m.name
                WHERE m.id=$1 AND m2.status='spare' AND m2.drone_id IS NULL
                ORDER BY m2.id LIMIT $2)`, id, -n)
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	http.Redirect(w, r, "/inventory", http.StatusSeeOther)
}

func (a *App) handleVTXAdjust(w http.ResponseWriter, r *http.Request) {
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
	var err error
	if n > 0 {
		_, err = a.db.Exec(ctx, `
            INSERT INTO vtx_units (brand,name,system,max_power_mw,resolution,weight_g,notes,status)
            SELECT brand,name,system,max_power_mw,resolution,weight_g,notes,'spare'
            FROM vtx_units, generate_series(1,$2) WHERE vtx_units.id=$1`, id, n)
	} else {
		_, err = a.db.Exec(ctx, `
            DELETE FROM vtx_units WHERE id IN (
                SELECT v2.id FROM vtx_units v
                JOIN vtx_units v2 ON v2.brand=v.brand AND v2.name=v.name
                WHERE v.id=$1 AND v2.status='spare'
                AND NOT EXISTS (SELECT 1 FROM drones d WHERE d.vtx_id=v2.id)
                ORDER BY v2.id LIMIT $2)`, id, -n)
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	http.Redirect(w, r, "/inventory", http.StatusSeeOther)
}
