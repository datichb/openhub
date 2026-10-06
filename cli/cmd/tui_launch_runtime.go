package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	"github.com/datichb/openhub/cli/internal/runtime/container"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// TUI wiring of the runtime option of the launch form (P4-T11): engine
// state and image of the project (cached, or build to expect).

// tuiRuntimeStatus returns the status line function of a project.
func tuiRuntimeStatus(a *app.App, project *domain.Project) views.RuntimeStatusFunc {
	return func(ctx context.Context, c views.LaunchChoices) string {
		kind := sessionspec.RuntimeKind(c.Runtime)
		if kind == "" || kind == sessionspec.RuntimeLocal || !v5Available(ctx) {
			return ""
		}
		svc, err := newRunService(ctx, a)
		if err != nil {
			return ""
		}
		av, err := svc.RuntimeAvailability(ctx, kind)
		if err != nil || !av.OK {
			return "" // the option label already shows the reason
		}
		req := v5Request(a, project, "")
		req.Runtime, req.Location = kind, project.Path
		est, ok, err := svc.PrepareEstimate(ctx, req)
		return runtimeStatusText(av, est, ok, err)
	}
}

// runtimeStatusText renders the status line: engine, then the image.
func runtimeStatusText(av ohruntime.Availability, est ohruntime.PrepareEstimate, ok bool, err error) string {
	engine := strings.TrimSpace(av.Engine + " " + av.Version)
	for _, d := range av.Details {
		engine += " · " + d
	}
	parts := []string{i18n.Tf("tui.launch.runtime.engine", engine)}
	switch {
	case err != nil:
		parts = append(parts, i18n.Tf("tui.launch.runtime.estimate_error", err.Error()))
	case !ok:
	case est.Ready:
		parts = append(parts, i18n.Tf("tui.launch.runtime.cached", est.Image))
	default:
		key := "tui.launch.runtime.build"
		if len(est.Steps) == 1 && est.Steps[0] == container.StepLayer {
			key = "tui.launch.runtime.layer"
		}
		if est.Duration > 0 {
			parts = append(parts, i18n.Tf(key+"_eta", approxDuration(est.Duration)))
		} else {
			parts = append(parts, i18n.T(key+"_first"))
		}
	}
	return strings.Join(parts, " · ")
}

// approxDuration is « ~45 s », « ~3 min ».
func approxDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("~%d s", max(int(d.Round(5*time.Second).Seconds()), 5))
	}
	return fmt.Sprintf("~%d min", int(d.Round(time.Minute).Minutes()))
}
