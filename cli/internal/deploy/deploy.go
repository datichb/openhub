// Package deploy handles transactional deployment of agents, skills, config,
// and MCP servers to project directories.
package deploy

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Plan represents a deployment plan with all phases.
type Plan struct {
	ProjectPath      string
	ProjectID        string
	HubDir           string // source hub directory (agents/, skills/, etc.)
	Provider         string
	Model            string
	WebsearchEnabled bool // inject permission.websearch/webfetch = "allow"
	Phases           []Phase
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
func DeployAgents(hubDir string) Phase {
	return Phase{
		Name: "Agents",
		Execute: func(ctx *Context) error {
			srcDir := filepath.Join(hubDir, "agents")
			destDir := filepath.Join(ctx.Plan.ProjectPath, ".opencode", "agents")

			if _, err := os.Stat(srcDir); os.IsNotExist(err) {
				return nil // No agents to deploy
			}

			if err := os.MkdirAll(destDir, 0o755); err != nil {
				return err
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

				rel, _ := filepath.Rel(srcDir, path)
				dest := filepath.Join(destDir, rel)
				if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
					return err
				}
				return copyFile(path, dest)
			})
		},
	}
}

// DeploySkills copies skill files to .opencode/skills/.
func DeploySkills(hubDir string) Phase {
	return Phase{
		Name: "Skills",
		Execute: func(ctx *Context) error {
			srcDir := filepath.Join(hubDir, "skills")
			destDir := filepath.Join(ctx.Plan.ProjectPath, ".opencode", "skills")

			if _, err := os.Stat(srcDir); os.IsNotExist(err) {
				return nil
			}

			if err := os.MkdirAll(destDir, 0o755); err != nil {
				return err
			}

			return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					relDir, _ := filepath.Rel(srcDir, path)
					if relDir != "." {
						return os.MkdirAll(filepath.Join(destDir, relDir), 0o755)
					}
					return nil
				}

				rel, _ := filepath.Rel(srcDir, path)
				dest := filepath.Join(destDir, rel)
				return copyFile(path, dest)
			})
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

			// Set provider if specified
			if provider != "" {
				providerCfg, ok := config["provider"].(map[string]interface{})
				if !ok {
					providerCfg = make(map[string]interface{})
				}
				providerCfg["default"] = provider
				config["provider"] = providerCfg
			}

			// Set model if specified
			if model != "" {
				modelCfg, ok := config["model"].(map[string]interface{})
				if !ok {
					modelCfg = make(map[string]interface{})
				}
				modelCfg["default"] = model
				config["model"] = modelCfg
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
