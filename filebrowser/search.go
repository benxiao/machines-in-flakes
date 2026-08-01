package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ---- Search ----

type SearchResult struct {
	DirPath    string       `json:"dir_path"`
	MatchCount int64        `json:"match_count"`
	SamplePath string       `json:"sample_path"`
	SampleType string       `json:"sample_type"`
	Files      []SearchFile `json:"files"`
}

type SearchFile struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// searchTermConds builds one (filename OR folder-name) LIKE condition per
// whitespace-separated query term, so word order doesn't matter and a folder
// named "Pink Floyd" is found by its name even when its files aren't.
func searchTermConds(q string, args *[]any) []string {
	terms := strings.Fields(strings.ToLower(q))
	if len(terms) > 5 {
		terms = terms[:5]
	}
	var conds []string
	for _, t := range terms {
		*args = append(*args, "%"+t+"%")
		n := len(*args)
		conds = append(conds, fmt.Sprintf(
			"(lower(fi.filename) LIKE $%d OR lower(substring(fi.dir_path from '[^/]+$')) LIKE $%d)", n, n))
	}
	return conds
}

func (a *App) handleSearchStatus(w http.ResponseWriter, r *http.Request) {
	userID := uid(r)
	_, running := a.reindexing.Load(userID)
	var count int64
	a.db.QueryRow(r.Context(), `SELECT COUNT(*) FROM file_index WHERE user_id = $1`, userID).Scan(&count)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"running": running, "count": count})
}

func (a *App) reindexUser(ctx context.Context, userID int64) {
	if _, running := a.reindexing.LoadOrStore(userID, true); running {
		return
	}
	defer a.reindexing.Delete(userID)
	rows, err := a.db.Query(ctx, `
		SELECT ip.path FROM indexed_paths ip
		WHERE ip.enabled = TRUE
		  AND (ip.user_id = $1 OR EXISTS (SELECT 1 FROM path_grants pg WHERE pg.path_id = ip.id AND pg.user_id = $1))
	`, userID)
	if err != nil {
		log.Printf("reindex: query paths: %v", err)
		return
	}
	var roots []string
	for rows.Next() {
		var p string
		if rows.Scan(&p) == nil {
			roots = append(roots, p)
		}
	}
	rows.Close()

	type entry struct {
		path, filename, fileType, dirPath string
		mtime                             time.Time
	}
	var entries []entry
	seen := make(map[string]bool) // nested roots walk the same files twice
	for _, root := range roots {
		filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				if info.Name() == trashDirName {
					return filepath.SkipDir
				}
				return nil
			}
			if seen[p] {
				return nil
			}
			ft := classifyExt(filepath.Ext(p))
			if ft == "other" || ft == "text" {
				return nil
			}
			seen[p] = true
			entries = append(entries, entry{p, filepath.Base(p), ft, filepath.Dir(p), info.ModTime()})
			return nil
		})
	}

	tx, err := a.db.Begin(ctx)
	if err != nil {
		log.Printf("reindex: begin tx: %v", err)
		return
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM file_index WHERE user_id = $1`, userID); err != nil {
		log.Printf("reindex: delete: %v", err)
		return
	}
	if _, err := tx.CopyFrom(ctx,
		pgx.Identifier{"file_index"},
		[]string{"user_id", "path", "filename", "file_type", "dir_path", "mtime"},
		pgx.CopyFromSlice(len(entries), func(i int) ([]any, error) {
			e := entries[i]
			return []any{userID, e.path, e.filename, e.fileType, e.dirPath, e.mtime}, nil
		}),
	); err != nil {
		log.Printf("reindex: copy: %v", err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		log.Printf("reindex: commit: %v", err)
		return
	}

	// Purge stale video_positions for files that no longer exist.
	if len(entries) > 0 {
		paths := make([]string, len(entries))
		for i, e := range entries {
			paths[i] = e.path
		}
		if _, err := a.db.Exec(ctx,
			`DELETE FROM video_positions WHERE user_id = $1 AND NOT (path = ANY($2))`,
			userID, paths,
		); err != nil {
			log.Printf("reindex: purge positions: %v", err)
		}
	}

	log.Printf("reindex: user %d indexed %d files", userID, len(entries))

	if a.ffmpegPath != "" {
		var thumbable []string
		for _, e := range entries {
			if e.fileType == "video" || e.fileType == "photo" {
				thumbable = append(thumbable, e.path)
			}
		}
		go a.pregenThumbs(thumbable)
	}
}

func (a *App) handleSearchReindex(w http.ResponseWriter, r *http.Request) {
	userID := uid(r)
	go func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		a.reindexUser(ctx2, userID)
	}()
	w.WriteHeader(http.StatusNoContent)
}

// fileTypeRankExpr orders same-folder search matches so files of the same
// type sit together in a fixed order (matching the sf-chip order, plus the
// two types without a chip) rather than interleaved alphabetically.
const fileTypeRankExpr = `(CASE fi.file_type WHEN 'video' THEN 1 WHEN 'audio' THEN 2 WHEN 'photo' THEN 3 WHEN 'pdf' THEN 4 WHEN 'archive' THEN 5 ELSE 6 END)`

func (a *App) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	typ := r.URL.Query().Get("type")
	if typ == "" {
		typ = "all"
	}
	w.Header().Set("Content-Type", "application/json")
	if len(q) < 2 {
		w.Write([]byte("[]"))
		return
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	// $3 is the full lowered query, used only for prefix-match ranking:
	// folders whose name starts with the query outrank filename prefix
	// matches, which outrank plain substring matches; ties break on size.
	args := []any{uid(r), typ, strings.ToLower(q)}
	conds := searchTermConds(q, &args)
	args = append(args, offset)
	rows, err := a.db.Query(r.Context(), `
		SELECT fi.dir_path,
		       COUNT(*) AS match_count,
		       (array_agg(fi.path ORDER BY `+fileTypeRankExpr+`, fi.filename))[1:3] AS sample_paths,
		       (array_agg(fi.filename ORDER BY `+fileTypeRankExpr+`, fi.filename))[1:3] AS sample_names,
		       (array_agg(fi.file_type ORDER BY `+fileTypeRankExpr+`, fi.filename))[1:3] AS sample_types,
		       MAX(CASE WHEN lower(substring(fi.dir_path from '[^/]+$')) LIKE $3 || '%' THEN 3
		                WHEN lower(fi.filename) LIKE $3 || '%' THEN 2
		                ELSE 1 END) AS rank
		FROM file_index fi
		WHERE fi.user_id = $1
		  AND ($2 = 'all' OR fi.file_type = $2)
		  AND `+strings.Join(conds, "\n		  AND ")+`
		GROUP BY fi.dir_path
		ORDER BY rank DESC, COUNT(*) DESC, fi.dir_path
		LIMIT 20 OFFSET $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	defer rows.Close()
	var results []SearchResult
	for rows.Next() {
		var sr SearchResult
		var paths, names, types []string
		var rank int
		if rows.Scan(&sr.DirPath, &sr.MatchCount, &paths, &names, &types, &rank) != nil {
			continue
		}
		for i := range paths {
			if i < len(names) && i < len(types) {
				sr.Files = append(sr.Files, SearchFile{Path: paths[i], Name: names[i], Type: types[i]})
			}
		}
		if len(sr.Files) > 0 {
			sr.SamplePath = sr.Files[0].Path
			sr.SampleType = sr.Files[0].Type
		}
		results = append(results, sr)
	}
	if results == nil {
		results = []SearchResult{}
	}
	json.NewEncoder(w).Encode(results)
}

// handleSearchFiles lists the matching files inside one folder, for expanding
// a grouped search result in place. Same term semantics as handleSearch.
func (a *App) handleSearchFiles(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	dir := r.URL.Query().Get("dir")
	typ := r.URL.Query().Get("type")
	if typ == "" {
		typ = "all"
	}
	w.Header().Set("Content-Type", "application/json")
	if len(q) < 2 || dir == "" {
		w.Write([]byte("[]"))
		return
	}
	args := []any{uid(r), typ, dir}
	conds := searchTermConds(q, &args)
	rows, err := a.db.Query(r.Context(), `
		SELECT fi.path, fi.filename, fi.file_type
		FROM file_index fi
		WHERE fi.user_id = $1
		  AND ($2 = 'all' OR fi.file_type = $2)
		  AND fi.dir_path = $3
		  AND `+strings.Join(conds, "\n		  AND ")+`
		ORDER BY `+fileTypeRankExpr+`, fi.filename
		LIMIT 50`, args...)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	defer rows.Close()
	var files []SearchFile
	for rows.Next() {
		var f SearchFile
		if rows.Scan(&f.Path, &f.Name, &f.Type) == nil {
			files = append(files, f)
		}
	}
	if files == nil {
		files = []SearchFile{}
	}
	json.NewEncoder(w).Encode(files)
}
