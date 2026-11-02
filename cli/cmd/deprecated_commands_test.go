package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// deprecatedCommands are the commands deprecated in 5.1, removed in v6.
var deprecatedCommands = [][]string{{"dashboard"}, {"metrics"}}

// The deprecated commands carry the Deprecated field of cobra, with a message
// in the language of the hub that names the view of the TUI.
func TestDeprecatedCommandsNameTheMetricsView(t *testing.T) {
	defer func() { i18n.SetLocale("en"); localizeCommands(rootCmd) }()
	want := map[string]string{"en": "Metrics view", "fr": "vue « Métriques »"}
	for _, lang := range []string{"en", "fr"} {
		i18n.SetLocale(lang)
		localizeCommands(rootCmd)
		for _, path := range deprecatedCommands {
			c, _, err := rootCmd.Find(path)
			require.NoError(t, err)
			require.Equal(t, path[0], c.Name())
			assert.Contains(t, c.Deprecated, want[lang], "%s %v", lang, path)
			assert.Contains(t, c.Deprecated, "Ctrl+P", "%s %v: how to reach the view", lang, path)
		}
	}
}

// They are out of the main help, listed or not by --all.
func TestDeprecatedCommandsAreOutOfTheHelp(t *testing.T) {
	for _, path := range deprecatedCommands {
		c, _, err := rootCmd.Find(path)
		require.NoError(t, err)
		assert.False(t, helpVisible(c), "%v is shown in the help", path)
	}
	for _, all := range []bool{false, true} {
		for _, sec := range buildHelpSections(rootCmd, all) {
			for _, c := range sec.Commands {
				name, _, _ := strings.Cut(c.Name, " ")
				assert.NotContains(t, []string{"dashboard", "metrics"}, name, "listed in the help (all=%v)", all)
			}
		}
	}
	for _, sec := range helpSectionOrder {
		for _, name := range sec.commands {
			assert.NotContains(t, []string{"dashboard", "metrics"}, name)
		}
	}
}

// The warning is in the language of the hub, says « removed in v6 » and
// names the view.
func TestDeprecatedCommandWarning(t *testing.T) {
	defer i18n.SetLocale("en")
	cases := []struct{ lang, old, key, want string }{
		{"en", "oh dashboard", "cmd.dashboard.deprecated", `"oh dashboard" is deprecated and will be removed in v6: use the Metrics view of the TUI`},
		{"fr", "oh dashboard", "cmd.dashboard.deprecated", `« oh dashboard » est déprécié et sera retiré en v6 : utilisez la vue « Métriques » du TUI`},
		{"en", "oh metrics", "cmd.metrics.deprecated", `"oh metrics" is deprecated and will be removed in v6: use the Metrics view of the TUI`},
		{"fr", "oh metrics", "cmd.metrics.deprecated", `« oh metrics » est déprécié et sera retiré en v6 : utilisez la vue « Métriques » du TUI`},
	}
	for _, tc := range cases {
		i18n.SetLocale(tc.lang)
		var out bytes.Buffer
		warnDeprecatedCommand(&out, tc.old, tc.key)
		assert.Contains(t, out.String(), tc.want, "%s %s", tc.lang, tc.old)
	}
}

// cobra's own English sentence is replaced by the translated warning of the
// command; the deprecation stays for the help, and comes back after the run.
func TestCobraDeprecationSentenceIsSilenced(t *testing.T) {
	root := &cobra.Command{Use: "oh", SilenceUsage: true, SilenceErrors: true}
	ran := false
	old := &cobra.Command{Use: "old", Deprecated: "use oh new", RunE: func(*cobra.Command, []string) error { ran = true; return nil }}
	root.AddCommand(old)

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"old"})
	restore := silenceCobraDeprecation(root, []string{"old"})
	require.NoError(t, root.Execute())
	require.True(t, ran, "the deprecated command must run")
	assert.NotContains(t, out.String(), "deprecated")
	restore()
	assert.Equal(t, "use oh new", old.Deprecated)

	// Without the silencing, cobra prints its sentence (what the test guards against).
	out.Reset()
	require.NoError(t, root.Execute())
	assert.Contains(t, out.String(), "deprecated")

	// A command that is not deprecated is left alone.
	silenceCobraDeprecation(root, []string{"nope"})
	assert.NotEmpty(t, old.Deprecated)
}

// `oh metrics` still prints its figures on the standard output (scripts read
// it); the warning goes to the error output.
func TestMetricsWarnsOnTheErrorOutput(t *testing.T) {
	a := withoutTerminal(t, &config.Config{})
	out := a.IO.Out.(*bytes.Buffer)
	c, _, errOut := launchCommand(t, metricsCmd)
	require.NoError(t, c.Flags().Set("period", "7d"))
	require.NoError(t, runMetrics(c, nil))
	assert.Contains(t, out.String(), "oh metrics")
	assert.NotContains(t, out.String(), "deprecated")
	assert.Contains(t, errOut.String(), `"oh metrics" is deprecated and will be removed in v6`)
}
