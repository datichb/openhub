package views

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// StatusView displays the real hub and project status.
type StatusView struct {
	app    *tview.Application
	appCtx *app.App
	tv     *tview.TextView
	shell  ShellAccess
}

var _ View = (*StatusView)(nil)

// NewStatusView creates a new status view.
func NewStatusView(a *app.App) *StatusView {
	return &StatusView{appCtx: a}
}

// SetShell provides the shell reference for modal interactions.
func (v *StatusView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *StatusView) ID() string { return "status" }

// Title returns the display title.
func (v *StatusView) Title() string { return i18n.T("tui.status.title") }

// StatusHints returns keybinding hints.
func (v *StatusView) StatusHints() string {
	return fmt.Sprintf("r %s · c %s · Esc %s", i18n.T("tui.hints.refresh"), i18n.T("tui.hints.conventions_check"), i18n.T("tui.hints.back"))
}

// Mount builds the status display with real data.
func (v *StatusView) Mount(content *tview.Flex, tvApp *tview.Application) {
	v.app = tvApp

	v.tv = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	v.tv.SetBackgroundColor(theme.BgPanel)
	v.tv.SetBorderPadding(1, 0, 2, 2)

	// Show loading placeholder immediately
	muted := theme.ColorTag(theme.TextMutedHex)
	v.tv.SetText(fmt.Sprintf("\n  %s%s%s", muted, i18n.T("tui.status.loading"), theme.TagColor))
	content.AddItem(v.tv, 0, 1, true)

	// Load status data asynchronously (subprocess + DB calls)
	go func() {
		text := v.buildStatusText()
		tvApp.QueueUpdateDraw(func() {
			if v.tv == nil {
				return
			}
			v.tv.SetText(text)
		})
	}()
}

// Unmount cleans up resources.
func (v *StatusView) Unmount() {
	v.app = nil
	v.tv = nil
}

// HandleKey processes status view key events.
func (v *StatusView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'r':
		muted := theme.ColorTag(theme.TextMutedHex)
		v.tv.SetText(fmt.Sprintf("\n  %s%s%s", muted, i18n.T("tui.status.loading_short"), theme.TagColor))
		go func() {
			text := v.buildStatusText()
			v.app.QueueUpdateDraw(func() {
				if v.tv == nil || v.app == nil {
					return
				}
				v.tv.SetText(text)
			})
		}()
		return nil
	case 'c':
		v.checkConventions()
		return nil
	}
	return event
}

// buildStatusText builds the status display text. Safe to call from any goroutine.
func (v *StatusView) buildStatusText() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "\n  [::b]%s%s\n\n", i18n.T("tui.status.hub_title"), theme.TagReset)

	// Hub name
	hubName := "OpenHub"
	if v.appCtx != nil && v.appCtx.Config != nil {
		hubName = v.appCtx.Config.Name
	}
	fmt.Fprintf(&sb, "  %s%-18s%s %s\n",
		theme.ColorTag(theme.TextSecondaryHex), i18n.T("tui.status.label_hub"), theme.TagColor, hubName)

	// Config path
	fmt.Fprintf(&sb, "  %s%-18s%s %s\n",
		theme.ColorTag(theme.TextSecondaryHex), i18n.T("tui.status.label_config"), theme.TagColor, config.ConfigPath())

	// Language
	lang := "en"
	if v.appCtx != nil && v.appCtx.Config != nil {
		lang = v.appCtx.Config.CLI.Language
	}
	fmt.Fprintf(&sb, "  %s%-18s%s %s\n",
		theme.ColorTag(theme.TextSecondaryHex), i18n.T("tui.status.label_language"), theme.TagColor, lang)

	// Opencode version
	ocVer, err := opencode.Version()
	if err != nil {
		ocVer = i18n.T("tui.status.not_found")
	}
	channel := "stable"
	if v.appCtx != nil && v.appCtx.Config != nil {
		channel = v.appCtx.Config.Opencode.Channel
	}
	fmt.Fprintf(&sb, "  %s%-18s%s %s (%s)\n",
		theme.ColorTag(theme.TextSecondaryHex), i18n.T("tui.status.label_opencode"), theme.TagColor, ocVer, channel)

	// Provider
	provider := "—"
	if v.appCtx != nil && v.appCtx.Config != nil {
		provider = v.appCtx.Config.Opencode.DefaultProvider
		if provider == "" {
			provider = i18n.T("tui.status.not_configured")
		}
	}
	fmt.Fprintf(&sb, "  %s%-18s%s %s\n",
		theme.ColorTag(theme.TextSecondaryHex), i18n.T("tui.status.label_provider"), theme.TagColor, provider)

	// Projects count
	sb.WriteString("\n")
	projectCount := 0
	if v.appCtx != nil && v.appCtx.Projects != nil {
		projects, _ := v.appCtx.Projects.List(context.Background(), "")
		projectCount = len(projects)
	}
	fmt.Fprintf(&sb, "  %s%-18s%s %s\n",
		theme.ColorTag(theme.TextSecondaryHex), i18n.T("tui.status.label_projects"), theme.TagColor, i18n.Tf("tui.status.registered_count", projectCount))

	return sb.String()
}

func (v *StatusView) checkConventions() {
	if v.shell == nil {
		return
	}

	// Determine project path
	projectPath := "."
	if v.appCtx != nil && v.appCtx.Projects != nil {
		p, err := resolveFirstProject(v.appCtx)
		if err == nil {
			projectPath = p.Path
		}
	}

	var text string
	issues := 0

	// 1. Branch naming
	branch := gitCurrentBranch(projectPath)
	if branch != "" {
		branchPattern := conventionBranchPattern(projectPath)
		if branchPattern != "" {
			re, err := regexp.Compile(branchPattern)
			if err == nil {
				if re.MatchString(branch) {
					text += "  [green]✓[-] " + i18n.Tf("tui.status.branch_compliant", branch) + "\n"
				} else {
					text += "  [yellow]⚠[-] " + i18n.Tf("tui.status.branch_non_compliant", branch, branchPattern) + "\n"
					issues++
				}
			}
		} else {
			text += "  · " + i18n.Tf("tui.status.branch_no_pattern", branch) + "\n"
		}
	} else {
		text += "  · " + i18n.T("tui.status.branch_not_detected") + "\n"
	}

	// 2. Commit format
	commits := gitLastCommits(projectPath, 5)
	commitPattern := conventionCommitPattern(projectPath)
	if commitPattern != "" && len(commits) > 0 {
		re, err := regexp.Compile(commitPattern)
		if err == nil {
			nonConform := 0
			for _, c := range commits {
				if !re.MatchString(c) {
					nonConform++
				}
			}
			if nonConform == 0 {
				text += fmt.Sprintf("  [green]✓[-] %s\n", i18n.Tf("tui.status.commits_compliant", len(commits)))
			} else {
				text += fmt.Sprintf("  [yellow]⚠[-] %s\n", i18n.Tf("tui.status.commits_non_compliant", nonConform, len(commits)))
				issues++
			}
		}
	} else if len(commits) > 0 {
		text += fmt.Sprintf("  · %s\n", i18n.Tf("tui.status.commits_no_format", len(commits)))
	}

	// 3. Summary
	text += "\n"
	if issues == 0 {
		text += "  [green]✓[-] " + i18n.T("tui.status.all_compliant")
	} else {
		text += fmt.Sprintf("  [yellow]⚠[-] %s", i18n.Tf("tui.status.warnings_count", issues))
	}

	v.shell.ShowScrollableModal(i18n.T("tui.status.conventions_title"), text, []ModalAction{
		{Label: i18n.T("tui.status.close"), Callback: func() {}},
	})
}

// Git helpers for conventions check.
func gitCurrentBranch(dir string) string {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitLastCommits(dir string, n int) []string {
	cmd := exec.Command("git", "-C", dir, "log", fmt.Sprintf("-%d", n), "--pretty=format:%s")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

// conventionBranchPattern looks for a branch naming pattern in conventions docs.
func conventionBranchPattern(dir string) string {
	content := readConventionsFile(dir)
	if content == "" {
		return ""
	}
	// Look for: branch.*pattern.*[=:].*`pattern` or "pattern"
	re := regexp.MustCompile(`(?i)branch.*pattern.*[=:]\s*` + "`" + `([^` + "`" + `]+)` + "`")
	if m := re.FindStringSubmatch(content); len(m) > 1 {
		return m[1]
	}
	return ""
}

// conventionCommitPattern looks for a commit format pattern in conventions docs.
func conventionCommitPattern(dir string) string {
	content := readConventionsFile(dir)
	if content == "" {
		return ""
	}
	// If "conventional commits" is mentioned, use the standard regex
	if strings.Contains(strings.ToLower(content), "conventional commit") {
		return `^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\(.+\))?!?:\s.+`
	}
	// Look for explicit commit pattern
	re := regexp.MustCompile(`(?i)commit.*pattern.*[=:]\s*` + "`" + `([^` + "`" + `]+)` + "`")
	if m := re.FindStringSubmatch(content); len(m) > 1 {
		return m[1]
	}
	return ""
}

func readConventionsFile(dir string) string {
	paths := []string{
		"docs/wiki/technical/conventions.md",
		"docs/wiki/conventions.md",
		"CONVENTIONS.md",
	}
	for _, p := range paths {
		full := filepath.Join(dir, p)
		data, err := os.ReadFile(full)
		if err == nil && len(data) > 0 {
			return string(data)
		}
	}
	return ""
}
