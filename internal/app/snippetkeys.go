package app

import (
	"slices"
	"strings"

	"gioui.org/io/event"

	"github.com/northlighthq/boreal/internal/app/snippets"
	"github.com/northlighthq/boreal/internal/vault"
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

func (a *App) expandTrigger(word string) (string, bool) {
	s, ok := a.snippetKeys().triggers[word]

	if !ok {
		return "", false
	}

	return snippetText(s), true
}

func snippetText(s vault.Snippet) string {
	cmd := strings.ReplaceAll(s.Command, "\r\n", "\n")
	return strings.ReplaceAll(cmd, "\n", "\r")
}
