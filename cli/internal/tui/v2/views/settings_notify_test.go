package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/notifypolicy"
)

func notifySettings(t *testing.T) (*SettingsView, *config.Config) {
	t.Helper()
	shared := seedHome(t)
	v := NewSettingsView(SettingsViewConfig{GetConfig: func() *config.Config { return shared }, UpdateConfig: hubHost{shared}.update})
	v.live = shared
	v.buildFields()
	return v, shared
}

// Every kind of notification has its row, and a choice is written to the file
// only when it differs from the default.
func TestSettingsNotificationsFields(t *testing.T) {
	v, _ := notifySettings(t)
	for _, k := range notifypolicy.Kinds {
		f := settingsField(t, v, "notif_event_"+string(k))
		assert.Equal(t, string(notifypolicy.Default(k)), f.Get(), "the default is shown")
	}

	v.pushUndo()
	settingsField(t, v, "notif_event_step_done").Set("system")
	settingsField(t, v, "notif_event_budget").Set("system") // the default: nothing written
	settingsField(t, v, "notif_sound").Set("false")
	settingsField(t, v, "notif_quiet_hours").Set(" 22:00-08:00 ")
	settingsField(t, v, "notif_when_attached").Set("true")
	v.scheduleAutoSave()
	v.doSave()

	got, err := config.Read()
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"step_done": "system"}, got.Notifications.Events)
	assert.False(t, got.NotifyPolicy().SoundOn())
	assert.Equal(t, "22:00-08:00", got.Notifications.QuietHours)
	assert.True(t, got.Notifications.WhenAttached)
	assert.Nil(t, got.Notifications.System, "what was not touched is not written")

	// Back to the default: the key leaves the file.
	v.pushUndo()
	settingsField(t, v, "notif_event_step_done").Set("toast")
	v.scheduleAutoSave()
	v.doSave()
	got, err = config.Read()
	require.NoError(t, err)
	assert.Empty(t, got.Notifications.Events)

	q := settingsField(t, v, "notif_quiet_hours")
	assert.Error(t, q.Validator.Validate("nonsense"))
	assert.NoError(t, q.Validator.Validate(""))
	assert.NoError(t, q.Validator.Validate("23:30-07:00"))
}

func TestSettingsFormerSwitchIsShownOff(t *testing.T) {
	v, shared := notifySettings(t)
	shared.Session.Notify = "off"
	assert.Equal(t, "false", settingsField(t, v, "notif_system").Get())
	settingsField(t, v, "notif_system").Set("true")
	assert.True(t, shared.NotifyPolicy().SystemOn(), "the new setting wins over [session] notify")
}

// The push_labels setting of the hub is locked when the team imposes it.
func TestSettingsPushLabelsLockedByTheTeam(t *testing.T) {
	v, _ := notifySettings(t)
	f := settingsField(t, v, "push_labels")
	require.NotNil(t, f.Locked)
	assert.False(t, f.Locked())
	assert.Empty(t, f.Source())
	v.pushLabelsEnforced = true
	assert.True(t, f.Locked())
	assert.NotEmpty(t, f.Source())
}
