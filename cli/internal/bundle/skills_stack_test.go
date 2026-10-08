package bundle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/workflow"
)

func ticketBundleSkills(t *testing.T, project string, plugins []workflow.PluginRef) map[string]bool {
	t.Helper()
	hub, cat, env := shippedWorkflows(t)
	r, diags := workflow.Check(cat, workflow.Ref{Layer: workflow.LayerHub, ID: "ticket"}, nil, env)
	require.False(t, diags.HasErrors(), "%v", diags)
	spec := *r.Spec
	spec.Plugins = plugins
	b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: &spec, ProjectPath: project})
	require.NoError(t, err)
	out := map[string]bool{}
	for _, s := range b.Spec.Skills {
		out[s.ID] = true
	}
	return out
}

func writeProjectFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, rel)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, rel), []byte(content), 0o644))
}

// A8: in a FastAPI project, the ticket bundle has the Python and FastAPI
// skills, neither the frontend domain skills nor the skills of plugins the
// workflow does not load (context-mode, rtk).
func TestBundleSkillsFollowStackAndPlugins(t *testing.T) {
	api := t.TempDir()
	writeProjectFile(t, api, "pyproject.toml", "[project]\nname = \"api\"\ndependencies = [\"fastapi\"]\n")
	writeProjectFile(t, api, "app/main.py", "from fastapi import FastAPI\n")
	writeProjectFile(t, api, "htmlcov/index.html", "<html></html>")
	skills := ticketBundleSkills(t, api, nil)
	assert.True(t, skills["dev-standards-fastapi"], "%v", skills)
	assert.True(t, skills["dev-standards-python"], "%v", skills)
	for _, id := range []string{"dev-standards-frontend", "dev-standards-frontend-a11y", "dev-standards-frontend-data", "context-mode-usage", "rtk-usage"} {
		assert.False(t, skills[id], "%s shipped: %v", id, skills)
	}

	web := t.TempDir()
	writeProjectFile(t, web, "package.json", `{"dependencies":{"react":"^19"}}`)
	skills = ticketBundleSkills(t, web, []workflow.PluginRef{{ID: "context-mode"}})
	assert.True(t, skills["dev-standards-frontend"], "%v", skills)
	assert.True(t, skills["context-mode-usage"], "plugin loaded: %v", skills)
	assert.False(t, skills["rtk-usage"])
}

// The permissions of a plugin (permissions/plugin-tools.yaml: context-mode
// ctx_* tools, rtk shell) are given to the agents only when the workflow
// loads the plugin.
func TestPluginPermissionsFollowPlugins(t *testing.T) {
	hub, cat, env := shippedWorkflows(t)
	r, diags := workflow.Check(cat, workflow.Ref{Layer: workflow.LayerHub, ID: "quick"}, nil, env)
	require.False(t, diags.HasErrors(), "%v", diags)
	perms := func(plugins []workflow.PluginRef) string {
		spec := *r.Spec
		spec.Plugins = plugins
		b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: &spec})
		require.NoError(t, err)
		var all []string
		for _, a := range b.Spec.Agents {
			for _, p := range a.Permissions {
				all = append(all, string(p.Action)+" "+p.Resource)
			}
		}
		return strings.Join(all, "\n")
	}
	without := perms(nil)
	assert.NotContains(t, without, "ctx_")
	assert.NotContains(t, without, "rtk ")
	with := perms([]workflow.PluginRef{{ID: "context-mode"}, {ID: "rtk"}})
	assert.Contains(t, with, "ctx_batch_execute")
	assert.Contains(t, with, "rtk *")
}

// An agent that names a skill left out of the bundle is told not to load it.
func TestAbsentSkillsNote(t *testing.T) {
	api := t.TempDir()
	writeProjectFile(t, api, "pyproject.toml", "[project]\ndependencies = [\"fastapi\"]\n")
	hub, cat, env := shippedWorkflows(t)
	r, diags := workflow.Check(cat, workflow.Ref{Layer: workflow.LayerHub, ID: "ticket"}, nil, env)
	require.False(t, diags.HasErrors(), "%v", diags)
	b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec, ProjectPath: api})
	require.NoError(t, err)
	var body string
	for _, a := range b.Spec.Agents {
		if a.ID == "orchestrator-dev" {
			body = a.Body
		}
	}
	require.NotEmpty(t, body)
	assert.Contains(t, body, "Skills absentes de ce paquet de session")
	assert.Contains(t, body, "`dev-standards-frontend`")
	assert.Equal(t, "", absentSkillsNote(nil))
}
