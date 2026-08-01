package main

import "testing"

func TestSearchTermConds(t *testing.T) {
	var args []any
	conds := searchTermConds("Pink Floyd", &args)
	if len(conds) != 2 {
		t.Fatalf("expected 2 conditions for 2 terms, got %d: %v", len(conds), conds)
	}
	if len(args) != 2 || args[0] != "%pink%" || args[1] != "%floyd%" {
		t.Errorf("expected lowercased wildcard args, got %v", args)
	}

	args = nil
	conds = searchTermConds("one two three four five six seven", &args)
	if len(conds) != 5 || len(args) != 5 {
		t.Errorf("expected terms capped at 5, got %d conditions / %d args", len(conds), len(args))
	}
}
