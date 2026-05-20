package main

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Queries struct {
	db *pgxpool.Pool
}

// ---- Drones ----

type DroneRow struct {
	ID         int
	Name       string
	Status     string
	BuildDate  *string
	SizeLabel  string
	CellLabel  string
	FrameBrand string
	FrameName  string
	FCBrand    string
	FCName     string
	ESCBrand   string
	ESCName    string
	VTXBrand   string
	VTXName    string
	MotorBrand string
	MotorName  string
	MotorCount int
	BattNames  string
	GPSBrand   string
	GPSName    string
	RXBrand    string
	RXName     string
	WeightG    *int
	Notes      string
}

const droneSelect = `
SELECT d.id, d.name, d.status, TO_CHAR(d.build_date,'YYYY-MM-DD'),
  COALESCE(sz.label,''), COALESCE(c.label,''),
  COALESCE(bf.name,''), COALESCE(f.name,''),
  COALESCE(bfc.name,''), COALESCE(fc.name,''),
  COALESCE(be.name,''), COALESCE(e.name,''),
  COALESCE(bv.name,''), COALESCE(v.name,''),
  COALESCE(bm.name,''), COALESCE(m.name,''), d.motor_count,
  COALESCE((SELECT string_agg(
      COALESCE(bb2.name||' ','')||b2.name||' ('||COALESCE(c2.label,'?')||' '||b2.capacity_mah||'mAh)',
      ', ' ORDER BY b2.name)
    FROM drone_batteries db2
    JOIN batteries b2 ON b2.id=db2.battery_id
    LEFT JOIN brands bb2 ON bb2.id=b2.brand_id
    LEFT JOIN cells c2 ON c2.id=b2.cell_id
    WHERE db2.drone_id=d.id),''),
  COALESCE(bg.name,''), COALESCE(g.name,''),
  COALESCE(brx.name,''), COALESCE(rx.name,''),
  d.weight_g, d.notes
FROM drones d
LEFT JOIN sizes sz ON sz.id=d.size_id
LEFT JOIN cells c ON c.id=d.cell_id
LEFT JOIN frames f ON f.id=d.frame_id
LEFT JOIN brands bf ON bf.id=f.brand_id
LEFT JOIN flight_controllers fc ON fc.id=d.fc_id
LEFT JOIN brands bfc ON bfc.id=fc.brand_id
LEFT JOIN escs e ON e.id=d.esc_id
LEFT JOIN brands be ON be.id=e.brand_id
LEFT JOIN vtx_units v ON v.id=d.vtx_id
LEFT JOIN brands bv ON bv.id=v.brand_id
LEFT JOIN motors m ON m.id=d.motor_id
LEFT JOIN brands bm ON bm.id=m.brand_id
LEFT JOIN gps_modules g ON g.id=d.gps_id
LEFT JOIN brands bg ON bg.id=g.brand_id
LEFT JOIN radio_receivers rx ON rx.id=d.rx_id
LEFT JOIN brands brx ON brx.id=rx.brand_id`

func scanDrone(fn func(...any) error) (DroneRow, error) {
	var d DroneRow
	err := fn(
		&d.ID, &d.Name, &d.Status, &d.BuildDate,
		&d.SizeLabel, &d.CellLabel,
		&d.FrameBrand, &d.FrameName,
		&d.FCBrand, &d.FCName,
		&d.ESCBrand, &d.ESCName,
		&d.VTXBrand, &d.VTXName,
		&d.MotorBrand, &d.MotorName, &d.MotorCount,
		&d.BattNames,
		&d.GPSBrand, &d.GPSName,
		&d.RXBrand, &d.RXName,
		&d.WeightG, &d.Notes,
	)
	return d, err
}

func (q *Queries) ListDrones(ctx context.Context) ([]DroneRow, error) {
	rows, err := q.db.Query(ctx, droneSelect+" ORDER BY d.name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DroneRow
	for rows.Next() {
		d, err := scanDrone(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (q *Queries) GetDrone(ctx context.Context, id int) (DroneRow, error) {
	return scanDrone(q.db.QueryRow(ctx, droneSelect+" WHERE d.id=$1", id).Scan)
}

// ---- Sessions ----

type SessionRow struct {
	ID          int
	Title       string
	Type        string
	SessionDate string
	DurationMin int
	Location    string
	Notes       string
	DroneNames  string
	BatteryList string
}

type SessionDrone struct {
	ID   int
	Name string
}

type SessionBattery struct {
	ID          int
	Brand       string
	Name        string
	CellLabel   string
	CapacityMAh int
	Count       int
}

type SessionDetail struct {
	Session   SessionRow
	Drones    []SessionDrone
	Batteries []SessionBattery
}

func (q *Queries) ListSessions(ctx context.Context, limit int) ([]SessionRow, error) {
	if limit <= 0 {
		limit = 20
	}
	const stmt = `
SELECT s.id, s.title, s.type, TO_CHAR(s.session_date,'YYYY-MM-DD'),
  s.duration_min, s.location, s.notes,
  COALESCE((SELECT string_agg(d.name,', ' ORDER BY d.name)
            FROM session_drones sd JOIN drones d ON d.id=sd.drone_id
            WHERE sd.session_id=s.id),''),
  COALESCE((SELECT string_agg(
      COALESCE(bb.name||' ','')||b.name||' ('||COALESCE(c.label,'?')||')',
      ', ' ORDER BY bb.name, b.name)
            FROM session_batteries sb JOIN batteries b ON b.id=sb.battery_id
            LEFT JOIN brands bb ON bb.id=b.brand_id
            LEFT JOIN cells c ON c.id=b.cell_id
            WHERE sb.session_id=s.id),'')
FROM sessions s
ORDER BY s.session_date DESC, s.created_at DESC
LIMIT $1`
	rows, err := q.db.Query(ctx, stmt, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SessionRow
	for rows.Next() {
		var s SessionRow
		if err := rows.Scan(&s.ID, &s.Title, &s.Type, &s.SessionDate,
			&s.DurationMin, &s.Location, &s.Notes, &s.DroneNames, &s.BatteryList); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (q *Queries) GetSession(ctx context.Context, id int) (SessionDetail, error) {
	var d SessionDetail
	err := q.db.QueryRow(ctx, `
SELECT s.id, s.title, s.type, TO_CHAR(s.session_date,'YYYY-MM-DD'),
  s.duration_min, s.location, s.notes
FROM sessions s WHERE s.id=$1`, id).
		Scan(&d.Session.ID, &d.Session.Title, &d.Session.Type, &d.Session.SessionDate,
			&d.Session.DurationMin, &d.Session.Location, &d.Session.Notes)
	if err != nil {
		return d, err
	}

	drows, err := q.db.Query(ctx, `
SELECT d.id, d.name FROM drones d
JOIN session_drones sd ON sd.drone_id=d.id
WHERE sd.session_id=$1 ORDER BY d.name`, id)
	if err != nil {
		return d, err
	}
	for drows.Next() {
		var sd SessionDrone
		if err := drows.Scan(&sd.ID, &sd.Name); err != nil {
			drows.Close()
			return d, err
		}
		d.Drones = append(d.Drones, sd)
	}
	drows.Close()
	if err := drows.Err(); err != nil {
		return d, err
	}

	brows, err := q.db.Query(ctx, `
SELECT b.id, COALESCE(bb.name,''), b.name, COALESCE(c.label,''), b.capacity_mah, sb.count
FROM batteries b JOIN session_batteries sb ON sb.battery_id=b.id
LEFT JOIN brands bb ON bb.id=b.brand_id
LEFT JOIN cells c ON c.id=b.cell_id
WHERE sb.session_id=$1 ORDER BY bb.name, b.name`, id)
	if err != nil {
		return d, err
	}
	for brows.Next() {
		var sb SessionBattery
		if err := brows.Scan(&sb.ID, &sb.Brand, &sb.Name, &sb.CellLabel, &sb.CapacityMAh, &sb.Count); err != nil {
			brows.Close()
			return d, err
		}
		d.Batteries = append(d.Batteries, sb)
	}
	brows.Close()
	return d, brows.Err()
}

// ---- Batteries ----

type BatteryRow struct {
	ID          int
	Brand       string
	Name        string
	CellLabel   string
	CapacityMAh int
	WeightG     *int
	Count       int
	Status      string
	Notes       string
	AssignedTo  string
}

func (q *Queries) ListBatteries(ctx context.Context) ([]BatteryRow, error) {
	const stmt = `
SELECT b.id, COALESCE(bb.name,''), b.name, COALESCE(c.label,''), b.capacity_mah, b.weight_g,
  b.count, b.status, b.notes,
  COALESCE((SELECT string_agg(d.name,', ' ORDER BY d.name)
            FROM drone_batteries db2 JOIN drones d ON d.id=db2.drone_id
            WHERE db2.battery_id=b.id),'')
FROM batteries b
LEFT JOIN brands bb ON bb.id=b.brand_id
LEFT JOIN cells c ON c.id=b.cell_id
ORDER BY bb.name, b.name, c.label, b.capacity_mah`
	rows, err := q.db.Query(ctx, stmt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BatteryRow
	for rows.Next() {
		var b BatteryRow
		if err := rows.Scan(&b.ID, &b.Brand, &b.Name, &b.CellLabel, &b.CapacityMAh, &b.WeightG,
			&b.Count, &b.Status, &b.Notes, &b.AssignedTo); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ---- Components ----

type ComponentRow struct {
	ID          int
	Type        string
	Brand       string
	Name        string
	Specs       string
	Total       int
	Installed   int
	Available   int
	InstalledOn string
}

func (q *Queries) ListComponents(ctx context.Context, typeFilter string) ([]ComponentRow, error) {
	switch typeFilter {
	case "frame":
		return q.listFrames(ctx)
	case "fc":
		return q.listFCs(ctx)
	case "esc":
		return q.listESCs(ctx)
	case "motor":
		return q.listMotors(ctx)
	case "vtx":
		return q.listVTXs(ctx)
	case "gps":
		return q.listGPS(ctx)
	case "rx":
		return q.listRX(ctx)
	default:
		return q.listAllComponents(ctx)
	}
}

func (q *Queries) listAllComponents(ctx context.Context) ([]ComponentRow, error) {
	var all []ComponentRow
	for _, fn := range []func(context.Context) ([]ComponentRow, error){
		q.listFrames, q.listFCs, q.listESCs, q.listMotors, q.listVTXs, q.listGPS, q.listRX,
	} {
		rows, err := fn(ctx)
		if err != nil {
			return nil, err
		}
		all = append(all, rows...)
	}
	return all, nil
}

func (q *Queries) scanComponents(ctx context.Context, typeName, stmt string) ([]ComponentRow, error) {
	rows, err := q.db.Query(ctx, stmt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ComponentRow
	for rows.Next() {
		var c ComponentRow
		c.Type = typeName
		if err := rows.Scan(&c.ID, &c.Brand, &c.Name, &c.Specs,
			&c.Total, &c.Installed, &c.Available, &c.InstalledOn); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (q *Queries) listFrames(ctx context.Context) ([]ComponentRow, error) {
	return q.scanComponents(ctx, "frame", `
SELECT f.id, COALESCE(bf.name,''), f.name,
  TRIM(COALESCE(sz.label||'"','') || CASE WHEN f.weight_g IS NOT NULL THEN ' '||f.weight_g||'g' ELSE '' END),
  COALESCE(ic.count,0), di.installed, COALESCE(ic.count,0)-di.installed, di.names
FROM frames f
LEFT JOIN brands bf ON bf.id=f.brand_id
LEFT JOIN sizes sz ON sz.id=f.size_id
LEFT JOIN item_counts ic ON ic.item_type='frame' AND ic.item_id=f.id
LEFT JOIN LATERAL (SELECT COUNT(*)::int AS installed, COALESCE(string_agg(d.name,', ' ORDER BY d.name),'') AS names FROM drones d WHERE d.frame_id=f.id) di ON true
ORDER BY bf.name, f.name`)
}

func (q *Queries) listFCs(ctx context.Context) ([]ComponentRow, error) {
	return q.scanComponents(ctx, "fc", `
SELECT fc.id, COALESCE(bfc.name,''), fc.name,
  TRIM(fc.mcu || CASE WHEN fc.firmware!='' THEN ' '||fc.firmware ELSE '' END),
  COALESCE(ic.count,0), di.installed, COALESCE(ic.count,0)-di.installed, di.names
FROM flight_controllers fc
LEFT JOIN brands bfc ON bfc.id=fc.brand_id
LEFT JOIN item_counts ic ON ic.item_type='fc' AND ic.item_id=fc.id
LEFT JOIN LATERAL (SELECT COUNT(*)::int AS installed, COALESCE(string_agg(d.name,', ' ORDER BY d.name),'') AS names FROM drones d WHERE d.fc_id=fc.id) di ON true
ORDER BY bfc.name, fc.name`)
}

func (q *Queries) listESCs(ctx context.Context) ([]ComponentRow, error) {
	return q.scanComponents(ctx, "esc", `
SELECT e.id, COALESCE(be.name,''), e.name,
  TRIM(
    CASE WHEN e.current_rating IS NOT NULL THEN e.current_rating::text||'A' ELSE '' END ||
    CASE WHEN e.cell_max IS NOT NULL THEN ' '||e.cell_max::text||'S max' ELSE '' END
  ),
  COALESCE(ic.count,0), di.installed, COALESCE(ic.count,0)-di.installed, di.names
FROM escs e
LEFT JOIN brands be ON be.id=e.brand_id
LEFT JOIN item_counts ic ON ic.item_type='esc' AND ic.item_id=e.id
LEFT JOIN LATERAL (SELECT COUNT(*)::int AS installed, COALESCE(string_agg(d.name,', ' ORDER BY d.name),'') AS names FROM drones d WHERE d.esc_id=e.id) di ON true
ORDER BY be.name, e.name`)
}

func (q *Queries) listMotors(ctx context.Context) ([]ComponentRow, error) {
	return q.scanComponents(ctx, "motor", `
SELECT m.id, COALESCE(bm.name,''), m.name,
  TRIM(m.stator_size || CASE WHEN m.kv IS NOT NULL THEN ' '||m.kv::text||'KV' ELSE '' END),
  COALESCE(ic.count,0), di.installed, COALESCE(ic.count,0)-di.installed, di.names
FROM motors m
LEFT JOIN brands bm ON bm.id=m.brand_id
LEFT JOIN item_counts ic ON ic.item_type='motor' AND ic.item_id=m.id
LEFT JOIN LATERAL (SELECT COALESCE(SUM(d.motor_count),0)::int AS installed, COALESCE(string_agg(d.name,', ' ORDER BY d.name),'') AS names FROM drones d WHERE d.motor_id=m.id) di ON true
ORDER BY bm.name, m.name`)
}

func (q *Queries) listVTXs(ctx context.Context) ([]ComponentRow, error) {
	return q.scanComponents(ctx, "vtx", `
SELECT v.id, COALESCE(bv.name,''), v.name,
  TRIM(
    v.system ||
    CASE WHEN v.max_power_mw IS NOT NULL THEN ' '||v.max_power_mw::text||'mW' ELSE '' END ||
    CASE WHEN v.resolution!='' THEN ' '||v.resolution ELSE '' END
  ),
  COALESCE(ic.count,0), di.installed, COALESCE(ic.count,0)-di.installed, di.names
FROM vtx_units v
LEFT JOIN brands bv ON bv.id=v.brand_id
LEFT JOIN item_counts ic ON ic.item_type='vtx' AND ic.item_id=v.id
LEFT JOIN LATERAL (SELECT COUNT(*)::int AS installed, COALESCE(string_agg(d.name,', ' ORDER BY d.name),'') AS names FROM drones d WHERE d.vtx_id=v.id) di ON true
ORDER BY bv.name, v.name`)
}

func (q *Queries) listGPS(ctx context.Context) ([]ComponentRow, error) {
	return q.scanComponents(ctx, "gps", `
SELECT g.id, COALESCE(bg.name,''), g.name, '',
  COALESCE(ic.count,0), di.installed, COALESCE(ic.count,0)-di.installed, di.names
FROM gps_modules g
LEFT JOIN brands bg ON bg.id=g.brand_id
LEFT JOIN item_counts ic ON ic.item_type='gps' AND ic.item_id=g.id
LEFT JOIN LATERAL (SELECT COUNT(*)::int AS installed, COALESCE(string_agg(d.name,', ' ORDER BY d.name),'') AS names FROM drones d WHERE d.gps_id=g.id) di ON true
ORDER BY bg.name, g.name`)
}

func (q *Queries) listRX(ctx context.Context) ([]ComponentRow, error) {
	return q.scanComponents(ctx, "rx", `
SELECT rx.id, COALESCE(brx.name,''), rx.name, rx.protocol,
  COALESCE(ic.count,0), di.installed, COALESCE(ic.count,0)-di.installed, di.names
FROM radio_receivers rx
LEFT JOIN brands brx ON brx.id=rx.brand_id
LEFT JOIN item_counts ic ON ic.item_type='rx' AND ic.item_id=rx.id
LEFT JOIN LATERAL (SELECT COUNT(*)::int AS installed, COALESCE(string_agg(d.name,', ' ORDER BY d.name),'') AS names FROM drones d WHERE d.rx_id=rx.id) di ON true
ORDER BY brx.name, rx.name`)
}

// ---- Drone logs ----

type DroneLogEntry struct {
	ID       int
	LoggedAt string
	Body     string
}

func (q *Queries) ListDroneLogs(ctx context.Context, droneID int) ([]DroneLogEntry, error) {
	rows, err := q.db.Query(ctx,
		`SELECT id, TO_CHAR(logged_at, 'YYYY-MM-DD HH24:MI'), body
		 FROM drone_log_entries WHERE drone_id=$1 ORDER BY logged_at DESC`, droneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DroneLogEntry
	for rows.Next() {
		var e DroneLogEntry
		if err := rows.Scan(&e.ID, &e.LoggedAt, &e.Body); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ---- Places ----

type PlaceRow struct {
	ID        int
	Name      string
	Address   string
	Lat       *float64
	Lng       *float64
	PlaceType string
	Notes     string
}

func (q *Queries) ListPlaces(ctx context.Context) ([]PlaceRow, error) {
	rows, err := q.db.Query(ctx,
		`SELECT id, name, address, CAST(lat AS FLOAT8), CAST(lng AS FLOAT8), place_type, notes
         FROM places ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PlaceRow
	for rows.Next() {
		var p PlaceRow
		if err := rows.Scan(&p.ID, &p.Name, &p.Address, &p.Lat, &p.Lng, &p.PlaceType, &p.Notes); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ---- Brands ----

type BrandRow struct {
	ID   int
	Name string
}

func (q *Queries) ListBrands(ctx context.Context) ([]BrandRow, error) {
	rows, err := q.db.Query(ctx, `SELECT id, name FROM brands ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BrandRow
	for rows.Next() {
		var b BrandRow
		if err := rows.Scan(&b.ID, &b.Name); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (q *Queries) CreateBrand(ctx context.Context, name string) (BrandRow, error) {
	var b BrandRow
	err := q.db.QueryRow(ctx,
		`INSERT INTO brands (name) VALUES ($1) RETURNING id, name`, name).
		Scan(&b.ID, &b.Name)
	return b, err
}

func (q *Queries) UpdateBrand(ctx context.Context, id int, name string) (BrandRow, error) {
	var b BrandRow
	err := q.db.QueryRow(ctx,
		`UPDATE brands SET name=$1 WHERE id=$2 RETURNING id, name`, name, id).
		Scan(&b.ID, &b.Name)
	return b, err
}

func (q *Queries) DeleteBrand(ctx context.Context, id int) error {
	tag, err := q.db.Exec(ctx, `DELETE FROM brands WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ---- Low Stock ----

type LowPropRow struct {
	ID               int
	Brand            string
	Name             string
	SizeLabel        string
	BladeCount       int
	Quantity         int
	ReorderThreshold int
	DroneNames       string
}

type LowSpareRow struct {
	ID               int
	Category         string
	Name             string
	Quantity         int
	ReorderThreshold int
}

func (q *Queries) ListLowStock(ctx context.Context) ([]LowPropRow, []LowSpareRow, error) {
	prows, err := q.db.Query(ctx, `
SELECT p.id, COALESCE(bp.name,''), p.name, COALESCE(sz.label,''), p.blade_count,
  p.quantity, p.reorder_threshold,
  COALESCE((SELECT string_agg(d.name,', ' ORDER BY d.name)
            FROM drone_props dp2 JOIN drones d ON d.id=dp2.drone_id
            WHERE dp2.prop_id=p.id),'')
FROM propellers p
LEFT JOIN brands bp ON bp.id=p.brand_id
LEFT JOIN sizes sz ON sz.id=p.size_id
WHERE p.reorder_threshold > 0 AND p.quantity <= p.reorder_threshold
ORDER BY bp.name, p.name`)
	if err != nil {
		return nil, nil, err
	}
	var props []LowPropRow
	for prows.Next() {
		var p LowPropRow
		if err := prows.Scan(&p.ID, &p.Brand, &p.Name, &p.SizeLabel, &p.BladeCount,
			&p.Quantity, &p.ReorderThreshold, &p.DroneNames); err != nil {
			prows.Close()
			return nil, nil, err
		}
		props = append(props, p)
	}
	prows.Close()
	if err := prows.Err(); err != nil {
		return nil, nil, err
	}

	srows, err := q.db.Query(ctx, `
SELECT id, category, name, quantity, reorder_threshold
FROM spare_parts
WHERE reorder_threshold > 0 AND quantity <= reorder_threshold
ORDER BY category, name`)
	if err != nil {
		return nil, nil, err
	}
	var spares []LowSpareRow
	for srows.Next() {
		var s LowSpareRow
		if err := srows.Scan(&s.ID, &s.Category, &s.Name, &s.Quantity, &s.ReorderThreshold); err != nil {
			srows.Close()
			return nil, nil, err
		}
		spares = append(spares, s)
	}
	srows.Close()
	return props, spares, srows.Err()
}
