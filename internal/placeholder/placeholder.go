// Package placeholder fills the {{...}} placeholders of a snippet: values
// from the host it runs on, and {{ask:Question}} answers asked when it runs.
package placeholder

import (
	"strings"
)

const (
	openTag   = "{{"
	closeTag  = "}}"
	askPrefix = "ask:"
)

const (
	nameHost  = "host"
	nameUser  = "user"
	namePort  = "port"
	nameLabel = "label"
)

// Values are the host details a snippet can use.
type Values struct {
	Host  string
	User  string
	Port  string
	Label string
}

type token struct {
	start, end int
	name       string
	ask        string
}

func scan(cmd string) []token {
	var out []token

	for i := 0; i < len(cmd); {
		s := strings.Index(cmd[i:], openTag)

		if s < 0 {
			break
		}

		s += i
		e := strings.Index(cmd[s+len(openTag):], closeTag)

		if e < 0 {
			break
		}

		e += s + len(openTag)
		inner := strings.TrimSpace(cmd[s+len(openTag) : e])
		t := token{start: s, end: e + len(closeTag)}

		switch {
		case len(inner) > len(askPrefix) && strings.EqualFold(inner[:len(askPrefix)], askPrefix):
			t.ask = strings.TrimSpace(inner[len(askPrefix):])
		default:
			t.name = strings.ToLower(inner)
		}

		if t.ask != "" || known(t.name) {
			out = append(out, t)
			i = t.end
			continue
		}

		i = s + 1
	}

	return out
}

func known(name string) bool {
	switch name {
	case nameHost, nameUser, namePort, nameLabel:
		return true
	}

	return false
}

// Asks lists the questions of a snippet in the order they first appear.
func Asks(cmd string) []string {
	var out []string
	seen := map[string]struct{}{}

	for _, t := range scan(cmd) {
		if t.ask == "" {
			continue
		}

		if _, ok := seen[t.ask]; ok {
			continue
		}

		seen[t.ask] = struct{}{}
		out = append(out, t.ask)
	}

	return out
}

// UsesHost reports whether the snippet has placeholders filled from the host.
func UsesHost(cmd string) bool {
	for _, t := range scan(cmd) {
		if t.name != "" {
			return true
		}
	}

	return false
}

// Fill replaces the placeholders. Questions without an answer become empty.
func Fill(cmd string, v Values, answers map[string]string) string {
	tokens := scan(cmd)

	if len(tokens) == 0 {
		return cmd
	}

	var b strings.Builder
	last := 0

	for _, t := range tokens {
		b.WriteString(cmd[last:t.start])
		last = t.end

		if t.ask != "" {
			b.WriteString(answers[t.ask])
			continue
		}

		switch t.name {
		case nameHost:
			b.WriteString(v.Host)
		case nameUser:
			b.WriteString(v.User)
		case namePort:
			b.WriteString(v.Port)
		case nameLabel:
			b.WriteString(v.Label)
		}
	}

	b.WriteString(cmd[last:])
	return b.String()
}
