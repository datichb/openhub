package views

import (
	"fmt"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
	"github.com/datichb/openhub/cli/internal/workflow"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// WorkflowViewConfig holds the dependencies for the workflow view.
type WorkflowViewConfig struct {
	// GetWorkflow returns the resolved workflow definition for the current level.
	GetWorkflow func() (*workflow.WorkflowDefinition, error)
	// GetOverrides returns the current level's overrides (may be nil).
	GetOverrides func() (*workflow.WorkflowOverride, error)
	// SaveOverrides persists overrides for the current level.
	SaveOverrides func(ov *workflow.WorkflowOverride) error
	// Level returns the current scope label ("hub", "team", or "project").
	// A function is used so the title updates dynamically when the user
	// switches mode without remounting the view.
	Level func() string
	// IsLocked returns true if a higher level has enforced the workflow.
	IsLocked func() bool
	// Deploy triggers a redeploy after saving.
	Deploy func() error
}

type workflowView struct {
	cfg   WorkflowViewConfig
	shell ShellAccess

	// Live state
	wf        *workflow.WorkflowDefinition
	overrides *workflow.WorkflowOverride
	dirty     bool

	// TUI components
	graph    *widgets.WorkflowGraph
	detail   *tview.TextView
	content  *tview.Flex
	app      *tview.Application
	mountGen uint64 // guards stale goroutines (standard pattern)
}

// NewWorkflowView creates a new workflow configuration view.
func NewWorkflowView(cfg WorkflowViewConfig) View {
	return &workflowView{cfg: cfg}
}

func (v *workflowView) ID() string    { return "workflow" }
func (v *workflowView) Title() string {
	level := "hub"
	if v.cfg.Level != nil {
		level = v.cfg.Level()
	}
	return i18n.Tf("tui.workflow.title", level)
}

func (v *workflowView) StatusHints() string {
	if v.cfg.IsLocked != nil && v.cfg.IsLocked() {
		return i18n.T("tui.workflow.hints_locked")
	}
	return i18n.T("tui.workflow.hints")
}

func (v *workflowView) SetShell(s ShellAccess) {
	v.shell = s
}

func (v *workflowView) Mount(content *tview.Flex, app *tview.Application) {
	v.content = content
	v.app = app
	v.mountGen++
	gen := v.mountGen

	// Show loading placeholder.
	loading := tview.NewTextView().SetText(i18n.T("tui.workflow.loading")).SetTextColor(theme.FgMuted)
	content.AddItem(loading, 0, 1, false)

	go func() {
		wf, err := v.cfg.GetWorkflow()
		if err != nil {
			app.QueueUpdateDraw(func() {
				if v.app == nil || v.mountGen != gen {
					return // view was unmounted or re-mounted
				}
				content.Clear()
				errView := tview.NewTextView().SetText(i18n.Tf("tui.workflow.error", err.Error())).SetTextColor(theme.Error)
				content.AddItem(errView, 0, 1, false)
			})
			return
		}

		v.wf = wf

		// Load current overrides.
		if v.cfg.GetOverrides != nil {
			ov, _ := v.cfg.GetOverrides()
			v.overrides = ov
		}
		if v.overrides == nil {
			v.overrides = &workflow.WorkflowOverride{}
		}

		readonly := v.cfg.IsLocked != nil && v.cfg.IsLocked()

		app.QueueUpdateDraw(func() {
			if v.app == nil || v.mountGen != gen {
				return // view was unmounted or re-mounted
			}
			content.Clear()
			v.buildUI(content, readonly)
			app.SetFocus(v.graph)
		})
	}()
}

func (v *workflowView) buildUI(content *tview.Flex, readonly bool) {
	// Left: workflow graph (70%)
	v.graph = widgets.NewWorkflowGraph(v.wf, readonly)
	v.graph.SetBorder(true).SetTitle(" " + i18n.T("tui.workflow.graph_title") + " ").SetBorderColor(theme.BorderNormal)

	// Right: detail panel (30%)
	v.detail = tview.NewTextView().
		SetDynamicColors(true).
		SetWordWrap(true)
	v.detail.SetBorder(true).SetTitle(" " + i18n.T("tui.workflow.detail_title") + " ").SetBorderColor(theme.BorderNormal)

	v.graph.SetOnChange(func() {
		v.updateDetailPanel()
	})

	v.graph.SetOnSelect(func(elem widgets.GraphElement) {
		if readonly {
			return
		}
		v.handleElementAction(elem)
	})

	content.SetDirection(tview.FlexColumn)
	content.AddItem(v.graph, 0, 7, true)
	content.AddItem(v.detail, 0, 3, false)

	// Initial detail.
	v.updateDetailPanel()
}

func (v *workflowView) updateDetailPanel() {
	if v.detail == nil || v.graph == nil {
		return
	}
	v.detail.Clear()

	sel := v.graph.SelectedElement()
	if sel == nil {
		fmt.Fprintf(v.detail, "[%s]%s[-]", theme.TextMutedHex, i18n.T("tui.workflow.select_element"))
		return
	}

	switch sel.Type {
	case widgets.ElementCheckpoint:
		v.showCheckpointDetail(sel.ID)
	case widgets.ElementAgent, widgets.ElementIndependentAgent:
		v.showAgentDetail(sel.ID)
	case widgets.ElementEdge:
		fmt.Fprintf(v.detail, "[%s::b]Edge[-:-:-]\n\n", theme.AccentHex)
		fmt.Fprintf(v.detail, "%s\n\n", sel.ID)
		fmt.Fprintf(v.detail, "[%s]%s[-]", theme.TextMutedHex, i18n.T("tui.workflow.edge_hint"))
	}
}

func (v *workflowView) showCheckpointDetail(id string) {
	cp := v.wf.FindCheckpoint(id)
	if cp == nil {
		return
	}

	fmt.Fprintf(v.detail, "[%s::b]Checkpoint: %s[-:-:-]\n", theme.AccentHex, cp.ID)
	fmt.Fprintf(v.detail, "%s\n\n", cp.Label)

	if cp.Mandatory {
		fmt.Fprintf(v.detail, "[%s]🔒 %s[-]\n\n", theme.WarningHex, i18n.T("tui.workflow.mandatory"))
	}

	if cp.Description != "" {
		fmt.Fprintf(v.detail, "%s\n\n", cp.Description)
	}

	fmt.Fprintf(v.detail, "[%s::b]%s[-:-:-]\n", theme.TextSecondaryHex, i18n.T("tui.workflow.behavior_per_mode"))
	for _, mode := range v.wf.Modes.Available {
		behavior := cp.Behavior[mode]
		icon := "○"
		switch behavior {
		case workflow.BehaviorPause:
			icon = "[" + theme.WarningHex + "]●[-]"
		case workflow.BehaviorAuto:
			icon = "[" + theme.SuccessHex + "]◆[-]"
		case workflow.BehaviorSkip:
			icon = "[" + theme.TextMutedHex + "]◌[-]"
		case workflow.BehaviorConditional:
			icon = "[" + theme.InfoHex + "]◈[-]"
		}
		fmt.Fprintf(v.detail, "  %s %s: %s\n", icon, mode, behavior)
	}

	if cp.Condition != "" {
		fmt.Fprintf(v.detail, "\n[%s::b]%s[-:-:-]\n%s\n", theme.TextSecondaryHex, i18n.T("tui.workflow.condition"), cp.Condition)
	}

	if len(cp.Agents) > 0 {
		fmt.Fprintf(v.detail, "\n[%s::b]%s[-:-:-]\n", theme.TextSecondaryHex, i18n.T("tui.workflow.agents"))
		for _, a := range cp.Agents {
			fmt.Fprintf(v.detail, "  • %s\n", a)
		}
	}
}

func (v *workflowView) showAgentDetail(id string) {
	agent := v.wf.FindAgent(id)
	if agent == nil {
		return
	}

	fmt.Fprintf(v.detail, "[%s::b]Agent: %s[-:-:-]\n\n", theme.AccentHex, agent.AgentID)

	if agent.Mandatory {
		fmt.Fprintf(v.detail, "[%s]🔒 %s[-]\n\n", theme.WarningHex, i18n.T("tui.workflow.mandatory"))
	}

	fmt.Fprintf(v.detail, "[%s::b]%s[-:-:-] %s\n", theme.TextSecondaryHex, i18n.T("tui.workflow.role"), agent.Role)
	fmt.Fprintf(v.detail, "[%s::b]%s[-:-:-] %s\n", theme.TextSecondaryHex, i18n.T("tui.workflow.mode"), agent.Mode)

	if agent.Position != nil {
		fmt.Fprintf(v.detail, "[%s::b]%s[-:-:-] %s", theme.TextSecondaryHex, i18n.T("tui.workflow.position"), i18n.Tf("tui.workflow.after_checkpoint", agent.Position.AfterCheckpoint))
		if agent.Position.Branch != "" {
			fmt.Fprintf(v.detail, " (%s)", i18n.Tf("tui.workflow.branch", agent.Position.Branch))
		}
		fmt.Fprintln(v.detail)
	}

	if agent.TaskPermissions != nil {
		if len(agent.TaskPermissions.CanInvoke) > 0 {
			fmt.Fprintf(v.detail, "\n[%s::b]%s[-:-:-]\n", theme.TextSecondaryHex, i18n.T("tui.workflow.can_invoke"))
			for _, a := range agent.TaskPermissions.CanInvoke {
				fmt.Fprintf(v.detail, "  → %s\n", a)
			}
		}
		if len(agent.TaskPermissions.CanBeInvokedBy) > 0 {
			fmt.Fprintf(v.detail, "\n[%s::b]%s[-:-:-]\n", theme.TextSecondaryHex, i18n.T("tui.workflow.invocable_by"))
			for _, a := range agent.TaskPermissions.CanBeInvokedBy {
				fmt.Fprintf(v.detail, "  ← %s\n", a)
			}
		}
	}
}

func (v *workflowView) handleElementAction(elem widgets.GraphElement) {
	switch elem.Type {
	case widgets.ElementCheckpoint:
		v.editCheckpoint(elem.ID)
	case widgets.ElementAgent, widgets.ElementIndependentAgent:
		v.editAgent(elem.ID)
	case widgets.ElementEdge:
		// No action on Enter for edges — use 'a' to add.
	}
}

func (v *workflowView) Unmount() {
	v.mountGen++ // invalidate in-flight async goroutine
	if v.dirty && v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.workflow.unsaved_discarded"), false)
	}
	v.graph = nil
	v.detail = nil
	v.content = nil
	v.app = nil
}

func (v *workflowView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if v.graph == nil {
		return event
	}
	if v.cfg.IsLocked != nil && v.cfg.IsLocked() {
		return event // readonly — pass through
	}

	switch event.Key() {
	case tcell.KeyRune:
		switch event.Rune() {
		case 'a':
			v.handleAdd()
			return nil
		case 'd':
			v.handleDelete()
			return nil
		case 't':
			v.toggleAgentMode()
			return nil
		case 'r':
			v.toggleAgentRole()
			return nil
		case 'm':
			v.editModes()
			return nil
		case 'w':
			v.save()
			return nil
		case 'R':
			v.reset()
			return nil
		}
	}
	return event
}

// ---------------------------------------------------------------------------
// Actions
// ---------------------------------------------------------------------------

func (v *workflowView) handleAdd() {
	sel := v.graph.SelectedElement()
	if sel == nil {
		return
	}
	if sel.Type == widgets.ElementEdge {
		v.addCheckpointOnEdge(sel.ID)
	}
}

func (v *workflowView) handleDelete() {
	sel := v.graph.SelectedElement()
	if sel == nil {
		return
	}
	switch sel.Type {
	case widgets.ElementCheckpoint:
		v.deleteCheckpoint(sel.ID)
	case widgets.ElementAgent, widgets.ElementIndependentAgent:
		v.disableAgent(sel.ID)
	}
}

func (v *workflowView) toggleAgentMode() {
	sel := v.graph.SelectedElement()
	if sel == nil || (sel.Type != widgets.ElementAgent && sel.Type != widgets.ElementIndependentAgent) {
		return
	}

	agent := v.wf.FindAgent(sel.ID)
	if agent == nil {
		return
	}

	newMode := workflow.ModeSubagent
	if agent.Mode == workflow.ModeSubagent {
		newMode = workflow.ModePrimary
	}

	v.overrides.AgentOverrides = appendOrUpdateAgentOverride(v.overrides.AgentOverrides, workflow.AgentOverride{
		AgentID: sel.ID,
		Mode:    &newMode,
	})
	v.dirty = true
	v.refreshWorkflow()
}

func (v *workflowView) toggleAgentRole() {
	sel := v.graph.SelectedElement()
	if sel == nil || (sel.Type != widgets.ElementAgent && sel.Type != widgets.ElementIndependentAgent) {
		return
	}

	agent := v.wf.FindAgent(sel.ID)
	if agent == nil || agent.Mandatory {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.workflow.agent_mandatory_role"), false)
		}
		return
	}

	// Cycle: workflow → independent → disabled → workflow
	var newRole workflow.AgentRole
	switch agent.Role {
	case workflow.RoleWorkflow:
		newRole = workflow.RoleIndependent
	case workflow.RoleIndependent:
		newRole = workflow.RoleDisabled
	case workflow.RoleDisabled:
		newRole = workflow.RoleWorkflow
	}

	v.overrides.AgentOverrides = appendOrUpdateAgentOverride(v.overrides.AgentOverrides, workflow.AgentOverride{
		AgentID: sel.ID,
		Role:    &newRole,
	})
	v.dirty = true
	v.refreshWorkflow()
}

func (v *workflowView) save() {
	if v.cfg.SaveOverrides == nil {
		return
	}
	if err := v.cfg.SaveOverrides(v.overrides); err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.Tf("tui.workflow.save_error", err.Error()), false)
		}
		return
	}
	v.dirty = false
	if v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.workflow.saved"), true)
	}
}

func (v *workflowView) reset() {
	v.overrides = &workflow.WorkflowOverride{}
	v.dirty = true
	v.refreshWorkflow()
	if v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.workflow.overrides_reset"), true)
	}
}

func (v *workflowView) refreshWorkflow() {
	base := workflow.BaseWorkflow()
	resolved, err := workflow.Resolve(base, *v.overrides)
	if err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.Tf("tui.workflow.invalid", err.Error()), false)
		}
		return
	}
	v.wf = &resolved
	if v.graph != nil {
		v.graph.SetWorkflow(v.wf)
	}
	v.updateDetailPanel()
}

// ---------------------------------------------------------------------------
// Modal-based editing
// ---------------------------------------------------------------------------

func (v *workflowView) editCheckpoint(id string) {
	cp := v.wf.FindCheckpoint(id)
	if cp == nil {
		return
	}

	fields := []FormField{
		{Key: "label", Label: "Label", Type: FieldText, Default: cp.Label},
		{Key: "description", Label: "Description", Type: FieldText, Default: cp.Description},
	}

	// Behavior fields per mode.
	behaviorOptions := []SelectOption{
		{Value: "pause", Label: "pause"},
		{Value: "auto", Label: "auto"},
		{Value: "skip", Label: "skip"},
		{Value: "conditional", Label: "conditional"},
	}
	for _, mode := range v.wf.Modes.Available {
		current := string(cp.Behavior[mode])
		fields = append(fields, FormField{
			Key:     "behavior_" + mode,
			Label:   "Mode " + mode,
			Type:    FieldSelect,
			Options: behaviorOptions,
			Default: current,
		})
	}

	fields = append(fields, FormField{
		Key:     "condition",
		Label:   i18n.T("tui.workflow.condition_label"),
		Type:    FieldText,
		Default: cp.Condition,
		Hint:    i18n.T("tui.workflow.condition_hint"),
	})

	v.shell.ShowInlineForm(InlineFormConfig{
		Title:  i18n.Tf("tui.workflow.edit_checkpoint", id),
		Fields: fields,
		OnSubmit: func(values map[string]string, _ map[string][]string) {
			label := values["label"]
			desc := values["description"]
			condition := values["condition"]

			behaviors := make(map[string]workflow.CheckpointBehavior)
			for _, mode := range v.wf.Modes.Available {
				if b, ok := values["behavior_"+mode]; ok {
					behaviors[mode] = workflow.CheckpointBehavior(b)
				}
			}

			v.overrides.CheckpointOverrides = appendOrUpdateCheckpointOverride(
				v.overrides.CheckpointOverrides,
				workflow.CheckpointOverride{
					ID:          id,
					Action:      workflow.ActionModify,
					Label:       &label,
					Description: &desc,
					Behavior:    behaviors,
					Condition:   &condition,
				},
			)
			v.dirty = true
			v.refreshWorkflow()
		},
	})
}

func (v *workflowView) editAgent(id string) {
	agent := v.wf.FindAgent(id)
	if agent == nil {
		return
	}

	roleOptions := []SelectOption{
		{Value: "workflow", Label: "workflow"},
		{Value: "independent", Label: "independent"},
		{Value: "disabled", Label: "disabled"},
	}
	modeOptions := []SelectOption{
		{Value: "primary", Label: "primary"},
		{Value: "subagent", Label: "subagent"},
	}

	// Build invocable-by options from all active agents.
	var invokeOptions []SelectOption
	var invokeSelected []string
	for _, a := range v.wf.ActiveAgents() {
		if a.AgentID == id {
			continue
		}
		invokeOptions = append(invokeOptions, SelectOption{Value: a.AgentID, Label: a.AgentID})
	}
	if agent.TaskPermissions != nil {
		invokeSelected = agent.TaskPermissions.CanInvoke
	}

	fields := []FormField{
		{Key: "role", Label: i18n.T("tui.workflow.role"), Type: FieldSelect, Options: roleOptions, Default: string(agent.Role)},
		{Key: "mode", Label: i18n.T("tui.workflow.mode"), Type: FieldSelect, Options: modeOptions, Default: string(agent.Mode)},
		{Key: "can_invoke", Label: i18n.T("tui.workflow.can_invoke"), Type: FieldMultiSelect, Options: invokeOptions, DefaultMulti: invokeSelected},
	}

	v.shell.ShowInlineForm(InlineFormConfig{
		Title:  i18n.Tf("tui.workflow.configure_agent", id),
		Fields: fields,
		OnSubmit: func(values map[string]string, multi map[string][]string) {
			role := workflow.AgentRole(values["role"])
			mode := workflow.AgentMode(values["mode"])
			canInvoke := multi["can_invoke"]

			v.overrides.AgentOverrides = appendOrUpdateAgentOverride(
				v.overrides.AgentOverrides,
				workflow.AgentOverride{
					AgentID: id,
					Role:    &role,
					Mode:    &mode,
					TaskPermissions: &workflow.TaskPermOverride{
						CanInvoke: canInvoke,
					},
				},
			)
			v.dirty = true
			v.refreshWorkflow()
		},
	})
}

func (v *workflowView) editModes() {
	modeStr := strings.Join(v.wf.Modes.Available, ", ")
	v.shell.ShowInputModal(i18n.T("tui.workflow.modes_prompt"), modeStr, func(newValue string) {
		modes := strings.Split(newValue, ",")
		var cleaned []string
		for _, m := range modes {
			m = strings.TrimSpace(m)
			if m != "" {
				cleaned = append(cleaned, m)
			}
		}
		if len(cleaned) == 0 {
			v.shell.ShowToastMsg(i18n.T("tui.workflow.at_least_one_mode"), false)
			return
		}

		v.shell.ShowSelectModal(i18n.T("tui.workflow.default_mode"), toSelectOptions(cleaned), v.wf.Modes.Default, func(def string) {
			v.overrides.ModeOverrides = &workflow.ModesOverride{
				Available: &cleaned,
				Default:   &def,
			}
			v.dirty = true
			v.refreshWorkflow()
		})
	})
}

func (v *workflowView) addCheckpointOnEdge(edgeID string) {
	parts := strings.SplitN(edgeID, "→", 2)
	if len(parts) != 2 {
		return
	}
	afterCP := parts[0]

	v.shell.ShowInlineForm(InlineFormConfig{
		Title: i18n.T("tui.workflow.new_checkpoint"),
		Fields: []FormField{
			{Key: "id", Label: "ID", Type: FieldText, Required: true, Hint: "ex: cp-review-gate"},
			{Key: "label", Label: "Label", Type: FieldText, Required: true},
		},
		OnSubmit: func(values map[string]string, _ map[string][]string) {
			cpID := values["id"]
			label := values["label"]

			v.overrides.CheckpointOverrides = append(v.overrides.CheckpointOverrides, workflow.CheckpointOverride{
				ID:          cpID,
				Action:      workflow.ActionAdd,
				Label:       &label,
				InsertAfter: &afterCP,
			})
			v.dirty = true
			v.refreshWorkflow()
		},
	})
}

func (v *workflowView) deleteCheckpoint(id string) {
	cp := v.wf.FindCheckpoint(id)
	if cp == nil {
		return
	}
	if cp.Mandatory {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.workflow.checkpoint_mandatory"), false)
		}
		return
	}

	v.overrides.CheckpointOverrides = append(v.overrides.CheckpointOverrides, workflow.CheckpointOverride{
		ID:     id,
		Action: workflow.ActionRemove,
	})
	v.dirty = true
	v.refreshWorkflow()
}

func (v *workflowView) disableAgent(id string) {
	agent := v.wf.FindAgent(id)
	if agent == nil {
		return
	}
	if agent.Mandatory {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.workflow.agent_mandatory_disable"), false)
		}
		return
	}

	disabled := true
	v.overrides.AgentOverrides = appendOrUpdateAgentOverride(v.overrides.AgentOverrides, workflow.AgentOverride{
		AgentID:  id,
		Disabled: &disabled,
	})
	v.dirty = true
	v.refreshWorkflow()
}

// ---------------------------------------------------------------------------
// Override helpers
// ---------------------------------------------------------------------------

func appendOrUpdateCheckpointOverride(existing []workflow.CheckpointOverride, newOv workflow.CheckpointOverride) []workflow.CheckpointOverride {
	for i, ov := range existing {
		if ov.ID == newOv.ID && ov.Action == newOv.Action {
			existing[i] = newOv
			return existing
		}
	}
	return append(existing, newOv)
}

func appendOrUpdateAgentOverride(existing []workflow.AgentOverride, newOv workflow.AgentOverride) []workflow.AgentOverride {
	for i, ov := range existing {
		if ov.AgentID == newOv.AgentID {
			// Merge non-nil fields.
			if newOv.Role != nil {
				existing[i].Role = newOv.Role
			}
			if newOv.Mode != nil {
				existing[i].Mode = newOv.Mode
			}
			if newOv.Disabled != nil {
				existing[i].Disabled = newOv.Disabled
			}
			if newOv.TaskPermissions != nil {
				existing[i].TaskPermissions = newOv.TaskPermissions
			}
			if newOv.Position != nil {
				existing[i].Position = newOv.Position
			}
			return existing
		}
	}
	return append(existing, newOv)
}

func toSelectOptions(values []string) []SelectOption {
	opts := make([]SelectOption, len(values))
	for i, v := range values {
		opts[i] = SelectOption{Value: v, Label: v}
	}
	return opts
}

// ContextCommands provides workflow-specific omnibar commands.
func (v *workflowView) ContextCommands() []ContextCommand {
	var cmds []ContextCommand
	if v.cfg.IsLocked != nil && v.cfg.IsLocked() {
		return cmds
	}
	cmds = append(cmds,
		ContextCommand{ID: "workflow.save", Label: i18n.T("tui.workflow.cmd_save"), Action: func() { v.save() }},
		ContextCommand{ID: "workflow.reset", Label: i18n.T("tui.workflow.cmd_reset"), Action: func() { v.reset() }},
	)
	return cmds
}
