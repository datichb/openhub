package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

// Help styles
var (
	helpSectionStyle = lipgloss.NewStyle().Bold(true).Foreground(common.Primary)
	helpCmdStyle     = lipgloss.NewStyle().Foreground(common.Success)
	helpFlagStyle    = lipgloss.NewStyle().Foreground(common.Subtle)
	helpDescStyle    = lipgloss.NewStyle()
)

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

// customHelpFunc replaces Cobra's default help with a paged, colored, i18n-aware display.
func customHelpFunc(cmd *cobra.Command, args []string) {
	content := buildHelpContent()

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
func buildHelpContent() string {
	var sb strings.Builder

	// Header
	header := fmt.Sprintf("oh — OpenHub CLI %s", Version)
	sb.WriteString(common.Bold.Render(header))
	sb.WriteString("\n\n")

	// Build sections
	sections := buildHelpSections()

	for _, section := range sections {
		sb.WriteString(helpSectionStyle.Render(section.Title))
		sb.WriteString("\n\n")

		for _, cmd := range section.Commands {
			// Command line: "  start             Description"
			cmdName := helpCmdStyle.Render(fmt.Sprintf("  %-18s", cmd.Name))
			sb.WriteString(fmt.Sprintf("%s%s\n", cmdName, cmd.Desc))

			// Flags
			for _, f := range cmd.Flags {
				flagStr := ""
				if f.Short != "" {
					flagStr = fmt.Sprintf("--%s, -%s", f.Long, f.Short)
				} else {
					flagStr = fmt.Sprintf("--%s", f.Long)
				}
				flagRendered := helpFlagStyle.Render(fmt.Sprintf("      %-16s", flagStr))
				sb.WriteString(fmt.Sprintf("%s%s\n", flagRendered, f.Desc))
			}
		}
		sb.WriteString("\n")
	}

	// Global flags
	sb.WriteString(helpSectionStyle.Render(i18n.T("help.global_flags")))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("  %s  %s\n", helpFlagStyle.Render("-v, --verbose"), i18n.T("help.flag.verbose")))
	sb.WriteString(fmt.Sprintf("  %s  %s\n", helpFlagStyle.Render("-h, --help   "), i18n.T("help.flag.help")))
	sb.WriteString("\n")

	// Footer
	sb.WriteString(helpDescStyle.Render(i18n.T("help.footer")))
	sb.WriteString("\n")

	return sb.String()
}

// buildHelpSections returns the structured help content using i18n keys.
func buildHelpSections() []helpSection {
	return []helpSection{
		{
			Title: i18n.T("help.section.session"),
			Commands: []helpCommand{
				{
					Name: "start",
					Desc: i18n.T("cmd.start.short"),
					Flags: []helpFlag{
						{"agent", "a", i18n.T("help.flag.start.agent")},
						{"prompt", "p", i18n.T("help.flag.start.prompt")},
						{"provider", "P", i18n.T("help.flag.start.provider")},
						{"project", "j", i18n.T("help.flag.start.project")},
						{"resume", "r", i18n.T("help.flag.start.resume")},
						{"worktree", "w", i18n.T("help.flag.start.worktree")},
						{"dev", "", i18n.T("help.flag.start.dev")},
						{"label", "l", i18n.T("help.flag.start.label")},
						{"assignee", "A", i18n.T("help.flag.start.assignee")},
						{"onboard", "", i18n.T("help.flag.start.onboard")},
						{"refresh", "", i18n.T("help.flag.start.refresh")},
						{"yes", "y", i18n.T("help.flag.start.yes")},
					},
				},
				{
					Name: "quick",
					Desc: i18n.T("cmd.quick.short"),
					Flags: []helpFlag{
						{"project", "j", i18n.T("help.flag.start.project")},
					},
				},
				{
					Name: "audit",
					Desc: i18n.T("cmd.audit.short"),
					Flags: []helpFlag{
						{"type", "t", i18n.T("help.flag.audit.type")},
						{"project", "j", i18n.T("help.flag.start.project")},
					},
				},
				{
					Name: "review",
					Desc: i18n.T("cmd.review.short"),
					Flags: []helpFlag{
						{"project", "j", i18n.T("help.flag.start.project")},
					},
				},
				{
					Name: "debug",
					Desc: i18n.T("cmd.debug.short"),
					Flags: []helpFlag{
						{"issue", "i", i18n.T("help.flag.debug.issue")},
						{"project", "j", i18n.T("help.flag.start.project")},
					},
				},
				{
					Name: "beads",
					Desc: i18n.T("cmd.beads.short"),
				},
			},
		},
		{
			Title: i18n.T("help.section.project"),
			Commands: []helpCommand{
				{Name: "init", Desc: i18n.T("cmd.init.short")},
				{
					Name: "project list",
					Desc: i18n.T("cmd.project.list.short"),
					Flags: []helpFlag{
						{"status", "s", i18n.T("help.flag.project.status")},
						{"json", "", i18n.T("help.flag.json")},
					},
				},
				{
					Name: "project add",
					Desc: i18n.T("cmd.project.add.short"),
					Flags: []helpFlag{
						{"name", "n", i18n.T("help.flag.project.name")},
						{"path", "p", i18n.T("help.flag.project.path")},
						{"language", "l", i18n.T("help.flag.project.language")},
						{"tracker", "t", i18n.T("help.flag.project.tracker")},
					},
				},
				{
					Name: "project remove",
					Desc: i18n.T("cmd.project.remove.short"),
					Flags: []helpFlag{
						{"force", "f", i18n.T("help.flag.force")},
					},
				},
				{Name: "project rename", Desc: i18n.T("cmd.project.rename.short")},
				{Name: "project move", Desc: i18n.T("cmd.project.move.short")},
				{
					Name: "project configure",
					Desc: i18n.T("cmd.project.configure.short"),
					Flags: []helpFlag{
						{"provider", "P", i18n.T("help.flag.start.provider")},
						{"model", "m", i18n.T("help.flag.deploy.model")},
						{"language", "l", i18n.T("help.flag.project.language")},
						{"tracker", "t", i18n.T("help.flag.project.tracker")},
					},
				},
			},
		},
		{
			Title: i18n.T("help.section.deploy"),
			Commands: []helpCommand{
				{
					Name: "deploy",
					Desc: i18n.T("cmd.deploy.short"),
					Flags: []helpFlag{
						{"project", "j", i18n.T("help.flag.start.project")},
						{"provider", "P", i18n.T("help.flag.start.provider")},
						{"model", "m", i18n.T("help.flag.deploy.model")},
						{"check", "", i18n.T("help.flag.deploy.check")},
						{"diff", "", i18n.T("help.flag.deploy.diff")},
					},
				},
				{
					Name: "sync",
					Desc: i18n.T("cmd.sync.short"),
					Flags: []helpFlag{
						{"project", "j", i18n.T("help.flag.start.project")},
						{"all", "", i18n.T("help.flag.sync.all")},
						{"dry-run", "", i18n.T("help.flag.sync.dryrun")},
					},
				},
			},
		},
		{
			Title: i18n.T("help.section.config"),
			Commands: []helpCommand{
				{Name: "config get", Desc: i18n.T("cmd.config.get.short")},
				{Name: "config set", Desc: i18n.T("cmd.config.set.short")},
				{Name: "config unset", Desc: i18n.T("cmd.config.unset.short")},
				{
					Name: "config list",
					Desc: i18n.T("cmd.config.list.short"),
					Flags: []helpFlag{
						{"json", "", i18n.T("help.flag.json")},
					},
				},
				{Name: "config path", Desc: i18n.T("cmd.config.path.short")},
				{Name: "config language", Desc: i18n.T("cmd.config.language.short")},
				{Name: "config websearch", Desc: i18n.T("cmd.config.websearch.short")},
				{Name: "service", Desc: i18n.T("cmd.service.short")},
				{Name: "service setup", Desc: i18n.T("cmd.service.setup.short")},
				{
					Name: "service remove",
					Desc: i18n.T("cmd.service.remove.short"),
					Flags: []helpFlag{
						{"force", "f", i18n.T("help.flag.force")},
					},
				},
				{Name: "plugin list", Desc: i18n.T("cmd.plugin.list.short")},
				{Name: "plugin install", Desc: i18n.T("cmd.plugin.install.short")},
				{
					Name: "plugin remove",
					Desc: i18n.T("cmd.plugin.remove.short"),
					Flags: []helpFlag{
						{"force", "f", i18n.T("help.flag.force")},
					},
				},
				{Name: "plugin status", Desc: i18n.T("cmd.plugin.status.short")},
			},
		},
		{
			Title: i18n.T("help.section.analytics"),
			Commands: []helpCommand{
				{
					Name: "status",
					Desc: i18n.T("cmd.status.short"),
					Flags: []helpFlag{
						{"json", "", i18n.T("help.flag.json")},
					},
				},
				{
					Name: "metrics",
					Desc: i18n.T("cmd.metrics.short"),
					Flags: []helpFlag{
						{"period", "p", i18n.T("help.flag.metrics.period")},
					},
				},
				{Name: "dashboard", Desc: i18n.T("cmd.dashboard.short")},
				{
					Name: "board",
					Desc: i18n.T("cmd.board.short"),
					Flags: []helpFlag{
						{"watch", "", i18n.T("help.flag.board.watch")},
					},
				},
			},
		},
		{
			Title: i18n.T("help.section.infra"),
			Commands: []helpCommand{
				{Name: "doctor", Desc: i18n.T("cmd.doctor.short")},
				{Name: "upgrade opencode", Desc: i18n.T("cmd.upgrade.short")},
				{Name: "version", Desc: i18n.T("cmd.version.short")},
				{
					Name: "worktree list",
					Desc: i18n.T("cmd.worktree.list.short"),
					Flags: []helpFlag{
						{"json", "", i18n.T("help.flag.json")},
					},
				},
				{Name: "worktree add", Desc: i18n.T("cmd.worktree.add.short")},
				{
					Name: "worktree remove",
					Desc: i18n.T("cmd.worktree.remove.short"),
					Flags: []helpFlag{
						{"force", "f", i18n.T("help.flag.force")},
					},
				},
				{
					Name: "worktree cleanup",
					Desc: i18n.T("cmd.worktree.cleanup.short"),
					Flags: []helpFlag{
						{"base", "b", i18n.T("help.flag.worktree.base")},
						{"force", "f", i18n.T("help.flag.force")},
					},
				},
				{Name: "mcp serve", Desc: i18n.T("cmd.mcp.serve.short")},
				{
					Name: "mcp list",
					Desc: i18n.T("cmd.mcp.list.short"),
					Flags: []helpFlag{
						{"json", "", i18n.T("help.flag.json")},
					},
				},
				{Name: "completion", Desc: i18n.T("cmd.completion.short")},
			},
		},
	}
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
