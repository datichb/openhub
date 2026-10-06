package views

import (
	"fmt"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// remoteFields is the « Distant » section of the hub settings: one block per
// remote target (oh-runner project). Changing a pipeline option takes effect
// at the next `oh remote setup`, which rewrites the generated pipeline.
func (v *SettingsView) remoteFields() []configField {
	fields := []configField{{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.settings.remote.section")}}
	targets := v.live.Remote.Targets
	if len(targets) == 0 {
		return append(fields, configField{Key: "remote_none", Kind: CfgFieldPlaceholder,
			Label: i18n.T("tui.settings.remote.none"), Description: i18n.T("tui.settings.remote.none_desc"),
			Get: func() string { return "" }})
	}
	for i := range targets {
		i := i
		t := func() int { return i } // index into the live slice (re-read at each call)
		name := targets[i].Name
		fields = append(fields,
			configField{Kind: CfgFieldSubHeader, Label: name},
			configField{Key: "remote_instance", Kind: CfgFieldReadonly, Label: i18n.T("tui.settings.remote.instance"),
				Get: func() string {
					tg := v.live.Remote.Targets[t()]
					return fmt.Sprintf("%s · %s", tg.URL, tg.Group)
				}},
			configField{Key: "remote_runner_project", Kind: CfgFieldReadonly, Label: i18n.T("tui.settings.remote.runner_project"),
				Description: i18n.T("tui.settings.remote.runner_project_desc"),
				Get:         func() string { return v.live.Remote.Targets[t()].RunnerProjectPath() }},
			configField{Key: "remote_tag", Kind: CfgFieldString, Label: i18n.T("tui.settings.remote.tag"),
				Description: i18n.T("tui.settings.remote.apply_desc"), Placeholder: "oh",
				Get: func() string { return v.live.Remote.Targets[t()].Tag },
				Set: func(val string) { v.live.Remote.Targets[t()].Tag = val }},
			configField{Key: "remote_builder", Kind: CfgFieldSelect, Label: i18n.T("tui.settings.remote.builder"),
				Description: i18n.T("tui.settings.remote.apply_desc"),
				Options: []SelectOption{
					{Label: i18n.T("tui.settings.remote.builder_kaniko"), Value: "kaniko"},
					{Label: i18n.T("tui.settings.remote.builder_dind"), Value: "dind"},
				},
				Validator: &FieldValidator{AllowedValues: []string{"kaniko", "dind"}, AllowEmpty: true},
				Get: func() string {
					if b := v.live.Remote.Targets[t()].Builder; b != "" {
						return b
					}
					return "kaniko"
				},
				Set: func(val string) { v.live.Remote.Targets[t()].Builder = val }},
			configField{Key: "remote_arch", Kind: CfgFieldSelect, Label: i18n.T("tui.settings.remote.arch"),
				Description: i18n.T("tui.settings.remote.apply_desc"),
				Options:     []SelectOption{{Label: "amd64", Value: "amd64"}, {Label: "arm64", Value: "arm64"}},
				Validator:   &FieldValidator{AllowedValues: []string{"amd64", "arm64"}, AllowEmpty: true},
				Get: func() string {
					if a := v.live.Remote.Targets[t()].Arch; a != "" {
						return a
					}
					return "amd64"
				},
				Set: func(val string) { v.live.Remote.Targets[t()].Arch = val }},
			configField{Key: "remote_timeout", Kind: CfgFieldString, Label: i18n.T("tui.settings.remote.timeout"),
				Description: i18n.T("tui.settings.remote.apply_desc"), Placeholder: "3h",
				Get: func() string { return v.live.Remote.Targets[t()].Timeout },
				Set: func(val string) { v.live.Remote.Targets[t()].Timeout = val }},
			configField{Key: "remote_token_key", Kind: CfgFieldReadonly, Label: i18n.T("tui.settings.remote.token_key"),
				Description: i18n.T("tui.settings.remote.token_key_desc"),
				Get:         func() string { return v.live.Remote.Targets[t()].TokenKeyOrDefault() }},
		)
	}
	return append(fields, configField{Key: "remote_setup", Kind: CfgFieldPlaceholder,
		Label: i18n.T("tui.settings.remote.setup"), Description: i18n.T("tui.settings.remote.none_desc"),
		Get: func() string { return "" }})
}
