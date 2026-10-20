//go:build !windows

package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gateway"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// S-15: the daemon checks the shell of a local session as the tool starts it
// and as an interactive shell (ENV, ~/.bashrc…), and the alert says which.
func TestShellAlertModes(t *testing.T) {
	prev := i18n.Locale()
	i18n.SetLocale("en")
	t.Cleanup(func() { i18n.SetLocale(prev) })
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	sessions, decisions := sqlite.NewSessionStore(st), sqlite.NewDecisionStore(st)

	root := t.TempDir()
	shims, other, home := filepath.Join(root, "shims"), filepath.Join(root, "other"), filepath.Join(root, "home")
	for _, d := range []string{shims, other, home} {
		require.NoError(t, os.MkdirAll(d, 0o755))
	}
	for _, d := range []string{shims, other} {
		require.NoError(t, os.WriteFile(filepath.Join(d, "bd"), []byte("#!/bin/sh\n"), 0o755))
	}
	userENV := filepath.Join(home, ".shrc")
	require.NoError(t, os.WriteFile(userENV, []byte("export PATH=\""+other+":$PATH\"\n"), 0o644))
	oh, err := gateway.ShellStartup(filepath.Join(root, "run", "shell"), []string{shims}, func(k string) string {
		return map[string]string{"ENV": userENV}[k]
	})
	require.NoError(t, err)

	sessionsDir := t.TempDir()
	write := func(id string, env map[string]string) {
		require.NoError(t, sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunActive, Runtime: "local"}))
		require.NoError(t, os.MkdirAll(filepath.Join(sessionsDir, id), 0o700))
		base := map[string]string{"SHELL": "/bin/sh", "HOME": home, "PATH": shims + ":/usr/bin:/bin", gateway.EnvPathFirst: shims}
		for k, v := range env {
			base[k] = v
		}
		data, _ := json.Marshal(base)
		require.NoError(t, os.WriteFile(filepath.Join(sessionsDir, id, "env.json"), data, 0o600))
	}
	write("ses_command", map[string]string{"PATH": other + ":" + shims + ":/usr/bin:/bin"}) // the tool's shell finds another bd
	write("ses_interactive", map[string]string{"ENV": userENV})                             // only an interactive sh does
	controlled := map[string]string{}
	for k, v := range oh {
		controlled[k] = v
	}
	controlled["ENV"] = oh["ENV"]
	controlled[gateway.EnvUserENV] = userENV
	write("ses_ok", controlled)

	d := &Daemon{opts: Options{Sessions: sessions, Decisions: decisions, SessionsDir: sessionsDir}}
	w := &watcher{d: d, srv: domain.Server{GroupKey: "g1"}, tracks: map[string]*sessionTrack{}}
	open := func(id string) []domain.Decision {
		l, err := decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: id})
		require.NoError(t, err)
		return l
	}
	for _, id := range []string{"ses_command", "ses_interactive", "ses_ok"} {
		w.checkShellOnce(ctx, id)
	}

	require.Len(t, open("ses_command"), 1)
	cmd := open("ses_command")[0]
	assert.True(t, isOhAlert(cmd))
	assert.Contains(t, cmd.Payload.Message, filepath.Join(other, "bd"))
	assert.NotContains(t, cmd.Payload.Message, "interactive")

	require.Len(t, open("ses_interactive"), 1, "ENV is read by an interactive sh only")
	inter := open("ses_interactive")[0]
	assert.True(t, isOhAlert(inter))
	assert.Contains(t, inter.Payload.Message, "interactive shell")
	assert.Contains(t, inter.Payload.Message, filepath.Join(other, "bd"))
	assert.Contains(t, inter.Payload.Message, "OH_PATH_FIRST", "the line to add to ~/.bashrc")

	assert.Empty(t, open("ses_ok"), "oh's ENV file loads the user's and fixes the PATH")

	// Once per session: a second check changes nothing.
	w.checkShellOnce(ctx, "ses_interactive")
	assert.Len(t, open("ses_interactive"), 1)
}
