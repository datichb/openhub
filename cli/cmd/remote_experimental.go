package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// The remote runtime is experimental in 5.0 (DC22): it has never been
// validated on a real runner, so it stays off unless `[remote] experimental
// = true` is set in hub.toml. Every entry point of the remote mode on the
// machine (setup, launch, launch form option, fetch, replay) requires it;
// the job side (`oh runner`) does not read hub.toml.

// remoteExperimental reports whether the remote mode is enabled.
func remoteExperimental(a *app.App) bool {
	return a != nil && a.Config != nil && a.Config.Remote.Experimental
}

// errRemoteDisabled is the translated refusal of a remote entry point.
func errRemoteDisabled() error {
	return errors.New(i18n.T("cmd.remote.experimental.disabled"))
}

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
func remoteRecapWarnings(p *preparedRun) []string {
	if p == nil || p.remote == nil {
		return nil
	}
	return []string{i18n.T("cmd.remote.experimental.warning")}
}

// pickLaunchRuntime chooses the runtime of a launch from the preferences
// (project, Settings) and the workflow; the remote runtime is not chosen by
// default while it is disabled, unless the workflow allows nothing else (the
// launch is then refused with the experimental message).
func pickLaunchRuntime(a *app.App, sp *workflow.Spec, project *domain.Project) workflow.Runtime {
	rt := sp.PickRuntime(runtimePrefs(a, project)...)
	if rt != workflow.RuntimeRemote || remoteExperimental(a) {
		return rt
	}
	for _, r := range sp.AllowedRuntimes() {
		if r != workflow.RuntimeRemote {
			return r
		}
	}
	return rt
}

// remoteExperimentalDoctorCheck reports the state of the remote mode: nothing
// when it is off and no target is configured, else a warning.
func remoteExperimentalDoctorCheck(a *app.App) []views.DoctorCheck {
	if a == nil || a.Config == nil {
		return nil
	}
	name := i18n.T("cmd.remote.experimental.doctor.name")
	switch {
	case remoteExperimental(a):
		return []views.DoctorCheck{{Name: name, OK: true, Warn: true, Detail: i18n.T("cmd.remote.experimental.doctor.enabled")}}
	case len(a.Config.Remote.Targets) > 0:
		return []views.DoctorCheck{{Name: name, OK: true, Warn: true, Detail: i18n.T("cmd.remote.experimental.doctor.disabled")}}
	}
	return nil
}
