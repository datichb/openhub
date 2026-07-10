// Package deploy handles transactional deployment of agents, skills, config,
// and MCP servers to project directories.
package deploy

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DisabledNativeAgents is the list of opencode built-in agents that are disabled
// when hub agents are deployed. Our agents replace their functionality.
var DisabledNativeAgents = []string{"build", "plan", "general", "explore", "scout"}

// Plan represents a deployment plan with all phases.
type Plan struct {
	ProjectPath       string
	ProjectID         string
	HubDir            string // source hub directory (agents/, skills/, etc.)
	Provider          string
	Model             string
	WebsearchEnabled  bool     // inject permission.websearch/webfetch = "allow"
	SelectedAgents    []string // agent names to deploy (empty = all)
	EnabledMCPServers []string // MCP server names enabled in hub config (for validation warnings)
	Phases            []Phase
}

// Phase represents a single deployment phase.
type Phase struct {
	Name    string
	Execute func(ctx *Context) error
}

// Context holds state during deployment.
type Context struct {
	Plan      *Plan
	BackupDir string
	Results   []PhaseResult
	StartedAt time.Time
}

// PhaseResult holds the outcome of a phase.
type PhaseResult struct {
	Name     string
	Success  bool
	Message  string
	Duration time.Duration
}

// Snapshot holds the backup state for rollback.
type Snapshot struct {
	BackupDir string
	CreatedAt time.Time
}

// Execute runs a full deployment with transactional rollback.
func Execute(plan *Plan) ([]PhaseResult, error) {
	ctx := &Context{
		Plan:      plan,
		StartedAt: time.Now(),
	}

	// Phase 0: Create backup snapshot
	snapshot, err := createSnapshot(plan.ProjectPath)
	if err != nil {
		return nil, fmt.Errorf("creating snapshot: %w", err)
	}
	ctx.BackupDir = snapshot.BackupDir
	defer os.RemoveAll(snapshot.BackupDir) // cleanup backup on success

	// Run all phases
	for _, phase := range plan.Phases {
		start := time.Now()
		err := phase.Execute(ctx)
		result := PhaseResult{
			Name:     phase.Name,
			Success:  err == nil,
			Duration: time.Since(start),
		}
		if err != nil {
			result.Message = err.Error()
			ctx.Results = append(ctx.Results, result)
			// Rollback
			if rbErr := rollback(plan.ProjectPath, snapshot); rbErr != nil {
				return ctx.Results, fmt.Errorf("phase %q failed: %w (rollback also failed: %v)", phase.Name, err, rbErr)
			}
			return ctx.Results, fmt.Errorf("phase %q failed (rolled back): %w", phase.Name, err)
		}
		result.Message = "OK"
		ctx.Results = append(ctx.Results, result)
	}

	// Write deploy state for future --check comparisons
	if err := writeDeployState(plan); err != nil {
		// Non-fatal: deploy succeeded, state tracking is best-effort
		_ = err
	}

	return ctx.Results, nil
}

// createSnapshot backs up .opencode/ and opencode.json.
func createSnapshot(projectPath string) (*Snapshot, error) {
	backupDir, err := os.MkdirTemp("", "oh-deploy-backup-*")
	if err != nil {
		return nil, err
	}

	// Backup .opencode/ directory
	ocDir := filepath.Join(projectPath, ".opencode")
	if info, err := os.Stat(ocDir); err == nil && info.IsDir() {
		if err := copyDir(ocDir, filepath.Join(backupDir, ".opencode")); err != nil {
			os.RemoveAll(backupDir)
			return nil, fmt.Errorf("backing up .opencode/: %w", err)
		}
	}

	// Backup opencode.json
	ocJson := filepath.Join(projectPath, "opencode.json")
	if _, err := os.Stat(ocJson); err == nil {
		if err := copyFile(ocJson, filepath.Join(backupDir, "opencode.json")); err != nil {
			os.RemoveAll(backupDir)
			return nil, fmt.Errorf("backing up opencode.json: %w", err)
		}
	}

	return &Snapshot{BackupDir: backupDir, CreatedAt: time.Now()}, nil
}

// rollback restores the project from the snapshot.
func rollback(projectPath string, snapshot *Snapshot) error {
	// Restore .opencode/
	backupOC := filepath.Join(snapshot.BackupDir, ".opencode")
	destOC := filepath.Join(projectPath, ".opencode")
	if _, err := os.Stat(backupOC); err == nil {
		os.RemoveAll(destOC)
		if err := copyDir(backupOC, destOC); err != nil {
			return fmt.Errorf("restoring .opencode/: %w", err)
		}
	}

	// Restore opencode.json
	backupJson := filepath.Join(snapshot.BackupDir, "opencode.json")
	destJson := filepath.Join(projectPath, "opencode.json")
	if _, err := os.Stat(backupJson); err == nil {
		if err := copyFile(backupJson, destJson); err != nil {
			return fmt.Errorf("restoring opencode.json: %w", err)
		}
	}

	return nil
}

// --- Standard deployment phases ---

// DeployAgents copies agent .md files to .opencode/agents/.
// If selected is non-empty, only agents whose filename (sans .md) is in the list are copied.
// The destination directory is wiped first to remove stale agents from previous deploys.
// Bucket A skills are assembled inline into each agent's body.
func DeployAgents(hubDir string, selected []string) Phase {
	return Phase{
		Name: "Agents",
		Execute: func(ctx *Context) error {
			srcDir := filepath.Join(hubDir, "agents")
			destDir := filepath.Join(ctx.Plan.ProjectPath, ".opencode", "agents")
			skillsDir := filepath.Join(hubDir, "skills")

			if _, err := os.Stat(srcDir); os.IsNotExist(err) {
				return nil // No agents to deploy
			}

			// Wipe existing agents directory to remove stale agents
			if err := os.RemoveAll(destDir); err != nil {
				return fmt.Errorf("cleaning agents directory: %w", err)
			}
			if err := os.MkdirAll(destDir, 0o755); err != nil {
				return err
			}

			// Build allow set for filtering
			allowSet := make(map[string]bool, len(selected))
			for _, a := range selected {
				allowSet[a] = true
			}

			return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				if filepath.Ext(path) != ".md" {
					return nil
				}

				// Filter by selected agents (match on filename without .md extension)
				name := strings.TrimSuffix(d.Name(), ".md")
				if len(allowSet) > 0 && !allowSet[name] {
					return nil // skip unselected agent
				}

				// Flatten: write all agents directly to .opencode/agents/<name>.md
				// The agent's frontmatter `id` matches the filename, ensuring opencode's
				// file-based discovery aligns with the JSON config keys.
				dest := filepath.Join(destDir, d.Name())

				// Assemble agent with Bucket A skills inlined
				assembled, err := assembleAgentWithSkills(path, skillsDir)
				if err != nil {
					// Fall back to raw copy if assembly fails
					return copyFile(path, dest)
				}
				return os.WriteFile(dest, assembled, 0o644)
			})
		},
	}
}

// DeploySkills deploys native skills (Bucket B) to .opencode/skills/<name>/SKILL.md.
// Only skills referenced by the selected agents' `native_skills` frontmatter are deployed.
// The destination directory is wiped first to remove stale skills from previous deploys.
func DeploySkills(hubDir string, selected []string) Phase {
	return Phase{
		Name: "Skills",
		Execute: func(ctx *Context) error {
			skillsDir := filepath.Join(hubDir, "skills")
			agentsDir := filepath.Join(hubDir, "agents")
			destDir := filepath.Join(ctx.Plan.ProjectPath, ".opencode", "skills")

			if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
				return nil
			}

			// Wipe existing skills directory to remove stale skills
			if err := os.RemoveAll(destDir); err != nil {
				return fmt.Errorf("cleaning skills directory: %w", err)
			}
			if err := os.MkdirAll(destDir, 0o755); err != nil {
				return err
			}

			// Build allow set for agent filtering
			allowSet := make(map[string]bool, len(selected))
			for _, a := range selected {
				allowSet[a] = true
			}

			// Collect all native_skills from selected agents
			nativeSkillRefs := make(map[string]bool)
			_ = filepath.WalkDir(agentsDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
					return err
				}
				name := strings.TrimSuffix(d.Name(), ".md")
				if len(allowSet) > 0 && !allowSet[name] {
					return nil
				}
				fm, err := ParseAgentFrontmatter(path)
				if err != nil {
					return nil //nolint:nilerr // intentional: skip unparseable agents
				}
				for _, ref := range fm.NativeSkills {
					nativeSkillRefs[ref] = true
				}
				return nil
			})

			// Add stack-detected skills (based on project tech stack)
			stackSkills := ResolveStackSkills(ctx.Plan.ProjectPath)
			for _, ref := range stackSkills {
				nativeSkillRefs[ref] = true
			}

			// Deploy each referenced native skill in opencode format: <name>/SKILL.md
			for ref := range nativeSkillRefs {
				if err := deployNativeSkill(skillsDir, ref, destDir); err != nil {
					// Non-fatal: skip missing skills
					continue
				}
			}

			return nil
		},
	}
}

// DeployConfig writes or updates opencode.json with provider/model settings.
func DeployConfig(provider, model string) Phase {
	return Phase{
		Name: "Configuration",
		Execute: func(ctx *Context) error {
			configPath := filepath.Join(ctx.Plan.ProjectPath, "opencode.json")

			// Read existing config or start fresh
			var config map[string]interface{}
			if data, err := os.ReadFile(configPath); err == nil {
				if err := json.Unmarshal(data, &config); err != nil {
					config = make(map[string]interface{})
				}
			} else {
				config = make(map[string]interface{})
			}

			// Always set $schema for IDE validation
			config["$schema"] = "https://opencode.ai/config.json"

			// Set model if specified (opencode expects a plain string, not an object)
			if model != "" {
				config["model"] = model
			}

			// Set provider configuration (opencode expects named provider blocks with options)
			if provider != "" {
				providerCfg, ok := config["provider"].(map[string]interface{})
				if !ok {
					providerCfg = make(map[string]interface{})
				}
				// Configure the active provider with options
				switch provider {
				case "anthropic":
					if _, exists := providerCfg["anthropic"]; !exists {
						providerCfg["anthropic"] = map[string]interface{}{
							"options": map[string]interface{}{
								"setCacheKey": true,
							},
						}
					}
				case "bedrock":
					if _, exists := providerCfg["amazon-bedrock"]; !exists {
						providerCfg["amazon-bedrock"] = map[string]interface{}{}
					}
				default:
					if _, exists := providerCfg[provider]; !exists {
						providerCfg[provider] = map[string]interface{}{}
					}
				}
				config["provider"] = providerCfg

				// Use enabled_providers to restrict to the selected provider
				config["enabled_providers"] = []interface{}{providerOpencodeName(provider)}
			}

			// Inject websearch/webfetch permissions if enabled
			if ctx.Plan.WebsearchEnabled {
				permCfg, ok := config["permission"].(map[string]interface{})
				if !ok {
					permCfg = make(map[string]interface{})
				}
				permCfg["websearch"] = "allow"
				permCfg["webfetch"] = "allow"
				config["permission"] = permCfg
			}

			// Inject disabled native agents (always — our agents replace opencode's)
			agentCfg, ok := config["agent"].(map[string]interface{})
			if !ok {
				agentCfg = make(map[string]interface{})
			}
			for _, native := range DisabledNativeAgents {
				if _, exists := agentCfg[native]; !exists {
					agentCfg[native] = map[string]interface{}{"disable": true}
				} else {
					// Preserve existing config but ensure disable is set
					if m, ok := agentCfg[native].(map[string]interface{}); ok {
						m["disable"] = true
					} else {
						agentCfg[native] = map[string]interface{}{"disable": true}
					}
				}
			}
			config["agent"] = agentCfg

			// Inject plugin (always deploy context-mode)
			config["plugin"] = []interface{}{"context-mode"}

			// Inject compaction settings (standard for all projects)
			config["compaction"] = map[string]interface{}{
				"auto":     true,
				"prune":    true,
				"reserved": 10000,
			}

			// Inject instructions if documentation files exist in the project
			instructions := discoverInstructionFiles(ctx.Plan.ProjectPath)
			if len(instructions) > 0 {
				config["instructions"] = instructions
			}

			// Write atomically (temp file + rename)
			data, err := json.MarshalIndent(config, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling config: %w", err)
			}

			tmpFile := configPath + ".tmp"
			if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
				return err
			}
			return os.Rename(tmpFile, configPath)
		},
	}
}

// providerOpencodeName maps hub provider names to opencode provider identifiers.
func providerOpencodeName(provider string) string {
	switch provider {
	case "bedrock":
		return "amazon-bedrock"
	default:
		return provider
	}
}

// discoverInstructionFiles checks for documentation files in the project that should
// be included as instructions for opencode. Returns a slice of relative paths.
func discoverInstructionFiles(projectPath string) []interface{} {
	candidates := []string{
		"ONBOARDING.md",
		"CONVENTIONS.md",
		".claude/CLAUDE.md",
	}

	var found []interface{}
	for _, name := range candidates {
		path := filepath.Join(projectPath, name)
		if _, err := os.Stat(path); err == nil {
			found = append(found, name)
		}
	}
	return found
}

// --- File utilities ---

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode())
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(src, path)
		destPath := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}
		return copyFile(path, destPath)
	})
}

// --- Deploy state tracking ---

// DeployState records metadata about the last successful deploy.
// Stored in .opencode/.deploy-state as JSON.
type DeployState struct {
	DeployedAt     string   `json:"deployed_at"`
	ConfigHash     string   `json:"config_hash"` // SHA-256 of opencode.json at deploy time
	HubDir         string   `json:"hub_dir"`     // hub source directory
	Provider       string   `json:"provider"`
	Model          string   `json:"model"`
	SelectedAgents []string `json:"selected_agents"`
}

const deployStateFile = ".deploy-state"

// writeDeployState writes a state file after a successful deploy.
func writeDeployState(plan *Plan) error {
	stateDir := filepath.Join(plan.ProjectPath, ".opencode")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}

	// Hash the deployed opencode.json
	configPath := filepath.Join(plan.ProjectPath, "opencode.json")
	configHash := ""
	if data, err := os.ReadFile(configPath); err == nil {
		configHash = hashBytes(data)
	}

	state := DeployState{
		DeployedAt:     time.Now().Format(time.RFC3339),
		ConfigHash:     configHash,
		HubDir:         plan.HubDir,
		Provider:       plan.Provider,
		Model:          plan.Model,
		SelectedAgents: plan.SelectedAgents,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(stateDir, deployStateFile), data, 0o644)
}

// ReadDeployState reads the last deploy state from .opencode/.deploy-state.
// Returns nil if the file doesn't exist (never deployed).
func ReadDeployState(projectPath string) *DeployState {
	path := filepath.Join(projectPath, ".opencode", deployStateFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var state DeployState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil
	}
	return &state
}

// hashBytes returns the SHA-256 hex digest of a byte slice.
func hashBytes(data []byte) string {
	h := fmt.Sprintf("%x", sha256Sum(data))
	return h
}

func sha256Sum(data []byte) [32]byte {
	return [32]byte(sha256.Sum256(data))
}
