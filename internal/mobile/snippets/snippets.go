package snippets

import (
	"strconv"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/common/icons"
	"github.com/northlighthq/boreal/internal/core/vault"
	"github.com/northlighthq/boreal/internal/mobile/ui"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type List struct {
	h     ui.Host
	list  widget.List
	rows  map[string]*widget.Clickable
	add   widget.Clickable
	empty widget.Clickable
}

func NewList(h ui.Host) *List {
	return &List{h: h, rows: map[string]*widget.Clickable{}}
}

func (l *List) Title() string { return "Snippets" }

func (l *List) Layout(gtx C) D {
	k := l.h.Kit()
	all := l.h.Vault().Snapshot().Snippets

	if l.add.Clicked(gtx) || l.empty.Clicked(gtx) {
		l.h.Push(NewEditor(l.h, vault.Snippet{}))
	}

	for _, s := range all {
		if c := l.rows[s.ID]; c != nil && c.Clicked(gtx) {
			l.h.Push(NewEditor(l.h, s))
		}
	}

	if len(all) == 0 {
		return ui.Empty(gtx, k, icons.Snippet, "No snippets",
			"Save commands you run often and send them to a session from the terminal menu.",
			func(gtx C) D { return ui.Button(gtx, k, &l.empty, ui.Primary, icons.Add, "Add snippet", true) })
	}

	rows := make([]ui.W, len(all))

	for i, s := range all {
		clk := l.rows[s.ID]

		if clk == nil {
			clk = new(widget.Clickable)
			l.rows[s.ID] = clk
		}

		rows[i] = func(gtx C) D {
			return ui.Row(gtx, k, clk, ui.RowOpts{Title: Label(s), Subtitle: firstLine(s.Command), Mono: true, Chevron: true})
		}
	}

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return ui.Page(gtx, &l.list, func(gtx C) D { return ui.Section(gtx, k, "", rows...) })
		}),
		layout.Expanded(func(gtx C) D { return ui.FAB(gtx, k, &l.add, icons.Add, "New snippet") }),
	)
}

type Editor struct {
	h       ui.Host
	snippet vault.Snippet
	label   widget.Editor
	command widget.Editor
	save    widget.Clickable
	saveBar widget.Clickable
	remove  widget.Clickable
	list    widget.List
	tried   bool
}

func NewEditor(h ui.Host, s vault.Snippet) *Editor {
	e := &Editor{h: h, snippet: s}
	ui.SetupField(&e.label, key.HintText, false)
	e.command.InputHint = key.HintPassword
	e.label.SetText(s.Label)
	e.command.SetText(s.Command)
	return e
}

func (e *Editor) Title() string {
	if e.snippet.ID == "" {
		return "New snippet"
	}

	return "Edit snippet"
}

func (e *Editor) Actions() []ui.Action {
	return []ui.Action{{Icon: icons.Check, Click: &e.save, Do: e.submit}}
}

func (e *Editor) Layout(gtx C) D {
	k := e.h.Kit()

	if e.saveBar.Clicked(gtx) {
		e.submit()
	}

	if e.remove.Clicked(gtx) {
		e.confirmDelete()
	}

	empty := e.tried && strings.TrimSpace(e.command.Text()) == ""
	help := "Each line is sent as you'd type it, followed by Enter."

	if empty {
		help = "Write the command to send."
	}

	children := []ui.W{
		func(gtx C) D { return ui.Field(gtx, k, &e.label, ui.FieldOpts{Label: "Name", Hint: "Disk usage"}) },
		func(gtx C) D {
			return ui.Field(gtx, k, &e.command, ui.FieldOpts{Label: "Command", Hint: "df -h", Mono: true, Invalid: empty, Help: help})
		},
		func(gtx C) D { return ui.Button(gtx, k, &e.saveBar, ui.Primary, nil, "Save snippet", true) },
	}

	if e.snippet.ID != "" {
		children = append(children, func(gtx C) D {
			return ui.Button(gtx, k, &e.remove, ui.Destructive, icons.Delete, "Delete snippet", true)
		})
	}

	return ui.Page(gtx, &e.list, children...)
}

func (e *Editor) submit() {
	e.tried = true
	cmd := strings.TrimRight(e.command.Text(), "\n ")

	if strings.TrimSpace(cmd) == "" {
		return
	}

	s := e.snippet
	s.Label = strings.TrimSpace(e.label.Text())
	s.Command = cmd

	if _, err := e.h.Vault().SaveSnippet(s); err != nil {
		e.h.Notify("Couldn't save the snippet", err.Error())
		return
	}

	e.h.Pop()
}

func (e *Editor) confirmDelete() {
	e.h.Ask(&ui.Dialog{
		Title:   "Delete " + Label(e.snippet) + "?",
		Confirm: "Delete",
		Cancel:  "Cancel",
		Danger:  true,
		Reply: func(ok bool, _ string) {
			if !ok {
				return
			}

			if err := e.h.Vault().DeleteSnippet(e.snippet.ID); err != nil {
				e.h.Notify("Couldn't delete the snippet", err.Error())
				return
			}

			e.h.Pop()
		},
	})
}

func Label(s vault.Snippet) string {
	if s.Label != "" {
		return s.Label
	}

	return firstLine(s.Command)
}

func Payload(s vault.Snippet) string {
	lines := strings.Split(strings.ReplaceAll(s.Command, "\r\n", "\n"), "\n")
	return strings.Join(lines, "\r") + "\r"
}

func firstLine(s string) string {
	line, rest, _ := strings.Cut(s, "\n")

	if rest != "" {
		n := strings.Count(rest, "\n") + 1
		return line + " (+" + strconv.Itoa(n) + ")"
	}

	return line
}
