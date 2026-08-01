package main

import (
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"
)

func TestArchiveEntryName(t *testing.T) {
	cases := map[string]string{
		"./foo/bar.txt": "foo/bar.txt",
		"foo/bar.txt":   "foo/bar.txt",
		"/foo/bar.txt":  "foo/bar.txt",
		"foo/":          "foo",
		"../escape.txt": "../escape.txt", // traversal names must NOT be normalized away
	}
	for in, want := range cases {
		if got := archiveEntryName(in); got != want {
			t.Errorf("archiveEntryName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeArchiveName(t *testing.T) {
	if got := normalizeArchiveName("plain-ascii.txt"); got != "plain-ascii.txt" {
		t.Errorf("expected a valid UTF-8 name to pass through unchanged, got %q", got)
	}
	// Bytes that aren't valid UTF-8 must still come out as valid UTF-8 either
	// way (a successful GBK/Shift-JIS decode or the ToValidUTF8 fallback) --
	// round-trip safety matters more than a correct encoding guess.
	got := normalizeArchiveName(string([]byte{0xff, 0xfe, 0xfd}))
	if !utf8.ValidString(got) {
		t.Errorf("expected fallback to produce valid UTF-8, got %q", got)
	}
}

func TestSplitArchivePath(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "photos.zip")
	if err := os.WriteFile(zipPath, []byte("stand-in for a real zip; splitArchivePath only checks it's a regular file"), 0644); err != nil {
		t.Fatal(err)
	}

	archiveFile, inner, ok := splitArchivePath(filepath.Join(dir, "photos.zip", "sub", "a.jpg"))
	if !ok || archiveFile != zipPath || inner != "sub/a.jpg" {
		t.Errorf("splitArchivePath into archive = (%q, %q, %v), want (%q, %q, true)", archiveFile, inner, ok, zipPath, "sub/a.jpg")
	}

	archiveFile, inner, ok = splitArchivePath(zipPath)
	if !ok || archiveFile != zipPath || inner != "" {
		t.Errorf("splitArchivePath at archive root = (%q, %q, %v), want (%q, \"\", true)", archiveFile, inner, ok, zipPath)
	}

	if _, _, ok := splitArchivePath(filepath.Join(dir, "not-an-archive", "a.jpg")); ok {
		t.Errorf("expected no archive match when no ancestor is a real archive file")
	}
}
