package core

import (
	"image/color"

	"github.com/northlighthq/boreal/internal/config"
	"github.com/northlighthq/boreal/internal/ui/icons"
)

// HostIcon picks the icon for a host. A zero color means monochrome: the caller
// keeps its own, and kit.IconTint resolves both cases.
func HostIcon(s config.Settings, osName string) (*icons.Icon, color.NRGBA) {
	if s.HostIcons == config.HostIconsTerminal {
		return icons.Terminal, color.NRGBA{}
	}

	b, ok := icons.OS(osName)

	if !ok {
		return icons.Terminal, color.NRGBA{}
	}

	if s.HostIconTint == config.HostIconTintMono {
		return b.Icon, color.NRGBA{}
	}

	return b.Icon, b.Color
}
