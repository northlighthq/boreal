package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/plattyssh/pty"
	"github.com/plattyssh/vt"

	"github.com/plattyssh/platty/internal/sshx"
	"github.com/plattyssh/platty/internal/vault"
)

const (
	readBufferSize  = 64 * 1024
	writeQueueSize  = 256
	initialSizeWait = 500 * time.Millisecond
)

type Kind uint8

const (
	Local Kind = iota
	SSH
)

type State uint8

const (
	Connecting State = iota
	Connected
	Closed
	Failed
)

const (
	styleInfo    = "\x1b[2m"
	styleError   = "\x1b[31m"
	styleSuccess = "\x1b[1m"
	styleReset   = "\x1b[0m"
)

type Transport interface {
	io.ReadWriteCloser
	Resize(cols, rows int) error
}

type Options struct {
	Kind      Kind
	Command   []string
	Target    sshx.Target
	Vault     *vault.Vault
	Hooks     sshx.Hooks
	Redact    func(string) string
	KeepAlive time.Duration

	Cols, Rows int
	Scrollback int
	Title      string
	OnChange   func()

	OnClipboard func(text string)
}

type Session struct {
	ID     string
	Kind   Kind
	HostID string

	opts Options
	term *vt.Terminal

	mu        sync.Mutex
	state     State
	err       error
	title     string
	transport Transport
	client    *sshx.Client
	gen       int
	started   time.Time

	writes    chan []byte
	done      chan struct{}
	sized     chan struct{}
	sizedOnce sync.Once

	steps []Step

	logMu  sync.Mutex
	log    *textLog
	mirror func(p []byte)
}

type Step struct {
	At    time.Time
	Text  string
	Error bool
}

func (s *Session) Steps() []Step {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Step(nil), s.steps...)
}

func (s *Session) step(text string, isErr bool) {
	s.mu.Lock()
	s.steps = append(s.steps, Step{At: time.Now(), Text: text, Error: isErr})
	s.mu.Unlock()
}

func New(o Options) *Session {
	if o.OnChange == nil {
		o.OnChange = func() {}
	}

	s := &Session{
		ID:     vault.NewID(),
		Kind:   o.Kind,
		HostID: o.Target.Host.ID,
		opts:   o,
		title:  o.Title,
		writes: make(chan []byte, writeQueueSize),
		done:   make(chan struct{}),
		sized:  make(chan struct{}),
	}

	s.term = vt.New(max(o.Cols, 1), max(o.Rows, 1),
		vt.WithReply(s.SendOnly),
		vt.WithClipboard(func(text string) {
			if o.OnClipboard != nil {
				o.OnClipboard(text)
			}
		}),
		vt.WithScrollback(o.Scrollback),
	)

	go s.writeLoop()
	go func() {
		select {
		case <-s.sized:
		case <-time.After(initialSizeWait):
		case <-s.done:
			return
		}

		s.connect()
	}()
	return s
}

func (s *Session) Terminal() *vt.Terminal {
	return s.term
}

func (s *Session) State() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.state, s.err
}

func (s *Session) Title() string {
	t := s.term.Title()

	if lt := strings.ToLower(t); t == "" || strings.HasSuffix(lt, ".exe") || strings.Contains(lt, ".exe - ") {
		return s.title
	}

	return t
}

func (s *Session) Client() *sshx.Client {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != Connected {
		return nil
	}

	return s.client
}

func (s *Session) Target() sshx.Target {
	return s.opts.Target
}

func (s *Session) Uptime() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != Connected {
		return 0
	}

	return time.Since(s.started)
}

func (s *Session) Send(p []byte) {
	if len(p) == 0 {
		return
	}

	if m := s.mirrorFunc(); m != nil {
		m(p)
	}

	s.SendOnly(p)
}

func (s *Session) SendOnly(p []byte) {
	if len(p) == 0 {
		return
	}

	st, _ := s.State()

	if st == Closed || st == Failed {
		if s.Kind == SSH && (p[0] == '\r' || p[0] == '\n') {
			s.Reconnect()
		}

		return
	}

	buf := make([]byte, len(p))
	copy(buf, p)

	select {
	case s.writes <- buf:
	case <-s.done:
	}
}

func (s *Session) Resize(cols, rows int) {
	s.term.Resize(cols, rows)

	s.mu.Lock()
	t := s.transport
	s.opts.Cols, s.opts.Rows = cols, rows
	s.mu.Unlock()

	if t != nil {
		t.Resize(cols, rows)
	}

	s.sizedOnce.Do(func() { close(s.sized) })
	s.opts.OnChange()
}

func (s *Session) Reconnect() {
	s.mu.Lock()

	if s.state == Connecting || s.state == Connected {
		s.mu.Unlock()
		return
	}

	s.state = Connecting
	s.err = nil
	s.mu.Unlock()

	s.print("\r\n")
	go s.connect()
}

func (s *Session) Close() {
	s.StopLog()

	s.mu.Lock()
	t, c := s.transport, s.client

	select {
	case <-s.done:
	default:
		close(s.done)
	}

	s.mu.Unlock()
	go release(t, c)
}

func release(t Transport, c *sshx.Client) {
	if t != nil {
		t.Close()
	}

	if c != nil {
		c.Close()
	}
}

func (s *Session) Done() <-chan struct{} {
	return s.done
}

func (s *Session) connect() {
	s.mu.Lock()
	s.gen++
	gen := s.gen
	cols, rows := s.opts.Cols, s.opts.Rows
	s.mu.Unlock()

	var t Transport
	var client *sshx.Client
	var err error

	switch s.Kind {
	case Local:
		t, err = pty.Start(pty.Options{
			Command: s.opts.Command,
			Cols:    cols,
			Rows:    rows,
			Env:     []string{"TERM=xterm-256color", "COLORTERM=truecolor", "TERM_PROGRAM=Boreal"},
		})
	case SSH:
		client, t, err = s.dialSSH(cols, rows)
	}

	if err != nil {
		s.fail(gen, err)
		return
	}

	s.mu.Lock()

	select {
	case <-s.done:
		s.mu.Unlock()
		t.Close()

		if client != nil {
			client.Close()
		}

		return
	default:
	}

	s.transport, s.client = t, client
	s.state = Connected
	s.started = time.Now()
	s.mu.Unlock()

	if s.Kind == SSH {
		t.Resize(s.term.Size())

		if cmd := strings.TrimSpace(s.opts.Target.Host.StartupCommand); cmd != "" {
			s.SendOnly([]byte(cmd + ""))
		}
	}

	s.opts.OnChange()
	s.readLoop(gen, t)
}

func (s *Session) dialSSH(cols, rows int) (*sshx.Client, Transport, error) {
	s.mu.Lock()
	s.steps = nil
	s.mu.Unlock()
	s.step("Starting connection to "+sshx.User(s.opts.Target.Host)+"@"+s.opts.Target.Host.Address, false)

	hooks := s.opts.Hooks
	hooks.Progress = func(msg string) {
		s.step(msg, false)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-s.done:
			cancel()
		case <-ctx.Done():
		}
	}()
	defer cancel()

	client, err := sshx.Dial(ctx, s.opts.Target, s.opts.Vault, hooks, s.opts.KeepAlive)

	if err != nil {
		return nil, nil, err
	}

	s.step("Opening shell…", false)
	shell, err := sshx.OpenShell(client, cols, rows, s.opts.Target.Host.ForwardAgent)

	if err != nil {
		client.Close()
		return nil, nil, err
	}

	s.step("Connected", false)
	return client, shell, nil
}

func (s *Session) readLoop(gen int, t Transport) {
	buf := make([]byte, readBufferSize)

	for {
		n, err := t.Read(buf)

		if n > 0 {
			if _, werr := s.term.Write(buf[:n]); werr != nil {
				s.print("\r\n" + styleError + "Boreal reset this terminal after an internal error." + styleReset + "\r\n")
			}

			s.writeLog(buf[:n])
			s.opts.OnChange()
		}

		if err != nil {
			s.closed(gen, err)
			return
		}
	}
}

func (s *Session) writeLoop() {
	for {
		select {
		case p := <-s.writes:
			s.mu.Lock()
			t := s.transport
			ok := s.state == Connected
			s.mu.Unlock()

			if ok && t != nil {
				if _, err := t.Write(p); err != nil {
					t.Close()
				}
			}
		case <-s.done:
			return
		}
	}
}

func (s *Session) closed(gen int, err error) {
	s.mu.Lock()

	if gen != s.gen || s.state != Connected {
		s.mu.Unlock()
		return
	}

	s.state = Closed

	if !errors.Is(err, io.EOF) {
		s.err = err
	}

	t, c := s.transport, s.client
	s.transport, s.client = nil, nil
	s.mu.Unlock()
	release(t, c)

	if s.Kind == SSH {
		s.print("\r\n" + styleInfo + "Connection closed. Press Enter to reconnect." + styleReset + "\r\n")
	}

	s.opts.OnChange()
}

func (s *Session) fail(gen int, err error) {
	s.mu.Lock()

	if gen != s.gen {
		s.mu.Unlock()
		return
	}

	s.state = Failed
	s.err = err
	s.mu.Unlock()

	msg := err.Error()

	if errors.Is(err, sshx.ErrHostKeyRejected) {
		msg = "Server key rejected."
	}

	if s.Kind == SSH {
		s.step(msg, true)
	}

	hint := ""

	if s.Kind == SSH {
		hint = " Press Enter to retry."
	}

	s.print(styleError + "✗ " + msg + styleReset + styleInfo + hint + styleReset + "\r\n")
	s.opts.OnChange()
}

func (s *Session) print(msg string) {
	if s.opts.Redact != nil {
		msg = s.opts.Redact(msg)
	}

	s.term.Write([]byte(msg))
	s.opts.OnChange()
}

func (s *Session) Describe() string {
	if s.Kind == Local {
		return "Local terminal"
	}

	return s.opts.Target.Host.Target()
}

func (s *Session) String() string {
	return fmt.Sprintf("session(%s)", s.Describe())
}
