package workflow

import (
	"fmt"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
)

// The audit workflow (workflows/audit.yaml) records, for oh, the report of the
// audit as its `findings` output and the type of the audit as its `type`
// output. A session of that workflow that deposited its report has produced
// findings: the team is told with an audit.finding event.
const (
	// AuditWorkflowID is the workflow of the audits.
	AuditWorkflowID = "audit"
	// AuditFindingsOutput is the output that holds the report of the audit.
	AuditFindingsOutput = "findings"
	// AuditTypeOutput is the output that holds the type of the audit.
	AuditTypeOutput = "type"
)

// AuditFindings gives the data of the audit.finding event of a session: ok is
// false when the session is not an audit, or deposited no report. The number
// of findings is not part of the data: oh does not read the report, which is
// free text written by the agent.
func AuditFindings(s domain.Session) (data map[string]any, ok bool) {
	if s.WorkflowID != AuditWorkflowID {
		return nil, false
	}
	report := outputText(s.Outputs[AuditFindingsOutput])
	if report == "" {
		return nil, false
	}
	data = map[string]any{"session_id": s.ID, "report": report}
	if typ := outputText(s.Outputs[AuditTypeOutput]); typ != "" {
		data["audit_type"] = typ
	}
	return data, true
}

func outputText(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}
