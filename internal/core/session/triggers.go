package session

import (
	"regexp"
	"sync"

	"github.com/northlighthq/vt/parser"
)

const maxTriggerLine = 4096

type Rule struct {
	ID string
	Re *regexp.Regexp
}

type hit struct {
	rule Rule
	line string
}

type lineMatcher struct {
	mu       sync.Mutex
	rules    []Rule
	on       func(Rule, string)
	parser   *parser.Parser
	line     []rune
	cr       bool
	wasTyped bool
	fired    map[string]struct{}
	hits     []hit
}

func (s *Session) SetTriggers(rules []Rule, on func(Rule, string)) {
	s.matcher.set(rules, on)
}

func (m *lineMatcher) set(rules []Rule, on func(Rule, string)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.rules, m.on = rules, on

	if m.parser == nil {
		m.parser = parser.New(m)
		m.fired = map[string]struct{}{}
	}
}

func (m *lineMatcher) typed() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.wasTyped = true
}

func (m *lineMatcher) feed(p []byte) {
	m.mu.Lock()

	if len(m.rules) == 0 {
		m.mu.Unlock()
		return
	}

	m.parser.Feed(p)
	m.check()
	hits, on := m.hits, m.on
	m.hits = nil
	m.mu.Unlock()

	for _, h := range hits {
		on(h.rule, h.line)
	}
}

func (m *lineMatcher) check() {
	if m.wasTyped || len(m.line) == 0 {
		return
	}

	line := string(m.line)

	for _, r := range m.rules {
		if _, ok := m.fired[r.ID]; ok || !r.Re.MatchString(line) {
			continue
		}

		m.fired[r.ID] = struct{}{}
		m.hits = append(m.hits, hit{rule: r, line: line})
	}
}

func (m *lineMatcher) restart() {
	m.line = m.line[:0]
	m.cr = false
	clear(m.fired)
}

func (m *lineMatcher) Print(r rune) {
	if m.cr {
		m.restart()
	}

	if len(m.line) < maxTriggerLine {
		m.line = append(m.line, r)
	}
}

func (m *lineMatcher) Execute(b byte) {
	switch b {
	case '\n':
		m.check()
		m.restart()
		m.wasTyped = false
	case '\r':
		m.cr = true
	case '\b':
		if n := len(m.line); n > 0 && !m.cr {
			m.line = m.line[:n-1]
		}
	case '\t':
		m.Print(' ')
	}
}

func (m *lineMatcher) CSI(*parser.Params, []byte, byte) {}

func (m *lineMatcher) ESC([]byte, byte) {}

func (m *lineMatcher) OSC([]byte) {}
