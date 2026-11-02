package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

func seedWikiProposal(t *testing.T, repo *teamstate.Repo, id string) {
	t.Helper()
	dir := filepath.Join(repo.Path(), "wiki", ".pending")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	body := "id = \"" + id + "\"\npage = \"decisions\"\ncontent = \"## Decision\"\nconfidence = \"CONFIRMED\"\nauthor = \"documentarian\"\nproject = \"p\"\ncreated_at = 2026-07-07T14:30:00Z\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, id+".toml"), []byte(body), 0o644))
	gittest.Run(t, repo.Path(), "add", "-A")
	gittest.Run(t, repo.Path(), "commit", "-qm", "propose "+id)
	gittest.Run(t, repo.Path(), "push", "-q")
}

func reviewWiki(t *testing.T, id, answers string) error {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.SetIn(strings.NewReader(answers))
	return runTeamWikiReview(cmd, []string{id})
}

// v5.1: refusing a proposal of the team wiki needs a reason, asked after the
// choice; without one nothing is refused, with one it goes to the history and
// to the event.
func TestTeamWikiReviewRefusalNeedsAReason(t *testing.T) {
	i18ntest.Set(t, "fr")
	_, repo, stdout := setupClaimTestApp(t)
	seedWikiProposal(t, repo, "wp-1")

	require.NoError(t, reviewWiki(t, "wp-1", "r\n\n"))
	pending, err := repo.WikiListPending()
	require.NoError(t, err)
	assert.Len(t, pending, 1, "no reason: the proposal stays")
	assert.Contains(t, stdout.String(), "Motif")

	stdout.Reset()
	require.NoError(t, reviewWiki(t, "wp-1", "r\ndéjà   couvert par l'ADR 12\n"))
	pending, err = repo.WikiListPending()
	require.NoError(t, err)
	assert.Empty(t, pending)
	assert.Contains(t, gittest.Run(t, repo.Path(), "log", "--format=%s", "-3"), "rejected proposal wp-1: déjà couvert par l'ADR 12")
	events, err := repo.ListEvents("p", time.Time{})
	require.NoError(t, err)
	var reasons []any
	for _, e := range events {
		if e.Type == teamstate.EventWikiRejected {
			reasons = append(reasons, e.Data["reason"])
		}
	}
	assert.Equal(t, []any{"déjà couvert par l'ADR 12"}, reasons)
}
