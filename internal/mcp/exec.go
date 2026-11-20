package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/northlighthq/boreal/internal/core/sshx"
	"github.com/northlighthq/boreal/internal/core/vault"
)

const (
	toolRunCommand  = "run_command"
	toolRunSnippet  = "run_snippet"
	defaultRunTime  = 30
	maxRunTime      = 300
	maxCommandBytes = 8 << 10
	loggedCommand   = 200
)

var (
	errNoExecute    = errors.New("running commands isn't allowed. It can be allowed in " + settingsPath)
	errEmptyCommand = errors.New("give the command to run")
	errLongCommand  = fmt.Errorf("the command is longer than %d bytes", maxCommandBytes)
	errNulCommand   = errors.New("the command contains a NUL byte")
)

type RunIn struct {
	Host           string `json:"host" jsonschema:"id or label of a host from list_hosts"`
	Command        string `json:"command" jsonschema:"the command to run in the user's shell on the host, without a terminal"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty" jsonschema:"seconds before Boreal stops the command, from 1 to 300. 30 by default"`
}

type RunSnippetIn struct {
	Host           string `json:"host" jsonschema:"id or label of a host from list_hosts"`
	Snippet        string `json:"snippet" jsonschema:"id or label of a snippet from list_snippets"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty" jsonschema:"seconds before Boreal stops the command, from 1 to 300. 30 by default"`
}

type RunOut struct {
	Host      string  `json:"host"`
	ExitCode  int     `json:"exitCode"`
	Stdout    string  `json:"stdout"`
	Stderr    string  `json:"stderr"`
	Truncated bool    `json:"truncated,omitempty"`
	Seconds   float64 `json:"seconds"`
}

func runTimeout(seconds int) time.Duration {
	switch {
	case seconds <= 0:
		seconds = defaultRunTime
	case seconds > maxRunTime:
		seconds = maxRunTime
	}

	return time.Duration(seconds) * time.Second
}

func checkCommand(cmd string) error {
	switch {
	case strings.TrimSpace(cmd) == "":
		return errEmptyCommand
	case len(cmd) > maxCommandBytes:
		return errLongCommand
	case strings.ContainsRune(cmd, 0):
		return errNulCommand
	}

	return nil
}

func shortCommand(cmd string) string {
	cmd = strings.Join(strings.Fields(cmd), " ")

	if r := []rune(cmd); len(r) > loggedCommand {
		return string(r[:loggedCommand]) + "…"
	}

	return cmd
}

func (s *Server) addExec(srv *mcp.Server) {
	destructive, closed := true, false
	ann := &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &closed}

	mcp.AddTool(srv, &mcp.Tool{Name: toolRunCommand, Description: fmt.Sprintf("Run a command on a host shared over MCP and return its exit code, stdout and stderr. It runs in the user's shell without a terminal, so interactive programs won't work. Output is capped at %d KB per stream and the command is stopped after the timeout.", sshx.MaxRunOutput>>10), Annotations: ann}, s.runCommand)
	mcp.AddTool(srv, &mcp.Tool{Name: toolRunSnippet, Description: "Run a snippet saved in Boreal on a host shared over MCP. Works like run_command.", Annotations: ann}, s.runSnippet)
}

func (s *Server) recordRun(tool, host, cmd string, err error) {
	r := Record{Time: s.now(), Tool: tool, Host: host, Command: shortCommand(cmd), Outcome: OutcomeOK}

	if err != nil {
		r.Outcome, r.Error = OutcomeFail, err.Error()
	}

	s.log.Record(r)
}

func (s *Server) execute(ctx context.Context, host, cmd string, seconds int) (RunOut, error) {
	if err := checkCommand(cmd); err != nil {
		return RunOut{}, err
	}

	v, err := s.load(tierExecute)

	if err != nil {
		return RunOut{}, err
	}

	h, err := findHost(v, host)

	if err != nil {
		return RunOut{}, err
	}

	r, err := v.Run(ctx, h, cmd, runTimeout(seconds))
	out := RunOut{Host: h.Title(), ExitCode: r.ExitCode, Stdout: r.Stdout, Stderr: r.Stderr, Truncated: r.Truncated, Seconds: r.Duration.Seconds()}
	return out, err
}

func (s *Server) runCommand(ctx context.Context, _ *mcp.CallToolRequest, in RunIn) (*mcp.CallToolResult, RunOut, error) {
	out, err := s.execute(ctx, in.Host, in.Command, in.TimeoutSeconds)
	s.recordRun(toolRunCommand, in.Host, in.Command, err)
	return nil, out, err
}

func findSnippet(d vault.Data, ref string) (vault.Snippet, error) {
	ref = strings.TrimSpace(ref)
	var found []vault.Snippet

	for _, sn := range d.Snippets {
		if sn.ID == ref {
			return sn, nil
		}

		if strings.EqualFold(sn.Label, ref) {
			found = append(found, sn)
		}
	}

	switch len(found) {
	case 0:
		return vault.Snippet{}, fmt.Errorf("there's no snippet %q in Boreal", ref)
	case 1:
		return found[0], nil
	}

	return vault.Snippet{}, fmt.Errorf("several snippets are called %q; use the id from list_snippets", ref)
}

func (s *Server) runSnippet(ctx context.Context, _ *mcp.CallToolRequest, in RunSnippetIn) (*mcp.CallToolResult, RunOut, error) {
	var out RunOut
	var cmd string
	v, err := s.load(tierExecute)

	if err == nil {
		var sn vault.Snippet

		if sn, err = findSnippet(v.Data(), in.Snippet); err == nil {
			cmd = sn.Command
			out, err = s.execute(ctx, in.Host, cmd, in.TimeoutSeconds)
		}
	}

	s.recordRun(toolRunSnippet, in.Host, cmd, err)
	return nil, out, err
}
