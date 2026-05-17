package main

import (
	"context"

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
	BattBrand  string
	BattName   string
	BattCells  int
	BattCount  int
	GPSBrand   string
	GPSName    string
	RXBrand    string
	RXName     string
	Notes      string
}

const droneJoin = `
FROM drones d
LEFT JOIN frames f ON f.id=d.frame_id
LEFT JOIN flight_controllers fc ON fc.id=d.fc_id
LEFT JOIN escs e ON e.id=d.esc_id
LEFT JOIN vtx_units v ON v.id=d.vtx_id
LEFT JOIN motors m ON m.id=d.motor_id
LEFT JOIN batteries b ON b.id=d.battery_id
LEFT JOIN gps_modules g ON g.id=d.gps_id
LEFT JOIN radio_receivers rx ON rx.id=d.rx_id`

const droneSelect = `SELECT d.id, d.name, d.status, TO_CHAR(d.build_date,'YYYY-MM-DD'),
  COALESCE(f.brand,''), COALESCE(f.name,''),
  COALESCE(fc.brand,''), COALESCE(fc.name,''),
  COALESCE(e.brand,''), COALESCE(e.name,''),
  COALESCE(v.brand,''), COALESCE(v.name,''),
  COALESCE(m.brand,''), COALESCE(m.name,''), d.motor_count,
  COALESCE(b.brand,''), COALESCE(b.name,''), COALESCE(b.cell_count,0), d.battery_count,
  COALESCE(g.brand,''), COALESCE(g.name,''),
  COALESCE(rx.brand,''), COALESCE(rx.name,''),
  d.notes` + droneJoin

func scanDrone(fn func(...any) error) (DroneRow, error) {
	var d DroneRow
	err := fn(
		&d.ID, &d.Name, &d.Status, &d.BuildDate,
		&d.FrameBrand, &d.FrameName,
		&d.FCBrand, &d.FCName,
		&d.ESCBrand, &d.ESCName,
		&d.VTXBrand, &d.VTXName,
		&d.MotorBrand, &d.MotorName, &d.MotorCount,
		&d.BattBrand, &d.BattName, &d.BattCells, &d.BattCount,
		&d.GPSBrand, &d.GPSName,
		&d.RXBrand, &d.RXName,
		&d.Notes,
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
	CellCount   int
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
      CASE WHEN b.brand!='' THEN b.brand||' '||b.name ELSE b.name END
      ||' '||b.cell_count||'S x'||sb.count, ', ' ORDER BY b.brand, b.name)
            FROM session_batteries sb JOIN batteries b ON b.id=sb.battery_id
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
SELECT b.id, b.brand, b.name, b.cell_count, b.capacity_mah, sb.count
FROM batteries b JOIN session_batteries sb ON sb.battery_id=b.id
WHERE sb.session_id=$1 ORDER BY b.brand, b.name`, id)
	if err != nil {
		return d, err
	}
	for brows.Next() {
		var sb SessionBattery
		if err := brows.Scan(&sb.ID, &sb.Brand, &sb.Name, &sb.CellCount, &sb.CapacityMAh, &sb.Count); err != nil {
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
	CellCount   int
	CapacityMAh int
	Count       int
	Status      string
	Notes       string
	AssignedTo  string
}

func (q *Queries) ListBatteries(ctx context.Context) ([]BatteryRow, error) {
	const stmt = `
SELECT b.id, b.brand, b.name, b.cell_count, b.capacity_mah, b.count, b.status, b.notes,
  COALESCE((SELECT string_agg(d.name,', ' ORDER BY d.name)
            FROM drones d WHERE d.battery_id=b.id),'')
FROM batteries b
ORDER BY b.brand, b.name, b.cell_count, b.capacity_mah`
	rows, err := q.db.Query(ctx, stmt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BatteryRow
	for rows.Next() {
		var b BatteryRow
		if err := rows.Scan(&b.ID, &b.Brand, &b.Name, &b.CellCount, &b.CapacityMAh,
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
SELECT f.id, f.brand, f.name,
  TRIM(f.size_inch || CASE WHEN f.weight_g IS NOT NULL THEN ' '||f.weight_g||'g' ELSE '' END),
  COALESCE(ic.count,0),
  COALESCE((SELECT COUNT(*)::int FROM drones d WHERE d.frame_id=f.id),0),
  COALESCE(ic.count,0) - COALESCE((SELECT COUNT(*)::int FROM drones d WHERE d.frame_id=f.id),0),
  COALESCE((SELECT string_agg(d.name,', ' ORDER BY d.name) FROM drones d WHERE d.frame_id=f.id),'')
FROM frames f
LEFT JOIN item_counts ic ON ic.item_type='frame' AND ic.item_id=f.id
ORDER BY f.brand, f.name`)
}

func (q *Queries) listFCs(ctx context.Context) ([]ComponentRow, error) {
	return q.scanComponents(ctx, "fc", `
SELECT fc.id, fc.brand, fc.name,
  TRIM(fc.mcu || CASE WHEN fc.firmware!='' THEN ' '||fc.firmware ELSE '' END),
  COALESCE(ic.count,0),
  COALESCE((SELECT COUNT(*)::int FROM drones d WHERE d.fc_id=fc.id),0),
  COALESCE(ic.count,0) - COALESCE((SELECT COUNT(*)::int FROM drones d WHERE d.fc_id=fc.id),0),
  COALESCE((SELECT string_agg(d.name,', ' ORDER BY d.name) FROM drones d WHERE d.fc_id=fc.id),'')
FROM flight_controllers fc
LEFT JOIN item_counts ic ON ic.item_type='fc' AND ic.item_id=fc.id
ORDER BY fc.brand, fc.name`)
}

func (q *Queries) listESCs(ctx context.Context) ([]ComponentRow, error) {
	return q.scanComponents(ctx, "esc", `
SELECT e.id, e.brand, e.name,
  TRIM(
    CASE WHEN e.current_rating IS NOT NULL THEN e.current_rating::text||'A' ELSE '' END ||
    CASE WHEN e.cell_max IS NOT NULL THEN ' '||e.cell_max::text||'S max' ELSE '' END
  ),
  COALESCE(ic.count,0),
  COALESCE((SELECT COUNT(*)::int FROM drones d WHERE d.esc_id=e.id),0),
  COALESCE(ic.count,0) - COALESCE((SELECT COUNT(*)::int FROM drones d WHERE d.esc_id=e.id),0),
  COALESCE((SELECT string_agg(d.name,', ' ORDER BY d.name) FROM drones d WHERE d.esc_id=e.id),'')
FROM escs e
LEFT JOIN item_counts ic ON ic.item_type='esc' AND ic.item_id=e.id
ORDER BY e.brand, e.name`)
}

func (q *Queries) listMotors(ctx context.Context) ([]ComponentRow, error) {
	return q.scanComponents(ctx, "motor", `
SELECT m.id, m.brand, m.name,
  TRIM(m.stator_size || CASE WHEN m.kv IS NOT NULL THEN ' '||m.kv::text||'KV' ELSE '' END),
  COALESCE(ic.count,0),
  COALESCE((SELECT SUM(d.motor_count)::int FROM drones d WHERE d.motor_id=m.id),0),
  COALESCE(ic.count,0) - COALESCE((SELECT SUM(d.motor_count)::int FROM drones d WHERE d.motor_id=m.id),0),
  COALESCE((SELECT string_agg(d.name,', ' ORDER BY d.name) FROM drones d WHERE d.motor_id=m.id),'')
FROM motors m
LEFT JOIN item_counts ic ON ic.item_type='motor' AND ic.item_id=m.id
ORDER BY m.brand, m.name`)
}

func (q *Queries) listVTXs(ctx context.Context) ([]ComponentRow, error) {
	return q.scanComponents(ctx, "vtx", `
SELECT v.id, v.brand, v.name,
  TRIM(
    v.system ||
    CASE WHEN v.max_power_mw IS NOT NULL THEN ' '||v.max_power_mw::text||'mW' ELSE '' END ||
    CASE WHEN v.resolution!='' THEN ' '||v.resolution ELSE '' END
  ),
  COALESCE(ic.count,0),
  COALESCE((SELECT COUNT(*)::int FROM drones d WHERE d.vtx_id=v.id),0),
  COALESCE(ic.count,0) - COALESCE((SELECT COUNT(*)::int FROM drones d WHERE d.vtx_id=v.id),0),
  COALESCE((SELECT string_agg(d.name,', ' ORDER BY d.name) FROM drones d WHERE d.vtx_id=v.id),'')
FROM vtx_units v
LEFT JOIN item_counts ic ON ic.item_type='vtx' AND ic.item_id=v.id
ORDER BY v.brand, v.name`)
}

func (q *Queries) listGPS(ctx context.Context) ([]ComponentRow, error) {
	return q.scanComponents(ctx, "gps", `
SELECT g.id, g.brand, g.name, '',
  COALESCE(ic.count,0),
  COALESCE((SELECT COUNT(*)::int FROM drones d WHERE d.gps_id=g.id),0),
  COALESCE(ic.count,0) - COALESCE((SELECT COUNT(*)::int FROM drones d WHERE d.gps_id=g.id),0),
  COALESCE((SELECT string_agg(d.name,', ' ORDER BY d.name) FROM drones d WHERE d.gps_id=g.id),'')
FROM gps_modules g
LEFT JOIN item_counts ic ON ic.item_type='gps' AND ic.item_id=g.id
ORDER BY g.brand, g.name`)
}

func (q *Queries) listRX(ctx context.Context) ([]ComponentRow, error) {
	return q.scanComponents(ctx, "rx", `
SELECT rx.id, rx.brand, rx.name, rx.protocol,
  COALESCE(ic.count,0),
  COALESCE((SELECT COUNT(*)::int FROM drones d WHERE d.rx_id=rx.id),0),
  COALESCE(ic.count,0) - COALESCE((SELECT COUNT(*)::int FROM drones d WHERE d.rx_id=rx.id),0),
  COALESCE((SELECT string_agg(d.name,', ' ORDER BY d.name) FROM drones d WHERE d.rx_id=rx.id),'')
FROM radio_receivers rx
LEFT JOIN item_counts ic ON ic.item_type='rx' AND ic.item_id=rx.id
ORDER BY rx.brand, rx.name`)
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

// ---- Low Stock ----

type LowPropRow struct {
	ID               int
	Brand            string
	Name             string
	SizeInch         *float64
	BladeCount       int
	Quantity         int
	ReorderThreshold int
	DroneName        string
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
SELECT p.id, p.brand, p.name, CAST(p.size_inch AS FLOAT8), p.blade_count,
  p.quantity, p.reorder_threshold, COALESCE(d.name,'')
FROM propellers p
LEFT JOIN drones d ON d.id=p.drone_id
WHERE p.reorder_threshold > 0 AND p.quantity <= p.reorder_threshold
ORDER BY p.brand, p.name`)
	if err != nil {
		return nil, nil, err
	}
	var props []LowPropRow
	for prows.Next() {
		var p LowPropRow
		if err := prows.Scan(&p.ID, &p.Brand, &p.Name, &p.SizeInch, &p.BladeCount,
			&p.Quantity, &p.ReorderThreshold, &p.DroneName); err != nil {
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
