package snippets

import (
	"testing"

	"github.com/northlighthq/boreal/internal/core/vault"
)

func TestPayload(t *testing.T) {
	if got := Payload(vault.Snippet{Command: "cd /tmp\r\nls -la"}); got != "cd /tmp\rls -la\r" {
		t.Fatalf("got %q", got)
	}
}

func TestLabel(t *testing.T) {
	if got := Label(vault.Snippet{Command: "uptime\nfree -h\ndf -h"}); got != "uptime (+2)" {
		t.Fatalf("got %q", got)
	}

	if got := Label(vault.Snippet{Label: "Disk", Command: "df -h"}); got != "Disk" {
		t.Fatalf("got %q", got)
	}
}
