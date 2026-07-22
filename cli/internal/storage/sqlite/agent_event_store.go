package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
)

// AgentEventStore implements domain.AgentEventStore backed by SQLite.
type AgentEventStore struct {
	db *sql.DB
}

// NewAgentEventStore creates an AgentEventStore from a shared Store.
func NewAgentEventStore(s *Store) *AgentEventStore {
	return &AgentEventStore{db: s.DB()}
}

// Ensure interface compliance at compile time.
var _ domain.AgentEventStore = (*AgentEventStore)(nil)

func (a *AgentEventStore) Create(ctx context.Context, e *domain.AgentEvent) error {
	if e.StartedAt.IsZero() {
		e.StartedAt = time.Now()
	}
	skillsJSON, _ := json.Marshal(e.SkillsLoaded)
	_, err := a.db.ExecContext(ctx,
		`INSERT INTO agent_events
		 (id, session_id, project_id, agent_name, skills_loaded, started_at, completed_at, status, tokens_in, tokens_out, cost_usd, error_message)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.SessionID, e.ProjectID, e.AgentName, string(skillsJSON),
		e.StartedAt, e.CompletedAt, string(e.Status),
		e.TokensIn, e.TokensOut, e.CostUSD, e.ErrorMessage,
	)
	if err != nil {
		return fmt.Errorf("creating agent event: %w", err)
	}
	return nil
}

func (a *AgentEventStore) Update(ctx context.Context, e *domain.AgentEvent) error {
	skillsJSON, _ := json.Marshal(e.SkillsLoaded)
	result, err := a.db.ExecContext(ctx,
		`UPDATE agent_events SET completed_at=?, status=?, tokens_in=?, tokens_out=?, cost_usd=?, error_message=?, skills_loaded=?
		 WHERE id=?`,
		e.CompletedAt, string(e.Status), e.TokensIn, e.TokensOut, e.CostUSD, e.ErrorMessage, string(skillsJSON), e.ID,
	)
	if err != nil {
		return fmt.Errorf("updating agent event: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (a *AgentEventStore) ListBySession(ctx context.Context, sessionID string) ([]domain.AgentEvent, error) {
	rows, err := a.db.QueryContext(ctx,
		`SELECT id, session_id, project_id, agent_name, skills_loaded, started_at, completed_at, status, tokens_in, tokens_out, cost_usd, error_message
		 FROM agent_events WHERE session_id = ? ORDER BY started_at ASC`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing agent events: %w", err)
	}
	defer rows.Close()
	return scanAgentEvents(rows)
}

func (a *AgentEventStore) Metrics(ctx context.Context, projectID string) ([]domain.AgentMetrics, error) {
	query := `
		SELECT
			agent_name,
			COUNT(*) AS total_runs,
			SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) AS success_count,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) AS failure_count,
			AVG(CASE WHEN completed_at IS NOT NULL THEN
				(julianday(completed_at) - julianday(started_at)) * 86400
			ELSE NULL END) AS avg_duration_sec,
			SUM(tokens_in) AS total_tokens_in,
			SUM(tokens_out) AS total_tokens_out,
			SUM(cost_usd) AS total_cost_usd
		FROM agent_events`

	var args []interface{}
	if projectID != "" {
		query += " WHERE project_id = ?"
		args = append(args, projectID)
	}
	query += " GROUP BY agent_name ORDER BY total_runs DESC"

	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("computing agent metrics: %w", err)
	}
	defer rows.Close()

	var metrics []domain.AgentMetrics
	for rows.Next() {
		var m domain.AgentMetrics
		var avgDur sql.NullFloat64
		if err := rows.Scan(&m.AgentName, &m.TotalRuns, &m.SuccessCount, &m.FailureCount,
			&avgDur, &m.TotalTokensIn, &m.TotalTokensOut, &m.TotalCostUSD); err != nil {
			return nil, fmt.Errorf("scanning metrics: %w", err)
		}
		if avgDur.Valid {
			m.AvgDurationSec = avgDur.Float64
		}
		if m.TotalRuns > 0 {
			m.SuccessRate = float64(m.SuccessCount) / float64(m.TotalRuns) * 100
		}
		metrics = append(metrics, m)
	}
	return metrics, rows.Err()
}

// TopSkillsForAgent returns the most loaded skills for a given agent.
func (a *AgentEventStore) TopSkillsForAgent(ctx context.Context, agentName string, limit int) ([]string, error) {
	rows, err := a.db.QueryContext(ctx,
		`SELECT skills_loaded FROM agent_events WHERE agent_name = ? AND skills_loaded != '[]' AND skills_loaded != 'null'`,
		agentName,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	freq := make(map[string]int)
	for rows.Next() {
		var skillsJSON string
		if err := rows.Scan(&skillsJSON); err != nil {
			continue
		}
		var skills []string
		if err := json.Unmarshal([]byte(skillsJSON), &skills); err != nil {
			continue
		}
		for _, s := range skills {
			freq[s]++
		}
	}

	// Sort by frequency (simple insertion sort for small N)
	type kv struct{ k string; v int }
	var sorted []kv
	for k, v := range freq {
		sorted = append(sorted, kv{k, v})
	}
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].v > sorted[j-1].v; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

	var result []string
	for i, item := range sorted {
		if i >= limit {
			break
		}
		result = append(result, item.k)
	}
	return result, nil
}

func scanAgentEvents(rows *sql.Rows) ([]domain.AgentEvent, error) {
	var events []domain.AgentEvent
	for rows.Next() {
		var e domain.AgentEvent
		var status string
		var completedAt sql.NullTime
		var skillsJSON string
		var errMsg sql.NullString
		if err := rows.Scan(&e.ID, &e.SessionID, &e.ProjectID, &e.AgentName, &skillsJSON,
			&e.StartedAt, &completedAt, &status, &e.TokensIn, &e.TokensOut, &e.CostUSD, &errMsg); err != nil {
			return nil, fmt.Errorf("scanning agent event: %w", err)
		}
		e.Status = domain.AgentEventStatus(status)
		if completedAt.Valid {
			e.CompletedAt = &completedAt.Time
		}
		if errMsg.Valid {
			e.ErrorMessage = errMsg.String
		}
		_ = json.Unmarshal([]byte(skillsJSON), &e.SkillsLoaded)
		// Normalise nil slice
		if e.SkillsLoaded == nil {
			e.SkillsLoaded = []string{}
		}
		// Convert comma-joined fallback if needed
		if len(e.SkillsLoaded) == 1 && strings.Contains(e.SkillsLoaded[0], ",") {
			e.SkillsLoaded = strings.Split(e.SkillsLoaded[0], ",")
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
