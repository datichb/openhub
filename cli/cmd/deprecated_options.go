package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// One letter, one meaning (U-03). When a command used a letter for another
// meaning, the old form stays accepted for one version as a deprecated
// alias: hidden from the help, with a translated warning when it is used.
// The alias is a second, hidden flag that writes into the real one (pflag's
// own MarkShorthandDeprecated prints an English sentence).

// deprecatedAlias is the value of an alias flag: it sets the target flag and
// warns on the error output of the command.
type deprecatedAlias struct {
	cmd    *cobra.Command
	target *pflag.Flag
	old    string // "-p" or "--how"
}

func (d *deprecatedAlias) String() string { return d.target.Value.String() }
func (d *deprecatedAlias) Type() string   { return d.target.Value.Type() }

func (d *deprecatedAlias) Set(v string) error {
	if err := d.target.Value.Set(v); err != nil {
		return err
	}
	d.target.Changed = true
	warnDeprecatedOption(d.cmd, i18n.Tf("cmd.deprecated_option.warn", d.old, d.cmd.CommandPath(), "--"+d.target.Name))
	return nil
}

// warnDeprecatedOption prints a warning of a deprecated option or value.
func warnDeprecatedOption(c *cobra.Command, msg string) {
	ensureLocale()
	fmt.Fprintf(c.ErrOrStderr(), "%s %s\n", theme.WarningStyle.Render(theme.IconWarning), msg)
}

// addDeprecatedAlias registers the hidden alias flag `name` (and the
// shorthand `short`, possibly empty) of the flag target of c.
func addDeprecatedAlias(c *cobra.Command, name, short, target string) *deprecatedAlias {
	t := c.Flags().Lookup(target)
	if t == nil {
		panic("deprecated option: unknown flag --" + target + " on " + c.CommandPath())
	}
	old := "--" + name
	if short != "" {
		old = "-" + short
	}
	d := &deprecatedAlias{cmd: c, target: t, old: old}
	f := c.Flags().VarPF(d, name, short, "")
	f.Hidden = true
	f.NoOptDefVal = t.NoOptDefVal
	return d
}

// deprecateShorthand moves the shorthand letter of the flag target to a
// deprecated alias: `-p` still sets --port, with a warning, and the help no
// longer shows it. The flag must have been defined without the shorthand.
func deprecateShorthand(c *cobra.Command, letter, target string) {
	addDeprecatedAlias(c, letter, letter, target)
}

// deprecateFlag keeps the former flag name `name` as a hidden alias of target.
func deprecateFlag(c *cobra.Command, name, target string) *deprecatedAlias {
	return addDeprecatedAlias(c, name, "", target)
}

// silenceCobraDeprecation stops cobra from printing its own English sentence
// (« Command "x" is deprecated, … ») for the deprecated command about to
// run: the command warns in the language of the hub, with the exact
// replacement (warnDeprecatedAlias). The command stays marked deprecated for
// the help; the returned function restores the message.
func silenceCobraDeprecation(root *cobra.Command, args []string) (restore func()) {
	c, _, err := root.Find(args)
	if err != nil || c == nil || c.Deprecated == "" {
		return func() {}
	}
	msg := c.Deprecated
	c.Deprecated = ""
	return func() { c.Deprecated = msg }
}
