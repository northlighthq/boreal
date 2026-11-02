package sshx

import (
	"errors"
	"io"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/northlighthq/boreal/internal/vault"
)

type pipeAddr string

func (a pipeAddr) Network() string { return "pipe" }
func (a pipeAddr) String() string  { return string(a) }

// cmdConn is the standard input and output of a ProxyCommand used as the
// connection to the server.
type cmdConn struct {
	io.Reader
	io.WriteCloser
	cmd *exec.Cmd
}

func (c *cmdConn) Close() error {
	c.WriteCloser.Close()

	if c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}

	c.cmd.Wait()
	return nil
}

func (c *cmdConn) LocalAddr() net.Addr              { return pipeAddr("proxy") }
func (c *cmdConn) RemoteAddr() net.Addr             { return pipeAddr(c.cmd.Path) }
func (c *cmdConn) SetDeadline(time.Time) error      { return nil }
func (c *cmdConn) SetReadDeadline(time.Time) error  { return nil }
func (c *cmdConn) SetWriteDeadline(time.Time) error { return nil }
func (c *cmdConn) Write(p []byte) (int, error)      { return c.WriteCloser.Write(p) }
func (c *cmdConn) Read(p []byte) (int, error)       { return c.Reader.Read(p) }

var errEmptyProxy = errors.New("the proxy command is empty")

func proxyConn(h vault.Host) (net.Conn, error) {
	line := ProxyCommandLine(h)

	if strings.TrimSpace(line) == "" {
		return nil, errEmptyProxy
	}

	cmd := shellCommand(line)
	stdin, err := cmd.StdinPipe()

	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()

	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &cmdConn{Reader: stdout, WriteCloser: stdin, cmd: cmd}, nil
}

// ProxyCommandLine fills in the %h, %p, %r and %n tokens of the host's proxy
// command, as OpenSSH does.
func ProxyCommandLine(h vault.Host) string {
	vars := map[byte]string{
		'h': h.Address,
		'p': strconv.Itoa(port(h)),
		'r': username(h),
		'n': h.Title(),
	}

	var b strings.Builder
	s := h.ProxyCommand

	for i := 0; i < len(s); i++ {
		if s[i] != '%' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}

		i++

		if s[i] == '%' {
			b.WriteByte('%')
			continue
		}

		if v, ok := vars[s[i]]; ok {
			b.WriteString(v)
			continue
		}

		b.WriteByte('%')
		b.WriteByte(s[i])
	}

	return b.String()
}
