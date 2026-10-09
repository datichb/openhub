package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/workflow"
)

const remoteEnableHint = "oh config set remote.experimental true"

func withRemoteApp(t *testing.T, experimental bool) *app.App {
	t.Helper()
	a := newMockApp(nil, nil)
	a.Config.Remote = config.RemoteConfig{Experimental: experimental,
		Targets: []config.RemoteTarget{{Name: "acme", URL: "https://gitlab.com", Group: "acme"}}}
	prev := application
	t.Cleanup(func() { application = prev })
	application = a
	return a
}

func assertRemoteDisabled(t *testing.T, what string, err error) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), remoteEnableHint) {
		t.Fatalf("%s: err = %v, want the experimental refusal", what, err)
	}
}

// The refusal is translated and names the command that enables the mode.
func TestRemoteExperimentalMessage(t *testing.T) {
	for _, l := range []string{"fr", "en"} {
		useLocale(t, l)
		assertRemoteDisabled(t, l, requireRemoteExperimental(nil))
	}
	useLocale(t, "fr")
	if err := requireRemoteExperimental(nil); !strings.Contains(err.Error(), "expérimental") {
		t.Fatalf("fr: %v", err)
	}
	useLocale(t, "en")
	if err := requireRemoteExperimental(nil); !strings.Contains(err.Error(), "experimental remote mode, not validated") {
		t.Fatalf("en: %v", err)
	}
	a := &app.App{Config: &config.Config{Remote: config.RemoteConfig{Experimental: true}}}
	if err := requireRemoteExperimental(a); err != nil {
		t.Fatal(err)
	}
}

// `oh config set remote.experimental true` enables the mode.
func TestRemoteExperimentalConfigKey(t *testing.T) {
	c := &config.Config{}
	f, ok := lookupConfigField("remote.experimental")
	if !ok {
		t.Fatal("remote.experimental is not settable")
	}
	if err := f.Set(c, "true"); err != nil || !c.Remote.Experimental {
		t.Fatalf("set: %v %v", err, c.Remote.Experimental)
	}
	f.Unset(c)
	if c.Remote.Experimental {
		t.Fatal("unset kept the mode")
	}
}

// Every machine entry point of the remote mode refuses without the flag.
func TestRemoteEntryPointsRefusedWithoutFlag(t *testing.T) {
	useLocale(t, "en")
	a := withRemoteApp(t, false)
	ctx := context.Background()
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	_, err := prepareRemote(ctx, a, &domain.Project{ID: "p1", Name: "p1"}, nil)
	assertRemoteDisabled(t, "oh run --runtime remote", err)
	setup := remoteSetupCmd()
	setup.SetContext(ctx)
	setup.SetOut(&bytes.Buffer{})
	assertRemoteDisabled(t, "oh remote setup", runRemoteSetup(setup, nil))
	assertRemoteDisabled(t, "oh session fetch", runSessionFetch(cmd, []string{"ses_x"}))
	assertRemoteDisabled(t, "oh session resolve", runSessionResolve(cmd, []string{"ses_x"}))

	ts := &tuiSessions{a: a}
	_, err = ts.FetchRemote(ctx, "ses_x")
	assertRemoteDisabled(t, "TUI fetch", err)
	_, err = ts.PlanReplay(ctx, "ses_x")
	assertRemoteDisabled(t, "TUI replay plan", err)
	_, err = ts.ApplyReplay(ctx, "ses_x", nil)
	assertRemoteDisabled(t, "TUI replay", err)

	lr := remoteLaunchRuntime(ctx, a, &domain.Project{ID: "p1", Name: "p1", Path: t.TempDir()}, views.LaunchRuntime{Kind: "remote", Label: "☁"})
	if lr.Available || !strings.Contains(lr.Reason, remoteEnableHint) {
		t.Fatalf("☁ option = %+v", lr)
	}
}

// The remote runtime is never chosen by default while disabled.
func TestPickLaunchRuntimeSkipsDisabledRemote(t *testing.T) {
	sp := &workflow.Spec{Runtime: &workflow.RuntimeSpec{Default: workflow.RuntimeRemote,
		Allowed: []workflow.Runtime{workflow.RuntimeRemote, workflow.RuntimeLocal}}}
	off := &app.App{Config: &config.Config{}}
	on := &app.App{Config: &config.Config{Remote: config.RemoteConfig{Experimental: true}}}
	if got := pickLaunchRuntime(off, sp, nil); got != workflow.RuntimeLocal {
		t.Fatalf("disabled: %s", got)
	}
	if got := pickLaunchRuntime(on, sp, nil); got != workflow.RuntimeRemote {
		t.Fatalf("enabled: %s", got)
	}
	only := &workflow.Spec{Runtime: &workflow.RuntimeSpec{Default: workflow.RuntimeRemote, Allowed: []workflow.Runtime{workflow.RuntimeRemote}}}
	if got := pickLaunchRuntime(off, only, nil); got != workflow.RuntimeRemote {
		t.Fatalf("remote only: %s (refused later with the experimental message)", got)
	}
}

// oh remote status and Doctor always warn about the experimental mode.
func TestRemoteExperimentalWarnings(t *testing.T) {
	useLocale(t, "en")
	var out bytes.Buffer
	printRemoteExperimentalWarning(&out)
	if !strings.Contains(out.String(), "never validated on a real GitLab runner") {
		t.Fatalf("warning = %q", out.String())
	}
	if got := remoteExperimentalDoctorCheck(&app.App{Config: &config.Config{}}); len(got) != 0 {
		t.Fatalf("no target, disabled: %+v", got)
	}
	a := withRemoteApp(t, false)
	got := remoteExperimentalDoctorCheck(a)
	if len(got) != 1 || !got[0].Warn || !strings.Contains(got[0].Detail, remoteEnableHint) {
		t.Fatalf("disabled with a target: %+v", got)
	}
	a.Config.Remote.Experimental = true
	got = remoteExperimentalDoctorCheck(a)
	if len(got) != 1 || !got[0].Warn || !strings.Contains(got[0].Detail, "not validated") {
		t.Fatalf("enabled: %+v", got)
	}
}

// The recap of a remote launch carries the warning.
func TestRunRecapWarnsRemote(t *testing.T) {
	useLocale(t, "en")
	if !strings.Contains(strings.Join(remoteRecapWarnings(&preparedRun{remote: &remotePrep{}}), "\n"), "Experimental remote mode") {
		t.Fatal("remote recap without the experimental warning")
	}
	if len(remoteRecapWarnings(&preparedRun{})) != 0 {
		t.Fatal("local recap with the remote warning")
	}
}
