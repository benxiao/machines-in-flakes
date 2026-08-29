package main

import (
	"testing"
	"time"
)

func TestBuildBreadcrumb(t *testing.T) {
	root := "/media/Movies"

	crumbs := buildBreadcrumb(root, root)
	if len(crumbs) != 1 || !crumbs[0].Current || crumbs[0].Path != root {
		t.Fatalf("breadcrumb at root = %+v, want a single current crumb", crumbs)
	}

	crumbs = buildBreadcrumb("/media/Movies/Action/2020", root)
	wantPaths := []string{"/media/Movies", "/media/Movies/Action", "/media/Movies/Action/2020"}
	if len(crumbs) != len(wantPaths) {
		t.Fatalf("breadcrumb depth = %d, want %d: %+v", len(crumbs), len(wantPaths), crumbs)
	}
	for i, c := range crumbs {
		if c.Path != wantPaths[i] {
			t.Errorf("crumb[%d].Path = %q, want %q", i, c.Path, wantPaths[i])
		}
		if c.Current != (i == len(crumbs)-1) {
			t.Errorf("crumb[%d].Current = %v, want %v", i, c.Current, i == len(crumbs)-1)
		}
	}
}

func TestChooseLandingPath(t *testing.T) {
	sidebar := []PathRow{{Path: "/media/Movies"}, {Path: "/media/Music"}}

	if got := chooseLandingPath("/media/Music/Jazz", true, sidebar); got != "/media/Music/Jazz" {
		t.Errorf("valid last-opened folder should win, got %q", got)
	}
	if got := chooseLandingPath("/media/Music/Jazz", false, sidebar); got != "/media/Movies" {
		t.Errorf("invalid last-opened folder should fall back to first sidebar path, got %q", got)
	}
	if got := chooseLandingPath("", false, sidebar); got != "/media/Movies" {
		t.Errorf("no last-opened folder should fall back to first sidebar path, got %q", got)
	}
	if got := chooseLandingPath("", false, nil); got != "" {
		t.Errorf("no last-opened folder and no sidebar paths should return empty, got %q", got)
	}
}

func TestOpenedBefore(t *testing.T) {
	var zero time.Time
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	if !openedBefore(newer, zero, older, zero) {
		t.Error("more recently opened entry should sort first")
	}
	if openedBefore(older, zero, newer, zero) {
		t.Error("less recently opened entry should not sort first")
	}
	if !openedBefore(older, zero, zero, zero) {
		t.Error("any opened entry should sort before a never-opened one")
	}
	if openedBefore(zero, zero, older, zero) {
		t.Error("a never-opened entry should not sort before an opened one")
	}
	// Both never opened: falls back to newest-modified first.
	if !openedBefore(zero, newer, zero, older) {
		t.Error("among never-opened entries, the newer-modified one should sort first")
	}
}

func TestSortEntriesOpenedDefault(t *testing.T) {
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	newestMod := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	subdirs := []SubdirRow{
		{Name: "b-never-opened-newer-mtime", ModTime: newestMod},
		{Name: "c-opened-recent", LastOpened: newer},
		{Name: "a-opened-older", LastOpened: older},
	}
	files := []FileRow{
		{Filename: "z.mp4", LastOpened: older},
		{Filename: "a.mp4", LastOpened: newer},
	}
	sortEntries(subdirs, files, "opened")

	wantSubdirs := []string{"c-opened-recent", "a-opened-older", "b-never-opened-newer-mtime"}
	for i, want := range wantSubdirs {
		if subdirs[i].Name != want {
			t.Errorf("subdirs[%d] = %q, want %q", i, subdirs[i].Name, want)
		}
	}
	if files[0].Filename != "a.mp4" || files[1].Filename != "z.mp4" {
		t.Errorf("files order = [%s, %s], want [a.mp4, z.mp4]", files[0].Filename, files[1].Filename)
	}
}
