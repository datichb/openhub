package cmd

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/notify"
	"github.com/datichb/openhub/cli/internal/sysnotify"
)

func TestDoctorNotificationsChecks(t *testing.T) {
	i18ntest.Set(t, "en")
	doctorTrackerHub(t, "http://127.0.0.1:1", "git@gitlab.example.com:acme/core.git", nil)
	seedTeamConfig(t, "core", "[notification]\nenabled = true\ntype = \"slack\"\nwebhook_url = \"https://hooks.slack.com/services/T0/B0/verysecretvalue\"\n")

	prevOffline, prevNotifier, prevReach := doctorOffline, systemNotifier, reachWebhook
	t.Cleanup(func() { doctorOffline, systemNotifier, reachWebhook = prevOffline, prevNotifier, prevReach })
	doctorOffline = func() bool { return false }
	systemNotifier = func() *sysnotify.Notifier {
		return &sysnotify.Notifier{LookPath: func(string) (string, error) { return "", errors.New("absent") }, GOOS: "linux"}
	}
	reachWebhook = func(context.Context, string) error { return errors.New("connection refused") }
	require.NoError(t, notifyOutbox().Add(notify.Spec{Type: "slack", URL: "https://hooks.slack.com/x"}, "hello", "core", nil))

	res := doctorCheckResults(t, doctorIDNotifications)
	require.Len(t, res, 3, "%+v", res)
	assert.True(t, res[0].Warn)
	assert.Contains(t, res[0].Detail, "No notification tool")
	assert.True(t, res[1].Warn)
	assert.Contains(t, res[1].Detail, "1 message")
	assert.True(t, res[2].Warn)
	assert.Contains(t, res[2].Detail, "connection refused")
	assert.NotContains(t, res[2].Detail, "verysecretvalue", "the webhook is masked")

	reachWebhook = func(context.Context, string) error { return nil }
	systemNotifier = func() *sysnotify.Notifier {
		return &sysnotify.Notifier{LookPath: func(string) (string, error) { return "/usr/bin/notify-send", nil }, GOOS: "linux"}
	}
	doctorOffline = func() bool { return true }
	res = doctorCheckResults(t, doctorIDNotifications)
	require.Len(t, res, 2, "offline: the webhook servers are not probed")
	assert.False(t, res[0].Warn)
}
