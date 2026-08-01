package main

import "testing"

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
