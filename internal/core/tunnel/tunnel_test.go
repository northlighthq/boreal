package tunnel

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/northlighthq/boreal/internal/core/sshx"
	"github.com/northlighthq/boreal/internal/core/vault"
)

func TestStoppingWhileConnectingStaysStopped(t *testing.T) {
	release := make(chan struct{})
	returned := make(chan struct{})
	var failed atomic.Int32

	m := New(func(ctx context.Context, r vault.Forward) (*sshx.Client, error) {
		<-release
		defer close(returned)
		return nil, errors.New("too slow")
	})
	m.OnNotify(func(string, string) {}, func(string, string) {}, func(string, string) { failed.Add(1) })

	r := vault.Forward{ID: "db", Label: "db"}
	m.Start(r)

	if st, ok := m.State(r.ID); !ok || !st.Starting {
		t.Fatalf("state after Start = %+v, %v", st, ok)
	}

	m.Stop(r.ID)
	close(release)
	<-returned
	time.Sleep(50 * time.Millisecond)

	if st, ok := m.State(r.ID); ok {
		t.Fatalf("a stopped tunnel came back: %+v", st)
	}

	if failed.Load() != 0 {
		t.Fatal("a tunnel the user stopped must not report a failure")
	}
}
