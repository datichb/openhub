package workflow

import (
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
)

// Verification and evidence of workflow_checkpoint (R3: E1, E2).

// evidenceSchema is the input schema of the evidence of a checkpoint.
func evidenceSchema() map[string]any {
	return map[string]any{
		"type":        "array",
		"description": i18n.T("cmd.mcp.workflow.checkpoint_evidence"),
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cmd":  map[string]any{"type": "string"},
				"exit": map[string]any{"type": "integer"},
			},
			"required": []string{"cmd", "exit"},
		},
	}
}

// writeVerify adds the verification run by oh and the evidence it did not
// observe to the answer of a checkpoint.
func writeVerify(b *strings.Builder, res checkpoint.Result) {
	if res.Verify != "" {
		b.WriteString("\n")
		b.WriteString(i18n.Tf("cmd.mcp.workflow.checkpoint_verify", res.Verify))
	}
	if len(res.Unobserved) > 0 {
		b.WriteString("\n")
		b.WriteString(i18n.Tf("cmd.mcp.workflow.checkpoint_unobserved", strings.Join(res.Unobserved, " · ")))
	}
}
