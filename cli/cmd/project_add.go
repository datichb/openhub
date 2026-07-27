package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/google/uuid"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/components/summary"
	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

func projectAddCmd() *cobra.Command {
	var (
		name     string
		path     string
		language string
	)

	cmd := &cobra.Command{
		Use:     "add",
		Aliases: []string{"register"},
		Short:   "Enregistre un nouveau projet",
		Long:    "Enregistre un projet dans le hub. Si aucun flag n'est fourni, lance un wizard interactif.",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()
			ctx := cmd.Context()

			// If no flags provided, run interactive form
			if name == "" && path == "" {
				return runProjectAddInteractive(ctx, a)
			}

			// Non-interactive mode (minimal — no agents/provider wizard)
			if name == "" {
				return fmt.Errorf("%s", i18n.T("cmd.project.add.name_required"))
			}
			if path == "" {
				path = "."
			}

			absPath, err := filepath.Abs(expandPath(path))
			if err != nil {
				return fmt.Errorf("resolving path: %w", err)
			}

			return doCreateProjectMinimal(ctx, a, name, absPath, language)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Nom du projet")
	cmd.Flags().StringVarP(&path, "path", "d", "", "Chemin du projet (défaut: répertoire courant)")
	cmd.Flags().StringVarP(&language, "language", "l", "", "Langage principal")

	return cmd
}

// runProjectAddInteractive is the full multi-step wizard for adding a project.
func runProjectAddInteractive(ctx context.Context, a *app.App) error {
	cwd, _ := os.Getwd()

	// ── Shared state across wizard steps ──
	var (
		name           string
		path           string
		language       string
		absPath        string
		useCustom      bool
		provider       string
		model          string
		apiKey         string
		agents         []string
		mcpServices    []string
		projectTeamCfg *domain.ProjectTeamConfig
		doDeploy       bool
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

	// MCP service definitions
	mcpOptions := []struct {
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
			Label: i18n.T("cmd.init.project_name"),
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
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error {
				if strings.TrimSpace(name) == "" {
					return fmt.Errorf("%s", i18n.T("cmd.init.project_name_required"))
				}
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
		// ── Step 2: Initialize Beads ──
		{
			Label:      "Beads",
			Processing: "Initializing beads...",
			SkipIf: func() bool {
				_, err := exec.LookPath("bd")
				return err != nil
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				doInit := true
				form.AddCheckbox(i18n.T("form.project.beads_init"), true,
					func(checked bool) { doInit = checked })
				form.AddButton("Next", func() {
					if !doInit {
						onDone()
						return
					}
					onDone()
				})
				return form
			},
			OnDone: func() error {
				id := generateProjectID(name)
				cmd := exec.Command("bd", "-C", absPath, "init", "--prefix", id, "--skip-hooks", "--skip-agents", "--setup-exclude")
				if output, err := cmd.CombinedOutput(); err != nil {
					fmt.Fprintf(a.IO.Out, "  %s bd init: %s\n",
						theme.WarningStyle.Render(theme.IconWarning),
						strings.TrimSpace(string(output)))
				} else {
					fmt.Fprintf(a.IO.Out, "  %s %s\n",
						theme.SuccessStyle.Render(theme.IconSuccess),
						i18n.T("form.project.beads_initialized"))
				}
				// Register default labels
				for _, label := range []string{"ai-delegated", "feature", "fix"} {
					_ = exec.Command("bd", "-C", absPath, "label", "create", label).Run()
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{{Label: "Beads", Value: "initialized"}}
			},
		},
		// ── Step 3: Provider — use hub default or custom? ──
		{
			Label: "Provider",
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddCheckbox(
					i18n.Tf("form.project.provider_custom", hubProvider),
					false,
					func(checked bool) { useCustom = checked },
				)
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error { return nil },
			InfoFields: func() []views.InfoField {
				if useCustom {
					return []views.InfoField{{Label: "Provider", Value: "custom (next step)"}}
				}
				return []views.InfoField{{Label: "Provider", Value: hubProvider + " (hub default)"}}
			},
		},
		// ── Step 4: Provider & Model details (conditional) ──
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
							provider = providerValues[index]
						}
					})
				provider = providerValues[0] // default
				form.AddInputField(i18n.T("form.project.model_input"), "", 0, nil,
					func(text string) { model = text })
				form.AddPasswordField(i18n.T("form.project.api_key"), "", 0, '*',
					func(text string) { apiKey = text })
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error {
				if model == "" {
					model = "claude-sonnet-4-5"
				}
				// Store API key in keychain if provided
				if apiKey != "" && a.Secrets != nil {
					keyName := provider + "-token-project"
					if err := a.Secrets.Set(context.Background(), keyName, apiKey); err != nil {
						fmt.Fprintf(a.IO.Out, "  %s %s\n",
							theme.WarningStyle.Render(theme.IconWarning),
							i18n.Tf("form.project.api_key_warning", err))
					} else {
						fmt.Fprintf(a.IO.Out, "  %s %s\n",
							theme.SuccessStyle.Render(theme.IconSuccess),
							i18n.T("form.project.api_key_stored"))
					}
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				fields := []views.InfoField{
					{Label: "Provider", Value: provider},
					{Label: "Model", Value: model},
				}
				if apiKey != "" {
					fields = append(fields, views.InfoField{Label: "API Key", Value: "stored"})
				}
				return fields
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
				// Track selection per agent (default: all selected)
				selected := make(map[string]bool, len(availableAgents))
				for _, ag := range availableAgents {
					selected[ag] = true
				}
				for _, ag := range availableAgents {
					agName := ag // capture
					form.AddCheckbox(agName, true,
						func(checked bool) { selected[agName] = checked })
				}
				form.AddButton("Next", func() {
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
			OnDone: func() error { return nil },
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
				selected := make(map[string]bool, len(mcpOptions))
				for _, opt := range mcpOptions {
					optVal := opt.value // capture
					form.AddCheckbox(opt.label, false,
						func(checked bool) { selected[optVal] = checked })
				}
				form.AddButton("Next", func() {
					mcpServices = nil
					for _, opt := range mcpOptions {
						if selected[opt.value] {
							mcpServices = append(mcpServices, opt.value)
						}
					}
					onDone()
				})
				return form
			},
			OnDone: func() error { return nil },
			InfoFields: func() []views.InfoField {
				if len(mcpServices) == 0 {
					return []views.InfoField{{Label: "MCP", Value: "none"}}
				}
				return []views.InfoField{{Label: "MCP", Value: strings.Join(mcpServices, ", ")}}
			},
		},
		// ── Step 7: Team ──
		buildProjectTeamStep(a, &projectTeamCfg),
		// ── Step 8: Deploy ──
		{
			Label: "Deploy",
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddCheckbox(i18n.T("form.project.deploy_now"), false,
					func(checked bool) { doDeploy = checked })
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error { return nil },
			InfoFields: func() []views.InfoField {
				if doDeploy {
					return []views.InfoField{{Label: "Deploy", Value: "yes"}}
				}
				return []views.InfoField{{Label: "Deploy", Value: "skip"}}
			},
		},
	}

	wizResult := views.RunWizard(views.WizardConfig{
		Layout: layout.Config{
			ProjectName: a.Config.Name,
			Command:     "project add",
			StatusHints: "enter confirm · esc skip",
		},
		Steps: steps,
	})

	if wizResult.Aborted {
		return nil
	}
	if wizResult.Err != nil {
		return wizResult.Err
	}

	// ── Create project in DB ──
	id := generateProjectID(name)
	now := time.Now()
	p := &domain.Project{
		ID:         id,
		Name:       name,
		Path:       absPath,
		Language:   language,
		Provider:   provider,
		Model:      model,
		Agents:     agents,
		MCP:        mcpServices,
		MCPConfig:  buildProjectMCPConfig(mcpServices),
		TeamConfig: projectTeamCfg,
		Status:     domain.ProjectStatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := a.Projects.Create(ctx, p); err != nil {
		return fmt.Errorf("creating project: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "\n%s %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		i18n.Tf("cmd.project.registered", theme.Bold.Render(name), absPath))

	// ── Execute deploy if requested ──
	if doDeploy {
		hubDir := findHubDir()
		if hubDir == "" {
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				theme.WarningStyle.Render(theme.IconWarning), i18n.T("cmd.start.hub_not_found_warning"))
		} else {
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				theme.SuccessStyle.Render(theme.IconArrow), i18n.T("form.project.deploying"))

			plan := buildDeployPlan(a, absPath, id, hubDir, provider, model, agents, nil, nil, nil)
			results, err := deploy.Execute(plan)
			if err != nil {
				fmt.Fprintf(a.IO.Out, "  %s %s\n",
					theme.ErrorStyle.Render(theme.IconError), err.Error())
			} else {
				for _, r := range results {
					icon := theme.SuccessStyle.Render(theme.IconSuccess)
					if !r.Success {
						icon = theme.ErrorStyle.Render(theme.IconError)
					}
					fmt.Fprintf(a.IO.Out, "  %s %s\n", icon, r.Name)
				}
			}
		}

		// Add .opencode/ and opencode.json to git excludes
		addGitExcludes(absPath)
	}

	// ── Summary ──
	fields := []summary.Field{
		{Label: "ID", Value: id},
		{Label: "Name", Value: name},
		{Label: "Path", Value: absPath},
		{Label: "Language", Value: displayOrDefault(language, "—")},
	}
	if provider != "" {
		fields = append(fields, summary.Field{Label: "Provider", Value: provider})
	}
	if model != "" {
		fields = append(fields, summary.Field{Label: "Model", Value: model})
	}
	if len(agents) > 0 {
		fields = append(fields, summary.Field{Label: "Agents", Value: fmt.Sprintf("%d configured", len(agents))})
	}
	if len(mcpServices) > 0 {
		fields = append(fields, summary.Field{Label: "MCP", Value: strings.Join(mcpServices, ", ")})
	}

	fmt.Fprint(a.IO.Out, summary.Render(summary.Config{
		Title:     i18n.T("form.project.summary"),
		Icon:      theme.IconSuccess,
		IconColor: theme.LipSuccess,
		Fields:    fields,
		Footer:    i18n.Tf("form.project.next_step", "oh start -p "+id),
	}))

	return nil
}

// ── Wizard sub-steps (kept for reuse by project_configure.go) ──

// wizardAgents dynamically lists available agents from the hub agents/ dir and lets the user pick.
func wizardAgents() ([]string, error) {
	hubDir := findHubDir()
	if hubDir == "" {
		return nil, nil // no hub → skip agent selection
	}

	agentsDir := filepath.Join(hubDir, "agents")
	if _, err := os.Stat(agentsDir); os.IsNotExist(err) {
		return nil, nil
	}

	// Discover agents from .md files
	var available []string
	_ = filepath.WalkDir(agentsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if filepath.Ext(path) == ".md" {
			name := strings.TrimSuffix(d.Name(), ".md")
			available = append(available, name)
		}
		return nil
	})

	if len(available) == 0 {
		return nil, nil
	}

	// Build multi-select options (all selected by default)
	options := make([]huh.Option[string], len(available))
	for i, name := range available {
		options[i] = huh.NewOption(name, name)
	}

	var selected []string
	// Default: all selected
	selected = append(selected, available...)

	form := theme.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(i18n.T("form.project.agents_select")).
				Description(i18n.Tf("form.project.agents_select_desc", len(available))).
				Options(options...).
				Value(&selected),
		),
	)
	if err := form.Run(); err != nil {
		return nil, err
	}

	return selected, nil
}

// discoverAgents returns the list of available agent names from the hub agents/ directory.
// Pure discovery logic (no UI). Returns nil if no agents found.
func discoverAgents() []string {
	hubDir := findHubDir()
	if hubDir == "" {
		return nil
	}

	agentsDir := filepath.Join(hubDir, "agents")
	if _, err := os.Stat(agentsDir); os.IsNotExist(err) {
		return nil
	}

	var available []string
	_ = filepath.WalkDir(agentsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if filepath.Ext(path) == ".md" {
			name := strings.TrimSuffix(d.Name(), ".md")
			available = append(available, name)
		}
		return nil
	})
	return available
}

// ── Non-interactive (minimal) ──

// doCreateProjectMinimal creates a project with only basic fields (CLI flags mode).
func doCreateProjectMinimal(ctx context.Context, a *app.App, name, absPath, language string) error {
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("%s", i18n.Tf("cmd.project.add.dir_not_exist", absPath))
	}

	id := generateProjectID(name)
	now := time.Now()
	p := &domain.Project{
		ID:        id,
		Name:      name,
		Path:      absPath,
		Language:  language,
		Status:    domain.ProjectStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := a.Projects.Create(ctx, p); err != nil {
		return fmt.Errorf("creating project: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		i18n.Tf("cmd.project.registered", theme.Bold.Render(name), absPath))
	return nil
}

// ── Helpers ──

func generateProjectID(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = strings.ReplaceAll(slug, " ", "-")
	var clean strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			clean.WriteRune(r)
		}
	}
	slug = strings.Trim(clean.String(), "-")
	if slug == "" {
		slug = "project"
	}
	if len(slug) > 32 {
		slug = slug[:32]
	}
	slug = strings.TrimRight(slug, "-")
	short := uuid.New().String()[:8]
	return slug + "-" + short
}

// expandPath resolves ~ to the user's home directory.
// Go's filepath.Abs does not handle ~ expansion.
func expandPath(path string) string {
	if path == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// buildProjectMCPConfig converts a list of MCP service names into a ProjectMCPConfig.
// Services are listed without credential overrides (inherit from hub).
func buildProjectMCPConfig(services []string) *domain.ProjectMCPConfig {
	if len(services) == 0 {
		return nil
	}
	cfg := &domain.ProjectMCPConfig{}
	for _, name := range services {
		cfg.Services = append(cfg.Services, domain.ProjectMCPService{Name: name})
	}
	return cfg
}
