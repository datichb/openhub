// Package deploy handles transactional deployment of agents, skills, config,
// and MCP servers to project directories.
package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DisabledNativeAgents is the default list of opencode built-in agents that are disabled
// when hub agents are deployed. Our agents replace their functionality.
// This can be overridden per-deploy via Plan.DisabledNativeAgents.
var DisabledNativeAgents = []string{"build", "plan", "general", "explore", "scout"}

// Plan represents a deployment plan with all phases.
type Plan struct {
	ProjectPath           string
	ProjectID             string
	HubDir                string // source hub directory (agents/, skills/, etc.)
	Provider              string
	Model                 string
	WebsearchEnabled      bool                  // inject permission.websearch/webfetch = "allow"
	SelectedAgents        []string              // agent names to deploy (empty = all)
	EnabledMCPServers     []string              // MCP server names enabled in hub config (for validation warnings)
	DisableNativeAgents   []string              // override the default DisabledNativeAgents list (nil = use default)
	ExtraInstructionFiles []string              // additional instruction files from hub.toml [deploy].instruction_files
	WorkflowResult        *WorkflowDeployResult // resolved workflow (nil = no workflow customization)
	Phases                []Phase
}

// Phase represents a single deployment phase.
type Phase struct {
	Name    string
	Execute func(ctx *Context) error
}

// Context holds state during deployment.
type Context struct {
	Ctx                    context.Context         // parent context for cancellation
	Plan                   *Plan
	BackupDir              string
	Results                []PhaseResult
	StartedAt              time.Time
	ItemCount              int                     // accumulator: items processed by current phase (reset between phases)
	MissingMCPIntegrations []MissingMCPIntegration // populated by DeployAgentConfig
}

// PhaseResult holds the outcome of a phase.
type PhaseResult struct {
	Name     string
	Success  bool
	Message  string
	Duration time.Duration
	// ItemCount is the number of items processed by this phase (agents deployed,
	// skills copied, MCP servers injected, etc.). Zero for phases that don't track items.
	ItemCount int
	// MissingIntegrations lists optional MCP integrations that agents declare
	// but that are not enabled. Populated only by the AgentConfig phase.
	// nil for all other phases.
	MissingIntegrations []MissingMCPIntegration
}

// MissingMCPIntegration describes an agent that declares an MCP server
// dependency which is not currently enabled. This is informational —
// agents work without these servers, but enabling them enriches capabilities.
type MissingMCPIntegration struct {
	AgentID     string // e.g. "designer"
	ServerName  string // e.g. "figma"
	Description string // e.g. "accès aux designs et composants via Figma"
}

// Snapshot holds the backup state for rollback.
type Snapshot struct {
	BackupDir       string
	CreatedAt       time.Time
	HadOpencodeDir  bool // .opencode/ existed before deploy
	HadOpencodeJSON bool // opencode.json existed before deploy
}

// Execute runs a full deployment with transactional rollback.
// The context is checked before each phase; if cancelled, the deployment
// is rolled back and an error is returned.
func Execute(goCtx context.Context, plan *Plan) ([]PhaseResult, error) {
	ctx := &Context{
		Ctx:       goCtx,
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
		// Check cancellation before each phase
		select {
		case <-goCtx.Done():
			if rbErr := rollback(plan.ProjectPath, snapshot); rbErr != nil {
				return ctx.Results, fmt.Errorf("cancelled (rollback failed: %v)", rbErr)
			}
			return ctx.Results, fmt.Errorf("cancelled (rolled back): %w", goCtx.Err())
		default:
		}

		start := time.Now()
		ctx.ItemCount = 0 // reset accumulator for each phase
		err := phase.Execute(ctx)
		result := PhaseResult{
			Name:                phase.Name,
			Success:             err == nil,
			Duration:            time.Since(start),
			ItemCount:           ctx.ItemCount,
			MissingIntegrations: ctx.MissingMCPIntegrations, // may be nil for non-AgentConfig phases
		}
		// Reset after capturing — only one phase should produce these
		ctx.MissingMCPIntegrations = nil
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
		slog.Warn("failed to write deploy state (next deploy may re-run)", "error", err)
	}

	// Write context manifest for freshness checking
	if plan.HubDir != "" {
		if err := WriteContextManifest(plan.HubDir, plan.ProjectPath); err != nil {
			slog.Warn("failed to write context manifest", "error", err)
		}
	}

	return ctx.Results, nil
}

// createSnapshot backs up .opencode/ and opencode.json.
func createSnapshot(projectPath string) (*Snapshot, error) {
	backupDir, err := os.MkdirTemp("", "oh-deploy-backup-*")
	if err != nil {
		return nil, err
	}

	snap := &Snapshot{BackupDir: backupDir, CreatedAt: time.Now()}

	// Backup .opencode/ directory
	ocDir := filepath.Join(projectPath, ".opencode")
	if info, err := os.Stat(ocDir); err == nil && info.IsDir() {
		snap.HadOpencodeDir = true
		if err := copyDir(ocDir, filepath.Join(backupDir, ".opencode")); err != nil {
			os.RemoveAll(backupDir)
			return nil, fmt.Errorf("backing up .opencode/: %w", err)
		}
	}

	// Backup opencode.json
	ocJson := filepath.Join(projectPath, "opencode.json")
	if _, err := os.Stat(ocJson); err == nil {
		snap.HadOpencodeJSON = true
		if err := copyFile(ocJson, filepath.Join(backupDir, "opencode.json")); err != nil {
			os.RemoveAll(backupDir)
			return nil, fmt.Errorf("backing up opencode.json: %w", err)
		}
	}

	return snap, nil
}

// rollback restores the project from the snapshot.
func rollback(projectPath string, snapshot *Snapshot) error {
	destOC := filepath.Join(projectPath, ".opencode")
	destJson := filepath.Join(projectPath, "opencode.json")

	// Restore or remove .opencode/
	if snapshot.HadOpencodeDir {
		backupOC := filepath.Join(snapshot.BackupDir, ".opencode")
		os.RemoveAll(destOC)
		if err := copyDir(backupOC, destOC); err != nil {
			return fmt.Errorf("restoring .opencode/: %w", err)
		}
	} else {
		// .opencode/ did not exist before deploy — remove whatever was created
		os.RemoveAll(destOC)
	}

	// Restore or remove opencode.json
	if snapshot.HadOpencodeJSON {
		backupJson := filepath.Join(snapshot.BackupDir, "opencode.json")
		if err := copyFile(backupJson, destJson); err != nil {
			return fmt.Errorf("restoring opencode.json: %w", err)
		}
	} else {
		// opencode.json did not exist before deploy — remove if created
		os.Remove(destJson)
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
					if cpErr := copyFile(path, dest); cpErr != nil {
						return cpErr
					}
					ctx.ItemCount++
					return nil
				}
				if err := os.WriteFile(dest, assembled, 0o644); err != nil {
					return err
				}
				ctx.ItemCount++
				return nil
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

			// Resolve which skills should be deployed
			nativeSkillRefs := ResolveNativeSkillRefs(agentsDir, selected, ctx.Plan.ProjectPath)

			// Deploy each referenced native skill in opencode format: <name>/SKILL.md
			for ref := range nativeSkillRefs {
				if err := deployNativeSkill(skillsDir, ref, destDir); err != nil {
					// Non-fatal: skip missing skills
					continue
				}
				ctx.ItemCount++
			}

			// Deploy workflow-generated skills (overwrite static equivalents)
			if ctx.Plan.WorkflowResult != nil && len(ctx.Plan.WorkflowResult.GeneratedSkills) > 0 {
				if err := WriteGeneratedSkills(
					filepath.Join(ctx.Plan.ProjectPath, ".opencode"),
					ctx.Plan.WorkflowResult.GeneratedSkills,
				); err != nil {
					return fmt.Errorf("writing generated workflow skills: %w", err)
				}
				ctx.ItemCount += len(ctx.Plan.WorkflowResult.GeneratedSkills)
			}

			return nil
		},
	}
}

// ResolveNativeSkillRefs collects all native skill references that should be
// deployed for the given set of selected agents. It parses native_skills from
// agent frontmatter and adds stack-detected skills based on the project path.
// This function is shared between DeploySkills and ComputeDiff to ensure the
// same set of skills is considered in both paths.
func ResolveNativeSkillRefs(agentsDir string, selectedAgents []string, projectPath string) map[string]bool {
	allowSet := make(map[string]bool, len(selectedAgents))
	for _, a := range selectedAgents {
		allowSet[a] = true
	}

	nativeSkillRefs := make(map[string]bool)

	if _, err := os.Stat(agentsDir); err == nil {
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
	}

	// Add stack-detected skills (based on project tech stack)
	if projectPath != "" {
		stackSkills := ResolveStackSkills(projectPath)
		for _, ref := range stackSkills {
			nativeSkillRefs[ref] = true
		}
	}

	return nativeSkillRefs
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
			disableList := ctx.Plan.DisableNativeAgents
			if disableList == nil {
				disableList = DisabledNativeAgents
			}
			for _, native := range disableList {
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

			// Inject plugin: ensure context-mode is present without removing
			// other plugins the user may have configured manually.
			existingPlugins, _ := config["plugin"].([]interface{})
			hasContextMode := false
			for _, p := range existingPlugins {
				if p == "context-mode" {
					hasContextMode = true
					break
				}
			}
			if !hasContextMode {
				existingPlugins = append(existingPlugins, "context-mode")
			}
			config["plugin"] = existingPlugins

			// Inject compaction settings only if not already configured,
			// so users can tune reserved tokens or disable auto-compaction.
			if _, exists := config["compaction"]; !exists {
				config["compaction"] = map[string]interface{}{
					"auto":     true,
					"prune":    true,
					"reserved": 10000,
				}
			}

			// Inject subagent_depth to support the full delegation chain:
			// orchestrator → orchestrator-dev → developer.
			// The hub TUI adds an extra parent session level when launching
			// sessions from worktrees, so depth 3 is required:
			//   hub(0) → orchestrator(1) → orchestrator-dev(2) → developer(3)
			// We only set it if it is not already explicitly configured,
			// so users can override upward if they have deeper chains.
			if _, exists := config["subagent_depth"]; !exists {
				config["subagent_depth"] = 3
			}

			// Inject instructions: merge discovered doc files with any
			// user-configured instructions instead of overwriting them.
			discovered := discoverInstructionFiles(ctx.Plan.ProjectPath, ctx.Plan.ExtraInstructionFiles)
			if len(discovered) > 0 {
				existing, _ := config["instructions"].([]interface{})
				existingSet := make(map[string]bool, len(existing))
				for _, e := range existing {
					if s, ok := e.(string); ok {
						existingSet[s] = true
					}
				}
				for _, d := range discovered {
					if s, ok := d.(string); ok && !existingSet[s] {
						existing = append(existing, d)
					}
				}
				config["instructions"] = existing
			}

			// NOTE: The workflow defaultMode is already injected into the
			// generated skill markdown (orchestrator-workflow-modes.md.tmpl).
			// Do NOT inject a "workflow" key into opencode.json — opencode
			// does not recognize it and rejects the config as invalid.

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
// be included as instructions for opencode. The extra parameter allows hub.toml to
// specify additional files beyond the built-in defaults. Returns a slice of relative paths.
func discoverInstructionFiles(projectPath string, extra []string) []interface{} {
	candidates := []string{
		"ONBOARDING.md",
		"CONVENTIONS.md",
		".claude/CLAUDE.md",
	}
	candidates = append(candidates, extra...)

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
	DeployedAt     string                 `json:"deployed_at"`
	ConfigHash     string                 `json:"config_hash"`     // SHA-256 of opencode.json at deploy time
	ConfigSnapshot map[string]interface{} `json:"config_snapshot"` // parsed JSON of opencode.json at deploy time (for key-by-key diff)
	HubDir         string                 `json:"hub_dir"`         // hub source directory
	Provider       string                 `json:"provider"`
	Model          string                 `json:"model"`
	SelectedAgents []string               `json:"selected_agents"`
	WorkflowHash   string                 `json:"workflow_hash,omitempty"` // SHA-256 of resolved workflow JSON
}

const deployStateFile = ".deploy-state"

// writeDeployState writes a state file after a successful deploy.
func writeDeployState(plan *Plan) error {
	stateDir := filepath.Join(plan.ProjectPath, ".opencode")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}

	// Hash the deployed opencode.json and capture a snapshot for key-by-key diff
	configPath := filepath.Join(plan.ProjectPath, "opencode.json")
	configHash := ""
	var configSnapshot map[string]interface{}
	if data, err := os.ReadFile(configPath); err == nil {
		configHash = BytesHash(data)
		_ = json.Unmarshal(data, &configSnapshot) // best-effort; nil on parse failure
	}

	state := DeployState{
		DeployedAt:     time.Now().Format(time.RFC3339),
		ConfigHash:     configHash,
		ConfigSnapshot: configSnapshot,
		HubDir:         plan.HubDir,
		Provider:       plan.Provider,
		Model:          plan.Model,
		SelectedAgents: plan.SelectedAgents,
	}

	// Record workflow hash for staleness detection
	if plan.WorkflowResult != nil {
		if wfData, err := json.Marshal(plan.WorkflowResult.Resolved); err == nil {
			state.WorkflowHash = BytesHash(wfData)
		}
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
