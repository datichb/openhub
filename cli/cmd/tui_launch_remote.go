package cmd

import (
	"context"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// remoteLaunchRuntime is the ☁ option of the launch form (P5-T18): available
// when the project has a remote target with a trigger token on the machine.
// The workflow checks (checkpoints forbidden remotely in the chosen mode,
// pushed branch) are made when launching.
func remoteLaunchRuntime(ctx context.Context, a *app.App, project *domain.Project, lr views.LaunchRuntime) views.LaunchRuntime {
	if project == nil {
		lr.Reason = i18n.T("tui.remote.launch.no_target")
		return lr
	}
	t, err := remoteTargetFor(a, project)
	if err != nil {
		lr.Reason = i18n.T("tui.remote.launch.no_target")
		return lr
	}
	if a.Secrets == nil {
		lr.Reason = i18n.T("tui.remote.launch.no_trigger")
		return lr
	}
	if trig, _ := a.Secrets.Get(ctx, t.TriggerKeyOrDefault()); trig == "" {
		lr.Reason = i18n.T("tui.remote.launch.no_trigger")
		return lr
	}
	lr.Available = true
	lr.Label += " · " + t.Name
	return lr
}
