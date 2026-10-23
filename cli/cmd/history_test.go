package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
)

// historyApp is an application over a database with one project.
func historyApp(t *testing.T, cfg *config.Config) (*app.App, *bytes.Buffer) {
	t.Helper()
	i18ntest.Set(t, "en")
	st := sqlitetest.New(t)
	sqlitetest.Seed(t, st).Project("p1", "/p1")
	var out bytes.Buffer
	a := &app.App{Config: cfg, Sessions: sqlite.NewSessionStore(st), IO: &app.IOStreams{Out: &out, ErrOut: &out}}
	orig := application
	application = a
	t.Cleanup(func() { application = orig })
	return a, &out
}

// historyCommand is a copy of oh history export / import with its flags.
func historyCommand(t *testing.T, base *cobra.Command, flags map[string]string) *cobra.Command {
	t.Helper()
	c := &cobra.Command{Use: base.Use}
	base.Flags().VisitAll(func(f *pflag.Flag) { c.Flags().String(f.Name, "", "") })
	for k, v := range flags {
		require.NoError(t, c.Flags().Set(k, v), k)
	}
	c.SetContext(context.Background())
	return c
}

// oh history export then import: the CLI and the TUI share the service, and
// export has --member-id like import.
func TestHistoryExportImportCLI(t *testing.T) {
	ctx := context.Background()
	team := &config.Config{Teams: []config.TeamConfig{{ID: "t", Enabled: true, MemberID: "carol"}}}
	a, out := historyApp(t, team)
	require.True(t, a.Config.ActiveTeam().Enabled, "control: the team is active")
	require.NoError(t, a.Sessions.Create(ctx, &domain.Session{ID: "ses_aaaaaaaa", ProjectID: "p1", StartedAt: time.Now().UTC().Truncate(time.Second), Status: domain.SessionStatusCompleted}))
	dir := t.TempDir()

	// Default: the member of the active team.
	def := filepath.Join(dir, "default.json")
	require.NoError(t, runHistoryExport(historyCommand(t, historyExportCmd, map[string]string{"output": def}), nil))
	assert.Contains(t, out.String(), "Exported 1 sessions to "+def)
	raw, err := os.ReadFile(def)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"member_id": "carol"`)

	// --member-id on export.
	forced := filepath.Join(dir, "forced.json")
	require.NoError(t, runHistoryExport(historyCommand(t, historyExportCmd, map[string]string{"output": forced, "member-id": "dave"}), nil))
	raw, err = os.ReadFile(forced)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"member_id": "dave"`)

	// Import into an empty database, the flag wins over the file.
	a2, out2 := historyApp(t, team)
	require.NoError(t, runHistoryImport(historyCommand(t, historyImportCmd, map[string]string{"member-id": "erin"}), []string{forced}))
	assert.Contains(t, out2.String(), "Imported 1 sessions (skipped 0 existing)")
	got, err := a2.Sessions.Get(ctx, "ses_aaaaaaaa")
	require.NoError(t, err)
	require.NotNil(t, got.MemberID)
	assert.Equal(t, "erin", *got.MemberID)

	// A file that is not an export: the error is the one of the command line.
	bad := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(bad, []byte("nope"), 0o600))
	err = runHistoryImport(historyCommand(t, historyImportCmd, nil), []string{bad})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing import file")
}

func TestHistoryFlagsOnBothSides(t *testing.T) {
	assert.NotNil(t, historyExportCmd.Flags().Lookup("member-id"), "export has --member-id")
	assert.NotNil(t, historyImportCmd.Flags().Lookup("member-id"), "import has --member-id")
}
