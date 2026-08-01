package main

import (
	"testing"
	"time"
)

func TestSameInstant(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 123456000, time.UTC) // .123456000s
	sameMicro := base.Add(500 * time.Nanosecond)                 // same microsecond, different ns
	diffMicro := base.Add(1 * time.Microsecond)

	if !sameInstant(base, sameMicro) {
		t.Errorf("expected same instant for a sub-microsecond difference (Postgres TIMESTAMPTZ only stores microseconds)")
	}
	if sameInstant(base, diffMicro) {
		t.Errorf("expected different instant when microseconds differ")
	}
}
