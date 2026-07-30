package tracker

import (
	"testing"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

func boolPtr(b bool) *bool { return &b }

func TestResolveFullMCPConfig_TeamEnforcedEnabled(t *testing.T) {
	shared := &teamstate.SharedMCPConfig{
		Enabled:         boolPtr(true),
		EnabledEnforced: boolPtr(true),
	}
	hub := config.MCPServerConfig{Enabled: false, Token: "tok"}
	project := &domain.ProjectMCPService{Enabled: boolPtr(false)} // tries to override

	eff := ResolveFullMCPConfig(shared, hub, project, "acme")
	if !eff.Enabled {
		t.Error("team enforced should win over project and hub")
	}
	if !eff.EnabledEnforced {
		t.Error("should be marked as enforced")
	}
}

func TestResolveFullMCPConfig_ProjectOverridesHub(t *testing.T) {
	hub := config.MCPServerConfig{Enabled: true, Token: "hub-tok", WriteEnabled: true}
	project := &domain.ProjectMCPService{
		Enabled:      boolPtr(false),
		TokenKey:     "proj-tok",
		WriteEnabled: boolPtr(false),
		URL:          "https://proj.gitlab.com",
	}

	eff := ResolveFullMCPConfig(nil, hub, project, "")
	if eff.Enabled {
		t.Error("project override should disable")
	}
	if eff.TokenKey != "proj-tok" {
		t.Errorf("expected proj-tok, got %s", eff.TokenKey)
	}
	if eff.WriteEnabled {
		t.Error("project should override write to false")
	}
	if eff.URL != "https://proj.gitlab.com" {
		t.Errorf("expected project URL, got %s", eff.URL)
	}
}

func TestResolveFullMCPConfig_HubFallsToTeamRecommended(t *testing.T) {
	shared := &teamstate.SharedMCPConfig{
		Enabled: boolPtr(true),
		URL:     "https://team.gitlab.com",
	}
	hub := config.MCPServerConfig{} // empty hub — no explicit config

	eff := ResolveFullMCPConfig(shared, hub, nil, "acme")
	if !eff.Enabled {
		t.Error("team recommendation should be used when hub is empty")
	}
	if eff.URL != "https://team.gitlab.com" {
		t.Errorf("expected team URL, got %s", eff.URL)
	}
	if eff.EnabledEnforced {
		t.Error("should not be enforced")
	}
}

func TestResolveFullMCPConfig_URLEnforced(t *testing.T) {
	shared := &teamstate.SharedMCPConfig{
		URL:         "https://enforced.gitlab.com",
		URLEnforced: boolPtr(true),
	}
	hub := config.MCPServerConfig{URL: "https://my-override.com"}
	project := &domain.ProjectMCPService{URL: "https://project.com"}

	eff := ResolveFullMCPConfig(shared, hub, project, "acme")
	if eff.URL != "https://enforced.gitlab.com" {
		t.Errorf("URL enforced should win, got %s", eff.URL)
	}
	if !eff.URLEnforced {
		t.Error("should be marked as URL enforced")
	}
}

func TestResolveFullMCPConfig_TokenAlwaysPersonal(t *testing.T) {
	shared := &teamstate.SharedMCPConfig{Enabled: boolPtr(true)}
	hub := config.MCPServerConfig{Token: "hub-tok"}

	// No project override → hub token
	eff := ResolveFullMCPConfig(shared, hub, nil, "acme")
	if eff.TokenKey != "hub-tok" {
		t.Errorf("expected hub-tok, got %s", eff.TokenKey)
	}

	// Project override → project token
	project := &domain.ProjectMCPService{TokenKey: "proj-tok"}
	eff = ResolveFullMCPConfig(shared, hub, project, "acme")
	if eff.TokenKey != "proj-tok" {
		t.Errorf("expected proj-tok, got %s", eff.TokenKey)
	}
}

func TestResolveFullMCPConfig_AllNil(t *testing.T) {
	hub := config.MCPServerConfig{}
	eff := ResolveFullMCPConfig(nil, hub, nil, "")
	if eff.Enabled {
		t.Error("default should be disabled")
	}
	if eff.URL != "" {
		t.Error("default URL should be empty")
	}
	if eff.TokenKey != "" {
		t.Error("default token should be empty")
	}
}

func TestResolveFullMCPConfig_CascadeOrder(t *testing.T) {
	// Full cascade: team recommends, hub overrides, project overrides hub
	shared := &teamstate.SharedMCPConfig{
		Enabled: boolPtr(true),
		URL:     "https://team.com",
	}
	hub := config.MCPServerConfig{
		Enabled: true,
		Token:   "hub-tok",
		URL:     "https://hub.com",
	}
	project := &domain.ProjectMCPService{
		URL: "https://project.com",
		// Enabled not set → inherits from hub
		// Token not set → inherits from hub
	}

	eff := ResolveFullMCPConfig(shared, hub, project, "acme")
	if !eff.Enabled {
		t.Error("should be enabled (hub explicit)")
	}
	if eff.URL != "https://project.com" {
		t.Errorf("project URL should win over hub, got %s", eff.URL)
	}
	if eff.TokenKey != "hub-tok" {
		t.Errorf("token should fall back to hub, got %s", eff.TokenKey)
	}
}
