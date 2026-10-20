package views

import (
	"context"
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/modelcheck"
)

func catalogView(t *testing.T) (*ModelsView, *viewShell) {
	t.Helper()
	hubConfig(t, func(c *config.Config) { c.Models.Default = "anthropic/claude-sonnet-4-5" })
	v := NewModelsView(nil)
	v.SetCatalog(func(context.Context) modelcheck.Catalog {
		return modelcheck.NewCatalog(map[string]string{"reviewer": "quality", "planner": "planning"})
	})
	v.catalog = v.loadCatalog(context.Background())
	v.table = tview.NewTable()
	sh := &viewShell{}
	v.SetShell(sh)
	return v, sh
}

func formFieldByKey(t *testing.T, form *InlineFormConfig, key string) FormField {
	t.Helper()
	for _, f := range form.Fields {
		if f.Key == key {
			return f
		}
	}
	require.Failf(t, "field not found", "%s", key)
	return FormField{}
}

func TestModelsViewAddOverrideValidatesIdentifiers(t *testing.T) {
	v, sh := catalogView(t)
	v.addOverride()
	require.NotNil(t, sh.form)

	agent := formFieldByKey(t, sh.form, "agent_id")
	msg, invalid := agent.Validate("reviwer")
	assert.True(t, invalid)
	assert.Contains(t, msg, "reviewer", "the message proposes the close names")
	_, invalid = agent.Validate("reviewer")
	assert.False(t, invalid)

	model := formFieldByKey(t, sh.form, "model")
	_, invalid = model.Validate("claude-haiku")
	assert.True(t, invalid)
	_, invalid = model.Validate("anthropic/claude-haiku-4-5")
	assert.False(t, invalid)

	family := formFieldByKey(t, sh.form, "family")
	var labels []string
	for _, o := range family.Options {
		labels = append(labels, o.Value)
	}
	assert.Equal(t, []string{"planning", "quality"}, labels, "families come from the catalogue")

	// Submitting past the field checks still writes nothing.
	sh.form.OnSubmit(map[string]string{"scope": "agent", "agent_id": "nope", "model": "anthropic/claude-haiku-4-5"}, nil)
	sh.form.OnSubmit(map[string]string{"scope": "agent", "agent_id": "reviewer", "model": "haiku"}, nil)
	config.Reset()
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Empty(t, cfg.Models.Agents)

	sh.form.OnSubmit(map[string]string{"scope": "agent", "agent_id": "reviewer", "model": "anthropic/claude-haiku-4-5"}, nil)
	config.Reset()
	cfg, err = config.Load()
	require.NoError(t, err)
	assert.Equal(t, "anthropic/claude-haiku-4-5", cfg.Models.Agents["reviewer"])
}

func TestModelsViewAcceptsAgentWhenCatalogueUnavailable(t *testing.T) {
	v, _ := catalogView(t)
	v.catalog = modelcheck.Catalog{}
	msg, invalid := v.checkAgent("whatever")
	assert.False(t, invalid, "an unreadable catalogue only warns")
	assert.NotEmpty(t, msg)
}

func TestModelsViewEditRefusesBadModel(t *testing.T) {
	v, sh := catalogView(t)
	v.loadEntries()
	v.editEntry(0)
	require.NotNil(t, sh.input)

	sh.input("sonnet")
	config.Reset()
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "anthropic/claude-sonnet-4-5", cfg.Models.Default, "unchanged")
	require.NotEmpty(t, sh.toasts)
	assert.Contains(t, sh.toasts[len(sh.toasts)-1], "sonnet")

	sh.input("anthropic/claude-opus-4-1")
	config.Reset()
	cfg, err = config.Load()
	require.NoError(t, err)
	assert.Equal(t, "anthropic/claude-opus-4-1", cfg.Models.Default)
}
