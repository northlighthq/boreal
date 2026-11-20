package mcp

import (
	"strings"
	"testing"
	"time"

	"github.com/northlighthq/boreal/internal/core/vault"
)

var withExecute = vault.MCPPolicy{Enabled: true, Read: true, Execute: true, Folders: []string{"Production"}}

func TestRunNeedsTheExecuteSwitch(t *testing.T) {
	f := setup(t, vault.MCPPolicy{Enabled: true, Read: true, Access: true, AllHosts: true})

	calls := map[string]map[string]any{
		toolRunCommand: {"host": "web", "command": "uptime"},
		toolRunSnippet: {"host": "web", "snippet": "Disk"},
	}

	for tool, args := range calls {
		text, isErr := f.call(t, tool, args)

		if !isErr || !strings.Contains(text, "running commands isn't allowed") {
			t.Fatalf("%s without Execute: %v %q", tool, isErr, text)
		}
	}

	if len(f.view.runs) != 0 {
		t.Fatal("a command ran without the Execute permission")
	}
}

func TestRunOnlyReachesSharedHosts(t *testing.T) {
	f := setup(t, withExecute)

	if text, isErr := f.call(t, toolRunCommand, map[string]any{"host": "db-01", "command": "uptime"}); !isErr || !strings.Contains(text, "no host") {
		t.Fatalf("unshared host: %v %q", isErr, text)
	}

	if len(f.view.runs) != 0 {
		t.Fatal("a command ran on an unshared host")
	}
}

func TestRunCommandAndSnippet(t *testing.T) {
	f := setup(t, withExecute)

	text, isErr := f.call(t, toolRunCommand, map[string]any{"host": "web-01", "command": "uptime", "timeoutSeconds": 5})

	if isErr || !strings.Contains(text, `"stdout":"ran: uptime"`) || !strings.Contains(text, `"exitCode":0`) {
		t.Fatalf("run_command = %v %q", isErr, text)
	}

	if r := f.view.runs[0]; r.host != "web" || r.timeout != 5*time.Second {
		t.Fatalf("run = %+v", r)
	}

	if _, isErr := f.call(t, toolRunSnippet, map[string]any{"host": "web", "snippet": "disk"}); isErr || f.view.runs[1].cmd != "df -h" {
		t.Fatalf("run_snippet ran %+v", f.view.runs)
	}

	f.call(t, toolRunCommand, map[string]any{"host": "web", "command": "uptime", "timeoutSeconds": 99999})

	if r := f.view.runs[2]; r.timeout != maxRunTime*time.Second {
		t.Fatalf("timeout = %v, want it capped", r.timeout)
	}
}

func TestRunRejectsBadCommandsBeforeConnecting(t *testing.T) {
	f := setup(t, withExecute)

	for _, cmd := range []string{"", "   ", strings.Repeat("a", maxCommandBytes+1), "ls\x00rm -rf /"} {
		if _, isErr := f.call(t, toolRunCommand, map[string]any{"host": "web", "command": cmd}); !isErr {
			t.Fatalf("command %q was accepted", cmd)
		}
	}

	if len(f.view.runs) != 0 {
		t.Fatal("a rejected command reached the host")
	}
}

func TestRunIsLoggedWithTheCommand(t *testing.T) {
	f := setup(t, withExecute)
	f.call(t, toolRunCommand, map[string]any{"host": "web", "command": "systemctl   restart nginx"})

	last := f.log.records[len(f.log.records)-1]

	if last.Tool != toolRunCommand || last.Host != "web" || last.Command != "systemctl restart nginx" || last.Outcome != OutcomeOK {
		t.Fatalf("record = %+v", last)
	}
}
