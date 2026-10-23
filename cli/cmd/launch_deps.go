package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/launcher"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/services/launch"
	remotesvc "github.com/datichb/openhub/cli/internal/services/remote"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// launchDeps wires services/launch to the app: the stores and the
// configuration of a, and the parts of the launch that stay in cmd (the
// RunService, the bundle, the daemon, the fan-out driver…).
func launchDeps(a *app.App) launch.Deps {
	return launch.Deps{
		Config:       a.Config,
		Secrets:      a.Secrets,
		Preferences:  a.Preferences,
		RequireTool:  requireV2,
		Workflows:    newWorkflowService,
		InputSources: func(note func(string)) map[string]workflowsvc.InputSource { return inputSources(a, note) },
		Tracker: func(ctx context.Context, project *domain.Project) launch.TrackerInfo {
			return launchTracker(ctx, a, project)
		},
		Credentials: func(ctx context.Context, project *domain.Project, providerFlag string) error {
			return launchCredentials(ctx, a, project, providerFlag)
		},
		NewRunService: func(ctx context.Context, planOnly bool) (*runsvc.Service, error) {
			return newRunServiceFor(ctx, a, planOnly)
		},
		TicketClaims: func(project *domain.Project) func(ctx context.Context, projectID string, tickets []string) []string {
			return startTicketClaims(a, project)
		},
		BuildBundle: func(project *domain.Project, res *workflowsvc.Resolution, prov string) (*bundle.Bundle, []string, error) {
			return buildWorkflowBundle(a, project, res, prov)
		},
		Request: func(project *domain.Project, providerFlag string) runsvc.StartRequest {
			return v5Request(a, project, providerFlag)
		},
		Daemon: func(ctx context.Context) (*daemon.Client, error) {
			c, _, err := ensureDaemon(ctx)
			return c, err
		},
		RemoteSender: func() *remotesvc.Service { return newRemoteSendService(a) },
		TeamRepo: func(ctx context.Context, project *domain.Project) (*teamstate.Repo, error) {
			return ensureTeamRepoForProject(ctx, a, project)
		},
		SpawnFanoutDriver: func(sessionID string) (string, error) { return spawnFanoutDriver(sessionID) },
		IsolationWarning:  isolationWarningText,
	}
}

// launchProgress renders the events of a preparation: the lines of the image
// build go to buildOut (nil: dropped), the notes on the way the launch is
// understood (alias, computed inputs) and the findings of an invalid
// workflow go to noticeOut.
func launchProgress(buildOut, noticeOut io.Writer) func(launch.Event) {
	return func(e launch.Event) {
		switch e.Kind {
		case launch.EventBuild:
			if buildOut != nil {
				fmt.Fprintln(buildOut, theme.Subtitle.Render("  "+e.Text))
			}
		case launch.EventNotice:
			fmt.Fprintln(noticeOut, "  "+e.Text)
		case launch.EventDiagnostics:
			printDiagnostics(noticeOut, e.Diagnostics)
		}
	}
}

// uiProgress renders the events of a start as notifications of ui.
func uiProgress(ui launcher.LaunchUI) func(launch.Event) {
	return func(e launch.Event) {
		level := launcher.LevelInfo
		switch e.Kind {
		case launch.EventSuccess:
			level = launcher.LevelSuccess
		case launch.EventWarning:
			level = launcher.LevelWarning
		}
		ui.Notify(e.Text, level)
	}
}

// buildLines keeps the lines of the image build of a progress renderer: the
// preparation of a launch run on behalf of another one tells nothing else.
func buildLines(f func(launch.Event)) func(launch.Event) {
	if f == nil {
		return nil
	}
	return func(e launch.Event) {
		if e.Kind == launch.EventBuild {
			f(e)
		}
	}
}
