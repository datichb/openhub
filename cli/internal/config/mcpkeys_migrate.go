package config

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/datichb/openhub/cli/internal/domain"
)

// MigrateProjectTokenKeys moves the project-scoped MCP token keys to the
// current naming: for each project service whose TokenKey is the legacy name
// of its own project key, the value is copied under the current name, then
// TokenKey is updated in the database. The legacy key is kept.
//
// It can be replayed: a service already on the current name is skipped, a key
// absent from the keychain is tolerated (only TokenKey is updated), and
// nothing changes when the keychain cannot be read (the legacy name stays and
// readers fall back to it). Returns the number of services migrated.
func MigrateProjectTokenKeys(ctx context.Context, store SecretReadWriter, projects domain.ProjectStore) (int, error) {
	if store == nil || projects == nil {
		return 0, nil
	}
	list, err := projects.List(ctx, "")
	if err != nil {
		return 0, fmt.Errorf("listing projects for token key migration: %w", err)
	}
	total := 0
	for _, listed := range list {
		if !hasLegacyProjectTokenKey(&listed) {
			continue
		}
		// Re-read the project: the update below rewrites the whole row.
		p, err := projects.Get(ctx, listed.ID)
		if err != nil || p.MCPConfig == nil {
			continue
		}
		n := 0
		for i := range p.MCPConfig.Services {
			svc := &p.MCPConfig.Services[i]
			oldKey := LegacyMCPProjectTokenKey(svc.Name, p.ID)
			if svc.TokenKey != oldKey {
				continue
			}
			newKey := MCPProjectTokenKey(svc.Name, p.ID)
			oldVal, err := store.Get(ctx, oldKey)
			if err != nil {
				slog.Warn("project token key migration: keychain read failed", "key", oldKey, "error", err)
				continue
			}
			if oldVal != "" {
				newVal, err := store.Get(ctx, newKey)
				if err != nil {
					slog.Warn("project token key migration: keychain read failed", "key", newKey, "error", err)
					continue
				}
				if newVal == "" {
					if err := store.Set(ctx, newKey, oldVal); err != nil {
						slog.Warn("project token key migration: keychain write failed", "key", newKey, "error", err)
						continue
					}
				}
			}
			svc.TokenKey = newKey
			n++
		}
		if n == 0 {
			continue
		}
		if err := projects.Update(ctx, p); err != nil {
			return total, fmt.Errorf("updating project %s after token key migration: %w", p.ID, err)
		}
		total += n
	}
	return total, nil
}

func hasLegacyProjectTokenKey(p *domain.Project) bool {
	if p.MCPConfig == nil {
		return false
	}
	for _, svc := range p.MCPConfig.Services {
		if svc.TokenKey != "" && svc.TokenKey == LegacyMCPProjectTokenKey(svc.Name, p.ID) {
			return true
		}
	}
	return false
}
