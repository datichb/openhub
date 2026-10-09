// Package termtext neutralizes untrusted text before it is shown in a
// terminal: control characters (ANSI/OSC escape sequences, carriage
// returns, C1 codes) and bidirectional overrides are made visible instead of
// being interpreted, so that they cannot hide or rewrite what is displayed.
package termtext

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Clean returns s on one line with every control character, bidirectional
// control and invalid byte escaped (\x1b, \n, \u202e…).
func Clean(s string) string {
	if isPlain(s) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&b, `\x%02x`, s[i])
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		case unicode.IsControl(r) || isBidi(r) || unicode.Is(unicode.Cf, r):
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r)
		}
		i += size
	}
	return b.String()
}

// CleanAll applies Clean to every element.
func CleanAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = Clean(s)
	}
	return out
}

func isPlain(s string) bool {
	for i := 0; i < len(s); i++ {
		if c := s[i]; c < 0x20 || c >= 0x7f {
			return false
		}
	}
	return true
}

func isBidi(r rune) bool {
	return (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069) || r == 0x200e || r == 0x200f || r == 0x061c
}
