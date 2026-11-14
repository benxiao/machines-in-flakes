package main

import (
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func binarySTL(faces int) []byte {
	b := make([]byte, 84+50*faces)
	binary.LittleEndian.PutUint32(b[80:], uint32(faces))
	return b
}

const asciiSTL = "solid x\n facet normal 0 0 1\n  outer loop\n   vertex 0 0 0\n   vertex 1 0 0\n   vertex 0 1 0\n  endloop\n endfacet\nendsolid x\n"

func TestCleanUploadName(t *testing.T) {
	for in, want := range map[string]string{
		"foo.stl":             "foo.stl",
		"FOO.STL":             "FOO.stl",
		"Master3X panel.stl":  "Master3X panel.stl",
		"../../etc/evil.stl":  "evil.stl",
		`C:\Users\me\foo.stl`: "foo.stl",
		"..foo.stl":           "foo.stl",
		"a\x00b\nc.stl":       "abc.stl",
	} {
		if got, err := cleanUploadName(in); err != nil || got != want {
			t.Errorf("cleanUploadName(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "foo", "foo.png", "foo.stl.exe", ".stl", " .stl", "../", strings.Repeat("a", 201) + ".stl"} {
		if got, err := cleanUploadName(in); err == nil {
			t.Errorf("cleanUploadName(%q) = %q; want error", in, got)
		}
	}
}

func TestUploadFileName(t *testing.T) {
	dir := t.TempDir()
	stl := filepath.Join(dir, "stl")
	os.Mkdir(stl, 0755)
	for _, f := range []string{"plain.stl", "only_v3.stl", "both.stl", "both_v2.stl", "both_v5.stl"} {
		os.WriteFile(filepath.Join(stl, f), nil, 0644)
	}
	p := scanProject(dir)
	for in, want := range map[string]string{
		"new.stl":     "new.stl",    // new part
		"new_v4.stl":  "new_v4.stl", // new part keeps its number
		"plain.stl":   "plain_v2.stl",
		"only.stl":    "only_v4.stl", // must not sort before v3 as the base file would
		"both.stl":    "both_v6.stl",
		"both_v5.stl": "both_v6.stl", // taken
		"both_v3.stl": "both_v3.stl", // free, explicit
		"both_v9.stl": "both_v9.stl",
	} {
		if got := uploadFileName(in, p); got != want {
			t.Errorf("uploadFileName(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestCheckSTL(t *testing.T) {
	solidBinary := binarySTL(2)
	copy(solidBinary, "solid exported by something")
	ok := map[string][]byte{
		"binary":          binarySTL(2),
		"binary/solid":    solidBinary,
		"binary/trailing": append(binarySTL(2), 1, 2, 3),
		"ascii":           []byte(asciiSTL),
		"ascii/bom":       []byte("\xef\xbb\xbf" + asciiSTL),
	}
	bad := map[string][]byte{
		"empty":        {},
		"short":        []byte("hello"),
		"no faces":     binarySTL(0),
		"truncated":    binarySTL(2)[:120],
		"ascii/empty":  []byte("solid x\nendsolid x\n"),
		"text":         []byte(strings.Repeat("not an stl at all, sorry\n", 20)),
		"solid prefix": append([]byte("solid"), make([]byte, 200)...),
	}
	check := func(b []byte) error { return checkSTL(b[:min(len(b), 4096)], int64(len(b))) }
	for name, b := range ok {
		if err := check(b); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for name, b := range bad {
		if check(b) == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// listDir returns every entry in dir, including hidden ones, so leftover temp files show up.
func listDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestHandleUpload(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, "proj", "stl"), 0755)
	os.Mkdir(filepath.Join(home, "notaproject"), 0755)
	a := &App{home: home} // the upload route never touches the database
	mux := http.NewServeMux()
	a.registerRoutes(mux)

	post := func(project, name string, body []byte, header ...string) *httptest.ResponseRecorder {
		t.Helper()
		q := url.Values{"project": {project}, "name": {name}}
		req := httptest.NewRequest("POST", "http://athena:10095/api/upload?"+q.Encode(), strings.NewReader(string(body)))
		if len(header) == 2 {
			req.Header.Set(header[0], header[1])
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	savedPath := func(rec *httptest.ResponseRecorder) string {
		t.Helper()
		if rec.Code != 200 {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		var out map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out["path"]
	}

	first, second := binarySTL(1), binarySTL(2)
	if got := savedPath(post("proj", "foo.stl", first)); got != "stl/foo.stl" {
		t.Errorf("first upload saved as %s", got)
	}
	if got := savedPath(post("proj", "foo.stl", second)); got != "stl/foo_v2.stl" {
		t.Errorf("second upload saved as %s", got)
	}
	if got, _ := os.ReadFile(filepath.Join(home, "proj", "stl", "foo.stl")); len(got) != len(first) {
		t.Errorf("second upload changed the first file: %d bytes", len(got))
	}
	if got := savedPath(post("proj", "../../escape.stl", first)); got != "stl/escape.stl" {
		t.Errorf("traversal name saved as %s", got)
	}

	// A new project appears, and a refused first upload leaves nothing behind.
	if got := post("fresh", "foo.stl", []byte("garbage")); got.Code != 400 {
		t.Errorf("garbage upload: status %d", got.Code)
	}
	if _, err := os.Stat(filepath.Join(home, "fresh")); !os.IsNotExist(err) {
		t.Errorf("refused upload left a project directory: %v", err)
	}
	if got := savedPath(post("fresh", "foo.stl", first)); got != "stl/foo.stl" {
		t.Errorf("new project upload saved as %s", got)
	}
	if got, ok := a.findProject("fresh"); !ok || len(got.Parts) != 1 {
		t.Errorf("new project not found by the scanner: %+v", got)
	}

	for _, project := range []string{"", ".", "..", ".ssh", "../proj", "a/b"} {
		if got := post(project, "foo.stl", first); got.Code != 400 {
			t.Errorf("project %q: status %d", project, got.Code)
		}
	}
	if got := post("notaproject", "foo.stl", first); got.Code != 409 {
		t.Errorf("existing non-project directory: status %d", got.Code)
	}
	if got := listDir(t, filepath.Join(home, "notaproject")); len(got) != 0 {
		t.Errorf("non-project directory gained %v", got)
	}
	if got := post("proj", "bar.stl", first, "Origin", "http://evil.example"); got.Code != 403 {
		t.Errorf("cross-origin upload: status %d", got.Code)
	}
	if got := post("proj", "bar.stl", first, "Origin", "http://athena:10095"); got.Code != 200 {
		t.Errorf("same-origin upload: status %d: %s", got.Code, got.Body)
	}

	// No temp files survive, whether the upload was accepted or refused.
	post("proj", "bad.stl", []byte("garbage"))
	want := []string{"bar.stl", "escape.stl", "foo.stl", "foo_v2.stl"}
	if got := listDir(t, filepath.Join(home, "proj", "stl")); !slices.Equal(got, want) {
		t.Errorf("stl/ holds %v; want %v", got, want)
	}
}

func TestHandleCreateProject(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, "proj", "stl"), 0755)
	os.WriteFile(filepath.Join(home, "proj", "stl", "foo.stl"), binarySTL(1), 0644)
	hourAgo := time.Now().Add(-time.Hour)
	os.Chtimes(filepath.Join(home, "proj", "stl", "foo.stl"), hourAgo, hourAgo)
	os.Mkdir(filepath.Join(home, "notaproject"), 0755)
	a := &App{home: home} // creating a project never touches the database
	mux := http.NewServeMux()
	a.registerRoutes(mux)
	if got, _ := json.Marshal((&App{home: t.TempDir()}).scan()); string(got) != "[]" {
		t.Errorf("a home with no projects lists %s; want []", got)
	}

	create := func(body string, header ...string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest("POST", "http://athena:10095/api/projects", strings.NewReader(body))
		if len(header) == 2 {
			req.Header.Set(header[0], header[1])
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}

	rec := create(`{"name":"fresh"}`)
	if rec.Code != 200 {
		t.Fatalf("create: status %d: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"parts":[]`) {
		t.Errorf("new project should list parts as []: %s", rec.Body)
	}
	if st, err := os.Stat(filepath.Join(home, "fresh", "stl")); err != nil || !st.IsDir() {
		t.Errorf("stl/ not created: %v", err)
	}
	// The empty project is listed, first since it is the newest, and an upload goes into it.
	names := []string{}
	for _, p := range a.scan() {
		names = append(names, p.Name)
	}
	if !slices.Equal(names, []string{"fresh", "proj"}) {
		t.Errorf("scan lists %v; want [fresh proj]", names)
	}
	req := httptest.NewRequest("POST", "http://athena:10095/api/upload?project=fresh&name=a.stl", strings.NewReader(string(binarySTL(1))))
	up := httptest.NewRecorder()
	mux.ServeHTTP(up, req)
	if up.Code != 200 {
		t.Errorf("upload into the empty project: status %d: %s", up.Code, up.Body)
	}

	for body, want := range map[string]int{
		`{"name":"fresh"}`:       409, // already a project
		`{"name":"proj"}`:        409,
		`{"name":"notaproject"}`: 409, // a directory that is not a project stays as it is
		`{"name":""}`:            400,
		`{"name":".ssh"}`:        400,
		`{"name":"../x"}`:        400,
		`{"name":"a/b"}`:         400,
		`not json`:               400,
	} {
		if got := create(body); got.Code != want {
			t.Errorf("create %s: status %d; want %d (%s)", body, got.Code, want, strings.TrimSpace(got.Body.String()))
		}
	}
	if got := listDir(t, filepath.Join(home, "notaproject")); len(got) != 0 {
		t.Errorf("non-project directory gained %v", got)
	}
	if got := create(`{"name":"evil"}`, "Origin", "http://evil.example"); got.Code != 403 {
		t.Errorf("cross-origin create: status %d", got.Code)
	}
	if _, err := os.Stat(filepath.Join(home, "evil")); !os.IsNotExist(err) {
		t.Errorf("cross-origin create made a directory: %v", err)
	}
}
