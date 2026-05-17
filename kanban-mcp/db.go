package main

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Queries struct{ db *pgxpool.Pool }

// ── Boards ────────────────────────────────────────────────────────────────────

type BoardRow struct {
	ID        int
	Name      string
	CardCount int
}

func (q *Queries) ListBoards(ctx context.Context) ([]BoardRow, error) {
	rows, err := q.db.Query(ctx, `
		SELECT b.id, b.name, COUNT(c.id)
		FROM boards b
		LEFT JOIN columns col ON col.board_id=b.id
		LEFT JOIN cards c ON c.column_id=col.id
		GROUP BY b.id, b.name ORDER BY b.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BoardRow
	for rows.Next() {
		var r BoardRow
		if err := rows.Scan(&r.ID, &r.Name, &r.CardCount); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── Columns ───────────────────────────────────────────────────────────────────

type ColumnRow struct {
	ID      int
	BoardID int
	Name    string
}

func (q *Queries) ListColumns(ctx context.Context, boardID int) ([]ColumnRow, error) {
	rows, err := q.db.Query(ctx,
		`SELECT id, board_id, name FROM columns WHERE board_id=$1 ORDER BY position`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ColumnRow
	for rows.Next() {
		var r ColumnRow
		if err := rows.Scan(&r.ID, &r.BoardID, &r.Name); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── Cards ─────────────────────────────────────────────────────────────────────

type CardRow struct {
	ID          int
	ColumnID    int
	ColumnName  string
	Title       string
	Description string
	LabelName   string
	LabelColor  string
	DueDate     *time.Time
	Overdue     bool
	CreatedByID *int
	CreatedBy   string
	CreatedByKind string
	Position    int
}

func (q *Queries) ListBoardCards(ctx context.Context, boardID int) ([]ColumnRow, map[int][]CardRow, error) {
	cols, err := q.ListColumns(ctx, boardID)
	if err != nil {
		return nil, nil, err
	}
	if len(cols) == 0 {
		return cols, nil, nil
	}
	colIDs := make([]int, len(cols))
	for i, c := range cols {
		colIDs[i] = c.ID
	}
	rows, err := q.db.Query(ctx, `
		SELECT c.id, c.column_id, col.name, c.title, c.description,
		       COALESCE(l.name,''), COALESCE(l.color,''),
		       c.due_date, c.created_by, COALESCE(u.name,''), COALESCE(u.kind,''),
		       c.position
		FROM cards c
		JOIN columns col ON col.id=c.column_id
		LEFT JOIN labels l ON l.id=c.label_id
		LEFT JOIN users u ON u.id=c.created_by
		WHERE c.column_id = ANY($1)
		ORDER BY c.column_id, c.position`, colIDs)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	byCol := map[int][]CardRow{}
	today := time.Now().Truncate(24 * time.Hour)
	for rows.Next() {
		var r CardRow
		if err := rows.Scan(&r.ID, &r.ColumnID, &r.ColumnName, &r.Title, &r.Description,
			&r.LabelName, &r.LabelColor, &r.DueDate,
			&r.CreatedByID, &r.CreatedBy, &r.CreatedByKind, &r.Position); err != nil {
			return nil, nil, err
		}
		if r.DueDate != nil {
			r.Overdue = r.DueDate.Before(today)
		}
		byCol[r.ColumnID] = append(byCol[r.ColumnID], r)
	}
	return cols, byCol, rows.Err()
}

// ── Card detail ───────────────────────────────────────────────────────────────

type CommentRow struct {
	ID        int
	UserName  string
	UserKind  string
	Body      string
	CreatedAt time.Time
}

type CardDetail struct {
	Card     CardRow
	Comments []CommentRow
}

func (q *Queries) GetCard(ctx context.Context, id int) (CardDetail, error) {
	var d CardDetail
	today := time.Now().Truncate(24 * time.Hour)
	err := q.db.QueryRow(ctx, `
		SELECT c.id, c.column_id, col.name, c.title, c.description,
		       COALESCE(l.name,''), COALESCE(l.color,''),
		       c.due_date, c.created_by, COALESCE(u.name,''), COALESCE(u.kind,''),
		       c.position
		FROM cards c
		JOIN columns col ON col.id=c.column_id
		LEFT JOIN labels l ON l.id=c.label_id
		LEFT JOIN users u ON u.id=c.created_by
		WHERE c.id=$1`, id).
		Scan(&d.Card.ID, &d.Card.ColumnID, &d.Card.ColumnName, &d.Card.Title, &d.Card.Description,
			&d.Card.LabelName, &d.Card.LabelColor, &d.Card.DueDate,
			&d.Card.CreatedByID, &d.Card.CreatedBy, &d.Card.CreatedByKind, &d.Card.Position)
	if err != nil {
		return d, err
	}
	if d.Card.DueDate != nil {
		d.Card.Overdue = d.Card.DueDate.Before(today)
	}

	rows, err := q.db.Query(ctx, `
		SELECT co.id, COALESCE(u.name,''), COALESCE(u.kind,''), co.body, co.created_at
		FROM comments co LEFT JOIN users u ON u.id=co.user_id
		WHERE co.card_id=$1 ORDER BY co.created_at`, id)
	if err != nil {
		return d, err
	}
	defer rows.Close()
	for rows.Next() {
		var c CommentRow
		if err := rows.Scan(&c.ID, &c.UserName, &c.UserKind, &c.Body, &c.CreatedAt); err != nil {
			return d, err
		}
		d.Comments = append(d.Comments, c)
	}
	return d, rows.Err()
}

// ── Create card ───────────────────────────────────────────────────────────────

type CreateCardParams struct {
	ColumnID    int
	Title       string
	Description string
	LabelID     *int
	DueDate     *time.Time
	CreatedBy   *int
}

func (q *Queries) CreateCard(ctx context.Context, p CreateCardParams) (int, error) {
	var maxPos int
	q.db.QueryRow(ctx, `SELECT COALESCE(MAX(position),0) FROM cards WHERE column_id=$1`, p.ColumnID).Scan(&maxPos)
	var id int
	err := q.db.QueryRow(ctx, `
		INSERT INTO cards (column_id, title, description, label_id, due_date, created_by, position)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		p.ColumnID, p.Title, p.Description, p.LabelID, p.DueDate, p.CreatedBy, maxPos+1000).Scan(&id)
	return id, err
}

// ── Add comment ───────────────────────────────────────────────────────────────

func (q *Queries) AddComment(ctx context.Context, cardID int, userID *int, body string) (int, error) {
	var id int
	err := q.db.QueryRow(ctx,
		`INSERT INTO comments (card_id, user_id, body) VALUES ($1,$2,$3) RETURNING id`,
		cardID, userID, body).Scan(&id)
	return id, err
}

// ── Move card ─────────────────────────────────────────────────────────────────

func (q *Queries) MoveCard(ctx context.Context, cardID, columnID, beforeID int) error {
	rows, err := q.db.Query(ctx,
		`SELECT id FROM cards WHERE column_id=$1 AND id!=$2 ORDER BY position`,
		columnID, cardID)
	if err != nil {
		return err
	}
	var ids []int
	for rows.Next() {
		var id int
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()

	insertAt := len(ids)
	if beforeID != 0 {
		for i, id := range ids {
			if id == beforeID {
				insertAt = i
				break
			}
		}
	}
	ordered := make([]int, 0, len(ids)+1)
	ordered = append(ordered, ids[:insertAt]...)
	ordered = append(ordered, cardID)
	ordered = append(ordered, ids[insertAt:]...)

	tx, err := q.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for i, id := range ordered {
		if _, err := tx.Exec(ctx,
			`UPDATE cards SET column_id=$1, position=$2 WHERE id=$3`,
			columnID, (i+1)*1000, id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ── Update card field ─────────────────────────────────────────────────────────

func (q *Queries) UpdateCardTitle(ctx context.Context, id int, title string) error {
	_, err := q.db.Exec(ctx, `UPDATE cards SET title=$1 WHERE id=$2`, title, id)
	return err
}

func (q *Queries) UpdateCardDescription(ctx context.Context, id int, desc string) error {
	_, err := q.db.Exec(ctx, `UPDATE cards SET description=$1 WHERE id=$2`, desc, id)
	return err
}

func (q *Queries) UpdateCardDueDate(ctx context.Context, id int, due *time.Time) error {
	_, err := q.db.Exec(ctx, `UPDATE cards SET due_date=$1 WHERE id=$2`, due, id)
	return err
}

func (q *Queries) UpdateCardLabel(ctx context.Context, id int, labelID *int) error {
	_, err := q.db.Exec(ctx, `UPDATE cards SET label_id=$1 WHERE id=$2`, labelID, id)
	return err
}

// ── Labels & Users ────────────────────────────────────────────────────────────

type LabelRow struct {
	ID    int
	Name  string
	Color string
}

func (q *Queries) ListLabels(ctx context.Context) ([]LabelRow, error) {
	rows, err := q.db.Query(ctx, `SELECT id, name, color FROM labels ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LabelRow
	for rows.Next() {
		var r LabelRow
		rows.Scan(&r.ID, &r.Name, &r.Color)
		out = append(out, r)
	}
	return out, rows.Err()
}

type UserRow struct {
	ID   int
	Name string
	Kind string
}

func (q *Queries) ListUsers(ctx context.Context) ([]UserRow, error) {
	rows, err := q.db.Query(ctx, `SELECT id, name, kind FROM users ORDER BY kind, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UserRow
	for rows.Next() {
		var r UserRow
		rows.Scan(&r.ID, &r.Name, &r.Kind)
		out = append(out, r)
	}
	return out, rows.Err()
}
