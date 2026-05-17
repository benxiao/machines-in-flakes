package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	db       *pgxpool.Pool
	videoDir string
}

func main() {
	ctx := context.Background()
	dsn := os.Getenv("FPV_DB_DSN")
	if dsn == "" {
		dsn = "host=/run/postgresql dbname=fpv_manager user=fpv_manager sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	videoDir := os.Getenv("FPV_VIDEO_DIR")
	if videoDir == "" {
		videoDir = "/var/lib/fpv-manager/videos"
	}
	if err := os.MkdirAll(videoDir, 0755); err != nil {
		log.Fatalf("create video dir: %v", err)
	}

	app := &App{db: pool, videoDir: videoDir}
	if err := app.initSchema(ctx); err != nil {
		log.Fatalf("init schema: %v", err)
	}
	initTemplates()

	addr := os.Getenv("FPV_LISTEN")
	if addr == "" {
		addr = ":10091"
	}
	mux := http.NewServeMux()
	app.registerRoutes(mux)
	log.Printf("fpv-manager listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

const schema = `
CREATE TABLE IF NOT EXISTS frames (
    id         SERIAL PRIMARY KEY,
    brand      TEXT NOT NULL DEFAULT '',
    name       TEXT NOT NULL,
    size_mm    INTEGER,
    weight_g   INTEGER,
    notes      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS flight_controllers (
    id         SERIAL PRIMARY KEY,
    brand      TEXT NOT NULL DEFAULT '',
    name       TEXT NOT NULL,
    mcu        TEXT NOT NULL DEFAULT '',
    firmware   TEXT NOT NULL DEFAULT '',
    notes      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS escs (
    id             SERIAL PRIMARY KEY,
    brand          TEXT NOT NULL DEFAULT '',
    name           TEXT NOT NULL,
    current_rating INTEGER,
    cell_max       INTEGER,
    notes          TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS vtx_units (
    id           SERIAL PRIMARY KEY,
    brand        TEXT NOT NULL DEFAULT '',
    name         TEXT NOT NULL,
    system       TEXT NOT NULL DEFAULT '',
    max_power_mw INTEGER,
    resolution   TEXT NOT NULL DEFAULT '',
    weight_g     INTEGER,
    notes        TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS motors (
    id          SERIAL PRIMARY KEY,
    brand       TEXT NOT NULL DEFAULT '',
    name        TEXT NOT NULL,
    stator_size TEXT NOT NULL DEFAULT '',
    kv          INTEGER,
    notes       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS gps_modules (
    id         SERIAL PRIMARY KEY,
    brand      TEXT NOT NULL DEFAULT '',
    name       TEXT NOT NULL,
    notes      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS radio_receivers (
    id         SERIAL PRIMARY KEY,
    brand      TEXT NOT NULL DEFAULT '',
    name       TEXT NOT NULL,
    protocol   TEXT NOT NULL DEFAULT '',
    notes      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS batteries (
    id           SERIAL PRIMARY KEY,
    brand        TEXT NOT NULL DEFAULT '',
    name         TEXT NOT NULL,
    cell_count   INTEGER NOT NULL,
    capacity_mah INTEGER NOT NULL,
    count        INTEGER NOT NULL DEFAULT 1,
    status       TEXT NOT NULL DEFAULT 'good'
                     CHECK (status IN ('good','degraded','dead','storage')),
    notes        TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS drones (
    id            SERIAL PRIMARY KEY,
    name          TEXT NOT NULL,
    frame_id      INTEGER REFERENCES frames(id) ON DELETE SET NULL,
    fc_id         INTEGER REFERENCES flight_controllers(id) ON DELETE SET NULL,
    esc_id        INTEGER REFERENCES escs(id) ON DELETE SET NULL,
    vtx_id        INTEGER REFERENCES vtx_units(id) ON DELETE SET NULL,
    motor_id      INTEGER REFERENCES motors(id) ON DELETE SET NULL,
    motor_count   INTEGER NOT NULL DEFAULT 4,
    battery_id    INTEGER REFERENCES batteries(id) ON DELETE SET NULL,
    battery_count INTEGER NOT NULL DEFAULT 1,
    gps_id        INTEGER REFERENCES gps_modules(id) ON DELETE SET NULL,
    rx_id         INTEGER REFERENCES radio_receivers(id) ON DELETE SET NULL,
    status        TEXT NOT NULL DEFAULT 'build'
                      CHECK (status IN ('flying','build','retired','repairing')),
    build_date    DATE,
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS item_counts (
    item_type TEXT NOT NULL CHECK (item_type IN ('frame','fc','esc','motor','vtx','gps','rx')),
    item_id   INTEGER NOT NULL,
    count     INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (item_type, item_id)
);

CREATE TABLE IF NOT EXISTS propellers (
    id                INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    brand             TEXT NOT NULL DEFAULT '',
    name              TEXT NOT NULL,
    size_inch         NUMERIC(4,1),
    pitch             NUMERIC(4,1),
    blade_count       INTEGER NOT NULL DEFAULT 3,
    material          TEXT NOT NULL DEFAULT '',
    quantity          INTEGER NOT NULL DEFAULT 0,
    reorder_threshold INTEGER NOT NULL DEFAULT 0,
    drone_id          INTEGER REFERENCES drones(id) ON DELETE SET NULL,
    notes             TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS spare_parts (
    id                INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    category          TEXT NOT NULL CHECK (category IN ('antennas','misc')),
    name              TEXT NOT NULL,
    quantity          INTEGER NOT NULL DEFAULT 0,
    reorder_threshold INTEGER NOT NULL DEFAULT 0,
    unit_price_cents  INTEGER NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sessions (
    id           SERIAL PRIMARY KEY,
    type         TEXT NOT NULL DEFAULT 'flight'
                     CHECK (type IN ('flight','maintenance','crash')),
    session_date DATE NOT NULL DEFAULT CURRENT_DATE,
    duration_min INTEGER NOT NULL DEFAULT 0,
    location     TEXT NOT NULL DEFAULT '',
    notes        TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS session_drones (
    session_id INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    drone_id   INTEGER NOT NULL REFERENCES drones(id) ON DELETE CASCADE,
    PRIMARY KEY (session_id, drone_id)
);

CREATE TABLE IF NOT EXISTS session_batteries (
    session_id INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    battery_id INTEGER NOT NULL REFERENCES batteries(id) ON DELETE CASCADE,
    count      INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (session_id, battery_id)
);

CREATE TABLE IF NOT EXISTS drone_photos (
    id            SERIAL PRIMARY KEY,
    drone_id      INTEGER NOT NULL REFERENCES drones(id) ON DELETE CASCADE,
    filename      TEXT NOT NULL,
    original_name TEXT NOT NULL DEFAULT '',
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS session_videos (
    id            SERIAL PRIMARY KEY,
    session_id    INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    filename      TEXT NOT NULL,
    original_name TEXT NOT NULL DEFAULT '',
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS session_photos (
    id            SERIAL PRIMARY KEY,
    session_id    INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    filename      TEXT NOT NULL,
    original_name TEXT NOT NULL DEFAULT '',
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS places (
    id         SERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    address    TEXT NOT NULL DEFAULT '',
    lat        NUMERIC(10,7),
    lng        NUMERIC(10,7),
    place_type TEXT NOT NULL DEFAULT 'field'
                   CHECK (place_type IN ('field','park','backyard','rooftop','other')),
    notes      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Migrations (DO blocks are idempotent; run after all tables exist):
DO $$ BEGIN ALTER TABLE motors      DROP COLUMN drone_id; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DROP INDEX IF EXISTS idx_motors_drone;
DO $$ BEGIN ALTER TABLE motors      DROP COLUMN status;   EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE frames      DROP COLUMN status;   EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE flight_controllers DROP COLUMN status; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE escs        DROP COLUMN status;   EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE vtx_units   DROP COLUMN status;   EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE drones ADD COLUMN motor_id INTEGER REFERENCES motors(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE drones ADD COLUMN motor_count INTEGER NOT NULL DEFAULT 4; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE drones ADD COLUMN battery_id INTEGER REFERENCES batteries(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE drones ADD COLUMN battery_count INTEGER NOT NULL DEFAULT 1; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE batteries DROP COLUMN cycle_count; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE batteries DROP COLUMN internal_resistance; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE batteries DROP COLUMN purchase_date; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE batteries DROP COLUMN drone_id; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DROP INDEX IF EXISTS idx_batteries_drone;
DO $$ BEGIN ALTER TABLE batteries ADD COLUMN count INTEGER NOT NULL DEFAULT 1; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE batteries ADD COLUMN notes TEXT NOT NULL DEFAULT ''; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE drones ADD COLUMN gps_id INTEGER REFERENCES gps_modules(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE drones ADD COLUMN rx_id  INTEGER REFERENCES radio_receivers(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN
  ALTER TABLE item_counts DROP CONSTRAINT IF EXISTS item_counts_item_type_check;
  ALTER TABLE item_counts ADD CONSTRAINT item_counts_item_type_check
    CHECK (item_type IN ('frame','fc','esc','motor','vtx','gps','rx'));
END $$;
DO $$ BEGIN
  INSERT INTO session_drones (session_id, drone_id)
    SELECT id, drone_id FROM sessions WHERE drone_id IS NOT NULL
    ON CONFLICT DO NOTHING;
EXCEPTION WHEN undefined_column THEN NULL;
END $$;
DO $$ BEGIN ALTER TABLE sessions DROP COLUMN drone_id; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE session_videos ADD COLUMN notes TEXT NOT NULL DEFAULT ''; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE session_batteries ADD COLUMN count INTEGER NOT NULL DEFAULT 1; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE frames ADD COLUMN size_inch TEXT NOT NULL DEFAULT ''; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE sessions ADD COLUMN title TEXT NOT NULL DEFAULT ''; EXCEPTION WHEN duplicate_column THEN NULL; END $$;

CREATE INDEX IF NOT EXISTS idx_drones_frame    ON drones(frame_id);
CREATE INDEX IF NOT EXISTS idx_drones_fc       ON drones(fc_id);
CREATE INDEX IF NOT EXISTS idx_drones_esc      ON drones(esc_id);
CREATE INDEX IF NOT EXISTS idx_drones_vtx      ON drones(vtx_id);
CREATE INDEX IF NOT EXISTS idx_drones_motor    ON drones(motor_id);
CREATE INDEX IF NOT EXISTS idx_drones_battery  ON drones(battery_id);
CREATE INDEX IF NOT EXISTS idx_drones_gps      ON drones(gps_id);
CREATE INDEX IF NOT EXISTS idx_drones_rx       ON drones(rx_id);
CREATE INDEX IF NOT EXISTS idx_props_drone     ON propellers(drone_id);
CREATE INDEX IF NOT EXISTS idx_sd_drone        ON session_drones(drone_id);
CREATE INDEX IF NOT EXISTS idx_dp_drone        ON drone_photos(drone_id);
CREATE INDEX IF NOT EXISTS idx_sv_session      ON session_videos(session_id);
CREATE INDEX IF NOT EXISTS idx_sp_session      ON session_photos(session_id);
CREATE INDEX IF NOT EXISTS idx_sessions_date   ON sessions(session_date DESC);
CREATE INDEX IF NOT EXISTS idx_sb_battery      ON session_batteries(battery_id);
`

func (a *App) initSchema(ctx context.Context) error {
	_, err := a.db.Exec(ctx, schema)
	return err
}

func (a *App) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/drones", http.StatusFound)
	})

	mux.HandleFunc("/drones", a.handleDrones)
	mux.HandleFunc("/drones/new", a.handleDroneNew)
	mux.HandleFunc("/drones/{id}/edit", a.handleDroneEdit)
	mux.HandleFunc("POST /drones/{id}/delete", a.handleDroneDelete)
	mux.HandleFunc("POST /drones/{id}/photos", a.handleDronePhotoUpload)
	mux.HandleFunc("GET /drone-photos/{id}", a.handleDronePhotoServe)
	mux.HandleFunc("POST /drone-photos/{id}/delete", a.handleDronePhotoDelete)
	mux.HandleFunc("POST /drone-photos/{id}/note", a.handleDronePhotoNote)

	mux.HandleFunc("/inventory", a.handleInventory)
	mux.HandleFunc("/frames/new", a.handleFrameNew)
	mux.HandleFunc("/frames/{id}/edit", a.handleFrameEdit)
	mux.HandleFunc("POST /frames/{id}/delete", a.handleFrameDelete)
	mux.HandleFunc("POST /frames/{id}/adjust", a.handleFrameAdjust)
	mux.HandleFunc("/fcs/new", a.handleFCNew)
	mux.HandleFunc("/fcs/{id}/edit", a.handleFCEdit)
	mux.HandleFunc("POST /fcs/{id}/delete", a.handleFCDelete)
	mux.HandleFunc("POST /fcs/{id}/adjust", a.handleFCAdjust)
	mux.HandleFunc("/escs/new", a.handleESCNew)
	mux.HandleFunc("/escs/{id}/edit", a.handleESCEdit)
	mux.HandleFunc("POST /escs/{id}/delete", a.handleESCDelete)
	mux.HandleFunc("POST /escs/{id}/adjust", a.handleESCAdjust)
	mux.HandleFunc("/motors/new", a.handleMotorNew)
	mux.HandleFunc("/motors/{id}/edit", a.handleMotorEdit)
	mux.HandleFunc("POST /motors/{id}/delete", a.handleMotorDelete)
	mux.HandleFunc("POST /motors/{id}/adjust", a.handleMotorAdjust)
	mux.HandleFunc("/vtx/new", a.handleVTXNew)
	mux.HandleFunc("/vtx/{id}/edit", a.handleVTXEdit)
	mux.HandleFunc("POST /vtx/{id}/delete", a.handleVTXDelete)
	mux.HandleFunc("POST /vtx/{id}/adjust", a.handleVTXAdjust)
	mux.HandleFunc("/gps/new", a.handleGPSNew)
	mux.HandleFunc("/gps/{id}/edit", a.handleGPSEdit)
	mux.HandleFunc("POST /gps/{id}/delete", a.handleGPSDelete)
	mux.HandleFunc("POST /gps/{id}/adjust", a.handleGPSAdjust)
	mux.HandleFunc("/rx/new", a.handleRXNew)
	mux.HandleFunc("/rx/{id}/edit", a.handleRXEdit)
	mux.HandleFunc("POST /rx/{id}/delete", a.handleRXDelete)
	mux.HandleFunc("POST /rx/{id}/adjust", a.handleRXAdjust)

	mux.HandleFunc("/props", a.handleProps)
	mux.HandleFunc("/props/new", a.handlePropNew)
	mux.HandleFunc("/props/{id}/edit", a.handlePropEdit)
	mux.HandleFunc("POST /props/{id}/delete", a.handlePropDelete)

	mux.HandleFunc("/batteries", a.handleBatteries)
	mux.HandleFunc("/batteries/new", a.handleBatteryNew)
	mux.HandleFunc("/batteries/{id}/edit", a.handleBatteryEdit)
	mux.HandleFunc("POST /batteries/{id}/delete", a.handleBatteryDelete)
	mux.HandleFunc("POST /batteries/{id}/adjust", a.handleBatteryAdjust)

	mux.HandleFunc("/log", a.handleLog)
	mux.HandleFunc("/log/new", a.handleSessionNew)
	mux.HandleFunc("/log/{id}", a.handleSessionDetail)
	mux.HandleFunc("/log/{id}/edit", a.handleSessionEdit)
	mux.HandleFunc("POST /log/{id}/delete", a.handleSessionDelete)
	mux.HandleFunc("POST /log/{id}/videos", a.handleVideoUpload)
	mux.HandleFunc("GET /videos/{id}", a.handleVideoServe)
	mux.HandleFunc("POST /videos/{id}/delete", a.handleVideoDelete)
	mux.HandleFunc("POST /videos/{id}/note", a.handleVideoNote)
	mux.HandleFunc("POST /log/{id}/photos", a.handlePhotoUpload)
	mux.HandleFunc("GET /photos/{id}", a.handlePhotoServe)
	mux.HandleFunc("POST /photos/{id}/delete", a.handlePhotoDelete)
	mux.HandleFunc("POST /photos/{id}/note", a.handlePhotoNote)

	mux.HandleFunc("/places", a.handlePlaces)
	mux.HandleFunc("/places/new", a.handlePlaceNew)
	mux.HandleFunc("/places/{id}", a.handlePlaceDetail)
	mux.HandleFunc("/places/{id}/edit", a.handlePlaceEdit)
	mux.HandleFunc("POST /places/{id}/delete", a.handlePlaceDelete)
}

func parseID(r *http.Request) (int, bool) {
	s := r.PathValue("id")
	id, err := strconv.Atoi(s)
	return id, err == nil && id > 0
}

func nullIntPtr(s string) *int {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &v
}

func nullFloat64(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

func nullDate(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	return &t
}

func httpErr(w http.ResponseWriter, err error) {
	log.Printf("error: %v", err)
	http.Error(w, "Internal server error", http.StatusInternalServerError)
}
