package views

import (
	"encoding/json"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// deepCopyConfig creates a fully independent copy of a config.Config.
// All slices, maps, and pointer fields are cloned so that mutations
// on the copy do not affect the original (required for UndoStack snapshots).
func deepCopyConfig(c *config.Config) config.Config {
	cp := *c // shallow struct copy

	// Clone Teams slice
	if c.Teams != nil {
		cp.Teams = make([]config.TeamConfig, len(c.Teams))
		copy(cp.Teams, c.Teams)
	}

	// Clone Deploy.DisableNativeAgents slice
	if c.Deploy.DisableNativeAgents != nil {
		cp.Deploy.DisableNativeAgents = make([]string, len(c.Deploy.DisableNativeAgents))
		copy(cp.Deploy.DisableNativeAgents, c.Deploy.DisableNativeAgents)
	}

	// Clone Models maps
	if c.Models.Families != nil {
		cp.Models.Families = make(map[string]string, len(c.Models.Families))
		for k, v := range c.Models.Families {
			cp.Models.Families[k] = v
		}
	}
	if c.Models.Agents != nil {
		cp.Models.Agents = make(map[string]string, len(c.Models.Agents))
		for k, v := range c.Models.Agents {
			cp.Models.Agents[k] = v
		}
	}

	// Clone Tracker pointer fields
	if c.Tracker.Enabled != nil {
		b := *c.Tracker.Enabled
		cp.Tracker.Enabled = &b
	}
	if c.Tracker.AutoSync != nil {
		b := *c.Tracker.AutoSync
		cp.Tracker.AutoSync = &b
	}
	if c.Tracker.PushLabels != nil {
		b := *c.Tracker.PushLabels
		cp.Tracker.PushLabels = &b
	}
	if c.Tracker.AutoPlanAssigned != nil {
		b := *c.Tracker.AutoPlanAssigned
		cp.Tracker.AutoPlanAssigned = &b
	}
	if c.Tracker.MaxAutoPlanPerMember != nil {
		n := *c.Tracker.MaxAutoPlanPerMember
		cp.Tracker.MaxAutoPlanPerMember = &n
	}

	// Clone Workflow (contains nested slices, maps, and pointers)
	cp.Workflow = deepCopyWorkflowHubConfig(c.Workflow)

	return cp
}

// deepCopyProject creates a fully independent copy of a domain.Project.
// All pointer fields, slices, and nested structs are cloned.
func deepCopyProject(p *domain.Project) domain.Project {
	cp := *p // shallow struct copy

	// Clone Agents slice
	if p.Agents != nil {
		cp.Agents = make([]string, len(p.Agents))
		copy(cp.Agents, p.Agents)
	}

	// Clone TeamConfig
	if p.TeamConfig != nil {
		tc := *p.TeamConfig
		cp.TeamConfig = &tc
	}

	// Clone TrackerConfig
	if p.TrackerConfig != nil {
		tc := *p.TrackerConfig
		cp.TrackerConfig = &tc
	}

	// Clone MCPConfig + Services slice
	if p.MCPConfig != nil {
		mc := *p.MCPConfig
		if p.MCPConfig.Services != nil {
			mc.Services = make([]domain.ProjectMCPService, len(p.MCPConfig.Services))
			for i, svc := range p.MCPConfig.Services {
				mc.Services[i] = svc
				// Clone Enabled and WriteEnabled pointers
				if svc.Enabled != nil {
					b := *svc.Enabled
					mc.Services[i].Enabled = &b
				}
				if svc.WriteEnabled != nil {
					b := *svc.WriteEnabled
					mc.Services[i].WriteEnabled = &b
				}
			}
		}
		cp.MCPConfig = &mc
	}

	// Clone ProviderConfig
	if p.ProviderConfig != nil {
		pc := *p.ProviderConfig
		cp.ProviderConfig = &pc
	}

	// Clone ModelOverrides
	if p.ModelOverrides != nil {
		mo := *p.ModelOverrides
		if p.ModelOverrides.Families != nil {
			mo.Families = make(map[string]string, len(p.ModelOverrides.Families))
			for k, v := range p.ModelOverrides.Families {
				mo.Families[k] = v
			}
		}
		if p.ModelOverrides.Agents != nil {
			mo.Agents = make(map[string]string, len(p.ModelOverrides.Agents))
			for k, v := range p.ModelOverrides.Agents {
				mo.Agents[k] = v
			}
		}
		cp.ModelOverrides = &mo
	}

	// Clone Labels slice
	if p.Labels != nil {
		cp.Labels = make([]string, len(p.Labels))
		copy(cp.Labels, p.Labels)
	}

	// Clone TeamID pointer
	if p.TeamID != nil {
		s := *p.TeamID
		cp.TeamID = &s
	}

	// Clone deprecated MCP slice
	if p.MCP != nil {
		cp.MCP = make([]string, len(p.MCP))
		copy(cp.MCP, p.MCP)
	}

	// Clone WorkflowConfig (contains nested slices, maps, and pointers)
	cp.WorkflowConfig = deepCopyProjectWorkflowConfig(p.WorkflowConfig)

	return cp
}

// deepCopyTeamConfig creates a fully independent copy of a teamstate.TeamConfig.
// All maps, slices, and pointer fields are cloned so that mutations on the copy
// do not affect the original (required for UndoStack snapshots in team views).
func deepCopyTeamConfig(c *teamstate.TeamConfig) teamstate.TeamConfig {
	cp := *c // shallow struct copy

	// Clone MCP map (each value has *bool pointers)
	if c.MCP != nil {
		cp.MCP = make(map[string]teamstate.SharedMCPConfig, len(c.MCP))
		for k, v := range c.MCP {
			svc := v // copy value struct
			if v.Enabled != nil {
				b := *v.Enabled
				svc.Enabled = &b
			}
			if v.EnabledEnforced != nil {
				b := *v.EnabledEnforced
				svc.EnabledEnforced = &b
			}
			if v.URLEnforced != nil {
				b := *v.URLEnforced
				svc.URLEnforced = &b
			}
			cp.MCP[k] = svc
		}
	}

	// Clone Models maps
	if c.Models.Families != nil {
		cp.Models.Families = make(map[string]string, len(c.Models.Families))
		for k, v := range c.Models.Families {
			cp.Models.Families[k] = v
		}
	}
	if c.Models.Agents != nil {
		cp.Models.Agents = make(map[string]string, len(c.Models.Agents))
		for k, v := range c.Models.Agents {
			cp.Models.Agents[k] = v
		}
	}

	// Clone Tracker maps and pointer fields
	if c.Tracker.TypeEnforced != nil {
		b := *c.Tracker.TypeEnforced
		cp.Tracker.TypeEnforced = &b
	}
	if c.Tracker.PushLabelsEnforced != nil {
		b := *c.Tracker.PushLabelsEnforced
		cp.Tracker.PushLabelsEnforced = &b
	}
	cp.Tracker.TicketPatterns = cloneStringMap(c.Tracker.TicketPatterns)
	cp.Tracker.Projects = cloneStringMap(c.Tracker.Projects)
	cp.Tracker.StatusMapping = cloneStringMap(c.Tracker.StatusMapping)
	cp.Tracker.LabelStatusMapping = cloneStringMap(c.Tracker.LabelStatusMapping)
	if c.Tracker.UnassignedLabels != nil {
		cp.Tracker.UnassignedLabels = make([]string, len(c.Tracker.UnassignedLabels))
		copy(cp.Tracker.UnassignedLabels, c.Tracker.UnassignedLabels)
	}

	// Clone Notification.Destinations slice
	if c.Notification.Destinations != nil {
		cp.Notification.Destinations = make([]teamstate.NotificationDestination, len(c.Notification.Destinations))
		copy(cp.Notification.Destinations, c.Notification.Destinations)
	}

	// Clone Workflow (contains WorkflowOverride with slices/maps/pointers + Enforced *bool)
	cp.Workflow = deepCopyWorkflowTeamConfig(c.Workflow)

	return cp
}

// ---------- Workflow deep copy helpers ----------

// deepCopyWorkflowOverride uses JSON roundtrip to deep copy a WorkflowOverride.
// This matches the approach used by WorkflowDefinition.DeepCopy() in workflow/types.go.
func deepCopyWorkflowOverride(o *workflow.WorkflowOverride) *workflow.WorkflowOverride {
	if o == nil {
		return nil
	}
	data, err := json.Marshal(o)
	if err != nil {
		// Should never happen for a well-formed struct; fallback to shallow copy.
		cp := *o
		return &cp
	}
	var cp workflow.WorkflowOverride
	if err := json.Unmarshal(data, &cp); err != nil {
		shallow := *o
		return &shallow
	}
	return &cp
}

func deepCopyWorkflowHubConfig(w *config.WorkflowHubConfig) *config.WorkflowHubConfig {
	if w == nil {
		return nil
	}
	return &config.WorkflowHubConfig{
		Overrides: deepCopyWorkflowOverride(w.Overrides),
	}
}

func deepCopyProjectWorkflowConfig(w *domain.ProjectWorkflowConfig) *domain.ProjectWorkflowConfig {
	if w == nil {
		return nil
	}
	return &domain.ProjectWorkflowConfig{
		Overrides: deepCopyWorkflowOverride(w.Overrides),
	}
}

func deepCopyWorkflowTeamConfig(w *teamstate.WorkflowTeamConfig) *teamstate.WorkflowTeamConfig {
	if w == nil {
		return nil
	}
	cp := &teamstate.WorkflowTeamConfig{
		Overrides: deepCopyWorkflowOverride(w.Overrides),
	}
	if w.Enforced != nil {
		b := *w.Enforced
		cp.Enforced = &b
	}
	return cp
}

// ---------- Generic helpers ----------

func cloneStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
