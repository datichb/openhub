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

// oh gc purges the closed proposals (applied, refused) past the history
// retention; open ones stay, and so does the last proposal of each project
// (its number is the next one's base).
func TestWikiProposalPurge(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	ws := NewWikiProposalStore(st)
	old := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	recent := time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)
	add := func(project, status string, at time.Time) {
		t.Helper()
		p := &domain.WikiProposal{ProjectID: project, SessionID: "s", Page: "index.md", Content: "x", Tag: "DÉDUIT", CreatedAt: at}
		require.NoError(t, ws.AddWikiProposal(ctx, p, 0, 0))
		if status != domain.WikiPending {
			p.Status, p.DecidedAt = status, &at
			if status == domain.WikiApplied {
				p.AppliedAt = &at
			}
			require.NoError(t, ws.UpdateWikiProposal(ctx, p))
		}
	}
	add("p1", domain.WikiApplied, old)    // wp-1: purged
	add("p1", domain.WikiRefused, old)    // wp-2: purged
	add("p1", domain.WikiPending, old)    // wp-3: open, kept
	add("p1", domain.WikiApplied, recent) // wp-4: recent, kept
	add("p1", domain.WikiRefused, old)    // wp-5: the last one of p1, kept
	add("p2", domain.WikiApplied, old)    // wp-1 of p2: purged
	add("p2", domain.WikiApplied, old)    // wp-2 of p2: the last one, kept
	hs := NewHistoryStore(st)
	cutoff := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	n, err := hs.Count(ctx, cutoff, cutoff)
	require.NoError(t, err)
	assert.Equal(t, 3, n.WikiProposals)
	n, err = hs.Delete(ctx, cutoff, cutoff)
	require.NoError(t, err)
	assert.Equal(t, 3, n.WikiProposals)
	left, err := ws.ListWikiProposals(ctx, domain.WikiProposalFilter{})
	require.NoError(t, err)
	var ids []string
	for _, p := range left {
		ids = append(ids, p.ProjectID+"/"+p.ID())
	}
	assert.ElementsMatch(t, []string{"p1/wp-3", "p1/wp-4", "p1/wp-5", "p2/wp-2"}, ids)
	p := &domain.WikiProposal{ProjectID: "p1", SessionID: "s", Page: "index.md", Content: "y", Tag: "DÉDUIT"}
	require.NoError(t, ws.AddWikiProposal(ctx, p, 0, 0))
	assert.Equal(t, "wp-6", p.ID(), "a purged number is never given again")
}

// v49 (p9-wiki-refusal): the reason of a refusal goes to the database and
// comes back from Get and List; a proposal without one reads "".
func TestWikiProposalReasonRoundTrip(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	ws := NewWikiProposalStore(st)
	add := func(content string) *domain.WikiProposal {
		p := &domain.WikiProposal{ProjectID: "p1", SessionID: "ses_a", Page: "index.md", Content: content, Tag: "DÉDUIT"}
		require.NoError(t, ws.AddWikiProposal(ctx, p, 0, 0))
		return p
	}
	refused, plain := add("un"), add("deux")
	now := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)
	refused.Status, refused.DecidedBy, refused.DecidedAt, refused.Reason = domain.WikiRefused, "oh", &now, "déjà couvert ; « stack.md »"
	require.NoError(t, ws.UpdateWikiProposal(ctx, refused))

	got, err := ws.GetWikiProposal(ctx, "p1", refused.Seq)
	require.NoError(t, err)
	assert.Equal(t, "déjà couvert ; « stack.md »", got.Reason)
	assert.Equal(t, domain.WikiRefused, got.Status)
	list, err := ws.ListWikiProposals(ctx, domain.WikiProposalFilter{ProjectID: "p1", Statuses: []string{domain.WikiRefused}})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, got.Reason, list[0].Reason)
	other, err := ws.GetWikiProposal(ctx, "p1", plain.Seq)
	require.NoError(t, err)
	assert.Empty(t, other.Reason)

	// A reason given at the creation is kept as well.
	q := &domain.WikiProposal{ProjectID: "p2", SessionID: "s", Page: "index.md", Content: "x", Tag: "DÉDUIT", Status: domain.WikiRefused, Reason: "non"}
	require.NoError(t, ws.AddWikiProposal(ctx, q, 0, 0))
	got, err = ws.GetWikiProposal(ctx, "p2", q.Seq)
	require.NoError(t, err)
	assert.Equal(t, "non", got.Reason)
}

// v49: the column is added with an empty default and dropped by the
// rollback; the proposals stay readable on both sides (fixture: a database
// as left by v48).
func TestMigrationV49WikiProposalReason(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	ws := NewWikiProposalStore(s)
	require.NoError(t, s.MigrateDown(48))
	assert.False(t, hasColumn(t, s, "wiki_proposals", "reason"), "rollback drops the column")
	version, err := s.SchemaVersion()
	require.NoError(t, err)
	assert.Equal(t, 48, version)
	_, err = s.DB().Exec(`INSERT INTO wiki_proposals (project_id, seq, session_id, page, content, tag, status, created_at)
		VALUES ('p1', 1, 'ses_a', 'index.md', 'un', 'DÉDUIT', 'refused', ?)`, time.Now())
	require.NoError(t, err)

	require.NoError(t, s.migrate())
	assert.True(t, hasColumn(t, s, "wiki_proposals", "reason"), "v49 adds the column")
	got, err := ws.GetWikiProposal(ctx, "p1", 1)
	require.NoError(t, err)
	assert.Empty(t, got.Reason, "existing rows read an empty reason")
	assert.Equal(t, "un", got.Content)
	got.Reason = "non"
	require.NoError(t, ws.UpdateWikiProposal(ctx, got))
	require.NoError(t, s.migrate(), "idempotent")

	require.NoError(t, s.MigrateDown(48))
	assert.False(t, hasColumn(t, s, "wiki_proposals", "reason"))
	var content string
	require.NoError(t, s.DB().QueryRow(`SELECT content FROM wiki_proposals WHERE project_id = 'p1' AND seq = 1`).Scan(&content))
	assert.Equal(t, "un", content)
}
