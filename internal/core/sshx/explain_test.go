package sshx

import (
	"errors"
	"fmt"
	"testing"
)

func TestExplain(t *testing.T) {
	cases := []struct {
		err   error
		title string
	}{
		{nil, "Couldn't connect"},
		{fmt.Errorf("dial: %w", ErrHostKeyRejected), "Server key rejected"},
		{errors.New("dial tcp 10.0.0.1:22: i/o timeout"), "The server didn't answer"},
		{errors.New("dial tcp 10.0.0.1:22: connect: connection refused"), "Connection refused"},
		{errors.New("dial tcp: lookup staging.example.dev: no such host"), "Can't find this server"},
		{errors.New("connect: network is unreachable"), "Network unreachable"},
		{errors.New("ssh: handshake failed: ssh: unable to authenticate"), "Sign-in failed"},
		{errors.New("read: connection reset by peer"), "The server closed the connection"},
		{errors.New("something odd"), "Couldn't connect"},
		{errors.New("dial tcp: lookup staging.example.dev on 10.0.0.53:53: i/o timeout"), "Can't find this server"},
		{errors.New("ssh: handshake failed: ssh: no authorities for hostname: x"), "Couldn't connect"},
		{errors.New("open /home/me/.ssh/id_ed25519: permission denied"), "Couldn't connect"},
		{errors.New("ssh: unable to authenticate, attempted methods [none publickey], no supported methods remain"), "Sign-in failed"},
	}

	for _, c := range cases {
		if title, hint := Explain(c.err, 2222); title != c.title || hint == "" {
			t.Errorf("Explain(%v) = %q, %q; want title %q", c.err, title, hint, c.title)
		}
	}

	if _, hint := Explain(errors.New("connection refused"), 0); hint != "Nothing is accepting connections on port 22. Is SSH running there?" {
		t.Errorf("a missing port must read as 22, got %q", hint)
	}

	if _, hint := Explain(errors.New("connection refused"), 2222); hint != "Nothing is accepting connections on port 2222. Is SSH running there?" {
		t.Errorf("refused hint = %q", hint)
	}
}
