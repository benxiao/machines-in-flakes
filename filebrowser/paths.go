package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func (a *App) handlePathsList(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT id, path FROM indexed_paths WHERE user_id = $1 ORDER BY path`, uid(r))
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	defer rows.Close()
	var paths []PathRow
	for rows.Next() {
		var p PathRow
		if err := rows.Scan(&p.ID, &p.Path); err != nil {
			continue
		}
		paths = append(paths, p)
	}
	render(w, "paths", PathsPage{ActiveTab: "paths", Paths: paths})
}

func (a *App) handlePathAdd(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	r.ParseForm()
	path := strings.TrimSpace(r.FormValue("path"))
	if path == "" {
		http.Redirect(w, r, "/paths", http.StatusFound)
		return
	}
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil {
		a.renderPathsWithError(w, r, fmt.Sprintf("%q: path not found", path))
		return
	}
	if !info.IsDir() {
		a.renderPathsWithError(w, r, fmt.Sprintf("%q is not a directory", path))
		return
	}
	_, err = a.db.Exec(r.Context(), `INSERT INTO indexed_paths (path, user_id) VALUES ($1, $2) ON CONFLICT (user_id, path) DO NOTHING`, path, uid(r))
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	userID := uid(r)
	go func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		a.reindexUser(ctx2, userID)
	}()
	http.Redirect(w, r, "/settings", http.StatusFound)
}

func (a *App) renderPathsWithError(w http.ResponseWriter, r *http.Request, errMsg string) {
	http.Redirect(w, r, "/settings?err="+url.QueryEscape(errMsg), http.StatusFound)
}

func (a *App) handlePathDelete(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	_, err := a.db.Exec(r.Context(), `DELETE FROM indexed_paths WHERE id=$1 AND user_id=$2`, id, uid(r))
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusFound)
}

func (a *App) handlePathToggle(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	enabled := r.FormValue("enabled") == "1"
	tag, err := a.db.Exec(r.Context(), `UPDATE indexed_paths SET enabled=$1 WHERE id=$2 AND user_id=$3`, enabled, id, uid(r))
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	if tag.RowsAffected() == 0 {
		http.NotFound(w, r)
		return
	}
	userID := uid(r)
	go func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		a.reindexUser(ctx2, userID)
	}()
	w.WriteHeader(http.StatusNoContent)
}

// ---- Settings page handlers ----

func (a *App) handleSettingsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := uid(r)
	admin := isAdmin(r)

	var pathQuery string
	if admin {
		pathQuery = `SELECT id, path, enabled FROM indexed_paths WHERE user_id = $1 ORDER BY path`
	} else {
		pathQuery = `SELECT ip.id, ip.path, ip.enabled FROM indexed_paths ip JOIN path_grants pg ON pg.path_id = ip.id WHERE pg.user_id = $1 AND ip.enabled = TRUE ORDER BY ip.path`
	}
	rows, err := a.db.Query(ctx, pathQuery, userID)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	var paths []PathRow
	for rows.Next() {
		var p PathRow
		if rows.Scan(&p.ID, &p.Path, &p.Enabled) == nil {
			paths = append(paths, p)
		}
	}
	rows.Close()

	pathErr := r.URL.Query().Get("err")
	render(w, "settings", SettingsPage{
		ActiveTab: "settings",
		IsAdmin:   admin,
		Paths:     paths,
		PathError: pathErr,
	})
}


func (a *App) handlePathSize(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	path = filepath.Clean(path)
	// The settings page shows sizes for the user's own roots (even disabled
	// ones) and for enabled granted roots; anything else is off limits.
	var count int
	err := a.db.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM indexed_paths ip
		WHERE ($2 = ip.path OR starts_with($2, ip.path || '/'))
		  AND (ip.user_id = $1
		       OR (ip.enabled AND EXISTS (SELECT 1 FROM path_grants pg WHERE pg.path_id = ip.id AND pg.user_id = $1)))
	`, uid(r), path).Scan(&count)
	if err != nil || count == 0 {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	out, err := exec.CommandContext(r.Context(), "du", "-sb", path).Output()
	var gb float64
	if err == nil {
		var bytes int64
		fmt.Sscanf(string(out), "%d", &bytes)
		gb = float64(bytes) / 1e9
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"gb":%.2f}`, gb)
}
