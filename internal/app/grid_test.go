package app

import "testing"

func leaves(n *node, out []*Pane) []*Pane {
	if n.pane != nil {
		return append(out, n.pane)
	}

	return leaves(n.b, leaves(n.a, out))
}

func TestGridKeepsOrderAndBalance(t *testing.T) {
	for count := 1; count <= maxSnippetHosts; count++ {
		panes := make([]*Pane, count)

		for i := range panes {
			panes[i] = &Pane{}
		}

		root := grid(panes, true)
		got := leaves(root, nil)

		if len(got) != count {
			t.Fatalf("%d panes: got %d leaves", count, len(got))
		}

		for i := range got {
			if got[i] != panes[i] {
				t.Fatalf("%d panes: order changed at %d", count, i)
			}
		}

		if count > 1 && !root.vertical {
			t.Fatalf("%d panes: the first split should be side by side", count)
		}
	}
}
