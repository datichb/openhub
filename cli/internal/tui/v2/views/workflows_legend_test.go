package views

import (
	"strings"
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Catalogue: column header, legend and runtime / risk in words.
func TestWorkflowCatalogLegendAndColumns(t *testing.T) {
	var created CatalogNew
	v := NewWorkflowCatalogView(WorkflowCatalogConfig{New: func(n CatalogNew) { created = n }})
	sh := &formShell{}
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() {
		v.Mount(content, app)
		v.SetData(CatalogData{Team: "core", Project: "web", Editable: true, Entries: []CatalogEntry{
			{ID: "ticket", Ref: "team:ticket", Layer: "team", Version: 3, Valid: true, Risk: "write",
				Runtimes: []string{"local", "container"}, Chain: []string{"hub:ticket", "team:ticket"}},
			{ID: "review", Ref: "hub:review", Layer: "hub", ReadOnly: true, Valid: true, Risk: "read"},
		}})
	})
	var first, line, detail string
	onLoop(app, func() {
		items := v.list.GetItems()
		require.True(t, items[0].IsHeader)
		first = items[0].MainText
		selectRef(t, v, "team:ticket|ticket")
		_, it, _ := v.list.CurrentItem()
		line = it.MainText + " " + it.SecondaryText
		detail = v.detail.GetText(true)
	})
	assert.Contains(t, first, i18n.T("tui.catalog.col.risk"))
	assert.Contains(t, line, theme.IconRuntimeLocal+" "+theme.IconRuntimeContainer)
	assert.Contains(t, line, theme.IconExtends+" hub:ticket")
	assert.Contains(t, detail, RiskDescription("write"))
	assert.Contains(t, detail, theme.IconRuntimeContainer+" "+i18n.T("tui.catalog.runtime.container"))
	assert.Contains(t, detail, i18n.T("tui.catalog.legend.title"))
	assert.Contains(t, detail, i18n.T("tui.catalog.legend.queued"))

	// n: the source is chosen among the references (shadowed parents too),
	// « other… » asks for a typed one.
	onLoop(app, func() { v.HandleKey(key('n')) })
	require.NotNil(t, sh.form)
	src := sh.form.Fields[1]
	require.Equal(t, FieldSelect, src.Type)
	var vals []string
	for _, o := range src.Options {
		vals = append(vals, o.Value)
	}
	assert.Equal(t, []string{"", "hub:review", "hub:ticket", "team:ticket", catalogSourceOther}, vals)
	sh.form.OnSubmit(map[string]string{"kind": CatalogNewExtends, "source": catalogSourceOther, "layer": "project", "id": "x"}, nil)
	require.NotNil(t, sh.input)
	sh.input(" team:other ")
	assert.Equal(t, CatalogNew{Kind: CatalogNewExtends, Source: "team:other", ID: "x", Layer: "project"}, created)
}

func formDefaults(c *InlineFormConfig) map[string]string {
	out := map[string]string{}
	for _, f := range c.Fields {
		out[f.Key] = f.Default
	}
	return out
}

func formField(t *testing.T, c *InlineFormConfig, key string) FormField {
	t.Helper()
	for _, f := range c.Fields {
		if f.Key == key {
			return f
		}
	}
	t.Fatalf("no field %s", key)
	return FormField{}
}

// Risk and isolation choices carry their description; extends is chosen
// among the allowed workflows.
func TestWorkflowEditorDescribedChoicesAndExtends(t *testing.T) {
	env := newEditorEnv(t, WorkflowEditorConfig{Workflows: []string{"hub:ticket", "hub:explore"}})
	env.selectField(t, i18n.T("tui.editor.field.risk"))
	onLoop(env.app, func() { env.v.activate(env.v.selectedRef()) })
	labels := map[string]string{}
	for _, o := range env.sh.selectOpt {
		labels[o.Value] = o.Label
	}
	assert.Equal(t, "write — "+i18n.T("tui.catalog.risk_desc.write"), labels["write"])
	assert.Equal(t, "read — "+i18n.T("tui.catalog.risk_desc.read"), labels["read"])

	env.selectField(t, i18n.T("tui.editor.field.extends"))
	onLoop(env.app, func() { env.v.activate(env.v.selectedRef()) })
	var vals []string
	for _, o := range env.sh.selectOpt {
		vals = append(vals, o.Value)
	}
	assert.Equal(t, []string{unsetValue, "hub:explore", "hub:ticket", catalogSourceOther}, vals)
	onLoop(env.app, func() { env.sh.selectFn("hub:explore") })
	assert.Contains(t, env.text(), "extends: hub:explore")
	onLoop(env.app, func() { env.sh.selectFn(catalogSourceOther) })
	require.NotNil(t, env.sh.input)
	onLoop(env.app, func() { env.sh.input("hub:ticket") })
	assert.Contains(t, env.text(), "extends: hub:ticket")

	// entry.selectable is exposed.
	env.selectField(t, i18n.T("tui.editor.field.selectable"))
	onLoop(env.app, func() { env.v.activate(env.v.selectedRef()) })
	onLoop(env.app, func() { env.sh.selectFn("true") })
	assert.Contains(t, env.text(), "selectable: true")
}

// Agent form: calls among the members (multi-select); checkpoint form:
// unlocks; graph detail: delegations and unreachable agents.
func TestWorkflowEditorAgentFlow(t *testing.T) {
	env := newEditorEnv(t, WorkflowEditorConfig{})
	onLoop(env.app, func() { env.v.setSection(edGraph) })
	onLoop(env.app, func() { env.v.editAgent(env.v.spec(), "orchestrator-dev") })
	require.NotNil(t, env.sh.form)
	calls := formField(t, env.sh.form, "calls")
	assert.Equal(t, FieldMultiSelect, calls.Type)
	var opts []string
	for _, o := range calls.Options {
		opts = append(opts, o.Value)
	}
	assert.ElementsMatch(t, []string{"orchestrator-dev", "developer", "reviewer", "documentarian"}, opts)
	assert.Contains(t, formField(t, env.sh.form, "role").Options[1].Label, i18n.T("tui.editor.opt.role.independent"))
	form := env.sh.form
	onLoop(env.app, func() { form.OnSubmit(formDefaults(form), map[string][]string{"calls": {"developer"}}) })
	e, err := workflow.ParseDocEdit([]byte(env.text()))
	require.NoError(t, err)
	l, ok := e.Strings("agents", "orchestrator-dev", "calls")
	require.True(t, ok)
	assert.Equal(t, []string{"developer"}, l)

	env.waitCheck(t)
	onLoop(env.app, func() { env.v.editCheckpoint(env.v.spec(), "cp-2") })
	unlocks := formField(t, env.sh.form, "unlocks")
	assert.Equal(t, FieldMultiSelect, unlocks.Type)
	assert.Len(t, unlocks.Options, len(workflow.UnlockOps))
	form = env.sh.form
	onLoop(env.app, func() { form.OnSubmit(formDefaults(form), map[string][]string{"unlocks": {"commit"}}) })
	e, err = workflow.ParseDocEdit([]byte(env.text()))
	require.NoError(t, err)
	l, _ = e.Strings("checkpoints", "cp-2", "unlocks")
	assert.Equal(t, []string{"commit"}, l)

	env.waitCheck(t)
	var detail string
	onLoop(env.app, func() {
		chk := *env.v.check
		chk.Delegation = map[string][]string{"orchestrator-dev": {"developer"}}
		env.v.SetCheck(&chk)
		env.v.setSection(edGraph)
		env.v.graph.Select("developer")
		env.v.showGraphDetail()
		detail = env.v.detail.GetText(true)
	})
	assert.Contains(t, detail, i18n.T("tui.editor.graph.called_by"))
	assert.Contains(t, detail, "orchestrator-dev")
	assert.NotContains(t, detail, i18n.T("tui.editor.graph.unreachable"))
	assert.Contains(t, detail, i18n.T("tui.editor.graph.edge_help"))
	onLoop(env.app, func() {
		env.v.graph.Select("reviewer")
		env.v.showGraphDetail()
		detail = env.v.detail.GetText(true)
	})
	assert.Contains(t, detail, i18n.T("tui.editor.graph.unreachable"))
	onLoop(env.app, func() {
		env.v.graph.Select("documentarian")
		env.v.showGraphDetail()
		detail = env.v.detail.GetText(true)
	})
	assert.NotContains(t, detail, i18n.T("tui.editor.graph.unreachable"), "independent agents are outside the flow")
	assert.True(t, strings.Contains(detail, i18n.T("tui.editor.graph.delegates_to")))
}
