package cmd

import (
	"testing"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// N20: cli.compact accepts auto (stored empty), always and never.
func TestConfigSetCompact(t *testing.T) {
	i18ntest.Set(t, "en")
	f, ok := lookupConfigField("cli.compact")
	if !ok {
		t.Fatal("cli.compact: unknown key")
	}
	c := &config.Config{}
	for _, v := range []string{"always", "never"} {
		if err := f.Set(c, v); err != nil || c.CLI.Compact != v {
			t.Fatalf("set %s: %v %q", v, err, c.CLI.Compact)
		}
	}
	if err := f.Set(c, "auto"); err != nil || c.CLI.Compact != "" {
		t.Fatalf("auto: %v %q", err, c.CLI.Compact)
	}
	if err := f.Set(c, "tiny"); err == nil {
		t.Fatal("invalid value accepted")
	}
	c.CLI.Compact = "never"
	f.Unset(c)
	if c.CLI.Compact != "" {
		t.Fatalf("unset: %q", c.CLI.Compact)
	}
}
