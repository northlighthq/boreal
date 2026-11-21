package sshx

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/northlighthq/boreal/internal/core/vault"
)

const closeLimit = 2 * time.Second

func TestTunnelCloseDoesNotWaitForOpenConnections(t *testing.T) {
	backend, err := net.Listen("tcp", "127.0.0.1:0")

	if err != nil {
		t.Fatal(err)
	}

	defer backend.Close()

	go func() {
		for {
			c, err := backend.Accept()

			if err != nil {
				return
			}

			defer c.Close()
		}
	}()

	l, err := net.Listen("tcp", "127.0.0.1:0")

	if err != nil {
		t.Fatal(err)
	}

	tun := &Tunnel{Rule: vault.Forward{Kind: vault.ForwardLocal}, listener: l, closed: make(chan struct{})}
	go tun.accept(nil, func() (net.Conn, error) { return net.Dial("tcp", backend.Addr().String()) })

	client, err := net.Dial("tcp", l.Addr().String())

	if err != nil {
		t.Fatal(err)
	}

	defer client.Close()

	deadline := time.Now().Add(closeLimit)

	for tun.Traffic().Conns == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if tun.Traffic().Conns != 1 {
		t.Fatal("the forwarded connection never opened")
	}

	done := make(chan struct{})

	go func() {
		var wg sync.WaitGroup

		for range 3 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				tun.Close()
			}()
		}

		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(closeLimit):
		t.Fatal("Close waited for a connection that was still open")
	}

	select {
	case <-tun.Done():
	default:
		t.Fatal("Done must be closed after Close")
	}
}
