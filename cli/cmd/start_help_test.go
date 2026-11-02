package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// oh start is hidden, so TestEveryHelpIsTranslated skips it: its help, its
// deprecation message and the notice of each deprecated flag are checked
// here, in both locales (the « Deprecated » help of --yes was French only).
func TestStartDeprecationIsTranslated(t *testing.T) {
	defer func() { i18n.SetLocale("en"); localizeCommands(rootCmd) }()
	for _, lang := range []string{"fr", "en"} {
		i18n.SetLocale(lang)
		root := &cobra.Command{Use: "oh"}
		c := &cobra.Command{Use: "start", Deprecated: i18n.T("cmd.start.deprecated")}
		root.AddCommand(c)
		addStartFlags(c)
		localizeCommands(root)

		translated := func(where, text string) {
			if k := i18n.KeyOf(text); k == "" || i18n.T(k) != text {
				t.Errorf("%s %s: not translated: %q", lang, where, text)
			}
		}
		translated("deprecation message", c.Deprecated)
		c.Flags().VisitAll(func(f *pflag.Flag) {
			if f.Name == "help" || f.Usage == "" { // the hidden aliases of -l and -w have none
				return
			}
			translated("--"+f.Name+" usage", f.Usage)
			if f.Deprecated != "" {
				translated("--"+f.Name+" deprecation", f.Deprecated)
			}
		})
	}
}

// A free launch is `oh run libre`: the deprecation message and the long help
// of oh start no longer send it to the feature workflow.
func TestStartDeprecationNamesTheFreeLaunch(t *testing.T) {
	defer i18n.SetLocale("en")
	for _, lang := range []string{"fr", "en"} {
		i18n.SetLocale(lang)
		msg := i18n.T("cmd.start.deprecated")
		if !strings.Contains(msg, "`oh run libre`") || strings.Contains(msg, "oh run feature") {
			t.Errorf("%s: %q must name `oh run libre` for a free launch", lang, msg)
		}
		if long := i18n.T("cmd.start.long"); !strings.Contains(long, "oh run libre") {
			t.Errorf("%s: the long help must name oh run libre: %q", lang, long)
		}
	}
}
