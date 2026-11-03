package cmd

import (
	"testing"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/limits"
)

// oh config set|unset know the v5 sections, with validated values.
func TestConfigSetKeys(t *testing.T) {
	i18ntest.Set(t, "en")
	c := &config.Config{Remote: config.RemoteConfig{Targets: []config.RemoteTarget{{Name: "acme", URL: "https://gitlab.com", Group: "acme"}}}}
	set := func(key, value string) error {
		t.Helper()
		f, ok := lookupConfigField(key)
		if !ok {
			t.Fatalf("%s: unknown key", key)
		}
		return f.Set(c, value)
	}
	for _, kv := range [][2]string{
		{"session.idle_sleep_minutes", "1"}, {"session.attach", "tmux"}, {"session.iterm_style", "split"}, {"session.notify", "off"},
		{"execution.runtime", "container"}, {"execution.engine", "podman"}, {"execution.keep_images", "3"},
		{"execution.colima_profile", "work"}, {"execution.tool_version", "v2.0.20"}, {"execution.strict_isolation", "true"},
		{"limits.daily_budget_usd", "12.5"}, {"limits.max_active_sessions", "4"}, {"limits.models", "eu.anthropic.*, us.*"},
		{"remote.projects.p1", "acme"}, {"remote.targets.acme.builder", "dind"}, {"remote.targets.acme.timeout", "1h 30m"},
	} {
		if err := set(kv[0], kv[1]); err != nil {
			t.Fatalf("set %s %s: %v", kv[0], kv[1], err)
		}
	}
	if c.Session.IdleSleepMinutes != 1 || c.Session.Attach != "tmux" || c.Session.Notify != "off" ||
		c.Execution.Runtime != "container" || c.Execution.Engine != "podman" || c.Execution.KeepImages != 3 ||
		c.Execution.ColimaProfile != "work" || c.Execution.ToolVersion != "2.0.20" || !c.Execution.StrictIsolation ||
		c.Limits.DailyBudgetUSD != 12.5 || c.Limits.MaxActiveSessions != 4 || len(c.Limits.Models) != 2 ||
		c.Remote.Projects["p1"] != "acme" || c.Remote.Targets[0].Builder != "dind" || c.Remote.Targets[0].Timeout != "1h 30m" {
		t.Fatalf("config = %+v", c)
	}

	// Default values are stored empty, like the TUI Settings.
	if err := set("execution.engine", "auto"); err != nil || c.Execution.Engine != "" {
		t.Fatalf("engine auto: %v %q", err, c.Execution.Engine)
	}
	if err := set("execution.colima_profile", "default"); err != nil || c.Execution.ColimaProfile != "" {
		t.Fatalf("colima profile default: %v %q", err, c.Execution.ColimaProfile)
	}

	for _, kv := range [][2]string{
		{"session.idle_sleep_minutes", "0"}, {"session.attach", "kitty"}, {"execution.runtime", "remote"},
		{"execution.colima_profile", "bad profile"},
		{"execution.tool_version", "latest"}, {"execution.strict_isolation", "maybe"}, {"limits.daily_budget_usd", "-1"},
		{"remote.projects.p1", "nope"}, {"remote.targets.nope.tag", "oh"}, {"remote.targets.acme.arch", "386"},
		{"remote.targets.acme.timeout", "soon"},
	} {
		if err := set(kv[0], kv[1]); err == nil {
			t.Fatalf("set %s %s accepted", kv[0], kv[1])
		}
	}
	if _, ok := lookupConfigField("remote.targets.acme.url"); ok {
		t.Fatal("remote.targets.<t>.url is set by oh remote setup only")
	}

	f, _ := lookupConfigField("limits.daily_budget_usd")
	f.Unset(c)
	if c.Limits.DailyBudgetUSD != 0 || c.Limits.MaxActiveSessions != 4 {
		t.Fatalf("unset limits.daily_budget_usd: %+v", c.Limits)
	}
	f, _ = lookupConfigField("remote.projects.p1")
	f.Unset(c)
	if _, ok := c.Remote.Projects["p1"]; ok {
		t.Fatal("remote.projects.p1 not removed")
	}
	for _, field := range limits.Fields {
		if _, ok := lookupConfigField("limits." + field); !ok {
			t.Fatalf("limits.%s missing", field)
		}
	}
}
