package session

import (
	"bufio"
	"io"

	"github.com/northlighthq/vt/parser"
)

type textLog struct {
	out    io.WriteCloser
	buf    *bufio.Writer
	parser *parser.Parser
}

func newTextLog(w io.WriteCloser) *textLog {
	l := &textLog{out: w, buf: bufio.NewWriter(w)}
	l.parser = parser.New(l)
	return l
}

func (l *textLog) Print(r rune) {
	l.buf.WriteRune(r)
}

func (l *textLog) Execute(b byte) {
	switch b {
	case '\n':
		l.buf.WriteString("\r\n")
	case '\t':
		l.buf.WriteByte('\t')
	}
}

func (l *textLog) CSI(*parser.Params, []byte, byte) {}

func (l *textLog) ESC([]byte, byte) {}

func (l *textLog) OSC([]byte) {}

func (l *textLog) close() error {
	l.buf.Flush()
	return l.out.Close()
}

func (s *Session) StartLog(w io.WriteCloser) {
	s.logMu.Lock()
	defer s.logMu.Unlock()

	if s.log != nil {
		s.log.close()
	}

	s.log = newTextLog(w)
}

func (s *Session) StopLog() error {
	s.logMu.Lock()
	defer s.logMu.Unlock()

	if s.log == nil {
		return nil
	}

	err := s.log.close()
	s.log = nil
	return err
}

func (s *Session) Logging() bool {
	s.logMu.Lock()
	defer s.logMu.Unlock()

	return s.log != nil
}

func (s *Session) writeLog(p []byte) {
	s.logMu.Lock()
	defer s.logMu.Unlock()

	if s.log != nil {
		s.log.parser.Feed(p)
		s.log.buf.Flush()
	}
}

func (s *Session) SetMirror(fn func(p []byte)) {
	s.logMu.Lock()
	defer s.logMu.Unlock()

	s.mirror = fn
}

func (s *Session) mirrorFunc() func(p []byte) {
	s.logMu.Lock()
	defer s.logMu.Unlock()

	return s.mirror
}
