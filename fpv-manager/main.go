package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var appVersion = "dev"

type App struct {
	db         *pgxpool.Pool
	videoDir   string
	ffmpegPath string
	nvencOK    atomic.Bool
}

// systemTimezone reads the IANA timezone name from /etc/localtime symlink.
// On NixOS, time.Local.String() returns "Local", so we resolve the symlink instead.
func systemTimezone() string {
	target, err := os.Readlink("/etc/localtime")
	if err != nil {
		return time.Local.String()
	}
	const marker = "zoneinfo/"
	if i := strings.LastIndex(target, marker); i >= 0 {
		return target[i+len(marker):]
	}
	return ""
}

func main() {
	ctx := context.Background()
	dsn := os.Getenv("FPV_DB_DSN")
	if dsn == "" {
		dsn = "host=/run/postgresql dbname=fpv_manager user=fpv_manager sslmode=disable"
	}
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("parse dsn: %v", err)
	}
	if tz := systemTimezone(); tz != "" && tz != "UTC" {
		poolCfg.ConnConfig.RuntimeParams["timezone"] = tz
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
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

	ffmpegPath := os.Getenv("FPV_FFMPEG")
	app := &App{db: pool, videoDir: videoDir, ffmpegPath: ffmpegPath}
	if err := app.initSchema(ctx); err != nil {
		log.Fatalf("init schema: %v", err)
	}
	if ffmpegPath != "" {
		app.nvencOK.Store(detectNVENC(ffmpegPath))
		log.Printf("nvenc: %v", app.nvencOK.Load())
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
CREATE TABLE IF NOT EXISTS sizes (
    id    SERIAL PRIMARY KEY,
    label TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS frames (
    id         SERIAL PRIMARY KEY,
    brand      TEXT NOT NULL DEFAULT '',
    name       TEXT NOT NULL,
    size_mm    INTEGER,
    size_id    INTEGER REFERENCES sizes(id) ON DELETE SET NULL,
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

CREATE TABLE IF NOT EXISTS cells (
    id    SERIAL PRIMARY KEY,
    label TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS batteries (
    id           SERIAL PRIMARY KEY,
    brand        TEXT NOT NULL DEFAULT '',
    name         TEXT NOT NULL,
    cell_id      INTEGER REFERENCES cells(id) ON DELETE SET NULL,
    capacity_mah INTEGER NOT NULL,
    weight_g     INTEGER,
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
    size_id       INTEGER REFERENCES sizes(id) ON DELETE SET NULL,
    cell_id       INTEGER REFERENCES cells(id) ON DELETE SET NULL,
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
    size_id           INTEGER REFERENCES sizes(id) ON DELETE SET NULL,
    pitch             NUMERIC(4,1),
    blade_count       INTEGER NOT NULL DEFAULT 3,
    material          TEXT NOT NULL DEFAULT '',
    quantity          INTEGER NOT NULL DEFAULT 0,
    reorder_threshold INTEGER NOT NULL DEFAULT 0,
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

CREATE TABLE IF NOT EXISTS drone_batteries (
    drone_id   INTEGER NOT NULL REFERENCES drones(id) ON DELETE CASCADE,
    battery_id INTEGER NOT NULL REFERENCES batteries(id) ON DELETE CASCADE,
    PRIMARY KEY (drone_id, battery_id)
);

CREATE TABLE IF NOT EXISTS drone_props (
    drone_id INTEGER NOT NULL REFERENCES drones(id) ON DELETE CASCADE,
    prop_id  INTEGER NOT NULL REFERENCES propellers(id) ON DELETE CASCADE,
    PRIMARY KEY (drone_id, prop_id)
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

CREATE TABLE IF NOT EXISTS frame_photos (
    id            SERIAL PRIMARY KEY,
    frame_id      INTEGER NOT NULL REFERENCES frames(id) ON DELETE CASCADE,
    filename      TEXT NOT NULL,
    original_name TEXT NOT NULL DEFAULT '',
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS fc_photos (
    id            SERIAL PRIMARY KEY,
    fc_id         INTEGER NOT NULL REFERENCES flight_controllers(id) ON DELETE CASCADE,
    filename      TEXT NOT NULL,
    original_name TEXT NOT NULL DEFAULT '',
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS esc_photos (
    id            SERIAL PRIMARY KEY,
    esc_id        INTEGER NOT NULL REFERENCES escs(id) ON DELETE CASCADE,
    filename      TEXT NOT NULL,
    original_name TEXT NOT NULL DEFAULT '',
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS motor_photos (
    id            SERIAL PRIMARY KEY,
    motor_id      INTEGER NOT NULL REFERENCES motors(id) ON DELETE CASCADE,
    filename      TEXT NOT NULL,
    original_name TEXT NOT NULL DEFAULT '',
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS vtx_photos (
    id            SERIAL PRIMARY KEY,
    vtx_id        INTEGER NOT NULL REFERENCES vtx_units(id) ON DELETE CASCADE,
    filename      TEXT NOT NULL,
    original_name TEXT NOT NULL DEFAULT '',
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS gps_photos (
    id            SERIAL PRIMARY KEY,
    gps_id        INTEGER NOT NULL REFERENCES gps_modules(id) ON DELETE CASCADE,
    filename      TEXT NOT NULL,
    original_name TEXT NOT NULL DEFAULT '',
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS rx_photos (
    id            SERIAL PRIMARY KEY,
    rx_id         INTEGER NOT NULL REFERENCES radio_receivers(id) ON DELETE CASCADE,
    filename      TEXT NOT NULL,
    original_name TEXT NOT NULL DEFAULT '',
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS battery_photos (
    id            SERIAL PRIMARY KEY,
    battery_id    INTEGER NOT NULL REFERENCES batteries(id) ON DELETE CASCADE,
    filename      TEXT NOT NULL,
    original_name TEXT NOT NULL DEFAULT '',
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS prop_photos (
    id            SERIAL PRIMARY KEY,
    prop_id       INTEGER NOT NULL REFERENCES propellers(id) ON DELETE CASCADE,
    filename      TEXT NOT NULL,
    original_name TEXT NOT NULL DEFAULT '',
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS drone_log_entries (
    id         SERIAL PRIMARY KEY,
    drone_id   INTEGER NOT NULL REFERENCES drones(id) ON DELETE CASCADE,
    logged_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    body       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
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

CREATE TABLE IF NOT EXISTS brands (
    id   SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);
CREATE TABLE IF NOT EXISTS radio_protocols (
    id   SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);
INSERT INTO radio_protocols (name) VALUES ('ELRS 2.4GHz'),('ELRS 915MHz') ON CONFLICT DO NOTHING;
CREATE TABLE IF NOT EXISTS mcus (
    id   SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);
INSERT INTO mcus (name) VALUES ('F405'),('F722') ON CONFLICT DO NOTHING;

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
DO $$ BEGIN ALTER TABLE drones DROP COLUMN battery_count; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE batteries ADD COLUMN drone_id INTEGER REFERENCES drones(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN UPDATE batteries b SET drone_id=d.id FROM drones d WHERE d.battery_id=b.id AND b.drone_id IS NULL; EXCEPTION WHEN others THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE drones DROP COLUMN battery_id; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN INSERT INTO drone_batteries (drone_id, battery_id) SELECT drone_id, id FROM batteries WHERE drone_id IS NOT NULL ON CONFLICT DO NOTHING; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE batteries DROP COLUMN cycle_count; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE batteries DROP COLUMN internal_resistance; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE batteries DROP COLUMN purchase_date; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE batteries DROP COLUMN drone_id; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DROP INDEX IF EXISTS idx_batteries_drone;
DO $$ BEGIN ALTER TABLE batteries ADD COLUMN count INTEGER NOT NULL DEFAULT 1; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE batteries ADD COLUMN notes TEXT NOT NULL DEFAULT ''; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE drones ADD COLUMN gps_id INTEGER REFERENCES gps_modules(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE drones ADD COLUMN rx_id  INTEGER REFERENCES radio_receivers(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE drones ADD COLUMN weight_g INTEGER; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE drones ADD COLUMN size_inch NUMERIC(4,1); EXCEPTION WHEN duplicate_column THEN NULL; END $$;
INSERT INTO sizes (label) VALUES ('2'),('2.5'),('3'),('3.5'),('4'),('5'),('7') ON CONFLICT DO NOTHING;
INSERT INTO cells (label) VALUES ('1S'),('2S'),('3S'),('4S'),('5S'),('6S') ON CONFLICT DO NOTHING;
DO $$ BEGIN ALTER TABLE batteries ADD COLUMN cell_id INTEGER REFERENCES cells(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE batteries ADD COLUMN weight_g INTEGER; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN UPDATE batteries SET cell_id=c.id FROM cells c WHERE c.label=CAST(batteries.cell_count AS TEXT)||'S' AND batteries.cell_id IS NULL; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE batteries DROP COLUMN cell_count; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE drones ADD COLUMN cell_id INTEGER REFERENCES cells(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE drones ADD COLUMN size_id INTEGER REFERENCES sizes(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN UPDATE drones SET size_id=s.id FROM sizes s WHERE CAST(drones.size_inch AS TEXT)=s.label AND drones.size_id IS NULL; EXCEPTION WHEN others THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE drones DROP COLUMN size_inch; EXCEPTION WHEN undefined_column THEN NULL; END $$;
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

-- Brand FK migration: seed → add brand_id → backfill → drop brand text
DO $$ BEGIN
  INSERT INTO brands (name)
  SELECT DISTINCT brand FROM (
    SELECT brand FROM frames            UNION
    SELECT brand FROM flight_controllers UNION
    SELECT brand FROM escs              UNION
    SELECT brand FROM vtx_units         UNION
    SELECT brand FROM motors            UNION
    SELECT brand FROM gps_modules       UNION
    SELECT brand FROM radio_receivers   UNION
    SELECT brand FROM batteries         UNION
    SELECT brand FROM propellers
  ) AS t WHERE brand != ''
  ON CONFLICT DO NOTHING;
EXCEPTION WHEN OTHERS THEN NULL; END $$;

DO $$ BEGIN ALTER TABLE frames             ADD COLUMN brand_id INT REFERENCES brands(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE flight_controllers ADD COLUMN brand_id INT REFERENCES brands(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE escs               ADD COLUMN brand_id INT REFERENCES brands(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE vtx_units          ADD COLUMN brand_id INT REFERENCES brands(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE motors             ADD COLUMN brand_id INT REFERENCES brands(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE gps_modules        ADD COLUMN brand_id INT REFERENCES brands(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE radio_receivers    ADD COLUMN brand_id INT REFERENCES brands(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE batteries          ADD COLUMN brand_id INT REFERENCES brands(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE propellers         ADD COLUMN brand_id INT REFERENCES brands(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;

DO $$ BEGIN
  UPDATE frames             SET brand_id=(SELECT id FROM brands b WHERE b.name=brand) WHERE brand!='' AND brand_id IS NULL;
  UPDATE flight_controllers SET brand_id=(SELECT id FROM brands b WHERE b.name=brand) WHERE brand!='' AND brand_id IS NULL;
  UPDATE escs               SET brand_id=(SELECT id FROM brands b WHERE b.name=brand) WHERE brand!='' AND brand_id IS NULL;
  UPDATE vtx_units          SET brand_id=(SELECT id FROM brands b WHERE b.name=brand) WHERE brand!='' AND brand_id IS NULL;
  UPDATE motors             SET brand_id=(SELECT id FROM brands b WHERE b.name=brand) WHERE brand!='' AND brand_id IS NULL;
  UPDATE gps_modules        SET brand_id=(SELECT id FROM brands b WHERE b.name=brand) WHERE brand!='' AND brand_id IS NULL;
  UPDATE radio_receivers    SET brand_id=(SELECT id FROM brands b WHERE b.name=brand) WHERE brand!='' AND brand_id IS NULL;
  UPDATE batteries          SET brand_id=(SELECT id FROM brands b WHERE b.name=brand) WHERE brand!='' AND brand_id IS NULL;
  UPDATE propellers         SET brand_id=(SELECT id FROM brands b WHERE b.name=brand) WHERE brand!='' AND brand_id IS NULL;
EXCEPTION WHEN undefined_column THEN NULL; END $$;

DO $$ BEGIN ALTER TABLE frames             DROP COLUMN brand; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE flight_controllers DROP COLUMN brand; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE escs               DROP COLUMN brand; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE vtx_units          DROP COLUMN brand; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE motors             DROP COLUMN brand; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE gps_modules        DROP COLUMN brand; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE radio_receivers    DROP COLUMN brand; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE batteries          DROP COLUMN brand; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE propellers         DROP COLUMN brand; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN INSERT INTO drone_props (drone_id, prop_id) SELECT drone_id, id FROM propellers WHERE drone_id IS NOT NULL ON CONFLICT DO NOTHING; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE propellers DROP COLUMN drone_id; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE frames      ADD COLUMN size_id INTEGER REFERENCES sizes(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE propellers  ADD COLUMN size_id INTEGER REFERENCES sizes(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN UPDATE frames     SET size_id=s.id FROM sizes s WHERE frames.size_inch=s.label     AND frames.size_id IS NULL; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN UPDATE propellers SET size_id=s.id FROM sizes s WHERE CAST(propellers.size_inch AS TEXT)=s.label AND propellers.size_id IS NULL; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE frames      DROP COLUMN size_inch; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE propellers  DROP COLUMN size_inch; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE radio_receivers ADD COLUMN protocol_id INTEGER REFERENCES radio_protocols(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE radio_receivers DROP COLUMN protocol; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE propellers DROP COLUMN material; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE escs ADD COLUMN cell_max_id INTEGER REFERENCES cells(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN UPDATE escs SET cell_max_id=c.id FROM cells c WHERE c.label=CAST(escs.cell_max AS TEXT)||'S' AND escs.cell_max_id IS NULL; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE escs DROP COLUMN cell_max; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE flight_controllers ADD COLUMN mcu_id INTEGER REFERENCES mcus(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN UPDATE flight_controllers SET mcu_id=m.id FROM mcus m WHERE m.name=flight_controllers.mcu AND flight_controllers.mcu_id IS NULL; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE flight_controllers DROP COLUMN mcu; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE drones ADD COLUMN sub_250g BOOLEAN NOT NULL DEFAULT FALSE; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE session_videos ADD COLUMN drone_id INTEGER REFERENCES drones(id) ON DELETE SET NULL; EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE vtx_units DROP COLUMN system; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE drones ADD COLUMN status_changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(); EXCEPTION WHEN duplicate_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE vtx_units DROP COLUMN max_power_mw; EXCEPTION WHEN undefined_column THEN NULL; END $$;
DO $$ BEGIN ALTER TABLE vtx_units DROP COLUMN resolution; EXCEPTION WHEN undefined_column THEN NULL; END $$;

CREATE INDEX IF NOT EXISTS idx_drones_frame    ON drones(frame_id);
CREATE INDEX IF NOT EXISTS idx_drones_fc       ON drones(fc_id);
CREATE INDEX IF NOT EXISTS idx_drones_esc      ON drones(esc_id);
CREATE INDEX IF NOT EXISTS idx_drones_vtx      ON drones(vtx_id);
CREATE INDEX IF NOT EXISTS idx_drones_motor    ON drones(motor_id);
CREATE INDEX IF NOT EXISTS idx_db_drone        ON drone_batteries(drone_id);
CREATE INDEX IF NOT EXISTS idx_db_battery      ON drone_batteries(battery_id);
CREATE INDEX IF NOT EXISTS idx_drones_gps      ON drones(gps_id);
CREATE INDEX IF NOT EXISTS idx_drones_rx       ON drones(rx_id);
CREATE INDEX IF NOT EXISTS idx_dp_drone2       ON drone_props(drone_id);
CREATE INDEX IF NOT EXISTS idx_dp_prop         ON drone_props(prop_id);
CREATE INDEX IF NOT EXISTS idx_sd_drone        ON session_drones(drone_id);
CREATE INDEX IF NOT EXISTS idx_dp_drone        ON drone_photos(drone_id);
CREATE INDEX IF NOT EXISTS idx_dle_drone       ON drone_log_entries(drone_id);
CREATE INDEX IF NOT EXISTS idx_fp_frame        ON frame_photos(frame_id);
CREATE INDEX IF NOT EXISTS idx_fcp_fc          ON fc_photos(fc_id);
CREATE INDEX IF NOT EXISTS idx_escp_esc        ON esc_photos(esc_id);
CREATE INDEX IF NOT EXISTS idx_mp_motor        ON motor_photos(motor_id);
CREATE INDEX IF NOT EXISTS idx_vtxp_vtx        ON vtx_photos(vtx_id);
CREATE INDEX IF NOT EXISTS idx_gpsp_gps        ON gps_photos(gps_id);
CREATE INDEX IF NOT EXISTS idx_rxp_rx          ON rx_photos(rx_id);
CREATE INDEX IF NOT EXISTS idx_bp_battery      ON battery_photos(battery_id);
CREATE INDEX IF NOT EXISTS idx_pp_prop         ON prop_photos(prop_id);
CREATE INDEX IF NOT EXISTS idx_sv_session      ON session_videos(session_id);
CREATE INDEX IF NOT EXISTS idx_sp_session      ON session_photos(session_id);
CREATE INDEX IF NOT EXISTS idx_sessions_date   ON sessions(session_date DESC);
CREATE INDEX IF NOT EXISTS idx_sb_battery      ON session_batteries(battery_id);

CREATE TABLE IF NOT EXISTS app_config (
    id              INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    ffmpeg_crf      INTEGER NOT NULL DEFAULT 23,
    ffmpeg_preset   TEXT NOT NULL DEFAULT 'fast',
    video_max_width INTEGER NOT NULL DEFAULT 1280,
    video_bitrate_k INTEGER NOT NULL DEFAULT 3000,
    hls_segment_sec INTEGER NOT NULL DEFAULT 6
);
INSERT INTO app_config DEFAULT VALUES ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS checklist_items (
    id         SERIAL PRIMARY KEY,
    label      TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    enabled    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS session_checklist (
    id         SERIAL PRIMARY KEY,
    session_id INT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    label      TEXT NOT NULL,
    checked    BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sc_session_label ON session_checklist(session_id, label);
DELETE FROM checklist_items WHERE id BETWEEN 1 AND 8 AND label IN ('Props secure','Battery charged','Video link OK','GPS lock','Failsafe tested','Camera angle set','Controller charged','Air space clear');
ALTER TABLE drones DROP CONSTRAINT IF EXISTS drones_status_check;
ALTER TABLE drones ADD CONSTRAINT drones_status_check CHECK (status IN ('flying','build','retired','repairing'));
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
	mux.HandleFunc("/drones/{id}", a.handleDroneDetail)
	mux.HandleFunc("POST /drones/{id}/save", a.handleDroneSave)
	mux.HandleFunc("POST /drones/{id}/batteries", a.handleDroneBatteries)
	mux.HandleFunc("/drones/{id}/edit", a.handleDroneEdit)
	mux.HandleFunc("POST /drones/{id}/delete", a.handleDroneDelete)
	mux.HandleFunc("POST /drones/{id}/quick-flight", a.handleQuickFlight)
	mux.HandleFunc("POST /drones/{id}/log", a.handleDroneLogAdd)
	mux.HandleFunc("POST /drone-log/{id}/delete", a.handleDroneLogDelete)
	mux.HandleFunc("POST /drone-log/{id}/edit", a.handleDroneLogEdit)
	mux.HandleFunc("POST /drones/{id}/photos", a.handleDronePhotoUpload)
	mux.HandleFunc("GET /drone-photos/{id}", a.handleDronePhotoServe)
	mux.HandleFunc("POST /drone-photos/{id}/delete", a.handleDronePhotoDelete)
	mux.HandleFunc("POST /drone-photos/{id}/note", a.handleDronePhotoNote)

	mux.HandleFunc("/inventory", a.handleInventory)
	mux.HandleFunc("/frames/new", a.handleFrameNew)
	mux.HandleFunc("/frames/{id}/edit", a.handleFrameEdit)
	mux.HandleFunc("POST /frames/{id}/delete", a.handleFrameDelete)
	mux.HandleFunc("POST /frames/{id}/adjust", a.handleFrameAdjust)
	mux.HandleFunc("POST /frames/{id}/photos", a.handleFramePhotoUpload)
	mux.HandleFunc("GET /frame-photos/{id}", a.handleFramePhotoServe)
	mux.HandleFunc("POST /frame-photos/{id}/delete", a.handleFramePhotoDelete)
	mux.HandleFunc("POST /frame-photos/{id}/note", a.handleFramePhotoNote)
	mux.HandleFunc("/fcs/new", a.handleFCNew)
	mux.HandleFunc("/fcs/{id}/edit", a.handleFCEdit)
	mux.HandleFunc("POST /fcs/{id}/delete", a.handleFCDelete)
	mux.HandleFunc("POST /fcs/{id}/adjust", a.handleFCAdjust)
	mux.HandleFunc("POST /fcs/{id}/photos", a.handleFCPhotoUpload)
	mux.HandleFunc("GET /fc-photos/{id}", a.handleFCPhotoServe)
	mux.HandleFunc("POST /fc-photos/{id}/delete", a.handleFCPhotoDelete)
	mux.HandleFunc("POST /fc-photos/{id}/note", a.handleFCPhotoNote)
	mux.HandleFunc("/escs/new", a.handleESCNew)
	mux.HandleFunc("/escs/{id}/edit", a.handleESCEdit)
	mux.HandleFunc("POST /escs/{id}/delete", a.handleESCDelete)
	mux.HandleFunc("POST /escs/{id}/adjust", a.handleESCAdjust)
	mux.HandleFunc("POST /escs/{id}/photos", a.handleESCPhotoUpload)
	mux.HandleFunc("GET /esc-photos/{id}", a.handleESCPhotoServe)
	mux.HandleFunc("POST /esc-photos/{id}/delete", a.handleESCPhotoDelete)
	mux.HandleFunc("POST /esc-photos/{id}/note", a.handleESCPhotoNote)
	mux.HandleFunc("/motors/new", a.handleMotorNew)
	mux.HandleFunc("/motors/{id}/edit", a.handleMotorEdit)
	mux.HandleFunc("POST /motors/{id}/delete", a.handleMotorDelete)
	mux.HandleFunc("POST /motors/{id}/adjust", a.handleMotorAdjust)
	mux.HandleFunc("POST /motors/{id}/photos", a.handleMotorPhotoUpload)
	mux.HandleFunc("GET /motor-photos/{id}", a.handleMotorPhotoServe)
	mux.HandleFunc("POST /motor-photos/{id}/delete", a.handleMotorPhotoDelete)
	mux.HandleFunc("POST /motor-photos/{id}/note", a.handleMotorPhotoNote)
	mux.HandleFunc("/vtx/new", a.handleVTXNew)
	mux.HandleFunc("/vtx/{id}/edit", a.handleVTXEdit)
	mux.HandleFunc("POST /vtx/{id}/delete", a.handleVTXDelete)
	mux.HandleFunc("POST /vtx/{id}/adjust", a.handleVTXAdjust)
	mux.HandleFunc("POST /vtx/{id}/photos", a.handleVTXPhotoUpload)
	mux.HandleFunc("GET /vtx-photos/{id}", a.handleVTXPhotoServe)
	mux.HandleFunc("POST /vtx-photos/{id}/delete", a.handleVTXPhotoDelete)
	mux.HandleFunc("POST /vtx-photos/{id}/note", a.handleVTXPhotoNote)
	mux.HandleFunc("/gps/new", a.handleGPSNew)
	mux.HandleFunc("/gps/{id}/edit", a.handleGPSEdit)
	mux.HandleFunc("POST /gps/{id}/delete", a.handleGPSDelete)
	mux.HandleFunc("POST /gps/{id}/adjust", a.handleGPSAdjust)
	mux.HandleFunc("POST /gps/{id}/photos", a.handleGPSPhotoUpload)
	mux.HandleFunc("GET /gps-photos/{id}", a.handleGPSPhotoServe)
	mux.HandleFunc("POST /gps-photos/{id}/delete", a.handleGPSPhotoDelete)
	mux.HandleFunc("POST /gps-photos/{id}/note", a.handleGPSPhotoNote)
	mux.HandleFunc("/rx/new", a.handleRXNew)
	mux.HandleFunc("/rx/{id}/edit", a.handleRXEdit)
	mux.HandleFunc("POST /rx/{id}/delete", a.handleRXDelete)
	mux.HandleFunc("POST /rx/{id}/adjust", a.handleRXAdjust)
	mux.HandleFunc("POST /rx/{id}/photos", a.handleRXPhotoUpload)
	mux.HandleFunc("GET /rx-photos/{id}", a.handleRXPhotoServe)
	mux.HandleFunc("POST /rx-photos/{id}/delete", a.handleRXPhotoDelete)
	mux.HandleFunc("POST /rx-photos/{id}/note", a.handleRXPhotoNote)

	mux.HandleFunc("/props", a.handleProps)
	mux.HandleFunc("/props/new", a.handlePropNew)
	mux.HandleFunc("/props/{id}/edit", a.handlePropEdit)
	mux.HandleFunc("POST /props/{id}/delete", a.handlePropDelete)
	mux.HandleFunc("POST /props/{id}/photos", a.handlePropPhotoUpload)
	mux.HandleFunc("GET /prop-photos/{id}", a.handlePropPhotoServe)
	mux.HandleFunc("POST /prop-photos/{id}/delete", a.handlePropPhotoDelete)
	mux.HandleFunc("POST /prop-photos/{id}/note", a.handlePropPhotoNote)

	mux.HandleFunc("/batteries", a.handleBatteries)
	mux.HandleFunc("/batteries/new", a.handleBatteryNew)
	mux.HandleFunc("/batteries/{id}/edit", a.handleBatteryEdit)
	mux.HandleFunc("POST /batteries/{id}/delete", a.handleBatteryDelete)
	mux.HandleFunc("POST /batteries/{id}/adjust", a.handleBatteryAdjust)
	mux.HandleFunc("POST /batteries/{id}/photos", a.handleBatteryPhotoUpload)
	mux.HandleFunc("GET /battery-photos/{id}", a.handleBatteryPhotoServe)
	mux.HandleFunc("POST /battery-photos/{id}/delete", a.handleBatteryPhotoDelete)
	mux.HandleFunc("POST /battery-photos/{id}/note", a.handleBatteryPhotoNote)

	mux.HandleFunc("/log", a.handleLog)
	mux.HandleFunc("/log/new", a.handleSessionNew)
	mux.HandleFunc("/log/{id}", a.handleSessionDetail)
	mux.HandleFunc("/log/{id}/edit", a.handleSessionEdit)
	mux.HandleFunc("POST /log/{id}/delete", a.handleSessionDelete)
	mux.HandleFunc("POST /log/{id}/videos", a.handleVideoUpload)
	mux.HandleFunc("GET /videos/{id}", a.handleVideoServe)
	mux.HandleFunc("GET /videos/{id}/mobile.m3u8", a.handleMobilePlaylist)
	mux.HandleFunc("GET /videos/{id}/mobile/{name}", a.handleMobileSegment)
	mux.HandleFunc("POST /videos/{id}/delete", a.handleVideoDelete)
	mux.HandleFunc("POST /videos/{id}/note", a.handleVideoNote)
	mux.HandleFunc("POST /videos/{id}/drone", a.handleVideoDrone)
	mux.HandleFunc("POST /log/{id}/photos", a.handlePhotoUpload)
	mux.HandleFunc("GET /photos/{id}", a.handlePhotoServe)
	mux.HandleFunc("POST /photos/{id}/delete", a.handlePhotoDelete)
	mux.HandleFunc("POST /photos/{id}/note", a.handlePhotoNote)

	mux.HandleFunc("/places", a.handlePlaces)
	mux.HandleFunc("/places/new", a.handlePlaceNew)
	mux.HandleFunc("/places/{id}", a.handlePlaceDetail)
	mux.HandleFunc("/places/{id}/edit", a.handlePlaceEdit)
	mux.HandleFunc("POST /places/{id}/delete", a.handlePlaceDelete)

	mux.HandleFunc("/settings", a.handleSettings)
	mux.HandleFunc("POST /settings/config", a.handleConfigSave)
	mux.HandleFunc("/brands/new", a.handleBrandNew)
	mux.HandleFunc("/brands/{id}/edit", a.handleBrandEdit)
	mux.HandleFunc("POST /brands/{id}/delete", a.handleBrandDelete)
	mux.HandleFunc("/sizes/new", a.handleSizeNew)
	mux.HandleFunc("/sizes/{id}/edit", a.handleSizeEdit)
	mux.HandleFunc("POST /sizes/{id}/delete", a.handleSizeDelete)
	mux.HandleFunc("/cells/new", a.handleCellNew)
	mux.HandleFunc("/cells/{id}/edit", a.handleCellEdit)
	mux.HandleFunc("POST /cells/{id}/delete", a.handleCellDelete)
	mux.HandleFunc("/radio-protocols/new", a.handleRadioProtocolNew)
	mux.HandleFunc("/radio-protocols/{id}/edit", a.handleRadioProtocolEdit)
	mux.HandleFunc("POST /radio-protocols/{id}/delete", a.handleRadioProtocolDelete)
	mux.HandleFunc("/mcus/new", a.handleMCUNew)
	mux.HandleFunc("/mcus/{id}/edit", a.handleMCUEdit)
	mux.HandleFunc("POST /mcus/{id}/delete", a.handleMCUDelete)

	mux.HandleFunc("POST /log/{id}/checklist", a.handleSessionChecklistSave)

	mux.HandleFunc("/checklist-items/new", a.handleChecklistItemNew)
	mux.HandleFunc("/checklist-items/{id}/edit", a.handleChecklistItemEdit)
	mux.HandleFunc("POST /checklist-items/{id}/delete", a.handleChecklistItemDelete)

	mux.HandleFunc("/stats", a.handleStats)

	mux.HandleFunc("/weather", a.handleWeather)
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

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
