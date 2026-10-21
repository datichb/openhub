package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Texts are cut in characters, never in the middle of one, and never panic
// whatever the limit.
func TestClipping(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{"fits", "abc", 3, "abc"},
		{"cut with an ellipsis", "abcdef", 4, "abc…"},
		{"accents count as one character", "équipe-état", 6, "équip…"},
		{"emoji and CJK are not split", "日本語の文字列", 4, "日本語…"},
		{"limit of one", "abc", 1, "…"},
		{"limit of zero", "abc", 0, ""},
		{"negative limit", "abc", -3, ""},
		{"empty text", "", 0, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, clipText(tc.in, tc.limit), "clipText")
			assert.Equal(t, tc.want, truncateString(tc.in, tc.limit), "truncateString")
		})
	}
}
