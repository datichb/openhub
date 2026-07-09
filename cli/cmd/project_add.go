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
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/common"
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
	cmd.Flags().StringVarP(&path, "path", "p", "", "Chemin du projet (défaut: répertoire courant)")
	cmd.Flags().StringVarP(&language, "language", "l", "", "Langage principal")

	return cmd
}

// runProjectAddInteractive is the full 6-step wizard for adding a project.
func runProjectAddInteractive(ctx context.Context, a *app.App) error {
	var (
		name     string
		path     string
		language string
	)

	cwd, _ := os.Getwd()

	// ── Step 1: Project identity ──
	form1 := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(i18n.T("cmd.init.project_name")).
				Description(i18n.T("form.project.add_name_desc")).
				Value(&name).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("%s", i18n.T("cmd.init.project_name_required"))
					}
					return nil
				}),

			huh.NewInput().
				Title(i18n.T("cmd.init.project_path")).
				Description(i18n.T("form.project.add_path_desc")).
				Value(&path).
				Placeholder(cwd),

			huh.NewSelect[string]().
				Title(i18n.T("cmd.init.language")).
				Options(
					huh.NewOption("Go", "go"),
					huh.NewOption("TypeScript", "typescript"),
					huh.NewOption("Python", "python"),
					huh.NewOption("Rust", "rust"),
					huh.NewOption("Java", "java"),
					huh.NewOption(i18n.T("form.option.other"), "other"),
				).
				Value(&language),
		),
	)
	if err := form1.Run(); err != nil {
		return err
	}

	if path == "" {
		path = cwd
	}
	absPath, err := filepath.Abs(expandPath(path))
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("%s", i18n.Tf("cmd.project.add.dir_not_exist", absPath))
	}

	// ── Step 2: Initialize Beads ──
	initBeads(a, absPath, generateProjectID(name))

	// ── Step 3: Provider & Model ──
	provider, model, err := wizardProviderModel(a)
	if err != nil {
		return err
	}

	// ── Step 4: Agents ──
	agents, err := wizardAgents()
	if err != nil {
		return err
	}

	// ── Step 5: MCP Services ──
	mcpServices, err := wizardMCP(a, ctx)
	if err != nil {
		return err
	}

	// ── Create project in DB ──
	id := generateProjectID(name)
	now := time.Now()
	p := &domain.Project{
		ID:        id,
		Name:      name,
		Path:      absPath,
		Language:  language,
		Provider:  provider,
		Model:     model,
		Agents:    agents,
		MCP:       mcpServices,
		MCPConfig: buildProjectMCPConfig(mcpServices),
		Status:    domain.ProjectStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := a.Projects.Create(ctx, p); err != nil {
		return fmt.Errorf("creating project: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "\n%s %s\n",
		common.SuccessStyle.Render(common.IconSuccess),
		i18n.Tf("cmd.project.registered", common.Bold.Render(name), absPath))

	// ── Step 6: Deploy ──
	var doDeploy bool
	_ = huh.NewConfirm().
		Title(i18n.T("form.project.deploy_now")).
		Value(&doDeploy).
		Affirmative(i18n.T("form.yes")).
		Negative(i18n.T("form.no")).
		Run()

	if doDeploy {
		hubDir := findHubDir()
		if hubDir == "" {
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				common.WarningStyle.Render(common.IconWarning), i18n.T("cmd.start.hub_not_found_warning"))
		} else {
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				common.SuccessStyle.Render(common.IconArrow), i18n.T("form.project.deploying"))

			plan := buildDeployPlan(a, absPath, id, hubDir, provider, model, agents, nil, nil)
			results, err := deploy.Execute(plan)
			if err != nil {
				fmt.Fprintf(a.IO.Out, "  %s %s\n",
					common.ErrorStyle.Render(common.IconError), err.Error())
			} else {
				for _, r := range results {
					icon := common.SuccessStyle.Render(common.IconSuccess)
					if !r.Success {
						icon = common.ErrorStyle.Render(common.IconError)
					}
					fmt.Fprintf(a.IO.Out, "  %s %s\n", icon, r.Name)
				}
			}
		}

		// Add .opencode/ and opencode.json to git excludes
		addGitExcludes(absPath)
	}

	// ── Summary ──
	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "%s\n", common.Title.Render("  "+i18n.T("form.project.summary")+"  "))
	fmt.Fprintf(a.IO.Out, "  %-14s %s\n", i18n.T("form.project.summary_id"), common.Bold.Render(id))
	fmt.Fprintf(a.IO.Out, "  %-14s %s\n", i18n.T("form.project.summary_name"), name)
	fmt.Fprintf(a.IO.Out, "  %-14s %s\n", i18n.T("form.project.summary_path"), absPath)
	fmt.Fprintf(a.IO.Out, "  %-14s %s\n", i18n.T("form.project.summary_lang"), displayOrDefault(language, "—"))
	if provider != "" {
		fmt.Fprintf(a.IO.Out, "  %-14s %s\n", "Provider:", provider)
	}
	if model != "" {
		fmt.Fprintf(a.IO.Out, "  %-14s %s\n", "Model:", model)
	}
	if len(agents) > 0 {
		fmt.Fprintf(a.IO.Out, "  %-14s %d agents\n", "Agents:", len(agents))
	}
	if len(mcpServices) > 0 {
		fmt.Fprintf(a.IO.Out, "  %-14s %s\n", "MCP:", strings.Join(mcpServices, ", "))
	}
	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("form.project.next_step", common.Bold.Render("oh start -j "+id)))

	return nil
}

// ── Wizard sub-steps ──

// wizardProviderModel asks the user to configure a project-specific provider or use hub default.
func wizardProviderModel(a *app.App) (prov, mod string, err error) {
	hubProvider := a.Config.Opencode.DefaultProvider
	if hubProvider == "" {
		hubProvider = "bedrock"
	}

	var useCustom bool
	_ = huh.NewConfirm().
		Title(i18n.Tf("form.project.provider_custom", hubProvider)).
		Description(i18n.T("form.project.provider_custom_desc")).
		Value(&useCustom).
		Affirmative(i18n.T("form.project.provider_specific")).
		Negative(i18n.Tf("form.project.provider_hub", hubProvider)).
		Run()

	if !useCustom {
		return "", "", nil // use hub default (empty = inherit)
	}

	var provider, model, apiKey string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(i18n.T("form.project.provider_select")).
				Options(
					huh.NewOption("Amazon Bedrock", "bedrock"),
					huh.NewOption("Anthropic (direct)", "anthropic"),
					huh.NewOption("OpenAI", "openai"),
					huh.NewOption("OpenRouter", "openrouter"),
					huh.NewOption(i18n.T("form.option.other"), "other"),
				).
				Value(&provider),

			huh.NewInput().
				Title(i18n.T("form.project.model_input")).
				Description(i18n.T("form.project.model_input_desc")).
				Value(&model).
				Placeholder("claude-sonnet-4-5"),

			huh.NewInput().
				Title(i18n.T("form.project.api_key")).
				Description(i18n.T("form.project.api_key_desc")).
				EchoMode(huh.EchoModePassword).
				Value(&apiKey),
		),
	)
	if err := form.Run(); err != nil {
		return "", "", err
	}

	// Store API key in keychain if provided
	if apiKey != "" && a.Secrets != nil {
		keyName := provider + "-token-project"
		if err := a.Secrets.Set(context.Background(), keyName, apiKey); err != nil {
			fmt.Fprintf(a.IO.Out, "  %s %s\n",
				common.WarningStyle.Render(common.IconWarning),
				i18n.Tf("form.project.api_key_warning", err))
		} else {
			fmt.Fprintf(a.IO.Out, "  %s %s\n",
				common.SuccessStyle.Render(common.IconSuccess),
				i18n.T("form.project.api_key_stored"))
		}
	}

	if model == "" {
		model = "claude-sonnet-4-5"
	}

	return provider, model, nil
}

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

	form := huh.NewForm(
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

// wizardMCP lets the user select which MCP services to enable.
func wizardMCP(a *app.App, ctx context.Context) ([]string, error) {
	options := []huh.Option[string]{
		huh.NewOption("Figma ("+i18n.T("form.project.mcp_requires")+" FIGMA_TOKEN)", "figma"),
		huh.NewOption("GitLab ("+i18n.T("form.project.mcp_requires")+" GITLAB_TOKEN)", "gitlab"),
		huh.NewOption("Google Slides ("+i18n.T("form.project.mcp_requires")+" GOOGLE_ACCESS_TOKEN)", "gslides"),
	}

	var selected []string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(i18n.T("form.project.mcp_select")).
				Description(i18n.T("form.project.mcp_select_desc")).
				Options(options...).
				Value(&selected),
		),
	)
	if err := form.Run(); err != nil {
		return nil, err
	}

	return selected, nil
}

// initBeads initializes Beads in the project if bd is available.
func initBeads(a *app.App, projectPath, projectID string) {
	if _, err := exec.LookPath("bd"); err != nil {
		fmt.Fprintf(a.IO.Out, "  %s %s\n",
			common.Subtitle.Render(common.IconArrow),
			i18n.T("form.project.bd_not_found"))
		return
	}

	var doInit bool
	_ = huh.NewConfirm().
		Title(i18n.T("form.project.beads_init")).
		Value(&doInit).
		Affirmative(i18n.T("form.yes")).
		Negative(i18n.T("form.no")).
		Run()

	if !doInit {
		return
	}

	// bd init --prefix PROJECT_ID --skip-hooks
	cmd := exec.Command("bd", "-C", projectPath, "init", "--prefix", projectID, "--skip-hooks")
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(a.IO.Out, "  %s bd init: %s\n",
			common.WarningStyle.Render(common.IconWarning),
			strings.TrimSpace(string(output)))
	} else {
		fmt.Fprintf(a.IO.Out, "  %s %s\n",
			common.SuccessStyle.Render(common.IconSuccess),
			i18n.T("form.project.beads_initialized"))
	}

	// Register default labels
	for _, label := range []string{"ai-delegated", "feature", "fix"} {
		_ = exec.Command("bd", "-C", projectPath, "label", "create", label).Run()
	}
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
		common.SuccessStyle.Render(common.IconSuccess),
		i18n.Tf("cmd.project.registered", common.Bold.Render(name), absPath))
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
