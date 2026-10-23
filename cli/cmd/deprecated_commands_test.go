package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/launch"
)

// deprecatedCommands are the deprecated aliases of oh run and the command
// that replaces each (U-02).
var deprecatedCommands = []struct {
	path        []string
	replacement string
}{
	{[]string{"start"}, "oh run"},
	{[]string{"audit"}, "oh run audit"},
	{[]string{"review"}, "oh run review"},
	{[]string{"review", "feedback"}, "oh run review-feedback"},
	{[]string{"debug"}, "oh run debug"},
	{[]string{"takeover-brief", "enrich"}, "oh run brief-enrich"},
}

// U-02: the aliases carry the Deprecated field of cobra, with a message in
// the language of the hub that names the replacement.
func TestAliasesAreMarkedDeprecated(t *testing.T) {
	defer func() { i18n.SetLocale("en"); localizeCommands(rootCmd) }()
	for _, lang := range []string{"en", "fr"} {
		i18n.SetLocale(lang)
		localizeCommands(rootCmd)
		for _, tc := range deprecatedCommands {
			c, _, err := rootCmd.Find(tc.path)
			if err != nil || c.Name() != tc.path[len(tc.path)-1] {
				t.Fatalf("%v: %v", tc.path, err)
			}
			if c.Deprecated == "" {
				t.Errorf("%v is not marked Deprecated", tc.path)
				continue
			}
			if !strings.Contains(c.Deprecated, tc.replacement) {
				t.Errorf("%s %v: message %q does not name %q", lang, tc.path, c.Deprecated, tc.replacement)
			}
			if lang == "fr" && !strings.Contains(c.Deprecated, "utilisez") {
				t.Errorf("fr %v: message not translated: %q", tc.path, c.Deprecated)
			}
			if lang == "en" && !strings.Contains(c.Deprecated, "use ") {
				t.Errorf("en %v: message not in English: %q", tc.path, c.Deprecated)
			}
		}
	}
}

// U-02: the deprecated commands are out of the main help (and of the
// section order), the replacement of the publication is in.
func TestDeprecatedCommandsAreOutOfTheMainHelp(t *testing.T) {
	listed := map[string]bool{}
	for _, sec := range buildHelpSections(rootCmd, true) {
		for _, c := range sec.Commands {
			name, _, _ := strings.Cut(c.Name, " ")
			listed[name] = true
			listed[c.Name] = true
		}
	}
	for _, name := range []string{"start", "audit", "review", "debug", "review feedback", "takeover-brief enrich"} {
		if listed[name] {
			t.Errorf("%s is listed in oh --help", name)
		}
	}
	if !listed["run"] || !listed["mr"] && !listed["mr create"] {
		t.Errorf("run and mr create must be listed: %v", listed)
	}
	for _, sec := range helpSectionOrder {
		for _, name := range sec.commands {
			if name == "start" || name == "audit" || name == "review" || name == "debug" {
				t.Errorf("%s is still in helpSectionOrder", name)
			}
		}
	}
}

// U-02: cobra's own English sentence is replaced by the translated warning of
// the command; the deprecation stays for the help, and comes back after the run.
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
	if err := root.Execute(); err != nil || !ran {
		t.Fatalf("the deprecated command must run: %v", err)
	}
	if strings.Contains(out.String(), "deprecated") {
		t.Errorf("cobra sentence printed: %q", out.String())
	}
	restore()
	if old.Deprecated != "use oh new" {
		t.Errorf("Deprecated = %q after the run", old.Deprecated)
	}

	// Without the silencing, cobra prints its sentence (what the test guards against).
	out.Reset()
	if err := root.Execute(); err != nil || !strings.Contains(out.String(), "deprecated") {
		t.Errorf("control: %q %v", out.String(), err)
	}
	// A command that is not deprecated is left alone.
	if silenceCobraDeprecation(root, []string{"nope"}); old.Deprecated == "" {
		t.Error("another command was changed")
	}
}

// U-02: the warning of an alias names the replacement, in both languages.
func TestAliasWarningNamesTheReplacement(t *testing.T) {
	defer i18n.SetLocale("en")
	cases := []struct {
		al   workflowAlias
		want string
	}{
		{workflowAlias{Old: "oh audit", Workflow: "audit", Opts: launch.Options{LooseInputs: map[string]string{"type": "security"}}}, "oh run audit -i type=security"},
		{workflowAlias{Old: "oh review feedback", Workflow: "review-feedback", Opts: launch.Options{LooseInputs: map[string]string{"mr": "https://x/mr/1"}}}, "oh run review-feedback -i mr="},
		{workflowAlias{Old: "oh debug", Workflow: "debug", Opts: launch.Options{LooseInputs: map[string]string{"issue": "boom"}}}, "oh run debug -i issue=boom"},
	}
	for _, lang := range []string{"en", "fr"} {
		i18n.SetLocale(lang)
		for _, tc := range cases {
			var out bytes.Buffer
			warnDeprecatedAlias(&out, tc.al.Old, aliasReplacement(tc.al))
			if !strings.Contains(out.String(), tc.al.Old) || !strings.Contains(out.String(), tc.want) {
				t.Errorf("%s: %q", lang, out.String())
			}
			if lang == "fr" && !strings.Contains(out.String(), "alias déprécié") {
				t.Errorf("fr warning not translated: %q", out.String())
			}
		}
	}
}

// U-02: oh mr create replaces oh review --publish and shares its code.
func TestMRCreateReplacesReviewPublish(t *testing.T) {
	c, _, err := rootCmd.Find([]string{"mr", "create"})
	if err != nil || c.Name() != "create" {
		t.Fatalf("oh mr create: %v", err)
	}
	for _, flag := range []string{"project", "reviewer"} {
		if c.Flags().Lookup(flag) == nil {
			t.Errorf("--%s missing from oh mr create", flag)
		}
	}
	if c.Deprecated != "" || !helpVisible(c) {
		t.Error("oh mr create must be a normal command")
	}
}
