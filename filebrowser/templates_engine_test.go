package main

import "testing"

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
