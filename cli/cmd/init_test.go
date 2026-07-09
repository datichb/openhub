package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildInitConfig_WithProvider(t *testing.T) {
	result := buildInitConfig("fr", "latest", "bedrock", nil)
	assert.Contains(t, result, `language = "fr"`)
	assert.Contains(t, result, `version = "latest"`)
	assert.Contains(t, result, `default_provider = "bedrock"`)
	assert.Contains(t, result, "enabled = false") // no MCP selected
}

func TestBuildInitConfig_WithMCP(t *testing.T) {
	result := buildInitConfig("en", "1.17.15", "anthropic", []string{"figma", "gitlab"})
	assert.Contains(t, result, `language = "en"`)
	assert.Contains(t, result, `version = "1.17.15"`)
	assert.Contains(t, result, `default_provider = "anthropic"`)

	// Figma and GitLab should be enabled
	lines := strings.Split(result, "\n")
	figmaSection := false
	gitlabSection := false
	gslidesSection := false
	for _, line := range lines {
		if strings.Contains(line, "[mcp.figma]") {
			figmaSection = true
			gitlabSection = false
			gslidesSection = false
		} else if strings.Contains(line, "[mcp.gitlab]") {
			figmaSection = false
			gitlabSection = true
			gslidesSection = false
		} else if strings.Contains(line, "[mcp.gslides]") {
			figmaSection = false
			gitlabSection = false
			gslidesSection = true
		}
		if strings.TrimSpace(line) == "enabled = true" {
			assert.True(t, figmaSection || gitlabSection, "only figma and gitlab should be enabled")
			assert.False(t, gslidesSection, "gslides should not be enabled")
		}
	}
}

func TestBuildInitConfig_NoMCP(t *testing.T) {
	result := buildInitConfig("fr", "latest", "openrouter", []string{})
	assert.Contains(t, result, `default_provider = "openrouter"`)
	// All MCP should be disabled
	assert.Equal(t, 3, strings.Count(result, "enabled = false"))
	assert.Equal(t, 0, strings.Count(result, "enabled = true"))
}
