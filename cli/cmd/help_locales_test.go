package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/limits"
)

// Every command has its Short in the locales and every option its usage, the
// hidden ones too (oh daemon run, oh eval pass, --exec of oh session attach):
// TestEveryHelpIsTranslated leaves them out because the overview does not list
// them, but oh <command> --help shows them, in the language of the locale.
func TestEveryCommandAndOptionHasItsHelpInTheLocales(t *testing.T) {
	defer func() { i18n.SetLocale("en"); localizeCommands(rootCmd) }()
	for _, lang := range []string{"fr", "en"} {
		i18n.SetLocale(lang)
		localizeCommands(rootCmd)
		var walk func(c *cobra.Command)
		walk = func(c *cobra.Command) {
			if c.Name() == "help" {
				return
			}
			if c.Short == "" {
				t.Errorf("%s %s: empty help", lang, c.CommandPath())
			} else if k := i18n.KeyOf(c.Short); k == "" || i18n.T(k) != c.Short {
				t.Errorf("%s %s: help not in the locale: %q", lang, c.CommandPath(), c.Short)
			}
			flag := func(f *pflag.Flag) {
				if f.Name == "help" {
					return
				}
				if k := i18n.KeyOf(f.Usage); f.Usage == "" || k == "" || i18n.T(k) != f.Usage {
					t.Errorf("%s %s --%s: help not in the locale: %q", lang, c.CommandPath(), f.Name, f.Usage)
				}
			}
			c.Flags().VisitAll(flag)
			c.PersistentFlags().VisitAll(flag)
			for _, s := range c.Commands() {
				walk(s)
			}
		}
		walk(rootCmd)
	}
}

// The help of oh budget set lists the restrictions in the locales, since the
// code no longer builds it: a restriction added to limits.Fields must be added
// to the text.
func TestBudgetSetHelpNamesEveryRestriction(t *testing.T) {
	defer func() { i18n.SetLocale("en"); localizeCommands(rootCmd) }()
	for _, lang := range []string{"fr", "en"} {
		i18n.SetLocale(lang)
		localizeCommands(rootCmd)
		c, _, err := rootCmd.Find([]string{"budget", "set"})
		require.NoError(t, err)
		for _, field := range limits.Fields {
			require.Contains(t, c.Short, field, "%s: oh budget set --help", lang)
		}
	}
}
