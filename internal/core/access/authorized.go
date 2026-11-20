package access

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/ssh"
)

const (
	maxFrom        = 32
	maxCommand     = 512
	maxComment     = 120
	expiryDay      = "20060102"
	expiryMinute   = "200601021504"
	commentSuffix  = " (boreal)"
	optFrom        = "from"
	optCommand     = "command"
	optExpiry      = "expiry-time"
	optNoPortFwd   = "no-port-forwarding"
	optNoAgentFwd  = "no-agent-forwarding"
	optNoX11Fwd    = "no-X11-forwarding"
	optNoPTY       = "no-pty"
	commentMarker  = "#"
	lineSeparator  = "\n"
	optSeparator   = ","
	fieldSeparator = " "
)

var (
	fromPattern   = regexp.MustCompile(`^!?[A-Za-z0-9._:*?/\-\[\]]+$`)
	expiryPattern = regexp.MustCompile(`^\d{8}(\d{4})?$`)

	ErrFrom    = errors.New("each source must be a host name, address or CIDR without spaces, quotes or commas")
	ErrCommand = errors.New("the command can't contain quotes, backslashes or control characters")
	ErrExpiry  = errors.New("the expiry must be a future date written as YYYYMMDD or YYYYMMDDHHMM")
)

type Restrictions struct {
	From         []string `json:"from,omitempty" jsonschema:"hosts or networks the key may connect from, like 10.0.0.0/8 or *.example.com"`
	Command      string   `json:"command,omitempty" jsonschema:"the only command this key may run"`
	Expiry       string   `json:"expiry,omitempty" jsonschema:"date after which the key stops working, as YYYYMMDD or YYYYMMDDHHMM in the server's time zone"`
	NoForwarding bool     `json:"noForwarding,omitempty" jsonschema:"block port, agent and X11 forwarding"`
	NoPTY        bool     `json:"noPty,omitempty" jsonschema:"block interactive terminals"`
}

type Entry struct {
	Line        int      `json:"line"`
	Type        string   `json:"type"`
	Fingerprint string   `json:"fingerprint"`
	Comment     string   `json:"comment,omitempty"`
	Options     []string `json:"options,omitempty"`
}

func (r Restrictions) Validate(now time.Time) error {
	if len(r.From) > maxFrom {
		return fmt.Errorf("at most %d sources: %w", maxFrom, ErrFrom)
	}

	for _, f := range r.From {
		if !fromPattern.MatchString(f) {
			return fmt.Errorf("%q: %w", f, ErrFrom)
		}
	}

	if len(r.Command) > maxCommand || strings.ContainsAny(r.Command, `"\`) || strings.IndexFunc(r.Command, unicode.IsControl) >= 0 {
		return ErrCommand
	}

	if r.Expiry == "" {
		return nil
	}

	if !expiryPattern.MatchString(r.Expiry) {
		return ErrExpiry
	}

	layout := expiryDay

	if len(r.Expiry) == len(expiryMinute) {
		layout = expiryMinute
	}

	at, err := time.Parse(layout, r.Expiry)

	if err != nil || !at.After(now) {
		return ErrExpiry
	}

	return nil
}

func (r Restrictions) options() []string {
	var out []string

	if len(r.From) > 0 {
		out = append(out, optFrom+`="`+strings.Join(r.From, optSeparator)+`"`)
	}

	if r.Command != "" {
		out = append(out, optCommand+`="`+r.Command+`"`)
	}

	if r.Expiry != "" {
		out = append(out, optExpiry+`="`+r.Expiry+`"`)
	}

	if r.NoForwarding {
		out = append(out, optNoPortFwd, optNoAgentFwd, optNoX11Fwd)
	}

	if r.NoPTY {
		out = append(out, optNoPTY)
	}

	return out
}

func Parse(content []byte) []Entry {
	var out []Entry

	for i, line := range strings.Split(string(content), lineSeparator) {
		key, comment, opts, ok := parseLine(line)

		if !ok {
			continue
		}

		out = append(out, Entry{
			Line:        i + 1,
			Type:        key.Type(),
			Fingerprint: ssh.FingerprintSHA256(key),
			Comment:     comment,
			Options:     opts,
		})
	}

	return out
}

func parseLine(line string) (ssh.PublicKey, string, []string, bool) {
	trimmed := strings.TrimSpace(line)

	if trimmed == "" || strings.HasPrefix(trimmed, commentMarker) {
		return nil, "", nil, false
	}

	key, comment, opts, _, err := ssh.ParseAuthorizedKey([]byte(trimmed))

	if err != nil {
		return nil, "", nil, false
	}

	return key, comment, opts, true
}

func cleanComment(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}

		return r
	}, s)
	s = strings.TrimSpace(s)

	if len([]rune(s)) > maxComment {
		s = string([]rune(s)[:maxComment])
	}

	return s + commentSuffix
}

func Line(key ssh.PublicKey, label string, r Restrictions) string {
	body := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
	line := body + fieldSeparator + cleanComment(label)

	if opts := r.options(); len(opts) > 0 {
		line = strings.Join(opts, optSeparator) + fieldSeparator + line
	}

	return line
}

func Grant(content []byte, key ssh.PublicKey, label string, r Restrictions, now time.Time) ([]byte, bool, error) {
	if err := r.Validate(now); err != nil {
		return nil, false, err
	}

	fp := ssh.FingerprintSHA256(key)
	want := Line(key, label, r)
	lines := strings.Split(string(content), lineSeparator)
	out := make([]string, 0, len(lines)+1)
	placed := false

	for _, line := range lines {
		k, _, _, ok := parseLine(line)

		if !ok || ssh.FingerprintSHA256(k) != fp {
			out = append(out, line)
			continue
		}

		if !placed {
			out = append(out, want)
			placed = true
		}
	}

	if !placed {
		for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
			out = out[:len(out)-1]
		}

		out = append(out, want, "")
	}

	next := []byte(strings.Join(out, lineSeparator))
	return next, !bytes.Equal(next, content), nil
}

func Revoke(content []byte, fingerprint string) ([]byte, int) {
	lines := strings.Split(string(content), lineSeparator)
	kept := slices.DeleteFunc(slices.Clone(lines), func(line string) bool {
		k, _, _, ok := parseLine(line)
		return ok && ssh.FingerprintSHA256(k) == fingerprint
	})

	return []byte(strings.Join(kept, lineSeparator)), len(lines) - len(kept)
}
