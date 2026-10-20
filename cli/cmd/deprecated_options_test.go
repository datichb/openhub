package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/datichb/openhub/cli/internal/app"
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
