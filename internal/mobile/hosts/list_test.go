package hosts

import (
	"testing"

	"github.com/northlighthq/boreal/internal/core/vault"
)

func TestGroupAndFilter(t *testing.T) {
	all := []vault.Host{
		{ID: "1", Label: "web", Address: "10.0.0.1", Group: "Prod"},
		{ID: "2", Label: "db", Address: "10.0.0.2", Group: "prod-db"},
		{ID: "3", Label: "box", Address: "home.lan"},
		{ID: "4", Label: "api", Address: "10.0.0.3", Group: "Prod"},
	}

	groups := group(all)

	if len(groups) != 3 || groups[0].name != "Prod" || groups[2].name != ungrouped {
		t.Fatalf("groups out of order: %+v", groups)
	}

	if groups[0].hosts[0].Label != "api" {
		t.Fatalf("hosts inside a group sort by name, got %q", groups[0].hosts[0].Label)
	}

	if got := filter(all, "prod"); len(got) != 3 {
		t.Fatalf("filter by folder should match 3, got %d", len(got))
	}

	if got := filter(all, "home"); len(got) != 1 || got[0].ID != "3" {
		t.Fatalf("filter by address failed: %+v", got)
	}
}

func TestTarget(t *testing.T) {
	if got := Target(vault.Host{Address: "srv", Username: "root", Port: 22}); got != "root@srv" {
		t.Fatalf("got %q", got)
	}

	if got := Target(vault.Host{Address: "srv", Username: "me", Port: 2222}); got != "me@srv:2222" {
		t.Fatalf("got %q", got)
	}
}
