package cmd

import (
	"testing"

	"github.com/datichb/openhub/cli/internal/config"
)

// A-04: the wait of the proxy for the start of a provider answer is
// settable (`oh config set execution.proxy_first_byte_timeout_minutes`).
func TestConfigSetProxyFirstByteTimeout(t *testing.T) {
	useLocale(t, "en")
	f, ok := lookupConfigField("execution.proxy_first_byte_timeout_minutes")
	if !ok {
		t.Fatal("unknown key")
	}
	c := &config.Config{}
	if err := f.Set(c, "10"); err != nil || c.Execution.ProxyFirstByteTimeoutMinutes != 10 {
		t.Fatalf("set 10: %v %+v", err, c.Execution)
	}
	for _, bad := range []string{"0", "-1", "soon", "241"} {
		if err := f.Set(c, bad); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
	f.Unset(c)
	if c.Execution.ProxyFirstByteTimeoutMinutes != 0 {
		t.Fatalf("unset: %+v", c.Execution)
	}
	if got := configDefaults["execution.proxy_first_byte_timeout_minutes"]; got != "5" {
		t.Fatalf("default shown = %q", got)
	}
}
