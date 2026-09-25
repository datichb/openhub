package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
)

// SessionStore implements domain.SessionStore backed by SQLite.
type SessionStore struct {
	db *sql.DB
}

// NewSessionStore creates a SessionStore from a shared Store.
func NewSessionStore(s *Store) *SessionStore {
	return &SessionStore{db: s.DB()}
}

// Ensure interface compliance at compile time.
var _ domain.SessionStore = (*SessionStore)(nil)

// sessionColumns is the canonical column list used by all SELECT queries.
const sessionColumns = `id, project_id, started_at, ended_at, status, provider, model, tokens_in, tokens_out, launch_path, member_id, cost, tokens_reasoning, tokens_cache_read, platform, external_session_id, slug`

// scanSession scans a row into a domain.Session. The row must match sessionColumns order.
func scanSession(scanner interface{ Scan(...any) error }) (domain.Session, error) {
	var s domain.Session
	var status string
	var endedAt sql.NullTime
	var memberID sql.NullString
	var externalSessionID sql.NullString
	var slug sql.NullString
	if err := scanner.Scan(&s.ID, &s.ProjectID, &s.StartedAt, &endedAt, &status,
		&s.Provider, &s.Model, &s.TokensIn, &s.TokensOut, &s.LaunchPath, &memberID,
		&s.Cost, &s.TokensReasoning, &s.TokensCacheRead, &s.Platform, &externalSessionID, &slug); err != nil {
		return s, err
	}
	s.Status = domain.SessionStatus(status)
	if endedAt.Valid {
		s.EndedAt = &endedAt.Time
	}
	if memberID.Valid {
		s.MemberID = &memberID.String
	}
	if externalSessionID.Valid {
		s.ExternalSessionID = &externalSessionID.String
	}
	if slug.Valid {
		s.Slug = &slug.String
	}
	return s, nil
}

func (ss *SessionStore) List(ctx context.Context, projectID string) ([]domain.Session, error) {
	query := `SELECT ` + sessionColumns + ` FROM sessions`
	var args []interface{}
	if projectID != "" {
		query += " WHERE project_id = ?"
		args = append(args, projectID)
	}
	query += " ORDER BY started_at DESC"

	rows, err := ss.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}
	defer rows.Close()

	var sessions []domain.Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning session: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (ss *SessionStore) Get(ctx context.Context, id string) (*domain.Session, error) {
	row := ss.db.QueryRowContext(ctx,
		`SELECT `+sessionColumns+` FROM sessions WHERE id = ?`, id)
	s, err := scanSession(row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting session %s: %w", id, err)
	}
	return &s, nil
}

func (ss *SessionStore) Create(ctx context.Context, s *domain.Session) error {
	if s.StartedAt.IsZero() {
		s.StartedAt = time.Now()
	}
	if s.Platform == "" {
		s.Platform = "opencode"
	}
	_, err := ss.db.ExecContext(ctx,
		`INSERT INTO sessions (id, project_id, started_at, ended_at, status, provider, model, tokens_in, tokens_out, launch_path, member_id, cost, tokens_reasoning, tokens_cache_read, platform, external_session_id, slug)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.ProjectID, s.StartedAt, s.EndedAt, string(s.Status),
		s.Provider, s.Model, s.TokensIn, s.TokensOut, s.LaunchPath, s.MemberID,
		s.Cost, s.TokensReasoning, s.TokensCacheRead, s.Platform, s.ExternalSessionID, s.Slug,
	)
	if err != nil {
		return fmt.Errorf("creating session: %w", err)
	}
	return nil
}

func (ss *SessionStore) Update(ctx context.Context, s *domain.Session) error {
	result, err := ss.db.ExecContext(ctx,
		`UPDATE sessions SET ended_at=?, status=?, provider=?, model=?, tokens_in=?, tokens_out=?, launch_path=?, member_id=?, cost=?, tokens_reasoning=?, tokens_cache_read=?, platform=?, external_session_id=?, slug=?
		 WHERE id=?`,
		s.EndedAt, string(s.Status), s.Provider, s.Model, s.TokensIn, s.TokensOut, s.LaunchPath, s.MemberID,
		s.Cost, s.TokensReasoning, s.TokensCacheRead, s.Platform, s.ExternalSessionID, s.Slug,
		s.ID,
	)
	if err != nil {
		return fmt.Errorf("updating session %s: %w", s.ID, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (ss *SessionStore) ListRunning(ctx context.Context, projectID string) ([]domain.Session, error) {
	query := `SELECT ` + sessionColumns + ` FROM sessions WHERE status = 'running'`
	var args []interface{}
	if projectID != "" {
		query += " AND project_id = ?"
		args = append(args, projectID)
	}
	query += " ORDER BY started_at DESC"

	rows, err := ss.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing running sessions: %w", err)
	}
	defer rows.Close()

	var sessions []domain.Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning running session: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}
