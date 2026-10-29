package dialogs

import (
	"image"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/ui/icons"
	"github.com/northlighthq/boreal/internal/ui/kit"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

const LabelWidth = unit.Dp(110)

func SingleLine(eds ...*widget.Editor) {
	for _, e := range eds {
		e.SingleLine = true
		e.Submit = true
	}
}

type Confirm struct {
	k                      *kit.Kit
	title, message, action string
	onOK                   func()
	dlg                    kit.Dialog
	ok, cancel             widget.Clickable
}

func NewConfirm(k *kit.Kit, title, message, action string, onOK func()) *Confirm {
	return &Confirm{k: k, title: title, message: message, action: action, onOK: onOK}
}

func (d *Confirm) Layout(gtx C) bool {
	k := d.k

	if d.ok.Clicked(gtx) {
		d.onOK()
		return true
	}

	if d.cancel.Clicked(gtx) {
		return true
	}

	return k.Dialog(gtx, &d.dlg, kit.DialogOpts{
		Title: d.title,
		Width: 420,
		Body: func(gtx C) D {
			return k.Label(gtx, kit.TextStyle{Color: k.Th.TextMuted, MaxLines: 6}, d.message)
		},
		Footer: kit.Buttons(
			func(gtx C) D { return k.Button(gtx, &d.cancel, kit.Ghost, nil, "Cancel") },
			func(gtx C) D { return k.Button(gtx, &d.ok, kit.Danger, nil, d.action) },
		),
	})
}

type Field struct {
	Label  string
	Hint   string
	Mono   bool
	Multi  bool
	Secret bool
	Editor widget.Editor
	reveal widget.Clickable
	shown  bool
}

func NewField(label, hint, value string) *Field {
	f := &Field{Label: label, Hint: hint}
	f.Editor.SetText(value)
	return f
}

type Form struct {
	k        *kit.Kit
	title    string
	Subtitle string
	action   string
	Width    unit.Dp
	fields   []*Field
	Extra    func(gtx C) D
	OnOK     func(values []string) bool
	dlg      kit.Dialog
	ok       widget.Clickable
	cancel   widget.Clickable
	focused  bool
}

func NewForm(k *kit.Kit, title, action string, fields ...*Field) *Form {
	for _, f := range fields {
		f.Editor.SingleLine = !f.Multi
		f.Editor.Submit = !f.Multi

		if f.Secret {
			f.Editor.Mask = '•'
		}
	}

	return &Form{k: k, title: title, action: action, fields: fields, Width: 480}
}

func (d *Form) Dialog() *kit.Dialog {
	return &d.dlg
}

func (d *Form) Values() []string {
	out := make([]string, len(d.fields))

	for i, f := range d.fields {
		out[i] = f.Editor.Text()
	}

	return out
}

func (d *Form) Layout(gtx C) bool {
	k := d.k

	submit := d.ok.Clicked(gtx)

	for _, f := range d.fields {
		if f.reveal.Clicked(gtx) {
			f.shown = !f.shown
			f.Editor.Mask = '•'

			if f.shown {
				f.Editor.Mask = 0
			}
		}
	}

	for _, f := range d.fields {
		for {
			ev, ok := f.Editor.Update(gtx)

			if !ok {
				break
			}

			if _, ok := ev.(widget.SubmitEvent); ok {
				submit = true
			}
		}
	}

	if submit && d.OnOK(d.Values()) {
		return true
	}

	if d.cancel.Clicked(gtx) {
		return true
	}

	if !d.focused && len(d.fields) > 0 {
		gtx.Execute(key.FocusCmd{Tag: &d.fields[0].Editor})
		d.focused = true
	}

	return k.Dialog(gtx, &d.dlg, kit.DialogOpts{
		Title:    d.title,
		Subtitle: d.Subtitle,
		Width:    d.Width,
		Body: func(gtx C) D {
			var children []layout.FlexChild

			for _, f := range d.fields {
				children = append(children, layout.Rigid(func(gtx C) D {
					return layout.Inset{Bottom: 16}.Layout(gtx, func(gtx C) D {
						if f.Multi {
							return d.multiField(gtx, f)
						}

						o := kit.FieldOpts{Hint: f.Hint, Mono: f.Mono}

						if f.Secret {
							ic := icons.Visibility

							if f.shown {
								ic = icons.VisibilityOff
							}

							o.Trailing = func(gtx C) D { return k.IconButtonSized(gtx, &f.reveal, ic, false, 22, 16, k.Th.TextMuted) }
						}

						return k.LabeledField(gtx, f.Label, &f.Editor, o)
					})
				}))
			}

			if d.Extra != nil {
				children = append(children, layout.Rigid(d.Extra))
			}

			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		},
		Footer: kit.Buttons(
			func(gtx C) D { return k.Button(gtx, &d.cancel, kit.Ghost, nil, "Cancel") },
			func(gtx C) D { return k.Button(gtx, &d.ok, kit.Primary, nil, d.action) },
		),
	})
}

func (d *Form) multiField(gtx C, f *Field) D {
	k := d.k
	th := k.Th

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 4}.Layout(gtx, func(gtx C) D {
				return k.Label(gtx, kit.TextStyle{Size: th.TextSizeSmall, Color: th.TextMuted}, f.Label)
			})
		}),
		layout.Rigid(func(gtx C) D {
			h := gtx.Dp(110)
			size := image.Pt(gtx.Constraints.Max.X, h)
			r := image.Rectangle{Max: size}
			kit.RRect(gtx, r, gtx.Dp(th.Radius), th.Field)
			line := th.FieldLine

			if gtx.Focused(&f.Editor) {
				line = th.TextFaint
			}

			kit.Border(gtx, r, gtx.Dp(th.Radius), 1, line)

			gtx.Constraints = layout.Exact(size)
			layout.UniformInset(8).Layout(gtx, func(gtx C) D {
				return layout.NW.Layout(gtx, func(gtx C) D {
					return k.Editor(gtx, &f.Editor, f.Hint, true)
				})
			})
			return D{Size: size}
		}),
	)
}
