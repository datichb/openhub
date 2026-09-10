package deploy

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/datichb/openhub/cli/internal/workflow"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// GenerateWorkflowSkills produces dynamically generated skill markdown files
// from the resolved workflow definition.
//
// Returns a map of skill name → markdown content.  These generated skills
// replace their static equivalents during the deploy pipeline.
func GenerateWorkflowSkills(wf *workflow.WorkflowDefinition) (map[string]string, error) {
	result := make(map[string]string)

	// 1. orchestrator-workflow-modes
	content, err := generateWorkflowModes(wf)
	if err != nil {
		return nil, fmt.Errorf("generating orchestrator-workflow-modes: %w", err)
	}
	result["orchestrator/orchestrator-workflow-modes"] = content

	// 2. hub-workflow-reference
	content, err = generateWorkflowReference(wf)
	if err != nil {
		return nil, fmt.Errorf("generating hub-workflow-reference: %w", err)
	}
	result["shared/hub-workflow-reference"] = content

	// 3. orchestrator-ticket-routing
	content, err = generateTicketRouting(wf)
	if err != nil {
		return nil, fmt.Errorf("generating orchestrator-ticket-routing: %w", err)
	}
	result["orchestrator/orchestrator-ticket-routing"] = content

	// 4. orchestrator-modes
	content, err = generateOrchestratorModes(wf)
	if err != nil {
		return nil, fmt.Errorf("generating orchestrator-modes: %w", err)
	}
	result["orchestrator/orchestrator-modes"] = content

	return result, nil
}

// ---------------------------------------------------------------------------
// orchestrator-workflow-modes
// ---------------------------------------------------------------------------

type modesTemplateData struct {
	Modes          workflow.ModesConfig
	Checkpoints    []workflow.Checkpoint
	CircuitBreaker workflow.CircuitBreakerConfig
	BehaviorMatrix behaviorMatrix
}

// behaviorMatrix provides a lookup for templates: BehaviorMatrix[cpID][mode]
type behaviorMatrix map[string]map[string]string

func (bm behaviorMatrix) Get(cpID, mode string) string {
	if m, ok := bm[cpID]; ok {
		if v, ok := m[mode]; ok {
			return v
		}
	}
	return "—"
}

func generateWorkflowModes(wf *workflow.WorkflowDefinition) (string, error) {
	// Build behavior matrix for template access.
	bm := make(behaviorMatrix)
	for _, cp := range wf.Checkpoints {
		bm[cp.ID] = make(map[string]string)
		for mode, behavior := range cp.Behavior {
			label := string(behavior)
			if behavior == workflow.BehaviorConditional && cp.Condition != "" {
				label = "conditional"
			}
			bm[cp.ID][mode] = label
		}
	}

	data := modesTemplateData{
		Modes:          wf.Modes,
		Checkpoints:    wf.Checkpoints,
		CircuitBreaker: wf.CircuitBreaker,
		BehaviorMatrix: bm,
	}

	return executeTemplate("templates/orchestrator-workflow-modes.md.tmpl", data)
}

// ---------------------------------------------------------------------------
// hub-workflow-reference
// ---------------------------------------------------------------------------

type referenceTemplateData struct {
	WorkflowAgents    []workflow.AgentSlot
	IndependentAgents []workflow.AgentSlot
	Checkpoints       []workflow.Checkpoint
	Permissions       map[string][]string
}

func generateWorkflowReference(wf *workflow.WorkflowDefinition) (string, error) {
	perms := workflow.DeriveTaskPermissions(wf)

	data := referenceTemplateData{
		WorkflowAgents:    wf.WorkflowAgents(),
		IndependentAgents: wf.IndependentAgents(),
		Checkpoints:       wf.Checkpoints,
		Permissions:       perms,
	}

	return executeTemplate("templates/hub-workflow-reference.md.tmpl", data)
}

// ---------------------------------------------------------------------------
// orchestrator-ticket-routing
// ---------------------------------------------------------------------------

type routingTemplateData struct {
	ActiveAgents []workflow.AgentSlot
}

func generateTicketRouting(wf *workflow.WorkflowDefinition) (string, error) {
	data := routingTemplateData{
		ActiveAgents: wf.ActiveAgents(),
	}
	return executeTemplate("templates/orchestrator-ticket-routing.md.tmpl", data)
}

// ---------------------------------------------------------------------------
// orchestrator-modes
// ---------------------------------------------------------------------------

type orchestratorModesData struct {
	ActiveAgents []workflow.AgentSlot
}

func generateOrchestratorModes(wf *workflow.WorkflowDefinition) (string, error) {
	data := orchestratorModesData{
		ActiveAgents: wf.ActiveAgents(),
	}
	return executeTemplate("templates/orchestrator-modes.md.tmpl", data)
}

// ---------------------------------------------------------------------------
// Template execution
// ---------------------------------------------------------------------------

func executeTemplate(name string, data interface{}) (string, error) {
	funcMap := template.FuncMap{
		"join": strings.Join,
		"index": func(bm behaviorMatrix, cpID, mode string) string {
			return bm.Get(cpID, mode)
		},
		"invokedBy": func(a workflow.AgentSlot) string {
			if a.TaskPermissions == nil || len(a.TaskPermissions.CanBeInvokedBy) == 0 {
				return "—"
			}
			return strings.Join(a.TaskPermissions.CanBeInvokedBy, ", ")
		},
		"hasAgent": func(agents []workflow.AgentSlot, id string) bool {
			for _, a := range agents {
				if a.AgentID == id && a.Role != workflow.RoleDisabled {
					return true
				}
			}
			return false
		},
		"planningAgent": func(agents []workflow.AgentSlot) string {
			for _, a := range agents {
				if a.AgentID == "planner" && a.Role != workflow.RoleDisabled {
					return "planner"
				}
			}
			for _, a := range agents {
				if a.AgentID == "pathfinder" && a.Role != workflow.RoleDisabled {
					return "pathfinder"
				}
			}
			return "planning agent"
		},
	}

	tmplContent, err := templateFS.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("reading template %s: %w", name, err)
	}

	tmpl, err := template.New(name).Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return "", fmt.Errorf("parsing template %s: %w", name, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template %s: %w", name, err)
	}

	return buf.String(), nil
}
