package cmd

import (
	"context"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
)

// deployLeftoversCheck reports the files left by the former per-project
// deployment in the registered projects (P3-T28): fixed by
// `oh migrate deploy-cleanup`.
func deployLeftoversCheck(ctx context.Context) doctor.Result {
	name := i18n.T("cmd.migrate.cleanup.doctor_name")
	a := TryApp()
	if a == nil || a.Projects == nil {
		return doctor.Result{ID: doctorIDDeployCleanup, Name: name, OK: true, Detail: i18n.T("cmd.migrate.cleanup.none")}
	}
	list, err := scanDeployLeftovers(ctx, a, "")
	if err != nil {
		return doctor.Result{ID: doctorIDDeployCleanup, Name: name, OK: false, Detail: err.Error()}
	}
	if len(list) == 0 {
		return doctor.Result{ID: doctorIDDeployCleanup, Name: name, OK: true, Detail: i18n.T("cmd.migrate.cleanup.none")}
	}
	names := make([]string, len(list))
	for i, pc := range list {
		names[i] = pc.Name
	}
	return doctor.Result{ID: doctorIDDeployCleanup, Name: name, OK: false,
		Detail: i18n.Tf("cmd.migrate.cleanup.doctor_found", len(list), strings.Join(names, ", "))}
}
