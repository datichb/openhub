// Package notifypolicy is what the user chose about being told: which events
// reach them and how (a system notification, a toast in the TUI, nothing),
// whether a sound plays, when oh stays quiet. It is read from hub.toml
// ([notifications]) by the daemon, which sends the system notifications, and
// by the TUI, which shows the toasts; both read it again when the file
// changes, so a setting applies without a restart.
package notifypolicy

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Channel is where an event is shown.
type Channel string

const (
	// ChannelSystem is a system notification (the daemon) and the toast of the
	// TUI when it is open: the most visible.
	ChannelSystem Channel = "system"
	// ChannelToast is the toast of the TUI only.
	ChannelToast Channel = "toast"
	// ChannelOff shows nothing.
	ChannelOff Channel = "off"
)

// Kind is a family of events the user can set apart.
type Kind string

// The kinds, as written in [notifications.events].
const (
	// KindCheckpoint, KindQuestion and KindPermission are the decisions a
	// session waits for.
	KindCheckpoint, KindQuestion, KindPermission Kind = "checkpoint", "question", "permission"
	// KindBudget is a budget that ran out.
	KindBudget Kind = "budget"
	// KindError is a session that failed or crashed.
	KindError Kind = "error"
	// KindCredential is a credential refused or a token expired (model
	// provider, tracker, git), and the other alerts that only the user can fix.
	KindCredential Kind = "credential"
	// KindStepDone is the end of a step of a session nobody is attached to.
	KindStepDone Kind = "step_done"
	// KindSessionComplete is a session or a workflow that finished.
	KindSessionComplete Kind = "session_complete"
	// KindRemote is a remote session that is ready or failed.
	KindRemote Kind = "remote"
	// KindMemory is the memory cap of the tool servers.
	KindMemory Kind = "memory"
	// KindSleep is a session put to sleep with a decision waiting.
	KindSleep Kind = "sleep"
	// KindReview is a merge request ready for review; KindVerdict its verdict.
	KindReview, KindVerdict Kind = "review", "verdict"
	// KindClaimTransferred is a claim handed to me; KindClaim the other claim
	// events (taken, released, conflict); KindClaimExpired a claim of mine
	// that expired or is about to.
	KindClaimTransferred, KindClaim, KindClaimExpired Kind = "claim_transferred", "claim", "claim_expired"
	// KindTrackerSync is the summary of a tracker synchronization.
	KindTrackerSync Kind = "tracker_sync"
	// KindWiki is a wiki proposal, accepted or refused.
	KindWiki Kind = "wiki"
	// KindConfigChanged is a change of the configuration of the team.
	KindConfigChanged Kind = "config_changed"
)

// defaults is the channel of each kind when the user chose none: the matrix of
// the notifications (docs/notifications). The team events that concern others
// (review, claims, wiki, configuration) are announced to the webhook of the
// team; here they are off until the user asks for a toast.
var defaults = map[Kind]Channel{
	KindCheckpoint: ChannelSystem, KindQuestion: ChannelSystem, KindPermission: ChannelSystem,
	KindBudget: ChannelSystem, KindError: ChannelSystem, KindCredential: ChannelSystem,
	KindStepDone: ChannelToast, KindSessionComplete: ChannelSystem, KindRemote: ChannelSystem,
	KindMemory: ChannelSystem, KindSleep: ChannelSystem,
	KindReview: ChannelOff, KindVerdict: ChannelOff,
	KindClaimTransferred: ChannelToast, KindClaim: ChannelOff, KindClaimExpired: ChannelSystem,
	KindTrackerSync: ChannelToast, KindWiki: ChannelOff, KindConfigChanged: ChannelOff,
}

// Kinds lists the kinds in the order the settings show them.
var Kinds = []Kind{
	KindCheckpoint, KindQuestion, KindPermission, KindSleep, KindBudget, KindError, KindCredential,
	KindStepDone, KindSessionComplete, KindRemote, KindMemory,
	KindReview, KindVerdict, KindClaimTransferred, KindClaim, KindClaimExpired,
	KindTrackerSync, KindWiki, KindConfigChanged,
}

// Channels lists the values of an event setting.
var Channels = []Channel{ChannelSystem, ChannelToast, ChannelOff}

// Default is the channel of a kind when the user chose none.
func Default(k Kind) Channel {
	if c, ok := defaults[k]; ok {
		return c
	}
	return ChannelToast
}

// Settings is the [notifications] section of hub.toml.
type Settings struct {
	// System turns the system notifications of the daemon on (default) or off.
	System *bool `toml:"system,omitempty"`
	// Sound plays the sound of the system with a notification (default true).
	Sound *bool `toml:"sound,omitempty"`
	// QuietHours is a period (« 22:00-08:00 ») without system notification
	// nor sound; the toasts stay.
	QuietHours string `toml:"quiet_hours,omitempty"`
	// WhenAttached sends the system notification of a session that a terminal
	// is attached to (default false: the person is looking at it).
	WhenAttached bool `toml:"when_attached,omitempty"`
	// WhenTUIFocused sends the system notification while a TUI of oh is open
	// (default true: oh cannot tell which window is in front; false = the toast
	// of the TUI is enough).
	WhenTUIFocused *bool `toml:"when_tui_focused,omitempty"`
	// Events sets the channel of a kind: system | toast | off.
	Events map[string]string `toml:"events,omitempty"`
}

func orTrue(b *bool) bool { return b == nil || *b }

// SystemOn reports whether the system notifications are on.
func (s Settings) SystemOn() bool { return orTrue(s.System) }

// SoundOn reports whether a sound plays.
func (s Settings) SoundOn() bool { return orTrue(s.Sound) }

// TUIFocusedOn reports whether the system notifications go on while a TUI is
// open.
func (s Settings) TUIFocusedOn() bool { return orTrue(s.WhenTUIFocused) }

// ChannelFor is where events of kind k are shown: the choice of the user, else
// the default. A value that is not a channel counts as no choice.
func (s Settings) ChannelFor(k Kind) Channel {
	if v, ok := s.Events[string(k)]; ok {
		switch c := Channel(strings.ToLower(strings.TrimSpace(v))); c {
		case ChannelSystem, ChannelToast, ChannelOff:
			return c
		}
	}
	return Default(k)
}

// ToastOn reports whether the toast of a kind is shown.
func (s Settings) ToastOn(k Kind) bool { return s.ChannelFor(k) != ChannelOff }

// SystemFor reports whether a system notification is sent for a kind.
func (s Settings) SystemFor(k Kind) bool { return s.SystemOn() && s.ChannelFor(k) == ChannelSystem }

// period parses « HH:MM-HH:MM » into minutes after midnight.
func period(text string) (from, to int, err error) {
	var h1, m1, h2, m2 int
	if n, e := fmt.Sscanf(strings.TrimSpace(text), "%d:%d-%d:%d", &h1, &m1, &h2, &m2); e != nil || n != 4 ||
		h1 < 0 || h1 > 23 || h2 < 0 || h2 > 23 || m1 < 0 || m1 > 59 || m2 < 0 || m2 > 59 {
		return 0, 0, fmt.Errorf("quiet_hours %q: want HH:MM-HH:MM", text)
	}
	return h1*60 + m1, h2*60 + m2, nil
}

// Quiet reports whether now is within the quiet hours. A period may cross
// midnight (22:00-08:00). An empty or unreadable period is never quiet.
func (s Settings) Quiet(now time.Time) bool {
	if strings.TrimSpace(s.QuietHours) == "" {
		return false
	}
	from, to, err := period(s.QuietHours)
	if err != nil || from == to {
		return false
	}
	m := now.Hour()*60 + now.Minute()
	if from < to {
		return m >= from && m < to
	}
	return m >= from || m < to
}

// Validate checks what a user typed: the quiet hours, the kinds and channels.
func (s Settings) Validate() error {
	if strings.TrimSpace(s.QuietHours) != "" {
		if _, _, err := period(s.QuietHours); err != nil {
			return err
		}
	}
	for _, k := range s.EventKeys() {
		if _, ok := defaults[Kind(k)]; !ok {
			return fmt.Errorf("events.%s: unknown event (%s)", k, kindList())
		}
		switch Channel(strings.ToLower(strings.TrimSpace(s.Events[k]))) {
		case ChannelSystem, ChannelToast, ChannelOff:
		default:
			return fmt.Errorf("events.%s = %q: want system, toast or off", k, s.Events[k])
		}
	}
	return nil
}

// EventKeys are the kinds the user set, sorted.
func (s Settings) EventKeys() []string {
	keys := make([]string, 0, len(s.Events))
	for k := range s.Events {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func kindList() string {
	names := make([]string, len(Kinds))
	for i, k := range Kinds {
		names[i] = string(k)
	}
	return strings.Join(names, ", ")
}
