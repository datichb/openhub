package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// runDeployForProject deploys hub content to a project in-process.
func runDeployForProject(a *app.App, project *domain.Project) error {
	hubDir := findHubDir()
	if hubDir == "" {
		return fmt.Errorf("hub content not found")
	}

	plan := buildDeployPlan(a, DeployRequest{Project: project, HubDir: hubDir, Provider: project.Provider})

	_, err := deploy.Execute(context.Background(), plan)
	if err != nil {
		return fmt.Errorf("deploy %s: %w", project.Name, err)
	}
	return nil
}

// runSyncAll synchronizes hub content to all active projects.
func runSyncAll(a *app.App) error {
	ctx := context.Background()
	projects, err := a.Projects.List(ctx, domain.ProjectStatusActive)
	if err != nil {
		return fmt.Errorf("listing projects: %w", err)
	}

	var lastErr error
	for i := range projects {
		if err := runDeployForProject(a, &projects[i]); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// runUpgradeOpencode updates the opencode binary to the latest version.
// An optional progressFn is called with (downloaded, total) bytes during the download.
func runUpgradeOpencode(progressFn opencode.ProgressFunc) error {
	installDir := filepath.Join(config.HubDir(), "bin")
	_, err := opencode.Download("latest", installDir, progressFn)
	return err
}

// ─────────────────────────────────────────────────────────────────────────────
// Project Add inline wizard
// ─────────────────────────────────────────────────────────────────────────────

func actionProjectAdd() {
	if tuiShell == nil {
		return
	}
	a := MustApp()
	wizard := buildProjectAddInlineWizard(a)
	tuiShell.PushView(wizard)
}

func buildProjectAddInlineWizard(a *app.App) *views.InlineWizardView {
	cwd, _ := os.Getwd()

	var (
		name          string
		path          string
		language      string
		absPath       string
		useCustom     bool
		providerVal   string
		model         string
		apiKey        string
		agents        []string
		mcpServices   []string
		projectTeamID *string
		doDeploy      bool
	)

	hubProvider := a.Config.Opencode.DefaultProvider
	if hubProvider == "" {
		hubProvider = "bedrock"
	}

	languageOptions := []string{"Go", "TypeScript", "Python", "Rust", "Java", i18n.T("form.option.other")}
	languageValues := []string{"go", "typescript", "python", "rust", "java", "other"}

	providerOptions := []string{"Amazon Bedrock", "Anthropic (direct)", "OpenAI", "OpenRouter", i18n.T("form.option.other")}
	providerValues := []string{"bedrock", "anthropic", "openai", "openrouter", "other"}

	// Discover available agents
	var availableAgents []string
	if hubDir := findHubDir(); hubDir != "" {
		agentsDir := filepath.Join(hubDir, "agents")
		if _, err := os.Stat(agentsDir); err == nil {
			_ = filepath.WalkDir(agentsDir, func(p string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				if filepath.Ext(p) == ".md" {
					availableAgents = append(availableAgents, strings.TrimSuffix(d.Name(), ".md"))
				}
				return nil
			})
		}
	}

	mcpOpts := []struct {
		label string
		value string
	}{
		{"Figma (" + i18n.T("form.project.mcp_requires") + " FIGMA_TOKEN)", "figma"},
		{"GitLab (" + i18n.T("form.project.mcp_requires") + " GITLAB_TOKEN)", "gitlab"},
		{"Google Slides (" + i18n.T("form.project.mcp_requires") + " GOOGLE_ACCESS_TOKEN)", "gslides"},
	}

	steps := []views.WizardStep{
		// ── Step 1: Project identity ──
		{
			Label:    i18n.T("cmd.init.project_name"),
			Required: true,
			Validate: func() string {
				if strings.TrimSpace(name) == "" {
					return i18n.T("cmd.init.project_name_required")
				}
				return ""
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddInputField(i18n.T("cmd.init.project_name"), "", 0, nil,
					func(text string) { name = text })
				form.AddInputField(i18n.T("cmd.init.project_path"), "", 0, nil,
					func(text string) { path = text })
				form.AddDropDown(i18n.T("cmd.init.language"), languageOptions, 0,
					func(_ string, index int) {
						if index >= 0 && index < len(languageValues) {
							language = languageValues[index]
						}
					})
				form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
				return form
			},
			OnDone: func() error {
				if path == "" {
					path = cwd
				}
				var err error
				absPath, err = filepath.Abs(expandPath(path))
				if err != nil {
					return fmt.Errorf("resolving path: %w", err)
				}
				if _, err := os.Stat(absPath); os.IsNotExist(err) {
					return fmt.Errorf("%s", i18n.Tf("cmd.project.add.dir_not_exist", absPath))
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{
					{Label: "Name", Value: name},
					{Label: "Path", Value: absPath},
					{Label: "Language", Value: language},
				}
			},
		},

		// ── Step 2: Beads ──
		{
			Label:      "Beads",
			Processing: i18n.T("form.project.beads_init_processing"),
			SkipIf: func() bool {
				_, err := exec.LookPath("bd")
				return err != nil
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddCheckbox(i18n.T("form.project.beads_init"), true, nil)
				form.AddButton(i18n.T("wizard.hint.submit"), func() {
					onDone()
				})
				return form
			},
			OnDone: func() error {
				id := generateProjectID(name)
				return beads.Init(absPath, id)
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: "Beads", Value: "initialized"}}
			},
		},

		// ── Step 3: Provider ──
		{
			Label: "Provider",
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddCheckbox(
					i18n.Tf("form.project.provider_custom", hubProvider),
					false,
					func(checked bool) { useCustom = checked },
				)
				form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
				return form
			},
			InfoFields: func() []views.InfoField {
				if useCustom {
					return []views.InfoField{{Label: "Provider", Value: "custom"}}
				}
				return []views.InfoField{{Label: "Provider", Value: hubProvider + " (hub)"}}
			},
		},

		// ── Step 4: Provider config (conditional) ──
		{
			Label: "Provider config",
			SkipIf: func() bool {
				return !useCustom
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddDropDown(i18n.T("form.project.provider_select"), providerOptions, 0,
					func(_ string, index int) {
						if index >= 0 && index < len(providerValues) {
							providerVal = providerValues[index]
						}
					})
				providerVal = providerValues[0]
				form.AddInputField(i18n.T("form.project.model_input"), "", 0, nil,
					func(text string) { model = text })
				form.AddPasswordField(i18n.T("form.project.api_key"), "", 0, '*',
					func(text string) { apiKey = text })
				form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
				return form
			},
			OnDone: func() error {
				if model == "" {
					model = "claude-sonnet-4-5"
				}
				if apiKey != "" && a.Secrets != nil {
					keyName := providerVal + "-token-project"
					if err := a.Secrets.Set(context.Background(), keyName, apiKey); err != nil {
						return fmt.Errorf("storing API key: %w", err)
					}
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{
					{Label: "Provider", Value: providerVal},
					{Label: "Model", Value: model},
				}
			},
		},

		// ── Step 5: Agents ──
		{
			Label: "Agents",
			SkipIf: func() bool {
				return len(availableAgents) == 0
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				selected := make(map[string]bool, len(availableAgents))
				for _, ag := range availableAgents {
					selected[ag] = true
				}
				for _, ag := range availableAgents {
					agName := ag
					form.AddCheckbox(agName, true,
						func(checked bool) { selected[agName] = checked })
				}
				form.AddButton(i18n.T("wizard.hint.submit"), func() {
					agents = nil
					for _, ag := range availableAgents {
						if selected[ag] {
							agents = append(agents, ag)
						}
					}
					onDone()
				})
				return form
			},
			InfoFields: func() []views.InfoField {
				if len(agents) == 0 {
					return []views.InfoField{{Label: "Agents", Value: "none"}}
				}
				return []views.InfoField{{Label: "Agents", Value: fmt.Sprintf("%d selected", len(agents))}}
			},
		},

		// ── Step 6: MCP Services ──
		{
			Label: "MCP",
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				selected := make(map[string]bool, len(mcpOpts))
				for _, opt := range mcpOpts {
					optVal := opt.value
					form.AddCheckbox(opt.label, false,
						func(checked bool) { selected[optVal] = checked })
				}
				form.AddButton(i18n.T("wizard.hint.submit"), func() {
					mcpServices = nil
					for _, opt := range mcpOpts {
						if selected[opt.value] {
							mcpServices = append(mcpServices, opt.value)
						}
					}
					onDone()
				})
				return form
			},
			InfoFields: func() []views.InfoField {
				if len(mcpServices) == 0 {
					return []views.InfoField{{Label: "MCP", Value: "none"}}
				}
				return []views.InfoField{{Label: "MCP", Value: strings.Join(mcpServices, ", ")}}
			},
		},

		// ── Step 7: Team ──
		buildProjectTeamStep(a, &projectTeamID),

		// ── Step 8: Deploy ──
		{
			Label: "Deploy",
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddCheckbox(i18n.T("form.project.deploy_now"), false,
					func(checked bool) { doDeploy = checked })
				form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
				return form
			},
			InfoFields: func() []views.InfoField {
				if doDeploy {
					return []views.InfoField{{Label: "Deploy", Value: "yes"}}
				}
				return []views.InfoField{{Label: "Deploy", Value: "skip"}}
			},
		},
	}

	return views.NewInlineWizardView(views.InlineWizardConfig{
		ID:                 "wizard.project.add",
		Title:              i18n.T("cmd.project.add.short"),
		Steps:              steps,
		SummaryTargetView:  "projects.list",
		SummaryTargetLabel: i18n.T("wizard.summary.goto_projects"),
		OnComplete: func(completed bool, err error) {
			if !completed || err != nil {
				return
			}
			ctx := context.Background()
			id := generateProjectID(name)
			now := time.Now()
			p := &domain.Project{
				ID:        id,
				Name:      name,
				Path:      absPath,
				Language:  language,
				Provider:  providerVal,
				Model:     model,
				Agents:    agents,
				MCP:       mcpServices,
				MCPConfig: buildProjectMCPConfig(mcpServices),
				TeamID:    projectTeamID,
				Status:    domain.ProjectStatusActive,
				CreatedAt: now,
				UpdatedAt: now,
			}
			result, _, upsertErr := upsertProject(ctx, a.Projects, p)
			if upsertErr != nil {
				slog.Warn("failed to create/update project", "error", upsertErr)
				return
			}

			if doDeploy {
				_ = runDeployForProject(a, result)
				if err := addGitExcludes(absPath); err != nil {
					slog.Warn("failed to update git excludes", "error", err)
				}
			}
		},
	})
}
