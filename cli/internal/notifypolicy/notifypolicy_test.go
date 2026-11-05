package notifypolicy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultsFollowTheMatrix(t *testing.T) {
	var s Settings
	for _, k := range []Kind{KindCheckpoint, KindQuestion, KindPermission, KindBudget, KindError, KindCredential, KindSessionComplete, KindRemote, KindClaimExpired} {
		assert.Equal(t, ChannelSystem, s.ChannelFor(k), k)
	}
	for _, k := range []Kind{KindStepDone, KindClaimTransferred, KindTrackerSync} {
		assert.Equal(t, ChannelToast, s.ChannelFor(k), k)
	}
	for _, k := range []Kind{KindReview, KindVerdict, KindClaim, KindWiki, KindConfigChanged} {
		assert.Equal(t, ChannelOff, s.ChannelFor(k), "%s is announced to the webhook, not to my screen", k)
	}
	for _, k := range Kinds {
		_, ok := defaults[k]
		assert.True(t, ok, "%s has a default", k)
	}
	assert.Len(t, Kinds, len(defaults))
	assert.True(t, s.SystemOn() && s.SoundOn() && s.TUIFocusedOn())
	assert.False(t, s.WhenAttached)
}

func TestTheUserChoiceWinsAndABadValueIsIgnored(t *testing.T) {
	off := false
	s := Settings{System: &off, Events: map[string]string{"step_done": "SYSTEM", "review": "toast", "budget": "loud"}}
	assert.Equal(t, ChannelSystem, s.ChannelFor(KindStepDone))
	assert.False(t, s.SystemFor(KindStepDone), "system notifications are off")
	assert.True(t, s.ToastOn(KindStepDone), "the toast stays")
	assert.Equal(t, ChannelToast, s.ChannelFor(KindReview))
	assert.Equal(t, ChannelSystem, s.ChannelFor(KindBudget), "an unknown value is no choice")
	assert.Error(t, s.Validate())
	assert.NoError(t, Settings{Events: map[string]string{"budget": "off"}}.Validate())
	assert.Error(t, Settings{Events: map[string]string{"nothing": "off"}}.Validate())
}

func TestQuietHours(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 11, 5, h, m, 0, 0, time.Local) }
	night := Settings{QuietHours: "22:00-08:00"}
	assert.True(t, night.Quiet(at(23, 0)))
	assert.True(t, night.Quiet(at(3, 30)))
	assert.True(t, night.Quiet(at(22, 0)))
	assert.False(t, night.Quiet(at(8, 0)), "the end is excluded")
	assert.False(t, night.Quiet(at(12, 0)))
	day := Settings{QuietHours: "12:00-14:00"}
	assert.True(t, day.Quiet(at(13, 0)))
	assert.False(t, day.Quiet(at(15, 0)))
	assert.False(t, Settings{}.Quiet(at(3, 0)))
	assert.False(t, Settings{QuietHours: "nonsense"}.Quiet(at(3, 0)))
	assert.Error(t, Settings{QuietHours: "25:00-08:00"}.Validate())
	assert.NoError(t, night.Validate())
}
