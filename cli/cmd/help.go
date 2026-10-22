package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/datichb/openhub/cli/internal/buildinfo"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// Help styles
var (
	helpSectionStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.Primary)
	helpCmdStyle     = lipgloss.NewStyle().Foreground(theme.LipSuccess)
	helpFlagStyle    = lipgloss.NewStyle().Foreground(theme.Subtle)
	helpDescStyle    = lipgloss.NewStyle()
)

// The overview of `oh --help` is built from the command tree (QB4: a
// hand-written list named commands that no longer exist and missed new
// ones). By default it is one line per top-level command (U-01: it ran to
// 570 lines); `oh help --all` (or `oh --help --all`) shows every visible
// command, in the section of its top-level command, with its own flags.

// helpFlag describes a flag for the help display.
type helpFlag struct {
	Long  string
	Short string
	Desc  string
}

// helpCommand describes a command for the help display.
type helpCommand struct {
	Name  string
	Desc  string
	Flags []helpFlag
}

// helpSection groups commands.
type helpSection struct {
	Title    string
	Commands []helpCommand
}

// helpSectionOrder lists the sections of the overview and, for each, its
// top-level commands in display order. A visible top-level command missing
// here is shown under « other » (TestHelpSectionsCoverEveryCommand).
var helpSectionOrder = []struct {
	key      string
	commands []string
}{
	{"help.section.workflows", []string{"run", "workflow", "brick", "bundle", "skill", "eval"}},
	{"help.section.session", []string{"session", "fanout", "budget", "takeover-brief"}},
	{"help.section.project", []string{"project", "worktree", "mr", "board", "wiki"}},
	{"help.section.mcp", []string{"mcp"}},
	{"help.section.config", []string{"config", "provider", "secrets"}},
	{"help.section.analytics", []string{"status", "metrics", "dashboard", "serve", "history"}},
	{"help.section.team", []string{"team", "teams", "conventions", "beads", "policies", "patterns"}},
	{"help.section.infra", []string{"init", "doctor", "repair", "gc", "export", "import", "purge", "upgrade", "migrate", "daemon", "remote", "version", "completion"}},
}

// helpAdvanced lists the top-level commands that the compact overview groups
// under « Advanced »; `--all` leaves them in their own section.
var helpAdvanced = []string{"bundle", "fanout", "migrate", "takeover-brief", "completion"}

func init() {
	rootCmd.Flags().Bool("all", false, i18n.T("cmd.help.flags.all"))
	rootCmd.SetHelpCommand(newHelpCommand())
}

// newHelpCommand is cobra's `oh help [command]` with the option --all, which
// shows every subcommand and option of the overview.
func newHelpCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "help [command]",
		Short: i18n.T("cmd.help.short"),
		Long:  i18n.T("cmd.help.long"),
		ValidArgsFunction: func(c *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
			var completions []cobra.Completion
			cmd, _, e := c.Root().Find(args)
			if e != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			if cmd == nil {
				cmd = c.Root()
			}
			for _, sub := range cmd.Commands() {
				if sub.IsAvailableCommand() || sub.Name() == "help" {
					if strings.HasPrefix(sub.Name(), toComplete) {
						completions = append(completions, cobra.CompletionWithDesc(sub.Name(), sub.Short))
					}
				}
			}
			return completions, cobra.ShellCompDirectiveNoFileComp
		},
		Run: func(c *cobra.Command, args []string) {
			cmd, _, e := c.Root().Find(args)
			if cmd == nil || e != nil {
				c.Printf("%s\n", i18n.Tf("help.unknown_topic", strings.Join(args, " ")))
				cobra.CheckErr(c.Root().Usage())
				return
			}
			if all, _ := c.Flags().GetBool("all"); all && cmd == c.Root() {
				_ = c.Root().Flags().Set("all", "true")
			}
			cmd.InitDefaultHelpFlag()
			cobra.CheckErr(cmd.Help())
		},
	}
	c.Flags().Bool("all", false, i18n.T("cmd.help.flags.all"))
	return c
}

// customHelpFunc replaces Cobra's default help with a paged, colored, i18n-aware display.
func customHelpFunc(cmd *cobra.Command, args []string) {
	all, _ := cmd.Root().Flags().GetBool("all")
	content := buildHelpContent(cmd.Root(), all)

	// Try pager if stdout is a terminal
	if isTerminal() {
		if runPager(content) {
			return
		}
	}
	// Fallback: print directly
	fmt.Print(content)
}

// buildHelpContent constructs the help text with colors: one line per
// top-level command, or with all the subcommands and options.
func buildHelpContent(root *cobra.Command, all bool) string {
	var sb strings.Builder

	header := fmt.Sprintf("oh — OpenHub CLI %s", buildinfo.Version)
	sb.WriteString(theme.Bold.Render(header))
	sb.WriteString("\n\n")

	for _, section := range buildHelpSections(root, all) {
		sb.WriteString(helpSectionStyle.Render(section.Title))
		sb.WriteString("\n")
		if all {
			sb.WriteString("\n")
		}
		for _, cmd := range section.Commands {
			if len(cmd.Name) < 24 {
				fmt.Fprintf(&sb, "%s%s\n", helpCmdStyle.Render(fmt.Sprintf("  %-24s", cmd.Name)), cmd.Desc)
			} else {
				fmt.Fprintf(&sb, "%s\n%s%s\n", helpCmdStyle.Render("  "+cmd.Name), strings.Repeat(" ", 26), cmd.Desc)
			}
			for _, f := range cmd.Flags {
				flagStr := "--" + f.Long
				if f.Short != "" {
					flagStr += ", -" + f.Short
				}
				flagRendered := helpFlagStyle.Render(fmt.Sprintf("      %-22s", flagStr))
				fmt.Fprintf(&sb, "%s%s\n", flagRendered, f.Desc)
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString(helpSectionStyle.Render(i18n.T("help.global_flags")))
	sb.WriteString("\n")
	if all {
		sb.WriteString("\n")
	}
	var globals []string
	root.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		name := "--" + f.Name
		if f.Shorthand != "" {
			name = "-" + f.Shorthand + ", " + name
		}
		if all {
			fmt.Fprintf(&sb, "  %s  %s\n", helpFlagStyle.Render(fmt.Sprintf("%-16s", name)), f.Usage)
		} else {
			globals = append(globals, name)
		}
	})
	if all {
		fmt.Fprintf(&sb, "  %s  %s\n", helpFlagStyle.Render(fmt.Sprintf("%-16s", "-h, --help")), i18n.T("help.flag.help"))
		sb.WriteString("\n")
		sb.WriteString(helpDescStyle.Render(i18n.T("help.footer")))
	} else {
		globals = append(globals, "-h, --help")
		fmt.Fprintf(&sb, "  %s\n\n", helpFlagStyle.Render(strings.Join(globals, " · ")))
		sb.WriteString(helpDescStyle.Render(i18n.T("help.footer.all")))
	}
	sb.WriteString("\n")
	return sb.String()
}

// helpVisible reports whether a command is listed in the help.
func helpVisible(c *cobra.Command) bool {
	return !c.Hidden && c.Deprecated == "" && c.Name() != "help"
}

// buildHelpSections lists the visible commands of root by section: every
// subcommand with its options when all is set, else one line per top-level
// command, the advanced ones apart.
func buildHelpSections(root *cobra.Command, all bool) []helpSection {
	byName := map[string]*cobra.Command{}
	for _, c := range root.Commands() {
		if helpVisible(c) {
			byName[c.Name()] = c
		}
	}
	list := func(c *cobra.Command) []helpCommand {
		if all {
			return helpCommands(c, "")
		}
		return []helpCommand{{Name: c.Name(), Desc: c.Short}}
	}
	var out []helpSection
	placed := map[string]bool{}
	advanced := helpSection{Title: i18n.T("help.section.advanced")}
	for _, sec := range helpSectionOrder {
		hs := helpSection{Title: i18n.T(sec.key)}
		for _, name := range sec.commands {
			if c := byName[name]; c != nil {
				placed[name] = true
				if !all && slices.Contains(helpAdvanced, name) {
					continue
				}
				hs.Commands = append(hs.Commands, list(c)...)
			}
		}
		if len(hs.Commands) > 0 {
			out = append(out, hs)
		}
	}
	if !all {
		for _, name := range helpAdvanced {
			if c := byName[name]; c != nil {
				advanced.Commands = append(advanced.Commands, list(c)...)
			}
		}
		if len(advanced.Commands) > 0 {
			out = append(out, advanced)
		}
	}
	other := helpSection{Title: i18n.T("help.section.other")}
	for _, c := range root.Commands() {
		if helpVisible(c) && !placed[c.Name()] {
			other.Commands = append(other.Commands, list(c)...)
		}
	}
	if len(other.Commands) > 0 {
		out = append(out, other)
	}
	return out
}

// helpCommands lists c (when it runs) and its visible subcommands.
func helpCommands(c *cobra.Command, prefix string) []helpCommand {
	name := strings.TrimSpace(prefix + " " + c.Name())
	var out []helpCommand
	if c.Runnable() {
		use := name
		if _, args, ok := strings.Cut(c.Use, " "); ok {
			use += " " + args
		}
		hc := helpCommand{Name: use, Desc: c.Short}
		c.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) {
			if !f.Hidden && f.Deprecated == "" && f.Name != "help" {
				hc.Flags = append(hc.Flags, helpFlag{Long: f.Name, Short: f.Shorthand, Desc: f.Usage})
			}
		})
		out = append(out, hc)
	}
	for _, sub := range c.Commands() {
		if helpVisible(sub) {
			out = append(out, helpCommands(sub, name)...)
		}
	}
	return out
}

// isTerminal checks if stdout is connected to a terminal.
func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// runPager pipes content through the system pager.
// Returns true if pager ran successfully, false if it failed.
func runPager(content string) bool {
	pager := os.Getenv("PAGER")
	if pager == "" {
		pager = "less"
	}

	cmd := exec.Command(pager, "-R")
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run() == nil
}
