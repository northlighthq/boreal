package sshx

import (
	"fmt"
	"io"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type Shell struct {
	sess   *ssh.Session
	stdin  io.WriteCloser
	stdout io.Reader
}

func OpenShell(c *Client, cols, rows int, forwardAgent bool) (*Shell, error) {
	sess, err := c.NewSession()

	if err != nil {
		return nil, err
	}

	if forwardAgent {
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

	if err := sess.Shell(); err != nil {
		sess.Close()
		return nil, err
	}

	return &Shell{sess: sess, stdin: stdin, stdout: stdout}, nil
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
