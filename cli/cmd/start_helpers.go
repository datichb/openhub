package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/launcher"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// projectUse tells resolveProject what the command does with the project
// it was not given (N12, DC13).
type projectUse int

const (
	// projectRead: a project chosen outside of its directory is announced on
	// stderr.
	projectRead projectUse = iota
	// projectWrite: a project chosen outside of its directory is confirmed
	// (refused without a terminal: run from the project or use -p).
	projectWrite
)

// cliConfirm asks a yes/no question on the terminal
// (replaced in tests).
var cliConfirm = func(w io.Writer, msg string) (bool, error) {
	return launcher.NewCLIUI(w).Confirm(msg)
}

// projectPick lets the user choose among several projects (replaced in tests).
var projectPick = func(projects []domain.Project) (string, error) {
	var selectedID string
	options := make([]huh.Option[string], len(projects))
	for i, p := range projects {
		options[i] = huh.NewOption(fmt.Sprintf("%s (%s)", p.Name, p.Path), p.ID)
	}
	form := theme.NewForm(huh.NewGroup(huh.NewSelect[string]().
		Title(i18n.T("cmd.run.project.pick")).Options(options...).Value(&selectedID)))
	return selectedID, form.Run()
}

// resolveProject finds the project to use. Priority:
//  1. --project flag (explicit ID or name)
//  2. Current directory detection
//  3. Otherwise: a single project is announced (read) or confirmed (write);
//     several projects are chosen in a list. Without a terminal, a command
//     that writes is refused.
func resolveProject(ctx context.Context, a *app.App, projectID string, use projectUse) (*domain.Project, error) {
	if projectID != "" {
		p, err := a.Projects.GetByName(ctx, projectID)
		if err == nil {
			return p, nil
		}
		p, err = a.Projects.Get(ctx, projectID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, errors.New(i18n.Tf("cmd.run.project.not_found", projectID))
			}
			return nil, err
		}
		return p, nil
	}

	cwd, _ := os.Getwd()
	projects, err := a.Projects.List(ctx, domain.ProjectStatusActive)
	if err != nil {
		return nil, err
	}

	if len(projects) == 0 {
		return nil, fmt.Errorf("%s", i18n.T("cmd.project.no_projects"))
	}

	for i, p := range projects {
		absPath, _ := filepath.Abs(p.Path)
		if absPath == cwd || isSubPath(cwd, absPath) {
			return &projects[i], nil
		}
	}

	errOut := io.Writer(os.Stderr)
	if a.IO != nil && a.IO.ErrOut != nil {
		errOut = a.IO.ErrOut
	}
	if len(projects) == 1 {
		p := &projects[0]
		if use == projectRead {
			fmt.Fprintln(errOut, i18n.Tf("cmd.run.project.implicit", p.Name, p.Path))
			return p, nil
		}
		if !stdinIsTerminal() {
			return nil, errors.New(i18n.T("cmd.run.project.outside"))
		}
		ok, err := cliConfirm(errOut, i18n.Tf("cmd.run.project.confirm", p.Name, p.Path))
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New(i18n.T("cmd.run.project.declined"))
		}
		return p, nil
	}

	if !stdinIsTerminal() {
		return nil, errors.New(i18n.T("cmd.run.project.outside"))
	}
	selectedID, err := projectPick(projects)
	if err != nil {
		return nil, err
	}
	for i, p := range projects {
		if p.ID == selectedID {
			return &projects[i], nil
		}
	}
	return nil, fmt.Errorf("%s", i18n.T("cmd.quick.not_found"))
}

func displayOrDefault(detected, fallback string) string {
	if detected != "" {
		return detected
	}
	if fallback != "" {
		return fallback
	}
	return "—"
}

// hubProviderCfg returns the hub-level provider.Config for the given provider name.
// Used as the fallback layer in the provider config cascade.
func hubProviderCfg(a *app.App, prov string) provider.Config {
	switch provider.Name(prov) {
	case provider.Bedrock:
		return provider.Config{
			AWSProfile: a.Config.Provider.Bedrock.AWSProfile,
			AWSRegion:  a.Config.Provider.Bedrock.AWSRegion,
			AuthMode:   a.Config.Provider.Bedrock.AuthMode,
		}
	default:
		return provider.Config{}
	}
}
