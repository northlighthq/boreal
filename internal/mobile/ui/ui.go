package ui

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/common/kit"
	"github.com/northlighthq/boreal/internal/core/config"
	"github.com/northlighthq/boreal/internal/core/vault"
	"github.com/northlighthq/boreal/internal/mobile/nav"
)

type (
	C = layout.Context
	D = layout.Dimensions
	W = layout.Widget
)

const (
	TextBody    = unit.Sp(15)
	TextSub     = unit.Sp(13)
	TextCaption = unit.Sp(12)
	TextBar     = unit.Sp(17)
	TextLarge   = unit.Sp(24)

	Gutter        = unit.Dp(16)
	MinTouch      = unit.Dp(48)
	RowHeight     = unit.Dp(60)
	FieldHeight   = unit.Dp(46)
	ButtonHeight  = unit.Dp(46)
	LeadSize      = unit.Dp(18)
	LeadWidth     = unit.Dp(22)
	SectionGap    = unit.Dp(24)
	EmptyMaxWidth = unit.Dp(300)
)

type Host interface {
	Kit() *kit.Kit
	Vault() *vault.Vault
	Settings() config.Settings
	UpdateSettings(s config.Settings)
	Push(s nav.Screen)
	Pop()
	ShowSheet(s *Sheet)
	Ask(d *Dialog)
	Notify(title, message string)
	Invalidate()
	Connect(h vault.Host)
}

type Action struct {
	Icon    *icons.Icon
	Click   *widget.Clickable
	Do      func()
	Primary bool
}

type Actionable interface {
	Actions() []Action
}

type FullScreen interface {
	FullScreen() bool
}

type Subtitled interface {
	Subtitle() string
}
