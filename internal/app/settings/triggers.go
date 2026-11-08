package settings

import (
	"image"
	"slices"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/app/core"
	"github.com/northlighthq/boreal/internal/app/dialogs"
	"github.com/northlighthq/boreal/internal/platform/sound"
	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
	"github.com/northlighthq/boreal/internal/vault"
)

const (
	triggerScopeWeight = 1.4
	scopeAll           = "All sessions"
	scopeLocal         = "Local terminals"
	scopeRemoved       = "Removed host"
	noHighlight        = "No color"
)

var triggerColorLabels = []string{noHighlight, "Red", "Yellow", "Green", "Blue", "Magenta", "Cyan"}

type triggerRow struct {
	id      string
	pattern widget.Editor
	scope   kit.Select
	color   kit.Select
	notify  widget.Bool
	sound   widget.Bool
	remove  widget.Clickable
}

type triggers struct {
	rows   []*triggerRow
	add    widget.Clickable
	scopes []string
	labels []string
	saved  []vault.Trigger
}

func (tr *triggers) load(a core.Host) {
	d := a.Vault().Snapshot()
	tr.saved = d.Triggers
	tr.scopes = []string{"", vault.TriggerLocal}
	tr.labels = []string{scopeAll, scopeLocal}

	for _, h := range d.Hosts {
		tr.scopes = append(tr.scopes, h.ID)
		tr.labels = append(tr.labels, h.Title())
	}

	for _, t := range d.Triggers {
		if !slices.Contains(tr.scopes, t.HostID) {
			tr.scopes = append(tr.scopes, t.HostID)
			tr.labels = append(tr.labels, scopeRemoved)
		}
	}

	tr.rows = tr.rows[:0]

	for _, t := range d.Triggers {
		tr.rows = append(tr.rows, tr.newRow(t))
	}
}

func (tr *triggers) newRow(t vault.Trigger) *triggerRow {
	r := &triggerRow{id: t.ID}
	dialogs.SingleLine(&r.pattern)
	r.pattern.SetText(t.Pattern)
	r.scope.Value = max(slices.Index(tr.scopes, t.HostID), 0)
	r.color.Value = slices.Index(vault.TriggerColors, t.Color) + 1
	r.notify.Value = t.Notify
	r.sound.Value = t.Sound
	return r
}

func (tr *triggers) collect() []vault.Trigger {
	out := make([]vault.Trigger, 0, len(tr.rows))

	for _, r := range tr.rows {
		t := vault.Trigger{
			ID:      r.id,
			Pattern: strings.TrimSpace(r.pattern.Text()),
			HostID:  tr.scopes[r.scope.Value],
			Notify:  r.notify.Value,
			Sound:   r.sound.Value,
		}

		if r.color.Value > 0 {
			t.Color = vault.TriggerColors[r.color.Value-1]
		}

		if t.Pattern == "" {
			continue
		}

		out = append(out, t)
	}

	return out
}

func (tr *triggers) changed() bool {
	return !slices.Equal(tr.collect(), tr.saved)
}

func (tr *triggers) update(gtx C) {
	if tr.add.Clicked(gtx) {
		r := tr.newRow(vault.Trigger{ID: vault.NewID(), Color: vault.TriggerRed, Notify: true})
		tr.rows = append(tr.rows, r)
		gtx.Execute(key.FocusCmd{Tag: &r.pattern})
	}

	tr.rows = slices.DeleteFunc(tr.rows, func(r *triggerRow) bool { return r.remove.Clicked(gtx) })
}

func (d *Dialog) triggersPage(a core.Host, gtx C) []layout.FlexChild {
	k := a.Kit()
	th := k.Th
	tr := &d.trig
	tr.update(gtx)

	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 18}.Layout(gtx, func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted, MaxLines: 3},
					"Highlight, notify or play a sound when a line of terminal output matches. Patterns are regular expressions and ignore case.")
			})
		}),
	}

	for _, r := range tr.rows {
		children = append(children, layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 12}.Layout(gtx, func(gtx C) D { return tr.row(a, gtx, r) })
		}))
	}

	return append(children, layout.Rigid(func(gtx C) D {
		return layout.W.Layout(gtx, func(gtx C) D {
			return k.Button(gtx, &tr.add, kit.Secondary, icons.Add, "Add trigger")
		})
	}))
}

func (tr *triggers) row(a core.Host, gtx C, r *triggerRow) D {
	th := a.Kit().Th
	m := op.Record(gtx.Ops)
	dims := layout.UniformInset(12).Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return tr.rowContent(a, gtx, r)
	})
	call := m.Stop()
	kit.Border(gtx, image.Rectangle{Max: dims.Size}, gtx.Dp(th.Radius), 1, th.FieldLine)
	call.Add(gtx.Ops)
	return dims
}

func (tr *triggers) rowContent(a core.Host, gtx C, r *triggerRow) D {
	k := a.Kit()
	th := k.Th
	text := strings.TrimSpace(r.pattern.Text())
	_, err := vault.Trigger{Pattern: text}.Compile()
	invalid := err != nil && text != ""

	sel := func(s *kit.Select, labels []string) layout.Widget {
		return func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return k.Select(gtx, s, labels)
		}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D {
					return k.Field(gtx, &r.pattern, kit.FieldOpts{Hint: "error|failed|password:", Mono: true, Invalid: invalid})
				}),
				layout.Rigid(kit.Gap(8)),
				layout.Rigid(func(gtx C) D {
					return k.IconButtonSized(gtx, &r.remove, icons.Delete, false, 30, 16, th.TextMuted)
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			if !invalid {
				return D{}
			}

			return layout.Inset{Top: 6}.Layout(gtx, func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.Danger, MaxLines: 2}, "This isn't a valid regular expression.")
			})
		}),
		layout.Rigid(kit.Gap(10)),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(triggerScopeWeight, sel(&r.scope, tr.labels)),
				layout.Rigid(kit.Gap(8)),
				layout.Flexed(1, sel(&r.color, triggerColorLabels)),
				layout.Rigid(kit.Gap(16)),
				layout.Rigid(func(gtx C) D { return k.Checkbox(gtx, &r.notify, "Notify") }),
				layout.Rigid(func(gtx C) D {
					if !sound.Available {
						return D{}
					}

					return layout.Inset{Left: 16}.Layout(gtx, func(gtx C) D { return k.Checkbox(gtx, &r.sound, "Sound") })
				}),
			)
		}),
	)
}
