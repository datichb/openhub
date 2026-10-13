package sqlite

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

func TestWikiProposalStore(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	ws := NewWikiProposalStore(st)

	at := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)
	p := &domain.WikiProposal{ProjectID: "p1", SessionID: "ses_a", ToolSession: "ses_child", Agent: "reviewer", Page: "technical/conventions.md",
		Section: "Nommage", Content: "Fichiers en kebab-case.", Tag: "CONFIRMÉ", Source: "src/a.ts:3", CreatedAt: at}
	require.NoError(t, ws.AddWikiProposal(ctx, p, 3, 5))
	assert.Equal(t, 1, p.Seq)
	assert.Equal(t, "wp-1", p.ID())
	assert.Equal(t, domain.WikiPending, p.Status)

	// Concurrent proposals get distinct numbers and stop at the session limit.
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- ws.AddWikiProposal(ctx, &domain.WikiProposal{ProjectID: "p1", SessionID: "ses_a", Page: "index.md", Content: "x", Tag: "DÉDUIT"}, 3, 5)
		}()
	}
	wg.Wait()
	close(errs)
	limited := 0
	for err := range errs {
		if err != nil {
			require.ErrorIs(t, err, domain.ErrWikiLimit)
			limited++
		}
	}
	assert.Equal(t, 2, limited, "3 open proposals at most per session")

	// The project limit counts every session; a refused proposal is no
	// longer open.
	all, err := ws.ListWikiProposals(ctx, domain.WikiProposalFilter{ProjectID: "p1"})
	require.NoError(t, err)
	require.Len(t, all, 3)
	all[1].Status = domain.WikiRefused
	require.NoError(t, ws.UpdateWikiProposal(ctx, &all[1]))
	require.NoError(t, ws.AddWikiProposal(ctx, &domain.WikiProposal{ProjectID: "p1", SessionID: "ses_b", Page: "index.md", Content: "y", Tag: "INCERTAIN"}, 3, 3))
	require.ErrorIs(t, ws.AddWikiProposal(ctx, &domain.WikiProposal{ProjectID: "p1", SessionID: "ses_c", Page: "index.md", Content: "z", Tag: "INCERTAIN"}, 3, 3), domain.ErrWikiLimit)
	// Another project numbers on its own.
	q := &domain.WikiProposal{ProjectID: "p2", SessionID: "ses_z", Page: "index.md", Content: "z", Tag: "INCERTAIN"}
	require.NoError(t, ws.AddWikiProposal(ctx, q, 3, 3))
	assert.Equal(t, 1, q.Seq)

	got, err := ws.GetWikiProposal(ctx, "p1", 1)
	require.NoError(t, err)
	assert.Equal(t, "reviewer", got.Agent)
	assert.Equal(t, "src/a.ts:3", got.Source)
	assert.Nil(t, got.DecidedAt)
	_, err = ws.GetWikiProposal(ctx, "p1", 99)
	require.ErrorIs(t, err, domain.ErrNotFound)

	now := at.Add(time.Hour)
	got.Status, got.Original, got.Content, got.DecidedBy, got.DecidedAt = domain.WikiAccepted, got.Content, "Fichiers en kebab-case (composants).", "oh", &now
	require.NoError(t, ws.UpdateWikiProposal(ctx, got))
	got.Status, got.AppliedBy, got.AppliedAt = domain.WikiApplied, "oh", &now
	require.NoError(t, ws.UpdateWikiProposal(ctx, got))
	again, err := ws.GetWikiProposal(ctx, "p1", 1)
	require.NoError(t, err)
	assert.Equal(t, domain.WikiApplied, again.Status)
	assert.Equal(t, "Fichiers en kebab-case.", again.Original)
	require.NotNil(t, again.AppliedAt)
	assert.True(t, again.AppliedAt.Equal(now))

	sess, err := ws.ListWikiProposals(ctx, domain.WikiProposalFilter{SessionID: "ses_a", Statuses: []string{domain.WikiPending, domain.WikiAccepted}})
	require.NoError(t, err)
	assert.Len(t, sess, 1, "wp-1 applied, one refused")
	require.ErrorIs(t, ws.UpdateWikiProposal(ctx, &domain.WikiProposal{ProjectID: "p1", Seq: 42}), domain.ErrNotFound)
}
