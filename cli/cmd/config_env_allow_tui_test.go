package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// The Settings screen of the TUI and `oh config set execution.env_allow`
// accept and refuse the same values, and store the same list.
func TestEnvAllowSettingsMatchTheCLI(t *testing.T) {
	i18ntest.Set(t, "en")
	f, ok := lookupConfigField("execution.env_allow")
	require.True(t, ok)
	for _, in := range []string{
		"", "SSH_AUTH_SOCK", "SSH_AUTH_SOCK, MYCO_*  JAVA_OPTS,SSH_AUTH_SOCK", "MYCO_*", "GH_TOKEN",
		"*", "A*", "BAD NAME=1", "X=Y", "-V", "OK, *", "A**", "1ABC", "NAME(1)", "  ,, ",
	} {
		c := &config.Config{}
		cliErr := f.Set(c, in)
		tui, tuiErr := views.ParseEnvAllow(in)
		assert.Equal(t, cliErr != nil, tuiErr != nil, "%q: CLI %v, TUI %v", in, cliErr, tuiErr)
		if cliErr == nil {
			assert.Equal(t, c.Execution.EnvAllow, tui, "%q", in)
		}
	}
}
