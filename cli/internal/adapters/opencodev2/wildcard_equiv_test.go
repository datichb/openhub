package opencodev2

import (
	"testing"

	"github.com/datichb/openhub/cli/internal/wildcard"
)

// The adapter keeps its own copy of the wildcard match (D19: the adapter
// does not depend on oh internals to interpret the rules it renders). The
// copy and the neutral package must never drift apart: oh evaluates the same
// permission rules with wildcard.Match.
func TestWildcardMatchEquivalentToNeutralPackage(t *testing.T) {
	patterns := []string{
		"", "*", "**", "?", "??", "a", "a?c", "a*c", "a*b*c", "*a", "a*", "*?*", "a*?", "é", "?é", "é?", "*é",
		"caf?", "caf??", "日?", "😀", "?😀", "*😀*", "??", "git status *", "*/bd", "/home/?/x", "x\\*",
	}
	values := []string{
		"", "a", "ab", "abc", "ac", "abbbc", "axxbyyc", "axxbyy", "é", "e", "éa", "aé", "café", "cafe", "cafés",
		"日本", "日", "😀", "a😀", "😀a", "a😀b", "😀😀", "git status --short", "x/bd", "/home/é/x", "/home/ab/x", "x\\y", "a*c", "a?c",
	}
	for _, p := range patterns {
		for _, s := range values {
			if got, want := wildcardMatch(p, s), wildcard.Match(p, s); got != want {
				t.Errorf("wildcardMatch(%q, %q) = %v, wildcard.Match = %v", p, s, got, want)
			}
		}
	}
}
