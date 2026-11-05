package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// sshHostSandbox gives the test a HOME of its own and an ssh that answers
// `ssh -G` with the server of the remote (nothing touches the network or the
// real ~/.ssh).
func sshHostSandbox(t *testing.T) (knownHosts string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Cleanup(teamstate.SetSSHToolRunner(func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "ssh" {
			return nil, errors.New("unexpected " + name)
		}
		return []byte("hostname git.example.test\nport 22\n"), nil
	}))
	return filepath.Join(home, ".ssh", "known_hosts")
}

// The doctor says which SSH servers of the shared teams known_hosts lacks,
// with the command to run, and talks to no server: ssh-keyscan is not even
// called (the fake runner of ssh refuses it).
func TestDoctorSSHHost(t *testing.T) {
	i18ntest.Set(t, "en")
	knownHosts := sshHostSandbox(t)
	io, _, _ := testIO()
	application = &app.App{IO: io, Config: &config.Config{Teams: []config.TeamConfig{
		{ID: "core", Enabled: true, StateRepo: "git@git.example.test:o/core.git"},
		{ID: "web", Enabled: true, StateRepo: "https://gitlab.example.com/o/web.git"},
		{ID: "solo", Enabled: true, Solo: true},
		{ID: "old", Enabled: false, StateRepo: "git@git.example.test:o/old.git"},
	}}}
	t.Cleanup(func() { application = nil })

	results := sshHostDoctorChecks(t.Context())
	require.Len(t, results, 1, "one per shared team over SSH")
	r := results[0]
	assert.Equal(t, doctorIDSSHHost, r.ID)
	assert.Equal(t, "Known SSH server (core)", r.Name)
	assert.True(t, r.OK)
	assert.True(t, r.Warn, "a warning, not a failure")
	assert.Contains(t, r.Detail, "git.example.test is not in known_hosts")
	assert.Contains(t, r.Detail, "ssh -T git@git.example.test")

	require.NoError(t, os.MkdirAll(filepath.Dir(knownHosts), 0o700))
	require.NoError(t, os.WriteFile(knownHosts, []byte("git.example.test ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMe2oLQFSPbU1ZXsHBUxEcjB6QExxb9fTzDW8/Z/Plf7\n"), 0o600))
	results = sshHostDoctorChecks(t.Context())
	require.Len(t, results, 1)
	assert.True(t, results[0].OK)
	assert.False(t, results[0].Warn)
	assert.Contains(t, results[0].Detail, "is in known_hosts")
}

func TestDoctorSSHHostWithoutATeamSaysNothing(t *testing.T) {
	io, _, _ := testIO()
	application = &app.App{IO: io, Config: &config.Config{}}
	t.Cleanup(func() { application = nil })
	assert.Empty(t, sshHostDoctorChecks(t.Context()))
}
