package main

import "testing"

func TestEscapeLikeArg(t *testing.T) {
	cases := []struct{ in, want string }{
		{`plain`, `plain`},
		{`50%_off`, `50\%\_off`},
		{`a\b`, `a\\b`},
		{`100%`, `100\%`},
		{`under_score`, `under\_score`},
		{`back\slash_and%percent`, `back\\slash\_and\%percent`},
	}
	for _, c := range cases {
		if got := escapeLikeArg(c.in); got != c.want {
			t.Errorf("escapeLikeArg(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
