package sshx

import (
	"io"
	"net"
	"sync/atomic"
	"testing"
)

func TestPipeCounts(t *testing.T) {
	in, client := net.Pipe()
	out, server := net.Pipe()
	var sent, received atomic.Int64
	done := make(chan struct{})

	go func() {
		pipe(in, out, &sent, &received)
		in.Close()
		out.Close()
		close(done)
	}()

	go func() {
		buf := make([]byte, 5)
		io.ReadFull(server, buf)
		server.Write([]byte("hello world"))
		server.Close()
	}()

	client.Write([]byte("abcde"))
	reply, _ := io.ReadAll(client)
	<-done

	if sent.Load() != 5 || received.Load() != int64(len(reply)) || len(reply) != 11 {
		t.Fatalf("sent %d, received %d, reply %q", sent.Load(), received.Load(), reply)
	}
}
