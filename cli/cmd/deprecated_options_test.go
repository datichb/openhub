package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// shortMeanings is the table « one letter, one meaning » (U-03, documented in
// docs/reference/cli.*.md): the long names a visible short option may have.
var shortMeanings = map[string][]string{
	"a": {"agent"},
	"A": {"assignee"},
	"b": {"base"},
	"d": {"path"},
	"f": {"force"},
	"i": {"input"},
	"l": {"language"},
	"m": {"message", "prompt"}, // prompt: the message of oh start
	"n": {"name"},
	"o": {"output"},
	"p": {"project"},
	"P": {"provider"},
	"r": {"resume"},
	"s": {"status"},
	"t": {"ticket"},
	"v": {"verbose"},
	"w": {"workflow"},
	"y": {"yes"},
}

func TestOneLetterOneMeaning(t *testing.T) {
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		check := func(f *pflag.Flag) {
			if f.Shorthand == "" || f.Name == "help" || f.Hidden || f.ShorthandDeprecated != "" {
				return
			}
			ok := false
			for _, name := range shortMeanings[f.Shorthand] {
				ok = ok || name == f.Name
			}
			if !ok {
				t.Errorf("%s: -%s means --%s, expected one of %v", c.CommandPath(), f.Shorthand, f.Name, shortMeanings[f.Shorthand])
			}
		}
		c.Flags().VisitAll(check)
		c.PersistentFlags().VisitAll(check)
		for _, s := range c.Commands() {
			walk(s)
		}
	}
	walk(rootCmd)
}

// U-03: the former forms stay accepted, with a warning; the new forms do not warn.
func TestDeprecatedShorthandsAreAcceptedWithWarning(t *testing.T) {
	defer i18n.SetLocale("en")
	// The language of the test, not the one of the hub (ensureLocale).
	saved := application
	application = &app.App{}
	defer func() { application = saved }()
	cases := []struct {
		path        []string
		old, target string
		value       string
		newForm     string
	}{
		{[]string{"serve"}, "-p", "port", "9191", "--port"},
		{[]string{"project", "configure"}, "-m", "model", "some-model", "--model"},
		{[]string{"metrics"}, "-d", "period", "7d", "--period"},
		{[]string{"audit"}, "-t", "type", "quality", "--type"},
		{[]string{"review"}, "-m", "mode", "full", "--mode"},
		{[]string{"review"}, "-b", "branch", "feat/x", "--branch"},
		{[]string{"debug"}, "-i", "issue", "it breaks", "--issue"},
		{[]string{"start"}, "-w", "worktree", "feat/y", "--worktree"},
		{[]string{"start"}, "-l", "label", "bug", "--label"},
		{[]string{"session", "attach"}, "--how", "attach", "tmux", "--attach"},
	}
	for _, lang := range []string{"en", "fr"} {
		i18n.SetLocale(lang)
		for _, tc := range cases {
			c, _, err := rootCmd.Find(tc.path)
			if err != nil || c == rootCmd {
				t.Fatalf("%v: %v", tc.path, err)
			}
			target := c.Flags().Lookup(tc.target)
			reset := func() { _ = target.Value.Set(target.DefValue); target.Changed = false }

			var errOut bytes.Buffer
			c.SetErr(&errOut)
			if err := c.ParseFlags([]string{tc.old, tc.value}); err != nil {
				t.Fatalf("%v %s: %v", tc.path, tc.old, err)
			}
			if !c.Flags().Changed(tc.target) || target.Value.String() != tc.value {
				t.Errorf("%v %s: --%s = %q (changed %v), want %q", tc.path, tc.old, tc.target, target.Value.String(), target.Changed, tc.value)
			}
			warn := errOut.String()
			if !strings.Contains(warn, tc.old) || !strings.Contains(warn, tc.newForm) || !strings.Contains(warn, c.CommandPath()) {
				t.Errorf("%s %v %s: warning = %q", lang, tc.path, tc.old, warn)
			}
			if lang == "fr" && !strings.Contains(warn, "dépréciée") {
				t.Errorf("fr warning not translated: %q", warn)
			}
			reset()

			errOut.Reset()
			if err := c.ParseFlags([]string{tc.newForm, tc.value}); err != nil {
				t.Fatalf("%v %s: %v", tc.path, tc.newForm, err)
			}
			if errOut.Len() != 0 || target.Value.String() != tc.value {
				t.Errorf("%v %s: warning %q, value %q", tc.path, tc.newForm, errOut.String(), target.Value.String())
			}
			reset()
			c.SetErr(nil)

			if target.Shorthand != "" && !strings.HasPrefix(tc.old, "--") {
				t.Errorf("%v: --%s keeps the shorthand -%s", tc.path, tc.target, target.Shorthand)
			}
			if usage := c.LocalFlags().FlagUsages(); strings.Contains(usage, tc.old+", ") || strings.Contains(usage, "  "+tc.old+" ") {
				t.Errorf("%v: the help shows the deprecated %s:\n%s", tc.path, tc.old, usage)
			}
		}
	}
}

// U-03: the deprecated forms are out of the help and of the completion.
func TestDeprecatedAliasesAreHidden(t *testing.T) {
	c, _, _ := rootCmd.Find([]string{"session", "attach"})
	if f := c.Flags().Lookup("how"); f == nil || !f.Hidden {
		t.Fatal("--how must exist and be hidden")
	}
	if strings.Contains(c.LocalFlags().FlagUsages(), "--how") {
		t.Error("--how shown in the help")
	}
	if !strings.Contains(c.LocalFlags().FlagUsages(), "--attach") {
		t.Error("--attach missing from the help")
	}
}

// U-03: a bool alias works without a value.
func TestDeprecatedBoolAlias(t *testing.T) {
	c := &cobra.Command{Use: "x"}
	c.Flags().Bool("force", false, "")
	deprecateShorthand(c, "z", "force")
	var errOut bytes.Buffer
	c.SetErr(&errOut)
	if err := c.ParseFlags([]string{"-z"}); err != nil {
		t.Fatal(err)
	}
	if v, _ := c.Flags().GetBool("force"); !v || !strings.Contains(errOut.String(), "--force") {
		t.Errorf("force = %v, warning %q", v, errOut.String())
	}
}

// U-03: `--attach here` was a synonym of suspend: accepted, with a warning.
func TestAttachHereIsDeprecated(t *testing.T) {
	if got := i18n.Tf("cmd.deprecated_option.value", "here", "--attach", "suspend"); !strings.Contains(got, "suspend") {
		t.Errorf("message = %q", got)
	}
}

// forceAsYes lists the commands whose --force only skipped the confirmation
// (X9): it is now a deprecated alias of --yes, with -f where it had one.
var forceAsYes = []struct {
	path     []string
	shortcut bool
}{
	{[]string{"purge"}, false},
	{[]string{"project", "remove"}, true},
	{[]string{"service", "remove"}, true},
	{[]string{"teams", "remove"}, true},
}

// forceKept lists the --force that override a safety: they stay as they are.
var forceKept = []struct {
	path     []string
	shortcut string
}{
	{[]string{"daemon", "stop"}, ""},
	{[]string{"remote", "setup"}, ""},
	{[]string{"worktree", "remove"}, "f"},
	{[]string{"worktree", "cleanup"}, "f"},
}

// X9: --force (and -f) set --yes and warn, in the language of the hub; --yes
// itself does not warn.
func TestForceIsADeprecatedAliasOfYes(t *testing.T) {
	defer i18n.SetLocale("en")
	saved := application
	application = &app.App{}
	defer func() { application = saved }()
	for _, lang := range []string{"en", "fr"} {
		i18n.SetLocale(lang)
		for _, tc := range forceAsYes {
			c, _, err := rootCmd.Find(tc.path)
			require.NoError(t, err)
			require.NotEqual(t, rootCmd, c, tc.path)
			yes := c.Flags().Lookup("yes")
			require.NotNil(t, yes, tc.path)
			olds := []string{"--force"}
			if tc.shortcut {
				olds = append(olds, "-f")
			}
			for _, old := range olds {
				var errOut bytes.Buffer
				c.SetErr(&errOut)
				require.NoError(t, c.ParseFlags([]string{old}), "%v %s", tc.path, old)
				got, _ := c.Flags().GetBool("yes")
				assert.True(t, got && c.Flags().Changed("yes"), "%v %s sets --yes", tc.path, old)
				warn := errOut.String()
				assert.Contains(t, warn, old, "%s %v", lang, tc.path)
				assert.Contains(t, warn, "--yes", "%s %v", lang, tc.path)
				assert.Contains(t, warn, c.CommandPath(), "%s %v", lang, tc.path)
				if lang == "fr" {
					assert.Contains(t, warn, "dépréciée", "%v %s", tc.path, old)
				}
				_ = yes.Value.Set("false")
				yes.Changed = false
			}
			var errOut bytes.Buffer
			c.SetErr(&errOut)
			require.NoError(t, c.ParseFlags([]string{"--yes"}))
			assert.Empty(t, errOut.String(), "--yes does not warn: %v", tc.path)
			_ = yes.Value.Set("false")
			yes.Changed = false
			c.SetErr(nil)
		}
	}
}

// X9: the aliases are out of the help and of the completion; --yes is shown.
func TestForceAliasesAreHiddenFromTheHelp(t *testing.T) {
	for _, tc := range forceAsYes {
		c, _, err := rootCmd.Find(tc.path)
		require.NoError(t, err)
		f := c.Flags().Lookup("force")
		require.NotNil(t, f, tc.path)
		assert.True(t, f.Hidden, "--force of %v must be hidden", tc.path)
		usage := c.LocalFlags().FlagUsages()
		assert.NotContains(t, usage, "--force", tc.path)
		assert.NotContains(t, usage, "-f,", tc.path)
		assert.Contains(t, usage, "--yes", tc.path)
		if tc.shortcut {
			sh := c.Flags().ShorthandLookup("f")
			require.NotNil(t, sh, tc.path)
			assert.True(t, sh.Hidden, "-f of %v must be hidden", tc.path)
		}
	}
}

// X9: the --force that override a safety are untouched: visible, and still
// not an alias of --yes.
func TestForceThatOverridesASafetyIsKept(t *testing.T) {
	for _, tc := range forceKept {
		c, _, err := rootCmd.Find(tc.path)
		require.NoError(t, err)
		f := c.Flags().Lookup("force")
		require.NotNil(t, f, tc.path)
		assert.False(t, f.Hidden, "--force of %v stays in the help", tc.path)
		assert.Equal(t, tc.shortcut, f.Shorthand, tc.path)
		_, isAlias := f.Value.(*deprecatedAlias)
		assert.False(t, isAlias, tc.path)
		assert.Contains(t, c.LocalFlags().FlagUsages(), "--force", tc.path)

		var errOut bytes.Buffer
		c.SetErr(&errOut)
		require.NoError(t, c.ParseFlags([]string{"--force"}))
		assert.Empty(t, errOut.String(), "no warning for %v --force", tc.path)
		if y := c.Flags().Lookup("yes"); y != nil {
			assert.False(t, y.Changed, "--force of %v does not set --yes", tc.path)
		}
		_ = f.Value.Set("false")
		f.Changed = false
		c.SetErr(nil)
	}
}

// X9: without a terminal --force has the effect of --yes on teams remove and
// service remove (it used to skip the question), and warns.
func TestForceHasTheEffectOfYesWithoutTerminal(t *testing.T) {
	t.Setenv("OH_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	terminalIs(t, false)
	stubConfirmForm(t)
	newApp := func(t *testing.T) {
		t.Helper()
		io, _, _ := testIO()
		application = &app.App{
			Config:   &config.Config{Teams: []config.TeamConfig{{ID: "acme"}}},
			Projects: &mockProjectStore{projects: []domain.Project{}},
			IO:       io,
		}
		t.Cleanup(func() { application = nil })
	}
	for _, tc := range []struct {
		name string
		cmd  func() *cobra.Command
		args []string
	}{
		{"teams remove", func() *cobra.Command { return teamsRemoveCmd }, []string{"acme"}},
		{"service remove", func() *cobra.Command { return serviceRemoveCmd }, []string{"gitlab"}},
	} {
		for _, flag := range []string{"--force", "-f", "--yes"} {
			t.Run(tc.name+" "+flag, func(t *testing.T) {
				newApp(t)
				c := tc.cmd()
				reset := func() {
					for _, n := range []string{"yes", "force"} {
						_ = c.Flags().Lookup(n).Value.Set("false")
						c.Flags().Lookup(n).Changed = false
					}
				}
				reset()
				t.Cleanup(reset)
				out, err := runSub(t, c, append([]string{flag}, tc.args...)...)
				require.NoError(t, err, out)
				if flag != "--yes" {
					assert.Contains(t, out, "--yes", "the alias warns")
				}
			})
		}
		t.Run(tc.name+" without option", func(t *testing.T) {
			newApp(t)
			c := tc.cmd()
			_ = c.Flags().Lookup("yes").Value.Set("false")
			_, err := runSub(t, c, tc.args...)
			require.Error(t, err)
			assert.Equal(t, ExitUsage, ExitCode(err))
		})
	}
}
