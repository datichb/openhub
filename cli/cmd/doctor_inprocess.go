package cmd

import (
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
)

// inProcessDoctorChecks warns when the oh daemon runs inside the oh process
// (Windows option A): sessions only run while an oh window is open.
func inProcessDoctorChecks() []doctor.Result {
	if !daemon.InProcessMode() {
		return nil
	}
	return []doctor.Result{{Name: i18n.T("cmd.daemon.doctor.inprocess"), OK: true, Detail: i18n.T("cmd.daemon.doctor.inprocess_detail")}}
}
