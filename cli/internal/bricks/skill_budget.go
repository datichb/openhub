package bricks

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// BudgetEntry represents the token budget contribution of a single skill.
type BudgetEntry struct {
	SkillRef string // e.g. "posture/concision-posture"
	Lines    int
	Tokens   int // estimated: ~2.5 tokens per line
}

// AgentBudget represents the context window budget for a single agent.
type AgentBudget struct {
	AgentID      string
	AgentMode    string // "primary" or "subagent"
	BodyLines    int
	BodyTokens   int
	BucketA      []BudgetEntry // skills: (always inlined)
	BucketB      []BudgetEntry // native_skills: (on-demand)
	TotalALines  int
	TotalATokens int
}

// ComputeAgentBudget calculates the context window budget for a single agent.
func ComputeAgentBudget(agentPath, skillsDir string) (*AgentBudget, error) {
	fm, err := ParseAgentFrontmatter(agentPath)
	if err != nil {
		return nil, fmt.Errorf("parsing agent %s: %w", agentPath, err)
	}

	// Count agent body lines
	data, err := os.ReadFile(agentPath)
	if err != nil {
		return nil, fmt.Errorf("reading agent %s: %w", agentPath, err)
	}
	_, body := splitFrontmatterAndBody(data)
	bodyLines := countLines(body)

	budget := &AgentBudget{
		AgentID:    fm.ID,
		AgentMode:  fm.Mode,
		BodyLines:  bodyLines,
		BodyTokens: estimateTokens(bodyLines),
	}

	// Bucket A: skills (always inlined)
	for _, ref := range fm.Skills {
		lines := countSkillLines(skillsDir, ref)
		budget.BucketA = append(budget.BucketA, BudgetEntry{
			SkillRef: ref,
			Lines:    lines,
			Tokens:   estimateTokens(lines),
		})
		budget.TotalALines += lines
		budget.TotalATokens += estimateTokens(lines)
	}

	// Bucket B: native_skills (on-demand)
	for _, ref := range fm.NativeSkills {
		lines := countSkillLines(skillsDir, ref)
		budget.BucketB = append(budget.BucketB, BudgetEntry{
			SkillRef: ref,
			Lines:    lines,
			Tokens:   estimateTokens(lines),
		})
	}

	return budget, nil
}

// ComputeAllBudgets calculates budgets for all agents in a directory.
func ComputeAllBudgets(agentsDir, skillsDir string) ([]*AgentBudget, error) {
	var budgets []*AgentBudget

	err := filepath.Walk(agentsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		budget, err := ComputeAgentBudget(path, skillsDir)
		if err != nil {
			return nil // skip agents that fail to parse
		}
		budgets = append(budgets, budget)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Sort by total Bucket A cost descending
	sort.Slice(budgets, func(i, j int) bool {
		return budgets[i].TotalALines+budgets[i].BodyLines > budgets[j].TotalALines+budgets[j].BodyLines
	})

	return budgets, nil
}

func countSkillLines(skillsDir, skillRef string) int {
	skillPath, err := resolveSkillPath(skillsDir, skillRef)
	if err != nil {
		return 0
	}
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return 0
	}
	_, body := splitFrontmatterAndBody(data)
	return countLines(body)
}

func countLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	n := bytes.Count(data, []byte("\n"))
	// If the last byte is not a newline, count the partial line
	if data[len(data)-1] != '\n' {
		n++
	}
	return n
}

func estimateTokens(lines int) int {
	// ~2.5 tokens per line of markdown (mix of prose/code/tables)
	return (lines * 5) / 2
}
