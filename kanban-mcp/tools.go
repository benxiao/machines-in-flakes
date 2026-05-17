package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTools(s *server.MCPServer, q *Queries) {

	// ── list_boards ──────────────────────────────────────────────────────────

	s.AddTool(
		mcp.NewTool("list_boards",
			mcp.WithDescription("List all Kanban boards with their card counts"),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			boards, err := q.ListBoards(ctx)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(formatBoards(boards)), nil
		},
	)

	// ── list_board ───────────────────────────────────────────────────────────

	s.AddTool(
		mcp.NewTool("list_board",
			mcp.WithDescription("Show all columns and cards on a board. Use this to get a full picture of current work."),
			mcp.WithNumber("board_id", mcp.Required(), mcp.Description("Board ID from list_boards")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id := req.GetInt("board_id", 0)
			if id <= 0 {
				return mcp.NewToolResultText("board_id must be a positive integer"), nil
			}
			cols, byCol, err := q.ListBoardCards(ctx, id)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(formatBoard(cols, byCol)), nil
		},
	)

	// ── list_columns ─────────────────────────────────────────────────────────

	s.AddTool(
		mcp.NewTool("list_columns",
			mcp.WithDescription("List columns (stages) for a board. Use to get column IDs when creating or moving cards."),
			mcp.WithNumber("board_id", mcp.Required(), mcp.Description("Board ID")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id := req.GetInt("board_id", 0)
			if id <= 0 {
				return mcp.NewToolResultText("board_id must be a positive integer"), nil
			}
			cols, err := q.ListColumns(ctx, id)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(formatColumns(cols)), nil
		},
	)

	// ── get_card ─────────────────────────────────────────────────────────────

	s.AddTool(
		mcp.NewTool("get_card",
			mcp.WithDescription("Get full detail for a card including description, label, due date, and all comments"),
			mcp.WithNumber("card_id", mcp.Required(), mcp.Description("Card ID")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id := req.GetInt("card_id", 0)
			if id <= 0 {
				return mcp.NewToolResultText("card_id must be a positive integer"), nil
			}
			detail, err := q.GetCard(ctx, id)
			if errors.Is(err, pgx.ErrNoRows) {
				return mcp.NewToolResultText(fmt.Sprintf("card #%d not found", id)), nil
			}
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(formatCardDetail(detail)), nil
		},
	)

	// ── create_card ──────────────────────────────────────────────────────────

	s.AddTool(
		mcp.NewTool("create_card",
			mcp.WithDescription("Create a new card in a column. Use list_columns to get column_id, list_labels for label_id, list_users for created_by."),
			mcp.WithNumber("column_id", mcp.Required(), mcp.Description("Column (stage) to place the card in")),
			mcp.WithString("title", mcp.Required(), mcp.Description("Card title")),
			mcp.WithString("description", mcp.Description("Card description / details")),
			mcp.WithNumber("label_id", mcp.Description("Label ID from list_labels (omit for none)")),
			mcp.WithString("due_date", mcp.Description("Due date in YYYY-MM-DD format (omit for none)")),
			mcp.WithNumber("created_by", mcp.Description("User ID from list_users (omit for none)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			colID := req.GetInt("column_id", 0)
			title := strings.TrimSpace(req.GetString("title", ""))
			if colID <= 0 || title == "" {
				return mcp.NewToolResultText("column_id and title are required"), nil
			}
			p := CreateCardParams{
				ColumnID:    colID,
				Title:       title,
				Description: req.GetString("description", ""),
			}
			if lid := req.GetInt("label_id", 0); lid > 0 {
				p.LabelID = &lid
			}
			if ds := req.GetString("due_date", ""); ds != "" {
				t, err := time.Parse("2006-01-02", ds)
				if err != nil {
					return mcp.NewToolResultText("due_date must be YYYY-MM-DD"), nil
				}
				p.DueDate = &t
			}
			if uid := req.GetInt("created_by", 0); uid > 0 {
				p.CreatedBy = &uid
			}
			id, err := q.CreateCard(ctx, p)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(fmt.Sprintf("Created card #%d: %s", id, title)), nil
		},
	)

	// ── add_comment ──────────────────────────────────────────────────────────

	s.AddTool(
		mcp.NewTool("add_comment",
			mcp.WithDescription("Post a comment on a card. Use this to report status, findings, or updates."),
			mcp.WithNumber("card_id", mcp.Required(), mcp.Description("Card ID")),
			mcp.WithString("body", mcp.Required(), mcp.Description("Comment text")),
			mcp.WithNumber("user_id", mcp.Description("User ID to post as (use your agent's user ID from list_users)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cardID := req.GetInt("card_id", 0)
			body := strings.TrimSpace(req.GetString("body", ""))
			if cardID <= 0 || body == "" {
				return mcp.NewToolResultText("card_id and body are required"), nil
			}
			var userID *int
			if uid := req.GetInt("user_id", 0); uid > 0 {
				userID = &uid
			}
			id, err := q.AddComment(ctx, cardID, userID, body)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(fmt.Sprintf("Added comment #%d on card #%d", id, cardID)), nil
		},
	)

	// ── move_card ────────────────────────────────────────────────────────────

	s.AddTool(
		mcp.NewTool("move_card",
			mcp.WithDescription("Move a card to a different column (stage), optionally positioning it before another card. Omit before_card_id to append to end."),
			mcp.WithNumber("card_id", mcp.Required(), mcp.Description("Card to move")),
			mcp.WithNumber("column_id", mcp.Required(), mcp.Description("Target column ID")),
			mcp.WithNumber("before_card_id", mcp.Description("Insert before this card ID (0 or omit = end of column)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cardID := req.GetInt("card_id", 0)
			colID := req.GetInt("column_id", 0)
			beforeID := req.GetInt("before_card_id", 0)
			if cardID <= 0 || colID <= 0 {
				return mcp.NewToolResultText("card_id and column_id are required"), nil
			}
			if err := q.MoveCard(ctx, cardID, colID, beforeID); err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(fmt.Sprintf("Moved card #%d to column #%d", cardID, colID)), nil
		},
	)

	// ── update_card ──────────────────────────────────────────────────────────

	s.AddTool(
		mcp.NewTool("update_card",
			mcp.WithDescription("Update one or more fields on a card. Only provided fields are changed."),
			mcp.WithNumber("card_id", mcp.Required(), mcp.Description("Card ID")),
			mcp.WithString("title", mcp.Description("New title")),
			mcp.WithString("description", mcp.Description("New description")),
			mcp.WithString("due_date", mcp.Description("New due date YYYY-MM-DD, or empty string to clear")),
			mcp.WithNumber("label_id", mcp.Description("New label ID, or 0 to clear")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id := req.GetInt("card_id", 0)
			if id <= 0 {
				return mcp.NewToolResultText("card_id is required"), nil
			}
			changed := []string{}
			if title := req.GetString("title", ""); title != "" {
				if err := q.UpdateCardTitle(ctx, id, title); err != nil {
					return nil, err
				}
				changed = append(changed, "title")
			}
			// description: sentinel "__unset__" means caller didn't provide it
			if desc := req.GetString("description", "__unset__"); desc != "__unset__" {
				if err := q.UpdateCardDescription(ctx, id, desc); err != nil {
					return nil, err
				}
				changed = append(changed, "description")
			}
			// due_date: empty string clears it, YYYY-MM-DD sets it, omitted = "__unset__"
			if ds := req.GetString("due_date", "__unset__"); ds != "__unset__" {
				if ds == "" {
					if err := q.UpdateCardDueDate(ctx, id, nil); err != nil {
						return nil, err
					}
				} else {
					t, err := time.Parse("2006-01-02", ds)
					if err != nil {
						return mcp.NewToolResultText("due_date must be YYYY-MM-DD or empty string to clear"), nil
					}
					if err := q.UpdateCardDueDate(ctx, id, &t); err != nil {
						return nil, err
					}
				}
				changed = append(changed, "due_date")
			}
			// label_id: -1 = unset, 0 = clear, >0 = set
			if n := req.GetInt("label_id", -1); n != -1 {
				if n == 0 {
					if err := q.UpdateCardLabel(ctx, id, nil); err != nil {
						return nil, err
					}
				} else {
					if err := q.UpdateCardLabel(ctx, id, &n); err != nil {
						return nil, err
					}
				}
				changed = append(changed, "label")
			}
			if len(changed) == 0 {
				return mcp.NewToolResultText("No fields updated — provide at least one of: title, description, due_date, label_id"), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Updated card #%d: %s", id, strings.Join(changed, ", "))), nil
		},
	)

	// ── list_labels ──────────────────────────────────────────────────────────

	s.AddTool(
		mcp.NewTool("list_labels",
			mcp.WithDescription("List all available labels with their IDs and colors"),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			labels, err := q.ListLabels(ctx)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(formatLabels(labels)), nil
		},
	)

	// ── list_users ───────────────────────────────────────────────────────────

	s.AddTool(
		mcp.NewTool("list_users",
			mcp.WithDescription("List all users (humans and agents). Use to find your agent user ID for created_by and user_id parameters."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			users, err := q.ListUsers(ctx)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(formatUsers(users)), nil
		},
	)
}

// ── Formatters ────────────────────────────────────────────────────────────────

func formatBoards(boards []BoardRow) string {
	if len(boards) == 0 {
		return "No boards found."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d board(s):\n", len(boards))
	for _, r := range boards {
		fmt.Fprintf(&b, "  #%d: %s  (%d cards)\n", r.ID, r.Name, r.CardCount)
	}
	return b.String()
}

func formatColumns(cols []ColumnRow) string {
	if len(cols) == 0 {
		return "No columns found."
	}
	var b strings.Builder
	for _, c := range cols {
		fmt.Fprintf(&b, "  #%d: %s\n", c.ID, c.Name)
	}
	return b.String()
}

func formatBoard(cols []ColumnRow, byCol map[int][]CardRow) string {
	if len(cols) == 0 {
		return "Board has no columns."
	}
	var b strings.Builder
	total := 0
	for _, cards := range byCol {
		total += len(cards)
	}
	fmt.Fprintf(&b, "%d column(s), %d card(s) total\n", len(cols), total)
	for _, col := range cols {
		cards := byCol[col.ID]
		fmt.Fprintf(&b, "\n[%s] (#%d) — %d card(s)\n", col.Name, col.ID, len(cards))
		for _, c := range cards {
			line := fmt.Sprintf("  #%d: %s", c.ID, c.Title)
			if c.LabelName != "" {
				line += fmt.Sprintf("  [%s]", c.LabelName)
			}
			if c.DueDate != nil {
				due := c.DueDate.Format("2006-01-02")
				if c.Overdue {
					due = "OVERDUE:" + due
				}
				line += "  due:" + due
			}
			if c.CreatedBy != "" {
				prefix := ""
				if c.CreatedByKind == "agent" {
					prefix = "🤖 "
				}
				line += "  by:" + prefix + c.CreatedBy
			}
			fmt.Fprintln(&b, line)
			if c.Description != "" {
				// Show first line of description
				first := c.Description
				if i := strings.Index(first, "\n"); i >= 0 {
					first = first[:i] + "…"
				}
				if len(first) > 80 {
					first = first[:80] + "…"
				}
				fmt.Fprintf(&b, "    %s\n", first)
			}
		}
	}
	return b.String()
}

func formatCardDetail(d CardDetail) string {
	c := d.Card
	var b strings.Builder
	fmt.Fprintf(&b, "Card #%d: %s\n", c.ID, c.Title)
	fmt.Fprintf(&b, "  Column:  %s (#%d)\n", c.ColumnName, c.ColumnID)
	if c.LabelName != "" {
		fmt.Fprintf(&b, "  Label:   %s (%s)\n", c.LabelName, c.LabelColor)
	}
	if c.DueDate != nil {
		due := c.DueDate.Format("2006-01-02")
		if c.Overdue {
			due += " (OVERDUE)"
		}
		fmt.Fprintf(&b, "  Due:     %s\n", due)
	}
	if c.CreatedBy != "" {
		prefix := ""
		if c.CreatedByKind == "agent" {
			prefix = "🤖 "
		}
		fmt.Fprintf(&b, "  Created by: %s%s\n", prefix, c.CreatedBy)
	}
	if c.Description != "" {
		fmt.Fprintf(&b, "\nDescription:\n%s\n", c.Description)
	}
	if len(d.Comments) == 0 {
		fmt.Fprintf(&b, "\nNo comments.\n")
	} else {
		fmt.Fprintf(&b, "\nComments (%d):\n", len(d.Comments))
		for _, co := range d.Comments {
			prefix := ""
			if co.UserKind == "agent" {
				prefix = "🤖 "
			}
			author := co.UserName
			if author == "" {
				author = "Anonymous"
			}
			fmt.Fprintf(&b, "\n  [%s] %s%s:\n", co.CreatedAt.Local().Format("2006-01-02 15:04"), prefix, author)
			// indent body
			for _, line := range strings.Split(co.Body, "\n") {
				fmt.Fprintf(&b, "    %s\n", line)
			}
		}
	}
	return b.String()
}

func formatLabels(labels []LabelRow) string {
	if len(labels) == 0 {
		return "No labels defined."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d label(s):\n", len(labels))
	for _, l := range labels {
		fmt.Fprintf(&b, "  #%d: %s  (%s)\n", l.ID, l.Name, l.Color)
	}
	return b.String()
}

func formatUsers(users []UserRow) string {
	if len(users) == 0 {
		return "No users defined."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d user(s):\n", len(users))
	for _, u := range users {
		prefix := ""
		if u.Kind == "agent" {
			prefix = "🤖 "
		}
		fmt.Fprintf(&b, "  #%d: %s%s  [%s]\n", u.ID, prefix, u.Name, u.Kind)
	}
	return b.String()
}
