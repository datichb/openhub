package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
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
	"m": {"message"},
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
			if f.Shorthand == "" || f.Name == "help" || f.Hidden {
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

// 5.1: the options deprecated in 5.0 are gone, they are plain unknown
// options (the long forms remain).
func TestRemovedOptionsAreUnknown(t *testing.T) {
	cases := []struct {
		path []string
		args []string
		kept string // the long form that remains
	}{
		{[]string{"serve"}, []string{"-p", "9191"}, "port"},
		{[]string{"metrics"}, []string{"-d", "7d"}, "period"},
		{[]string{"project", "configure"}, []string{"-m", "some-model"}, "model"},
		{[]string{"session", "attach"}, []string{"--how", "tmux"}, "attach"},
		{[]string{"repair"}, []string{"--auto"}, "yes"},
		{[]string{"purge"}, []string{"--force"}, "yes"},
		{[]string{"project", "remove"}, []string{"--force"}, "yes"},
		{[]string{"project", "remove"}, []string{"-f"}, "yes"},
		{[]string{"teams", "remove"}, []string{"--force"}, "yes"},
		{[]string{"teams", "remove"}, []string{"-f"}, "yes"},
	}
	for _, tc := range cases {
		c, _, err := rootCmd.Find(tc.path)
		require.NoError(t, err)
		require.NotEqual(t, rootCmd, c, tc.path)
		err = c.ParseFlags(tc.args)
		require.Error(t, err, "%v %v", tc.path, tc.args)
		assert.Contains(t, err.Error(), "unknown", "%v %v", tc.path, tc.args)
		assert.NotNil(t, c.Flags().Lookup(tc.kept), "%v keeps --%s", tc.path, tc.kept)
	}
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

func TestForceThatOverridesASafetyIsKept(t *testing.T) {
	for _, tc := range forceKept {
		c, _, err := rootCmd.Find(tc.path)
		require.NoError(t, err)
		f := c.Flags().Lookup("force")
		require.NotNil(t, f, tc.path)
		assert.False(t, f.Hidden, "--force of %v stays in the help", tc.path)
		assert.Equal(t, tc.shortcut, f.Shorthand, tc.path)
		assert.Contains(t, c.LocalFlags().FlagUsages(), "--force", tc.path)
		require.NoError(t, c.ParseFlags([]string{"--force"}))
		if y := c.Flags().Lookup("yes"); y != nil {
			assert.False(t, y.Changed, "--force of %v does not set --yes", tc.path)
		}
		_ = f.Value.Set("false")
		f.Changed = false
	}
}

// Without a terminal, teams remove is refused without --yes and goes through with it.
func TestTeamsRemoveNeedsYesWithoutTerminal(t *testing.T) {
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
	reset := func() {
		_ = teamsRemoveCmd.Flags().Lookup("yes").Value.Set("false")
		teamsRemoveCmd.Flags().Lookup("yes").Changed = false
	}
	t.Run("--yes", func(t *testing.T) {
		newApp(t)
		reset()
		t.Cleanup(reset)
		out, err := runSub(t, teamsRemoveCmd, "--yes", "acme")
		require.NoError(t, err, out)
	})
	t.Run("without option", func(t *testing.T) {
		newApp(t)
		reset()
		_, err := runSub(t, teamsRemoveCmd, "acme")
		require.Error(t, err)
		assert.Equal(t, ExitUsage, ExitCode(err))
	})
}
