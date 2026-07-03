package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise oh pour la première fois",
	Long: `Lance un wizard de configuration pour initialiser le hub et enregistrer le premier projet.

Le wizard configure :
  - La langue de l'interface et la version d'opencode
  - Le projet (nom, chemin, langage, tracker)
  - Les serveurs MCP à activer (Figma, GitLab, Google Slides)
  - Le tracker de tickets (bd) si disponible
  - Déploie automatiquement les agents et skills dans le projet`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, common.Title.Render(i18n.T("cmd.init.welcome")))
	fmt.Fprintln(os.Stdout)

	var (
		language    string
		projectName string
		projectPath string
		projectLang string
		tracker     string
		opencodeVer string
	)

	cwd, _ := os.Getwd()

	// --- Group 1: Global config ---
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(i18n.T("cmd.init.language_select")).
				Options(
					huh.NewOption("Français", "fr"),
					huh.NewOption("English", "en"),
				).
				Value(&language),
		).Title(i18n.T("cmd.init.global_config")),

		huh.NewGroup(
			huh.NewInput().
				Title(i18n.T("cmd.init.opencode_version")).
				Description(i18n.T("cmd.init.opencode_version_desc")).
				Placeholder("latest").
				Value(&opencodeVer),
		).Title(i18n.T("cmd.init.opencode_dep_title")),

		huh.NewGroup(
			huh.NewInput().
				Title(i18n.T("cmd.init.project_name")).
				Description(i18n.T("cmd.init.project_name_desc")).
				Value(&projectName).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("%s", i18n.T("cmd.init.project_name_required"))
					}
					return nil
				}),

			huh.NewInput().
				Title(i18n.T("cmd.init.project_path")).
				Placeholder(cwd).
				Value(&projectPath),

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
				Value(&projectLang),

			huh.NewSelect[string]().
				Title(i18n.T("cmd.init.tracker")).
				Options(
					huh.NewOption(i18n.T("form.option.none"), ""),
					huh.NewOption("GitHub Issues", "github"),
					huh.NewOption("GitLab Issues", "gitlab"),
					huh.NewOption("Jira", "jira"),
					huh.NewOption("Linear", "linear"),
				).
				Value(&tracker),
		).Title(i18n.T("cmd.init.first_project")),
	)

	if err := form.Run(); err != nil {
		return err
	}

	// --- Group 2: MCP services selection ---
	var mcpServices []string
	mcpForm := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(i18n.T("cmd.init.mcp_select")).
				Description(i18n.T("cmd.init.mcp_select_desc")).
				Options(
					huh.NewOption("Figma — design tokens & composants", "figma"),
					huh.NewOption("GitLab — merge requests & pipelines", "gitlab"),
					huh.NewOption("Google Slides — présentations", "gslides"),
				).
				Value(&mcpServices),
		).Title(i18n.T("cmd.init.mcp_title")),
	)

	if err := mcpForm.Run(); err != nil {
		return err
	}

	// --- Group 3: Tracker (bd) integration ---
	trackerInitialized := false
	if tracker != "" {
		trackerInitialized = initTracker(cwd, tracker)
	}

	// --- Write config ---
	cfgDir := config.HubDir()
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	if opencodeVer == "" {
		opencodeVer = "latest"
	}

	tomlContent := buildInitConfig(language, opencodeVer, mcpServices)
	cfgPath := config.ConfigPath()
	if err := os.WriteFile(cfgPath, []byte(tomlContent), 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Fprintf(os.Stdout, "%s %s\n",
		common.SuccessStyle.Render(common.IconSuccess), i18n.Tf("cmd.init.config_written", cfgPath))

	// --- Register project ---
	if projectPath == "" {
		projectPath = cwd
	}
	absPath, err := filepath.Abs(expandPath(projectPath))
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	// Reset config cache so initApp picks up new config
	config.Reset()

	// Init app to get store access
	if err := initApp(); err != nil {
		return err
	}

	a := MustApp()

	// Create project with MCP and tracker info
	project, err := doCreateProjectFull(ctx, a, projectName, absPath, projectLang, tracker, mcpServices)
	if err != nil {
		return err
	}

	// --- Auto-deploy agents & skills ---
	if err := autoDeployAfterInit(a, project); err != nil {
		// Non-fatal: warn but don't fail init
		fmt.Fprintf(os.Stdout, "%s %s\n",
			common.WarningStyle.Render(common.IconWarning), i18n.Tf("cmd.init.deploy_failed", err))
		fmt.Fprintf(os.Stdout, "  %s\n", i18n.Tf("cmd.init.deploy_manual", common.Bold.Render("oh deploy")))
	}

	// --- Git exclude management ---
	addGitExcludes(absPath)

	// --- Final summary ---
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "%s\n", common.SuccessStyle.Render(i18n.T("cmd.init.done")))
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "  %s\n", i18n.Tf("cmd.init.done_project", common.Bold.Render(projectName), absPath))
	fmt.Fprintf(os.Stdout, "  %s\n", i18n.Tf("cmd.init.done_language", projectLang))
	if len(mcpServices) > 0 {
		fmt.Fprintf(os.Stdout, "  %s\n", i18n.Tf("cmd.init.done_mcp", strings.Join(mcpServices, ", ")))
	}
	if trackerInitialized {
		fmt.Fprintf(os.Stdout, "  %s\n", i18n.Tf("cmd.init.done_tracker", tracker))
	} else if tracker != "" {
		fmt.Fprintf(os.Stdout, "  %s\n", i18n.Tf("cmd.init.done_tracker_only", tracker))
	}
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "  %s\n", i18n.Tf("cmd.init.done_start", common.Bold.Render("oh start")))

	return nil
}

// buildInitConfig generates the hub.toml content.
func buildInitConfig(language, opencodeVer string, mcpServices []string) string {
	var sb strings.Builder
	sb.WriteString(`# oh — OpenHub CLI configuration
# Generated by oh init

[cli]
`)
	sb.WriteString(fmt.Sprintf("language = %q\n", language))
	sb.WriteString(`
[opencode]
`)
	sb.WriteString(fmt.Sprintf("version = %q\n", opencodeVer))
	sb.WriteString(`channel = "stable"
auto_update = false
install_dir = "~/.oh/bin"

[worktree]
auto_cleanup = true
base_branch = ""
`)

	// MCP configuration
	mcpSet := make(map[string]bool)
	for _, s := range mcpServices {
		mcpSet[s] = true
	}

	sb.WriteString(`
[mcp.figma]
`)
	sb.WriteString(fmt.Sprintf("enabled = %v\n", mcpSet["figma"]))
	sb.WriteString(`token_key = "figma-token"
`)

	sb.WriteString(`
[mcp.gitlab]
`)
	sb.WriteString(fmt.Sprintf("enabled = %v\n", mcpSet["gitlab"]))
	sb.WriteString(`token_key = "gitlab-token"
`)

	sb.WriteString(`
[mcp.gslides]
`)
	sb.WriteString(fmt.Sprintf("enabled = %v\n", mcpSet["gslides"]))
	sb.WriteString(`token_key = "gslides-token"
`)

	return sb.String()
}

// doCreateProjectFull creates a project with all fields populated.
func doCreateProjectFull(ctx context.Context, a *app.App, name, absPath, language, tracker string, mcpServices []string) (*domain.Project, error) {
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s", i18n.Tf("cmd.project.add.dir_not_exist", absPath))
	}

	id := generateProjectID(name)
	now := time.Now()

	p := &domain.Project{
		ID:        id,
		Name:      name,
		Path:      absPath,
		Language:  language,
		Tracker:   tracker,
		MCP:       mcpServices,
		Status:    domain.ProjectStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := a.Projects.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("creating project: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n",
		common.SuccessStyle.Render(common.IconSuccess),
		i18n.Tf("cmd.project.registered", common.Bold.Render(name), absPath),
	)
	return p, nil
}

// autoDeployAfterInit deploys hub agents, skills, config and MCP into the project.
func autoDeployAfterInit(a *app.App, project *domain.Project) error {
	hubDir := findHubDir()
	if hubDir == "" {
		return fmt.Errorf("%s", i18n.T("cmd.init.hub_not_found"))
	}

	fmt.Fprintf(os.Stdout, "\n%s %s\n",
		common.SuccessStyle.Render(common.IconArrow), i18n.T("cmd.init.deploying"))

	plan := buildDeployPlan(a, project.Path, project.ID, hubDir, "", "")

	results, err := deploy.Execute(plan)
	for _, r := range results {
		if r.Success {
			fmt.Fprintf(os.Stdout, "    %s %s\n",
				common.SuccessStyle.Render(common.IconSuccess), r.Name)
		}
	}

	return err
}

// initTracker attempts to initialize the bd ticket tracker.
// Returns true if bd was successfully initialized.
func initTracker(projectPath, tracker string) bool {
	// Check if bd is available
	bdPath, err := exec.LookPath("bd")
	if err != nil {
		fmt.Fprintf(os.Stdout, "\n%s %s\n",
			common.WarningStyle.Render(common.IconWarning), i18n.T("cmd.init.bd_not_found"))
		fmt.Fprintf(os.Stdout, "  %s\n",
			i18n.Tf("cmd.init.bd_install_hint", common.Bold.Render("brew install datichb/tap/bd")))
		return false
	}

	// Check if already initialized
	beadsDir := filepath.Join(projectPath, ".beads")
	if _, err := os.Stat(beadsDir); err == nil {
		fmt.Fprintf(os.Stdout, "\n%s %s\n",
			common.SuccessStyle.Render(common.IconSuccess), i18n.T("cmd.init.bd_already_init"))
		return true
	}

	// Initialize bd
	fmt.Fprintf(os.Stdout, "\n%s %s\n",
		common.SuccessStyle.Render(common.IconArrow), i18n.T("cmd.init.bd_init"))

	cmd := exec.Command(bdPath, "init", "--prefix")
	cmd.Dir = projectPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stdout, "%s %s\n",
			common.WarningStyle.Render(common.IconWarning), i18n.Tf("cmd.init.bd_init_failed", err))
		return false
	}

	fmt.Fprintf(os.Stdout, "%s %s\n",
		common.SuccessStyle.Render(common.IconSuccess), i18n.T("cmd.init.bd_init_done"))
	return true
}

// addGitExcludes adds opencode and oh artifacts to .git/info/exclude.
func addGitExcludes(projectPath string) {
	excludeFile := filepath.Join(projectPath, ".git", "info", "exclude")

	// Check if .git exists (it's a git repo)
	if _, err := os.Stat(filepath.Join(projectPath, ".git")); err != nil {
		return // not a git repo
	}

	// Read existing excludes
	existing, _ := os.ReadFile(excludeFile)
	content := string(existing)

	patterns := []string{
		".opencode/",
		"opencode.json",
	}

	var toAdd []string
	for _, p := range patterns {
		if !strings.Contains(content, p) {
			toAdd = append(toAdd, p)
		}
	}

	if len(toAdd) == 0 {
		return
	}

	// Ensure directory exists
	_ = os.MkdirAll(filepath.Dir(excludeFile), 0o755)

	// Append patterns
	f, err := os.OpenFile(excludeFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	// Add a comment header
	if len(existing) > 0 && !strings.HasSuffix(content, "\n") {
		f.WriteString("\n")
	}
	f.WriteString("\n# oh — OpenHub CLI artifacts\n")
	for _, p := range toAdd {
		f.WriteString(p + "\n")
	}
}
