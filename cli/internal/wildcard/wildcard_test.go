package wildcard

import "testing"

func TestMatch(t *testing.T) {
	cases := []struct {
		pattern, s string
		want       bool
	}{
		{"", "", true},
		{"", "a", false},
		{"*", "", true},
		{"*", "anything/with/slashes", true},
		{"**", "", true},
		{"?", "", false},
		{"?", "a", true},
		{"?", "ab", false},
		{"a?c", "abc", true},
		{"a?c", "ac", false},
		{"abc", "abc", true},
		{"abc", "abd", false},
		{"abc", "ab", false},
		{"a*c", "ac", true},
		{"a*c", "abbbc", true},
		{"a*c", "abbbd", false},
		{"a*b*c", "axxbyyc", true},
		{"a*b*c", "axxbyy", false},
		{"*b", "abab", true},
		{"*b", "abac", false},
		{"go test*", "go test ./...", true},
		{"claude-*", "claude-sonnet-5", true},
		{"claude-*", "gpt-5", false},
		{"a*?", "a", false},
		{"a*?", "ab", true},
		{"*?*", "", false},
		// '*' and '?' in the value are plain characters
		{"a?c", "a*c", true},
		{"a*c", "a?c", true},
		{"a\\*", "a\\x", true},
		// multi-byte characters: one '?' is one character, not one byte
		{"?", "é", true},
		{"??", "é", false},
		{"caf?", "café", true},
		{"caf??", "café", false},
		{"caf?", "cafe", true},
		{"é?", "éà", true},
		{"é?", "éab", false},
		{"*é", "café", true},
		{"/home/?/projet", "/home/é/projet", true},
		{"/home/*/projet", "/home/zoé/projet", true},
		{"?", "日", true},
		{"?", "😀", true},
		{"??", "😀", false},
		{"a?b", "a😀b", true},
		{"a??b", "a😀b", false},
		{"*😀*", "a😀b", true},
		{"😀", "😀", true},
		{"😀", "😁", false},
		{"é", "e", false},
		{"e", "é", false},
	}
	for _, c := range cases {
		if got := Match(c.pattern, c.s); got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.pattern, c.s, got, c.want)
		}
	}
}
