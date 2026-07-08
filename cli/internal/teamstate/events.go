package teamstate

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Event represents a team activity event stored in JSONL files.
type Event struct {
	Timestamp time.Time              `json:"ts"`
	Actor     string                 `json:"actor"`
	Type      string                 `json:"event"`
	Project   string                 `json:"project"`
	Ticket    string                 `json:"ticket,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// Event types.
const (
	EventSessionComplete  = "session.complete"
	EventReviewReady      = "review.ready"
	EventAuditFinding     = "audit.finding"
	EventClaimTaken       = "claim.taken"
	EventClaimConflict    = "claim.conflict"
	EventClaimTransferred = "claim.transferred"
	EventClaimReleased    = "claim.released"
	EventWikiProposal     = "wiki.proposal"
	EventWikiAccepted     = "wiki.accepted"
	EventWikiRejected     = "wiki.rejected"
)

// AppendEvent appends an event to the monthly JSONL file and pushes.
func (r *Repo) AppendEvent(ctx context.Context, e Event) error {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}

	// Determine file path: projects/<project>/events/YYYY-MM.jsonl
	month := e.Timestamp.Format("2006-01")
	dir := filepath.Join(r.path, "projects", e.Project, "events")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating events dir: %w", err)
	}

	filename := month + ".jsonl"
	path := filepath.Join(dir, filename)

	// Marshal event to JSON
	line, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}

	// Append to file
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening events file: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return fmt.Errorf("writing event: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing events file: %w", err)
	}

	// Commit and push
	relPath := filepath.Join("projects", e.Project, "events", filename)
	msg := fmt.Sprintf("event: %s by %s on %s", e.Type, e.Actor, e.Project)
	return r.CommitAndPush(ctx, msg, relPath)
}

// ListEvents returns events for a project since the given time, sorted newest first.
// If project is empty, returns events across all projects.
func (r *Repo) ListEvents(project string, since time.Time) ([]Event, error) {
	if project != "" {
		return r.listEventsForProject(project, since)
	}

	projectsDir := filepath.Join(r.path, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var all []Event
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		events, err := r.listEventsForProject(e.Name(), since)
		if err != nil {
			continue // skip broken projects
		}
		all = append(all, events...)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].Timestamp.After(all[j].Timestamp)
	})
	return all, nil
}

// ListEventsLimited returns at most limit events, newest first.
func (r *Repo) ListEventsLimited(project string, limit int) ([]Event, error) {
	// Use a zero time to get all events, then cap
	events, err := r.ListEvents(project, time.Time{})
	if err != nil {
		return nil, err
	}
	if len(events) > limit {
		events = events[:limit]
	}
	return events, nil
}

func (r *Repo) listEventsForProject(project string, since time.Time) ([]Event, error) {
	dir := filepath.Join(r.path, "projects", project, "events")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var events []Event
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		fileEvents, err := r.readEventsFile(filepath.Join(dir, entry.Name()), since)
		if err != nil {
			continue // skip malformed files
		}
		for i := range fileEvents {
			fileEvents[i].Project = project
		}
		events = append(events, fileEvents...)
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.After(events[j].Timestamp)
	})
	return events, nil
}

func (r *Repo) readEventsFile(path string, since time.Time) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			continue // skip malformed lines
		}
		if !since.IsZero() && e.Timestamp.Before(since) {
			continue
		}
		events = append(events, e)
	}
	return events, scanner.Err()
}
