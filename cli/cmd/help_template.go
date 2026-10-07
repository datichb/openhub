package cmd

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// The help of a command (`oh <command> --help`) is cobra's, with its
// headings, the help flag and the flag defaults in the current locale
// (QB4: one language per locale).

const localizedUsageTemplate = `{{tr "help.usage.usage"}}{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} {{tr "help.usage.command"}}{{end}}{{if gt (len .Aliases) 0}}

{{tr "help.usage.aliases"}}
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

{{tr "help.usage.examples"}}
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

{{tr "help.usage.commands"}}{{range .Commands}}{{if .IsAvailableCommand}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

{{tr "help.usage.flags"}}
{{flagUsages .LocalFlags | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

{{tr "help.usage.global_flags"}}
{{flagUsages .InheritedFlags | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableSubCommands}}

{{trf "help.usage.more" .CommandPath}}{{end}}
`

func init() {
	cobra.AddTemplateFunc("tr", i18n.T)
	cobra.AddTemplateFunc("trf", i18n.Tf)
	cobra.AddTemplateFunc("flagUsages", localizedFlagUsages)
	rootCmd.SetUsageTemplate(localizedUsageTemplate)
}

// localizedFlagUsages renders the flags with their default value labelled
// in the current locale.
func localizedFlagUsages(fs *pflag.FlagSet) string {
	return strings.ReplaceAll(fs.FlagUsages(), " (default ", " ("+i18n.T("help.usage.default")+" ")
}

// localizeHelpFlag translates the usage of the help flag of c.
func localizeHelpFlag(c *cobra.Command) {
	c.InitDefaultHelpFlag()
	if f := c.Flags().Lookup("help"); f != nil {
		f.Usage = i18n.Tf("help.usage.help_for", c.Name())
	}
}
