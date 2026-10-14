package bundle

import (
	"bytes"
	"embed"
	"fmt"
	"slices"
	"strings"
	"text/template"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Chain skills generated from an oh/v1 workflow (P1-T11). They replace the
// static skills of the same reference in the bundle (inlined or on demand),
// so that every agent reads the enchaînement of the workflow it runs in,
// never a hard-coded one.
//
// The bundle does not depend on the session mode (sessions of every mode
// share a server group): the generated text lists every allowed mode, and
// the session prompt states the mode.

//go:embed templates/*.md.tmpl
var templateFS embed.FS

// Skill references generated from the workflow.
const (
	// WorkflowMapSkill is the full map, read by the conductor (P1-T12).
	WorkflowMapSkill = "workflow/workflow-map"
	// Existing references, loaded by orchestrator, orchestrator-dev and planner.
	workflowModesSkill     = "orchestrator/orchestrator-workflow-modes"
	workflowReferenceSkill = "shared/hub-workflow-reference"
)

var generatedTemplates = map[string]string{
	WorkflowMapSkill:       "workflow-map.md.tmpl",
	workflowModesSkill:     "orchestrator-workflow-modes.md.tmpl",
	workflowReferenceSkill: "hub-workflow-reference.md.tmpl",
}

// generatedLang is the language of the generated text (the hub content is
// written in French). Fixed so that the bundle hash does not depend on the
// UI language.
const generatedLang = "fr"

// mapData is the template view of a workflow.
type mapData struct {
	ID, Description, Entry string
	Risk                   workflow.Risk
	Beads                  []string
	Agents                 []agentRow
	Modes                  []string
	DefaultMode            string
	Checkpoints            []checkpointRow
	Sequence               string
	Remote                 bool
	MaxConsecutive         int
	MaxFixRounds           int
	Outputs                []outputRow
	// CheckpointAgents are the agents that pass a checkpoint (L3), with
	// the sequence of their own checkpoints.
	CheckpointAgents []string
	Sequences        []agentSequence
	// Writes are the documentation paths opened to agents (R4).
	Writes []agentWrites
}

type agentWrites struct {
	Agent string
	Paths []string
}

type agentSequence struct{ Agent, Sequence string }

type agentRow struct {
	ID    string
	Role  workflow.AgentRole
	Mode  workflow.AgentMode
	Entry bool
	After string
	Calls []string
}

type checkpointRow struct {
	ID, Label, Description, Condition string
	Mandatory                         bool
	Behaviors                         []workflow.CheckpointBehavior // one per allowed mode
	Remote                            workflow.RemotePolicy
	Unlocks                           []string
	// By are the agents that pass it (L3): the other agents do not see it.
	By []string
	// Check is what oh checks before it passes, for a test written first
	// (R7, testCheckText; "" otherwise).
	Check string
}

type outputRow struct {
	ID    string
	Type  workflow.OutputType
	Label string
}

// GenerateWorkflowSkills renders the chain skills of spec. graph is the
// delegation graph of the bundle; modeOf gives the mode of a catalogue agent
// (used when the workflow does not set one).
func GenerateWorkflowSkills(spec *workflow.Spec, graph map[string][]string, modeOf func(id string) workflow.AgentMode) (map[string]string, error) {
	data := buildMapData(spec, graph, modeOf)
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"codes": func(ss []string) string {
			out := make([]string, len(ss))
			for i, s := range ss {
				out[i] = "`" + s + "`"
			}
			return strings.Join(out, ", ")
		},
		"unlockText": unlockText,
		"join":       strings.Join,
	}).ParseFS(templateFS, "templates/*.md.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parsing workflow skill templates: %w", err)
	}
	out := make(map[string]string, len(generatedTemplates))
	for ref, name := range generatedTemplates {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
			return nil, fmt.Errorf("generating %s: %w", ref, err)
		}
		out[ref] = buf.String()
	}
	return out, nil
}

// unlockOpText describes each operation a checkpoint may unlock (generated
// text, generatedLang).
var unlockOpText = map[string]string{
	sessionspec.UnlockCommit: "`git commit` (et les commandes qui commitent : revert, cherry-pick, merge, pull)",
	sessionspec.UnlockPush:   "`git push`",
	sessionspec.UnlockClose:  "la fermeture d'un ticket",
	sessionspec.UnlockCreate: "la création de tickets",
	sessionspec.UnlockApply:  "les modifications d'infrastructure (`terraform apply`, `kubectl apply`…)",
}

// unlockText is the rule of a checkpoint that unlocks ops: only the declared
// operations are named (N10), in the order the agent should run them.
func unlockText(id string, ops []string) string {
	has := map[string]bool{}
	var names []string
	for _, op := range ops {
		if t, ok := unlockOpText[op]; ok && !has[op] {
			has[op] = true
			names = append(names, t)
		}
	}
	if len(names) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "avant son passage, oh refuse à tous les agents de la session %s, quel que soit le mode.", strings.Join(names, ", "))
	switch {
	case has[sessionspec.UnlockCommit] && has[sessionspec.UnlockClose]:
		fmt.Fprintf(&b, " Une fois passé, fais-les aussitôt, dans l'ordre : commit, puis fermeture du ticket (oh refuse la fermeture tant que le travail n'est pas commité). Le travail commité, le commit est de nouveau verrouillé ; le ticket fermé, la fermeture aussi, jusqu'au prochain passage de **%s**.", id)
	case has[sessionspec.UnlockCommit]:
		fmt.Fprintf(&b, " Une fois passé, commite aussitôt ; la fenêtre se referme une fois tout le travail commité, ou quand **%s** est demandé de nouveau.", id)
	case has[sessionspec.UnlockClose]:
		fmt.Fprintf(&b, " Le ticket fermé, la fermeture est de nouveau verrouillée jusqu'au prochain passage de **%s**.", id)
	}
	if has[sessionspec.UnlockCreate] {
		fmt.Fprintf(&b, " Une fois passé, crée les tickets validés aussitôt : la création se referme au checkpoint obligatoire suivant, ou quand **%s** est demandé de nouveau.", id)
	}
	if !has[sessionspec.UnlockPush] && (has[sessionspec.UnlockCommit] || has[sessionspec.UnlockClose]) {
		b.WriteString(" Le push revient à l'utilisateur : ne pousse pas.")
	}
	return b.String()
}

// checkpointSequence is the order of the checkpoints with the operations
// each one unlocks: "`cp-1` → `cp-2` [commit, close] → `cp-3`".
func checkpointSequence(cps []checkpointRow) string {
	steps := make([]string, len(cps))
	for i, cp := range cps {
		steps[i] = "`" + cp.ID + "`"
		if len(cp.Unlocks) > 0 {
			steps[i] += " [" + strings.Join(cp.Unlocks, ", ") + "]"
		}
	}
	return strings.Join(steps, " → ")
}

func buildMapData(spec *workflow.Spec, graph map[string][]string, modeOf func(string) workflow.AgentMode) mapData {
	d := mapData{
		ID:          spec.ID,
		Description: strings.TrimSpace(spec.Description.Text(generatedLang)),
		Entry:       spec.EntryAgent(),
		Risk:        spec.Risk,
		Modes:       spec.AllowedModes(),
		DefaultMode: spec.DefaultMode(),
		Remote:      spec.AllowsRuntime(workflow.RuntimeRemote),
	}
	if spec.Beads != nil {
		d.Beads = spec.Beads.Allow
	}
	for _, id := range spec.Agents.Keys() {
		if w := spec.WritesOf(id); len(w) > 0 {
			d.Writes = append(d.Writes, agentWrites{Agent: id, Paths: w})
		}
	}
	if cb := spec.CircuitBreaker; cb != nil && cb.MaxConsecutiveSubagents != nil {
		d.MaxConsecutive = *cb.MaxConsecutiveSubagents
	}
	if l := spec.Limits; l != nil && l.MaxFixRounds != nil {
		d.MaxFixRounds = *l.MaxFixRounds
	}
	for _, id := range spec.Members() {
		ref, listed := spec.Agents.Get(id)
		row := agentRow{ID: id, Role: workflow.RoleWorkflow, Entry: id == d.Entry, Calls: graph[id]}
		if listed {
			row.Role, row.After, row.Mode = ref.Role, ref.After, ref.Mode
		}
		if row.Mode == "" && modeOf != nil {
			row.Mode = modeOf(id)
		}
		if row.Mode == "" {
			row.Mode = workflow.ModePrimary
		}
		d.Agents = append(d.Agents, row)
	}
	for _, id := range spec.Checkpoints.Keys() {
		cp, _ := spec.Checkpoints.Get(id)
		row := checkpointRow{
			ID:          id,
			Label:       cp.Label.Text(generatedLang),
			Description: strings.TrimSpace(cp.Description.Text(generatedLang)),
			Condition:   cp.Condition,
			Mandatory:   cp.IsMandatory(),
			Remote:      cp.RemotePolicy(),
			By:          spec.CheckpointBy(id, graph),
			Check:       testCheckText(cp.Verify),
		}
		for _, a := range row.By {
			if !slices.Contains(d.CheckpointAgents, a) {
				d.CheckpointAgents = append(d.CheckpointAgents, a)
			}
		}
		for _, op := range cp.Unlocks {
			row.Unlocks = append(row.Unlocks, string(op))
		}
		for _, m := range d.Modes {
			b := cp.Behavior(m)
			if row.Mandatory && b == workflow.BehaviorSkip {
				b = workflow.BehaviorPause // a mandatory checkpoint is never skipped
			}
			row.Behaviors = append(row.Behaviors, b)
		}
		d.Checkpoints = append(d.Checkpoints, row)
	}
	d.Sequence = checkpointSequence(d.Checkpoints)
	for _, a := range d.CheckpointAgents {
		var own []checkpointRow
		for _, cp := range d.Checkpoints {
			if slices.Contains(cp.By, a) {
				own = append(own, cp)
			}
		}
		d.Sequences = append(d.Sequences, agentSequence{Agent: a, Sequence: checkpointSequence(own)})
	}
	for _, o := range spec.Outputs {
		d.Outputs = append(d.Outputs, outputRow{ID: o.ID, Type: o.Type, Label: o.Label.Text(generatedLang)})
	}
	return d
}
