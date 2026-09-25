package opencode

import (
	"testing"

	"github.com/datichb/openhub/cli/internal/platform"
)

// --- parseHeadlessJSON tests (table-driven) ---

func TestParseHeadlessJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantModel   string
		wantCost    float64
		wantIn      int64
		wantOut     int64
		wantContent string
	}{
		{
			name:      "summary with usage object",
			input:     `{"model":"claude-opus","usage":{"input_tokens":100,"output_tokens":50},"cost":0.12}`,
			wantModel: "claude-opus",
			wantCost:  0.12,
			wantIn:    100,
			wantOut:   50,
		},
		{
			name:      "flat tokens fields (alternative shape)",
			input:     `{"tokens_input":200,"tokens_output":75,"model":"claude-sonnet"}`,
			wantModel: "claude-sonnet",
			wantIn:    200,
			wantOut:   75,
		},
		{
			name:        "text events JSONL",
			input:       "{\"type\":\"text\",\"text\":\"hello\"}\n{\"type\":\"text\",\"text\":\" world\"}",
			wantContent: "hello world",
		},
		{
			name:        "mixed JSON and non-JSON lines",
			input:       "Some log line\n{\"type\":\"text\",\"text\":\"ok\"}",
			wantContent: "ok",
		},
		{
			name:        "empty string",
			input:       "",
			wantContent: "", // no change to default
		},
		{
			name:        "malformed JSON",
			input:       "{broken json",
			wantContent: "", // should not panic, leave defaults
		},
		{
			name:        "multi-line with summary at end",
			input:       "{\"type\":\"text\",\"text\":\"result\"}\n{\"model\":\"opus\",\"cost\":0.5}",
			wantContent: "result",
			wantModel:   "opus",
			wantCost:    0.5,
		},
		{
			name:      "usage with reasoning tokens",
			input:     `{"model":"claude-opus","usage":{"input_tokens":100,"output_tokens":50,"reasoning_tokens":30},"cost":0.15}`,
			wantModel: "claude-opus",
			wantCost:  0.15,
			wantIn:    100,
			wantOut:   50,
		},
		{
			name:      "only model, no tokens",
			input:     `{"model":"gpt-4"}`,
			wantModel: "gpt-4",
		},
		{
			name:      "only cost",
			input:     `{"cost":1.23}`,
			wantCost:  1.23,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &platform.HeadlessResult{}
			parseHeadlessJSON(tt.input, result)

			if tt.wantModel != "" && result.Model != tt.wantModel {
				t.Errorf("Model = %q, want %q", result.Model, tt.wantModel)
			}
			if tt.wantCost != 0 && result.Cost != tt.wantCost {
				t.Errorf("Cost = %f, want %f", result.Cost, tt.wantCost)
			}
			if tt.wantIn != 0 && result.TokensIn != tt.wantIn {
				t.Errorf("TokensIn = %d, want %d", result.TokensIn, tt.wantIn)
			}
			if tt.wantOut != 0 && result.TokensOut != tt.wantOut {
				t.Errorf("TokensOut = %d, want %d", result.TokensOut, tt.wantOut)
			}
			if tt.wantContent != "" && result.Content != tt.wantContent {
				t.Errorf("Content = %q, want %q", result.Content, tt.wantContent)
			}
		})
	}
}

func TestParseHeadlessJSON_EmptyDoesNotMutate(t *testing.T) {
	result := &platform.HeadlessResult{
		Content: "original",
		Model:   "original-model",
	}
	parseHeadlessJSON("", result)

	if result.Content != "original" {
		t.Errorf("Content mutated: %q", result.Content)
	}
	if result.Model != "original-model" {
		t.Errorf("Model mutated: %q", result.Model)
	}
}

func TestParseHeadlessJSON_DoesNotPanic(t *testing.T) {
	// Various edge cases that should never panic
	inputs := []string{
		"",
		"  ",
		"\n\n\n",
		"{",
		"{}",
		"[1,2,3]",
		`{"type":"unknown"}`,
		`not json at all`,
		`{"usage": "not an object"}`,
		`{"usage": {"input_tokens": "not a number"}}`,
	}
	for _, input := range inputs {
		result := &platform.HeadlessResult{}
		parseHeadlessJSON(input, result) // should not panic
	}
}

// --- toStartOpts tests ---

func TestToStartOpts_AllFields(t *testing.T) {
	opts := platform.RunOpts{
		ProjectPath: "/path/to/project",
		ProjectID:   "proj-123",
		Agent:       "developer",
		Prompt:      "fix the bug",
		Provider:    "bedrock",
		Credentials: platform.Credentials{
			BearerToken: "token-abc",
			APIKey:      "key-xyz",
			AWSProfile:  "my-profile",
			AWSRegion:   "eu-west-1",
		},
		ResumeID:  "session-456",
		ExtraArgs: []string{"--verbose", "--debug"},
	}

	result := toStartOpts(opts)

	if result.ProjectPath != opts.ProjectPath {
		t.Errorf("ProjectPath = %q, want %q", result.ProjectPath, opts.ProjectPath)
	}
	if result.ProjectID != opts.ProjectID {
		t.Errorf("ProjectID = %q, want %q", result.ProjectID, opts.ProjectID)
	}
	if result.Agent != opts.Agent {
		t.Errorf("Agent = %q, want %q", result.Agent, opts.Agent)
	}
	if result.Prompt != opts.Prompt {
		t.Errorf("Prompt = %q, want %q", result.Prompt, opts.Prompt)
	}
	if result.Provider != opts.Provider {
		t.Errorf("Provider = %q, want %q", result.Provider, opts.Provider)
	}
	if result.BearerToken != opts.Credentials.BearerToken {
		t.Errorf("BearerToken = %q, want %q", result.BearerToken, opts.Credentials.BearerToken)
	}
	if result.APIKey != opts.Credentials.APIKey {
		t.Errorf("APIKey = %q, want %q", result.APIKey, opts.Credentials.APIKey)
	}
	if result.AWSProfile != opts.Credentials.AWSProfile {
		t.Errorf("AWSProfile = %q, want %q", result.AWSProfile, opts.Credentials.AWSProfile)
	}
	if result.AWSRegion != opts.Credentials.AWSRegion {
		t.Errorf("AWSRegion = %q, want %q", result.AWSRegion, opts.Credentials.AWSRegion)
	}
	if result.ResumeSessionID != opts.ResumeID {
		t.Errorf("ResumeSessionID = %q, want %q", result.ResumeSessionID, opts.ResumeID)
	}
	if len(result.ExtraArgs) != len(opts.ExtraArgs) {
		t.Fatalf("ExtraArgs len = %d, want %d", len(result.ExtraArgs), len(opts.ExtraArgs))
	}
	for i, arg := range result.ExtraArgs {
		if arg != opts.ExtraArgs[i] {
			t.Errorf("ExtraArgs[%d] = %q, want %q", i, arg, opts.ExtraArgs[i])
		}
	}
}

func TestToStartOpts_EmptyFields(t *testing.T) {
	opts := platform.RunOpts{}
	result := toStartOpts(opts)

	if result.ProjectPath != "" {
		t.Errorf("expected empty ProjectPath, got %q", result.ProjectPath)
	}
	if result.Agent != "" {
		t.Errorf("expected empty Agent, got %q", result.Agent)
	}
	if result.ResumeSessionID != "" {
		t.Errorf("expected empty ResumeSessionID, got %q", result.ResumeSessionID)
	}
	if result.ExtraArgs != nil {
		t.Errorf("expected nil ExtraArgs, got %v", result.ExtraArgs)
	}
}
