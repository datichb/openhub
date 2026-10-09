package termtext

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// S-05: a journal entry cannot erase, recolor or retitle the terminal.
func TestClean(t *testing.T) {
	for in, want := range map[string]string{
		"update bd-42 --notes ok":       "update bd-42 --notes ok",
		"note bd-42 café ✓":             "note bd-42 café ✓",
		"x\x1b[2K\x1b[1A\rclose bd-1":   `x\x1b[2K\x1b[1A\x0dclose bd-1`,
		"\x1b]0;pwned\x07title":         `\x1b]0;pwned\x07title`,
		"\x1b]8;;http://evil\x1b\\link": `\x1b]8;;http://evil\x1b\link`,
		"a\u009b31mb":                   `a\u009b31mb`,
		"line1\nline2\tx":               `line1\nline2\tx`,
		"admin\u202egnp.exe":            `admin\u202egnp.exe`,
		"bad\xffbyte":                   `bad\xffbyte`,
		"zero\u200bwidth":               `zero\u200bwidth`,
	} {
		got := Clean(in)
		assert.Equal(t, want, got, "%q", in)
		assert.False(t, strings.ContainsAny(got, "\x1b\r\n\a\u009b\u202e"), "%q", got)
	}
	assert.Equal(t, []string{`a\x1b`, "b"}, CleanAll([]string{"a\x1b", "b"}))
}
