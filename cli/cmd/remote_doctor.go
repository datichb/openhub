package cmd

import (
	"context"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/i18n"
	remotesvc "github.com/datichb/openhub/cli/internal/services/remote"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// remoteDoctorChecks reports each remote target (oh-runner project, pipeline,
// trigger, variables, runners). Nothing when no target is configured.
func remoteDoctorChecks() []views.DoctorCheck {
	a := TryApp()
	if a == nil || len(a.Config.Remote.Targets) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	svc := newRemoteService(a)
	var out []views.DoctorCheck
	for _, t := range a.Config.Remote.Targets {
		rep := svc.Check(ctx, t)
		var problems []string
		for _, s := range rep.Steps {
			if s.Status == remotesvc.StepFailed || s.Status == remotesvc.StepWarn {
				problems = append(problems, RemoteStepLine(s))
			}
		}
		detail := i18n.Tf("cmd.remote.doctor.ok", t.RunnerProjectPath())
		if len(problems) > 0 {
			detail = strings.Join(problems, " · ")
		}
		out = append(out, views.DoctorCheck{Name: i18n.Tf("cmd.remote.doctor.name", t.Name), OK: rep.OK(), Detail: detail})
	}
	return out
}
