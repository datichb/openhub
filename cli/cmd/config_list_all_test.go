package cmd

import (
	"testing"

	"github.com/datichb/openhub/cli/internal/config"
)

// oh config list --all: every settable key, with its value in hub.toml or
// the default of oh, and where it comes from.
func TestConfigListAll(t *testing.T) {
	cfg := &config.Config{}
	cfg.Session.IdleSleepMinutes = 10
	rows := map[string]configListRow{}
	for _, r := range configListAll(cfg) {
		rows[r.Key] = r
	}
	if r := rows["session.idle_sleep_minutes"]; r.Value != "10" || r.Origin != "hub.toml" {
		t.Fatalf("idle = %+v", r)
	}
	if r := rows["session.attach"]; r.Value != "auto" || r.Origin != "default" {
		t.Fatalf("attach = %+v", r)
	}
	if r := rows["execution.keep_images"]; r.Value != "2" || r.Origin != "default" {
		t.Fatalf("keep_images = %+v", r)
	}
	for _, k := range configFieldKeys() {
		if _, ok := rows[k]; !ok {
			t.Fatalf("%s missing", k)
		}
	}
}
