package config_test

import (
	"testing"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
)

func TestNotificationsSectionKeepsTheFormerSwitch(t *testing.T) {
	var c config.Config
	require.NoError(t, toml.Unmarshal([]byte(`
[session]
notify = "off"

[notifications]
sound = false
quiet_hours = "22:00-08:00"
when_attached = true

[notifications.events]
step_done = "system"
review = "toast"
`), &c))
	p := c.NotifyPolicy()
	assert.False(t, p.SystemOn(), "[session] notify = off still turns the system notifications off")
	assert.False(t, p.SoundOn())
	assert.True(t, p.WhenAttached)
	assert.Equal(t, "22:00-08:00", p.QuietHours)
	assert.Equal(t, "system", p.Events["step_done"])

	on := true
	c.Notifications.System = &on
	assert.True(t, c.NotifyPolicy().SystemOn(), "[notifications] system wins over the former switch")

	out, err := toml.Marshal(config.Config{})
	require.NoError(t, err)
	assert.NotContains(t, string(out), "notifications", "nothing is written when nothing was chosen")
}

// The section is read again from the file, and a value that is not valid is refused when saved.
func TestNotificationsAreValidatedWhenSaved(t *testing.T) {
	configtest.Home(t)
	require.NoError(t, config.Update(func(c *config.Config) error {
		c.Notifications.QuietHours = "22:00-08:00"
		c.Notifications.Events = map[string]string{"budget": "toast"}
		return nil
	}))
	got, err := config.Read()
	require.NoError(t, err)
	assert.Equal(t, "22:00-08:00", got.NotifyPolicy().QuietHours)
	assert.Equal(t, "toast", got.NotifyPolicy().Events["budget"])

	err = config.Update(func(c *config.Config) error {
		c.Notifications.QuietHours = "never"
		return nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quiet_hours")
	again, err := config.Read()
	require.NoError(t, err)
	assert.Equal(t, "22:00-08:00", again.Notifications.QuietHours, "nothing invalid was written")
}
