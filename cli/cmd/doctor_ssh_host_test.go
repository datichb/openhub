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
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/services/doctor"
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
	isolateGitSSH(t)
	t.Cleanup(teamstate.SetSSHToolRunner(func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "ssh" {
			return nil, errors.New("unexpected " + name)
		}
		return []byte("hostname git.example.test\nport 22\n"), nil
	}))
	return filepath.Join(home, ".ssh", "known_hosts")
}

// isolateGitSSH keeps the ssh command of the machine out of a test: no
// GIT_SSH_COMMAND, no GIT_SSH, and the global configuration of the tests
// (gittest.Defaults: an identity, no core.sshCommand) in a file of its own that
// the test can complete (GIT_CONFIG_GLOBAL), with no system one. It returns the
// path of that file.
func isolateGitSSH(t *testing.T) (globalConfig string) {
	t.Helper()
	t.Setenv("GIT_SSH_COMMAND", "")
	t.Setenv("GIT_SSH", "")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	globalConfig = filepath.Join(t.TempDir(), "gitconfig")
	require.NoError(t, os.WriteFile(globalConfig, []byte(gittest.Defaults()), 0o600))
	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)
	return globalConfig
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

// The doctor says what became of the ssh command the user told git to use:
// the options it carries over (the clone of the team is where core.sshCommand
// is read), or why it does not follow it.
func TestDoctorSSHHostSaysWhatBecameOfTheSSHCommandOfGit(t *testing.T) {
	i18ntest.Set(t, "en")
	knownHosts := sshHostSandbox(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(knownHosts), 0o700))
	require.NoError(t, os.WriteFile(knownHosts, []byte("git.example.test ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMe2oLQFSPbU1ZXsHBUxEcjB6QExxb9fTzDW8/Z/Plf7\n"), 0o600))
	clone := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(clone, ".git"), 0o700))
	io, _, _ := testIO()
	application = &app.App{IO: io, Config: &config.Config{Teams: []config.TeamConfig{
		{ID: "core", Enabled: true, StateRepo: "git@git.example.test:o/core.git", StatePath: clone},
	}}}
	t.Cleanup(func() { application = nil })
	check := func() doctor.Result {
		t.Helper()
		results := sshHostDoctorChecks(t.Context())
		require.Len(t, results, 1)
		return results[0]
	}

	r := check()
	assert.False(t, r.Warn)
	assert.Equal(t, "git.example.test is in known_hosts", r.Detail, "no custom command: nothing more said")

	t.Setenv("GIT_SSH_COMMAND", "ssh -i /keys/work -o StrictHostKeyChecking=yes")
	r = check()
	assert.False(t, r.Warn, "a command that is followed is not a warning")
	assert.Equal(t, "git.example.test is in known_hosts\nSSH options of git (GIT_SSH_COMMAND) carried over: -i /keys/work -o StrictHostKeyChecking=yes", r.Detail)

	t.Setenv("GIT_SSH_COMMAND", "ssh -i /keys/work; touch /tmp/oh-must-not-exist")
	r = check()
	assert.True(t, r.OK)
	assert.True(t, r.Warn, "a command that is not followed is a warning, even when the server is known")
	assert.Contains(t, r.Detail, "The SSH command of git (GIT_SSH_COMMAND) is not followed")
	assert.Contains(t, r.Detail, `the shell syntax ";" is not interpreted`)
	assert.NoFileExists(t, "/tmp/oh-must-not-exist")

	t.Setenv("GIT_SSH_COMMAND", "")
	t.Setenv("GIT_SSH", "/opt/tools/ssh-wrapper")
	r = check()
	assert.True(t, r.Warn)
	assert.Contains(t, r.Detail, "GIT_SSH (/opt/tools/ssh-wrapper) is not used")

	// core.sshCommand of the clone of the team
	t.Setenv("GIT_SSH", "")
	require.NoError(t, os.WriteFile(filepath.Join(clone, ".git", "config"), []byte("[core]\n\tsshCommand = ssh -F /tmp/evil_config\n"), 0o600))
	r = check()
	assert.True(t, r.Warn, "-F from the configuration of a repository is not carried over")
	assert.Contains(t, r.Detail, "(core.sshCommand) is not followed")
	assert.Contains(t, r.Detail, "-F")
	require.NoError(t, os.WriteFile(filepath.Join(clone, ".git", "config"), []byte("[core]\n\tsshCommand = ssh -i /keys/clone\n"), 0o600))
	r = check()
	assert.False(t, r.Warn)
	assert.Contains(t, r.Detail, "SSH options of git (core.sshCommand) carried over: -i /keys/clone")
}
