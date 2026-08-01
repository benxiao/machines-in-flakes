package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// ---- Trash ----
//
// Deleted files/folders move into a hidden trash directory inside whichever
// indexed root they belong to (not one global location), so the move is a
// same-filesystem rename even for large media spanning separate ZFS pools.
// DB rows that reference the original path (video_positions, favorites,
// playlist_items, file_index) are left alone until permanent purge, so a
// restore makes everything valid again for free.

const trashDirName = ".filebrowser-trash"

// rootFor returns the longest allowed root absPath is under, or "" if none
// matches. Callers should already have verified access some other way.
func (a *App) rootFor(r *http.Request, absPath string) string {
	var best string
	for _, root := range a.allowedRoots(r) {
		if underRoot(absPath, root) && len(root) > len(best) {
			best = root
		}
	}
	return best
}

// underAllowedRoot is a lighter check than isAllowedPath for paths that may
// legitimately not exist on disk right now (a trashed item's original
// location) — pure lexical prefix matching, no symlink resolution.
func (a *App) underAllowedRoot(r *http.Request, absPath string) bool {
	for _, root := range a.allowedRoots(r) {
		if underRoot(absPath, root) {
			return true
		}
	}
	return false
}

// moveToTrash relocates absPath into its root's hidden trash directory and
// records where it went, so it can be restored or permanently purged later.
func (a *App) moveToTrash(r *http.Request, absPath string, isFolder bool) (string, error) {
	root := a.rootFor(r, absPath)
	if root == "" {
		return "", errors.New("no matching indexed root")
	}
	trashDir := filepath.Join(root, trashDirName, randToken())
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		return "", err
	}
	trashPath := filepath.Join(trashDir, filepath.Base(absPath))
	var moveErr error
	if isFolder {
		moveErr = moveDirAny(absPath, trashPath)
	} else {
		moveErr = moveFile(absPath, trashPath)
	}
	if moveErr != nil {
		os.RemoveAll(trashDir)
		return "", moveErr
	}
	if _, err := a.db.Exec(r.Context(),
		`INSERT INTO trash_items (original_path, trash_path, is_folder, deleted_by) VALUES ($1, $2, $3, $4)`,
		absPath, trashPath, isFolder, uid(r),
	); err != nil {
		return "", err
	}
	return trashPath, nil
}

// purgeTrashItem permanently removes a trashed item from disk and cleans up
// any DB state that still points at its original path.
func (a *App) purgeTrashItem(ctx context.Context, id int64, trashPath, originalPath string, isFolder bool) error {
	if err := os.RemoveAll(trashPath); err != nil {
		return err
	}
	os.Remove(filepath.Dir(trashPath)) // best-effort: drop the now-empty per-item dir
	os.Remove(thumbCachePath(originalPath))
	var dbErr error
	if isFolder {
		dbErr = a.purgeFolderRows(ctx, originalPath)
	} else {
		dbErr = a.purgeFileRows(ctx, []string{originalPath})
	}
	if dbErr != nil {
		log.Printf("trash purge db cleanup for %s: %v", originalPath, dbErr)
	}
	_, err := a.db.Exec(ctx, `DELETE FROM trash_items WHERE id = $1`, id)
	return err
}

// restoreTrashItem moves a trashed item back to its original location,
// refusing if something already occupies that path.
func (a *App) restoreTrashItem(ctx context.Context, id int64) error {
	var trashPath, originalPath string
	var isFolder bool
	if err := a.db.QueryRow(ctx,
		`SELECT trash_path, original_path, is_folder FROM trash_items WHERE id = $1`, id,
	).Scan(&trashPath, &originalPath, &isFolder); err != nil {
		return errors.New("not found")
	}
	if _, err := os.Lstat(originalPath); err == nil {
		return errors.New("a file already exists at the original location")
	}
	if err := os.MkdirAll(filepath.Dir(originalPath), 0755); err != nil {
		return err
	}
	var moveErr error
	if isFolder {
		moveErr = moveDirAny(trashPath, originalPath)
	} else {
		moveErr = moveFile(trashPath, originalPath)
	}
	if moveErr != nil {
		return moveErr
	}
	os.Remove(filepath.Dir(trashPath))
	_, err := a.db.Exec(ctx, `DELETE FROM trash_items WHERE id = $1`, id)
	return err
}

// purgeExpiredTrash permanently removes trash items past the retention
// window; called hourly from main.go.
func (a *App) purgeExpiredTrash(ctx context.Context) {
	rows, err := a.db.Query(ctx,
		`SELECT id, trash_path, original_path, is_folder FROM trash_items WHERE deleted_at < now() - interval '30 days'`)
	if err != nil {
		return
	}
	type expired struct {
		id                      int64
		trashPath, originalPath string
		isFolder                bool
	}
	var items []expired
	for rows.Next() {
		var it expired
		if rows.Scan(&it.id, &it.trashPath, &it.originalPath, &it.isFolder) == nil {
			items = append(items, it)
		}
	}
	rows.Close()
	for _, it := range items {
		if err := a.purgeTrashItem(ctx, it.id, it.trashPath, it.originalPath, it.isFolder); err != nil {
			log.Printf("trash retention purge %d: %v", it.id, err)
		} else {
			log.Printf("trash: retention-purged %s", it.originalPath)
		}
	}
}

func (a *App) renderTrashWithError(w http.ResponseWriter, r *http.Request, errMsg string) {
	http.Redirect(w, r, "/trash?err="+url.QueryEscape(errMsg), http.StatusFound)
}

func (a *App) handleTrashPage(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	rows, err := a.db.Query(r.Context(),
		`SELECT id, original_path, is_folder, deleted_at FROM trash_items ORDER BY deleted_at DESC`)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	defer rows.Close()
	var items []TrashItemRow
	for rows.Next() {
		var it TrashItemRow
		var t time.Time
		if rows.Scan(&it.ID, &it.OriginalPath, &it.IsFolder, &t) == nil {
			it.Name = filepath.Base(it.OriginalPath)
			it.DeletedAt = t.Local().Format("2006-01-02 15:04")
			items = append(items, it)
		}
	}
	render(w, "trash", TrashPage{ActiveTab: "trash", IsAdmin: true, Items: items, Error: r.URL.Query().Get("err")})
}

func (a *App) handleTrashRestore(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var originalPath string
	if err := a.db.QueryRow(r.Context(), `SELECT original_path FROM trash_items WHERE id = $1`, id).Scan(&originalPath); err != nil {
		http.NotFound(w, r)
		return
	}
	if !a.underAllowedRoot(r, originalPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := a.restoreTrashItem(r.Context(), id); err != nil {
		a.renderTrashWithError(w, r, err.Error())
		return
	}
	http.Redirect(w, r, "/trash", http.StatusFound)
}

func (a *App) handleTrashPurge(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var trashPath, originalPath string
	var isFolder bool
	if err := a.db.QueryRow(r.Context(),
		`SELECT trash_path, original_path, is_folder FROM trash_items WHERE id = $1`, id,
	).Scan(&trashPath, &originalPath, &isFolder); err != nil {
		http.NotFound(w, r)
		return
	}
	if !a.underAllowedRoot(r, originalPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := a.purgeTrashItem(r.Context(), id, trashPath, originalPath, isFolder); err != nil {
		httpErr(w, err, 500)
		return
	}
	http.Redirect(w, r, "/trash", http.StatusFound)
}

func (a *App) handleTrashEmpty(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	rows, err := a.db.Query(r.Context(), `SELECT id, trash_path, original_path, is_folder FROM trash_items`)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	type row struct {
		id                      int64
		trashPath, originalPath string
		isFolder                bool
	}
	var items []row
	for rows.Next() {
		var it row
		if rows.Scan(&it.id, &it.trashPath, &it.originalPath, &it.isFolder) == nil && a.underAllowedRoot(r, it.originalPath) {
			items = append(items, it)
		}
	}
	rows.Close()
	for _, it := range items {
		if err := a.purgeTrashItem(r.Context(), it.id, it.trashPath, it.originalPath, it.isFolder); err != nil {
			log.Printf("trash empty: purge %d: %v", it.id, err)
		}
	}
	http.Redirect(w, r, "/trash", http.StatusFound)
}
