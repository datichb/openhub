package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/cliout"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	mcpgitlab "github.com/datichb/openhub/cli/internal/mcp/gitlab"
	"github.com/datichb/openhub/cli/internal/mcp/mcpregistry"
	mcpwiki "github.com/datichb/openhub/cli/internal/mcp/wiki"
	mcpworkflow "github.com/datichb/openhub/cli/internal/mcp/workflow"
	"github.com/datichb/openhub/cli/internal/mcpresolve"
	teamsvc "github.com/datichb/openhub/cli/internal/services/team"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/tracker"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// validMCPServices returns all registered MCP service names (built-in + custom).
func validMCPServiceNames() []string {
	return mcpregistry.NewDefaultRegistry().Names()
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: i18n.T("cmd.mcp.short"),
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.AddCommand(withInitLevel(mcpServeCmd(), levelLight))
	mcpCmd.AddCommand(mcpListCmd())
	mcpCmd.AddCommand(mcpEnableCmd())
	mcpCmd.AddCommand(mcpDisableCmd())
	mcpCmd.AddCommand(mcpResetCmd())
	mcpCmd.AddCommand(mcpSetupCmd())
	mcpCmd.AddCommand(mcpStatusCmd())
}

// --- Helpers ---

func isValidMCPService(name string) bool {
	return mcpregistry.NewDefaultRegistry().Get(name) != nil
}

func completeMCPServices(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return validMCPServiceNames(), cobra.ShellCompDirectiveNoFileComp
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

// projectIDOf returns the ID of a project, or "" when there is none (hub scope).
func projectIDOf(p *domain.Project) string {
	if p == nil {
		return ""
	}
	return p.ID
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
		return unknownMCPServiceError(serviceName, validMCPServiceNames())
	}

	projectID, _ := cmd.Flags().GetString("project")

	if projectID == "" {
		// Hub-level enable
		if err := config.Update(func(c *config.Config) error {
			if s := c.MCPServer(serviceName); s != nil {
				s.Enabled = true
			}
			return nil
		}); err != nil {
			return fmt.Errorf("%s: %w", i18n.T("cmd.config.save_failed"), err)
		}
		cliout.Success(cmd.OutOrStdout(), i18n.Tf("cmd.mcp.enable.success", theme.Bold.Render(serviceName)))
		return nil
	}

	// Project-level enable
	a := MustApp()
	ctx := cmd.Context()
	project, err := resolveProject(ctx, a, projectID, projectWrite)
	if err != nil {
		return err
	}

	// Check if there's a token available (hub or project-scoped)
	hasToken := checkProjectToken(ctx, a, serviceName, project)
	if !hasToken && serviceName != "team" {
		// Prompt user: inherit from hub or configure?
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n\n",
			cliout.WarnIcon(),
			i18n.Tf("cmd.mcp.enable.no_token_prompt", serviceName))

		var choice string
		form := theme.NewForm(
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
		return fmt.Errorf(i18n.T("cmd.mcp.updating_project"), err)
	}

	cliout.Success(cmd.OutOrStdout(), i18n.Tf("cmd.mcp.enable.success_project", theme.Bold.Render(serviceName), project.Name))
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
		return unknownMCPServiceError(serviceName, validMCPServiceNames())
	}

	projectID, _ := cmd.Flags().GetString("project")

	if projectID == "" {
		// Hub-level disable
		if err := config.Update(func(c *config.Config) error {
			if s := c.MCPServer(serviceName); s != nil {
				s.Enabled = false
			}
			return nil
		}); err != nil {
			return fmt.Errorf("%s: %w", i18n.T("cmd.config.save_failed"), err)
		}
		cliout.Success(cmd.OutOrStdout(), i18n.Tf("cmd.mcp.disable.success", theme.Bold.Render(serviceName)))
		return nil
	}

	// Project-level disable
	a := MustApp()
	ctx := cmd.Context()
	project, err := resolveProject(ctx, a, projectID, projectWrite)
	if err != nil {
		return err
	}

	svc := domain.ProjectMCPService{
		Name:    serviceName,
		Enabled: boolPtr(false),
	}
	upsertProjectMCPService(project, svc)

	if err := a.Projects.Update(ctx, project); err != nil {
		return fmt.Errorf(i18n.T("cmd.mcp.updating_project"), err)
	}

	cliout.Success(cmd.OutOrStdout(), i18n.Tf("cmd.mcp.disable.success_project", theme.Bold.Render(serviceName), project.Name))
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
		return unknownMCPServiceError(serviceName, validMCPServiceNames())
	}

	projectID, _ := cmd.Flags().GetString("project")
	if projectID == "" {
		return fmt.Errorf("%s", i18n.T("cmd.mcp.reset.requires_project"))
	}

	a := MustApp()
	ctx := cmd.Context()
	project, err := resolveProject(ctx, a, projectID, projectWrite)
	if err != nil {
		return err
	}

	removeProjectMCPService(project, serviceName)

	if err := a.Projects.Update(ctx, project); err != nil {
		return fmt.Errorf(i18n.T("cmd.mcp.updating_project"), err)
	}

	cliout.Success(cmd.OutOrStdout(), i18n.Tf("cmd.mcp.reset.success", theme.Bold.Render(serviceName), project.Name))
	return nil
}

// --- oh mcp setup ---

func mcpSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:       "setup [service]",
		Short:     i18n.T("cmd.mcp.setup.short"),
		Long:      i18n.T("cmd.mcp.setup.long"),
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: mcpSetupServices,
		RunE:      runMCPSetup,
	}
	cmd.Flags().StringP("project", "p", "", i18n.T("cmd.mcp.flags.project"))
	_ = cmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
	return cmd
}

// mcpSetupServices are the services oh mcp setup configures (token in the
// keychain, hub or project settings): every service oh knows
// (mcpresolve.Services), the same list as oh mcp status and the screens.
var mcpSetupServices = mcpServiceNames()

// mcpServiceNames lists the names of the known services, in the order the
// screens show them.
func mcpServiceNames() []string {
	var names []string
	for _, s := range mcpresolve.Services() {
		names = append(names, s.Name)
	}
	return names
}

// unknownMCPServiceError is the error of a service name no command knows: it
// names the valid ones.
func unknownMCPServiceError(name string, valid []string) error {
	return errors.New(i18n.Tf("v55m.mcp.unknown_service", name, strings.Join(valid, ", ")))
}

// mcpSetupService returns the service named on the command line ("" =
// ask; the argument used to be ignored).
func mcpSetupService(args []string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	name := strings.ToLower(strings.TrimSpace(args[0]))
	if !slices.Contains(mcpSetupServices, name) {
		return "", errors.New(i18n.Tf("cmd.mcp.setup.unknown_service", args[0], strings.Join(mcpSetupServices, ", ")))
	}
	return name, nil
}

func runMCPSetup(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()
	if !stdinIsTerminal() {
		return errors.New(i18n.T("cmd.errors.no_terminal"))
	}

	projectID, _ := cmd.Flags().GetString("project")
	var project *domain.Project
	if projectID != "" {
		var err error
		project, err = resolveProject(ctx, a, projectID, projectWrite)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s (%s)\n\n",
			theme.Title.Render("oh mcp setup"),
			i18n.T("cmd.service.project_scope"), project.Name)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n\n",
			theme.Title.Render("oh mcp setup"),
			i18n.T("cmd.mcp.setup.short"))
	}

	serviceName, err := mcpSetupService(args)
	if err != nil {
		return err
	}
	if serviceName == "" {
		opts := make([]huh.Option[string], 0, len(mcpSetupServices))
		for _, name := range mcpSetupServices {
			opts = append(opts, huh.NewOption(i18n.T("v55m.mcp.setup.option_"+name), name))
		}
		form := theme.NewForm(huh.NewGroup(huh.NewSelect[string]().
			Title(i18n.T("cmd.service.select")).Options(opts...).Value(&serviceName)))
		if err := form.Run(); err != nil {
			return err
		}
	}
	return runMCPSetupForService(cmd, serviceName, project)
}

// runMCPSetupForService configures a specific service's token and options.
// If project is nil, configuration is hub-level.
func runMCPSetupForService(cmd *cobra.Command, serviceName string, project *domain.Project) error {
	a := MustApp()
	ctx := cmd.Context()

	// Shared state between wizard steps
	var token string
	var writeEnabled bool
	var emptyTokenWarning bool // said once the wizard has left the terminal

	envHint := mcpServiceEnvVar[serviceName]

	steps := []views.WizardStep{
		{
			Label: i18n.Tf("cmd.service.token_prompt", serviceName),
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddPasswordField(
					i18n.Tf("cmd.service.token_prompt", serviceName),
					"", 0, '*',
					func(text string) { token = text },
				)
				form.AddButton(i18n.T("cmd.button.next"), func() { onDone() })
				return form
			},
			OnDone: func() error {
				// Store token
				if token == "" && envHint != "" {
					emptyTokenWarning = true
				} else if token != "" && a.Secrets != nil {
					if _, err := config.StoreMCPToken(ctx, a.Secrets, serviceName, projectIDOf(project), token); err != nil {
						return fmt.Errorf("%s", i18n.Tf("cmd.service.keychain_error", err))
					}
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				status := "stored"
				if token == "" {
					status = "skipped (env: " + envHint + ")"
				}
				return []views.InfoField{
					{Label: "Token", Value: status},
					{Label: i18n.T("cmd.mcp.env_hint"), Value: envHint},
				}
			},
			SkipIf: func() bool { return envHint == "" },
		},
		{
			Label: i18n.Tf("cmd.mcp.setup.write_label", serviceName),
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddCheckbox(
					i18n.Tf("cmd.mcp.setup.write_checkbox", serviceName),
					false,
					func(checked bool) { writeEnabled = checked },
				)
				form.AddButton(i18n.T("cmd.button.next"), func() { onDone() })
				return form
			},
			OnDone: func() error {
				return nil
			},
			InfoFields: func() []views.InfoField {
				mode := i18n.T("cmd.mcp.setup.mode_read_only")
				if writeEnabled {
					mode = i18n.T("cmd.mcp.setup.mode_read_write")
				}
				return []views.InfoField{
					{Label: "Mode", Value: mode},
				}
			},
			SkipIf: func() bool { return mcpWriteEnv[serviceName] == "" },
		},
		{
			Label:      i18n.T("cmd.mcp.setup.persist_label"),
			Processing: i18n.T("cmd.mcp.setup.persist_processing"),
			OnDone: func() error {
				if project != nil {
					tokenKey := config.MCPProjectTokenKey(serviceName, project.ID)
					if token == "" {
						tokenKey = "" // inherit hub token
					}
					svc := domain.ProjectMCPService{
						Name:     serviceName,
						Enabled:  boolPtr(true),
						TokenKey: tokenKey,
					}
					if mcpWriteEnv[serviceName] != "" {
						svc.WriteEnabled = &writeEnabled
					}
					upsertProjectMCPService(project, svc)

					if err := a.Projects.Update(ctx, project); err != nil {
						return fmt.Errorf(i18n.T("cmd.mcp.updating_project_mcp_config"), err)
					}
				} else {
					if err := config.Update(func(c *config.Config) error {
						if s := c.MCPServer(serviceName); s != nil {
							s.Enabled = true
							s.Token = config.MCPTokenKey(serviceName)
							if mcpWriteEnv[serviceName] != "" {
								s.WriteEnabled = writeEnabled
							}
						}
						return nil
					}); err != nil {
						return fmt.Errorf("%s: %w", i18n.T("cmd.config.save_failed"), err)
					}
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				scope := "hub"
				if project != nil {
					scope = "project: " + project.Name
				}
				return []views.InfoField{
					{Label: "Service", Value: serviceName},
					{Label: "Scope", Value: scope},
				}
			},
		},
	}

	wizResult := runCLIWizard(a, cliWizard{Command: "mcp setup", Steps: steps})

	if emptyTokenWarning {
		cliout.Warn(cmd.OutOrStdout(), i18n.Tf("cmd.service.token_empty_warning", envHint))
	}

	if wizResult.Aborted {
		return nil
	}
	if wizResult.Err != nil {
		return wizResult.Err
	}

	// Print success message
	if project != nil {
		cliout.Success(cmd.OutOrStdout(), i18n.Tf("cmd.service.project_configured", theme.Bold.Render(serviceName), project.Name))
	} else {
		cliout.Success(cmd.OutOrStdout(), i18n.Tf("cmd.service.enabled", theme.Bold.Render(serviceName)))
	}
	return nil
}

// --- oh mcp status ---

func mcpStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "status [service]",
		Short:             i18n.T("cmd.mcp.status.short"),
		Long:              i18n.T("cmd.mcp.status.long"),
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeMCPStatusServices,
		RunE:              runMCPStatus,
	}
	cmd.Flags().StringP("project", "p", "", i18n.T("cmd.mcp.flags.project"))
	_ = cmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
	return cmd
}

// mcpStatusServices are the rows oh mcp status can show: the known services
// and the team server.
func mcpStatusServices() []string {
	return append(mcpServiceNames(), "team")
}

func completeMCPStatusServices(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return mcpStatusServices(), cobra.ShellCompDirectiveNoFileComp
}

// mcpStatusService returns the service named on the command line ("" = all of
// them). A name no row has is an error that lists the valid ones.
func mcpStatusService(args []string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	name := strings.ToLower(strings.TrimSpace(args[0]))
	if !slices.Contains(mcpStatusServices(), name) {
		valid := append(mcpresolve.Names(), "team")
		return "", unknownMCPServiceError(args[0], valid)
	}
	return name, nil
}

func runMCPStatus(cmd *cobra.Command, args []string) error {
	only, err := mcpStatusService(args)
	if err != nil {
		return err
	}

	a := MustApp()
	ctx := cmd.Context()

	projectID, _ := cmd.Flags().GetString("project")
	var project *domain.Project
	if projectID != "" {
		var err error
		project, err = resolveProject(ctx, a, projectID, projectRead)
		if err != nil {
			return err
		}
	}

	fmt.Fprint(cmd.OutOrStdout(), i18n.Tf("cmd.mcp.mcp_services", theme.Title.Render("oh mcp status")))
	if project != nil {
		fmt.Fprintf(cmd.OutOrStdout(), " — %s", project.Name)
	}
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout())

	tc := resolvedTeamConfigCtx(ctx, a, project)
	var override *domain.ProjectMCPConfig
	if project != nil {
		override = project.MCPConfig
	}
	shared := sharedMCPForTeam(ctx, tc)
	states := resolveMCPStatesWith(a.Config, override, tc, shared)
	if only != "" {
		states = slices.DeleteFunc(states, func(st mcpState) bool { return st.Service.Name != only })
	}

	w := cliout.Table(cmd.OutOrStdout())
	fmt.Fprintln(w, i18n.T("cmd.mcp.status.header"))

	for _, st := range states {
		status := theme.ErrorStyle.Render(i18n.T("cmd.service.status_disabled"))
		if st.Eff.Enabled {
			status = theme.SuccessStyle.Render(i18n.T("cmd.service.status_enabled"))
		}
		fmt.Fprintf(w, "  %-15s\t%s\t%s\t%s\n", st.Service.Label, status, mcpSourceText(st.Source), mcpTokenText(ctx, a, st))
	}
	if only == "" || only == "team" {
		status := theme.ErrorStyle.Render(i18n.T("cmd.service.status_disabled"))
		if teamEnabledFor(ctx, a, project) {
			status = theme.SuccessStyle.Render(i18n.T("cmd.service.status_enabled"))
		}
		fmt.Fprintf(w, "  %-15s\t%s\t%s\t%s\n", "Team", status, mcpSourceText(mcpSourceHub), "—")
	}

	w.Flush()
	printEnforcedURLNotices(cmd.OutOrStdout(), enforcedURLNotices(a.Config, shared))
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintf(cmd.OutOrStdout(), "  %s\n",
		i18n.Tf("cmd.service.setup_hint", theme.Bold.Render("oh mcp setup")))
	return nil
}

// mcpSourceText names the level that decided whether a service is enabled.
func mcpSourceText(src mcpSource) string {
	switch src {
	case mcpSourceProject:
		return i18n.T("cmd.mcp.status.source_project")
	case mcpSourceTeam:
		return i18n.T("v55m.mcp.status.source_team")
	}
	return i18n.T("cmd.mcp.status.source_hub")
}

// mcpTokenText says where the token of a service comes from, never what it
// is: the environment variable, the keychain, a key whose value is missing,
// or nothing.
func mcpTokenText(ctx context.Context, a *app.App, st mcpState) string {
	switch {
	case st.Service.TokenEnv != "" && os.Getenv(st.Service.TokenEnv) != "":
		return "env:" + st.Service.TokenEnv
	case st.Eff.TokenKey != "" && a.Secrets != nil:
		if t, _ := config.ReadMCPToken(ctx, a.Secrets, st.Service.Name, st.Eff.TokenKey); t != "" {
			return "keychain"
		}
		return theme.WarningStyle.Render(i18n.T("cmd.service.token_missing"))
	}
	return "—"
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

			tokenKey, _ := cmd.Flags().GetString("token-key")
			if tokenKey != "" {
				if err := injectTokenFromKeychain(name, tokenKey); err != nil {
					fmt.Fprint(cmd.ErrOrStderr(), i18n.Tf("cmd.mcp.warning_could_not_resolve_token_from", err))
				}
			}

			// Injected in every session bundle built from a workflow; not a
			// user-configurable service (absent from list/setup/enable).
			if name == sessionspec.WorkflowMCPServer {
				return mcpworkflow.Serve()
			}
			if name == sessionspec.WikiMCPServer {
				return mcpwiki.Serve()
			}
			if name == "gitlab" {
				mcpgitlab.OnMRCreated = announceMRCreated
			}
			registry := mcpregistry.NewDefaultRegistry()
			server := registry.Get(name)
			if server == nil {
				return fmt.Errorf("%s", i18n.Tf("cmd.mcp.serve.unknown", name))
			}
			return server.Serve()
		},
	}
	cmd.Flags().String("token-key", "", "")
	return cmd
}

// mcpServiceEnvVar maps MCP service names to their expected token environment variable.
var mcpServiceEnvVar = func() map[string]string {
	m := map[string]string{}
	for _, s := range mcpresolve.Services() {
		m[s.Name] = s.TokenEnv
	}
	return m
}()

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
		return errors.New(i18n.T("cmd.mcp.no_secret_store_available"))
	}
	return injectTokenFromStore(secrets, serviceName, tokenKey)
}

// injectTokenFromStore sets the environment variable of a service from its
// keychain token (current key name, legacy name as a fallback).
func injectTokenFromStore(secrets domain.SecretStore, serviceName, tokenKey string) error {
	envVar := mcpServiceEnvVar[serviceName]
	token, err := config.ReadMCPToken(context.Background(), secrets, serviceName, tokenKey)
	if err != nil {
		return fmt.Errorf(i18n.T("cmd.mcp.reading_key"), tokenKey, err)
	}
	if token == "" {
		return fmt.Errorf("%s", i18n.Tf("cmd.mcp.setup.token_not_found", tokenKey, tokenKey))
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
			servers := mcpServerList()
			if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
				return cliout.JSONCompact(cmd.OutOrStdout(), servers)
			}
			w := cliout.Table(cmd.OutOrStdout())
			fmt.Fprintln(w, i18n.T("cmd.mcp.list.header"))
			for _, s := range servers {
				fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, s.Description, theme.Subtitle.Render(s.Command))
			}
			return w.Flush()
		},
	}

	cmd.Flags().Bool("json", false, i18n.T("cmd.mcp.list.flags.json"))
	return cmd
}

// mcpServer is a line of oh mcp list.
type mcpServer struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Command     string `json:"command"`
}

// mcpServerList lists every oh MCP server (built-in and custom;
// formerly figma, gitlab and gslides only), with a localized description
// for the built-in ones.
func mcpServerList() []mcpServer {
	var out []mcpServer
	for _, s := range mcpregistry.NewDefaultRegistry().All() {
		desc := s.Description()
		if key := "cmd.mcp.list." + s.Name() + "_desc"; i18n.T(key) != key {
			desc = i18n.T(key)
		}
		out = append(out, mcpServer{Name: s.Name(), Description: desc, Command: "oh mcp serve " + s.Name()})
	}
	return out
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
	case "gslides":
		if os.Getenv("GOOGLE_ACCESS_TOKEN") != "" {
			return true
		}
	}

	// GitLab: the unique cascade, restricted to what the MCP server is given
	// (the environment variable and the MCP tokens of the project and of the hub).
	if serviceName == "gitlab" {
		res := teamService(a).GitLabToken(ctx, "", "", teamsvc.WithProject(project), teamsvc.WithAnyServer())
		switch res.Source {
		case tracker.TokenSourceEnv, tracker.TokenSourceProject, tracker.TokenSourceMCP:
			return true
		}
		return false
	}

	if a.Secrets == nil {
		return false
	}

	// Check project-scoped token
	if project != nil {
		if t, _ := config.ReadMCPToken(ctx, a.Secrets, serviceName, config.MCPProjectTokenKey(serviceName, project.ID)); t != "" {
			return true
		}
	}

	// Check hub-level token
	if t, _ := config.ReadMCPToken(ctx, a.Secrets, serviceName, ""); t != "" {
		return true
	}

	return false
}
