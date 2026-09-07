package views

import (
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
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

	return cp
}
