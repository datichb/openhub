package deploy

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// AgentFrontmatter represents the parsed YAML frontmatter of an agent .md file.
type AgentFrontmatter struct {
	ID           string                 `yaml:"id"`
	Label        string                 `yaml:"label"`
	Description  string                 `yaml:"description"`
	Mode         string                 `yaml:"mode"`          // "primary" (default) | "subagent"
	Model        string                 `yaml:"model"`         // floor model (optional, e.g. "anthropic/claude-sonnet-4-6")
	Permission   map[string]interface{} `yaml:"permission"`    // structured permissions (nested maps for bash/task)
	Skills       []string               `yaml:"skills"`        // Bucket A — inline skills
	NativeSkills []string               `yaml:"native_skills"` // Bucket B — on-demand skills
	MCPServers   []string               `yaml:"mcpServers"`    // required MCP servers
}

// ParseAgentFrontmatter reads an agent .md file and extracts its YAML frontmatter.
// Returns an error if the file cannot be read or the frontmatter is malformed.
func ParseAgentFrontmatter(filePath string) (*AgentFrontmatter, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading agent file %s: %w", filePath, err)
	}
	return ParseAgentFrontmatterFromBytes(data)
}

// ParseAgentFrontmatterFromBytes parses YAML frontmatter from raw markdown bytes.
// The frontmatter must be delimited by "---" on the first line and a closing "---".
func ParseAgentFrontmatterFromBytes(data []byte) (*AgentFrontmatter, error) {
	fm, err := extractFrontmatter(data)
	if err != nil {
		return nil, err
	}

	var agent AgentFrontmatter
	if err := yaml.Unmarshal(fm, &agent); err != nil {
		return nil, fmt.Errorf("parsing frontmatter YAML: %w", err)
	}

	// Normalize mode: default to "primary" if empty
	if agent.Mode == "" {
		agent.Mode = "primary"
	}

	// Normalize permission values: yaml.v3 may decode nested maps as map[string]interface{}
	// with values that are still yaml-typed. We need to ensure all values are plain Go types.
	if agent.Permission != nil {
		agent.Permission = normalizeMap(agent.Permission)
	}

	return &agent, nil
}

// extractFrontmatter extracts the raw YAML bytes between the opening and closing "---" delimiters.
func extractFrontmatter(data []byte) ([]byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))

	// First line must be "---"
	if !scanner.Scan() {
		return nil, fmt.Errorf("empty file: no frontmatter found")
	}
	if strings.TrimSpace(scanner.Text()) != "---" {
		return nil, fmt.Errorf("file does not start with --- frontmatter delimiter")
	}

	// Collect lines until closing "---"
	var buf bytes.Buffer
	found := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			found = true
			break
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
	}

	if !found {
		return nil, fmt.Errorf("no closing --- frontmatter delimiter found")
	}

	if buf.Len() == 0 {
		return nil, fmt.Errorf("empty frontmatter")
	}

	return buf.Bytes(), nil
}

// normalizeMap recursively normalizes a map parsed from YAML to ensure all values
// are standard Go types (string, bool, int, float64, map[string]interface{}, []interface{}).
func normalizeMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = normalizeValue(v)
	}
	return result
}

// normalizeValue converts YAML-decoded values to standard Go types.
func normalizeValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		return normalizeMap(val)
	case map[interface{}]interface{}:
		// yaml.v3 sometimes produces this for nested maps
		result := make(map[string]interface{}, len(val))
		for k, v2 := range val {
			result[fmt.Sprintf("%v", k)] = normalizeValue(v2)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, item := range val {
			result[i] = normalizeValue(item)
		}
		return result
	default:
		return v
	}
}

// AgentFamily derives the family name from an agent's file path relative to the agents/ directory.
// For example: "planning/orchestrator.md" → "planning", "developer/developer.md" → "developer".
// If the agent is at the root level (no subdirectory), returns "".
func AgentFamily(relPath string) string {
	parts := strings.Split(relPath, "/")
	if len(parts) < 2 {
		// Agent is at root of agents/ directory (no family subfolder)
		return ""
	}
	return parts[0]
}
