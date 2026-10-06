package main

import (
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ── Constants ─────────────────────────────────────────────────────────────────

const defaultListen = ":10095"
const defaultDSN = "host=/run/postgresql dbname=stl_review user=stl_review sslmode=disable"

// set via -ldflags from flake.nix
var appVersion = "dev"

// Subdirectories of a project that hold viewable meshes. A project is any directory
// under STLR_HOME that has an stl/ subdirectory; scan/ and ref/ hold reference meshes
// (e.g. a 3D scan of the mating part) that are useful to compare against.
var meshSubdirs = []string{"stl", "scan", "ref"}

// foo_bar_v12.stl → part "foo_bar", version 12
var versionRe = regexp.MustCompile(`^(.+?)_v(\d+)$`)

const schema = `
CREATE TABLE IF NOT EXISTS annotations (
    id          SERIAL PRIMARY KEY,
    project     TEXT NOT NULL,
    part        TEXT NOT NULL,
    file        TEXT NOT NULL,
    version     INTEGER,
    kind        TEXT NOT NULL DEFAULT 'pin',
    x           DOUBLE PRECISION NOT NULL,
    y           DOUBLE PRECISION NOT NULL,
    z           DOUBLE PRECISION NOT NULL,
    nx          DOUBLE PRECISION NOT NULL DEFAULT 0,
    ny          DOUBLE PRECISION NOT NULL DEFAULT 0,
    nz          DOUBLE PRECISION NOT NULL DEFAULT 0,
    x2          DOUBLE PRECISION,
    y2          DOUBLE PRECISION,
    z2          DOUBLE PRECISION,
    body        TEXT NOT NULL DEFAULT '',
    camera      JSONB,
    has_shot    BOOLEAN NOT NULL DEFAULT FALSE,
    status      TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved', 'wontfix')),
    reply       TEXT NOT NULL DEFAULT '',
    resolved_in TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_annotations_part ON annotations(project, part);

-- Migrations
-- region: area notes. circle/rect: the drawn shape (in the saved camera's NDC); paint: the
-- brush dabs. Both carry the surface points sampled inside, in model coordinates.
ALTER TABLE annotations ADD COLUMN IF NOT EXISTS region JSONB;
ALTER TABLE annotations DROP CONSTRAINT IF EXISTS annotations_kind_check;
ALTER TABLE annotations ADD CONSTRAINT annotations_kind_check CHECK (kind IN ('pin', 'measure', 'circle', 'rect', 'paint'));
`

//go:embed static
var staticFS embed.FS

// ── Data types ────────────────────────────────────────────────────────────────

type MeshFile struct {
	Path    string    `json:"path"` // relative to the project dir, e.g. "stl/foo_v3.stl"
	Name    string    `json:"name"`
	Part    string    `json:"part"`
	Version *int      `json:"version"`
	Ref     bool      `json:"ref"` // lives in scan/ or ref/
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mtime"`
}

type Part struct {
	Name   string     `json:"name"`
	Ref    bool       `json:"ref"`
	Files  []MeshFile `json:"files"` // newest version last
	Latest time.Time  `json:"latest"`
}

type Project struct {
	Name   string    `json:"name"`
	Dir    string    `json:"dir"`
	Parts  []Part    `json:"parts"` // most recently touched first
	Latest time.Time `json:"latest"`
}

type Annotation struct {
	ID         int              `json:"id"`
	Project    string           `json:"project"`
	Part       string           `json:"part"`
	File       string           `json:"file"`
	Version    *int             `json:"version"`
	Kind       string           `json:"kind"`
	Point      [3]float64       `json:"point"`
	Normal     [3]float64       `json:"normal"`
	Point2     *[3]float64      `json:"point2,omitempty"`
	Body       string           `json:"body"`
	Camera     *json.RawMessage `json:"camera,omitempty"`
	Region     *json.RawMessage `json:"region,omitempty"`
	Screenshot string           `json:"screenshot,omitempty"` // URL path, empty if none
	ShotFile   string           `json:"shot_file,omitempty"`  // absolute path on disk
	Status     string           `json:"status"`
	Reply      string           `json:"reply"`
	ResolvedIn string           `json:"resolved_in"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
}

// ── App ───────────────────────────────────────────────────────────────────────

type App struct {
	db      *pgxpool.Pool
	home    string
	shotDir string
}

func (a *App) initSchema(ctx context.Context) error {
	_, err := a.db.Exec(ctx, schema)
	return err
}

// ── Mesh discovery ────────────────────────────────────────────────────────────

func (a *App) projectDirs() []string {
	var dirs []string
	entries, err := os.ReadDir(a.home)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			d := filepath.Join(a.home, e.Name())
			if st, err := os.Stat(filepath.Join(d, "stl")); err == nil && st.IsDir() {
				dirs = append(dirs, d)
			}
		}
	}
	return dirs
}

func scanProject(dir string) Project {
	p := Project{Name: filepath.Base(dir), Dir: dir}
	parts := map[string]*Part{}
	add := func(rel string, info fs.FileInfo, ref bool) {
		stem := strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))
		f := MeshFile{Path: rel, Name: info.Name(), Part: stem, Ref: ref, Size: info.Size(), ModTime: info.ModTime()}
		if m := versionRe.FindStringSubmatch(stem); m != nil {
			v, _ := strconv.Atoi(m[2])
			f.Part, f.Version = m[1], &v
		}
		pt := parts[f.Part]
		if pt == nil {
			pt = &Part{Name: f.Part, Ref: ref}
			parts[f.Part] = pt
		}
		pt.Files = append(pt.Files, f)
		if f.ModTime.After(pt.Latest) {
			pt.Latest = f.ModTime
		}
	}
	readDir := func(sub string, ref bool) {
		entries, err := os.ReadDir(filepath.Join(dir, sub))
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".stl") {
				continue
			}
			if info, err := e.Info(); err == nil {
				add(filepath.Join(sub, e.Name()), info, ref)
			}
		}
	}
	for _, sub := range meshSubdirs {
		readDir(sub, sub != "stl")
	}
	for _, pt := range parts {
		sort.Slice(pt.Files, func(i, j int) bool {
			vi, vj := pt.Files[i].Version, pt.Files[j].Version
			if vi != nil && vj != nil && *vi != *vj {
				return *vi < *vj
			}
			return pt.Files[i].Name < pt.Files[j].Name
		})
		p.Parts = append(p.Parts, *pt)
		if pt.Latest.After(p.Latest) {
			p.Latest = pt.Latest
		}
	}
	sort.Slice(p.Parts, func(i, j int) bool {
		if p.Parts[i].Ref != p.Parts[j].Ref {
			return !p.Parts[i].Ref
		}
		return p.Parts[i].Latest.After(p.Parts[j].Latest)
	})
	return p
}

func (a *App) scan() []Project {
	var out []Project
	for _, d := range a.projectDirs() {
		if p := scanProject(d); len(p.Parts) > 0 {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Latest.After(out[j].Latest) })
	return out
}

func (a *App) findProject(name string) (Project, bool) {
	for _, d := range a.projectDirs() {
		if filepath.Base(d) == name {
			return scanProject(d), true
		}
	}
	return Project{}, false
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Println("encode:", err)
	}
}

func httpError(w http.ResponseWriter, code int, err error) {
	if code >= 500 {
		log.Println(err)
	}
	http.Error(w, err.Error(), code)
}

func (a *App) shotPath(id int) string {
	return filepath.Join(a.shotDir, fmt.Sprintf("%d.png", id))
}

const annotationCols = `id, project, part, file, version, kind, x, y, z, nx, ny, nz, x2, y2, z2,
	body, camera, region, has_shot, status, reply, resolved_in, created_at, updated_at`

func (a *App) scanAnnotation(row pgx.Row) (Annotation, error) {
	var n Annotation
	var x2, y2, z2 *float64
	var cam, region []byte
	var hasShot bool
	err := row.Scan(&n.ID, &n.Project, &n.Part, &n.File, &n.Version, &n.Kind,
		&n.Point[0], &n.Point[1], &n.Point[2], &n.Normal[0], &n.Normal[1], &n.Normal[2],
		&x2, &y2, &z2, &n.Body, &cam, &region, &hasShot, &n.Status, &n.Reply, &n.ResolvedIn,
		&n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return n, err
	}
	if x2 != nil && y2 != nil && z2 != nil {
		n.Point2 = &[3]float64{*x2, *y2, *z2}
	}
	if cam != nil {
		raw := json.RawMessage(cam)
		n.Camera = &raw
	}
	if region != nil {
		raw := json.RawMessage(region)
		n.Region = &raw
	}
	if hasShot {
		n.Screenshot = fmt.Sprintf("/shots/%d.png", n.ID)
		n.ShotFile = a.shotPath(n.ID)
	}
	return n, nil
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	b, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		httpError(w, 500, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write([]byte(strings.ReplaceAll(string(b), "{{VERSION}}", appVersion)))
}

func (a *App) handleProjects(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, a.scan())
}

// handleState is polled by the page: a cheap fingerprint of every mesh file and of the
// annotations table, so the page can reload when a new version lands or a pin changes.
func (a *App) handleState(w http.ResponseWriter, r *http.Request) {
	h := fnv.New64a()
	for _, p := range a.scan() {
		for _, pt := range p.Parts {
			for _, f := range pt.Files {
				fmt.Fprintf(h, "%s/%s:%d:%d;", p.Name, f.Path, f.Size, f.ModTime.UnixNano())
			}
		}
	}
	var count int
	var latest *time.Time
	if err := a.db.QueryRow(r.Context(), `SELECT COUNT(*), MAX(updated_at) FROM annotations`).Scan(&count, &latest); err != nil {
		httpError(w, 500, err)
		return
	}
	ann := fmt.Sprint(count)
	if latest != nil {
		ann += ":" + strconv.FormatInt(latest.UnixNano(), 10)
	}
	writeJSON(w, map[string]string{"meshes": strconv.FormatUint(h.Sum64(), 16), "annotations": ann})
}

func (a *App) handleMesh(w http.ResponseWriter, r *http.Request) {
	p, ok := a.findProject(r.PathValue("project"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	rel := r.PathValue("path")
	// only files the scanner lists are servable — no path is ever joined from user input unchecked
	for _, pt := range p.Parts {
		for _, f := range pt.Files {
			if f.Path == rel {
				w.Header().Set("Cache-Control", "no-cache")
				w.Header().Set("Content-Type", "model/stl")
				http.ServeFile(w, r, filepath.Join(p.Dir, f.Path))
				return
			}
		}
	}
	http.NotFound(w, r)
}

func (a *App) handleListAnnotations(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	where := []string{"TRUE"}
	var args []any
	for _, k := range []string{"project", "part", "file", "status"} {
		if v := q.Get(k); v != "" {
			args = append(args, v)
			where = append(where, fmt.Sprintf("%s = $%d", k, len(args)))
		}
	}
	rows, err := a.db.Query(r.Context(),
		`SELECT `+annotationCols+` FROM annotations WHERE `+strings.Join(where, " AND ")+` ORDER BY id`, args...)
	if err != nil {
		httpError(w, 500, err)
		return
	}
	defer rows.Close()
	out := []Annotation{}
	for rows.Next() {
		n, err := a.scanAnnotation(rows)
		if err != nil {
			httpError(w, 500, err)
			return
		}
		out = append(out, n)
	}
	writeJSON(w, out)
}

type createReq struct {
	Project    string           `json:"project"`
	File       string           `json:"file"` // project-relative path
	Kind       string           `json:"kind"`
	Point      [3]float64       `json:"point"`
	Normal     [3]float64       `json:"normal"`
	Point2     *[3]float64      `json:"point2"`
	Body       string           `json:"body"`
	Camera     *json.RawMessage `json:"camera"`
	Region     *json.RawMessage `json:"region"`
	Screenshot string           `json:"screenshot"` // data:image/png;base64,...
}

func (a *App) handleCreateAnnotation(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 20<<20)
	var req createReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	p, ok := a.findProject(req.Project)
	if !ok {
		httpError(w, 400, errors.New("unknown project"))
		return
	}
	var file *MeshFile
	for _, pt := range p.Parts {
		for i := range pt.Files {
			if pt.Files[i].Path == req.File {
				file = &pt.Files[i]
			}
		}
	}
	if file == nil {
		httpError(w, 400, errors.New("unknown file"))
		return
	}
	if req.Kind == "" {
		req.Kind = "pin"
	}
	var png []byte
	if req.Screenshot != "" {
		const prefix = "data:image/png;base64,"
		if !strings.HasPrefix(req.Screenshot, prefix) {
			httpError(w, 400, errors.New("screenshot must be a PNG data URL"))
			return
		}
		var err error
		if png, err = base64.StdEncoding.DecodeString(req.Screenshot[len(prefix):]); err != nil {
			httpError(w, 400, err)
			return
		}
	}
	var x2, y2, z2 *float64
	if req.Point2 != nil {
		x2, y2, z2 = &req.Point2[0], &req.Point2[1], &req.Point2[2]
	}
	var cam, region []byte
	if req.Camera != nil {
		cam = *req.Camera
	}
	if req.Region != nil {
		region = *req.Region
	}
	var id int
	err := a.db.QueryRow(r.Context(), `
		INSERT INTO annotations (project, part, file, version, kind, x, y, z, nx, ny, nz, x2, y2, z2, body, camera, region, has_shot)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18) RETURNING id`,
		p.Name, file.Part, file.Path, file.Version, req.Kind,
		req.Point[0], req.Point[1], req.Point[2], req.Normal[0], req.Normal[1], req.Normal[2],
		x2, y2, z2, req.Body, cam, region, png != nil).Scan(&id)
	if err != nil {
		httpError(w, 500, err)
		return
	}
	if png != nil {
		if err := os.WriteFile(a.shotPath(id), png, 0644); err != nil {
			log.Println("screenshot:", err)
			a.db.Exec(r.Context(), `UPDATE annotations SET has_shot = FALSE WHERE id = $1`, id)
		}
	}
	n, err := a.scanAnnotation(a.db.QueryRow(r.Context(), `SELECT `+annotationCols+` FROM annotations WHERE id = $1`, id))
	if err != nil {
		httpError(w, 500, err)
		return
	}
	writeJSON(w, n)
}

// handleUpdateAnnotation takes any subset of body/status/reply/resolved_in. The page edits
// body and status; Claude sets reply, status and resolved_in after addressing a pin.
func (a *App) handleUpdateAnnotation(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	sets := []string{"updated_at = NOW()"}
	args := []any{id}
	for _, k := range []string{"body", "status", "reply", "resolved_in"} {
		if v, ok := req[k]; ok {
			args = append(args, v)
			sets = append(sets, fmt.Sprintf("%s = $%d", k, len(args)))
		}
	}
	n, err := a.scanAnnotation(a.db.QueryRow(r.Context(),
		`UPDATE annotations SET `+strings.Join(sets, ", ")+` WHERE id = $1 RETURNING `+annotationCols, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		httpError(w, 400, err)
		return
	}
	writeJSON(w, n)
}

func (a *App) handleDeleteAnnotation(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if _, err := a.db.Exec(r.Context(), `DELETE FROM annotations WHERE id = $1`, id); err != nil {
		httpError(w, 500, err)
		return
	}
	os.Remove(a.shotPath(id))
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleShot(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(strings.TrimSuffix(r.PathValue("name"), ".png"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, a.shotPath(id))
}

func (a *App) registerRoutes(mux *http.ServeMux) {
	static, _ := fs.Sub(staticFS, "static")
	mux.HandleFunc("GET /{$}", a.handleIndex)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	mux.HandleFunc("GET /api/projects", a.handleProjects)
	mux.HandleFunc("GET /api/state", a.handleState)
	mux.HandleFunc("GET /mesh/{project}/{path...}", a.handleMesh)
	mux.HandleFunc("GET /api/annotations", a.handleListAnnotations)
	mux.HandleFunc("POST /api/annotations", a.handleCreateAnnotation)
	mux.HandleFunc("PATCH /api/annotations/{id}", a.handleUpdateAnnotation)
	mux.HandleFunc("DELETE /api/annotations/{id}", a.handleDeleteAnnotation)
	mux.HandleFunc("GET /shots/{name}", a.handleShot)
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	listen := os.Getenv("STLR_LISTEN")
	if listen == "" {
		listen = defaultListen
	}
	dsn := os.Getenv("STLR_DB_DSN")
	if dsn == "" {
		dsn = defaultDSN
	}
	home := os.Getenv("STLR_HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	shotDir := os.Getenv("STLR_SHOT_DIR")
	if shotDir == "" {
		shotDir = "/var/lib/stl-review/shots"
	}
	if err := os.MkdirAll(shotDir, 0755); err != nil {
		log.Fatal("shot dir:", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal("db connect:", err)
	}
	app := &App{db: pool, home: home, shotDir: shotDir}
	if err := app.initSchema(ctx); err != nil {
		log.Fatal("schema:", err)
	}

	mux := http.NewServeMux()
	app.registerRoutes(mux)

	log.Printf("STL Review %s on http://0.0.0.0%s (projects under %s)", appVersion, listen, home)
	if err := http.ListenAndServe(listen, mux); err != nil {
		log.Fatal(err)
	}
}
