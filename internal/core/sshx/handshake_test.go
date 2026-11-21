package sshx

import (
	"context"
	"errors"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/northlighthq/boreal/internal/core/vault"
)

func TestDialGivesUpOnAServerThatNeverAnswers(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")

	if err != nil {
		t.Fatal(err)
	}

	defer l.Close()
	held := make(chan net.Conn, 1)

	go func() {
		c, err := l.Accept()

		if err == nil {
			held <- c
		}
	}()

	_, port, _ := net.SplitHostPort(l.Addr().String())
	p, _ := strconv.Atoi(port)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = Dial(ctx, Target{Host: vault.Host{Address: "127.0.0.1", Port: p, Username: "x", Agent: AgentNone}}, nil, Hooks{}, 0)

	if err == nil {
		t.Fatal("a silent server must not let the handshake succeed")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want the deadline", err)
	}

	if time.Since(start) > 3*time.Second {
		t.Fatalf("Dial took %v, the deadline wasn't honoured", time.Since(start))
	}

	select {
	case c := <-held:
		c.Close()
	default:
	}
}
