package core

import (
	"image/color"
	"testing"

	"github.com/northlighthq/boreal/internal/config"
	"github.com/northlighthq/boreal/internal/sshx"
	"github.com/northlighthq/boreal/internal/ui/icons"
)

func TestEveryDetectedOSHasAnIcon(t *testing.T) {
	for _, name := range sshx.OSNames() {
		if _, ok := icons.OS(name); !ok {
			t.Errorf("no icon for %q", name)
		}
	}
}

func TestHostIcon(t *testing.T) {
	s := config.Defaults()
	brand, _ := icons.OS("debian")

	if ic, c := HostIcon(s, "debian"); ic != brand.Icon || c != brand.Color {
		t.Fatal("defaults should show the system logo in color")
	}

	if ic, c := HostIcon(s, ""); ic != icons.Terminal || c != (color.NRGBA{}) {
		t.Fatal("a host with no detected system should keep the terminal icon")
	}

	s.HostIconTint = config.HostIconTintMono

	if ic, c := HostIcon(s, "debian"); ic != brand.Icon || c != (color.NRGBA{}) {
		t.Fatal("monochrome should keep the logo without its color")
	}

	s.HostIcons = config.HostIconsTerminal

	if ic, _ := HostIcon(s, "debian"); ic != icons.Terminal {
		t.Fatal("the terminal mode should ignore the system")
	}
}
