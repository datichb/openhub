package views

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/envfilter"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// « Exécution » section of the Settings (P4-T10, 10 §11): default runtime,
// container engine, image cache, pinned tool version, strict isolation,
// variables passed to the agents (execution.env_allow) and the wait of the
// proxy for a provider (execution.proxy_first_byte_timeout_minutes).

// ParseEnvAllow reads the value of execution.env_allow: names of variables
// and PREFIX_* patterns, separated by commas or spaces, without duplicates.
// It is the rule of `oh config set execution.env_allow`: an entry must be a
// name or a pattern (envfilter.ValidPattern), and "*" alone is refused, since
// it would switch the filter off.
func ParseEnvAllow(value string) ([]string, error) {
	var out []string
	for _, e := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || unicode.IsSpace(r) }) {
		switch {
		case e == "*":
			return nil, errors.New(i18n.T("tui.settings.exec.env_allow.star"))
		case !envfilter.ValidPattern(e):
			return nil, errors.New(i18n.Tf("tui.settings.exec.env_allow.invalid", e))
		}
		if !slices.Contains(out, e) {
			out = append(out, e)
		}
	}
	return out, nil
}

// envAllowWarning says what is wrong, without being refused, with the entries
// of env_allow: names that look like secrets (the agents can read them and send
// them anywhere their tools reach) and credentials oh manages (removed anyway).
func envAllowWarning(entries []string) string {
	var secret, managed []string
	for _, e := range entries {
		switch {
		case envfilter.HardDenied(e):
			managed = append(managed, e)
		case envfilter.LooksSecret(e):
			secret = append(secret, e)
		}
	}
	var notes []string
	if len(secret) > 0 {
		notes = append(notes, "⚠ "+i18n.Tf("tui.settings.exec.env_allow.secret", strings.Join(secret, ", ")))
	}
	if len(managed) > 0 {
		notes = append(notes, i18n.Tf("tui.settings.exec.env_allow.managed", strings.Join(managed, ", ")))
	}
	return strings.Join(notes, " · ")
}

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
		return errors.New(i18n.Tf("tui.settings.exec.tool.invalid", s))
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
		{Key: "exec_tool_version", Kind: CfgFieldString, Label: i18n.T("tui.settings.exec.tool.label"),
			Description: i18n.T("tui.settings.exec.tool.desc"),
			Validator:   &FieldValidator{Check: checkVersion},
			Get:         func() string { return ex().ToolVersion },
			Set:         func(val string) { ex().ToolVersion = strings.TrimSpace(val) },
			Source: func() string {
				tool := v.toolInfo()
				client := tool.client
				pinned := strings.TrimPrefix(ex().ToolVersion, "v")
				switch {
				case client == "":
					return ""
				case pinned == "":
					return i18n.Tf("tui.settings.exec.tool.auto", client)
				case pinned != strings.TrimPrefix(client, "v"):
					return i18n.Tf("tui.settings.exec.tool.mismatch", tool.name, pinned, client)
				}
				return ""
			}},
		{Key: "exec_strict_isolation", Kind: CfgFieldBool, Label: i18n.T("tui.settings.exec.strict.label"),
			Description: i18n.T("tui.settings.exec.strict.desc"),
			Get:         func() string { return boolStr(ex().StrictIsolation) },
			Set:         func(val string) { ex().StrictIsolation = val == "true" }},
		{Key: "exec_env_allow", Kind: CfgFieldString, Label: i18n.T("tui.settings.exec.env_allow.label"),
			Description: i18n.T("tui.settings.exec.env_allow.desc"),
			Placeholder: i18n.T("tui.settings.exec.env_allow.placeholder"),
			Validator:   &FieldValidator{Check: func(val string) error { _, err := ParseEnvAllow(val); return err }},
			Get:         func() string { return strings.Join(ex().EnvAllow, ", ") },
			Set: func(val string) {
				if entries, err := ParseEnvAllow(val); err == nil { // an invalid list is refused by the validator
					ex().EnvAllow = entries
				}
			},
			Source: func() string { return envAllowWarning(ex().EnvAllow) },
			Warn: func(val string) string {
				entries, _ := ParseEnvAllow(val)
				return envAllowWarning(entries)
			}},
		{Key: "exec_proxy_first_byte", Kind: CfgFieldInt, Label: i18n.T("tui.settings.exec.proxy_timeout.label"),
			Description: i18n.T("tui.settings.exec.proxy_timeout.desc"),
			Placeholder: strconv.Itoa(config.DefaultProxyFirstByteTimeoutMinutes),
			Validator:   &FieldValidator{Numeric: true, MinInt: intPtr(1), MaxInt: intPtr(240), AllowEmpty: true},
			Get: func() string {
				if ex().ProxyFirstByteTimeoutMinutes == 0 {
					return ""
				}
				return strconv.Itoa(ex().ProxyFirstByteTimeoutMinutes)
			},
			Set: func(val string) { ex().ProxyFirstByteTimeoutMinutes, _ = strconv.Atoi(val) }},
	}
}
