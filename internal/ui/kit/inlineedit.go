package kit

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/ui/icons"
)

type InlineEdit struct {
	editor  widget.Editor
	focused bool
	had     bool
	cancel  widget.Clickable
	inside  bool
	seen    int
	blurred bool
}

type EditResult int

const (
	Editing EditResult = iota
	EditCommit
	EditCancel
)

func (e *InlineEdit) Start(k *Kit, text string) {
	*e = InlineEdit{}
	e.editor.SingleLine = true
	e.editor.Submit = true
	e.editor.SetText(text)
	e.editor.SetCaret(len([]rune(text)), 0)
	e.seen = k.Presses()
}

func (e *InlineEdit) Text() string { return e.editor.Text() }

func (e *InlineEdit) Blurred() bool { return e.blurred }

func (e *InlineEdit) Layout(gtx C, k *Kit, hint string) (D, EditResult) {
	th := k.Th

	for {
		ev, ok := e.editor.Update(gtx)

		if !ok {
			break
		}

		if _, ok := ev.(widget.SubmitEvent); ok {
			e.blurred = false
			return D{}, EditCommit
		}
	}

	for {
		ev, ok := gtx.Event(key.Filter{Focus: &e.editor, Name: key.NameEscape})

		if !ok {
			break
		}

		if ev, ok := ev.(key.Event); ok && ev.State == key.Press {
			return D{}, EditCancel
		}
	}

	if e.cancel.Clicked(gtx) {
		return D{}, EditCancel
	}

	if !e.focused {
		gtx.Execute(key.FocusCmd{Tag: &e.editor})
		e.focused = true
	}

	if gtx.Focused(&e.editor) {
		e.had = true
	} else if e.had && !e.cancel.Pressed() {
		e.blurred = true
		return D{}, EditCommit
	}

	h := gtx.Dp(24)
	size := image.Pt(gtx.Constraints.Max.X, h)

	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &e.inside, Kinds: pointer.Press}); !ok {
			break
		}

		e.seen = k.Presses()
	}

	if k.Presses() != e.seen {
		e.blurred = true
		return D{}, EditCommit
	}

	pass := pointer.PassOp{}.Push(gtx.Ops)
	in := clip.Rect{Max: size}.Push(gtx.Ops)
	event.Op(gtx.Ops, &e.inside)
	in.Pop()
	pass.Pop()

	gtx.Constraints = layout.Exact(size)
	r := image.Rectangle{Max: size}
	RRect(gtx, r, gtx.Dp(th.RadiusSmall), th.Field)
	Border(gtx, r, gtx.Dp(th.RadiusSmall), 1, th.TextFaint)
	return layout.Inset{Left: 6, Right: 2}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				gtx.Constraints.Min.Y = h
				return layout.W.Layout(gtx, func(gtx C) D {
					return k.Editor(gtx, &e.editor, hint, false)
				})
			}),
			layout.Rigid(func(gtx C) D {
				return k.IconButtonSized(gtx, &e.cancel, icons.Close, false, 20, 12, th.TextMuted)
			}),
		)
	}), Editing
}

func (e *InlineEdit) Editor() *widget.Editor {
	return &e.editor
}
