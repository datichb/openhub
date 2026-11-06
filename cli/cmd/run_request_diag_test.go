package cmd

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/mcpresolve"
)

// The diagnostic and the launch agree: a service is reported exactly when
// the launch leaves it out.
func TestDiagnosticMatchesWhatTheLaunchLeavesOut(t *testing.T) {
	for _, svc := range mcpresolve.Services() {
		for _, enabled := range []bool{true, false} {
			for _, key := range []string{"", "openhub.mcp.x.token"} {
				for _, env := range []string{"", "from-env"} {
					t.Setenv(svc.TokenEnv, env)
					def := bricks.MCPServerDef{Name: svc.Name, Enabled: enabled, TokenKey: key, TokenEnv: svc.TokenEnv}
					leftOut := enabled && !bricks.MCPServerUsable(def)
					got := mcpresolve.DiagnoseToken(svc.Name, enabled, key, os.Getenv) != nil
					assert.Equal(t, leftOut, got, "%s enabled=%v key=%q env=%q", svc.Name, enabled, key, env)
				}
			}
		}
	}
}

type recordingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}
func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(string) slog.Handler      { return h }

// A session leaves out an enabled service that has no token, and says so
// with one readable warning (no raw attributes). The other servers start.
func TestSessionWarnsAboutAnEnabledServiceWithoutToken(t *testing.T) {
	i18ntest.Set(t, "en")
	isolatedHub(t)
	for _, svc := range mcpresolve.Services() {
		t.Setenv(svc.TokenEnv, "")
	}
	h := &recordingHandler{}
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })

	cfg := &config.Config{}
	cfg.MCP.Linear.Enabled = true // no token_key
	cfg.MCP.Gitlab = config.MCPServerConfig{Enabled: true, Token: "openhub.mcp.gitlab.token"}
	out := sessionMCP(t.Context(), &app.App{Config: cfg}, &domain.Project{ID: "p1"}, config.ResolvedTeamConfig{})

	var names []string
	for _, s := range out {
		names = append(names, s.Name)
	}
	assert.Contains(t, names, "gitlab")
	assert.NotContains(t, names, "linear", "left out")
	require.Len(t, h.records, 1, "one warning, for the service that is left out")
	r := h.records[0]
	assert.Equal(t, slog.LevelWarn, r.Level)
	assert.Zero(t, r.NumAttrs(), "a sentence, not raw attributes")
	assert.Contains(t, r.Message, "linear")
	assert.Contains(t, r.Message, "LINEAR_API_KEY")
	assert.Contains(t, r.Message, "oh mcp setup linear")
}
