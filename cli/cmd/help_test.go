package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// QB4: every help (command Short and Long, flag usage) is translated, in a
// single language per locale: it was half French (texts written in the
// code), half English (i18n.T run before the locale was known).
func TestEveryHelpIsTranslated(t *testing.T) {
	defer func() { i18n.SetLocale("en"); localizeCommands(rootCmd) }()
	for _, lang := range []string{"fr", "en"} {
		i18n.SetLocale(lang)
		localizeCommands(rootCmd)
		check := func(where, text string, required bool) {
			if text == "" {
				if required {
					t.Errorf("%s %s: empty help", lang, where)
				}
				return
			}
			if k := i18n.KeyOf(text); k == "" || i18n.T(k) != text {
				t.Errorf("%s %s: help not translated: %q", lang, where, text)
			}
		}
		var walk func(c *cobra.Command)
		walk = func(c *cobra.Command) {
			if !helpVisible(c) {
				return
			}
			check(c.CommandPath(), c.Short, true)
			check(c.CommandPath()+" (long)", c.Long, false)
			flag := func(f *pflag.Flag) {
				if !f.Hidden && f.Name != "help" {
					check(c.CommandPath()+" --"+f.Name, f.Usage, true)
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

// QB4: oh --help lists every visible command, from the command tree (it
// named claim and release at top level, which do not exist, and missed
// session, budget, daemon, remote, bundle build…), each in a section.
func TestHelpSectionsCoverEveryCommand(t *testing.T) {
	listed := map[string]bool{}
	for _, sec := range buildHelpSections(rootCmd) {
		if sec.Title == i18n.T("help.section.other") {
			for _, c := range sec.Commands {
				t.Errorf("%s has no help section (helpSectionOrder)", c.Name)
			}
		}
		for _, c := range sec.Commands {
			name, _, _ := strings.Cut(c.Name, " <")
			name, _, _ = strings.Cut(name, " [")
			listed[name] = true
		}
	}
	var walk func(c *cobra.Command, prefix string)
	walk = func(c *cobra.Command, prefix string) {
		name := strings.TrimSpace(prefix + " " + c.Name())
		if c.Runnable() && !listed[name] {
			t.Errorf("%s is missing from oh --help", name)
		}
		for _, s := range c.Commands() {
			if helpVisible(s) {
				walk(s, name)
			}
		}
	}
	for _, c := range rootCmd.Commands() {
		if helpVisible(c) {
			walk(c, "")
		}
	}
	for _, sec := range helpSectionOrder {
		for _, name := range sec.commands {
			if c, _, err := rootCmd.Find([]string{name}); err != nil || c == rootCmd {
				t.Errorf("help section %s names %s, which is not a command", sec.key, name)
			}
		}
	}
	for _, gone := range []string{"claim", "release"} {
		if listed[gone] {
			t.Errorf("%s is not a top-level command", gone)
		}
	}
	for _, want := range []string{"session list", "budget show", "daemon status", "remote setup", "bundle build", "team claim", "migrate deploy-cleanup"} {
		if !listed[want] {
			t.Errorf("oh --help misses %s", want)
		}
	}
}

// QB4: the headings, the help flag and the flag defaults of a command help
// are in the locale too (cobra writes them in English).
func TestCommandHelpTemplateIsLocalized(t *testing.T) {
	defer func() { i18n.SetLocale("en"); localizeCommands(rootCmd) }()
	i18n.SetLocale("fr")
	localizeCommands(rootCmd)
	c, _, err := rootCmd.Find([]string{"run"})
	if err != nil {
		t.Fatal(err)
	}
	out := c.UsageString()
	for _, en := range []string{"Usage:", "Flags:", "Global Flags:", "help for", "(default "} {
		if strings.Contains(out, en) {
			t.Errorf("oh run --help (fr) contains %q:\n%s", en, out)
		}
	}
	if !strings.Contains(out, i18n.T("help.usage.flags")) {
		t.Errorf("no localized heading:\n%s", out)
	}
}

// QB4: the command lines (Use) hold no French placeholder, and `oh config
// model` takes -p for --project like every other command (it was -j).
func TestHelpUseLinesAndProjectShorthand(t *testing.T) {
	french := []string{"valeur", "montant", "fichier", "clé", "fournisseur", "modèle", "consigne"}
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, w := range french {
			if strings.Contains(c.Use, w) {
				t.Errorf("%s: Use %q is not language-neutral", c.CommandPath(), c.Use)
			}
		}
		if f := c.LocalFlags().Lookup("project"); f != nil && f.Shorthand != "" && f.Shorthand != "p" {
			t.Errorf("%s: --project has shorthand -%s (want -p)", c.CommandPath(), f.Shorthand)
		}
		for _, s := range c.Commands() {
			walk(s)
		}
	}
	walk(rootCmd)
}
