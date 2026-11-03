package forwards

import (
	"testing"

	"github.com/northlighthq/boreal/internal/vault"
)

func TestGroup(t *testing.T) {
	rules := []vault.Forward{
		{ID: "a", Profile: "staging"},
		{ID: "b"},
		{ID: "c", Profile: "Databases"},
		{ID: "d", Profile: "staging"},
	}

	var got []string

	for _, it := range group(rules) {
		if it.rule == nil {
			got = append(got, "#"+it.profile)
			continue
		}

		got = append(got, it.rule.ID)
	}

	want := []string{"b", "#Databases", "c", "#staging", "a", "d"}

	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}
