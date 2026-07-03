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

func (ss *SessionStore) List(ctx context.Context, projectID string) ([]domain.Session, error) {
	query := `SELECT id, project_id, started_at, ended_at, status, provider, model, tokens_in, tokens_out FROM sessions`
	var args []interface{}
	if projectID != "" {
		query += " WHERE project_id = ?"
		args = append(args, projectID)
	}
	query += " ORDER BY started_at DESC"

	rows, err := ss.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}
	defer rows.Close()

	var sessions []domain.Session
	for rows.Next() {
		var s domain.Session
		var status string
		var endedAt sql.NullTime
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.StartedAt, &endedAt, &status,
			&s.Provider, &s.Model, &s.TokensIn, &s.TokensOut); err != nil {
			return nil, fmt.Errorf("scanning session: %w", err)
		}
		s.Status = domain.SessionStatus(status)
		if endedAt.Valid {
			s.EndedAt = &endedAt.Time
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (ss *SessionStore) Get(ctx context.Context, id string) (*domain.Session, error) {
	var s domain.Session
	var status string
	var endedAt sql.NullTime
	err := ss.db.QueryRow(
		`SELECT id, project_id, started_at, ended_at, status, provider, model, tokens_in, tokens_out FROM sessions WHERE id = ?`,
		id,
	).Scan(&s.ID, &s.ProjectID, &s.StartedAt, &endedAt, &status,
		&s.Provider, &s.Model, &s.TokensIn, &s.TokensOut)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting session %s: %w", id, err)
	}
	s.Status = domain.SessionStatus(status)
	if endedAt.Valid {
		s.EndedAt = &endedAt.Time
	}
	return &s, nil
}

func (ss *SessionStore) Create(ctx context.Context, s *domain.Session) error {
	if s.StartedAt.IsZero() {
		s.StartedAt = time.Now()
	}
	_, err := ss.db.Exec(
		`INSERT INTO sessions (id, project_id, started_at, ended_at, status, provider, model, tokens_in, tokens_out)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.ProjectID, s.StartedAt, s.EndedAt, string(s.Status),
		s.Provider, s.Model, s.TokensIn, s.TokensOut,
	)
	if err != nil {
		return fmt.Errorf("creating session: %w", err)
	}
	return nil
}

func (ss *SessionStore) Update(ctx context.Context, s *domain.Session) error {
	result, err := ss.db.Exec(
		`UPDATE sessions SET ended_at=?, status=?, provider=?, model=?, tokens_in=?, tokens_out=?
		 WHERE id=?`,
		s.EndedAt, string(s.Status), s.Provider, s.Model, s.TokensIn, s.TokensOut, s.ID,
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
