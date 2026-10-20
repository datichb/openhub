package runsvc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/limits"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

func TestCheckAllowedModels(t *testing.T) {
	haiku := &sessionspec.ModelRef{Provider: "amazon-bedrock", Model: "eu.anthropic.claude-haiku-4-5"}
	opus := &sessionspec.ModelRef{Provider: "amazon-bedrock", Model: "eu.anthropic.claude-opus-4-6", Variant: "high"}
	spec := func(def *sessionspec.ModelRef, agents ...sessionspec.AgentDef) sessionspec.BundleSpec {
		return sessionspec.BundleSpec{Agents: agents, DefaultModel: def}
	}
	cases := []struct {
		name    string
		spec    sessionspec.BundleSpec
		allowed []string
		agent   string // agent named by the refusal ("" = none)
	}{
		{"no list", spec(nil, sessionspec.AgentDef{ID: "a", Model: opus}), nil, ""},
		{"all match", spec(nil, sessionspec.AgentDef{ID: "a", Model: haiku}, sessionspec.AgentDef{ID: "b", Model: haiku}), []string{"eu.anthropic.claude-haiku-*"}, ""},
		{"one agent outside", spec(nil, sessionspec.AgentDef{ID: "a", Model: haiku}, sessionspec.AgentDef{ID: "b", Model: opus}), []string{"eu.anthropic.claude-haiku-*"}, "b"},
		{"default model for an agent without one", spec(opus, sessionspec.AgentDef{ID: "a"}), []string{"eu.anthropic.claude-haiku-*"}, "a"},
		{"unused default model", spec(opus, sessionspec.AgentDef{ID: "a", Model: haiku}), []string{"eu.anthropic.claude-haiku-*"}, ""},
		{"no model resolved", spec(nil, sessionspec.AgentDef{ID: "a"}), []string{"eu.anthropic.claude-haiku-*"}, ""},
		{"variant ignored", spec(nil, sessionspec.AgentDef{ID: "a", Model: opus}), []string{"eu.anthropic.claude-opus-4-6"}, ""},
		{"provider prefixed pattern", spec(nil, sessionspec.AgentDef{ID: "a", Model: haiku}), []string{"amazon-bedrock/eu.*"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkAllowedModels(tc.spec, tc.allowed)
			if tc.agent == "" {
				assert.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, ErrModelNotAllowed)
			var me *ModelNotAllowedError
			require.ErrorAs(t, err, &me)
			assert.Equal(t, tc.agent, me.Agent)
			assert.Contains(t, err.Error(), tc.agent)
			assert.Contains(t, err.Error(), me.Model)
			assert.Contains(t, err.Error(), tc.allowed[0])
		})
	}
}

// F-04: with an allow-list, a launch whose agents resolve to a model it
// leaves out fails before anything starts, naming the agent and the model;
// the list of the restrictions applies as well as an explicit one, and so
// does a launch without interface.
func TestStartSessionRefusesAModelOutsideTheList(t *testing.T) {
	f := newRTFixture(t)
	f.svc.SessionsDir = filepath.Join(f.root, "sessions")
	f.b.Spec.Agents = []sessionspec.AgentDef{
		{ID: "lead", Mode: "primary", Model: &sessionspec.ModelRef{Provider: "amazon-bedrock", Model: "eu.anthropic.claude-haiku-4-5"}},
		{ID: "reviewer", Mode: "subagent", Model: &sessionspec.ModelRef{Provider: "amazon-bedrock", Model: "eu.anthropic.claude-opus-4-6"}},
	}
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		set  func(*StartRequest)
	}{
		{"restrictions", func(r *StartRequest) {
			r.Limits = limits.Resolve(limits.Input{Hub: limits.Limits{Models: []string{"eu.anthropic.claude-haiku-*"}}, ProjectID: "p1"})
		}},
		{"explicit list", func(r *StartRequest) { r.AllowedModels = []string{"eu.anthropic.claude-haiku-*"} }},
		{"headless", func(r *StartRequest) {
			r.Headless, r.Prompt = true, "go"
			r.AllowedModels = []string{"eu.anthropic.claude-haiku-*"}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := f.request(f.project)
			tc.set(&req)
			_, err := f.svc.StartSession(ctx, req)
			require.ErrorIs(t, err, ErrModelNotAllowed)
			assert.Contains(t, err.Error(), "reviewer")
			assert.Contains(t, err.Error(), "eu.anthropic.claude-opus-4-6")
			assert.Empty(t, f.ad.started, "no server started")
			assert.Empty(t, f.dc.grants, "no grant asked")
		})
	}

	req := f.request(f.project)
	req.AllowedModels = []string{"eu.anthropic.*"}
	_, err := f.svc.StartSession(ctx, req)
	assert.NoError(t, err, "every model of the agents is in the list")
}

// The plan is checked before its worktrees are created or its sessions
// stopped.
func TestStartChecksTheModelsBeforeCreatingAnything(t *testing.T) {
	f := newRTFixture(t)
	f.b.Spec.Agents = []sessionspec.AgentDef{{ID: "lead", Mode: "primary", Model: &sessionspec.ModelRef{Provider: "amazon-bedrock", Model: "eu.anthropic.claude-opus-4-6"}}}
	req := f.request(f.project)
	req.AllowedModels = []string{"eu.anthropic.claude-haiku-*"}
	wt := filepath.Join(f.root, "wt-new")
	plan := &RunPlan{Request: RunRequest{Base: req, ProjectPath: f.project},
		Sessions: []PlannedSession{{Location: Location{Path: wt, Kind: LocationWorktree, Branch: "feat/x", Create: true}}}}

	_, err := f.svc.Start(context.Background(), plan)
	require.ErrorIs(t, err, ErrModelNotAllowed)
	_, statErr := os.Stat(wt)
	assert.True(t, os.IsNotExist(statErr), "no worktree created")
	assert.Empty(t, f.ad.started)
}
