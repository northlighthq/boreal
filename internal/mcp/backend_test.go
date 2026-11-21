package mcp

import (
	"testing"

	"github.com/northlighthq/boreal/internal/core/sshx"
)

func TestHostKeysAreNeverAccepted(t *testing.T) {
	for _, changed := range []bool{false, true} {
		var k keyCheck

		if k.reject(sshx.HostKeyInfo{Changed: changed}) {
			t.Fatalf("a host key was accepted (changed=%v)", changed)
		}
	}
}

func TestChangedHostKeyIsReportedAsChanged(t *testing.T) {
	var k keyCheck

	if k.err() != errNewHostKey {
		t.Fatalf("unknown key: %v", k.err())
	}

	k.reject(sshx.HostKeyInfo{Changed: true})
	k.reject(sshx.HostKeyInfo{})

	if k.err() != errHostKeyMoved {
		t.Fatalf("changed key: %v", k.err())
	}
}
