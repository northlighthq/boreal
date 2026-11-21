package mcp

import (
	"strconv"
	"testing"
	"time"
)

func TestRecentReadsTheEndOfABigLog(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	log, err := NewFileLog()

	if err != nil {
		t.Fatal(err)
	}

	for i := range 3000 {
		log.Record(Record{Time: time.Now(), Tool: "tool-" + strconv.Itoa(i), Outcome: OutcomeOK})
	}

	got := Recent(3)

	if len(got) != 3 || got[0].Tool != "tool-2997" || got[2].Tool != "tool-2999" {
		t.Fatalf("recent = %+v", got)
	}
}
