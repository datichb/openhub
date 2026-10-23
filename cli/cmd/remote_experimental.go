package cmd

import (
	"fmt"
	"io"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
	"github.com/datichb/openhub/cli/internal/services/launch"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// The remote runtime is experimental in 5.0 (DC22): it has never been
// validated on a real runner, so it stays off unless `[remote] experimental
// = true` is set in hub.toml. Every entry point of the remote mode on the
// machine (setup, launch, launch form option, fetch, replay) requires it;
// the job side (`oh runner`) does not read hub.toml.

// remoteExperimental reports whether the remote mode is enabled.
func remoteExperimental(a *app.App) bool {
	return a != nil && launch.RemoteExperimental(a.Config)
}

// errRemoteDisabled is the translated refusal of a remote entry point.
func errRemoteDisabled() error { return launch.ErrRemoteDisabled() }

// requireRemoteExperimental refuses when the remote mode is not enabled.
func requireRemoteExperimental(a *app.App) error {
	if !remoteExperimental(a) {
		return errRemoteDisabled()
	}
	return nil
}

// printRemoteExperimentalWarning prints the permanent warning of the remote
// mode.
func printRemoteExperimentalWarning(w io.Writer) {
	fmt.Fprintf(w, "%s %s\n", theme.WarningStyle.Render(theme.IconWarning), i18n.T("cmd.remote.experimental.warning"))
}

// remoteRecapWarnings is the permanent warning of a remote launch in the
// recap (oh run --recap, TUI launch form).
func remoteRecapWarnings(p *launch.Prepared) []string {
	if p == nil || p.Remote == nil {
		return nil
	}
	return []string{i18n.T("cmd.remote.experimental.warning")}
}

// remoteExperimentalDoctorCheck reports the state of the remote mode: nothing
// when it is off and no target is configured, else a warning.
func remoteExperimentalDoctorCheck(a *app.App) []doctor.Result {
	if a == nil || a.Config == nil {
		return nil
	}
	name := i18n.T("cmd.remote.experimental.doctor.name")
	switch {
	case remoteExperimental(a):
		return []doctor.Result{{Name: name, OK: true, Warn: true, Detail: i18n.T("cmd.remote.experimental.doctor.enabled")}}
	case len(a.Config.Remote.Targets) > 0:
		return []doctor.Result{{Name: name, OK: true, Warn: true, Detail: i18n.T("cmd.remote.experimental.doctor.disabled")}}
	}
	return nil
}
