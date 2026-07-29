package config

import (
	"testing"
)

func TestResolveBool_TeamEnforced(t *testing.T) {
	teamVal := true
	enforced := true
	hubVal := false
	projectVal := false

	result := ResolveBool(&teamVal, &enforced, &hubVal, &projectVal, "acme", false)
	if result.Value != true {
		t.Error("enforced team value should win")
	}
	if result.Source != SourceTeamEnforced {
		t.Errorf("expected source team:enforced, got %s", result.Source)
	}
	if !result.Locked {
		t.Error("enforced values should be locked")
	}
	if result.TeamID != "acme" {
		t.Errorf("expected teamID 'acme', got %q", result.TeamID)
	}
}

func TestResolveBool_ProjectOverride(t *testing.T) {
	teamVal := true
	hubVal := true
	projectVal := false

	result := ResolveBool(&teamVal, nil, &hubVal, &projectVal, "acme", true)
	if result.Value != false {
		t.Error("project override should win over hub and team recommended")
	}
	if result.Source != SourceProject {
		t.Errorf("expected source project, got %s", result.Source)
	}
}

func TestResolveBool_HubValue(t *testing.T) {
	teamVal := true
	hubVal := false

	result := ResolveBool(&teamVal, nil, &hubVal, nil, "acme", true)
	if result.Value != false {
		t.Error("hub value should win over team recommended")
	}
	if result.Source != SourceHub {
		t.Errorf("expected source hub, got %s", result.Source)
	}
}

func TestResolveBool_TeamRecommended(t *testing.T) {
	teamVal := true

	result := ResolveBool(&teamVal, nil, nil, nil, "acme", false)
	if result.Value != true {
		t.Error("team recommended should be used as fallback")
	}
	if result.Source != SourceTeamRecommended {
		t.Errorf("expected source team:recommended, got %s", result.Source)
	}
	if result.Locked {
		t.Error("recommended values should not be locked")
	}
}

func TestResolveBool_Default(t *testing.T) {
	result := ResolveBool(nil, nil, nil, nil, "", true)
	if result.Value != true {
		t.Error("default value should be used when no other source")
	}
	if result.Source != SourceDefault {
		t.Errorf("expected source default, got %s", result.Source)
	}
}

func TestResolveString_TeamEnforced(t *testing.T) {
	enforced := true
	result := ResolveString("https://gitlab.team.com", &enforced, "https://gitlab.mine.com", "", "acme", "")
	if result.Value != "https://gitlab.team.com" {
		t.Errorf("enforced should win, got %s", result.Value)
	}
	if !result.Locked {
		t.Error("expected locked")
	}
}

func TestResolveString_ProjectOverride(t *testing.T) {
	result := ResolveString("team-url", nil, "hub-url", "project-url", "acme", "default")
	if result.Value != "project-url" || result.Source != SourceProject {
		t.Errorf("expected project-url from project, got %s from %s", result.Value, result.Source)
	}
}

func TestResolveString_CascadeFallthrough(t *testing.T) {
	result := ResolveString("team-model", nil, "", "", "acme", "default-model")
	if result.Value != "team-model" || result.Source != SourceTeamRecommended {
		t.Errorf("expected team-model from team:recommended, got %s from %s", result.Value, result.Source)
	}
}
