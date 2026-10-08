// Package sessionresults keeps the results snapshot of a session (diff,
// branch, cost, tokens) under ~/.oh/sessions/<id>/results/, so that the
// recap stays available once the tool server sleeps or stops (P3-T11).
package sessionresults

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
)

// FileStat is one changed file (without its patch).
type FileStat struct {
	File      string `json:"file"`
	Status    string `json:"status,omitempty"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// Summary is the persisted results of a session.
type Summary struct {
	SessionID        string     `json:"session_id"`
	Title            string     `json:"title,omitempty"`
	Agent            string     `json:"agent,omitempty"`
	Branch           string     `json:"branch,omitempty"`
	Cost             float64    `json:"cost"`
	TokensIn         int64      `json:"tokens_in"`
	TokensOut        int64      `json:"tokens_out"`
	TokensReasoning  int64      `json:"tokens_reasoning,omitempty"`
	TokensCacheRead  int64      `json:"tokens_cache_read,omitempty"`
	TokensCacheWrite int64      `json:"tokens_cache_write,omitempty"`
	Files            []FileStat `json:"files,omitempty"`
	Additions        int        `json:"additions"`
	Deletions        int        `json:"deletions"`
	CapturedAt       time.Time  `json:"captured_at"`
}

// FromResult builds a summary from a tool result.
func FromResult(r adapters.SessionResult, at time.Time) Summary {
	s := Summary{
		SessionID: r.SessionID, Title: r.Title, Agent: r.Agent, Branch: r.Branch, Cost: r.Cost,
		TokensIn: r.TokensIn, TokensOut: r.TokensOut, TokensReasoning: r.TokensReasoning,
		TokensCacheRead: r.TokensCacheRead, TokensCacheWrite: r.TokensCacheWrite, CapturedAt: at,
	}
	for _, c := range r.Changes {
		s.Files = append(s.Files, FileStat{File: c.File, Status: c.Status, Additions: c.Additions, Deletions: c.Deletions})
		s.Additions += c.Additions
		s.Deletions += c.Deletions
	}
	return s
}

func dir(sessionsDir, id string) string { return filepath.Join(sessionsDir, id, "results") }

// Save writes summary.json and diff.patch (owner-only).
func Save(sessionsDir string, r adapters.SessionResult, at time.Time) error {
	if sessionsDir == "" || r.SessionID == "" {
		return errors.New("sessionresults: sessions dir and session id are required")
	}
	return SaveSummary(sessionsDir, FromResult(r, at), Patch(r.Changes))
}

// SaveSummary writes a summary and its patch (owner-only).
func SaveSummary(sessionsDir string, sum Summary, patch string) error {
	if sessionsDir == "" || sum.SessionID == "" {
		return errors.New("sessionresults: sessions dir and session id are required")
	}
	d := dir(sessionsDir, sum.SessionID)
	if err := os.MkdirAll(d, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(sum, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(d, "summary.json"), data, 0o600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d, "diff.patch"), []byte(patch), 0o600)
}

// Patch concatenates the patches of the changed files.
func Patch(changes []adapters.FileChange) string {
	var b strings.Builder
	for _, c := range changes {
		if c.Patch == "" {
			continue
		}
		b.WriteString(c.Patch)
		if !strings.HasSuffix(c.Patch, "\n") {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// ErrNoSnapshot is returned when a session has no saved results.
var ErrNoSnapshot = errors.New("no saved results for this session")

// Load reads the saved summary and patch of a session.
func Load(sessionsDir, id string) (Summary, string, error) {
	var s Summary
	if sessionsDir == "" {
		return s, "", ErrNoSnapshot
	}
	data, err := os.ReadFile(filepath.Join(dir(sessionsDir, id), "summary.json"))
	if errors.Is(err, os.ErrNotExist) {
		return s, "", ErrNoSnapshot
	}
	if err != nil {
		return s, "", err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, "", fmt.Errorf("reading results of %s: %w", id, err)
	}
	patch, _ := os.ReadFile(filepath.Join(dir(sessionsDir, id), "diff.patch"))
	return s, string(patch), nil
}
