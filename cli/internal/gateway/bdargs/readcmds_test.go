package bdargs

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadCommandsAreKnown(t *testing.T) {
	for k := range readCommands {
		p, ok := Known(strings.Fields(k)...)
		require.True(t, ok, k)
		assert.Equal(t, k, strings.Join(p, " "), "canonical path")
	}
}

func TestIsRead(t *testing.T) {
	for _, tc := range []struct {
		argv []string
		want bool
	}{
		{[]string{"show", "bd-1"}, true},
		{[]string{"view", "bd-1"}, true},
		{[]string{"query", "status=open"}, true},
		{[]string{"comments", "bd-1"}, true},
		{[]string{"comments", "add", "bd-1", "x"}, false},
		{[]string{"dep", "tree", "bd-1"}, true},
		{[]string{"dep", "add", "a", "b"}, false},
		{[]string{"ready"}, true},
		{[]string{"ready", "--claim"}, false},
		{[]string{"update", "bd-1"}, false},
		{[]string{"note", "bd-1", "x"}, false},
		{[]string{"config", "get", "k"}, true},
		{[]string{"config", "set", "k", "v"}, false},
		{[]string{"show", "--help"}, false},
		{[]string{"--version"}, false},
	} {
		c, err := Parse(tc.argv)
		require.NoError(t, err, tc.argv)
		assert.Equal(t, tc.want, IsRead(c), tc.argv)
	}
}

func TestEntryIsReadAndReadOnly(t *testing.T) {
	for entry, want := range map[string]bool{
		"show": true, "view": true, "comments": true, "comments list": true, "comments add": false,
		"dep tree": true, "dep": false, "dep add": false, "ready": true, "query": true, "create": false,
		"note": false, "assign": false, "label": false, "label list-all": true, "unknown": false, "": false, "a b c": false,
		"comments bd-1": true,
	} {
		assert.Equal(t, want, EntryIsRead(entry), entry)
	}
	assert.True(t, ReadOnly([]string{"show", "list", "comments"}))
	assert.False(t, ReadOnly([]string{"show", "create"}))
	assert.False(t, ReadOnly(nil), "an empty list grants nothing")
}
