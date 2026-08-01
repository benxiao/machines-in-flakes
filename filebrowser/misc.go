package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// handleClientLog lets the mobile player beacon diagnostic events into the
// server journal, since background playback failures on the phone are
// otherwise unobservable (no devtools with the screen off).
func (a *App) handleClientLog(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(io.LimitReader(r.Body, 2048))
	// Strip control characters (newlines, ANSI escapes) so a client can't
	// forge journal lines or inject terminal sequences.
	msg := strings.Map(func(c rune) rune {
		if c < 0x20 || c == 0x7f {
			return ' '
		}
		return c
	}, string(b))
	log.Printf("[client u=%d] %s", uid(r), msg)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handlePDFMarkdown(w http.ResponseWriter, r *http.Request) {
	absPath := filepath.Clean(r.URL.Query().Get("path"))
	if !a.isAllowedPath(r, absPath) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if strings.ToLower(filepath.Ext(absPath)) != ".pdf" {
		http.Error(w, "not a PDF", http.StatusBadRequest)
		return
	}
	srcPath, err := a.resolveMediaPath(r.Context(), absPath)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	if info, err := os.Stat(srcPath); err != nil || info.IsDir() {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	mdBin := a.markitdownPath
	if mdBin == "" {
		mdBin = "markitdown"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, mdBin, srcPath).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			http.Error(w, "markitdown failed: "+string(exitErr.Stderr), http.StatusInternalServerError)
		} else {
			http.Error(w, "markitdown error: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(out)
}
