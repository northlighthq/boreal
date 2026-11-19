package ui

import (
	"testing"
	"time"
)

func TestAgo(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		since time.Duration
		want  string
	}{
		{30 * time.Second, "now"},
		{5 * time.Minute, "5m"},
		{2 * time.Hour, "2h"},
		{3 * day, "3d"},
		{15 * day, "2w"},
		{70 * day, "2mo"},
		{800 * day, "2y"},
	}

	for _, c := range cases {
		if got := Ago(now.Add(-c.since), now); got != c.want {
			t.Errorf("Ago(-%v) = %q, want %q", c.since, got, c.want)
		}
	}

	if got := Ago(time.Time{}, now); got != "" {
		t.Errorf("zero time = %q, want empty", got)
	}
}
