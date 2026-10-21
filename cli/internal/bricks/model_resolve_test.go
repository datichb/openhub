package bricks

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveAgentModel(t *testing.T) {
	t.Parallel()
	projectOverrides := &ModelOverrides{
		Default:  "claude-sonnet-4-5",
		Families: map[string]string{"quality": "claude-opus-4"},
		Agents:   map[string]string{"debugger": "claude-sonnet-4-6"},
	}
	hubOverrides := &ModelOverrides{
		Default:  "claude-haiku-3-5",
		Families: map[string]string{"planning": "claude-sonnet-4-6"},
		Agents:   map[string]string{"reviewer": "claude-opus-4"},
	}
	hubOnly := &ModelOverrides{
		Default:  "claude-sonnet-4-5",
		Families: map[string]string{"planning": "claude-sonnet-4-6"},
		Agents:   map[string]string{"reviewer": "claude-opus-4"},
	}

	tests := []struct {
		name             string
		agentID          string
		family           string
		project          *ModelOverrides
		hub              *ModelOverrides
		frontmatterModel string
		provider         string
		expected         string
	}{
		{name: "Level 1: project agent override wins", agentID: "debugger", family: "quality", project: projectOverrides, hub: hubOverrides,
			provider: "anthropic", expected: "anthropic/claude-sonnet-4-6"},
		{name: "Level 2: project family override (no agent override)", agentID: "reviewer", family: "quality", project: projectOverrides, hub: hubOverrides,
			provider: "anthropic", expected: "anthropic/claude-opus-4"},
		{name: "Level 3: project global (no agent/family override)", agentID: "designer", family: "design", project: projectOverrides, hub: hubOverrides,
			provider: "anthropic", expected: "anthropic/claude-sonnet-4-5"},
		{name: "Level 4: hub agent override", agentID: "reviewer", family: "quality", hub: hubOnly,
			provider: "anthropic", expected: "anthropic/claude-opus-4"},
		{name: "Level 5: hub family override", agentID: "orchestrator", family: "planning", hub: hubOnly,
			provider: "anthropic", expected: "anthropic/claude-sonnet-4-6"},
		{name: "Level 6: hub global model", agentID: "designer", family: "design", hub: hubOnly,
			provider: "anthropic", expected: "anthropic/claude-sonnet-4-5"},
		{name: "frontmatter is the floor when there is no override", agentID: "orchestrator", family: "planning",
			frontmatterModel: "anthropic/claude-sonnet-4-6", provider: "anthropic", expected: "anthropic/claude-sonnet-4-6"},
		{name: "no model at any level", agentID: "designer", family: "design", provider: "anthropic", expected: ""},
		{name: "bedrock provider normalizes the hub override", agentID: "reviewer", family: "quality",
			hub:      &ModelOverrides{Agents: map[string]string{"reviewer": "claude-opus-4"}},
			provider: "bedrock", expected: "amazon-bedrock/anthropic.claude-opus-4-6-v1"},
		{name: "bedrock provider normalizes the frontmatter model", agentID: "orchestrator", family: "planning",
			frontmatterModel: "anthropic/claude-sonnet-4-6", provider: "bedrock", expected: "amazon-bedrock/anthropic.claude-sonnet-4-6"},
		{name: "empty provider returns the model as is, without normalization", agentID: "orchestrator", family: "planning",
			frontmatterModel: "anthropic/claude-sonnet-4-6", expected: "anthropic/claude-sonnet-4-6"},
		{name: "project override beats hub for the same agent", agentID: "reviewer", family: "quality",
			project:          &ModelOverrides{Agents: map[string]string{"reviewer": "claude-sonnet-4-5"}},
			hub:              &ModelOverrides{Agents: map[string]string{"reviewer": "claude-opus-4"}},
			frontmatterModel: "anthropic/claude-opus-4", provider: "anthropic", expected: "anthropic/claude-sonnet-4-5"},
		{name: "family override beats the global model", agentID: "reviewer", family: "quality",
			project:  &ModelOverrides{Default: "claude-haiku-3-5", Families: map[string]string{"quality": "claude-opus-4"}},
			provider: "anthropic", expected: "anthropic/claude-opus-4"},
		{name: "an agent without family skips the family levels", agentID: "standalone",
			project:  &ModelOverrides{Default: "claude-sonnet-4-5", Families: map[string]string{"planning": "claude-opus-4"}},
			provider: "anthropic", expected: "anthropic/claude-sonnet-4-5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveAgentModel(tt.agentID, tt.family, tt.project, tt.hub, nil, tt.frontmatterModel, tt.provider)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeModelForProvider(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		model    string
		provider string
		expected string
	}{
		// Anthropic provider
		{"anthropic short name", "claude-opus-4", "anthropic", "anthropic/claude-opus-4"},
		{"anthropic already prefixed", "anthropic/claude-opus-4", "anthropic", "anthropic/claude-opus-4"},
		{"anthropic from bedrock format", "amazon-bedrock/anthropic.claude-opus-4-6-v1", "anthropic", "anthropic/claude-opus-4-6"},

		// Bedrock provider
		{"bedrock short name", "claude-opus-4", "bedrock", "amazon-bedrock/anthropic.claude-opus-4-6-v1"},
		{"bedrock from anthropic prefixed", "anthropic/claude-opus-4", "bedrock", "amazon-bedrock/anthropic.claude-opus-4-6-v1"},
		{"bedrock already bedrock format", "amazon-bedrock/anthropic.claude-opus-4-6-v1", "bedrock", "amazon-bedrock/anthropic.claude-opus-4-6-v1"},
		{"bedrock sonnet", "claude-sonnet-4-5", "bedrock", "amazon-bedrock/anthropic.claude-sonnet-4-5-20250929-v1:0"},
		{"bedrock sonnet 4-6", "claude-sonnet-4-6", "bedrock", "amazon-bedrock/anthropic.claude-sonnet-4-6"},
		{"bedrock unknown model fallback", "claude-future-5", "bedrock", "amazon-bedrock/anthropic.claude-future-5"},
		{"bedrock inference profile", "amazon-bedrock/eu.anthropic.claude-sonnet-4-6", "bedrock", "amazon-bedrock/anthropic.claude-sonnet-4-6"},
		{"bedrock inference profile irregular id", "amazon-bedrock/us.anthropic.claude-sonnet-4-5-20250929-v1:0", "bedrock", "amazon-bedrock/anthropic.claude-sonnet-4-5-20250929-v1:0"},
		{"bedrock variant kept", "claude-sonnet-4-5#high", "bedrock", "amazon-bedrock/anthropic.claude-sonnet-4-5-20250929-v1:0#high"},
		{"bedrock other vendor kept", "amazon-bedrock/eu.amazon.nova-pro-v1:0", "bedrock", "amazon-bedrock/amazon.nova-pro-v1:0"},
		{"anthropic from inference profile", "amazon-bedrock/eu.anthropic.claude-opus-4-6-v1", "anthropic", "anthropic/claude-opus-4-6"},
		{"anthropic variant kept", "anthropic/claude-opus-4-6#max", "anthropic", "anthropic/claude-opus-4-6#max"},

		// GitHub Copilot
		{"github-copilot sonnet", "claude-sonnet-4-5", "github-copilot", "github-copilot/claude-sonnet-4.5"},
		{"github-copilot opus", "claude-opus-4-6", "github-copilot", "github-copilot/claude-opus-4.6"},

		// OpenRouter
		{"openrouter", "claude-opus-4", "openrouter", "anthropic/claude-opus-4"},

		// Empty inputs
		{"empty model", "", "anthropic", ""},
		{"empty provider", "claude-opus-4", "", "claude-opus-4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeModelForProvider(tt.model, tt.provider)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractShortModelName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"claude-opus-4", "claude-opus-4"},
		{"anthropic/claude-opus-4", "claude-opus-4"},
		{"amazon-bedrock/anthropic.claude-opus-4-20250514-v1:0", "claude-opus-4"},
		{"amazon-bedrock/anthropic.claude-opus-4-6-v1", "claude-opus-4-6"},
		{"amazon-bedrock/anthropic.claude-sonnet-4-5-20250929-v1:0", "claude-sonnet-4-5"},
		{"amazon-bedrock/anthropic.claude-sonnet-4-6", "claude-sonnet-4-6"},
		{"amazon-bedrock/anthropic.claude-haiku-4-5-20251001-v1:0", "claude-haiku-4-5"},
		{"github-copilot/claude-sonnet-4.5", "claude-sonnet-4-5"},
		{"openrouter/claude-opus-4", "claude-opus-4"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractShortModelName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
