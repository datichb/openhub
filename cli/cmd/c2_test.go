package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/limits"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
)

// A15: oh config set|unset know the v5 sections, with validated values.
func TestConfigSetV5Keys(t *testing.T) {
	useLocale(t, "en")
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
		{"execution.tool_version", "v2.0.20"}, {"execution.strict_isolation", "true"},
		{"limits.daily_budget_usd", "12.5"}, {"limits.max_active_sessions", "4"}, {"limits.models", "eu.anthropic.*, us.*"},
		{"remote.projects.p1", "acme"}, {"remote.targets.acme.builder", "dind"}, {"remote.targets.acme.timeout", "1h 30m"},
	} {
		if err := set(kv[0], kv[1]); err != nil {
			t.Fatalf("set %s %s: %v", kv[0], kv[1], err)
		}
	}
	if c.Session.IdleSleepMinutes != 1 || c.Session.Attach != "tmux" || c.Session.Notify != "off" ||
		c.Execution.Runtime != "container" || c.Execution.Engine != "podman" || c.Execution.KeepImages != 3 ||
		c.Execution.ToolVersion != "2.0.20" || !c.Execution.StrictIsolation ||
		c.Limits.DailyBudgetUSD != 12.5 || c.Limits.MaxActiveSessions != 4 || len(c.Limits.Models) != 2 ||
		c.Remote.Projects["p1"] != "acme" || c.Remote.Targets[0].Builder != "dind" || c.Remote.Targets[0].Timeout != "1h 30m" {
		t.Fatalf("config = %+v", c)
	}

	// Default values are stored empty, like the TUI Settings.
	if err := set("execution.engine", "auto"); err != nil || c.Execution.Engine != "" {
		t.Fatalf("engine auto: %v %q", err, c.Execution.Engine)
	}

	for _, kv := range [][2]string{
		{"session.idle_sleep_minutes", "0"}, {"session.attach", "kitty"}, {"execution.runtime", "remote"},
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

// A11: the model cascade shows the project, hub and workflow levels in
// priority order, translated.
func TestModelCascadeLevels(t *testing.T) {
	testHubEnv(t)
	t.Setenv(workflowsDirEnv, "")
	useLocale(t, "fr")
	a := newMockApp(nil, nil)
	a.Config.Models.Default = "hub-model"
	project := &domain.Project{ID: "p1", Name: "demo", Model: "project-model",
		ModelOverrides: &domain.ProjectModelOverrides{Agents: map[string]string{"developer": "dev-model"}}}
	levels, err := modelCascadeLevels(context.Background(), a, project, "ticket")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, l := range levels {
		ids = append(ids, l.ID)
	}
	if strings.Join(ids, ",") != "workflow,project,hub" {
		t.Fatalf("levels = %v", ids)
	}
	if levels[1].Models.Default != "project-model" || levels[1].Models.Agents["developer"] != "dev-model" || levels[2].Models.Default != "hub-model" {
		t.Fatalf("levels = %+v", levels)
	}
	var out strings.Builder
	printModelCascade(&out, levels)
	for _, want := range []string{"Cascade des modèles", "Projet demo", "Hub (hub.toml)", "Frontmatter des agents"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output without %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "Model Configuration") || strings.Contains(out.String(), "Hub-level") {
		t.Fatalf("untranslated output:\n%s", out.String())
	}
}

// A12: cobra errors, unknown sessions and the daemon refusal are translated.
func TestLocalizeError(t *testing.T) {
	useLocale(t, "fr")
	cases := map[string]error{
		"option courte inconnue : 'j' dans -j":   errors.New("unknown shorthand flag: 'j' in -j"),
		"option inconnue : --nope":               errors.New("unknown flag: --nope"),
		`commande inconnue "session approve"`:    errors.New("unknown command \"session approve\" for \"oh\"\n\nDid you mean this?\n\tsession\n"),
		"Vouliez-vous dire : session ?":          errors.New("unknown command \"sesion\" for \"oh\"\n\nDid you mean this?\n\tsession\n"),
		"1 argument(s) attendu(s), 0 reçu(s)":    errors.New("accepts 1 arg(s), received 0"),
		"Des sessions tournent":                  &daemon.APIError{Method: "POST", Path: "/shutdown", Status: 409, Message: "sessions are running (use force)"},
		"lancez cette commande dans un terminal": errors.New("open /dev/tty: device not configured"),
	}
	for want, err := range cases {
		if got := localizeError(err); !strings.Contains(got, want) {
			t.Errorf("localizeError(%q) = %q, want %q", err, got, want)
		}
	}
	err := sessionRefError("zzz", errors.Join(errors.New("session zzz"), domain.ErrNotFound))
	if err == nil || !strings.Contains(err.Error(), "Session introuvable : zzz") {
		t.Fatalf("sessionRefError = %v", err)
	}
	if err := sessionRefError("ses", sessionsvc.ErrAmbiguous); err == nil || !strings.Contains(err.Error(), "ambiguë") {
		t.Fatalf("sessionRefError = %v", err)
	}
	if got := localizeError(errors.New("plain")); got != "plain" {
		t.Fatalf("other errors unchanged: %q", got)
	}
}

// A13: an interactive assistant without terminal stops with a clear message
// and a non-zero code.
func TestMCPSetupWithoutTerminal(t *testing.T) {
	useLocale(t, "en")
	prev := stdinIsTerminal
	t.Cleanup(func() { stdinIsTerminal = prev })
	stdinIsTerminal = func() bool { return false }
	a := newMockApp(nil, nil)
	prevApp := application
	t.Cleanup(func() { application = prevApp })
	application = a
	cmd := mcpSetupCmd()
	cmd.SetContext(context.Background())
	err := runMCPSetup(cmd, []string{"gitlab"})
	if err == nil || !strings.Contains(err.Error(), "run this command in a terminal") || ExitCode(err) == 0 {
		t.Fatalf("err = %v", err)
	}
}
