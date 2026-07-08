package teamstate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	toml "github.com/pelletier/go-toml/v2"
)

const maxProposalLines = 200

// WikiProposal represents a pending wiki contribution awaiting human review.
type WikiProposal struct {
	ID         string    `toml:"id"`
	Page       string    `toml:"page"`       // target page name (without .md)
	Content    string    `toml:"content"`    // markdown content to add
	Confidence string    `toml:"confidence"` // CONFIRMED | INFERRED | UNCERTAIN
	Author     string    `toml:"author"`     // agent that proposed (e.g. "documentarian")
	Project    string    `toml:"project"`    // originating project
	CreatedAt  time.Time `toml:"created_at"`
}

// WikiListPages returns the names of all wiki pages (without .md extension).
func (r *Repo) WikiListPages() ([]string, error) {
	dir := filepath.Join(r.path, "wiki")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading wiki dir: %w", err)
	}

	var pages []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		pages = append(pages, strings.TrimSuffix(e.Name(), ".md"))
	}
	return pages, nil
}

// WikiReadPage returns the content of a wiki page.
func (r *Repo) WikiReadPage(name string) (string, error) {
	path := filepath.Join(r.path, "wiki", name+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrWikiPageNotFound
		}
		return "", fmt.Errorf("reading wiki page %s: %w", name, err)
	}
	return string(data), nil
}

// WikiCreateProposal creates a new pending proposal for human review.
func (r *Repo) WikiCreateProposal(ctx context.Context, p WikiProposal) error {
	// Validate
	if err := validateProposal(p); err != nil {
		return err
	}

	// Generate ID if not set
	if p.ID == "" {
		p.ID = uuid.New().String()[:8]
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}

	// Ensure pending directory
	dir := filepath.Join(r.path, "wiki", ".pending")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating pending dir: %w", err)
	}

	// Write proposal file
	data, err := toml.Marshal(&p)
	if err != nil {
		return fmt.Errorf("marshaling proposal: %w", err)
	}

	filename := fmt.Sprintf("%s-%s.toml", p.CreatedAt.Format("2006-01-02T15h04"), p.Page)
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing proposal: %w", err)
	}

	// Commit and push
	relPath := filepath.Join("wiki", ".pending", filename)
	msg := fmt.Sprintf("wiki: proposal for %s by %s (from %s)", p.Page, p.Author, p.Project)
	return r.CommitAndPush(ctx, msg, relPath)
}

// WikiListPending returns all pending proposals.
func (r *Repo) WikiListPending() ([]WikiProposal, error) {
	dir := filepath.Join(r.path, "wiki", ".pending")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading pending dir: %w", err)
	}

	var proposals []WikiProposal
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var p WikiProposal
		if err := toml.Unmarshal(data, &p); err != nil {
			continue
		}
		proposals = append(proposals, p)
	}
	return proposals, nil
}

// WikiAcceptProposal merges a proposal into the target page and removes it from pending.
func (r *Repo) WikiAcceptProposal(ctx context.Context, id string) error {
	proposal, filename, err := r.findPendingByID(id)
	if err != nil {
		return err
	}

	// Append to target page (create if doesn't exist)
	pagePath := filepath.Join(r.path, "wiki", proposal.Page+".md")
	f, err := os.OpenFile(pagePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening wiki page: %w", err)
	}
	content := "\n\n" + strings.TrimSpace(proposal.Content) + "\n"
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		return fmt.Errorf("writing to wiki page: %w", err)
	}
	f.Close()

	// Remove pending file
	pendingPath := filepath.Join(r.path, "wiki", ".pending", filename)
	if err := os.Remove(pendingPath); err != nil {
		return fmt.Errorf("removing pending file: %w", err)
	}

	// Commit and push
	pageRelPath := filepath.Join("wiki", proposal.Page+".md")
	pendingRelPath := filepath.Join("wiki", ".pending", filename)
	msg := fmt.Sprintf("wiki: accepted proposal %s for %s", id, proposal.Page)
	return r.CommitAndPush(ctx, msg, pageRelPath, pendingRelPath)
}

// WikiRejectProposal removes a pending proposal without merging.
func (r *Repo) WikiRejectProposal(ctx context.Context, id string) error {
	_, filename, err := r.findPendingByID(id)
	if err != nil {
		return err
	}

	pendingPath := filepath.Join(r.path, "wiki", ".pending", filename)
	if err := os.Remove(pendingPath); err != nil {
		return fmt.Errorf("removing pending file: %w", err)
	}

	relPath := filepath.Join("wiki", ".pending", filename)
	msg := fmt.Sprintf("wiki: rejected proposal %s", id)
	return r.CommitAndPush(ctx, msg, relPath)
}

func (r *Repo) findPendingByID(id string) (*WikiProposal, string, error) {
	dir := filepath.Join(r.path, "wiki", ".pending")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, "", ErrProposalNotFound
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var p WikiProposal
		if err := toml.Unmarshal(data, &p); err != nil {
			continue
		}
		if p.ID == id {
			return &p, e.Name(), nil
		}
	}
	return nil, "", ErrProposalNotFound
}

func validateProposal(p WikiProposal) error {
	if p.Page == "" {
		return fmt.Errorf("%w: page is required", ErrProposalInvalid)
	}
	if p.Content == "" {
		return fmt.Errorf("%w: content is required", ErrProposalInvalid)
	}
	if p.Confidence == "" {
		return fmt.Errorf("%w: confidence tag is required (CONFIRMED, INFERRED, or UNCERTAIN)", ErrProposalInvalid)
	}
	validConfidence := p.Confidence == "CONFIRMED" || p.Confidence == "INFERRED" || p.Confidence == "UNCERTAIN"
	if !validConfidence {
		return fmt.Errorf("%w: confidence must be CONFIRMED, INFERRED, or UNCERTAIN", ErrProposalInvalid)
	}
	if p.Author == "" {
		return fmt.Errorf("%w: author is required", ErrProposalInvalid)
	}

	// Check size
	lines := strings.Count(p.Content, "\n") + 1
	if lines > maxProposalLines {
		return fmt.Errorf("%w: %d lines exceeds max %d", ErrProposalTooLarge, lines, maxProposalLines)
	}

	return nil
}
