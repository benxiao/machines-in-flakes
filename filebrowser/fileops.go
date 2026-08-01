package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// ---- File operations (admin only) ----

// fileOpCheck validates one source path for a file operation. It does not
// write to the response so bulk handlers can report partial failures.
func (a *App) fileOpCheck(r *http.Request, rawPath string) (string, error) {
	if rawPath == "" {
		return "", errors.New("empty path")
	}
	absPath := filepath.Clean(rawPath)
	if !a.isAllowedPath(r, absPath) {
		return "", errors.New("forbidden")
	}
	info, err := os.Lstat(absPath)
	if err != nil {
		return "", errors.New("not found")
	}
	if info.IsDir() {
		return "", errors.New("files only")
	}
	return absPath, nil
}

// purgeFileRows removes all DB state for deleted files. Paths are global
// identity, so rows for every user are removed.
func (a *App) purgeFileRows(ctx context.Context, paths []string) error {
	tx, err := a.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, q := range []string{
		`DELETE FROM file_index WHERE path = ANY($1)`,
		`DELETE FROM video_positions WHERE path = ANY($1)`,
		`DELETE FROM playlist_items WHERE path = ANY($1)`,
		`DELETE FROM favorites WHERE path = ANY($1) AND is_folder = FALSE`,
		`DELETE FROM file_hashes WHERE path = ANY($1)`,
	} {
		if _, err := tx.Exec(ctx, q, paths); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// repathFileRows points all DB state at a file's new path. Stale rows already
// at the new path (leftovers from a previously deleted file) are removed
// first so the unique constraints can't fail the UPDATEs.
func (a *App) repathFileRows(ctx context.Context, oldPath, newPath string) error {
	tx, err := a.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	steps := []struct {
		q    string
		args []any
	}{
		{`DELETE FROM file_index WHERE path = $1`, []any{newPath}},
		{`UPDATE file_index SET path = $1, filename = $2, dir_path = $3 WHERE path = $4`,
			[]any{newPath, filepath.Base(newPath), filepath.Dir(newPath), oldPath}},
		{`DELETE FROM video_positions WHERE path = $1`, []any{newPath}},
		{`UPDATE video_positions SET path = $1 WHERE path = $2`, []any{newPath, oldPath}},
		{`DELETE FROM playlist_items WHERE path = $1`, []any{newPath}},
		{`UPDATE playlist_items SET path = $1 WHERE path = $2`, []any{newPath, oldPath}},
		{`DELETE FROM favorites WHERE path = $1 AND is_folder = FALSE`, []any{newPath}},
		{`UPDATE favorites SET path = $1 WHERE path = $2 AND is_folder = FALSE`, []any{newPath, oldPath}},
		{`DELETE FROM file_hashes WHERE path = $1`, []any{newPath}},
		{`UPDATE file_hashes SET path = $1 WHERE path = $2`, []any{newPath, oldPath}},
	}
	for _, s := range steps {
		if _, err := tx.Exec(ctx, s.q, s.args...); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// moveFile renames, falling back to copy+remove when source and destination
// are on different filesystems (indexed roots span multiple ZFS pools).
func moveFile(src, dst string) error {
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}
	var lerr *os.LinkError
	if !errors.As(err, &lerr) || !errors.Is(lerr.Err, syscall.EXDEV) {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(dst)
		return err
	}
	return os.Remove(src)
}

func (a *App) handleFileDelete(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Paths) == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var deleted int
	var errs []string
	for _, p := range req.Paths {
		absPath, err := a.fileOpCheck(r, p)
		if err != nil {
			errs = append(errs, p+": "+err.Error())
			continue
		}
		if _, err := a.moveToTrash(r, absPath, false); err != nil {
			errs = append(errs, p+": "+err.Error())
			continue
		}
		deleted++
	}
	if deleted > 0 {
		log.Printf("file delete (to trash) u=%d: %d file(s)", uid(r), deleted)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"deleted": deleted, "errors": errs})
}

func (a *App) handleFileRename(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	var req struct {
		Path    string `json:"path"`
		NewName string `json:"new_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	absPath, err := a.fileOpCheck(r, req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(req.NewName)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\x00") {
		http.Error(w, "invalid name", http.StatusBadRequest)
		return
	}
	newPath := filepath.Join(filepath.Dir(absPath), name)
	if newPath == absPath {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if _, err := os.Lstat(newPath); err == nil {
		http.Error(w, "target exists", http.StatusConflict)
		return
	}
	if err := os.Rename(absPath, newPath); err != nil {
		httpErr(w, err, 500)
		return
	}
	os.Rename(thumbCachePath(absPath), thumbCachePath(newPath))
	if err := a.repathFileRows(r.Context(), absPath, newPath); err != nil {
		log.Printf("file rename db update: %v", err)
	}
	log.Printf("file rename u=%d: %s -> %s", uid(r), absPath, newPath)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleFileMove(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	var req struct {
		Paths   []string `json:"paths"`
		DestDir string   `json:"dest_dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Paths) == 0 || req.DestDir == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	destDir := filepath.Clean(req.DestDir)
	if !a.isAllowedPath(r, destDir) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if info, err := os.Stat(destDir); err != nil || !info.IsDir() {
		http.Error(w, "destination is not a directory", http.StatusBadRequest)
		return
	}
	var moved int
	var errs []string
	for _, p := range req.Paths {
		absPath, err := a.fileOpCheck(r, p)
		if err != nil {
			errs = append(errs, p+": "+err.Error())
			continue
		}
		newPath := filepath.Join(destDir, filepath.Base(absPath))
		if newPath == absPath {
			moved++
			continue
		}
		if _, err := os.Lstat(newPath); err == nil {
			errs = append(errs, p+": target exists")
			continue
		}
		if err := moveFile(absPath, newPath); err != nil {
			errs = append(errs, p+": "+err.Error())
			continue
		}
		os.Rename(thumbCachePath(absPath), thumbCachePath(newPath))
		if err := a.repathFileRows(r.Context(), absPath, newPath); err != nil {
			log.Printf("file move db update: %v", err)
		}
		moved++
	}
	if moved > 0 {
		log.Printf("file move u=%d: %d file(s) -> %s", uid(r), moved, destDir)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"moved": moved, "errors": errs})
}

func (a *App) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	dir := filepath.Clean(r.URL.Query().Get("dir"))
	if !a.isAllowedPath(r, dir) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		http.Error(w, "not a directory", http.StatusBadRequest)
		return
	}
	mr, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var uploaded []string
	var errs []string
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			errs = append(errs, err.Error())
			break
		}
		if part.FormName() != "files" {
			part.Close()
			continue
		}
		name := filepath.Base(part.FileName())
		if name == "" || name == "." || name == ".." || strings.ContainsRune(name, 0) {
			part.Close()
			errs = append(errs, part.FileName()+": invalid name")
			continue
		}
		dst := filepath.Join(dir, name)
		if _, err := os.Lstat(dst); err == nil {
			part.Close()
			errs = append(errs, name+": already exists")
			continue
		}
		out, err := os.Create(dst)
		if err != nil {
			part.Close()
			errs = append(errs, name+": "+err.Error())
			continue
		}
		if _, err := io.Copy(out, part); err != nil {
			out.Close()
			part.Close()
			os.Remove(dst)
			errs = append(errs, name+": "+err.Error())
			continue
		}
		out.Close()
		part.Close()
		uploaded = append(uploaded, name)
	}
	if len(uploaded) > 0 {
		log.Printf("file upload u=%d: %d file(s) to %s", uid(r), len(uploaded), dir)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"uploaded": len(uploaded), "errors": errs})
}
