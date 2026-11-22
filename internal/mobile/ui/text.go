package ui

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"gioui.org/widget"

	"github.com/northlighthq/boreal/internal/core/vault"
)

func Sentence(err error) string {
	s := strings.TrimSpace(err.Error())

	if s == "" {
		return s
	}

	r, n := utf8.DecodeRuneInString(s)
	s = string(unicode.ToUpper(r)) + s[n:]

	if !strings.HasSuffix(s, ".") && !strings.HasSuffix(s, "?") && !strings.HasSuffix(s, "!") {
		s += "."
	}

	return s
}

func PasswordField(err error, password, repeat *widget.Editor) *widget.Editor {
	if errors.Is(err, vault.ErrMismatch) {
		return repeat
	}

	return password
}
