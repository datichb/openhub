package views

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
)

// StatusView displays the real hub and project status.
type StatusView struct {
	app    *tview.Application
	appCtx *app.App
	tv     *tview.TextView
	shell  ShellAccess
	loads  loadSlot // the reads of the status: only the latest is applied
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
	return keys.Hints(keys.Refresh.HintAs(i18n.T("tui.hints.refresh")), conventionsKey.Hint(), keys.Back.HintAs(i18n.T("tui.hints.back")))
}

// conventionsKey checks the conventions of the project (branch, commits).
var conventionsKey = keys.Own("status.conventions", 'c', "tui.hints.conventions_check")

// Mount builds the status display with real data.
func (v *StatusView) Mount(content *tview.Flex, tvApp *tview.Application) {
	v.app = tvApp

	v.tv = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	v.tv.SetBackgroundColor(theme.BgPanel)
	v.tv.SetBorderPadding(1, 0, 2, 2)

	content.AddItem(v.tv, 0, 1, true)
	v.reload("tui.status.loading")
}

// reload reads the status off the event loop (subprocess and DB calls);
// the loading text is shown meanwhile.
func (v *StatusView) reload(loadingKey string) {
	if v.tv == nil {
		return
	}
	setState(v.tv, stateLoading, i18n.T(loadingKey))
	load(v.app, context.Background(), &v.loads, func(context.Context) (string, error) {
		return v.buildStatusText(), nil
	}, func(text string, _ error) {
		if v.tv == nil {
			return
		}
		v.tv.SetText(text)
	})
}

// Unmount cleans up resources.
func (v *StatusView) Unmount() {
	v.loads.Invalidate()
	v.app = nil
	v.tv = nil
}

// HandleKey processes status view key events.
func (v *StatusView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch {
	case keys.Refresh.Matches(event):
		v.reload("tui.status.loading_short")
		return nil
	case conventionsKey.Matches(event):
		v.checkConventions()
		return nil
	}
	return event
}

// buildStatusText builds the status display text. Safe to call from any goroutine.
func (v *StatusView) buildStatusText() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "\n  %s%s%s\n\n", theme.StyleTag(theme.AttrBold), i18n.T("tui.status.hub_title"), theme.TagAttrs)

	// Hub name
	hubName := "OpenHub"
	if v.appCtx != nil && v.appCtx.Config != nil {
		hubName = v.appCtx.Config.Name
	}
	fmt.Fprintf(&sb, "  %s %s\n", secondaryLabel(i18n.T("tui.status.label_hub")), hubName)

	// Config path
	fmt.Fprintf(&sb, "  %s %s\n", secondaryLabel(i18n.T("tui.status.label_config")), config.ConfigPath())

	// Language
	lang := "en"
	if v.appCtx != nil && v.appCtx.Config != nil {
		lang = v.appCtx.Config.CLI.Language
	}
	fmt.Fprintf(&sb, "  %s %s\n", secondaryLabel(i18n.T("tui.status.label_language")), lang)

	// Session tool version
	ocVer := i18n.T("tui.status.not_found")
	if v.appCtx != nil && v.appCtx.ToolVersion != nil {
		if ver, err := v.appCtx.ToolVersion(); err == nil {
			ocVer = ver
		}
	}
	fmt.Fprintf(&sb, "  %s %s\n", secondaryLabel(i18n.Tf("tui.status.label_tool", ToolName())), ocVer)

	// Provider
	provider := "—"
	if v.appCtx != nil && v.appCtx.Config != nil {
		provider = v.appCtx.Config.LLM.DefaultProvider
		if provider == "" {
			provider = i18n.T("tui.status.not_configured")
		}
	}
	fmt.Fprintf(&sb, "  %s %s\n", secondaryLabel(i18n.T("tui.status.label_provider")), provider)

	// Projects count
	sb.WriteString("\n")
	projectCount := 0
	if v.appCtx != nil && v.appCtx.Projects != nil {
		projects, _ := v.appCtx.Projects.List(context.Background(), "")
		projectCount = len(projects)
	}
	fmt.Fprintf(&sb, "  %s %s\n", secondaryLabel(i18n.T("tui.status.label_projects")), i18n.Tf("tui.status.registered_count", projectCount))

	return sb.String()
}

// secondaryLabel is a label of the status, padded to the column of the values.
func secondaryLabel(label string) string {
	return theme.Paint(theme.TextSecondaryHex, fmt.Sprintf("%-18s", label))
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
					text += "  " + tagged(theme.SuccessHex, theme.SymSuccess) + " " + i18n.Tf("tui.status.branch_compliant", branch) + "\n"
				} else {
					text += "  " + tagged(theme.WarningHex, theme.SymWarning) + " " + i18n.Tf("tui.status.branch_non_compliant", branch, branchPattern) + "\n"
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
				text += fmt.Sprintf("  %s %s\n", tagged(theme.SuccessHex, theme.SymSuccess), i18n.Tf("tui.status.commits_compliant", len(commits)))
			} else {
				text += fmt.Sprintf("  %s %s\n", tagged(theme.WarningHex, theme.SymWarning), i18n.Tf("tui.status.commits_non_compliant", nonConform, len(commits)))
				issues++
			}
		}
	} else if len(commits) > 0 {
		text += fmt.Sprintf("  · %s\n", i18n.Tf("tui.status.commits_no_format", len(commits)))
	}

	// 3. Summary
	text += "\n"
	if issues == 0 {
		text += "  " + tagged(theme.SuccessHex, theme.SymSuccess) + " " + i18n.T("tui.status.all_compliant")
	} else {
		text += fmt.Sprintf("  %s %s", tagged(theme.WarningHex, theme.SymWarning), i18n.Tf("tui.status.warnings_count", issues))
	}

	v.shell.ShowScrollableModal(i18n.T("tui.status.conventions_title"), text, []ModalAction{
		{Label: i18n.T("tui.status.close"), Callback: func() {}},
	})
}

// Git helpers for conventions check.
func gitCurrentBranch(dir string) string {
	cmd := gitutil.Command(context.Background(), dir, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitLastCommits(dir string, n int) []string {
	cmd := gitutil.Command(context.Background(), dir, "log", fmt.Sprintf("-%d", n), "--pretty=format:%s")
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
