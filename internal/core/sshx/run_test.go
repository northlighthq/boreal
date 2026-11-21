package sshx

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func exitStatus(code uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, code)
	return b
}

func serveExec(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer ch.Close()

	for req := range reqs {
		switch req.Type {
		case "exec":
			var payload struct{ Command string }
			ssh.Unmarshal(req.Payload, &payload)
			req.Reply(true, nil)

			switch payload.Command {
			case "echo hi":
				ch.Write([]byte("hi\n"))
				ch.SendRequest("exit-status", false, exitStatus(0))
				return
			case "fail":
				ch.Stderr().Write([]byte("boom\n"))
				ch.SendRequest("exit-status", false, exitStatus(3))
				return
			case "big":
				ch.Write([]byte(strings.Repeat("x", 3*MaxRunOutput)))
				ch.SendRequest("exit-status", false, exitStatus(0))
				return
			case "hang":
			}
		case "signal":
			return
		default:
			req.Reply(false, nil)
		}
	}
}

func testClient(t *testing.T) *Client {
	t.Helper()
	_, hostPriv, _ := ed25519.GenerateKey(rand.Reader)
	hostKey, err := ssh.NewSignerFromKey(hostPriv)

	if err != nil {
		t.Fatal(err)
	}

	cfg := &ssh.ServerConfig{NoClientAuth: true}
	cfg.AddHostKey(hostKey)
	l, err := net.Listen("tcp", "127.0.0.1:0")

	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { l.Close() })

	go func() {
		for {
			conn, err := l.Accept()

			if err != nil {
				return
			}

			go func() {
				_, chans, reqs, err := ssh.NewServerConn(conn, cfg)

				if err != nil {
					return
				}

				go ssh.DiscardRequests(reqs)

				for nc := range chans {
					ch, creqs, err := nc.Accept()

					if err == nil {
						go serveExec(ch, creqs)
					}
				}
			}()
		}
	}()

	conn, err := net.Dial("tcp", l.Addr().String())

	if err != nil {
		t.Fatal(err)
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, l.Addr().String(), &ssh.ClientConfig{User: "x", HostKeyCallback: ssh.InsecureIgnoreHostKey()})

	if err != nil {
		t.Fatal(err)
	}

	client := &Client{Client: ssh.NewClient(c, chans, reqs), stop: make(chan struct{})}
	t.Cleanup(func() { client.Close() })
	return client
}

func TestRunReturnsOutputAndExitCode(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()

	r, err := c.Run(ctx, "echo hi", 0)

	if err != nil || r.ExitCode != 0 || r.Stdout != "hi\n" || r.Truncated {
		t.Fatalf("echo = %+v, %v", r, err)
	}

	r, err = c.Run(ctx, "fail", 0)

	if err != nil || r.ExitCode != 3 || r.Stderr != "boom\n" {
		t.Fatalf("fail = %+v, %v", r, err)
	}
}

func TestRunCapsTheOutput(t *testing.T) {
	r, err := testClient(t).Run(context.Background(), "big", 1000)

	if err != nil || len(r.Stdout) != 1000 || !r.Truncated {
		t.Fatalf("big = %d bytes, truncated=%v, %v", len(r.Stdout), r.Truncated, err)
	}
}

func TestRunStopsACommandThatRunsTooLong(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()

	_, err := testClient(t).Run(ctx, "hang", 0)

	if !errors.Is(err, ErrRunTimeout) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want a timeout", err)
	}

	if time.Since(start) > 3*time.Second {
		t.Fatalf("took %v", time.Since(start))
	}
}
