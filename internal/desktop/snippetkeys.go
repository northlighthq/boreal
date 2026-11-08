package desktop

import (
	"slices"

	"gioui.org/io/event"

	"github.com/northlighthq/boreal/internal/core/placeholder"
	"github.com/northlighthq/boreal/internal/core/vault"
	"github.com/northlighthq/boreal/internal/desktop/snippets"
)

type snippetKeys struct {
	rev      uint64
	built    bool
	filters  []event.Filter
	bindings map[string]vault.Snippet
	triggers map[string]vault.Snippet
}

func (a *App) snippetKeys() *snippetKeys {
	k := &a.snipKeys
	rev := a.vault.Revision()

	if k.built && k.rev == rev {
		return k
	}

	k.rev, k.built = rev, true
	k.filters = slices.Clone(shortcuts)
	k.bindings = map[string]vault.Snippet{}
	k.triggers = map[string]vault.Snippet{}

	if a.vault.Locked() {
		return k
	}

	for _, s := range a.vault.Snapshot().Snippets {
		if s.Trigger != "" {
			k.triggers[s.Trigger] = s
		}

		if s.Shortcut == "" {
			continue
		}

		b, err := snippets.ParseShortcut(s.Shortcut)

		if err != nil {
			continue
		}

		k.bindings[b.String()] = s
		k.filters = append(k.filters, b.Filter())
	}

	return k
}

func (a *App) runSnippetKey(name string) bool {
	s, ok := a.snippetKeys().bindings[name]

	if ok {
		a.RunSnippet(s, false)
	}

	return ok
}

func (a *App) expandTrigger(p *Pane, word string) (string, bool) {
	s, ok := a.snippetKeys().triggers[word]

	if !ok {
		return "", false
	}

	if len(placeholder.Asks(s.Command)) == 0 {
		return terminalText(placeholder.Fill(s.Command, a.paneValues(p), nil)), true
	}

	a.withAnswers(s, func(answers map[string]string) {
		p.view.SendText(terminalText(placeholder.Fill(s.Command, a.paneValues(p), answers)))
		p.view.Focus()
	})
	return "", true
}
