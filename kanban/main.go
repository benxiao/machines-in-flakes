package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ── Constants ─────────────────────────────────────────────────────────────────

const defaultListen = ":10092"
const defaultDSN = "host=/run/postgresql dbname=kanban user=kanban sslmode=disable"

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id         SERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    kind       TEXT NOT NULL DEFAULT 'human' CHECK (kind IN ('human', 'agent')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS boards (
    id         SERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS columns (
    id         SERIAL PRIMARY KEY,
    board_id   INTEGER NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    position   INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS cards (
    id          SERIAL PRIMARY KEY,
    column_id   INTEGER NOT NULL REFERENCES columns(id) ON DELETE CASCADE,
    title       TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    label       TEXT NOT NULL DEFAULT '',
    due_date    DATE,
    position    INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Migration: add created_by to existing cards tables
DO $$ BEGIN
  ALTER TABLE cards ADD COLUMN created_by INTEGER REFERENCES users(id) ON DELETE SET NULL;
EXCEPTION WHEN duplicate_column THEN NULL; END $$;

CREATE INDEX IF NOT EXISTS idx_columns_board    ON columns(board_id);
CREATE INDEX IF NOT EXISTS idx_columns_position ON columns(board_id, position);
CREATE INDEX IF NOT EXISTS idx_cards_column     ON cards(column_id);
CREATE INDEX IF NOT EXISTS idx_cards_position   ON cards(column_id, position);
CREATE INDEX IF NOT EXISTS idx_cards_created_by ON cards(created_by);
`

// ── Data types ────────────────────────────────────────────────────────────────

type User struct {
	ID   int
	Name string
	Kind string // "human" | "agent"
}

type Board struct {
	ID        int
	Name      string
	CardCount int
}

type Column struct {
	ID       int
	BoardID  int
	Name     string
	Position int
	Cards    []Card
}

type Card struct {
	ID            int
	ColumnID      int
	Title         string
	Description   string
	Label         string
	DueDate       *time.Time
	DueDateStr    string // "2006-01-02" for <input type="date">
	Position      int
	Overdue       bool
	CreatedByID   *int
	CreatedByName string
	CreatedByKind string
}

type LabelOption struct{ Name, Color string }

var labelOptions = []LabelOption{
	{"", ""},
	{"red", "#f85149"},
	{"green", "#3fb950"},
	{"blue", "#58a6ff"},
	{"yellow", "#d29922"},
	{"purple", "#bc8cff"},
}

var labelColorMap = map[string]string{
	"red": "#f85149", "green": "#3fb950", "blue": "#58a6ff",
	"yellow": "#d29922", "purple": "#bc8cff",
}

// ── Page data structs ─────────────────────────────────────────────────────────

type BoardListPage struct{ Boards []Board }

type BoardViewPage struct {
	Board   Board
	Columns []Column
}

type CardNewPage struct {
	Board   Board
	Columns []Column
	Users   []User
}

type CardEditPage struct {
	Card         Card
	BoardID      int
	BoardName    string
	ColumnName   string
	LabelOptions []LabelOption
	Users        []User
}

type UserListPage struct{ Users []User }

// ── App ───────────────────────────────────────────────────────────────────────

type App struct{ db *pgxpool.Pool }

func newApp(ctx context.Context, dsn string) (*App, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &App{db: pool}, nil
}

func (a *App) initSchema(ctx context.Context) error {
	_, err := a.db.Exec(ctx, schema)
	return err
}

func (a *App) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", a.handleBoards)
	mux.HandleFunc("POST /boards", a.handleBoardCreate)
	mux.HandleFunc("/boards/{id}", a.handleBoardView)
	mux.HandleFunc("POST /boards/{id}/delete", a.handleBoardDelete)
	mux.HandleFunc("POST /boards/{id}/rename", a.handleBoardRename)
	mux.HandleFunc("POST /boards/{id}/columns", a.handleColumnCreate)
	mux.HandleFunc("POST /columns/{id}/delete", a.handleColumnDelete)
	mux.HandleFunc("POST /columns/{id}/rename", a.handleColumnRename)
	mux.HandleFunc("POST /columns/{id}/cards", a.handleCardCreate)
	mux.HandleFunc("/boards/{id}/cards/new", a.handleCardNew)
	mux.HandleFunc("/cards/{id}", a.handleCardEdit)
	mux.HandleFunc("POST /cards/{id}/delete", a.handleCardDelete)
	mux.HandleFunc("POST /api/move", a.handleMove)
	mux.HandleFunc("/users", a.handleUsers)
	mux.HandleFunc("POST /users", a.handleUserCreate)
	mux.HandleFunc("POST /users/{id}/delete", a.handleUserDelete)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func parseID(r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	return id, err == nil && id > 0
}

func httpErr(w http.ResponseWriter, err error) {
	log.Println(err)
	http.Error(w, "internal error", 500)
}

func seeOther(w http.ResponseWriter, r *http.Request, url string) {
	http.Redirect(w, r, url, http.StatusSeeOther)
}

func enrichCard(c *Card) {
	if c.DueDate != nil {
		c.DueDateStr = c.DueDate.Format("2006-01-02")
		today := time.Now().Truncate(24 * time.Hour)
		c.Overdue = c.DueDate.Before(today)
	}
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (a *App) handleBoards(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	rows, err := a.db.Query(r.Context(), `
		SELECT b.id, b.name, COUNT(c.id)
		FROM boards b
		LEFT JOIN columns col ON col.board_id = b.id
		LEFT JOIN cards c ON c.column_id = col.id
		GROUP BY b.id, b.name
		ORDER BY b.created_at`)
	if err != nil {
		httpErr(w, err)
		return
	}
	defer rows.Close()
	var boards []Board
	for rows.Next() {
		var b Board
		if err := rows.Scan(&b.ID, &b.Name, &b.CardCount); err != nil {
			httpErr(w, err)
			return
		}
		boards = append(boards, b)
	}
	render(w, "board-list", BoardListPage{Boards: boards})
}

func (a *App) handleBoardCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		httpErr(w, err)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		seeOther(w, r, "/")
		return
	}
	var id int
	err := a.db.QueryRow(r.Context(), `INSERT INTO boards (name) VALUES ($1) RETURNING id`, name).Scan(&id)
	if err != nil {
		httpErr(w, err)
		return
	}
	seeOther(w, r, fmt.Sprintf("/boards/%d", id))
}

func (a *App) handleBoardDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(r.Context(), `DELETE FROM boards WHERE id=$1`, id)
	seeOther(w, r, "/")
}

func (a *App) handleBoardRename(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		httpErr(w, err)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name != "" {
		a.db.Exec(r.Context(), `UPDATE boards SET name=$1 WHERE id=$2`, name, id)
	}
	w.WriteHeader(200)
}

func (a *App) handleBoardView(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	var board Board
	err := a.db.QueryRow(ctx, `SELECT id, name FROM boards WHERE id=$1`, id).Scan(&board.ID, &board.Name)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}

	colRows, err := a.db.Query(ctx,
		`SELECT id, name, position FROM columns WHERE board_id=$1 ORDER BY position`, id)
	if err != nil {
		httpErr(w, err)
		return
	}
	defer colRows.Close()
	var columns []Column
	for colRows.Next() {
		var col Column
		col.BoardID = id
		colRows.Scan(&col.ID, &col.Name, &col.Position)
		columns = append(columns, col)
	}
	colRows.Close()

	if len(columns) > 0 {
		colIDs := make([]int, len(columns))
		for i, c := range columns {
			colIDs[i] = c.ID
		}
		cardRows, err := a.db.Query(ctx,
			`SELECT c.id, c.column_id, c.title, c.description, c.label, c.due_date,
			        c.position, c.created_by, COALESCE(u.name,''), COALESCE(u.kind,'')
			 FROM cards c LEFT JOIN users u ON u.id = c.created_by
			 WHERE c.column_id = ANY($1) ORDER BY c.position`, colIDs)
		if err != nil {
			httpErr(w, err)
			return
		}
		defer cardRows.Close()
		cardsByCol := map[int][]Card{}
		for cardRows.Next() {
			var c Card
			cardRows.Scan(&c.ID, &c.ColumnID, &c.Title, &c.Description, &c.Label, &c.DueDate,
				&c.Position, &c.CreatedByID, &c.CreatedByName, &c.CreatedByKind)
			enrichCard(&c)
			cardsByCol[c.ColumnID] = append(cardsByCol[c.ColumnID], c)
		}
		cardRows.Close()
		for i := range columns {
			columns[i].Cards = cardsByCol[columns[i].ID]
		}
	}

	render(w, "board-view", BoardViewPage{Board: board, Columns: columns})
}

func (a *App) handleColumnCreate(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		httpErr(w, err)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = "New Column"
	}
	ctx := r.Context()
	var maxPos int
	a.db.QueryRow(ctx, `SELECT COALESCE(MAX(position),0) FROM columns WHERE board_id=$1`, id).Scan(&maxPos)
	a.db.Exec(ctx, `INSERT INTO columns (board_id, name, position) VALUES ($1,$2,$3)`, id, name, maxPos+1000)
	seeOther(w, r, fmt.Sprintf("/boards/%d", id))
}

func (a *App) handleColumnDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	var boardID int
	a.db.QueryRow(ctx, `SELECT board_id FROM columns WHERE id=$1`, id).Scan(&boardID)
	a.db.Exec(ctx, `DELETE FROM columns WHERE id=$1`, id)
	seeOther(w, r, fmt.Sprintf("/boards/%d", boardID))
}

func (a *App) handleColumnRename(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		httpErr(w, err)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name != "" {
		a.db.Exec(r.Context(), `UPDATE columns SET name=$1 WHERE id=$2`, name, id)
	}
	w.WriteHeader(200)
}

func (a *App) handleCardCreate(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r) // column id
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		httpErr(w, err)
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		// find board to redirect back
		var boardID int
		a.db.QueryRow(r.Context(), `SELECT board_id FROM columns WHERE id=$1`, id).Scan(&boardID)
		seeOther(w, r, fmt.Sprintf("/boards/%d", boardID))
		return
	}
	ctx := r.Context()
	var maxPos int
	a.db.QueryRow(ctx, `SELECT COALESCE(MAX(position),0) FROM cards WHERE column_id=$1`, id).Scan(&maxPos)
	a.db.Exec(ctx, `INSERT INTO cards (column_id, title, position) VALUES ($1,$2,$3)`, id, title, maxPos+1000)
	var boardID int
	a.db.QueryRow(ctx, `SELECT board_id FROM columns WHERE id=$1`, id).Scan(&boardID)
	seeOther(w, r, fmt.Sprintf("/boards/%d", boardID))
}

func (a *App) handleCardEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		title := strings.TrimSpace(r.FormValue("title"))
		desc := r.FormValue("description")
		label := r.FormValue("label")
		dueDateStr := r.FormValue("due_date")
		var dueDate *time.Time
		if dueDateStr != "" {
			t, err := time.Parse("2006-01-02", dueDateStr)
			if err == nil {
				dueDate = &t
			}
		}
		var createdBy *int
		if v := r.FormValue("created_by"); v != "" {
			if uid, err := strconv.Atoi(v); err == nil {
				createdBy = &uid
			}
		}
		a.db.Exec(ctx,
			`UPDATE cards SET title=$1, description=$2, label=$3, due_date=$4, created_by=$5 WHERE id=$6`,
			title, desc, label, dueDate, createdBy, id)
		var colID int
		a.db.QueryRow(ctx, `SELECT column_id FROM cards WHERE id=$1`, id).Scan(&colID)
		var boardID int
		a.db.QueryRow(ctx, `SELECT board_id FROM columns WHERE id=$1`, colID).Scan(&boardID)
		seeOther(w, r, fmt.Sprintf("/boards/%d", boardID))
		return
	}

	var c Card
	err := a.db.QueryRow(ctx,
		`SELECT c.id, c.column_id, c.title, c.description, c.label, c.due_date,
		        c.position, c.created_by, COALESCE(u.name,''), COALESCE(u.kind,'')
		 FROM cards c LEFT JOIN users u ON u.id = c.created_by WHERE c.id=$1`, id).
		Scan(&c.ID, &c.ColumnID, &c.Title, &c.Description, &c.Label, &c.DueDate,
			&c.Position, &c.CreatedByID, &c.CreatedByName, &c.CreatedByKind)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}
	enrichCard(&c)

	var boardID int
	var boardName, colName string
	a.db.QueryRow(ctx,
		`SELECT b.id, b.name, col.name FROM columns col JOIN boards b ON b.id=col.board_id WHERE col.id=$1`,
		c.ColumnID).Scan(&boardID, &boardName, &colName)

	users, _ := a.listUsers(ctx)
	render(w, "card-edit", CardEditPage{
		Card: c, BoardID: boardID, BoardName: boardName,
		ColumnName: colName, LabelOptions: labelOptions, Users: users,
	})
}

func (a *App) handleCardDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	var colID int
	a.db.QueryRow(ctx, `SELECT column_id FROM cards WHERE id=$1`, id).Scan(&colID)
	var boardID int
	a.db.QueryRow(ctx, `SELECT board_id FROM columns WHERE id=$1`, colID).Scan(&boardID)
	a.db.Exec(ctx, `DELETE FROM cards WHERE id=$1`, id)
	seeOther(w, r, fmt.Sprintf("/boards/%d", boardID))
}

func (a *App) handleMove(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CardID   int `json:"card_id"`
		ColumnID int `json:"column_id"`
		BeforeID int `json:"before_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	ctx := r.Context()

	rows, err := a.db.Query(ctx,
		`SELECT id FROM cards WHERE column_id=$1 AND id!=$2 ORDER BY position`,
		req.ColumnID, req.CardID)
	if err != nil {
		httpErr(w, err)
		return
	}
	var ids []int
	for rows.Next() {
		var id int
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()

	insertAt := len(ids)
	if req.BeforeID != 0 {
		for i, id := range ids {
			if id == req.BeforeID {
				insertAt = i
				break
			}
		}
	}

	ordered := make([]int, 0, len(ids)+1)
	ordered = append(ordered, ids[:insertAt]...)
	ordered = append(ordered, req.CardID)
	ordered = append(ordered, ids[insertAt:]...)

	tx, err := a.db.Begin(ctx)
	if err != nil {
		httpErr(w, err)
		return
	}
	defer tx.Rollback(ctx)
	for i, id := range ordered {
		if _, err := tx.Exec(ctx,
			`UPDATE cards SET column_id=$1, position=$2 WHERE id=$3`,
			req.ColumnID, (i+1)*1000, id); err != nil {
			httpErr(w, err)
			return
		}
	}
	if err := tx.Commit(ctx); err != nil {
		httpErr(w, err)
		return
	}
	w.WriteHeader(200)
}

// ── Card creation ─────────────────────────────────────────────────────────────

func (a *App) handleCardNew(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r) // board id
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()

	var board Board
	err := a.db.QueryRow(ctx, `SELECT id, name FROM boards WHERE id=$1`, id).Scan(&board.ID, &board.Name)
	if err == pgx.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpErr(w, err)
		return
	}

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			httpErr(w, err)
			return
		}
		title := strings.TrimSpace(r.FormValue("title"))
		colID, _ := strconv.Atoi(r.FormValue("column_id"))
		if title == "" || colID == 0 {
			seeOther(w, r, fmt.Sprintf("/boards/%d/cards/new", id))
			return
		}
		var createdBy *int
		if v := r.FormValue("created_by"); v != "" {
			if uid, err := strconv.Atoi(v); err == nil {
				createdBy = &uid
			}
		}
		var maxPos int
		a.db.QueryRow(ctx, `SELECT COALESCE(MAX(position),0) FROM cards WHERE column_id=$1`, colID).Scan(&maxPos)
		a.db.Exec(ctx,
			`INSERT INTO cards (column_id, title, position, created_by) VALUES ($1,$2,$3,$4)`,
			colID, title, maxPos+1000, createdBy)
		seeOther(w, r, fmt.Sprintf("/boards/%d", id))
		return
	}

	colRows, err := a.db.Query(ctx,
		`SELECT id, name FROM columns WHERE board_id=$1 ORDER BY position`, id)
	if err != nil {
		httpErr(w, err)
		return
	}
	defer colRows.Close()
	var columns []Column
	for colRows.Next() {
		var col Column
		colRows.Scan(&col.ID, &col.Name)
		columns = append(columns, col)
	}
	colRows.Close()

	users, _ := a.listUsers(ctx)
	render(w, "card-new", CardNewPage{Board: board, Columns: columns, Users: users})
}

// ── User handlers ─────────────────────────────────────────────────────────────

func (a *App) listUsers(ctx context.Context) ([]User, error) {
	rows, err := a.db.Query(ctx, `SELECT id, name, kind FROM users ORDER BY kind, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		rows.Scan(&u.ID, &u.Name, &u.Kind)
		users = append(users, u)
	}
	return users, nil
}

func (a *App) handleUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.listUsers(r.Context())
	if err != nil {
		httpErr(w, err)
		return
	}
	render(w, "user-list", UserListPage{Users: users})
}

func (a *App) handleUserCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		httpErr(w, err)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	kind := r.FormValue("kind")
	if name == "" {
		seeOther(w, r, "/users")
		return
	}
	if kind != "human" && kind != "agent" {
		kind = "human"
	}
	a.db.Exec(r.Context(), `INSERT INTO users (name, kind) VALUES ($1, $2)`, name, kind)
	seeOther(w, r, "/users")
}

func (a *App) handleUserDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.db.Exec(r.Context(), `DELETE FROM users WHERE id=$1`, id)
	seeOther(w, r, "/users")
}

// ── Templates ─────────────────────────────────────────────────────────────────

const css = `
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;font-size:14px;background:#0d1117;color:#c9d1d9;line-height:1.5;min-height:100vh}
a{color:#58a6ff;text-decoration:none}
a:hover{text-decoration:underline}
header{background:#161b22;border-bottom:1px solid #30363d;padding:12px 24px;display:flex;align-items:center;gap:16px}
.logo{font-size:18px;font-weight:700;color:#f0f6fc}
.logo a{color:#f0f6fc;text-decoration:none}
.header-back{font-size:13px;color:#8b949e}
.header-back:hover{color:#c9d1d9;text-decoration:none}
.header-title{font-size:16px;font-weight:600;color:#f0f6fc;flex:1}
main{padding:24px}
h2{font-size:20px;font-weight:600;margin-bottom:20px;color:#f0f6fc}
.btn{display:inline-block;padding:6px 14px;border-radius:6px;font-size:13px;font-weight:500;border:1px solid;cursor:pointer;text-decoration:none;line-height:1.4;font-family:inherit}
.btn-primary{background:#238636;border-color:#2ea043;color:#fff}
.btn-primary:hover{background:#2ea043;color:#fff;text-decoration:none}
.btn-danger{background:transparent;border-color:rgba(248,81,73,.4);color:#f85149}
.btn-danger:hover{background:rgba(248,81,73,.1);text-decoration:none}
.btn-cancel{background:transparent;border-color:#30363d;color:#8b949e}
.btn-cancel:hover{background:#21262d;color:#c9d1d9;text-decoration:none}
.btn-sm{padding:3px 10px;font-size:12px}
.form-group{margin-bottom:16px}
label{display:block;font-size:13px;color:#8b949e;margin-bottom:4px}
input[type=text],input[type=date],textarea,select{width:100%;padding:7px 10px;background:#0d1117;border:1px solid #30363d;border-radius:6px;color:#c9d1d9;font-size:14px;font-family:inherit}
input:focus,textarea:focus,select:focus{outline:none;border-color:#58a6ff}
textarea{min-height:100px;resize:vertical}
.form-actions{display:flex;gap:8px;margin-top:24px;align-items:center}
.page-header{display:flex;justify-content:space-between;align-items:center;margin-bottom:20px;flex-wrap:wrap;gap:10px}

/* Board list */
.board-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(220px,1fr));gap:14px;margin-bottom:32px}
.board-card{background:#161b22;border:1px solid #30363d;border-radius:8px;padding:16px;cursor:pointer;text-decoration:none;display:block;transition:border-color .15s}
.board-card:hover{border-color:#58a6ff;text-decoration:none}
.board-card-name{font-size:16px;font-weight:600;color:#f0f6fc;margin-bottom:6px}
.board-card-count{font-size:12px;color:#6e7681}
.new-board-form{display:flex;gap:8px;align-items:center;max-width:400px}
.new-board-form input{flex:1}

/* Kanban board */
.board-wrap{overflow-x:auto;padding-bottom:16px}
.board-cols{display:flex;gap:12px;align-items:flex-start;min-height:calc(100vh - 140px)}
.column{background:#161b22;border:1px solid #30363d;border-radius:8px;padding:10px;flex:1;min-width:160px;display:flex;flex-direction:column}
.col-header{display:flex;align-items:center;gap:6px;margin-bottom:8px;min-height:32px}
.col-name{font-weight:600;color:#f0f6fc;flex:1;padding:4px 6px;border-radius:4px;outline:none;cursor:text;font-size:14px}
.col-name:hover{background:#21262d}
.col-name:focus{background:#21262d;box-shadow:0 0 0 2px #58a6ff}
.col-delete{color:#6e7681;border:none;background:none;cursor:pointer;font-size:16px;padding:2px 6px;border-radius:4px;line-height:1}
.col-delete:hover{color:#f85149;background:rgba(248,81,73,.1)}
.card-list{flex:1;min-height:40px;border-radius:6px;transition:background .15s}
.card-list.col-drag-over{background:rgba(88,166,255,.06);border:1px dashed #58a6ff}
.card{background:#0d1117;border:1px solid #30363d;border-radius:6px;padding:10px 12px;margin-bottom:6px;cursor:grab;transition:border-color .15s}
.card:hover{border-color:#484f58}
.card:active{cursor:grabbing}
.card.dragging{opacity:.4}
.card.drop-before{border-top:2px solid #58a6ff;margin-top:-1px}
.card-top{display:flex;align-items:flex-start;gap:6px;margin-bottom:4px}
.card-label{width:10px;height:10px;border-radius:50%;flex-shrink:0;margin-top:3px}
.card-title{font-size:13px;color:#c9d1d9;flex:1;text-decoration:none}
.card-title:hover{color:#f0f6fc;text-decoration:none}
.card-due{font-size:11px;color:#6e7681;margin-top:4px}
.card-due.overdue{color:#f85149;font-weight:600}
.card-desc{font-size:11px;color:#6e7681;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;margin-top:2px}
.add-col-panel{background:#161b22;border:1px dashed #30363d;border-radius:8px;padding:10px;width:200px;flex-shrink:0;display:flex;flex-direction:column;gap:8px}
.add-col-panel input{background:#0d1117;border:1px solid #30363d;border-radius:6px;color:#c9d1d9;padding:7px 10px;font-size:13px;font-family:inherit;width:100%}
.add-col-panel input:focus{outline:none;border-color:#58a6ff}

/* Card edit */
.form-page{max-width:560px}
.label-picker{display:flex;gap:8px;align-items:center;flex-wrap:wrap;margin-top:4px}
.label-picker input[type=radio]{display:none}
.label-dot{width:24px;height:24px;border-radius:50%;cursor:pointer;display:inline-block;border:2px solid transparent;transition:border-color .1s}
.label-dot:hover{border-color:#c9d1d9}
.label-dot-none{background:#21262d;display:inline-flex;align-items:center;justify-content:center;font-size:12px;color:#6e7681}
.label-picker input[type=radio]:checked+.label-dot{border-color:#f0f6fc}
.label-picker input[type=radio]:checked+.label-dot-none{border-color:#f0f6fc;color:#c9d1d9}

@media(max-width:640px){
  main{padding:12px}
  header{padding:10px 16px}
  .board-wrap{margin:0 -12px;padding:0 12px 12px}
  .new-board-form{flex-direction:column;align-items:stretch;max-width:100%}
}
`

const baseTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Kanban</title>
<style>` + css + `</style>
</head>
<body>
<header>
  <span class="logo"><a href="/">Kanban</a></span>
  {{block "header-extra" .}}{{end}}
  <a href="/users" style="margin-left:auto;font-size:13px;color:#8b949e;text-decoration:none" onmouseover="this.style.color='#c9d1d9'" onmouseout="this.style.color='#8b949e'">Users</a>
</header>
<main>
{{block "content" .}}{{end}}
</main>
</body>
</html>`

const boardListTmpl = `{{define "content"}}
<div class="page-header">
  <h2>Boards</h2>
</div>
{{if .Boards}}
<div class="board-grid">
{{range .Boards}}
<a href="/boards/{{.ID}}" class="board-card">
  <div class="board-card-name">{{.Name}}</div>
  <div class="board-card-count">{{.CardCount}} card{{if ne .CardCount 1}}s{{end}}</div>
</a>
{{end}}
</div>
{{else}}
<p style="color:#6e7681;margin-bottom:24px">No boards yet. Create one below.</p>
{{end}}
<form method="POST" action="/boards" class="new-board-form">
  <input type="text" name="name" placeholder="New board name…" required autofocus>
  <button class="btn btn-primary" type="submit">Create Board</button>
</form>
{{end}}`

const boardViewTmpl = `{{define "header-extra"}}
<a href="/" class="header-back">← All Boards</a>
<span class="header-title" id="board-title" contenteditable="true"
  data-id="{{.Board.ID}}"
  onblur="renameBoard(this)"
  onkeydown="if(event.key==='Enter'){event.preventDefault();this.blur()}">{{.Board.Name}}</span>
<div style="display:flex;gap:8px;align-items:center;margin-left:auto">
  <a href="/boards/{{.Board.ID}}/cards/new" class="btn btn-primary btn-sm">+ Add Card</a>
  <form method="POST" action="/boards/{{.Board.ID}}/delete"
    onsubmit="return confirm('Delete this board and all its cards?')">
    <button class="btn btn-sm btn-danger">Delete Board</button>
  </form>
</div>
{{end}}
{{define "content"}}
<div class="board-wrap">
<div class="board-cols" id="board-cols">

{{range .Columns}}
<div class="column" data-col-id="{{.ID}}">
  <div class="col-header">
    <span class="col-name" contenteditable="true"
      data-id="{{.ID}}"
      onblur="renameCol(this)"
      onkeydown="if(event.key==='Enter'){event.preventDefault();this.blur()}">{{.Name}}</span>
    <form method="POST" action="/columns/{{.ID}}/delete"
      onsubmit="return confirm('Delete column and all its cards?')" style="display:inline">
      <button class="col-delete" type="submit" title="Delete column">×</button>
    </form>
  </div>

  <div class="card-list" id="cl-{{.ID}}"
    ondragover="colDragOver(event)"
    ondragleave="colDragLeave(event)"
    ondrop="colDrop(event,{{.ID}})">
    {{$colID := .ID}}
    {{range .Cards}}
    <div class="card" id="card-{{.ID}}"
      draggable="true"
      ondragstart="dragStart(event,{{.ID}})"
      ondragend="dragEnd(event)"
      ondragover="cardDragOver(event,{{.ID}})"
      ondragleave="cardDragLeave(event)"
      ondrop="cardDrop(event,{{$colID}},{{.ID}})">
      <div class="card-top">
        {{if .Label}}<span class="card-label" style="background:{{labelColor .Label}}"></span>{{end}}
        <a href="/cards/{{.ID}}" class="card-title">{{.Title}}</a>
      </div>
      {{if .DueDateStr}}
      <div class="card-due{{if .Overdue}} overdue{{end}}">{{if .Overdue}}⚠ {{end}}Due {{.DueDateStr}}</div>
      {{end}}
      {{if .Description}}<div class="card-desc">{{.Description}}</div>{{end}}
    </div>
    {{end}}
  </div>

</div>
{{end}}

<div class="add-col-panel">
  <form method="POST" action="/boards/{{.Board.ID}}/columns">
    <input type="text" name="name" placeholder="Column name…">
    <button class="btn btn-primary" type="submit" style="width:100%;margin-top:6px">+ Add Column</button>
  </form>
</div>

</div>
</div>

<script>
let dragId = null;

function dragStart(e, id) {
  dragId = id;
  e.currentTarget.classList.add('dragging');
  e.dataTransfer.effectAllowed = 'move';
}
function dragEnd(e) {
  e.currentTarget.classList.remove('dragging');
  document.querySelectorAll('.drop-before').forEach(el => el.classList.remove('drop-before'));
}
function cardDragOver(e, id) {
  if (dragId == id) return;
  e.preventDefault();
  e.stopPropagation();
  document.querySelectorAll('.drop-before').forEach(el => el.classList.remove('drop-before'));
  e.currentTarget.classList.add('drop-before');
}
function cardDragLeave(e) {
  e.currentTarget.classList.remove('drop-before');
}
function cardDrop(e, colId, beforeId) {
  e.preventDefault();
  e.stopPropagation();
  e.currentTarget.classList.remove('drop-before');
  if (!dragId || dragId == beforeId) return;
  moveCard(dragId, colId, beforeId);
}
function colDragOver(e) {
  e.preventDefault();
  document.querySelectorAll('.drop-before').forEach(el => el.classList.remove('drop-before'));
  e.currentTarget.classList.add('col-drag-over');
}
function colDragLeave(e) {
  e.currentTarget.classList.remove('col-drag-over');
}
function colDrop(e, colId) {
  e.preventDefault();
  e.currentTarget.classList.remove('col-drag-over');
  if (!dragId) return;
  moveCard(dragId, colId, 0);
}
function moveCard(cardId, colId, beforeId) {
  fetch('/api/move', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({card_id: cardId, column_id: colId, before_id: beforeId})
  }).then(r => { if (r.ok) location.reload(); });
}
function renameCol(el) {
  const name = el.textContent.trim();
  if (!name) return;
  fetch('/columns/' + el.dataset.id + '/rename', {
    method: 'POST',
    headers: {'Content-Type': 'application/x-www-form-urlencoded'},
    body: 'name=' + encodeURIComponent(name)
  });
}
function renameBoard(el) {
  const name = el.textContent.trim();
  if (!name) return;
  fetch('/boards/' + el.dataset.id + '/rename', {
    method: 'POST',
    headers: {'Content-Type': 'application/x-www-form-urlencoded'},
    body: 'name=' + encodeURIComponent(name)
  });
}
</script>
{{end}}`

const cardEditTmpl = `{{define "header-extra"}}
<a href="/boards/{{.BoardID}}" class="header-back">← {{.BoardName}}</a>
<span class="header-back" style="color:#6e7681">/ {{.ColumnName}}</span>
{{end}}
{{define "content"}}
<div class="form-page">
<form method="POST">
  <div class="form-group">
    <label>Title</label>
    <input type="text" name="title" value="{{.Card.Title}}" autofocus required>
  </div>
  <div class="form-group">
    <label>Description</label>
    <textarea name="description">{{.Card.Description}}</textarea>
  </div>
  <div class="form-group">
    <label>Label</label>
    <div class="label-picker">
      {{range .LabelOptions}}
      {{if eq .Name ""}}
      <input type="radio" name="label" value="" id="lbl-none" {{if eq $.Card.Label ""}}checked{{end}}>
      <label for="lbl-none" class="label-dot label-dot-none" title="None">✕</label>
      {{else}}
      <input type="radio" name="label" value="{{.Name}}" id="lbl-{{.Name}}" {{if eq $.Card.Label .Name}}checked{{end}}>
      <label for="lbl-{{.Name}}" class="label-dot" style="background:{{.Color}}" title="{{.Name}}"></label>
      {{end}}
      {{end}}
    </div>
  </div>
  <div class="form-group">
    <label>Due Date</label>
    <input type="date" name="due_date" value="{{.Card.DueDateStr}}">
  </div>
  <div class="form-group">
    <label>Created By</label>
    <select name="created_by">
      <option value="">— none —</option>
      {{range .Users}}
      <option value="{{.ID}}" {{if and $.Card.CreatedByID (eq .ID (deref $.Card.CreatedByID))}}selected{{end}}>
        {{if eq .Kind "agent"}}🤖 {{end}}{{.Name}}
      </option>
      {{end}}
    </select>
    {{if .Card.CreatedByName}}
    <p style="font-size:12px;color:#6e7681;margin-top:4px">
      Currently: {{if eq .Card.CreatedByKind "agent"}}🤖 {{end}}{{.Card.CreatedByName}}
    </p>
    {{end}}
  </div>
  <div class="form-actions">
    <button class="btn btn-primary" type="submit">Save</button>
    <a href="/boards/{{.BoardID}}" class="btn btn-cancel">Cancel</a>
  </div>
</form>
<form method="POST" action="/cards/{{.Card.ID}}/delete" style="margin-top:32px"
  onsubmit="return confirm('Delete this card?')">
  <button class="btn btn-danger btn-sm" type="submit">Delete Card</button>
</form>
</div>
{{end}}`

const cardNewTmpl = `{{define "header-extra"}}
<a href="/boards/{{.Board.ID}}" class="header-back">← {{.Board.Name}}</a>
<span class="header-back" style="color:#6e7681">/ New Card</span>
{{end}}
{{define "content"}}
<div class="form-page">
<form method="POST">
  <div class="form-group">
    <label>Title *</label>
    <input type="text" name="title" autofocus required placeholder="e.g. Fix login bug">
  </div>
  <div class="form-group">
    <label>Column *</label>
    <select name="column_id" required>
      <option value="">— choose a column —</option>
      {{range .Columns}}
      <option value="{{.ID}}">{{.Name}}</option>
      {{end}}
    </select>
  </div>
  <div class="form-group">
    <label>Created By</label>
    <select name="created_by">
      <option value="">— none —</option>
      {{range .Users}}
      <option value="{{.ID}}">{{if eq .Kind "agent"}}🤖 {{end}}{{.Name}}</option>
      {{end}}
    </select>
  </div>
  <div class="form-actions">
    <button class="btn btn-primary" type="submit">Create Card</button>
    <a href="/boards/{{.Board.ID}}" class="btn btn-cancel">Cancel</a>
  </div>
</form>
</div>
{{end}}`

const userListTmpl = `{{define "content"}}
<div class="page-header">
  <h2>Users</h2>
  <a href="/" class="btn btn-cancel btn-sm">← Boards</a>
</div>
{{if .Users}}
<table style="width:100%;max-width:560px;border-collapse:collapse;margin-bottom:28px">
<thead><tr>
  <th style="text-align:left;padding:8px 12px;background:#161b22;border-bottom:1px solid #30363d;color:#8b949e;font-size:12px;text-transform:uppercase">Name</th>
  <th style="text-align:left;padding:8px 12px;background:#161b22;border-bottom:1px solid #30363d;color:#8b949e;font-size:12px;text-transform:uppercase">Kind</th>
  <th style="padding:8px 12px;background:#161b22;border-bottom:1px solid #30363d"></th>
</tr></thead>
<tbody>
{{range .Users}}
<tr style="border-bottom:1px solid #21262d">
  <td style="padding:9px 12px">{{if eq .Kind "agent"}}🤖 {{end}}{{.Name}}</td>
  <td style="padding:9px 12px;color:#8b949e;font-size:13px">{{.Kind}}</td>
  <td style="padding:9px 12px;text-align:right">
    <form method="POST" action="/users/{{.ID}}/delete" style="display:inline"
      onsubmit="return confirm('Delete user?')">
      <button class="btn btn-sm btn-danger" type="submit">Delete</button>
    </form>
  </td>
</tr>
{{end}}
</tbody>
</table>
{{else}}
<p style="color:#6e7681;margin-bottom:24px">No users yet.</p>
{{end}}
<form method="POST" action="/users" style="display:flex;gap:8px;align-items:flex-end;max-width:560px;flex-wrap:wrap">
  <div class="form-group" style="flex:1;margin:0">
    <label>Name</label>
    <input type="text" name="name" placeholder="e.g. Alice or gpt-agent-1" required>
  </div>
  <div class="form-group" style="margin:0">
    <label>Kind</label>
    <select name="kind">
      <option value="human">Human</option>
      <option value="agent">Agent</option>
    </select>
  </div>
  <button class="btn btn-primary" type="submit" style="margin-bottom:16px">Add User</button>
</form>
{{end}}`

// ── Template engine ───────────────────────────────────────────────────────────

var pages map[string]*template.Template

func initTemplates() {
	funcMap := template.FuncMap{
		"labelColor": func(label string) string {
			if c, ok := labelColorMap[label]; ok {
				return c
			}
			return "#6e7681"
		},
		"deref": func(p *int) int {
			if p == nil {
				return 0
			}
			return *p
		},
	}
	base := template.Must(template.New("base").Funcs(funcMap).Parse(baseTmpl))
	add := func(name, tmpl string) {
		t := template.Must(template.Must(base.Clone()).Funcs(funcMap).Parse(tmpl))
		if pages == nil {
			pages = map[string]*template.Template{}
		}
		pages[name] = t
	}
	add("board-list", boardListTmpl)
	add("board-view", boardViewTmpl)
	add("card-new", cardNewTmpl)
	add("card-edit", cardEditTmpl)
	add("user-list", userListTmpl)
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

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	listen := os.Getenv("KANBAN_LISTEN")
	if listen == "" {
		listen = defaultListen
	}
	dsn := os.Getenv("KANBAN_DB_DSN")
	if dsn == "" {
		dsn = defaultDSN
	}

	ctx := context.Background()
	app, err := newApp(ctx, dsn)
	if err != nil {
		log.Fatal("db connect:", err)
	}
	if err := app.initSchema(ctx); err != nil {
		log.Fatal("schema:", err)
	}

	initTemplates()

	mux := http.NewServeMux()
	app.registerRoutes(mux)

	log.Printf("Kanban on http://0.0.0.0%s", listen)
	if err := http.ListenAndServe(listen, mux); err != nil {
		log.Fatal(err)
	}
}
