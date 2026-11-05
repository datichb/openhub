package views

import (
	"errors"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/notifypolicy"
)

// notifyFields are the settings of the notifications of the hub (hub.toml
// [notifications]): what reaches the user and how. The daemon and the TUI read
// the file again when it changes, so a choice applies at once.
func (v *SettingsView) notifyFields() []configField {
	on := func(key string, label string, get func() bool, set func(bool)) configField {
		return configField{
			Key: key, Kind: CfgFieldBool, Label: i18n.T(label + ".label"), Description: i18n.T(label + ".desc"),
			Get: func() string { return boolStr(get()) },
			Set: func(val string) { set(val == "true") },
		}
	}
	fields := []configField{
		{Kind: CfgFieldSectionHeader, Label: i18n.T("v54n.settings.section")},
		on("notif_system", "v54n.settings.system",
			func() bool { return v.live.NotifyPolicy().SystemOn() },
			func(b bool) { v.live.Notifications.System = &b }),
		on("notif_sound", "v54n.settings.sound",
			func() bool { return v.live.NotifyPolicy().SoundOn() },
			func(b bool) { v.live.Notifications.Sound = &b }),
		{
			Key: "notif_quiet_hours", Kind: CfgFieldString, Label: i18n.T("v54n.settings.quiet.label"), Description: i18n.T("v54n.settings.quiet.desc"),
			Placeholder: "22:00-08:00",
			Validator: &FieldValidator{AllowEmpty: true, Check: func(val string) error {
				if err := (notifypolicy.Settings{QuietHours: strings.TrimSpace(val)}).Validate(); err != nil {
					return errors.New(i18n.T("v54n.settings.quiet.invalid"))
				}
				return nil
			}},
			Get: func() string { return v.live.Notifications.QuietHours },
			Set: func(val string) { v.live.Notifications.QuietHours = strings.TrimSpace(val) },
		},
		on("notif_when_attached", "v54n.settings.attached",
			func() bool { return v.live.Notifications.WhenAttached },
			func(b bool) { v.live.Notifications.WhenAttached = b }),
		on("notif_when_tui", "v54n.settings.tui",
			func() bool { return v.live.NotifyPolicy().TUIFocusedOn() },
			func(b bool) { v.live.Notifications.WhenTUIFocused = &b }),
		{Kind: CfgFieldSubHeader, Label: i18n.T("v54n.settings.events")},
	}
	options := []SelectOption{
		{Label: i18n.T("v54n.settings.channel.system"), Value: string(notifypolicy.ChannelSystem)},
		{Label: i18n.T("v54n.settings.channel.toast"), Value: string(notifypolicy.ChannelToast)},
		{Label: i18n.T("v54n.settings.channel.off"), Value: string(notifypolicy.ChannelOff)},
	}
	for _, k := range notifypolicy.Kinds {
		fields = append(fields, configField{
			Key: "notif_event_" + string(k), Kind: CfgFieldSelect,
			Label: i18n.T("v54n.settings.kind." + string(k) + ".label"), Description: i18n.T("v54n.settings.kind." + string(k) + ".desc"),
			Options:   options,
			Validator: &FieldValidator{AllowedValues: []string{"system", "toast", "off"}},
			Get:       func() string { return string(v.live.NotifyPolicy().ChannelFor(k)) },
			Set: func(val string) {
				// Only a choice that differs from the default is written.
				if notifypolicy.Channel(val) == notifypolicy.Default(k) {
					delete(v.live.Notifications.Events, string(k))
					if len(v.live.Notifications.Events) == 0 {
						v.live.Notifications.Events = nil
					}
					return
				}
				if v.live.Notifications.Events == nil {
					v.live.Notifications.Events = map[string]string{}
				}
				v.live.Notifications.Events[string(k)] = val
			},
		})
	}
	return fields
}

// lockPushLabels shows the push_labels setting of the hub as locked when the
// team enforces it (tracker.push_labels_enforced): the member's choice would
// be ignored.
func (v *SettingsView) lockPushLabels() {
	for i := range v.fields {
		if v.fields[i].Key == "push_labels" {
			v.fields[i].Locked = func() bool { return v.pushLabelsEnforced }
			v.fields[i].Source = func() string {
				if v.pushLabelsEnforced {
					return i18n.T("v54n.settings.push_labels_locked")
				}
				return ""
			}
		}
	}
}
