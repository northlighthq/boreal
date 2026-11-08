package terminal

import (
	"time"

	"github.com/northlighthq/vt/input"

	"github.com/northlighthq/boreal/internal/common/termview"
	"github.com/northlighthq/boreal/internal/core/session"
	"github.com/northlighthq/boreal/internal/core/vault"
	"github.com/northlighthq/boreal/internal/mobile/ui"
)

type Host interface {
	ui.Host
	Sessions() []*Session
	Close(s *Session)
	Open(s *Session)
}

type Session struct {
	Sess   *session.Session
	View   *termview.View
	Host   vault.Host
	Opened time.Time
	ctrl   bool
	alt    bool
}

func NewSession(s *session.Session, v *termview.View, h vault.Host) *Session {
	t := &Session{Sess: s, View: v, Host: h, Opened: time.Now()}
	v.Transform = t.transform
	return t
}

func (s *Session) State() session.State {
	st, _ := s.Sess.State()
	return st
}

func (s *Session) Err() error {
	_, err := s.Sess.State()
	return err
}

func (s *Session) mods() input.Mod {
	var m input.Mod

	if s.ctrl {
		m |= input.ModCtrl
	}

	if s.alt {
		m |= input.ModAlt
	}

	s.ctrl, s.alt = false, false
	return m
}

func (s *Session) transform(text string) string {
	if !s.ctrl && !s.alt {
		return text
	}

	rs := []rune(text)

	if len(rs) == 0 {
		return text
	}

	ctrl, alt := s.ctrl, s.alt
	s.ctrl, s.alt = false, false
	first := string(rs[0])

	if ctrl {
		if c, ok := controlChar(rs[0]); ok {
			first = string(c)
		}
	}

	if alt {
		first = "\x1b" + first
	}

	return first + string(rs[1:])
}

func controlChar(r rune) (rune, bool) {
	switch {
	case r >= 'a' && r <= 'z':
		return r - 'a' + 1, true
	case r >= 'A' && r <= 'Z':
		return r - 'A' + 1, true
	case r == '@' || r == ' ' || r == '2':
		return 0, true
	case r == '[' || r == '3':
		return 0x1b, true
	case r == '\\' || r == '4':
		return 0x1c, true
	case r == ']' || r == '5':
		return 0x1d, true
	case r == '^' || r == '6':
		return 0x1e, true
	case r == '_' || r == '7' || r == '-':
		return 0x1f, true
	case r == '?' || r == '8':
		return 0x7f, true
	}

	return 0, false
}
