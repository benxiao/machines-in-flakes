package main

import "testing"

func TestClassifyExt(t *testing.T) {
	cases := map[string]string{
		".jpg": "photo", ".JPEG": "photo", ".png": "photo",
		".mp4": "video", ".MKV": "video",
		".mp3": "audio", ".flac": "audio",
		".pdf": "pdf",
		".zip": "archive", ".rar": "archive",
		".txt": "text", ".md": "text",
		".exe": "other", "": "other",
	}
	for ext, want := range cases {
		if got := classifyExt(ext); got != want {
			t.Errorf("classifyExt(%q) = %q, want %q", ext, got, want)
		}
	}
}

func TestFormatSize(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{2684354560, "2.5 GB"},
	}
	for _, c := range cases {
		if got := formatSize(c.n); got != c.want {
			t.Errorf("formatSize(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestUnderRoot(t *testing.T) {
	cases := []struct {
		path, root string
		want       bool
	}{
		{"/media", "/media", true},
		{"/media/movies/a.mp4", "/media", true},
		{"/media2/a.mp4", "/media", false}, // prefix collision without a separator
		{"/other/a.mp4", "/media", false},
	}
	for _, c := range cases {
		if got := underRoot(c.path, c.root); got != c.want {
			t.Errorf("underRoot(%q, %q) = %v, want %v", c.path, c.root, got, c.want)
		}
	}
}
