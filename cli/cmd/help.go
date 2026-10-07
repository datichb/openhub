package cmd

import (
	"fmt"
	"os"
	"os/exec"
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
// ones): every visible command, in the section of its top-level command,
// with its own flags.

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
	{"help.section.workflows", []string{"run", "workflow", "bundle", "skill"}},
	{"help.section.session", []string{"session", "budget", "start", "audit", "review", "debug", "takeover-brief"}},
	{"help.section.project", []string{"project", "worktree", "board"}},
	{"help.section.mcp", []string{"mcp"}},
	{"help.section.config", []string{"config", "provider", "secrets"}},
	{"help.section.analytics", []string{"status", "metrics", "dashboard", "serve", "history"}},
	{"help.section.team", []string{"team", "teams", "conventions", "beads", "policies", "patterns"}},
	{"help.section.infra", []string{"init", "doctor", "repair", "export", "import", "purge", "upgrade", "migrate", "daemon", "remote", "version", "completion"}},
}

// customHelpFunc replaces Cobra's default help with a paged, colored, i18n-aware display.
func customHelpFunc(cmd *cobra.Command, args []string) {
	content := buildHelpContent(cmd.Root())

	// Try pager if stdout is a terminal
	if isTerminal() {
		if runPager(content) {
			return
		}
	}
	// Fallback: print directly
	fmt.Print(content)
}

// buildHelpContent constructs the full help text with colors.
func buildHelpContent(root *cobra.Command) string {
	var sb strings.Builder

	header := fmt.Sprintf("oh — OpenHub CLI %s", buildinfo.Version)
	sb.WriteString(theme.Bold.Render(header))
	sb.WriteString("\n\n")

	for _, section := range buildHelpSections(root) {
		sb.WriteString(helpSectionStyle.Render(section.Title))
		sb.WriteString("\n\n")
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
	sb.WriteString("\n\n")
	root.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		name := "--" + f.Name
		if f.Shorthand != "" {
			name = "-" + f.Shorthand + ", " + name
		}
		fmt.Fprintf(&sb, "  %s  %s\n", helpFlagStyle.Render(fmt.Sprintf("%-16s", name)), f.Usage)
	})
	fmt.Fprintf(&sb, "  %s  %s\n", helpFlagStyle.Render(fmt.Sprintf("%-16s", "-h, --help")), i18n.T("help.flag.help"))
	sb.WriteString("\n")

	sb.WriteString(helpDescStyle.Render(i18n.T("help.footer")))
	sb.WriteString("\n")
	return sb.String()
}

// helpVisible reports whether a command is listed in the help.
func helpVisible(c *cobra.Command) bool {
	return !c.Hidden && c.Deprecated == "" && c.Name() != "help"
}

// buildHelpSections lists the visible commands of root by section.
func buildHelpSections(root *cobra.Command) []helpSection {
	byName := map[string]*cobra.Command{}
	for _, c := range root.Commands() {
		if helpVisible(c) {
			byName[c.Name()] = c
		}
	}
	var out []helpSection
	placed := map[string]bool{}
	for _, sec := range helpSectionOrder {
		hs := helpSection{Title: i18n.T(sec.key)}
		for _, name := range sec.commands {
			if c := byName[name]; c != nil {
				hs.Commands = append(hs.Commands, helpCommands(c, "")...)
				placed[name] = true
			}
		}
		if len(hs.Commands) > 0 {
			out = append(out, hs)
		}
	}
	other := helpSection{Title: i18n.T("help.section.other")}
	for _, c := range root.Commands() {
		if helpVisible(c) && !placed[c.Name()] {
			other.Commands = append(other.Commands, helpCommands(c, "")...)
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
