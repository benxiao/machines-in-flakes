package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// ---- Folder operations ----

// escapeLikeArg escapes PostgreSQL LIKE wildcards so folder paths with '_' or '%'
// match only themselves. Pair with ESCAPE '\' in the SQL clause.
func escapeLikeArg(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func (a *App) purgeFolderRows(ctx context.Context, dir string) error {
	likeDir := escapeLikeArg(dir)
	tx, err := a.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// LIKE-based deletes: children of dir
	for _, q := range []string{
		`DELETE FROM file_index WHERE path LIKE $1 || '/%' ESCAPE '\'`,
		`DELETE FROM video_positions WHERE path LIKE $1 || '/%' ESCAPE '\'`,
		`DELETE FROM playlist_items WHERE path LIKE $1 || '/%' ESCAPE '\'`,
		`DELETE FROM favorites WHERE path LIKE $1 || '/%' ESCAPE '\' AND is_folder = FALSE`,
		`DELETE FROM indexed_paths WHERE path LIKE $1 || '/%' ESCAPE '\'`,
		`DELETE FROM file_hashes WHERE path LIKE $1 || '/%' ESCAPE '\'`,
	} {
		if _, err := tx.Exec(ctx, q, likeDir); err != nil {
			return err
		}
	}
	// Exact-match deletes: the dir itself
	for _, q := range []string{
		`DELETE FROM favorites WHERE path = $1 AND is_folder = TRUE`,
		`DELETE FROM indexed_paths WHERE path = $1`,
	} {
		if _, err := tx.Exec(ctx, q, dir); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (a *App) repathFolderRows(ctx context.Context, oldDir, newDir string) error {
	likeOld := escapeLikeArg(oldDir)
	tx, err := a.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// $1=likeOld for LIKE matching, $2=oldDir for LENGTH (unescaped), $3=newDir for prefix
	steps := []struct {
		q    string
		args []any
	}{
		{`UPDATE file_index
		  SET path = $3 || SUBSTRING(path FROM LENGTH($2)+1),
		      dir_path = $3 || SUBSTRING(dir_path FROM LENGTH($2)+1)
		  WHERE path LIKE $1 || '/%' ESCAPE '\'`, []any{likeOld, oldDir, newDir}},
		{`UPDATE video_positions
		  SET path = $3 || SUBSTRING(path FROM LENGTH($2)+1)
		  WHERE path LIKE $1 || '/%' ESCAPE '\'`, []any{likeOld, oldDir, newDir}},
		{`UPDATE playlist_items
		  SET path = $3 || SUBSTRING(path FROM LENGTH($2)+1)
		  WHERE path LIKE $1 || '/%' ESCAPE '\'`, []any{likeOld, oldDir, newDir}},
		{`UPDATE favorites
		  SET path = $3 || SUBSTRING(path FROM LENGTH($2)+1)
		  WHERE path LIKE $1 || '/%' ESCAPE '\' AND is_folder = FALSE`, []any{likeOld, oldDir, newDir}},
		{`UPDATE indexed_paths
		  SET path = $3 || SUBSTRING(path FROM LENGTH($2)+1)
		  WHERE path LIKE $1 || '/%' ESCAPE '\'`, []any{likeOld, oldDir, newDir}},
		{`UPDATE file_hashes
		  SET path = $3 || SUBSTRING(path FROM LENGTH($2)+1)
		  WHERE path LIKE $1 || '/%' ESCAPE '\'`, []any{likeOld, oldDir, newDir}},
		// Exact-match updates use unescaped oldDir
		{`UPDATE favorites SET path = $2 WHERE path = $1 AND is_folder = TRUE`, []any{oldDir, newDir}},
		{`UPDATE indexed_paths SET path = $2 WHERE path = $1`, []any{oldDir, newDir}},
	}
	for _, s := range steps {
		if _, err := tx.Exec(ctx, s.q, s.args...); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func copyDirRecursive(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_EXCL, info.Mode().Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			os.Remove(target)
			return err
		}
		if err := out.Sync(); err != nil {
			out.Close()
			os.Remove(target)
			return err
		}
		return out.Close()
	})
}

func moveDirAny(src, dst string) error {
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}
	var lerr *os.LinkError
	if !errors.As(err, &lerr) || !errors.Is(lerr.Err, syscall.EXDEV) {
		return err
	}
	if err := copyDirRecursive(src, dst); err != nil {
		os.RemoveAll(dst)
		return err
	}
	return os.RemoveAll(src)
}

func (a *App) handleFolderDownload(w http.ResponseWriter, r *http.Request) {
	path := filepath.Clean(r.URL.Query().Get("path"))
	if !a.isAllowedPath(r, path) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		http.Error(w, "not a directory", http.StatusBadRequest)
		return
	}
	name := strings.ReplaceAll(filepath.Base(path), `"`, `'`)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, name))
	zw := zip.NewWriter(w)
	defer zw.Close()
	filepath.Walk(path, func(fpath string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(path, fpath)
		// Media files are already compressed; Deflate would just burn CPU
		// and stall the download.
		fw, err := zw.CreateHeader(&zip.FileHeader{Name: rel, Method: zip.Store, Modified: fi.ModTime()})
		if err != nil {
			log.Printf("folder download zip create %s: %v", rel, err)
			return nil
		}
		f, err := os.Open(fpath)
		if err != nil {
			log.Printf("folder download open %s: %v", fpath, err)
			return nil
		}
		defer f.Close()
		if _, err = io.Copy(fw, f); err != nil {
			log.Printf("folder download copy %s: %v", fpath, err)
		}
		return nil
	})
}

func (a *App) handleFolderDelete(w http.ResponseWriter, r *http.Request) {
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
		absPath := filepath.Clean(p)
		if !a.isAllowedPath(r, absPath) {
			errs = append(errs, p+": forbidden")
			continue
		}
		info, err := os.Stat(absPath)
		if err != nil || !info.IsDir() {
			errs = append(errs, p+": not a directory")
			continue
		}
		if _, err := a.moveToTrash(r, absPath, true); err != nil {
			errs = append(errs, p+": "+err.Error())
			continue
		}
		deleted++
	}
	if deleted > 0 {
		log.Printf("folder delete (to trash) u=%d: %d dir(s)", uid(r), deleted)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"deleted": deleted, "errors": errs})
}

func (a *App) handleFolderCreate(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	var req struct {
		Dir  string `json:"dir"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	dir := filepath.Clean(req.Dir)
	if !a.isAllowedPath(r, dir) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		http.Error(w, "not a directory", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\x00") {
		http.Error(w, "invalid name", http.StatusBadRequest)
		return
	}
	newPath := filepath.Join(dir, name)
	if _, err := os.Lstat(newPath); err == nil {
		http.Error(w, "target exists", http.StatusConflict)
		return
	}
	if err := os.Mkdir(newPath, 0755); err != nil {
		httpErr(w, err, 500)
		return
	}
	log.Printf("folder create u=%d: %s", uid(r), newPath)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleFolderRename(w http.ResponseWriter, r *http.Request) {
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
	absPath := filepath.Clean(req.Path)
	if !a.isAllowedPath(r, absPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	info, err := os.Stat(absPath)
	if err != nil || !info.IsDir() {
		http.Error(w, "not a directory", http.StatusBadRequest)
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
	if err := a.repathFolderRows(r.Context(), absPath, newPath); err != nil {
		log.Printf("folder rename db update: %v", err)
	}
	log.Printf("folder rename u=%d: %s -> %s", uid(r), absPath, newPath)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleFolderMove(w http.ResponseWriter, r *http.Request) {
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
		absPath := filepath.Clean(p)
		if !a.isAllowedPath(r, absPath) {
			errs = append(errs, p+": forbidden")
			continue
		}
		info, err := os.Stat(absPath)
		if err != nil || !info.IsDir() {
			errs = append(errs, p+": not a directory")
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
		if err := moveDirAny(absPath, newPath); err != nil {
			errs = append(errs, p+": "+err.Error())
			continue
		}
		if err := a.repathFolderRows(r.Context(), absPath, newPath); err != nil {
			log.Printf("folder move db update: %v", err)
		}
		moved++
	}
	if moved > 0 {
		log.Printf("folder move u=%d: %d dir(s) -> %s", uid(r), moved, destDir)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"moved": moved, "errors": errs})
}
