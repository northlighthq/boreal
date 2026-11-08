package session

import (
	"regexp"
	"slices"
	"testing"
)

type fired struct {
	id, line string
}

func newMatcher(t *testing.T, patterns ...string) (*lineMatcher, *[]fired) {
	t.Helper()
	var got []fired
	rules := make([]Rule, len(patterns))

	for i, p := range patterns {
		rules[i] = Rule{ID: p, Re: regexp.MustCompile(p)}
	}

	m := &lineMatcher{}
	m.set(rules, func(r Rule, line string) { got = append(got, fired{r.ID, line}) })
	return m, &got
}

func TestTriggersMatchOutputLines(t *testing.T) {
	m, got := newMatcher(t, "ERROR", "done")

	m.feed([]byte("starting\r\n\x1b[31mERR"))
	m.feed([]byte("OR\x1b[0m: disk full\r\nall done\r\n"))

	want := []fired{{"ERROR", "ERROR: disk full"}, {"done", "all done"}}

	if !slices.Equal(*got, want) {
		t.Fatalf("got %q, want %q", *got, want)
	}
}

func TestTriggersFirePromptsOnce(t *testing.T) {
	m, got := newMatcher(t, "(?i)password:")

	m.feed([]byte("[sudo] password: "))
	m.feed([]byte(" "))
	m.feed([]byte("\r\n"))

	if len(*got) != 1 {
		t.Fatalf("a prompt without a newline should fire once, got %q", *got)
	}
}

func TestTriggersSkipTypedLines(t *testing.T) {
	m, got := newMatcher(t, "ERROR")

	m.feed([]byte("$ "))
	m.typed()
	m.feed([]byte("grep ERROR app.log\r\n"))
	m.feed([]byte("ERROR: failed\r\n"))

	want := []fired{{"ERROR", "ERROR: failed"}}

	if !slices.Equal(*got, want) {
		t.Fatalf("the echo of a typed command should not fire, got %q", *got)
	}
}

func TestTriggersFollowOverwrittenLines(t *testing.T) {
	m, got := newMatcher(t, "100%")

	m.feed([]byte("10%\r50%\r100%\r\n"))
	m.feed([]byte("ab\bc\r\n"))

	if !slices.Equal(*got, []fired{{"100%", "100%"}}) {
		t.Fatalf("got %q", *got)
	}

	if len(m.line) != 0 {
		t.Fatalf("the line should be empty after a newline, got %q", string(m.line))
	}
}

func TestTriggersOffWithoutRules(t *testing.T) {
	m, got := newMatcher(t)

	m.feed([]byte("ERROR\r\n"))

	if len(*got) != 0 || len(m.line) != 0 {
		t.Fatal("with no rules nothing should be parsed")
	}
}
