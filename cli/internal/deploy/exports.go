package deploy

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The functions below expose the deploy building blocks to the session bundle
// builder (internal/bundle) without changing the behaviour of `oh deploy`.
// They will move to the bundle service when per-project deployment is removed
// (v5 phase 3).

// AssembledAgent is an agent read from the hub with its Bucket A skills inlined.
type AssembledAgent struct {
	Frontmatter *AgentFrontmatter
	Family      string
	Path        string
	Body        string // markdown body without frontmatter, Bucket A skills appended
}

// AssembleAgentInline reads agents/<…>/<id>.md and returns its body followed
// by the given skill bodies (already resolved by the caller, without
// frontmatter), separated by horizontal rules.
func AssembleAgentInline(hubDir, agentPath string, skillBodies [][]byte) (*AssembledAgent, error) {
	data, err := os.ReadFile(agentPath)
	if err != nil {
		return nil, fmt.Errorf("reading agent file: %w", err)
	}
	fm, err := ParseAgentFrontmatterFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("parsing agent frontmatter %s: %w", agentPath, err)
	}
	_, body := splitFrontmatterAndBody(data)

	var out bytes.Buffer
	out.Write(bytes.TrimRight(body, "\n"))
	for _, content := range skillBodies {
		out.WriteString("\n\n---\n\n")
		out.Write(bytes.TrimRight(content, "\n"))
	}
	out.WriteByte('\n')

	rel, _ := filepath.Rel(filepath.Join(hubDir, "agents"), agentPath)
	return &AssembledAgent{Frontmatter: fm, Family: AgentFamily(rel), Path: agentPath, Body: out.String()}, nil
}

// SplitFrontmatter splits a markdown file into its frontmatter (with the
// "---" delimiters, nil when absent) and its body.
func SplitFrontmatter(data []byte) (frontmatter, body []byte) {
	return splitFrontmatterAndBody(data)
}

// FindAgentFiles maps agent IDs (file names without .md) to their paths under hubDir/agents.
func FindAgentFiles(hubDir string) (map[string]string, error) {
	root := filepath.Join(hubDir, "agents")
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		out[strings.TrimSuffix(d.Name(), ".md")] = path
		return nil
	})
	return out, err
}

// SkillSourcePath resolves a skill ref (hub path, then community registry).
func SkillSourcePath(hubDir, ref string) (string, error) {
	return resolveSkillPath(filepath.Join(hubDir, "skills"), ref)
}

// InstructionFiles returns the project documentation files that `oh deploy`
// registers as opencode instructions (relative paths).
func InstructionFiles(projectPath string, extra []string) []string {
	var out []string
	for _, f := range discoverInstructionFiles(projectPath, extra) {
		if s, ok := f.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// OpencodeProviderID maps a hub provider name to the opencode provider ID.
func OpencodeProviderID(provider string) string {
	return providerOpencodeName(provider)
}

// MCPServerUsable reports whether an MCP server has a usable token source
// (same rule as DeployMCP).
func MCPServerUsable(s MCPServerDef) bool { return checkMCPToken(s) }
