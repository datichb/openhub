package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/domain"
)

func TestAuditFindings(t *testing.T) {
	audit := func(outputs map[string]any) domain.Session {
		return domain.Session{ID: "ses_au", WorkflowID: "audit", Outputs: outputs}
	}
	tests := []struct {
		name string
		s    domain.Session
		want map[string]any
	}{
		{"an audit with its report", audit(map[string]any{"findings": "rp-2", "type": "security"}),
			map[string]any{"session_id": "ses_au", "report": "rp-2", "audit_type": "security"}},
		{"the type is optional", audit(map[string]any{"findings": " rp-1 "}),
			map[string]any{"session_id": "ses_au", "report": "rp-1"}},
		{"no report deposited", audit(map[string]any{"type": "security"}), nil},
		{"an empty report output", audit(map[string]any{"findings": ""}), nil},
		{"no output at all", audit(nil), nil},
		{"another workflow", domain.Session{ID: "ses_t", WorkflowID: "ticket", Outputs: map[string]any{"findings": "rp-1"}}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := AuditFindings(tc.s)
			assert.Equal(t, tc.want != nil, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}
