package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/mcpresolve"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// The Google Slides server always calls slides.googleapis.com (as the Figma
// one calls api.figma.com): a URL recorded by the team, even imposed, is
// ignored without any warning, and oh mcp trust has nothing to pin.
func TestRecordedGslidesURLIsIgnoredWithoutWarning(t *testing.T) {
	urlOK := true
	shared := map[string]teamstate.SharedMCPConfig{
		"gslides": {URL: "http://slides.example.com", URLEnforced: &urlOK},
	}
	assert.Empty(t, enforcedURLNotices(&config.Config{}, shared), "no notice for the imposed URL of a service without URL")
	_, err := pinService("gslides")
	assert.Error(t, err, "nothing to pin for gslides")

	var out bytes.Buffer
	s := shared["gslides"]
	printMCPStatus(&out, "gslides", &s, config.MCPServerConfig{URL: "https://local.example"}, mcpresolve.Resolve(&s, config.MCPServerConfig{}))
	assert.NotContains(t, out.String(), "url:", "the status does not show a URL the server ignores")
	assert.NotContains(t, out.String(), "slides.example.com")
}
