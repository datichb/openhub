package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
)

// WikiProposalStore implements domain.WikiProposalStore on wiki_proposals
// (v45; replaces: v48; reason: v49).
type WikiProposalStore struct {
	db *sql.DB
}

// NewWikiProposalStore creates a WikiProposalStore from a shared Store.
func NewWikiProposalStore(s *Store) *WikiProposalStore { return &WikiProposalStore{db: s.DB()} }

var _ domain.WikiProposalStore = (*WikiProposalStore)(nil)

const wikiColumns = `project_id, seq, session_id, tool_session, agent, page, section, content, tag, source, status, original,
	decided_by, decided_at, applied_by, applied_at, error, created_at, replaces, reason`

// AddWikiProposal implements domain.WikiProposalStore: number and limits
// are read and the row written in one write transaction.
func (s *WikiProposalStore) AddWikiProposal(ctx context.Context, p *domain.WikiProposal, maxSession, maxProject int) error {
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	if p.Status == "" {
		p.Status = domain.WikiPending
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return fmt.Errorf("adding a wiki proposal: %w", err)
	}
	done := false
	defer func() {
		if !done {
			_, _ = conn.ExecContext(context.WithoutCancel(ctx), `ROLLBACK`)
		}
	}()
	var last, openProject, openSession int
	if err := conn.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq), 0),
		COALESCE(SUM(CASE WHEN status IN ('pending', 'accepted') THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN status IN ('pending', 'accepted') AND session_id = ? THEN 1 ELSE 0 END), 0)
		FROM wiki_proposals WHERE project_id = ?`, p.SessionID, p.ProjectID).Scan(&last, &openProject, &openSession); err != nil {
		return fmt.Errorf("adding a wiki proposal: %w", err)
	}
	if (maxProject > 0 && openProject+1 > maxProject) || (maxSession > 0 && openSession+1 > maxSession) {
		return domain.ErrWikiLimit
	}
	p.Seq = last + 1
	if _, err := conn.ExecContext(ctx, `INSERT INTO wiki_proposals (`+wikiColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ProjectID, p.Seq, p.SessionID, p.ToolSession, p.Agent, p.Page, p.Section, p.Content, p.Tag, p.Source, p.Status, p.Original,
		p.DecidedBy, p.DecidedAt, p.AppliedBy, p.AppliedAt, p.Error, p.CreatedAt, p.Replaces, p.Reason); err != nil {
		return fmt.Errorf("adding a wiki proposal: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return fmt.Errorf("adding a wiki proposal: %w", err)
	}
	done = true
	return nil
}

func scanWikiProposal(sc interface{ Scan(...any) error }) (domain.WikiProposal, error) {
	var p domain.WikiProposal
	var decided, applied sql.NullTime
	if err := sc.Scan(&p.ProjectID, &p.Seq, &p.SessionID, &p.ToolSession, &p.Agent, &p.Page, &p.Section, &p.Content, &p.Tag, &p.Source,
		&p.Status, &p.Original, &p.DecidedBy, &decided, &p.AppliedBy, &applied, &p.Error, &p.CreatedAt, &p.Replaces, &p.Reason); err != nil {
		return p, err
	}
	if decided.Valid {
		t := decided.Time
		p.DecidedAt = &t
	}
	if applied.Valid {
		t := applied.Time
		p.AppliedAt = &t
	}
	return p, nil
}

// ListWikiProposals implements domain.WikiProposalStore.
func (s *WikiProposalStore) ListWikiProposals(ctx context.Context, f domain.WikiProposalFilter) ([]domain.WikiProposal, error) {
	var where []string
	var args []any
	if f.ProjectID != "" {
		where, args = append(where, "project_id = ?"), append(args, f.ProjectID)
	}
	if f.SessionID != "" {
		where, args = append(where, "session_id = ?"), append(args, f.SessionID)
	}
	if len(f.Statuses) > 0 {
		marks := strings.TrimSuffix(strings.Repeat("?, ", len(f.Statuses)), ", ")
		where = append(where, "status IN ("+marks+")")
		for _, st := range f.Statuses {
			args = append(args, st)
		}
	}
	q := `SELECT ` + wikiColumns + ` FROM wiki_proposals`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	rows, err := s.db.QueryContext(ctx, q+` ORDER BY created_at, project_id, seq`, args...)
	if err != nil {
		return nil, fmt.Errorf("listing wiki proposals: %w", err)
	}
	defer rows.Close()
	var out []domain.WikiProposal
	for rows.Next() {
		p, err := scanWikiProposal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetWikiProposal implements domain.WikiProposalStore.
func (s *WikiProposalStore) GetWikiProposal(ctx context.Context, projectID string, seq int) (*domain.WikiProposal, error) {
	p, err := scanWikiProposal(s.db.QueryRowContext(ctx, `SELECT `+wikiColumns+` FROM wiki_proposals WHERE project_id = ? AND seq = ?`, projectID, seq))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdateWikiProposal implements domain.WikiProposalStore.
func (s *WikiProposalStore) UpdateWikiProposal(ctx context.Context, p *domain.WikiProposal) error {
	res, err := s.db.ExecContext(ctx, `UPDATE wiki_proposals SET status = ?, content = ?, original = ?, decided_by = ?, decided_at = ?,
		applied_by = ?, applied_at = ?, error = ?, reason = ? WHERE project_id = ? AND seq = ?`,
		p.Status, p.Content, p.Original, p.DecidedBy, p.DecidedAt, p.AppliedBy, p.AppliedAt, p.Error, p.Reason, p.ProjectID, p.Seq)
	if err != nil {
		return fmt.Errorf("updating a wiki proposal: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}
