package views

import (
	"errors"
	"strconv"
	"strings"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// « Exécution » section of the Settings (P4-T10, 10 §11): default runtime,
// container engine, image cache, pinned tool version, strict isolation.

// checkVersion accepts "" or a version such as 2.0.20 (optional "v").
func checkVersion(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(s, "v"), ".")
	ok := len(parts) >= 2 && len(parts) <= 3
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			ok = false
		}
	}
	if !ok {
		return errors.New(i18n.Tf("tui.settings.exec.opencode.invalid", s))
	}
	return nil
}

// execSettingsFields are the fields of the « Exécution » section.
func (v *SettingsView) execSettingsFields() []configField {
	ex := func() *config.ExecutionConfig { return &v.live.Execution }
	return []configField{
		{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.settings.exec.section")},
		{Key: "exec_runtime", Kind: CfgFieldSelect, Label: i18n.T("tui.settings.exec.runtime.label"),
			Description: i18n.T("tui.settings.exec.runtime.desc"),
			Options: []SelectOption{
				{Label: i18n.T("tui.settings.exec.runtime.workflow"), Value: ""},
				{Label: i18n.T("tui.launch.runtime_local"), Value: "local"},
				{Label: i18n.T("tui.launch.runtime_container"), Value: "container"},
			},
			Validator: &FieldValidator{AllowedValues: []string{"local", "container"}, AllowEmpty: true},
			Get:       func() string { return ex().Runtime },
			Set:       func(val string) { ex().Runtime = val }},
		{Key: "exec_engine", Kind: CfgFieldSelect, Label: i18n.T("tui.settings.exec.engine.label"),
			Description: i18n.T("tui.settings.exec.engine.desc"),
			Options: []SelectOption{
				{Label: i18n.T("tui.settings.exec.engine.auto"), Value: "auto"},
				{Label: "Colima", Value: "colima"},
				{Label: "Podman", Value: "podman"},
				{Label: "Docker", Value: "docker"},
			},
			Validator: &FieldValidator{AllowedValues: []string{"auto", "colima", "podman", "docker"}, AllowEmpty: true},
			Get: func() string {
				if ex().Engine == "" {
					return "auto"
				}
				return ex().Engine
			},
			Set: func(val string) {
				if val == "auto" {
					val = ""
				}
				ex().Engine = val
			}},
		{Key: "exec_keep_images", Kind: CfgFieldInt, Label: i18n.T("tui.settings.exec.keep_images.label"),
			Description: i18n.T("tui.settings.exec.keep_images.desc"),
			Placeholder: strconv.Itoa(config.DefaultKeepImages),
			Validator:   &FieldValidator{Numeric: true, MinInt: intPtr(1), MaxInt: intPtr(20), AllowEmpty: true},
			Get: func() string {
				if ex().KeepImages == 0 {
					return ""
				}
				return strconv.Itoa(ex().KeepImages)
			},
			Set: func(val string) { ex().KeepImages, _ = strconv.Atoi(val) }},
		{Key: "exec_opencode_version", Kind: CfgFieldString, Label: i18n.T("tui.settings.exec.opencode.label"),
			Description: i18n.T("tui.settings.exec.opencode.desc"),
			Validator:   &FieldValidator{Check: checkVersion},
			Get:         func() string { return ex().OpencodeVersion },
			Set:         func(val string) { ex().OpencodeVersion = strings.TrimSpace(val) },
			Source: func() string {
				client := ""
				if v.cfg.ToolVersion != nil {
					client = v.cfg.ToolVersion()
				}
				pinned := strings.TrimPrefix(ex().OpencodeVersion, "v")
				switch {
				case client == "":
					return ""
				case pinned == "":
					return i18n.Tf("tui.settings.exec.opencode.auto", client)
				case pinned != strings.TrimPrefix(client, "v"):
					return i18n.Tf("tui.settings.exec.opencode.mismatch", pinned, client)
				}
				return ""
			}},
		{Key: "exec_strict_isolation", Kind: CfgFieldBool, Label: i18n.T("tui.settings.exec.strict.label"),
			Description: i18n.T("tui.settings.exec.strict.desc"),
			Get:         func() string { return boolStr(ex().StrictIsolation) },
			Set:         func(val string) { ex().StrictIsolation = val == "true" }},
	}
}
