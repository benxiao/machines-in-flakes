package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFmtDurStr(t *testing.T) {
	cases := []struct {
		sec  int64
		want string
	}{
		{0, "0s"},
		{45, "45s"},
		{60, "1m"},
		{90, "1m"},
		{3600, "1h 0m"},
		{3661, "1h 1m"},
		{7325, "2h 2m"},
	}
	for _, c := range cases {
		if got := fmtDurStr(c.sec); got != c.want {
			t.Errorf("fmtDurStr(%d) = %q, want %q", c.sec, got, c.want)
		}
	}
}

// TestLoginPageRenders is a template-parsing/execution regression check, not
// a DB test: initTemplates parses every page template once at startup, and
// a syntax mistake in any of them (e.g. the googleLoginURL funcMap wiring)
// would only otherwise surface the first time someone loads that specific
// page in a browser.
func TestLoginPageRenders(t *testing.T) {
	initTemplates()

	w := httptest.NewRecorder()
	render(w, "login", LoginPage{Next: "/browse?dir=/media"})
	if w.Code != 0 && w.Code != 200 {
		t.Fatalf("render status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `href="/auth/google/login?next=%2Fbrowse%3Fdir%3D%2Fmedia"`) {
		t.Errorf("expected escaped googleLoginURL href in rendered login page, got:\n%s", body)
	}
	if !strings.Contains(body, "Sign in with Google") {
		t.Errorf("expected Google sign-in button text in rendered login page")
	}

	w2 := httptest.NewRecorder()
	render(w2, "login", LoginPage{Error: "Invalid username or password."})
	if !strings.Contains(w2.Body.String(), "Invalid username or password.") {
		t.Errorf("expected error message in rendered login page")
	}
	if strings.Contains(w2.Body.String(), "?next=") {
		t.Errorf("expected no next= query param when Next is empty, got:\n%s", w2.Body.String())
	}
}
