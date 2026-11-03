package sshx

import (
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/northlighthq/boreal/internal/vault"
)

const tmuxFallback = `exec "${SHELL:-sh}" -l`

var tmuxName = strings.NewReplacer(".", "_", ":", "_")

type Shell struct {
	sess   *ssh.Session
	stdin  io.WriteCloser
	stdout io.Reader
}

func OpenShell(c *Client, cols, rows int, h vault.Host) (*Shell, error) {
	sess, err := c.NewSession()

	if err != nil {
		return nil, err
	}

	if h.ForwardAgent {
		if err := c.forwardAgent(sess); err != nil {
			sess.Close()
			return nil, err
		}
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := sess.RequestPty(termType, rows, cols, modes); err != nil {
		sess.Close()
		return nil, err
	}

	sess.Setenv("COLORTERM", "truecolor")
	sess.Setenv("TERM_PROGRAM", "Boreal")

	stdin, err := sess.StdinPipe()

	if err != nil {
		sess.Close()
		return nil, err
	}

	stdout, err := sess.StdoutPipe()

	if err != nil {
		sess.Close()
		return nil, err
	}

	sess.Stderr = io.Discard

	if err := start(sess, h); err != nil {
		sess.Close()
		return nil, err
	}

	return &Shell{sess: sess, stdin: stdin, stdout: stdout}, nil
}

func start(sess *ssh.Session, h vault.Host) error {
	if h.Tmux == "" {
		return sess.Shell()
	}

	return sess.Start(TmuxCommand(h.Tmux, h.StartupCommand))
}

func TmuxCommand(name, startup string) string {
	name = tmuxName.Replace(name)
	target := shellQuote("=" + name)
	create := "tmux new-session -d -s " + shellQuote(name)

	if startup = strings.TrimSpace(startup); startup != "" {
		create += " && tmux send-keys -t " + shellQuote("="+name+":") + " " + shellQuote(startup) + " Enter"
	}

	script := "if command -v tmux >/dev/null 2>&1; then tmux has-session -t " + target + " 2>/dev/null || { " + create + "; }; exec tmux attach-session -t " + target + "; else " + tmuxFallback + "; fi"
	return "exec sh -c " + shellQuote(script)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (s *Shell) Read(p []byte) (int, error) {
	return s.stdout.Read(p)
}

func (s *Shell) Write(p []byte) (int, error) {
	return s.stdin.Write(p)
}

func (s *Shell) Resize(cols, rows int) error {
	return s.sess.WindowChange(rows, cols)
}

func (s *Shell) Close() error {
	return s.sess.Close()
}

func (c *Client) forwardAgent(sess *ssh.Session) error {
	c.agentOnce.Do(func() {
		ag, closeAgent, err := dialAgent()

		if err != nil {
			c.agentErr = fmt.Errorf("agent forwarding: %w", err)
			return
		}

		if err := agent.ForwardToAgent(c.Client, ag); err != nil {
			closeAgent()
			c.agentErr = fmt.Errorf("agent forwarding: %w", err)
			return
		}

		c.agentClose = closeAgent
	})

	if c.agentErr != nil {
		return c.agentErr
	}

	return agent.RequestAgentForwarding(sess)
}
