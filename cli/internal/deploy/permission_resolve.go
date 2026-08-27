package deploy

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ResolvePermissions loads the base permission file (if declared via permission_base)
// and deep-merges the agent's local overrides on top. Returns the final permission map.
//
// Merge semantics:
//   - Base provides the default permission set
//   - Agent overrides ADD or REPLACE keys (deep merge for nested maps like bash/task)
//   - To REMOVE a base permission, the agent overrides it with "deny"
//   - Single-level inheritance only (a base cannot reference another base)
func ResolvePermissions(hubDir string, fm *AgentFrontmatter) (map[string]interface{}, error) {
	if fm.PermissionBase == "" {
		// No base declared — use agent's own permissions directly
		return fm.Permission, nil
	}

	// Load base permission file
	basePath := filepath.Join(hubDir, "permissions", fm.PermissionBase+".yaml")
	baseData, err := os.ReadFile(basePath)
	if err != nil {
		return nil, fmt.Errorf("loading permission base %q: %w", fm.PermissionBase, err)
	}

	var base map[string]interface{}
	if err := yaml.Unmarshal(baseData, &base); err != nil {
		return nil, fmt.Errorf("parsing permission base %q: %w", fm.PermissionBase, err)
	}
	base = normalizeMap(base)

	// If agent has no local overrides, return base as-is
	if len(fm.Permission) == 0 {
		return base, nil
	}

	// Merge: agent overrides win (deep merge for nested maps like bash/task)
	return deepMerge(base, fm.Permission), nil
}

// deepMerge merges src into dst recursively. For each key:
//   - If both src and dst have a nested map → recurse (allows adding new bash rules)
//   - Otherwise src value wins (override)
//
// Returns a new map — neither dst nor src are mutated.
func deepMerge(dst, src map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(dst)+len(src))
	for k, v := range dst {
		result[k] = v
	}
	for k, v := range src {
		if srcMap, ok := v.(map[string]interface{}); ok {
			if dstMap, ok := result[k].(map[string]interface{}); ok {
				result[k] = deepMerge(dstMap, srcMap)
				continue
			}
		}
		result[k] = v
	}
	return result
}
