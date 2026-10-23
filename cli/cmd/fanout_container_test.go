package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/launch"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// fanout-container: a fan-out launches locally or in a container, never in
// a remote session (backlog); its sessions run in the runtime of the parent.
func TestFanoutLaunchRuntimes(t *testing.T) {
	a := &app.App{Config: &config.Config{Fanout: config.FanoutConfig{Experimental: true}}}
	for _, rt := range []sessionspec.RuntimeKind{"", sessionspec.RuntimeLocal, sessionspec.RuntimeContainer} {
		assert.NoError(t, launch.CheckFanoutLaunch(a.Config, rt, nil), "runtime %q", rt)
	}
	err := launch.CheckFanoutLaunch(a.Config, sessionspec.RuntimeRemote, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remote")

	off := &app.App{Config: &config.Config{}}
	assert.Error(t, launch.CheckFanoutLaunch(off.Config, sessionspec.RuntimeContainer, nil), "still experimental")
}

func TestFanoutSessionsRuntimeOfTheParent(t *testing.T) {
	for parent, want := range map[string]sessionspec.RuntimeKind{
		"": sessionspec.RuntimeLocal, "local": sessionspec.RuntimeLocal, "container": sessionspec.RuntimeContainer,
	} {
		got, err := fanoutRuntime(&domain.Session{Runtime: parent})
		require.NoError(t, err)
		assert.Equal(t, want, got, "parent %q", parent)
	}
	_, err := fanoutRuntime(&domain.Session{Runtime: "remote"})
	assert.Error(t, err, "an imported remote parent never gets children")
}
