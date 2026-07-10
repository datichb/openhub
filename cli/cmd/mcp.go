package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/mcp/figma"
	"github.com/datichb/openhub/cli/internal/mcp/gitlab"
	"github.com/datichb/openhub/cli/internal/mcp/gslides"
	"github.com/datichb/openhub/cli/internal/mcp/team"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

// validMCPServices is the canonical list of supported MCP service names.
var validMCPServices = []string{"figma", "gitlab", "gslides", "team"}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: i18n.T("cmd.mcp.short"),
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.AddCommand(mcpServeCmd())
	mcpCmd.AddCommand(mcpListCmd())
	mcpCmd.AddCommand(mcpEnableCmd())
	mcpCmd.AddCommand(mcpDisableCmd())
	mcpCmd.AddCommand(mcpResetCmd())
	mcpCmd.AddCommand(mcpSetupCmd())
	mcpCmd.AddCommand(mcpStatusCmd())
}

// --- Helpers ---

func isValidMCPService(name string) bool {
	for _, s := range validMCPServices {
		if s == name {
			return true
		}
	}
	return false
}

func completeMCPServices(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return validMCPServices, cobra.ShellCompDirectiveNoFileComp
}

func boolPtr(v bool) *bool { return &v }

// upsertProjectMCPService inserts or updates a service entry in the project's MCPConfig.
func upsertProjectMCPService(project *domain.Project, svc domain.ProjectMCPService) {
	if project.MCPConfig == nil {
		project.MCPConfig = &domain.ProjectMCPConfig{}
	}
	for i, existing := range project.MCPConfig.Services {
		if existing.Name == svc.Name {
			project.MCPConfig.Services[i] = svc
			return
		}
	}
	project.MCPConfig.Services = append(project.MCPConfig.Services, svc)
}

// removeProjectMCPService removes a service entry from the project's MCPConfig.
func removeProjectMCPService(project *domain.Project, serviceName string) {
	if project.MCPConfig == nil {
		return
	}
	services := project.MCPConfig.Services[:0]
	for _, s := range project.MCPConfig.Services {
		if s.Name != serviceName {
			services = append(services, s)
		}
	}
	project.MCPConfig.Services = services
}

// --- oh mcp enable ---

func mcpEnableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "enable <service>",
		Short:             i18n.T("cmd.mcp.enable.short"),
		Long:              i18n.T("cmd.mcp.enable.long"),
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeMCPServices,
		RunE:              runMCPEnable,
	}
	cmd.Flags().StringP("project", "p", "", i18n.T("cmd.mcp.flags.project"))
	_ = cmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
	return cmd
}

func runMCPEnable(cmd *cobra.Command, args []string) error {
	serviceName := args[0]
	if !isValidMCPService(serviceName) {
		return fmt.Errorf("%s", i18n.Tf("cmd.mcp.invalid_service", serviceName))
	}

	projectID, _ := cmd.Flags().GetString("project")

	if projectID == "" {
		// Hub-level enable
		v := configViper()
		v.Set("mcp."+serviceName+".enabled", true)
		cfgPath := config.ConfigPath()
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
			return fmt.Errorf("creating config dir: %w", err)
		}
		if err := v.WriteConfigAs(cfgPath); err != nil {
			return fmt.Errorf("writing config: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n",
			common.SuccessStyle.Render(common.IconSuccess),
			i18n.Tf("cmd.mcp.enable.success", common.Bold.Render(serviceName)))
		return nil
	}

	// Project-level enable
	a := MustApp()
	ctx := cmd.Context()
	project, err := resolveProject(ctx, a, projectID)
	if err != nil {
		return err
	}

	// Check if there's a token available (hub or project-scoped)
	hasToken := checkProjectToken(ctx, a, serviceName, project)
	if !hasToken && serviceName != "team" {
		// Prompt user: inherit from hub or configure?
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n\n",
			common.WarningStyle.Render(common.IconWarning),
			i18n.Tf("cmd.mcp.enable.no_token_prompt", serviceName))

		var choice string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Options(
						huh.NewOption(i18n.T("cmd.mcp.enable.token_choice_hub"), "hub"),
						huh.NewOption(i18n.T("cmd.mcp.enable.token_choice_project"), "project"),
					).
					Value(&choice),
			),
		)
		if err := form.Run(); err != nil {
			return err
		}

		if choice == "project" {
			// Delegate to setup wizard for this project/service
			return runMCPSetupForService(cmd, serviceName, project)
		}
		// choice == "hub": just enable, token will be inherited
	}

	svc := domain.ProjectMCPService{
		Name:    serviceName,
		Enabled: boolPtr(true),
	}
	upsertProjectMCPService(project, svc)

	if err := a.Projects.Update(ctx, project); err != nil {
		return fmt.Errorf("updating project: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n",
		common.SuccessStyle.Render(common.IconSuccess),
		i18n.Tf("cmd.mcp.enable.success_project", common.Bold.Render(serviceName), project.Name))
	return nil
}

// --- oh mcp disable ---

func mcpDisableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "disable <service>",
		Short:             i18n.T("cmd.mcp.disable.short"),
		Long:              i18n.T("cmd.mcp.disable.long"),
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeMCPServices,
		RunE:              runMCPDisable,
	}
	cmd.Flags().StringP("project", "p", "", i18n.T("cmd.mcp.flags.project"))
	_ = cmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
	return cmd
}

func runMCPDisable(cmd *cobra.Command, args []string) error {
	serviceName := args[0]
	if !isValidMCPService(serviceName) {
		return fmt.Errorf("%s", i18n.Tf("cmd.mcp.invalid_service", serviceName))
	}

	projectID, _ := cmd.Flags().GetString("project")

	if projectID == "" {
		// Hub-level disable
		v := configViper()
		v.Set("mcp."+serviceName+".enabled", false)
		cfgPath := config.ConfigPath()
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
			return fmt.Errorf("creating config dir: %w", err)
		}
		if err := v.WriteConfigAs(cfgPath); err != nil {
			return fmt.Errorf("writing config: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n",
			common.SuccessStyle.Render(common.IconSuccess),
			i18n.Tf("cmd.mcp.disable.success", common.Bold.Render(serviceName)))
		return nil
	}

	// Project-level disable
	a := MustApp()
	ctx := cmd.Context()
	project, err := resolveProject(ctx, a, projectID)
	if err != nil {
		return err
	}

	svc := domain.ProjectMCPService{
		Name:    serviceName,
		Enabled: boolPtr(false),
	}
	upsertProjectMCPService(project, svc)

	if err := a.Projects.Update(ctx, project); err != nil {
		return fmt.Errorf("updating project: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n",
		common.SuccessStyle.Render(common.IconSuccess),
		i18n.Tf("cmd.mcp.disable.success_project", common.Bold.Render(serviceName), project.Name))
	return nil
}

// --- oh mcp reset ---

func mcpResetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "reset <service>",
		Short:             i18n.T("cmd.mcp.reset.short"),
		Long:              i18n.T("cmd.mcp.reset.long"),
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeMCPServices,
		RunE:              runMCPReset,
	}
	cmd.Flags().StringP("project", "p", "", i18n.T("cmd.mcp.flags.project")+" (required)")
	_ = cmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
	return cmd
}

func runMCPReset(cmd *cobra.Command, args []string) error {
	serviceName := args[0]
	if !isValidMCPService(serviceName) {
		return fmt.Errorf("%s", i18n.Tf("cmd.mcp.invalid_service", serviceName))
	}

	projectID, _ := cmd.Flags().GetString("project")
	if projectID == "" {
		return fmt.Errorf("%s", i18n.T("cmd.mcp.reset.requires_project"))
	}

	a := MustApp()
	ctx := cmd.Context()
	project, err := resolveProject(ctx, a, projectID)
	if err != nil {
		return err
	}

	removeProjectMCPService(project, serviceName)

	if err := a.Projects.Update(ctx, project); err != nil {
		return fmt.Errorf("updating project: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n",
		common.SuccessStyle.Render(common.IconSuccess),
		i18n.Tf("cmd.mcp.reset.success", common.Bold.Render(serviceName), project.Name))
	return nil
}

// --- oh mcp setup ---

func mcpSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: i18n.T("cmd.mcp.setup.short"),
		Long:  i18n.T("cmd.mcp.setup.long"),
		RunE:  runMCPSetup,
	}
	cmd.Flags().StringP("project", "p", "", i18n.T("cmd.mcp.flags.project"))
	_ = cmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
	return cmd
}

func runMCPSetup(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	projectID, _ := cmd.Flags().GetString("project")
	var project *domain.Project
	if projectID != "" {
		var err error
		project, err = resolveProject(ctx, a, projectID)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s (%s)\n\n",
			common.Title.Render("oh mcp setup"),
			i18n.T("cmd.service.project_scope"), project.Name)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n\n",
			common.Title.Render("oh mcp setup"),
			i18n.T("cmd.mcp.setup.short"))
	}

	// Select service
	var serviceName string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(i18n.T("cmd.service.select")).
				Options(
					huh.NewOption("Figma — design tokens & composants", "figma"),
					huh.NewOption("GitLab — merge requests & pipelines", "gitlab"),
					huh.NewOption("Google Slides — présentations", "gslides"),
				).
				Value(&serviceName),
		),
	)
	if err := form.Run(); err != nil {
		return err
	}

	return runMCPSetupForService(cmd, serviceName, project)
}

// runMCPSetupForService configures a specific service's token and options.
// If project is nil, configuration is hub-level.
func runMCPSetupForService(cmd *cobra.Command, serviceName string, project *domain.Project) error {
	a := MustApp()
	ctx := cmd.Context()

	// Prompt for token
	var token string
	envHint := ""
	switch serviceName {
	case "figma":
		envHint = "FIGMA_TOKEN"
	case "gitlab":
		envHint = "GITLAB_TOKEN"
	case "gslides":
		envHint = "GOOGLE_ACCESS_TOKEN"
	}

	if envHint != "" {
		tokenForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title(i18n.Tf("cmd.service.token_prompt", serviceName)).
					Description(i18n.Tf("cmd.service.token_env_hint", envHint)).
					EchoMode(huh.EchoModePassword).
					Value(&token),
			),
		)
		if err := tokenForm.Run(); err != nil {
			return err
		}
	}

	// Store token
	if token == "" && envHint != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n",
			common.WarningStyle.Render(common.IconWarning),
			i18n.Tf("cmd.service.token_empty_warning", envHint))
	} else if token != "" && a.Secrets != nil {
		keyName := serviceName + "-token"
		if project != nil {
			keyName = serviceName + "-token-" + project.ID
		}
		if err := a.Secrets.Set(ctx, keyName, token); err != nil {
			return fmt.Errorf("%s", i18n.Tf("cmd.service.keychain_error", err))
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n",
			common.SuccessStyle.Render(common.IconSuccess),
			i18n.Tf("cmd.service.token_stored", keyName))
	}

	// GitLab write mode
	var writeEnabled bool
	if serviceName == "gitlab" {
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), common.Bold.Render("  Droits requis pour le token GitLab :"))
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "  Mode lecture seule (par défaut) :")
		fmt.Fprintf(cmd.OutOrStdout(), "    %s read_api\n", common.SuccessStyle.Render(common.IconSuccess))
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "  Mode lecture + écriture :")
		fmt.Fprintf(cmd.OutOrStdout(), "    %s api (inclut read + write)\n", common.SuccessStyle.Render(common.IconSuccess))
		fmt.Fprintln(cmd.OutOrStdout(), "    Permet : créer MR, commenter, assigner, modifier labels/statuts")
		fmt.Fprintln(cmd.OutOrStdout())

		_ = huh.NewConfirm().
			Title("Activer le mode écriture (créer MR, commenter, assigner) ?").
			Description("Nécessite un token avec le scope 'api'").
			Value(&writeEnabled).
			Run()

		if writeEnabled {
			fmt.Fprintf(cmd.OutOrStdout(), "%s Mode écriture activé\n",
				common.SuccessStyle.Render(common.IconSuccess))
		}
	}

	// Persist configuration
	if project != nil {
		tokenKey := serviceName + "-token-" + project.ID
		if token == "" {
			tokenKey = "" // inherit hub token
		}
		svc := domain.ProjectMCPService{
			Name:     serviceName,
			Enabled:  boolPtr(true),
			TokenKey: tokenKey,
		}
		if serviceName == "gitlab" {
			svc.WriteEnabled = &writeEnabled
		}
		upsertProjectMCPService(project, svc)

		if err := a.Projects.Update(ctx, project); err != nil {
			return fmt.Errorf("updating project MCP config: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n",
			common.SuccessStyle.Render(common.IconSuccess),
			i18n.Tf("cmd.service.project_configured", common.Bold.Render(serviceName), project.Name))
	} else {
		v := configViper()
		v.Set("mcp."+serviceName+".enabled", true)
		v.Set("mcp."+serviceName+".token_key", serviceName+"-token")
		if serviceName == "gitlab" {
			v.Set("mcp.gitlab.write_enabled", writeEnabled)
		}

		cfgPath := config.ConfigPath()
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
			return fmt.Errorf("creating config dir: %w", err)
		}
		if err := v.WriteConfigAs(cfgPath); err != nil {
			return fmt.Errorf("writing config: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n",
			common.SuccessStyle.Render(common.IconSuccess),
			i18n.Tf("cmd.service.enabled", common.Bold.Render(serviceName)))
	}
	return nil
}

// --- oh mcp status ---

func mcpStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: i18n.T("cmd.mcp.status.short"),
		Long:  i18n.T("cmd.mcp.status.long"),
		RunE:  runMCPStatus,
	}
	cmd.Flags().StringP("project", "p", "", i18n.T("cmd.mcp.flags.project"))
	_ = cmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
	return cmd
}

func runMCPStatus(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	projectID, _ := cmd.Flags().GetString("project")
	var project *domain.Project
	if projectID != "" {
		var err error
		project, err = resolveProject(ctx, a, projectID)
		if err != nil {
			return err
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s Services MCP",
		common.Title.Render("oh mcp status"))
	if project != nil {
		fmt.Fprintf(cmd.OutOrStdout(), " — %s", project.Name)
	}
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout())

	type serviceInfo struct {
		name    string
		label   string
		enabled bool
		token   string
		envVar  string
	}

	services := []serviceInfo{
		{"figma", "Figma", a.Config.MCP.Figma.Enabled, a.Config.MCP.Figma.Token, "FIGMA_TOKEN"},
		{"gitlab", "GitLab", a.Config.MCP.Gitlab.Enabled, a.Config.MCP.Gitlab.Token, "GITLAB_TOKEN"},
		{"gslides", "Google Slides", a.Config.MCP.Gslides.Enabled, a.Config.MCP.Gslides.Token, "GOOGLE_ACCESS_TOKEN"},
		{"team", "Team", a.Config.Team.Enabled, "", ""},
	}

	// Build project override lookup
	projectOverrides := make(map[string]*domain.ProjectMCPService)
	if project != nil && project.MCPConfig != nil {
		for i := range project.MCPConfig.Services {
			s := &project.MCPConfig.Services[i]
			projectOverrides[s.Name] = s
		}
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, i18n.T("cmd.mcp.status.header"))

	for _, svc := range services {
		effectiveEnabled := svc.enabled
		source := i18n.T("cmd.mcp.status.source_hub")

		if ps, ok := projectOverrides[svc.name]; ok {
			source = i18n.T("cmd.mcp.status.source_project")
			if ps.Enabled != nil {
				effectiveEnabled = *ps.Enabled
			}
		}

		// Status display
		status := common.ErrorStyle.Render(i18n.T("cmd.service.status_disabled"))
		if effectiveEnabled {
			status = common.SuccessStyle.Render(i18n.T("cmd.service.status_enabled"))
		}

		// Token display
		tokenStatus := "—"
		switch {
		case svc.envVar != "" && os.Getenv(svc.envVar) != "":
			tokenStatus = fmt.Sprintf("env:%s", svc.envVar)
		case svc.token != "" && a.Secrets != nil:
			tokenKey := svc.token
			if ps, ok := projectOverrides[svc.name]; ok && ps.TokenKey != "" {
				tokenKey = ps.TokenKey
			}
			if t, _ := a.Secrets.Get(ctx, tokenKey); t != "" {
				tokenStatus = "keychain"
			} else {
				tokenStatus = common.WarningStyle.Render(i18n.T("cmd.service.token_missing"))
			}
		case svc.name == "team":
			tokenStatus = "—"
		}

		fmt.Fprintf(w, "  %-15s\t%s\t%s\t%s\n", svc.label, status, source, tokenStatus)
	}

	w.Flush()
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintf(cmd.OutOrStdout(), "  %s\n",
		i18n.Tf("cmd.service.setup_hint", common.Bold.Render("oh mcp setup")))
	return nil
}

// --- oh mcp serve ---

func mcpServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "serve <name>",
		Short:             i18n.T("cmd.mcp.serve.short"),
		Long:              i18n.T("cmd.mcp.serve.long"),
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeMCPServices,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Resolve token from keychain if --token-key is provided and env var not already set.
			tokenKey, _ := cmd.Flags().GetString("token-key")
			if tokenKey != "" {
				if err := injectTokenFromKeychain(name, tokenKey); err != nil {
					// Non-fatal: the server will report "token not set" if needed.
					fmt.Fprintf(os.Stderr, "warning: could not resolve token from keychain: %v\n", err)
				}
			}

			switch name {
			case "figma":
				return figma.Serve()
			case "gitlab":
				return gitlab.Serve()
			case "gslides":
				return gslides.Serve()
			case "team":
				return team.Serve()
			default:
				return fmt.Errorf("%s", i18n.Tf("cmd.mcp.serve.unknown", name))
			}
		},
	}
	cmd.Flags().String("token-key", "", "keychain key name to resolve the service token")
	return cmd
}

// mcpServiceEnvVar maps MCP service names to their expected token environment variable.
var mcpServiceEnvVar = map[string]string{
	"figma":   "FIGMA_TOKEN",
	"gitlab":  "GITLAB_TOKEN",
	"gslides": "GOOGLE_ACCESS_TOKEN",
}

// injectTokenFromKeychain reads a token from the keychain and sets the corresponding
// environment variable for the MCP service, but only if the env var is not already set.
func injectTokenFromKeychain(serviceName, tokenKey string) error {
	envVar, ok := mcpServiceEnvVar[serviceName]
	if !ok {
		return nil // no env var mapping (e.g., team) — nothing to inject
	}

	// Don't override an explicitly set env var
	if os.Getenv(envVar) != "" {
		return nil
	}

	secrets := resolveSecretStore()
	if secrets == nil {
		return fmt.Errorf("no secret store available")
	}

	token, err := secrets.Get(context.Background(), tokenKey)
	if err != nil {
		return fmt.Errorf("reading key %q: %w", tokenKey, err)
	}
	if token == "" {
		return fmt.Errorf("key %q not found in keychain", tokenKey)
	}

	return os.Setenv(envVar, token)
}

// --- oh mcp list ---

func mcpListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   i18n.T("cmd.mcp.list.short"),
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				type mcpServer struct {
					Name        string `json:"name"`
					Description string `json:"description"`
					Command     string `json:"command"`
				}
				servers := []mcpServer{
					{Name: "figma", Description: i18n.T("cmd.mcp.list.figma_desc"), Command: "oh mcp serve figma"},
					{Name: "gitlab", Description: i18n.T("cmd.mcp.list.gitlab_desc"), Command: "oh mcp serve gitlab"},
					{Name: "gslides", Description: i18n.T("cmd.mcp.list.gslides_desc"), Command: "oh mcp serve gslides"},
				}
				return json.NewEncoder(os.Stdout).Encode(servers)
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, i18n.T("cmd.mcp.list.header"))
			fmt.Fprintf(w, "%s\t%s\t%s\n",
				"figma", i18n.T("cmd.mcp.list.figma_desc"), common.Subtitle.Render("oh mcp serve figma"))
			fmt.Fprintf(w, "%s\t%s\t%s\n",
				"gitlab", i18n.T("cmd.mcp.list.gitlab_desc"), common.Subtitle.Render("oh mcp serve gitlab"))
			fmt.Fprintf(w, "%s\t%s\t%s\n",
				"gslides", i18n.T("cmd.mcp.list.gslides_desc"), common.Subtitle.Render("oh mcp serve gslides"))
			w.Flush()
			return nil
		},
	}

	cmd.Flags().Bool("json", false, i18n.T("cmd.mcp.list.flags.json"))
	return cmd
}

// --- Token helpers ---

// checkProjectToken checks if a token is available for a given service/project combination.
func checkProjectToken(ctx context.Context, a *app.App, serviceName string, project *domain.Project) bool {
	// Team doesn't need a token
	if serviceName == "team" {
		return true
	}

	// Check env variable
	switch serviceName {
	case "figma":
		if os.Getenv("FIGMA_TOKEN") != "" {
			return true
		}
	case "gitlab":
		if os.Getenv("GITLAB_TOKEN") != "" {
			return true
		}
	case "gslides":
		if os.Getenv("GOOGLE_ACCESS_TOKEN") != "" {
			return true
		}
	}

	if a.Secrets == nil {
		return false
	}

	// Check project-scoped token
	if project != nil {
		projectKey := serviceName + "-token-" + project.ID
		if t, _ := a.Secrets.Get(ctx, projectKey); t != "" {
			return true
		}
	}

	// Check hub-level token
	hubKey := serviceName + "-token"
	if t, _ := a.Secrets.Get(ctx, hubKey); t != "" {
		return true
	}

	return false
}
