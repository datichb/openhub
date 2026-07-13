package teamstate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

// TakeoverBrief holds the raw data collected during a claim transfer.
type TakeoverBrief struct {
	Meta     TakeoverMeta       `toml:"meta"`
	Activity TakeoverActivity   `toml:"activity"`
	Git      TakeoverGit        `toml:"git"`
	Events   []TakeoverEvent    `toml:"events"`
}

// TakeoverMeta holds transfer metadata.
type TakeoverMeta struct {
	TicketID        string    `toml:"ticket_id"`
	Project         string    `toml:"project"`
	TransferredFrom string    `toml:"transferred_from"`
	TransferredTo   string    `toml:"transferred_to"`
	TransferDate    time.Time `toml:"transfer_date"`
	Reason          string    `toml:"reason"` // transfer | stale
	StaleDays       int       `toml:"stale_days,omitempty"`
}

// TakeoverActivity holds session activity summary.
type TakeoverActivity struct {
	SessionsCount        int       `toml:"sessions_count"`
	FirstSession         time.Time `toml:"first_session,omitempty"`
	LastSession          time.Time `toml:"last_session,omitempty"`
	TotalDurationMinutes int       `toml:"total_duration_minutes"`
}

// TakeoverGit holds git-related context.
type TakeoverGit struct {
	Branch            string               `toml:"branch"`
	CommitsCount      int                  `toml:"commits_count"`
	LastCommitMessage string               `toml:"last_commit_message"`
	LastCommitDate    time.Time            `toml:"last_commit_date,omitempty"`
	FilesModified     []TakeoverFileChange `toml:"files_modified"`
	FilesCreated      []TakeoverFileChange `toml:"files_created"`
}

// TakeoverFileChange represents a file touched during the work.
type TakeoverFileChange struct {
	Path      string `toml:"path"`
	Additions int    `toml:"additions"`
	Deletions int    `toml:"deletions,omitempty"`
}

// TakeoverEvent is a simplified event for the brief.
type TakeoverEvent struct {
	Timestamp time.Time `toml:"ts"`
	Type      string    `toml:"type"`
	Summary   string    `toml:"summary"`
}

// GenerateRawBrief collects all available data for a takeover brief.
// It reads events from the team-state and produces a raw .toml brief.
func (r *Repo) GenerateRawBrief(ctx context.Context, project, ticketID, from, to, reason string) (*TakeoverBrief, error) {
	now := time.Now().UTC()

	brief := &TakeoverBrief{
		Meta: TakeoverMeta{
			TicketID:        ticketID,
			Project:         project,
			TransferredFrom: from,
			TransferredTo:   to,
			TransferDate:    now,
			Reason:          reason,
		},
	}

	// Collect events for this ticket
	events, err := r.ListEvents(project, time.Time{})
	if err == nil {
		var ticketEvents []TakeoverEvent
		for _, e := range events {
			if e.Ticket == ticketID || (e.Actor == from && e.Type == EventSessionComplete) {
				ticketEvents = append(ticketEvents, TakeoverEvent{
					Timestamp: e.Timestamp,
					Type:      e.Type,
					Summary:   formatEventSummary(e),
				})
			}
		}
		brief.Events = ticketEvents

		// Calculate activity from events
		if len(ticketEvents) > 0 {
			brief.Activity.SessionsCount = len(ticketEvents)
			brief.Activity.FirstSession = ticketEvents[len(ticketEvents)-1].Timestamp // events are newest-first
			brief.Activity.LastSession = ticketEvents[0].Timestamp
		}
	}

	// Calculate stale days if reason is stale
	if reason == "stale" {
		claim, _ := r.GetClaim(project, ticketID)
		if claim != nil {
			lastActive := claim.LastActivity
			if lastActive.IsZero() {
				lastActive = claim.ClaimedAt
			}
			brief.Meta.StaleDays = int(now.Sub(lastActive).Hours() / 24)
		}
	}

	return brief, nil
}

// SaveBrief writes a takeover brief to the team-state repo.
func (r *Repo) SaveBrief(ctx context.Context, brief *TakeoverBrief) error {
	dir := filepath.Join(r.path, "projects", brief.Meta.Project, "takeover-briefs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating takeover-briefs dir: %w", err)
	}

	// Write raw TOML
	date := brief.Meta.TransferDate.Format("2006-01-02")
	baseName := fmt.Sprintf("%s_%s", brief.Meta.TicketID, date)
	tomlPath := filepath.Join(dir, baseName+".toml")

	data, err := toml.Marshal(brief)
	if err != nil {
		return fmt.Errorf("marshaling brief: %w", err)
	}
	if err := os.WriteFile(tomlPath, data, 0o644); err != nil {
		return fmt.Errorf("writing brief .toml: %w", err)
	}

	// Write template markdown
	mdContent := RenderTemplateBrief(brief)
	mdPath := filepath.Join(dir, baseName+".md")
	if err := os.WriteFile(mdPath, []byte(mdContent), 0o644); err != nil {
		return fmt.Errorf("writing brief .md: %w", err)
	}

	// Commit and push
	relToml := filepath.Join("projects", brief.Meta.Project, "takeover-briefs", baseName+".toml")
	relMd := filepath.Join("projects", brief.Meta.Project, "takeover-briefs", baseName+".md")
	msg := fmt.Sprintf("takeover: brief for %s/%s (%s → %s)",
		brief.Meta.Project, brief.Meta.TicketID,
		brief.Meta.TransferredFrom, brief.Meta.TransferredTo)
	return r.CommitAndPush(ctx, msg, relToml, relMd)
}

// ReadBrief reads the best available brief for a ticket.
// Priority: .enriched.md > .md > .toml
func (r *Repo) ReadBrief(project, ticketID string) (string, error) {
	dir := filepath.Join(r.path, "projects", project, "takeover-briefs")

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrBriefNotFound
		}
		return "", fmt.Errorf("reading takeover-briefs dir: %w", err)
	}

	// Find the most recent brief for this ticket
	prefix := ticketID + "_"
	var candidates []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) {
			candidates = append(candidates, e.Name())
		}
	}

	if len(candidates) == 0 {
		return "", ErrBriefNotFound
	}

	// Sort to get the most recent date
	sort.Strings(candidates)
	latest := candidates[len(candidates)-1]
	baseName := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(latest, ".toml"), ".md"), ".enriched")

	// Try enriched first
	enrichedPath := filepath.Join(dir, baseName+".enriched.md")
	if data, err := os.ReadFile(enrichedPath); err == nil {
		return string(data), nil
	}

	// Try template md
	mdPath := filepath.Join(dir, baseName+".md")
	if data, err := os.ReadFile(mdPath); err == nil {
		return string(data), nil
	}

	// Fallback to toml
	tomlPath := filepath.Join(dir, baseName+".toml")
	if data, err := os.ReadFile(tomlPath); err == nil {
		return string(data), nil
	}

	return "", ErrBriefNotFound
}

// ListBriefs returns all briefs for a project.
func (r *Repo) ListBriefs(project string) ([]TakeoverMeta, error) {
	dir := filepath.Join(r.path, "projects", project, "takeover-briefs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var metas []TakeoverMeta
	seen := map[string]bool{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var brief TakeoverBrief
		if err := toml.Unmarshal(data, &brief); err != nil {
			continue
		}
		key := brief.Meta.TicketID + brief.Meta.TransferDate.Format("2006-01-02")
		if seen[key] {
			continue
		}
		seen[key] = true
		metas = append(metas, brief.Meta)
	}
	return metas, nil
}

// BriefExists returns true if any brief exists for the given ticket.
func (r *Repo) BriefExists(project, ticketID string) bool {
	dir := filepath.Join(r.path, "projects", project, "takeover-briefs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	prefix := ticketID + "_"
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) {
			return true
		}
	}
	return false
}

// IsStale returns true if a claim has been inactive for longer than staleDays.
func (r *Repo) IsStale(claim *Claim, staleDays int) bool {
	if staleDays <= 0 {
		staleDays = 3
	}
	lastActive := claim.LastActivity
	if lastActive.IsZero() {
		lastActive = claim.ClaimedAt
	}
	return time.Since(lastActive) > time.Duration(staleDays)*24*time.Hour
}

func formatEventSummary(e Event) string {
	if data, ok := e.Data["summary"]; ok {
		if s, ok := data.(string); ok {
			return s
		}
	}
	if data, ok := e.Data["message"]; ok {
		if s, ok := data.(string); ok {
			return s
		}
	}
	return e.Type
}
