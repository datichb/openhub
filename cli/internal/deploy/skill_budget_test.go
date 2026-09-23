package deploy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComputeAgentBudget(t *testing.T) {
	// Setup temp dirs
	tmp := t.TempDir()
	agentsDir := filepath.Join(tmp, "agents")
	skillsDir := filepath.Join(tmp, "skills", "shared")
	_ = os.MkdirAll(agentsDir, 0o755)
	_ = os.MkdirAll(skillsDir, 0o755)

	// Create a mock skill
	skillContent := `---
name: test-skill
description: A test skill
---

# Test Skill

Rule 1: do this.
Rule 2: do that.
Rule 3: never do this.
`
	_ = os.WriteFile(filepath.Join(skillsDir, "test-skill.md"), []byte(skillContent), 0o644)

	// Create a mock agent
	agentContent := `---
id: test-agent
label: TestAgent
mode: primary
skills: [shared/test-skill]
native_skills: [shared/test-skill]
---

# Test Agent

You are a test agent. Follow the rules.
`
	agentPath := filepath.Join(agentsDir, "test-agent.md")
	_ = os.WriteFile(agentPath, []byte(agentContent), 0o644)

	budget, err := ComputeAgentBudget(agentPath, filepath.Join(tmp, "skills"))
	if err != nil {
		t.Fatalf("ComputeAgentBudget failed: %v", err)
	}

	if budget.AgentID != "test-agent" {
		t.Errorf("expected AgentID=test-agent, got %s", budget.AgentID)
	}
	if budget.AgentMode != "primary" {
		t.Errorf("expected AgentMode=primary, got %s", budget.AgentMode)
	}

	// Body should have some lines (the "# Test Agent" + text)
	if budget.BodyLines < 2 {
		t.Errorf("expected BodyLines >= 2, got %d", budget.BodyLines)
	}

	// Bucket A should have 1 entry
	if len(budget.BucketA) != 1 {
		t.Fatalf("expected 1 Bucket A entry, got %d", len(budget.BucketA))
	}
	if budget.BucketA[0].SkillRef != "shared/test-skill" {
		t.Errorf("expected SkillRef=shared/test-skill, got %s", budget.BucketA[0].SkillRef)
	}
	if budget.BucketA[0].Lines < 3 {
		t.Errorf("expected skill lines >= 3, got %d", budget.BucketA[0].Lines)
	}

	// Bucket B should also have 1 entry
	if len(budget.BucketB) != 1 {
		t.Fatalf("expected 1 Bucket B entry, got %d", len(budget.BucketB))
	}

	// Total A should match
	if budget.TotalALines != budget.BucketA[0].Lines {
		t.Errorf("TotalALines mismatch: %d vs %d", budget.TotalALines, budget.BucketA[0].Lines)
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"hello\n", 1},
		{"hello", 1},
		{"a\nb\nc\n", 3},
		{"a\nb\nc", 3},
	}

	for _, tt := range tests {
		got := countLines([]byte(tt.input))
		if got != tt.expected {
			t.Errorf("countLines(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestEstimateTokens(t *testing.T) {
	// 100 lines -> ~250 tokens
	got := estimateTokens(100)
	if got != 250 {
		t.Errorf("estimateTokens(100) = %d, want 250", got)
	}
}
