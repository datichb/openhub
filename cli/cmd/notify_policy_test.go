package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/notifypolicy"
)

// hub.toml is read again when it changes: a setting applies without a restart.
func TestPolicyReaderFollowsHubToml(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hub.toml")
	reads := 0
	r := newPolicyReader(notifypolicy.Settings{})
	r.path = func() string { return path }
	r.read = func() (*config.Config, error) {
		reads++
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		c := &config.Config{}
		if string(data) == "quiet" {
			c.Notifications.QuietHours = "22:00-08:00"
		}
		return c, nil
	}
	assert.Equal(t, notifypolicy.Settings{}, r.Get(), "no file: the start-up policy")
	require.NoError(t, os.WriteFile(path, []byte("quiet"), 0o644))
	assert.Equal(t, "22:00-08:00", r.Get().QuietHours)
	_ = r.Get()
	assert.Equal(t, 1, reads, "an unchanged file is not read again")

	require.NoError(t, os.WriteFile(path, []byte("calm!"), 0o644))
	future := time.Now().Add(time.Hour)
	require.NoError(t, os.Chtimes(path, future, future))
	assert.Empty(t, r.Get().QuietHours)
	assert.Equal(t, 2, reads)
}

func TestDecisionKindOfAnAlertIsACredential(t *testing.T) {
	alert := domain.Decision{Kind: domain.DecisionError, Payload: domain.DecisionPayload{Data: map[string]any{"oh_alert": "team_access"}}}
	assert.Equal(t, notifypolicy.KindCredential, decisionKind(alert))
	assert.Equal(t, notifypolicy.KindError, decisionKind(domain.Decision{Kind: domain.DecisionError}))
	assert.Equal(t, notifypolicy.KindCheckpoint, decisionKind(domain.Decision{Kind: domain.DecisionCheckpoint}))
}
