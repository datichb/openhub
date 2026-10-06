package views

import (
	"testing"

	"github.com/datichb/openhub/cli/internal/config"
)

func TestSettingsRemoteFields(t *testing.T) {
	v := &SettingsView{live: &config.Config{}}
	f := v.remoteFields()
	if len(f) != 2 || f[0].Kind != CfgFieldSectionHeader || f[1].Kind != CfgFieldPlaceholder {
		t.Fatalf("no target: %+v", f)
	}

	v.live.Remote.Targets = []config.RemoteTarget{
		{Name: "acme", URL: "https://gitlab.com", Group: "acme"},
		{Name: "corp", URL: "https://git.corp", Group: "dev", Builder: "dind", Tag: "oh-corp"},
	}
	byKey := map[string][]configField{}
	for _, fld := range v.remoteFields() {
		byKey[fld.Key] = append(byKey[fld.Key], fld)
	}
	if got := byKey["remote_runner_project"][0].Get(); got != "acme/oh-runner" {
		t.Fatalf("runner project = %q", got)
	}
	if got := byKey["remote_builder"][0].Get(); got != "kaniko" {
		t.Fatalf("default builder = %q", got)
	}
	if got := byKey["remote_builder"][1].Get(); got != "dind" {
		t.Fatalf("builder = %q", got)
	}
	byKey["remote_tag"][1].Set("gpu")
	if v.live.Remote.Targets[1].Tag != "gpu" || v.live.Remote.Targets[0].Tag != "" {
		t.Fatalf("set tag on the wrong target: %+v", v.live.Remote.Targets)
	}
	if got := byKey["remote_token_key"][0].Get(); got != "openhub.remote.acme.token" {
		t.Fatalf("token key = %q", got)
	}

	// The undo snapshot does not share the targets with the live config.
	cp := deepCopyConfig(v.live)
	cp.Remote.Targets[0].Tag = "changed"
	if v.live.Remote.Targets[0].Tag != "" {
		t.Fatal("deep copy shares the targets")
	}
}
