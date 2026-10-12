package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// The Bedrock API key of the user's shell (AWS_BEARER_TOKEN_BEDROCK) is
// never passed to the tool: oh only uses stored keys. It can be imported
// into the keychain, under the key a launch uses, by `oh provider setup
// bedrock --from-env`, the « use existing » step of the setup wizard, or
// the launch prompt.

// bedrockImportKey is the keychain key the shell key is stored under: the
// project key (or its explicit key) for a project chosen with -p, else the
// stored key a launch from here uses today, else the hub key.
func bedrockImportKey(ctx context.Context, a *app.App, project *domain.Project, explicit bool) (key, scope string) {
	if project != nil && explicit {
		if project.ProviderConfig != nil && project.ProviderConfig.TokenKey != "" {
			return project.ProviderConfig.TokenKey, "project"
		}
		return provider.KeychainKey(provider.Bedrock, project.ID), "project"
	}
	if project != nil && a.Secrets != nil {
		_, tokenKey, _ := projectProvider(a, project, string(provider.Bedrock))
		teamID := ""
		if tc := config.ResolveTeamForProject(a.Config, project); tc.Enabled {
			teamID = tc.TeamID
		}
		cred, err := provider.ResolveCredentialSource(ctx, a.Secrets, provider.Bedrock, project.ID, teamID, tokenKey, &provider.Config{})
		if err == nil && cred.Source.KeychainKey != "" {
			return cred.Source.KeychainKey, cred.Source.Scope
		}
	}
	return provider.KeychainKey(provider.Bedrock, ""), "hub"
}

// importBedrockEnvToken stores the shell Bedrock API key in the keychain
// (bedrockImportKey) and sets auth_mode = bearer where the launch reads it.
// It returns the key used.
func importBedrockEnvToken(ctx context.Context, a *app.App, project *domain.Project, explicit bool) (string, error) {
	tok := strings.TrimSpace(provider.EnvBearerToken())
	if tok == "" {
		return "", errors.New(i18n.T("cmd.provider.from_env.none"))
	}
	if a.Secrets == nil {
		return "", errors.New(i18n.T("cmd.provider.from_env.no_store"))
	}
	key, scope := bedrockImportKey(ctx, a, project, explicit)
	if err := a.Secrets.Set(ctx, key, tok); err != nil {
		return "", fmt.Errorf("storing bedrock token: %w", err)
	}
	projectMode := project != nil && project.ProviderConfig != nil && project.ProviderConfig.AuthMode != "" && project.ProviderConfig.AuthMode != provider.AuthBearer
	if project != nil && (scope == "project" || projectMode) {
		if project.ProviderConfig == nil {
			project.ProviderConfig = &domain.ProjectProviderConfig{}
		}
		if explicit && project.ProviderConfig.TokenKey == "" {
			project.ProviderConfig.TokenKey = key
		}
		if err := setBedrockAuthMode(ctx, a, project, provider.AuthBearer); err != nil {
			return key, err
		}
	}
	if scope != "project" {
		if err := setBedrockAuthMode(ctx, a, nil, provider.AuthBearer); err != nil {
			return key, err
		}
	}
	return key, nil
}

// setBedrockAuthMode records the Bedrock auth mode of project (nil = hub).
func setBedrockAuthMode(ctx context.Context, a *app.App, project *domain.Project, mode string) error {
	if project != nil {
		if project.ProviderConfig == nil {
			project.ProviderConfig = &domain.ProjectProviderConfig{}
		}
		project.ProviderConfig.AuthMode = mode
		if a.Projects == nil {
			return errors.New("no project store")
		}
		if err := a.Projects.Update(ctx, project); err != nil {
			return fmt.Errorf("updating project provider config: %w", err)
		}
		return nil
	}
	if err := config.Update(func(c *config.Config) error {
		c.Provider.Bedrock.AuthMode = mode
		return nil
	}); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.config.save_failed"), err)
	}
	if a.Config != nil {
		a.Config.Provider.Bedrock.AuthMode = mode
	}
	return nil
}

// bedrockSetupHint is the command setting up (fromEnv: importing the shell
// key for) the Bedrock key of a launch of project.
func bedrockSetupHint(project *domain.Project, fromEnv bool) string {
	h := "oh provider setup bedrock"
	if fromEnv {
		h += " --from-env"
	}
	if project != nil {
		name := project.Name
		if name == "" {
			name = project.ID
		}
		if strings.ContainsAny(name, " \t'\"") {
			name = "'" + strings.ReplaceAll(name, "'", `'\''`) + "'"
		}
		h += " -p " + name
	}
	return h
}

// offerBedrockEnvImport offers, on a terminal, to import the shell Bedrock
// API key when a launch was refused for want of a stored key. It reports
// whether the key was imported (the launch can be prepared again).
func offerBedrockEnvImport(ctx context.Context, a *app.App, project *domain.Project, providerFlag string, launchErr error, w io.Writer) bool {
	var pe *ProviderCredentialError
	if !errors.As(launchErr, &pe) || project == nil || !stdinIsTerminal() {
		return false
	}
	if !resolveCredentialReport(ctx, a, project, providerFlag).envBearerUnused {
		return false
	}
	ok, err := cliConfirm(w, i18n.Tf("cmd.provider.from_env.confirm", project.Name))
	if err != nil || !ok {
		return false
	}
	key, err := importBedrockEnvToken(ctx, a, project, true)
	if err != nil {
		fmt.Fprintf(w, "%s %v\n", theme.ErrorStyle.Render(theme.IconError), err)
		return false
	}
	fmt.Fprintf(w, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.provider.from_env.done", key))
	return true
}
