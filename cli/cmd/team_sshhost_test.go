package cmd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// The first contact with an SSH server in the assistants. No real ssh or
// ssh-keyscan runs (a fake runner), no network is reached, and the
// known_hosts file is one of the test: the real ~/.ssh is never touched.

// a throwaway ed25519 public key
const testHostKey = "AAAAC3NzaC1lZDI1NTE5AAAAIMe2oLQFSPbU1ZXsHBUxEcjB6QExxb9fTzDW8/Z/Plf7"

func testHostFingerprint(t *testing.T) string {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(testHostKey)
	require.NoError(t, err)
	sum := sha256.Sum256(raw)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}

// sshFixture is the SSH side of a test: the known_hosts file, the servers
// that answer a scan, and what was run.
type sshFixture struct {
	t          *testing.T
	knownHosts string
	mu         sync.Mutex
	scanned    []string // ssh-keyscan invocations, as one line each
}

// fakeSSHServers installs the fake tools; the hosts given are already in
// known_hosts. The input of the commands is not a terminal until
// fx.terminal().
func fakeSSHServers(t *testing.T, known ...string) *sshFixture {
	t.Helper()
	isolateGitSSH(t)
	fx := &sshFixture{t: t, knownHosts: filepath.Join(t.TempDir(), "known_hosts")}
	var content strings.Builder
	for _, h := range known {
		content.WriteString(h + " ssh-ed25519 " + testHostKey + "\n")
	}
	require.NoError(t, os.WriteFile(fx.knownHosts, []byte(content.String()), 0o600))
	t.Cleanup(teamstate.SetSSHToolRunner(fx.run))
	prev := sshHostPromptable
	sshHostPromptable = func(io.Reader) bool { return false }
	t.Cleanup(func() { sshHostPromptable = prev })
	return fx
}

func (fx *sshFixture) terminal() {
	prev := sshHostPromptable
	sshHostPromptable = func(io.Reader) bool { return true }
	fx.t.Cleanup(func() { sshHostPromptable = prev })
}

func (fx *sshFixture) run(_ context.Context, name string, args ...string) ([]byte, error) {
	host := args[len(args)-1]
	switch name {
	case "ssh": // ssh -G [-p port] [-l user] -- host
		cfg := "hostname " + host + "\nport 22\nuserknownhostsfile " + fx.knownHosts + "\nglobalknownhostsfile none\n"
		for i, a := range args {
			if a == "-p" {
				cfg = strings.Replace(cfg, "port 22", "port "+args[i+1], 1)
			}
		}
		return []byte(cfg), nil
	case "ssh-keyscan":
		fx.mu.Lock()
		fx.scanned = append(fx.scanned, strings.Join(args, " "))
		fx.mu.Unlock()
		name := host
		for i, a := range args {
			if a == "-p" && args[i+1] != "22" {
				name = "[" + host + "]:" + args[i+1]
			}
		}
		return []byte("# " + host + " SSH-2.0-fake\n" + name + " ssh-ed25519 " + testHostKey + "\n"), nil
	}
	return nil, os.ErrNotExist
}

func (fx *sshFixture) scans() []string {
	fx.mu.Lock()
	defer fx.mu.Unlock()
	return append([]string(nil), fx.scanned...)
}

func (fx *sshFixture) knownHostsText() string {
	data, err := os.ReadFile(fx.knownHosts)
	require.NoError(fx.t, err)
	return string(data)
}

func prompterOn(answers string) (*cliPrompter, *bytes.Buffer) {
	out := &bytes.Buffer{}
	return &cliPrompter{in: strings.NewReader(answers), out: out}, out
}

func TestEnsureSSHHostIgnoresWhatIsNotAnUnknownSSHServer(t *testing.T) {
	i18ntest.Set(t, "en")
	fx := fakeSSHServers(t, "known.example.test")
	for _, remote := range []string{
		"git@known.example.test:org/team.git",
		"https://gitlab.example.test/org/team.git",
		"/srv/git/team.git",
	} {
		p, out := prompterOn("")
		require.NoError(t, p.ensureSSHHost(t.Context(), remote), remote)
		assert.Empty(t, out.String(), remote)
	}
	assert.Empty(t, fx.scans(), "a known server is not scanned: nothing to ask")
}

func TestEnsureSSHHostWithoutATerminalRefusesAndTellsWhatToRun(t *testing.T) {
	i18ntest.Set(t, "en")
	fx := fakeSSHServers(t)
	p, out := prompterOn("y\n") // an answer is there, and must not be read
	err := p.ensureSSHHost(t.Context(), "ssh://git@new.example.test:2222/org/team.git")
	require.Error(t, err)
	unknown := teamstate.AsUnknownHostError(err)
	require.NotNil(t, unknown, "a typed refusal: %v", err)
	assert.Contains(t, err.Error(), "ssh -p 2222 -T git@new.example.test")
	assert.Contains(t, err.Error(), "ssh-keyscan -p 2222 new.example.test >>")
	assert.Empty(t, out.String(), "nothing is asked")
	assert.Empty(t, fx.scans(), "and the server is not contacted")
	assert.Empty(t, fx.knownHostsText(), "nothing is written")
}

func TestEnsureSSHHostOnATerminal(t *testing.T) {
	i18ntest.Set(t, "en")
	remote := "git@new.example.test:org/team.git"

	t.Run("the fingerprint is shown, a refusal stops and writes nothing", func(t *testing.T) {
		fx := fakeSSHServers(t)
		fx.terminal()
		p, out := prompterOn("n\n")
		err := p.ensureSSHHost(t.Context(), remote)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not trusted")
		assert.Contains(t, out.String(), testHostFingerprint(t))
		assert.Contains(t, out.String(), "ssh-ed25519")
		assert.Empty(t, fx.knownHostsText())
	})

	t.Run("Enter alone is a refusal", func(t *testing.T) {
		fx := fakeSSHServers(t)
		fx.terminal()
		p, _ := prompterOn("\n")
		require.Error(t, p.ensureSSHHost(t.Context(), remote))
		assert.Empty(t, fx.knownHostsText())
	})

	t.Run("a confirmation adds the server, once", func(t *testing.T) {
		fx := fakeSSHServers(t, "other.example.test")
		fx.terminal()
		p, out := prompterOn("y\n")
		require.NoError(t, p.ensureSSHHost(t.Context(), remote))
		assert.Contains(t, out.String(), testHostFingerprint(t))
		assert.Contains(t, fx.knownHostsText(), "other.example.test ssh-ed25519 ", "what was there stays")
		assert.Contains(t, fx.knownHostsText(), "new.example.test ssh-ed25519 "+testHostKey)

		p, out = prompterOn("") // known now: no question, no scan
		require.NoError(t, p.ensureSSHHost(t.Context(), remote))
		assert.Empty(t, out.String())
		assert.Len(t, fx.scans(), 1)
	})

	t.Run("a server that does not answer is an error, not a question", func(t *testing.T) {
		fx := fakeSSHServers(t)
		fx.terminal()
		restore := teamstate.SetSSHToolRunner(func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "ssh-keyscan" {
				return nil, os.ErrDeadlineExceeded
			}
			return fx.run(ctx, name, args...)
		})
		t.Cleanup(restore)
		p, out := prompterOn("y\n")
		err := p.ensureSSHHost(t.Context(), remote)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "new.example.test")
		assert.Contains(t, err.Error(), "ssh -T git@new.example.test")
		assert.NotContains(t, out.String(), "Trust this server")
		assert.Empty(t, fx.knownHostsText())
	})
}

// oh team rejoin / oh teams add: the server comes before the probe and the
// clone.
func TestTeamRejoinConfirmsAnUnknownSSHServer(t *testing.T) {
	remote := "git@new.example.test:org/team.git"

	t.Run("without a terminal: refused, nothing cloned, nothing written", func(t *testing.T) {
		fx := fakeSSHServers(t)
		fakeSSHRefusing(t)
		a, out, err := rejoinOn(t, remote, "", "--member-id=testuser")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ssh -T git@new.example.test")
		assert.Contains(t, err.Error(), "ssh-keyscan new.example.test >> ")
		assert.NotContains(t, err.Error(), "ssh-add", "the key was never tried")
		assert.Empty(t, fx.scans())
		assert.Empty(t, fx.knownHostsText())
		assert.Nil(t, a.Config.FindTeamByRepo(remote))
		assert.NotContains(t, out, "Token")
	})

	t.Run("on a terminal, refused: nothing is added", func(t *testing.T) {
		fx := fakeSSHServers(t)
		fx.terminal()
		fakeSSHRefusing(t)
		_, out, err := rejoinOn(t, remote, "n\n", "--member-id=testuser")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not trusted")
		assert.Contains(t, out, testHostFingerprint(t))
		assert.Empty(t, fx.knownHostsText())
	})

	t.Run("on a terminal, confirmed: the server is known and the access goes on", func(t *testing.T) {
		fx := fakeSSHServers(t)
		fx.terminal()
		fakeSSHRefusing(t)
		_, out, err := rejoinOn(t, remote, "y\n", "--member-id=testuser")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ssh-add", "the probe ran: the stub ssh refuses the key")
		assert.Contains(t, out, testHostFingerprint(t))
		assert.Contains(t, fx.knownHostsText(), "new.example.test ssh-ed25519 "+testHostKey)
	})

	t.Run("a known server is not asked about", func(t *testing.T) {
		fx := fakeSSHServers(t, "new.example.test")
		fakeSSHRefusing(t)
		_, out, err := rejoinOn(t, remote, "", "--member-id=testuser")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ssh-add")
		assert.NotContains(t, out, "known_hosts")
		assert.Empty(t, fx.scans())
	})
}

// oh teams set <id> repo <url>: the new server is confirmed before the URL
// is touched.
func TestTeamsSetRepoConfirmsAnUnknownSSHServer(t *testing.T) {
	i18ntest.Set(t, "en")
	configtest.Home(t)
	fx := fakeSSHServers(t)
	a, repo, _ := setupClaimTestApp(t)
	require.NoError(t, config.Save(a.Config))
	oldRemote := repo.Remote()

	err := runTeamsSetArgs(t, "test-team", "repo", "git@moved.example.test:org/team.git")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ssh -T git@moved.example.test")
	assert.Equal(t, oldRemote, a.Config.FindTeam("test-team").StateRepo, "hub.toml is untouched")
	assert.Equal(t, oldRemote, gittest.Run(t, repo.Path(), "remote", "get-url", "origin"))
	assert.Empty(t, fx.scans())
	assert.Empty(t, fx.knownHostsText())
}

// oh team promote --remote: nothing is pushed to a server nobody confirmed.
func TestTeamPromoteConfirmsAnUnknownSSHServer(t *testing.T) {
	i18ntest.Set(t, "en")
	a, out := setupSoloTestApp(t)
	fx := fakeSSHServers(t)
	require.NoError(t, runTeamSub(t, teamInitCmd, out, "--solo", "--member-id", "bob", "--project", "web"))

	err := runTeamSub(t, findTeamSub(t, "promote"), out, "--remote", "ssh://git@new.example.test:2222/org/team.git")
	require.Error(t, err)
	assert.NotNil(t, teamstate.AsUnknownHostError(err), "%v", err)
	assert.Contains(t, err.Error(), "ssh -p 2222 -T git@new.example.test")
	assert.True(t, a.Config.Teams[0].Solo, "still solo")
	assert.Empty(t, fx.knownHostsText())
}

// The steps of the assistants (create, join, palette): the same question, on
// the screen.
func TestTeamAccessHostSteps(t *testing.T) {
	i18ntest.Set(t, "en")
	repo := "https://gitlab.example.test/org/team.git"
	fakeSSHServers(t)
	acc := &teamAccess{Repo: &repo}
	steps := acc.HostSteps()
	require.Len(t, steps, 2)
	scan, host := steps[0], steps[1]
	assert.Equal(t, stepTeamHostScan, scan.ID)
	assert.Equal(t, stepTeamHost, host.ID)
	assert.True(t, scan.Required && host.Required, "Esc cannot skip the confirmation: the first connection would accept the server")

	t.Run("nothing for HTTPS, a local path or a known server", func(t *testing.T) {
		assert.True(t, scan.SkipIf())
		assert.True(t, host.SkipIf())
		repo = "/srv/git/team.git"
		assert.True(t, scan.SkipIf())
		fx2 := fakeSSHServers(t, "known.example.test")
		repo = "git@known.example.test:org/team.git"
		assert.True(t, scan.SkipIf())
		assert.Empty(t, fx2.scans())
	})

	t.Run("an unknown server: scanned, shown, refused or trusted", func(t *testing.T) {
		fx := fakeSSHServers(t)
		repo = "ssh://git@new.example.test:2222/org/team.git"
		assert.False(t, scan.SkipIf(), "unknown: the keys are read")
		assert.True(t, host.SkipIf(), "and nothing is asked before they are")

		require.NoError(t, scan.OnDone())
		assert.Contains(t, fx.scans()[0], "-p 2222")
		require.False(t, host.SkipIf())
		done := 0
		form := host.Form(tview.NewApplication(), func() { done++ })
		text := form.GetFormItem(0).(*tview.TextView).GetText(true)
		assert.Contains(t, text, "new.example.test:2222")
		assert.Contains(t, text, testHostFingerprint(t))
		assert.Equal(t, 2, form.GetButtonCount())
		assert.Equal(t, i18n.T("v54a2.host.refuse"), form.GetButton(0).GetLabel(), "the safe answer is the first button")
		assert.Equal(t, i18n.T("v54a2.host.trust"), form.GetButton(1).GetLabel())

		pressButton(form.GetButton(0))
		assert.Equal(t, 1, done)
		err := host.OnDone()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not trusted")
		assert.Empty(t, fx.knownHostsText())
		actions := host.ErrorActions(err)
		require.NotEmpty(t, actions)
		var labels []string
		for _, a := range actions {
			labels = append(labels, a.Label)
		}
		assert.Contains(t, labels, i18n.T("v532w.action.change_url"), "the URL can be changed")

		form = host.Form(tview.NewApplication(), func() {})
		pressButton(form.GetButton(1))
		require.NoError(t, host.OnDone())
		assert.Contains(t, fx.knownHostsText(), "[new.example.test]:2222 ssh-ed25519 "+testHostKey)
		assert.True(t, scan.SkipIf(), "known now")
		assert.True(t, host.SkipIf())
		assert.Contains(t, host.InfoFields()[0].Value, "new.example.test:2222")
	})

	t.Run("a server that does not answer is an error of the first step", func(t *testing.T) {
		fx := fakeSSHServers(t)
		t.Cleanup(teamstate.SetSSHToolRunner(func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "ssh-keyscan" {
				return nil, os.ErrDeadlineExceeded
			}
			return fx.run(ctx, name, args...)
		}))
		repo = "git@down.example.test:org/team.git"
		err := scan.OnDone()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ssh -T git@down.example.test")
		assert.True(t, host.SkipIf(), "no confirmation without keys")
		assert.Empty(t, fx.knownHostsText())
	})

	t.Run("the group given up skips them", func(t *testing.T) {
		fakeSSHServers(t)
		repo = "git@new.example.test:org/team.git"
		acc.Active = func() bool { return false }
		assert.True(t, scan.SkipIf())
		assert.True(t, host.SkipIf())
	})
}

// On the screen, with keys: the question is drawn, a refusal gives the error
// screen, the trust goes on.
func TestTeamAccessHostStepsOnTheScreen(t *testing.T) {
	if testing.Short() {
		t.Skip("simulation screen")
	}
	i18ntest.Set(t, "en")
	newWizard := func(t *testing.T, repo string) (*sshFixture, views.InlineWizardConfig, *bool) {
		fx := fakeSSHServers(t)
		acc := &teamAccess{Repo: &repo}
		reached := false
		steps := append(acc.HostSteps(), views.WizardStep{
			ID:    "after",
			Label: "after",
			Form: func(_ *tview.Application, done func()) *tview.Form {
				reached = true
				return tview.NewForm().AddButton("Finish", done)
			},
		})
		return fx, views.InlineWizardConfig{ID: "wizard.test.host", Title: "host", Steps: steps}, &reached
	}

	t.Run("trust", func(t *testing.T) {
		fx, cfg, reached := newWizard(t, "git@new.example.test:org/team.git")
		res := runWizardOnKeys(t, cfg, func(k *wizardKeys) {
			waitForText(k, "new.example.test")
			waitForText(k, testHostFingerprint(t))
			k.key(tcell.KeyTab, tcell.KeyEnter) // Refuse → Trust this server
			waitForText(k, "Finish")
			k.key(tcell.KeyEnter)
		})
		require.NoError(t, res.Err)
		assert.True(t, *reached)
		assert.Contains(t, fx.knownHostsText(), "new.example.test ssh-ed25519 "+testHostKey)
	})

	t.Run("refuse", func(t *testing.T) {
		fx, cfg, reached := newWizard(t, "git@new.example.test:org/team.git")
		res := runWizardOnKeys(t, cfg, func(k *wizardKeys) {
			waitForText(k, testHostFingerprint(t))
			k.key(tcell.KeyEnter) // Refuse is the first button
			waitForText(k, "not trusted")
			k.key(tcell.KeyCtrlC)
		})
		_ = res
		assert.False(t, *reached, "nothing goes on after a refusal")
		assert.Empty(t, fx.knownHostsText())
	})
}
