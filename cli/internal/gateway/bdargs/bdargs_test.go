package bdargs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseForms(t *testing.T) {
	for _, tc := range []struct {
		argv   []string
		key    string
		status string
		args   []string
	}{
		{[]string{"update", "pr-1", "-s", "closed"}, "update", "closed", []string{"pr-1"}},
		{[]string{"update", "pr-1", "-sclosed"}, "update", "closed", []string{"pr-1"}},
		{[]string{"update", "pr-1", "-s=closed"}, "update", "closed", []string{"pr-1"}},
		{[]string{"update", "pr-1", "--status=closed"}, "update", "closed", []string{"pr-1"}},
		{[]string{"update", "--status", "closed", "pr-1"}, "update", "closed", []string{"pr-1"}},
		{[]string{"-q", "update", "pr-1", "-s", "open", "-sclosed"}, "update", "closed", []string{"pr-1"}},
		{[]string{"--actor", "me", "update", "pr-1", "-s", "closed"}, "update", "closed", []string{"pr-1"}},
		{[]string{"-s", "closed", "update", "pr-1"}, "update", "closed", []string{"pr-1"}},
		{[]string{"update", "--", "-s", "closed"}, "update", "", []string{"-s", "closed"}},
		{[]string{"done", "pr-1"}, "close", "", []string{"pr-1"}},
		{[]string{"dep", "add", "a", "b"}, "dep add", "", []string{"a", "b"}},
		{[]string{"view", "pr-1"}, "show", "", []string{"pr-1"}},
		{[]string{"--version"}, "", "", nil},
	} {
		c, err := Parse(tc.argv)
		require.NoError(t, err, tc.argv)
		assert.Equal(t, tc.key, c.Key(), tc.argv)
		st, _ := c.Value("status")
		assert.Equal(t, tc.status, st, tc.argv)
		assert.Equal(t, tc.args, c.Args, tc.argv)
	}
}

// S-08: attached and combined short flags are read as bd reads them.
func TestParseCombinedShorts(t *testing.T) {
	c, err := Parse([]string{"list", "-qC/other"})
	require.NoError(t, err)
	assert.True(t, c.Bool("quiet"))
	dir, ok := c.Value("directory")
	assert.True(t, ok)
	assert.Equal(t, "/other", dir)

	c, err = Parse([]string{"export", "-o/etc/x"})
	require.NoError(t, err)
	out, _ := c.Value("output")
	assert.Equal(t, "/etc/x", out)

	c, err = Parse([]string{"create", "-f/home/u/.ssh/id_rsa"})
	require.NoError(t, err)
	f, _ := c.Value("file")
	assert.Equal(t, "/home/u/.ssh/id_rsa", f)

	c, err = Parse([]string{"close", "-f", "pr-1"})
	require.NoError(t, err)
	assert.True(t, c.Bool("force"), "-f is --force for close")
	assert.Equal(t, []string{"pr-1"}, c.Args)
}

func TestParseErrors(t *testing.T) {
	for _, argv := range [][]string{
		{"--DB=/x", "list"},
		{"list", "--no_color"},
		{"list", "-Z"},
		{"update", "pr-1", "-s"},
		{"nosuchcommand"},
		{"list", "---x"},
	} {
		_, err := Parse(argv)
		var e *Error
		assert.ErrorAs(t, err, &e, argv)
	}
}

func TestArgvIsCanonical(t *testing.T) {
	c, err := Parse([]string{"-q", "update", "pr-1", "-sclosed", "--claim", "--", "-x"})
	require.NoError(t, err)
	assert.Equal(t, []string{"update", "--quiet", "--status=closed", "--claim", "--", "pr-1", "-x"}, c.Argv())
	again, err := Parse(c.Argv())
	require.NoError(t, err)
	assert.Equal(t, c, again)
}

func TestKnown(t *testing.T) {
	p, ok := Known("done")
	assert.True(t, ok)
	assert.Equal(t, []string{"close"}, p)
	_, ok = Known("dep", "nope")
	assert.False(t, ok)
}
