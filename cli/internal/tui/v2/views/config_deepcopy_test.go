package views

import (
	"testing"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// ---------- helpers ----------

func dcBoolPtr(b bool) *bool    { return &b }
func dcIntPtr(n int) *int       { return &n }
func dcStrPtr(s string) *string { return &s }

// ---------- deepCopyConfig ----------

func TestDeepCopyConfig_Scalars(t *testing.T) {
	orig := &config.Config{Name: "hub"}
	cp := deepCopyConfig(orig)
	cp.Name = "changed"
	if orig.Name != "hub" {
		t.Fatalf("scalar Name aliased: got %q", orig.Name)
	}
}

func TestDeepCopyConfig_Maps(t *testing.T) {
	orig := &config.Config{
		Models: config.ModelsConfig{
			Default:  "sonnet",
			Families: map[string]string{"fast": "haiku"},
			Agents:   map[string]string{"coder": "opus"},
		},
	}
	cp := deepCopyConfig(orig)
	cp.Models.Families["fast"] = "changed"
	cp.Models.Agents["new"] = "added"
	if orig.Models.Families["fast"] != "haiku" {
		t.Fatal("Models.Families map aliased")
	}
	if _, ok := orig.Models.Agents["new"]; ok {
		t.Fatal("Models.Agents map aliased")
	}
}

func TestDeepCopyConfig_TrackerPointers(t *testing.T) {
	orig := &config.Config{
		Tracker: config.TrackerLocalConfig{
			Enabled:              dcBoolPtr(true),
			AutoSync:             dcBoolPtr(false),
			PushLabels:           dcBoolPtr(true),
			AutoPlanAssigned:     dcBoolPtr(false),
			MaxAutoPlanPerMember: dcIntPtr(5),
		},
	}
	cp := deepCopyConfig(orig)
	*cp.Tracker.Enabled = false
	*cp.Tracker.MaxAutoPlanPerMember = 99
	if *orig.Tracker.Enabled != true {
		t.Fatal("Tracker.Enabled pointer aliased")
	}
	if *orig.Tracker.MaxAutoPlanPerMember != 5 {
		t.Fatal("Tracker.MaxAutoPlanPerMember pointer aliased")
	}
}

func TestDeepCopyConfig_Workflow(t *testing.T) {
	orig := &config.Config{
		Workflow: &config.WorkflowHubConfig{
			Overrides: &workflow.WorkflowOverride{
				CheckpointOverrides: []workflow.CheckpointOverride{
					{ID: "cp1", Label: dcStrPtr("original")},
				},
				AgentOverrides: []workflow.AgentOverride{
					{AgentID: "a1", Disabled: dcBoolPtr(false)},
				},
				CircuitBreakerOverride: dcIntPtr(10),
			},
		},
	}
	cp := deepCopyConfig(orig)

	// Mutate copy
	cp.Workflow.Overrides.CheckpointOverrides[0].ID = "changed"
	*cp.Workflow.Overrides.CheckpointOverrides[0].Label = "changed"
	*cp.Workflow.Overrides.AgentOverrides[0].Disabled = true
	*cp.Workflow.Overrides.CircuitBreakerOverride = 99

	// Verify original unchanged
	if orig.Workflow.Overrides.CheckpointOverrides[0].ID != "cp1" {
		t.Fatal("Workflow CheckpointOverrides aliased (ID)")
	}
	if *orig.Workflow.Overrides.CheckpointOverrides[0].Label != "original" {
		t.Fatal("Workflow CheckpointOverrides aliased (Label)")
	}
	if *orig.Workflow.Overrides.AgentOverrides[0].Disabled != false {
		t.Fatal("Workflow AgentOverrides aliased")
	}
	if *orig.Workflow.Overrides.CircuitBreakerOverride != 10 {
		t.Fatal("Workflow CircuitBreakerOverride aliased")
	}
}

func TestDeepCopyConfig_WorkflowNil(t *testing.T) {
	orig := &config.Config{Workflow: nil}
	cp := deepCopyConfig(orig)
	if cp.Workflow != nil {
		t.Fatal("nil Workflow should stay nil after copy")
	}
}

func TestDeepCopyConfig_Teams(t *testing.T) {
	orig := &config.Config{
		Teams: []config.TeamConfig{{ID: "t1", Name: "team1"}},
	}
	cp := deepCopyConfig(orig)
	cp.Teams[0].Name = "changed"
	if orig.Teams[0].Name != "team1" {
		t.Fatal("Teams slice aliased")
	}
}

// ---------- deepCopyProject ----------

func TestDeepCopyProject_WorkflowConfig(t *testing.T) {
	orig := &domain.Project{
		ID:   "p1",
		Name: "myproject",
		WorkflowConfig: &domain.ProjectWorkflowConfig{
			Overrides: &workflow.WorkflowOverride{
				AgentOverrides: []workflow.AgentOverride{
					{AgentID: "coder", Disabled: dcBoolPtr(false)},
				},
			},
		},
	}
	cp := deepCopyProject(orig)

	// Mutate copy
	*cp.WorkflowConfig.Overrides.AgentOverrides[0].Disabled = true

	// Verify original unchanged
	if *orig.WorkflowConfig.Overrides.AgentOverrides[0].Disabled != false {
		t.Fatal("Project WorkflowConfig aliased")
	}
}

func TestDeepCopyProject_WorkflowConfigNil(t *testing.T) {
	orig := &domain.Project{ID: "p1", WorkflowConfig: nil}
	cp := deepCopyProject(orig)
	if cp.WorkflowConfig != nil {
		t.Fatal("nil WorkflowConfig should stay nil after copy")
	}
}

func TestDeepCopyProject_MCPServices(t *testing.T) {
	orig := &domain.Project{
		MCPConfig: &domain.ProjectMCPConfig{
			Services: []domain.ProjectMCPService{
				{Name: "gitlab", Enabled: dcBoolPtr(true), WriteEnabled: dcBoolPtr(false)},
			},
		},
	}
	cp := deepCopyProject(orig)

	// Mutate copy
	*cp.MCPConfig.Services[0].Enabled = false
	*cp.MCPConfig.Services[0].WriteEnabled = true

	if *orig.MCPConfig.Services[0].Enabled != true {
		t.Fatal("MCPConfig.Services[0].Enabled aliased")
	}
	if *orig.MCPConfig.Services[0].WriteEnabled != false {
		t.Fatal("MCPConfig.Services[0].WriteEnabled aliased")
	}
}

func TestDeepCopyProject_ModelOverrides(t *testing.T) {
	orig := &domain.Project{
		ModelOverrides: &domain.ProjectModelOverrides{
			Families: map[string]string{"fast": "haiku"},
			Agents:   map[string]string{"coder": "opus"},
		},
	}
	cp := deepCopyProject(orig)
	cp.ModelOverrides.Families["fast"] = "changed"
	if orig.ModelOverrides.Families["fast"] != "haiku" {
		t.Fatal("ModelOverrides.Families aliased")
	}
}

func TestDeepCopyProject_DeprecatedMCP(t *testing.T) {
	orig := &domain.Project{
		MCP: []string{"gitlab", "jira"},
	}
	cp := deepCopyProject(orig)
	cp.MCP[0] = "changed"
	if orig.MCP[0] != "gitlab" {
		t.Fatal("deprecated MCP slice aliased")
	}
}

func TestDeepCopyProject_TeamID(t *testing.T) {
	orig := &domain.Project{TeamID: dcStrPtr("team1")}
	cp := deepCopyProject(orig)
	*cp.TeamID = "changed"
	if *orig.TeamID != "team1" {
		t.Fatal("TeamID pointer aliased")
	}
}

// ---------- deepCopyTeamConfig ----------

func TestDeepCopyTeamConfig_MCPMap(t *testing.T) {
	orig := &teamstate.TeamConfig{
		MCP: map[string]teamstate.SharedMCPConfig{
			"gitlab": {
				Enabled:         dcBoolPtr(true),
				EnabledEnforced: dcBoolPtr(true),
				URL:             "https://gitlab.example.com",
				URLEnforced:     dcBoolPtr(false),
			},
		},
	}
	cp := deepCopyTeamConfig(orig)

	// Mutate copy
	*cp.MCP["gitlab"].Enabled = false
	gitlab := cp.MCP["gitlab"]
	gitlab.URL = "changed"
	cp.MCP["gitlab"] = gitlab

	// Verify original unchanged
	if *orig.MCP["gitlab"].Enabled != true {
		t.Fatal("MCP.Enabled pointer aliased")
	}
	if orig.MCP["gitlab"].URL != "https://gitlab.example.com" {
		t.Fatal("MCP map entry aliased")
	}
}

func TestDeepCopyTeamConfig_TrackerMaps(t *testing.T) {
	orig := &teamstate.TeamConfig{
		Tracker: teamstate.TrackerConfig{
			LabelStatusMapping: map[string]string{"blocked": "blocked"},
			StatusMapping:      map[string]string{"In Progress": "in_progress"},
			Projects:           map[string]string{"p1": "42"},
			TicketPatterns:     map[string]string{"p1": `PRJ-\d+`},
			UnassignedLabels:   []string{"team::backend"},
			TypeEnforced:       dcBoolPtr(true),
			PushLabelsEnforced: dcBoolPtr(false),
		},
	}
	cp := deepCopyTeamConfig(orig)

	// Mutate copy maps
	cp.Tracker.LabelStatusMapping["blocked"] = "changed"
	cp.Tracker.StatusMapping["new"] = "added"
	cp.Tracker.UnassignedLabels[0] = "changed"
	*cp.Tracker.TypeEnforced = false
	*cp.Tracker.PushLabelsEnforced = true

	// Verify original unchanged
	if orig.Tracker.LabelStatusMapping["blocked"] != "blocked" {
		t.Fatal("LabelStatusMapping aliased")
	}
	if _, ok := orig.Tracker.StatusMapping["new"]; ok {
		t.Fatal("StatusMapping aliased")
	}
	if orig.Tracker.UnassignedLabels[0] != "team::backend" {
		t.Fatal("UnassignedLabels aliased")
	}
	if *orig.Tracker.TypeEnforced != true {
		t.Fatal("TypeEnforced pointer aliased")
	}
	if *orig.Tracker.PushLabelsEnforced != false {
		t.Fatal("PushLabelsEnforced pointer aliased")
	}
}

func TestDeepCopyTeamConfig_Models(t *testing.T) {
	orig := &teamstate.TeamConfig{
		Models: teamstate.TeamModelsConfig{
			Default:  "sonnet",
			Families: map[string]string{"fast": "haiku"},
			Agents:   map[string]string{"coder": "opus"},
		},
	}
	cp := deepCopyTeamConfig(orig)
	cp.Models.Families["fast"] = "changed"
	cp.Models.Agents["new"] = "added"
	if orig.Models.Families["fast"] != "haiku" {
		t.Fatal("Models.Families aliased")
	}
	if _, ok := orig.Models.Agents["new"]; ok {
		t.Fatal("Models.Agents aliased")
	}
}

func TestDeepCopyTeamConfig_Workflow(t *testing.T) {
	orig := &teamstate.TeamConfig{
		Workflow: &teamstate.WorkflowTeamConfig{
			Overrides: &workflow.WorkflowOverride{
				AgentOverrides: []workflow.AgentOverride{
					{AgentID: "coder", Disabled: dcBoolPtr(false)},
				},
			},
			Enforced: dcBoolPtr(true),
		},
	}
	cp := deepCopyTeamConfig(orig)

	// Mutate copy
	*cp.Workflow.Overrides.AgentOverrides[0].Disabled = true
	*cp.Workflow.Enforced = false

	if *orig.Workflow.Overrides.AgentOverrides[0].Disabled != false {
		t.Fatal("Workflow.Overrides aliased")
	}
	if *orig.Workflow.Enforced != true {
		t.Fatal("Workflow.Enforced aliased")
	}
}

func TestDeepCopyTeamConfig_WorkflowNil(t *testing.T) {
	orig := &teamstate.TeamConfig{Workflow: nil}
	cp := deepCopyTeamConfig(orig)
	if cp.Workflow != nil {
		t.Fatal("nil Workflow should stay nil after copy")
	}
}

func TestDeepCopyTeamConfig_NotificationDestinations(t *testing.T) {
	orig := &teamstate.TeamConfig{
		Notification: teamstate.NotificationConfig{
			Destinations: []teamstate.NotificationDestination{
				{Type: "slack", WebhookURL: "https://hooks.slack.com/xxx"},
			},
		},
	}
	cp := deepCopyTeamConfig(orig)
	cp.Notification.Destinations[0].WebhookURL = "changed"
	if orig.Notification.Destinations[0].WebhookURL != "https://hooks.slack.com/xxx" {
		t.Fatal("Notification.Destinations aliased")
	}
}

// ---------- cloneStringMap ----------

func TestCloneStringMap_Nil(t *testing.T) {
	if cloneStringMap(nil) != nil {
		t.Fatal("cloneStringMap(nil) should return nil")
	}
}

func TestCloneStringMap_Independent(t *testing.T) {
	orig := map[string]string{"a": "1", "b": "2"}
	cp := cloneStringMap(orig)
	cp["a"] = "changed"
	cp["c"] = "new"
	if orig["a"] != "1" {
		t.Fatal("cloneStringMap aliased")
	}
	if _, ok := orig["c"]; ok {
		t.Fatal("cloneStringMap aliased (new key)")
	}
}
