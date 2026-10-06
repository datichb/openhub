package cmd

import (
	"path/filepath"
	"testing"

	"github.com/datichb/openhub/cli/internal/config"
)

func TestTeamByStatePath(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Teams: []config.TeamConfig{
		{ID: "off", StatePath: filepath.Join(dir, "off")},
		{ID: "solo", Enabled: true, Solo: true, StatePath: filepath.Join(dir, "solo")},
	}}
	if got := teamByStatePath(cfg, filepath.Join(dir, "solo")+"/"); got == nil || got.ID != "solo" {
		t.Fatalf("solo = %+v", got)
	}
	if got := teamByStatePath(cfg, filepath.Join(dir, "off")); got != nil {
		t.Fatalf("disabled team matched: %+v", got)
	}
}
