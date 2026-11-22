package ui

import (
	"errors"
	"testing"

	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/core/vault"
)

func TestSentence(t *testing.T) {
	cases := map[string]string{
		"the two passwords don't match": "The two passwords don't match.",
		"already done.":                 "Already done.",
		"really?":                       "Really?",
		"  émoji first  ":               "Émoji first.",
		"":                              "",
	}

	for in, want := range cases {
		if got := Sentence(errors.New(in)); got != want {
			t.Errorf("Sentence(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPasswordField(t *testing.T) {
	var pass, repeat widget.Editor

	if PasswordField(vault.ErrMismatch, &pass, &repeat) != &repeat {
		t.Error("a mismatch belongs to the repeat field")
	}

	if PasswordField(vault.ErrShortPassword, &pass, &repeat) != &pass {
		t.Error("a short password belongs to the password field")
	}
}
