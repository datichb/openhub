package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
)

// ReportStore implements domain.ReportStore on session_reports (v44).
type ReportStore struct {
	db *sql.DB
}

// NewReportStore creates a ReportStore from a shared Store.
func NewReportStore(s *Store) *ReportStore { return &ReportStore{db: s.DB()} }

var _ domain.ReportStore = (*ReportStore)(nil)

const reportColumns = `session_id, seq, agent, tool_session, title, bytes, sha256, sections, created_at`

func sectionsJSON(s []string) string {
	if len(s) == 0 {
		return ""
	}
	data, _ := json.Marshal(s)
	return string(data)
}

// AddReport implements domain.ReportStore: number and limits are read and
// the row written in one write transaction (two concurrent reports never
// get the same number nor pass the limits together).
func (s *ReportStore) AddReport(ctx context.Context, r *domain.Report, maxCount int, maxBytes int64) error {
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return fmt.Errorf("adding a report: %w", err)
	}
	done := false
	defer func() {
		if !done {
			_, _ = conn.ExecContext(context.WithoutCancel(ctx), `ROLLBACK`)
		}
	}()
	var count, last int
	var total int64
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(MAX(seq), 0), COALESCE(SUM(bytes), 0) FROM session_reports WHERE session_id = ?`,
		r.SessionID).Scan(&count, &last, &total); err != nil {
		return fmt.Errorf("adding a report: %w", err)
	}
	if (maxCount > 0 && count+1 > maxCount) || (maxBytes > 0 && total+int64(r.Bytes) > maxBytes) {
		return domain.ErrReportLimit
	}
	r.Seq = last + 1
	if _, err := conn.ExecContext(ctx, `INSERT INTO session_reports (`+reportColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.SessionID, r.Seq, r.Agent, r.ToolSession, r.Title, r.Bytes, r.SHA256, sectionsJSON(r.Sections), r.CreatedAt); err != nil {
		if strings.Contains(err.Error(), "FOREIGN KEY") {
			return fmt.Errorf("session %s: %w", r.SessionID, domain.ErrNotFound)
		}
		return fmt.Errorf("adding a report: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return fmt.Errorf("adding a report: %w", err)
	}
	done = true
	return nil
}

// ImportReport implements domain.ReportStore.
func (s *ReportStore) ImportReport(ctx context.Context, r *domain.Report) (bool, error) {
	res, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO session_reports (`+reportColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.SessionID, r.Seq, r.Agent, r.ToolSession, r.Title, r.Bytes, r.SHA256, sectionsJSON(r.Sections), r.CreatedAt)
	if err != nil {
		return false, fmt.Errorf("importing a report: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// DeleteReport implements domain.ReportStore.
func (s *ReportStore) DeleteReport(ctx context.Context, sessionID string, seq int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM session_reports WHERE session_id = ? AND seq = ?`, sessionID, seq)
	return err
}

func scanReport(sc interface{ Scan(...any) error }) (domain.Report, error) {
	var r domain.Report
	var sections string
	if err := sc.Scan(&r.SessionID, &r.Seq, &r.Agent, &r.ToolSession, &r.Title, &r.Bytes, &r.SHA256, &sections, &r.CreatedAt); err != nil {
		return r, err
	}
	if sections != "" {
		_ = json.Unmarshal([]byte(sections), &r.Sections)
	}
	return r, nil
}

// ListReports implements domain.ReportStore.
func (s *ReportStore) ListReports(ctx context.Context, sessionID string) ([]domain.Report, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+reportColumns+` FROM session_reports WHERE session_id = ? ORDER BY seq`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("listing reports: %w", err)
	}
	defer rows.Close()
	var out []domain.Report
	for rows.Next() {
		r, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetReport implements domain.ReportStore.
func (s *ReportStore) GetReport(ctx context.Context, sessionID string, seq int) (*domain.Report, error) {
	r, err := scanReport(s.db.QueryRowContext(ctx, `SELECT `+reportColumns+` FROM session_reports WHERE session_id = ? AND seq = ?`, sessionID, seq))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}
