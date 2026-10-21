package cmd

import (
	"testing"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// A-04: the wait of the proxy for the start of a provider answer is
// settable (`oh config set execution.proxy_first_byte_timeout_minutes`).
func TestConfigSetProxyFirstByteTimeout(t *testing.T) {
	i18ntest.Set(t, "en")
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

// S-28: [execution] env_allow adds variables to the environment the tool
// server inherits (`oh config set execution.env_allow`).
func TestConfigSetEnvAllow(t *testing.T) {
	i18ntest.Set(t, "en")
	f, ok := lookupConfigField("execution.env_allow")
	if !ok {
		t.Fatal("unknown key")
	}
	c := &config.Config{}
	if err := f.Set(c, "SSH_AUTH_SOCK, MYCO_*  JAVA_OPTS,SSH_AUTH_SOCK"); err != nil {
		t.Fatal(err)
	}
	want := []string{"SSH_AUTH_SOCK", "MYCO_*", "JAVA_OPTS"}
	if got := c.Execution.EnvAllow; len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("env_allow = %v, want %v (no duplicate)", got, want)
	}
	for _, bad := range []string{"*", "A*", "BAD NAME=1", "X=Y", "-V"} {
		before := c.Execution.EnvAllow
		if err := f.Set(c, bad); err == nil {
			t.Fatalf("%q accepted", bad)
		}
		if len(c.Execution.EnvAllow) != len(before) {
			t.Fatalf("%q changed the list", bad)
		}
	}
	if err := f.Set(c, ""); err != nil || len(c.Execution.EnvAllow) != 0 {
		t.Fatalf("empty clears: %v %v", err, c.Execution.EnvAllow)
	}
	_ = f.Set(c, "A_B")
	f.Unset(c)
	if c.Execution.EnvAllow != nil {
		t.Fatalf("unset: %v", c.Execution.EnvAllow)
	}
}
