package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// The command line that stands for each command removed in 5.1, with the
// replacement the message must name.
var removedCommandCases = []struct {
	args        []string
	old         string
	replacement string
}{
	{[]string{"start", "--dev"}, "oh start", "oh run libre"},
	{[]string{"audit", "-t", "security"}, "oh audit", "oh run audit"},
	{[]string{"review", "--publish"}, "oh review", "oh mr create"},
	{[]string{"review", "feedback", "PROJ-1"}, "oh review feedback", "oh run review-feedback"},
	{[]string{"debug", "-i", "boom"}, "oh debug", "oh run debug"},
	{[]string{"service"}, "oh service", "oh mcp status"},
	{[]string{"service", "setup", "gitlab"}, "oh service setup", "oh mcp setup"},
	{[]string{"service", "remove", "gitlab", "--force"}, "oh service remove", "oh mcp disable"},
	{[]string{"takeover-brief", "enrich", "bd-1"}, "oh takeover-brief enrich", "oh run brief-enrich"},
	{[]string{"tb", "enrich", "bd-1"}, "oh takeover-brief enrich", "oh run brief-enrich"},
}

// 5.1: a removed command says what replaces it, in the language of the hub,
// and fails with ExitUsage (a script must notice); its former options do not
// make it fail differently.
func TestRemovedCommandsNameTheirReplacement(t *testing.T) {
	defer i18n.SetLocale("en")
	for _, lang := range []string{"en", "fr"} {
		i18n.SetLocale(lang)
		for _, tc := range removedCommandCases {
			c, rest, err := rootCmd.Find(tc.args)
			require.NoError(t, err, tc.args)
			require.True(t, c.Hidden, "%v is out of the help", tc.args)

			var out bytes.Buffer
			c.SetErr(&out)
			err = c.RunE(c, rest)
			require.Error(t, err, tc.args)
			assert.Equal(t, ExitUsage, ExitCode(err), tc.args)
			assert.Contains(t, out.String(), tc.old, "%s %v", lang, tc.args)
			assert.Contains(t, out.String(), tc.replacement, "%s %v", lang, tc.args)
			assert.Contains(t, out.String(), "5.1", "%s %v", lang, tc.args)
			if lang == "fr" {
				assert.Contains(t, out.String(), "retirée", tc.args)
			}
			c.SetErr(nil)
		}
	}
}

// 5.1: the commands the messages send to exist, and the removed ones are in
// neither the help of oh, nor the list of the commands of takeover-brief.
func TestRemovedCommandsReplacementsExist(t *testing.T) {
	for _, path := range [][]string{{"run"}, {"mr", "create"}, {"session", "attach"}, {"mcp", "status"}, {"mcp", "setup"}, {"mcp", "disable"}, {"secrets", "delete"}} {
		c, _, err := rootCmd.Find(path)
		require.NoError(t, err, path)
		assert.Equal(t, path[len(path)-1], c.Name(), path)
		assert.False(t, c.Hidden, path)
	}

	listed := map[string]bool{}
	for _, sec := range buildHelpSections(rootCmd, true) {
		for _, c := range sec.Commands {
			name, _, _ := strings.Cut(c.Name, " ")
			listed[name] = true
			listed[c.Name] = true
		}
	}
	for _, name := range []string{"start", "audit", "review", "debug", "service", "takeover-brief enrich"} {
		assert.False(t, listed[name], "%s is listed in oh --help", name)
	}
	assert.True(t, listed["run"] && (listed["mr"] || listed["mr create"]), "run and mr create must be listed: %v", listed)
	enrich, _, _ := rootCmd.Find([]string{"takeover-brief", "enrich"})
	assert.True(t, enrich.Hidden)
}

// 5.1: oh mr create is the publication of a merge request (the review itself is oh run review).
func TestMRCreateIsANormalCommand(t *testing.T) {
	c, _, err := rootCmd.Find([]string{"mr", "create"})
	require.NoError(t, err)
	require.Equal(t, "create", c.Name())
	for _, flag := range []string{"project", "reviewer"} {
		assert.NotNil(t, c.Flags().Lookup(flag), "--%s missing from oh mr create", flag)
	}
	assert.True(t, helpVisible(c))
}
