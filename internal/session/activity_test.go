package session

import (
	"testing"
	"time"
)

func TestActivity(t *testing.T) {
	start := time.Unix(1000, 0)
	var a activity

	a.hush(start, startupQuiet)
	a.note(start.Add(time.Second))

	if !a.latest().IsZero() {
		t.Fatal("output while starting should not count")
	}

	a.hush(start.Add(time.Second), echoQuiet)
	a.note(start.Add(2 * time.Second))

	if !a.latest().IsZero() {
		t.Fatal("typing must not shorten the quiet time after starting")
	}

	at := start.Add(startupQuiet)
	a.note(at)

	if !a.latest().Equal(at) {
		t.Fatalf("output after the quiet time should count, got %v", a.latest())
	}

	a.hush(at, echoQuiet)
	a.note(at.Add(echoQuiet / 2))

	if !a.latest().Equal(at) {
		t.Fatal("the echo of what was typed should not count")
	}
}
