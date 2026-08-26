package teamstate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupBenchRepo creates a realistic team-state repo for benchmarking:
// - 10 members
// - 5 projects with 10 claims each (50 total)
// - 6 months of events (~600 events/month/project = ~18000 total)
func setupBenchRepo(b *testing.B) *Repo {
	b.Helper()
	dir := b.TempDir()

	// Create .git to pass IsCloned() check
	require.NoError(b, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))

	// Members
	var members string
	for i := 0; i < 10; i++ {
		members += fmt.Sprintf(`
[members.member%d]
display_name = "Member %d"
role = "dev"
default_mode = "manual"
`, i, i)
	}
	require.NoError(b, os.WriteFile(filepath.Join(dir, "members.toml"), []byte(members), 0o644))

	// Projects with claims and events
	statuses := []string{"planned", "in_progress", "review", "blocked", "done"}
	baseTime := time.Now().UTC()

	for p := 0; p < 5; p++ {
		project := fmt.Sprintf("project-%d", p)

		// Claims (10 per project)
		claimsDir := filepath.Join(dir, "projects", project, "claims")
		require.NoError(b, os.MkdirAll(claimsDir, 0o755))
		for c := 0; c < 10; c++ {
			claim := fmt.Sprintf(`claimed_by = "member%d"
claimed_at = %s
status = "%s"
worktree = "feat/TICKET-%d-%d"
`, c%10, baseTime.Add(-time.Duration(c)*time.Hour).Format("2006-01-02T15:04:05Z"),
				statuses[c%5], p, c)
			filename := fmt.Sprintf("TICKET-%d-%d.toml", p, c)
			require.NoError(b, os.WriteFile(filepath.Join(claimsDir, filename), []byte(claim), 0o644))
		}

		// Events (6 months × ~600 events = ~3600 per project)
		eventsDir := filepath.Join(dir, "projects", project, "events")
		require.NoError(b, os.MkdirAll(eventsDir, 0o755))
		eventTypes := []string{"claim.taken", "claim.released", "session.complete", "review.ready"}

		for month := 0; month < 6; month++ {
			monthTime := baseTime.AddDate(0, -month, 0)
			filename := monthTime.Format("2006-01") + ".jsonl"
			var content string
			for e := 0; e < 600; e++ {
				ts := monthTime.Add(-time.Duration(e) * 2 * time.Minute)
				event := Event{
					Timestamp: ts,
					Actor:     fmt.Sprintf("member%d", e%10),
					Type:      eventTypes[e%4],
					Project:   project,
					Ticket:    fmt.Sprintf("TICKET-%d-%d", p, e%10),
				}
				line, _ := json.Marshal(event)
				content += string(line) + "\n"
			}
			require.NoError(b, os.WriteFile(filepath.Join(eventsDir, filename), []byte(content), 0o644))
		}
	}

	return NewRepo("", dir)
}

// ─── Benchmarks ──────────────────────────────────────────────────────────────

func BenchmarkListEventsLimited_5(b *testing.B) {
	repo := setupBenchRepo(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		events, err := repo.ListEventsLimited("project-0", 5)
		if err != nil {
			b.Fatal(err)
		}
		if len(events) != 5 {
			b.Fatalf("expected 5 events, got %d", len(events))
		}
	}
}

func BenchmarkListEventsLimited_20(b *testing.B) {
	repo := setupBenchRepo(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		events, err := repo.ListEventsLimited("project-0", 20)
		if err != nil {
			b.Fatal(err)
		}
		if len(events) != 20 {
			b.Fatalf("expected 20 events, got %d", len(events))
		}
	}
}

func BenchmarkListEventsLimited_AllProjects_5(b *testing.B) {
	repo := setupBenchRepo(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		events, err := repo.ListEventsLimited("", 5)
		if err != nil {
			b.Fatal(err)
		}
		if len(events) != 5 {
			b.Fatalf("expected 5 events, got %d", len(events))
		}
	}
}

func BenchmarkListEvents_LastWeek(b *testing.B) {
	repo := setupBenchRepo(b)
	since := time.Now().UTC().AddDate(0, 0, -7)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := repo.ListEvents("project-0", since)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkListEvents_All(b *testing.B) {
	repo := setupBenchRepo(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := repo.ListEvents("project-0", time.Time{})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkListClaims_AllProjects(b *testing.B) {
	repo := setupBenchRepo(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		claims, err := repo.ListClaims("")
		if err != nil {
			b.Fatal(err)
		}
		if len(claims) != 50 {
			b.Fatalf("expected 50 claims, got %d", len(claims))
		}
	}
}

func BenchmarkListMembers(b *testing.B) {
	repo := setupBenchRepo(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		members, err := repo.ListMembers()
		if err != nil {
			b.Fatal(err)
		}
		if len(members) != 10 {
			b.Fatalf("expected 10 members, got %d", len(members))
		}
	}
}

// ─── Concurrency Tests ───────────────────────────────────────────────────────

func TestConcurrentClaims(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	// Pre-create the project dir
	dir := filepath.Join(repo.path, "projects", "T-CONC", "claims")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	// Spawn 5 goroutines, each creating a different claim
	var wg sync.WaitGroup
	errs := make([]error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := repo.CreateClaim(ctx, Claim{
				TicketID:  fmt.Sprintf("CONC-%d", idx),
				Project:   "T-CONC",
				ClaimedBy: fmt.Sprintf("member%d", idx),
				Status:    ClaimStatusInProgress,
			})
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	// All should succeed (different tickets)
	for i, err := range errs {
		assert.NoError(t, err, "claim %d failed", i)
	}

	// All claims should exist
	claims, err := repo.ListClaims("T-CONC")
	require.NoError(t, err)
	assert.Len(t, claims, 5)
}

func TestConcurrentReadWrite(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	// Pre-create events dir
	eventsDir := filepath.Join(repo.path, "projects", "T-RW", "events")
	require.NoError(t, os.MkdirAll(eventsDir, 0o755))

	// Write a seed event
	require.NoError(t, repo.AppendEvent(ctx, Event{
		Type:    EventClaimTaken,
		Project: "T-RW",
		Actor:   "seed",
		Ticket:  "RW-0",
	}))

	// Run concurrent reads and writes
	var wg sync.WaitGroup
	var writeErrs []error
	var mu sync.Mutex

	// 1 writer: appends events
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			err := repo.AppendEvent(ctx, Event{
				Type:    EventSessionComplete,
				Project: "T-RW",
				Actor:   fmt.Sprintf("writer%d", i),
				Ticket:  fmt.Sprintf("RW-%d", i),
			})
			if err != nil {
				mu.Lock()
				writeErrs = append(writeErrs, err)
				mu.Unlock()
			}
		}
	}()

	// 3 readers: list events
	for r := 0; r < 3; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 5; i++ {
				_, _ = repo.ListEventsLimited("T-RW", 10)
			}
		}()
	}

	wg.Wait()

	// No panics, no write errors
	assert.Empty(t, writeErrs, "write errors: %v", writeErrs)

	// Events should be readable
	events, err := repo.ListEventsLimited("T-RW", 20)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(events), 1, "should have at least the seed event")
}
