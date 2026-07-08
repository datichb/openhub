package teamstate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendEventWritesFile(t *testing.T) {
	repo := setupTestRepo(t)

	// Create project events dir
	dir := filepath.Join(repo.path, "projects", "T-SRU", "events")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	ts := time.Date(2026, 7, 7, 14, 30, 0, 0, time.UTC)
	e := Event{
		Timestamp: ts,
		Actor:     "benjamin",
		Type:      EventSessionComplete,
		Project:   "T-SRU",
		Ticket:    "SRU-142",
		Data:      map[string]interface{}{"duration_min": float64(75)},
	}

	// Write directly to file (skip git for unit test)
	month := ts.Format("2006-01")
	path := filepath.Join(dir, month+".jsonl")
	line, err := json.Marshal(e)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, append(line, '\n'), 0o644))

	// Read it back
	events, err := repo.ListEvents("T-SRU", time.Time{})
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, EventSessionComplete, events[0].Type)
	assert.Equal(t, "benjamin", events[0].Actor)
	assert.Equal(t, "SRU-142", events[0].Ticket)
}

func TestListEventsFilterByDate(t *testing.T) {
	repo := setupTestRepo(t)

	dir := filepath.Join(repo.path, "projects", "T-SRU", "events")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	// Write events for July
	events := []Event{
		{Timestamp: time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), Actor: "alice", Type: EventClaimTaken, Project: "T-SRU"},
		{Timestamp: time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC), Actor: "benjamin", Type: EventSessionComplete, Project: "T-SRU"},
		{Timestamp: time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC), Actor: "charlie", Type: EventReviewReady, Project: "T-SRU"},
	}

	path := filepath.Join(dir, "2026-07.jsonl")
	f, err := os.Create(path)
	require.NoError(t, err)
	for _, e := range events {
		line, _ := json.Marshal(e)
		f.Write(append(line, '\n'))
	}
	f.Close()

	// Filter: only events since July 4
	since := time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC)
	result, err := repo.ListEvents("T-SRU", since)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	// Newest first
	assert.Equal(t, "charlie", result[0].Actor)
	assert.Equal(t, "benjamin", result[1].Actor)
}

func TestListEventsMultipleMonths(t *testing.T) {
	repo := setupTestRepo(t)

	dir := filepath.Join(repo.path, "projects", "T-SRU", "events")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	// June event
	e1 := Event{Timestamp: time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC), Actor: "alice", Type: EventClaimTaken, Project: "T-SRU"}
	line1, _ := json.Marshal(e1)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "2026-06.jsonl"), append(line1, '\n'), 0o644))

	// July event
	e2 := Event{Timestamp: time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), Actor: "benjamin", Type: EventSessionComplete, Project: "T-SRU"}
	line2, _ := json.Marshal(e2)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "2026-07.jsonl"), append(line2, '\n'), 0o644))

	result, err := repo.ListEvents("T-SRU", time.Time{})
	require.NoError(t, err)
	assert.Len(t, result, 2)
	// Newest first
	assert.Equal(t, "benjamin", result[0].Actor)
	assert.Equal(t, "alice", result[1].Actor)
}

func TestListEventsAllProjects(t *testing.T) {
	repo := setupTestRepo(t)

	// Events in two projects
	dir1 := filepath.Join(repo.path, "projects", "T-SRU", "events")
	dir2 := filepath.Join(repo.path, "projects", "OTHER", "events")
	require.NoError(t, os.MkdirAll(dir1, 0o755))
	require.NoError(t, os.MkdirAll(dir2, 0o755))

	e1 := Event{Timestamp: time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC), Actor: "alice", Type: EventClaimTaken}
	e2 := Event{Timestamp: time.Date(2026, 7, 7, 11, 0, 0, 0, time.UTC), Actor: "benjamin", Type: EventSessionComplete}

	line1, _ := json.Marshal(e1)
	line2, _ := json.Marshal(e2)
	require.NoError(t, os.WriteFile(filepath.Join(dir1, "2026-07.jsonl"), append(line1, '\n'), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "2026-07.jsonl"), append(line2, '\n'), 0o644))

	// Empty project = all
	result, err := repo.ListEvents("", time.Time{})
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestListEventsLimited(t *testing.T) {
	repo := setupTestRepo(t)

	dir := filepath.Join(repo.path, "projects", "T-SRU", "events")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	// Write 5 events
	f, err := os.Create(filepath.Join(dir, "2026-07.jsonl"))
	require.NoError(t, err)
	for i := range 5 {
		e := Event{
			Timestamp: time.Date(2026, 7, 1+i, 10, 0, 0, 0, time.UTC),
			Actor:     "dev",
			Type:      EventClaimTaken,
			Project:   "T-SRU",
		}
		line, _ := json.Marshal(e)
		f.Write(append(line, '\n'))
	}
	f.Close()

	result, err := repo.ListEventsLimited("T-SRU", 3)
	require.NoError(t, err)
	assert.Len(t, result, 3)
}

func TestListEventsEmpty(t *testing.T) {
	repo := setupTestRepo(t)

	result, err := repo.ListEvents("NONEXISTENT", time.Time{})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestListEventsSkipsMalformed(t *testing.T) {
	repo := setupTestRepo(t)

	dir := filepath.Join(repo.path, "projects", "T-SRU", "events")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	content := `{"ts":"2026-07-07T10:00:00Z","actor":"alice","event":"claim.taken","project":"T-SRU"}
this is not json
{"ts":"2026-07-07T11:00:00Z","actor":"benjamin","event":"session.complete","project":"T-SRU"}
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "2026-07.jsonl"), []byte(content), 0o644))

	result, err := repo.ListEvents("T-SRU", time.Time{})
	require.NoError(t, err)
	// Malformed line is skipped
	assert.Len(t, result, 2)
}
