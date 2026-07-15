package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/buildinfo"
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
	header := fmt.Sprintf("oh — OpenHub CLI %s", buildinfo.Version)
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
			fmt.Fprintf(&sb, "%s%s\n", cmdName, cmd.Desc)

			// Flags
			for _, f := range cmd.Flags {
				flagStr := ""
				if f.Short != "" {
					flagStr = fmt.Sprintf("--%s, -%s", f.Long, f.Short)
				} else {
					flagStr = fmt.Sprintf("--%s", f.Long)
				}
				flagRendered := helpFlagStyle.Render(fmt.Sprintf("      %-16s", flagStr))
				fmt.Fprintf(&sb, "%s%s\n", flagRendered, f.Desc)
			}
		}
		sb.WriteString("\n")
	}

	// Global flags
	sb.WriteString(helpSectionStyle.Render(i18n.T("help.global_flags")))
	sb.WriteString("\n\n")
	fmt.Fprintf(&sb, "  %s  %s\n", helpFlagStyle.Render("-v, --verbose"), i18n.T("help.flag.verbose"))
	fmt.Fprintf(&sb, "  %s  %s\n", helpFlagStyle.Render("-h, --help   "), i18n.T("help.flag.help"))
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
						{"prompt", "m", i18n.T("help.flag.start.prompt")},
						{"provider", "P", i18n.T("help.flag.start.provider")},
						{"project", "p", i18n.T("help.flag.start.project")},
						{"resume", "r", i18n.T("help.flag.start.resume")},
						{"worktree", "w", i18n.T("help.flag.start.worktree")},
						{"dev", "", i18n.T("help.flag.start.dev")},
						{"label", "l", i18n.T("help.flag.start.label")},
						{"assignee", "A", i18n.T("help.flag.start.assignee")},
						{"onboard", "", i18n.T("help.flag.start.onboard")},
						{"refresh", "", i18n.T("help.flag.start.refresh")},
						{"yes", "y", i18n.T("help.flag.start.yes")},
						{"parallel", "", i18n.T("help.flag.start.parallel")},
						{"tickets", "", i18n.T("help.flag.start.tickets")},
						{"max-sessions", "", i18n.T("help.flag.start.max_sessions")},
						{"priority", "", i18n.T("help.flag.start.priority")},
					},
				},
				{
					Name: "quick",
					Desc: i18n.T("cmd.quick.short"),
				},
				{
					Name: "audit",
					Desc: i18n.T("cmd.audit.short"),
					Flags: []helpFlag{
						{"type", "t", i18n.T("help.flag.audit.type")},
						{"project", "p", i18n.T("help.flag.start.project")},
					},
				},
			{
				Name: "review",
				Desc: i18n.T("cmd.review.short"),
				Flags: []helpFlag{
					{"mode", "m", i18n.T("help.flag.review.mode")},
					{"branch", "b", i18n.T("help.flag.review.branch")},
					{"publish", "", i18n.T("help.flag.review.publish")},
					{"project", "p", i18n.T("help.flag.start.project")},
				},
			},
				{
					Name: "debug",
					Desc: i18n.T("cmd.debug.short"),
					Flags: []helpFlag{
						{"issue", "i", i18n.T("help.flag.debug.issue")},
						{"project", "p", i18n.T("help.flag.start.project")},
					},
				},
			},
		},
		{
			Title: i18n.T("help.section.project"),
			Commands: []helpCommand{
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
						{"path", "d", i18n.T("help.flag.project.path")},
						{"language", "l", i18n.T("help.flag.project.language")},
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
					},
				},
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
			},
		},
		{
			Title: i18n.T("help.section.deploy"),
			Commands: []helpCommand{
				{
					Name: "deploy",
					Desc: i18n.T("cmd.deploy.short"),
					Flags: []helpFlag{
						{"project", "p", i18n.T("help.flag.start.project")},
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
						{"project", "p", i18n.T("help.flag.start.project")},
						{"all", "", i18n.T("help.flag.sync.all")},
						{"dry-run", "", i18n.T("help.flag.sync.dryrun")},
					},
				},
			},
		},
		{
			Title: i18n.T("help.section.mcp"),
			Commands: []helpCommand{
				{
					Name: "mcp status",
					Desc: i18n.T("cmd.mcp.status.short"),
					Flags: []helpFlag{
						{"project", "p", i18n.T("help.flag.start.project")},
					},
				},
				{
					Name: "mcp enable",
					Desc: i18n.T("cmd.mcp.enable.short"),
					Flags: []helpFlag{
						{"project", "p", i18n.T("help.flag.start.project")},
					},
				},
				{
					Name: "mcp disable",
					Desc: i18n.T("cmd.mcp.disable.short"),
					Flags: []helpFlag{
						{"project", "p", i18n.T("help.flag.start.project")},
					},
				},
				{
					Name: "mcp setup",
					Desc: i18n.T("cmd.mcp.setup.short"),
					Flags: []helpFlag{
						{"project", "p", i18n.T("help.flag.start.project")},
					},
				},
				{
					Name: "mcp reset",
					Desc: i18n.T("cmd.mcp.reset.short"),
					Flags: []helpFlag{
						{"project", "p", i18n.T("help.flag.start.project")},
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
			},
		},
		{
			Title: i18n.T("help.section.config"),
			Commands: []helpCommand{
				{
					Name: "config list",
					Desc: i18n.T("cmd.config.list.short"),
					Flags: []helpFlag{
						{"json", "", i18n.T("help.flag.json")},
					},
				},
				{Name: "config get", Desc: i18n.T("cmd.config.get.short")},
				{Name: "config set", Desc: i18n.T("cmd.config.set.short")},
				{Name: "config unset", Desc: i18n.T("cmd.config.unset.short")},
				{Name: "config path", Desc: i18n.T("cmd.config.path.short")},
				{Name: "config language", Desc: i18n.T("cmd.config.language.short")},
				{Name: "config websearch", Desc: i18n.T("cmd.config.websearch.short")},
				{Name: "config model default", Desc: i18n.T("cmd.config.model.default.short")},
				{Name: "config model family", Desc: i18n.T("cmd.config.model.family.short")},
				{Name: "config model agent", Desc: i18n.T("cmd.config.model.agent.short")},
				{
					Name: "config model show",
					Desc: i18n.T("cmd.config.model.show.short"),
					Flags: []helpFlag{
						{"json", "", i18n.T("help.flag.json")},
					},
				},
				{Name: "config model unset", Desc: i18n.T("cmd.config.model.unset.short")},
				{
					Name: "provider setup",
					Desc: i18n.T("cmd.provider.setup.short"),
					Flags: []helpFlag{
						{"project", "p", i18n.T("help.flag.start.project")},
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
						{"period", "d", i18n.T("help.flag.metrics.period")},
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
				{Name: "optimize", Desc: i18n.T("cmd.optimize.short")},
				{Name: "yield", Desc: i18n.T("cmd.yield.short")},
			},
		},
		{
			Title: i18n.T("help.section.team"),
			Commands: []helpCommand{
				{Name: "team init", Desc: i18n.T("help.cmd.team.init")},
				{
					Name: "team status",
					Desc: i18n.T("help.cmd.team.status"),
					Flags: []helpFlag{
						{"detail", "", i18n.T("help.flag.team.detail")},
					},
				},
				{Name: "team activity", Desc: i18n.T("help.cmd.team.activity")},
				{Name: "team board", Desc: i18n.T("help.cmd.team.board")},
				{
					Name: "claim",
					Desc: i18n.T("help.cmd.claim"),
					Flags: []helpFlag{
						{"project", "p", i18n.T("help.flag.start.project")},
					},
				},
				{
					Name: "claim transfer",
					Desc: i18n.T("help.cmd.claim.transfer"),
					Flags: []helpFlag{
						{"to", "", i18n.T("help.flag.claim.to")},
						{"project", "p", i18n.T("help.flag.start.project")},
					},
				},
				{
					Name: "release",
					Desc: i18n.T("help.cmd.release"),
					Flags: []helpFlag{
						{"project", "p", i18n.T("help.flag.start.project")},
					},
				},
				{Name: "conventions check", Desc: i18n.T("cmd.conventions.short")},
				{Name: "beads", Desc: i18n.T("cmd.beads.short")},
				{
					Name: "policies list",
					Desc: i18n.T("help.cmd.policies.list"),
					Flags: []helpFlag{
						{"project", "p", i18n.T("help.flag.start.project")},
					},
				},
				{Name: "policies check", Desc: i18n.T("help.cmd.policies.check")},
				{Name: "policies add", Desc: i18n.T("help.cmd.policies.add")},
				{Name: "takeover-brief show", Desc: i18n.T("help.cmd.takeover.show")},
				{Name: "takeover-brief list", Desc: i18n.T("help.cmd.takeover.list")},
				{Name: "takeover-brief enrich", Desc: i18n.T("help.cmd.takeover.enrich")},
				{
					Name: "patterns list",
					Desc: i18n.T("help.cmd.patterns.list"),
					Flags: []helpFlag{
						{"tags", "", i18n.T("help.flag.patterns.tags")},
					},
				},
				{Name: "patterns show", Desc: i18n.T("help.cmd.patterns.show")},
				{Name: "patterns add", Desc: i18n.T("help.cmd.patterns.add")},
				{Name: "patterns validate", Desc: i18n.T("help.cmd.patterns.validate")},
				{Name: "patterns remove", Desc: i18n.T("help.cmd.patterns.remove")},
			},
		},
		{
			Title: i18n.T("help.section.infra"),
			Commands: []helpCommand{
				{Name: "init", Desc: i18n.T("cmd.init.short")},
				{Name: "doctor", Desc: i18n.T("cmd.doctor.short")},
				{Name: "upgrade opencode", Desc: i18n.T("cmd.upgrade.short")},
				{Name: "version", Desc: i18n.T("cmd.version.short")},
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
