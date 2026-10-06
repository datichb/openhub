package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	remotesvc "github.com/datichb/openhub/cli/internal/services/remote"
)

func TestRemoteTargetName(t *testing.T) {
	for in, want := range map[string]string{
		"acme":           "acme",
		"Acme/Dev Team/": "acme-dev-team",
		"/":              "default",
	} {
		if got := remoteTargetName(in); got != want {
			t.Errorf("remoteTargetName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPrintRemoteReport(t *testing.T) {
	i18n.SetLocale("en")
	t.Cleanup(func() { i18n.SetLocale("fr") })
	rep := &remotesvc.Report{
		Target: config.RemoteTarget{Name: "acme", URL: "https://gitlab.com", Group: "acme"},
		Steps: []remotesvc.Step{
			{ID: remotesvc.StepAccess, Status: remotesvc.StepOK, Args: []string{"alice", "https://gitlab.com"}},
			{ID: remotesvc.StepVariable, Status: remotesvc.StepFailed, Args: []string{"OH_LLM_KEY"}, Err: errors.New("missing")},
			{ID: remotesvc.StepVariable, Status: remotesvc.StepWarn, Args: []string{"OH_TEAMSTATE_TOKEN"}},
		},
	}
	var out bytes.Buffer
	printRemoteReport(&out, rep)
	s := out.String()
	for _, want := range []string{"acme/oh-runner", "GitLab API access — alice", "CI variable — failed — OH_LLM_KEY (missing)", "oh remote setup --llm"} {
		if !strings.Contains(s, want) {
			t.Fatalf("report lacks %q:\n%s", want, s)
		}
	}
	if strings.Count(s, "oh remote setup --llm") != 1 {
		t.Fatalf("hint repeated:\n%s", s)
	}
}

// Every step has a label and a hint in both languages.
func TestRemoteStepKeys(t *testing.T) {
	for _, loc := range []string{"fr", "en"} {
		i18n.SetLocale(loc)
		for _, id := range []string{remotesvc.StepAccess, remotesvc.StepGroup, remotesvc.StepProject, remotesvc.StepFeatures,
			remotesvc.StepProtected, remotesvc.StepPipeline, remotesvc.StepTrigger, remotesvc.StepVariable,
			remotesvc.StepRunners, remotesvc.StepBinary} {
			for _, k := range []string{"cmd.remote.step." + id, "cmd.remote.hint." + id} {
				if i18n.T(k) == k {
					t.Errorf("%s: missing %s", loc, k)
				}
			}
		}
		for _, st := range []remotesvc.StepStatus{remotesvc.StepCreated, remotesvc.StepUpdated, remotesvc.StepWarn, remotesvc.StepFailed, remotesvc.StepSkipped} {
			if k := "cmd.remote.state." + string(st); i18n.T(k) == k {
				t.Errorf("%s: missing %s", loc, k)
			}
		}
	}
	i18n.SetLocale("fr")
}
