package main

import "testing"

func TestFolderKeyFor(t *testing.T) {
	roots := []string{"/media/Music", "/media/Music/Classical"}

	// The more specific (longest) matching root wins, so a file under the
	// nested Classical root groups by composer, not by "Classical" itself.
	if got := folderKeyFor("/media/Music/Classical/Mozart/k550.mp3", roots); got != "Mozart" {
		t.Errorf("folderKeyFor = %q, want %q", got, "Mozart")
	}

	// Directly under the (non-nested) root uses the root's own name.
	if got := folderKeyFor("/media/Music/loose-track.mp3", roots); got != "Music" {
		t.Errorf("folderKeyFor = %q, want %q", got, "Music")
	}

	if got := folderKeyFor("/other/place/file.mp3", roots); got != "" {
		t.Errorf("folderKeyFor for unmatched path = %q, want empty", got)
	}
}
