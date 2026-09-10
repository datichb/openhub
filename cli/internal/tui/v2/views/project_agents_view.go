package views

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// ProjectAgentsViewConfig holds external dependencies for the project agents view.
type ProjectAgentsViewConfig struct {
	// GetProject returns the currently active project (copy).
	GetProject func() *domain.Project
	// SaveProject persists the modified project to the DB.
	SaveProject func(ctx context.Context, p *domain.Project) error
	// AllAgents returns the list of all available agent IDs.
	AllAgents func() []string
	// GetWorkflow returns the resolved workflow definition (optional, nil = no workflow).
	GetWorkflow func() *workflow.WorkflowDefinition
}

// ProjectAgentsView displays and edits the agent selection for the active project.
type ProjectAgentsView struct {
	app      *tview.Application
	list     *widgets.SectionedList
	shell    ShellAccess
	cfg      ProjectAgentsViewConfig
	mountGen uint64

	live      *domain.Project
	undoStack *widgets.UndoStack[domain.Project]
	autoSaver *AutoSaver
}

var _ View = (*ProjectAgentsView)(nil)
var _ CommandProvider = (*ProjectAgentsView)(nil)

// NewProjectAgentsView creates the project agents view.
func NewProjectAgentsView(cfg ProjectAgentsViewConfig) *ProjectAgentsView {
	return &ProjectAgentsView{
		cfg:       cfg,
		undoStack: widgets.NewUndoStack[domain.Project](10),
	}
}

func (v *ProjectAgentsView) SetShell(s ShellAccess) { v.shell = s }

func (v *ProjectAgentsView) ID() string    { return "project.agents" }
func (v *ProjectAgentsView) Title() string { return "Agents" }

func (v *ProjectAgentsView) StatusHints() string {
	return fmt.Sprintf("j/k %s · Space %s · u %s · Esc %s",
		i18n.T("tui.hints.nav"),
		i18n.T("tui.hints.toggle"),
		i18n.T("tui.hints.undo"),
		i18n.T("tui.hints.back"),
	)
}

// ─────────────────────────────────────────────────────────────────────────────
// Mount / Unmount
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectAgentsView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.mountGen++
	gen := v.mountGen
	v.undoStack.Clear()
	v.live = v.cfg.GetProject()

	v.autoSaver = NewAutoSaver(200*time.Millisecond, app, func() {
		v.doSave()
	})

	// Show loading placeholder immediately
	loading := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	loading.SetBackgroundColor(theme.BgPanel)
	muted := theme.ColorTag(theme.TextMutedHex)
	loading.SetText(fmt.Sprintf("\n  %s%s%s", muted, i18n.T("tui.project.loading"), theme.TagColor))
	content.AddItem(loading, 0, 1, true)

	// Build UI asynchronously
	go func() {
		app.QueueUpdateDraw(func() {
			if v.app == nil || v.mountGen != gen {
				return
			}

			v.list = widgets.NewSectionedList()
			v.list.SetApp(app)
			v.list.SetBorderPadding(1, 0, 2, 2)

			if v.live == nil {
				items := []widgets.SectionItem{{
					MainText: i18n.T("tui.project.no_active"),
				}}
				v.list.SetItems(items)
				content.RemoveItem(loading)
				content.AddItem(v.list, 0, 1, true)
				app.SetFocus(v.list)
				return
			}

			v.list.SetItemSelectedFunc(func(_ int, item widgets.SectionItem) {
				if _, ok := item.Reference.(int); ok {
					v.toggleSelected()
				}
			})

			v.renderLines()
			content.RemoveItem(loading)
			content.AddItem(v.list, 0, 1, true)
			app.SetFocus(v.list)
		})
	}()
}

func (v *ProjectAgentsView) Unmount() {
	if v.autoSaver != nil {
		v.autoSaver.Flush()
	}
	v.undoStack.Clear()
	v.app = nil
	v.list = nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectAgentsView) renderLines() {
	if v.list == nil || v.live == nil {
		return
	}
	savedIdx := v.list.GetCurrentItem()

	allAgents := v.cfg.AllAgents()
	sort.Strings(allAgents)

	activeSet := make(map[string]bool, len(v.live.Agents))
	for _, a := range v.live.Agents {
		activeSet[a] = true
	}

	items := make([]widgets.SectionItem, 0, len(allAgents)+1)

	// Section header
	title := "Agents"
	items = append(items, widgets.SectionItem{
		IsHeader: true,
		MainText: title,
	})

	// Agent rows
	green := theme.ColorTag(theme.SuccessHex)
	muted := theme.ColorTag(theme.TextMutedHex)
	warn := theme.ColorTag(theme.WarningHex)
	reset := theme.TagColor

	// Build workflow lookup for mandatory/disabled indicators.
	var wfSlots map[string]*workflow.AgentSlot
	if v.cfg.GetWorkflow != nil {
		if wf := v.cfg.GetWorkflow(); wf != nil {
			wfSlots = make(map[string]*workflow.AgentSlot, len(wf.Agents))
			for i := range wf.Agents {
				wfSlots[wf.Agents[i].AgentID] = &wf.Agents[i]
			}
		}
	}

	for idx, agent := range allAgents {
		var status string
		var suffix string

		// Check workflow constraints.
		if slot, ok := wfSlots[agent]; ok {
			if slot.Role == workflow.RoleDisabled {
				suffix = fmt.Sprintf(" %s[workflow: disabled]%s", muted, reset)
			} else if slot.Mandatory {
				suffix = fmt.Sprintf(" %s🔒%s", warn, reset)
			}
		}

		if activeSet[agent] {
			status = fmt.Sprintf("%s✓ %s%s", green, i18n.T("tui.settings.enabled"), reset)
		} else {
			status = fmt.Sprintf("%s✗ %s%s", muted, i18n.T("tui.settings.disabled"), reset)
		}
		mainText := fmt.Sprintf("%-28s %s%s", agent, status, suffix)
		items = append(items, widgets.SectionItem{
			MainText:  mainText,
			Reference: idx,
		})
	}

	v.list.SetItems(items)
	if savedIdx >= 0 {
		v.list.SelectIndex(savedIdx)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Key handling
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectAgentsView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if v.live == nil || v.list == nil {
		return event
	}
	if event.Key() == tcell.KeyEnter {
		v.toggleSelected()
		return nil
	}
	switch event.Rune() {
	case ' ':
		v.toggleSelected()
		return nil
	case 'u':
		v.undo()
		return nil
	}
	return event
}

// ─────────────────────────────────────────────────────────────────────────────
// Toggle
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectAgentsView) toggleSelected() {
	if v.list == nil || v.live == nil {
		return
	}
	_, item, ok := v.list.CurrentItem()
	if !ok {
		return
	}
	ref, ok := item.Reference.(int)
	if !ok {
		return
	}

	allAgents := v.cfg.AllAgents()
	sort.Strings(allAgents)
	if ref < 0 || ref >= len(allAgents) {
		return
	}
	agent := allAgents[ref]

	// Build active set early (needed for workflow constraint checks).
	activeSet := make(map[string]bool, len(v.live.Agents))
	for _, a := range v.live.Agents {
		activeSet[a] = true
	}

	// Check workflow constraints before toggling.
	if v.cfg.GetWorkflow != nil {
		if wf := v.cfg.GetWorkflow(); wf != nil {
			if slot := wf.FindAgent(agent); slot != nil {
				if slot.Mandatory && activeSet[agent] {
					// Cannot deselect a mandatory agent.
					if v.shell != nil {
						v.shell.ShowToastMsg(fmt.Sprintf("Agent %q is mandatory in the workflow", agent), false)
					}
					return
				}
				if slot.Role == workflow.RoleDisabled && !activeSet[agent] {
					// Cannot enable an agent disabled by workflow.
					if v.shell != nil {
						v.shell.ShowToastMsg(fmt.Sprintf("Agent %q is disabled by the workflow", agent), false)
					}
					return
				}
			}
		}
	}

	v.pushUndo()

	// Toggle: add or remove from live.Agents
	if activeSet[agent] {
		newAgents := make([]string, 0, len(v.live.Agents))
		for _, a := range v.live.Agents {
			if a != agent {
				newAgents = append(newAgents, a)
			}
		}
		v.live.Agents = newAgents
	} else {
		v.live.Agents = append(v.live.Agents, agent)
		sort.Strings(v.live.Agents)
	}

	if v.autoSaver != nil {
		v.autoSaver.Schedule()
	}
	v.renderLines()
}

func (v *ProjectAgentsView) pushUndo() {
	snapshot := deepCopyProject(v.live)
	v.undoStack.Push(snapshot)
}

// ─────────────────────────────────────────────────────────────────────────────
// Undo
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectAgentsView) undo() {
	prev, ok := v.undoStack.Pop()
	if !ok {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.config.nothing_to_undo"), true)
		}
		return
	}
	*v.live = prev
	if v.autoSaver != nil {
		v.autoSaver.Cancel()
	}
	v.doSave()
	v.renderLines()
	if v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.config.undone"), true)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Save (auto-save via AutoSaver)
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectAgentsView) doSave() {
	if v.live == nil {
		return
	}
	ctx := context.Background()
	if err := v.cfg.SaveProject(ctx, v.live); err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.project.save_error")+": "+err.Error(), false)
		}
		return
	}
	if v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.config.autosaved"), true)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Commands
// ─────────────────────────────────────────────────────────────────────────────

// ContextCommands implements CommandProvider.
func (v *ProjectAgentsView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{ID: "project.agents.undo", Label: i18n.T("tui.hints.undo"), Aliases: []string{"undo", "annuler"}, Description: i18n.T("tui.settings.cmd_undo"), Category: "Projet", Action: func() { v.undo() }},
	}
}
