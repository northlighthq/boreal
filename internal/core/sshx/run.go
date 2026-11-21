package sshx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	MaxRunOutput = 64 << 10
	exitUnknown  = -1
	drainWait    = 2 * time.Second
)

var ErrRunTimeout = errors.New("the command didn't finish in time, so Boreal stopped it")

type RunResult struct {
	ExitCode  int           `json:"exitCode"`
	Stdout    string        `json:"stdout"`
	Stderr    string        `json:"stderr"`
	Truncated bool          `json:"truncated,omitempty"`
	Duration  time.Duration `json:"-"`
}

type capped struct {
	buf   bytes.Buffer
	limit int
	cut   bool
}

func (c *capped) Write(p []byte) (int, error) {
	room := c.limit - c.buf.Len()

	switch {
	case room <= 0:
		c.cut = c.cut || len(p) > 0
	case len(p) > room:
		c.buf.Write(p[:room])
		c.cut = true
	default:
		c.buf.Write(p)
	}

	return len(p), nil
}

func (c *capped) text() string {
	return strings.ToValidUTF8(c.buf.String(), "�")
}

func (c *Client) Run(ctx context.Context, cmd string, limit int) (RunResult, error) {
	if limit <= 0 || limit > MaxRunOutput {
		limit = MaxRunOutput
	}

	sess, err := c.NewSession()

	if err != nil {
		return RunResult{}, err
	}

	defer sess.Close()
	stdout, stderr := &capped{limit: limit}, &capped{limit: limit}
	sess.Stdout, sess.Stderr = stdout, stderr
	start := time.Now()

	if err := sess.Start(cmd); err != nil {
		return RunResult{}, err
	}

	done := make(chan error, 1)
	go func() { done <- sess.Wait() }()

	var waitErr error

	select {
	case waitErr = <-done:
	case <-ctx.Done():
		sess.Signal(ssh.SIGKILL)
		sess.Close()

		select {
		case <-done:
		case <-time.After(drainWait):
			return RunResult{ExitCode: exitUnknown, Duration: time.Since(start)}, fmt.Errorf("%w: %w", ErrRunTimeout, ctx.Err())
		}

		return result(stdout, stderr, exitUnknown, start), fmt.Errorf("%w: %w", ErrRunTimeout, ctx.Err())
	}

	var exitErr *ssh.ExitError
	var missing *ssh.ExitMissingError

	switch {
	case waitErr == nil:
		return result(stdout, stderr, 0, start), nil
	case errors.As(waitErr, &exitErr):
		return result(stdout, stderr, exitErr.ExitStatus(), start), nil
	case errors.As(waitErr, &missing):
		return result(stdout, stderr, exitUnknown, start), nil
	}

	return result(stdout, stderr, exitUnknown, start), waitErr
}

func result(stdout, stderr *capped, code int, start time.Time) RunResult {
	return RunResult{
		ExitCode:  code,
		Stdout:    stdout.text(),
		Stderr:    stderr.text(),
		Truncated: stdout.cut || stderr.cut,
		Duration:  time.Since(start),
	}
}
