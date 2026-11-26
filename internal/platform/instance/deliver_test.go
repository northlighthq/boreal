package instance

import (
	"slices"
	"testing"
)

func TestDeliverQueuesUntilHandled(t *testing.T) {
	t.Cleanup(func() { setLocal(func([]string) {}) })

	local.handle = nil
	Deliver([]string{"ssh://a"})
	Deliver([]string{"ssh://b"})

	var got []string
	setLocal(func(args []string) { got = append(got, args...) })
	Deliver([]string{"ssh://c"})

	if want := []string{"ssh://a", "ssh://b", "ssh://c"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
