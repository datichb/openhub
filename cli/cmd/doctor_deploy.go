package cmd

import (
	"context"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// deployLeftoversCheck reports the files left by the former per-project
// deployment in the registered projects (P3-T28): fixed by
// `oh migrate deploy-cleanup`.
func deployLeftoversCheck(ctx context.Context) views.DoctorCheck {
	name := i18n.T("cmd.migrate.cleanup.doctor_name")
	a := TryApp()
	if a == nil || a.Projects == nil {
		return views.DoctorCheck{Name: name, OK: true, Detail: i18n.T("cmd.migrate.cleanup.none")}
	}
	list, err := scanDeployLeftovers(ctx, a, "")
	if err != nil {
		return views.DoctorCheck{Name: name, OK: false, Detail: err.Error()}
	}
	if len(list) == 0 {
		return views.DoctorCheck{Name: name, OK: true, Detail: i18n.T("cmd.migrate.cleanup.none")}
	}
	names := make([]string, len(list))
	for i, pc := range list {
		names[i] = pc.Name
	}
	return views.DoctorCheck{Name: name, OK: false,
		Detail: i18n.Tf("cmd.migrate.cleanup.doctor_found", len(list), strings.Join(names, ", "))}
}
