package evalrun

import (
	"fmt"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
)

// Checkpoint choices of a case (see checkpoint.Choice*).
var checkpointChoices = map[string]bool{"approve": true, "fix": true, "reject": true, "other": true}

// ParseChoice reads a checkpoint answer: "approve", or "fix: message".
func ParseChoice(v string) (choice, message string, err error) {
	choice, message, _ = strings.Cut(v, ":")
	choice, message = strings.TrimSpace(choice), strings.TrimSpace(message)
	if !checkpointChoices[choice] {
		return "", "", fmt.Errorf("checkpoint answer %q (approve, fix, reject, other[: message])", v)
	}
	return choice, message, nil
}

// Answer is how a pass answers one decision.
type Answer struct {
	Decision string
	Message  string
	Answer   map[string]any
	// Stop ends the pass (budget of the pass spent).
	Stop   bool
	Reason string
}

// EvalNote is the message given with an automatic answer of the evaluation.
const EvalNote = "oh eval"

// Decide returns the answer of a case to a decision.
func (a Answers) Decide(d domain.Decision) Answer {
	switch d.Kind {
	case domain.DecisionCheckpoint:
		id, _ := d.Payload.Data["checkpoint"].(string)
		v, ok := a.Checkpoints[id]
		if !ok {
			v, ok = a.Checkpoints["*"]
		}
		if !ok {
			v = "approve"
		}
		choice, msg, err := ParseChoice(v)
		if err != nil {
			choice = "approve"
		}
		return Answer{Decision: choice, Message: msg}
	case domain.DecisionPermission:
		p := a.Permission
		if p == "" {
			p = "once"
		}
		return Answer{Decision: p, Message: EvalNote}
	case domain.DecisionQuestion:
		return Answer{Answer: questionAnswer(d.Payload.Fields, a.Question)}
	case domain.DecisionBudget:
		return Answer{Decision: "stop", Stop: true, Reason: "budget"}
	case domain.DecisionCircuit:
		return Answer{Decision: "dismiss"}
	}
	return Answer{Decision: "dismiss"}
}

// questionAnswer fills every field: the first option (or the given text
// when it matches an option or the field takes a free answer).
func questionAnswer(fields []domain.DecisionField, text string) map[string]any {
	out := map[string]any{}
	for _, f := range fields {
		switch f.Type {
		case "external":
			continue
		case "boolean":
			out[f.Key] = true
			continue
		case "number", "integer":
			out[f.Key] = 1
			continue
		}
		var v string
		if text != "" && text != "first" {
			for _, o := range f.Options {
				if strings.EqualFold(o.Value, text) || strings.EqualFold(o.Label, text) {
					v = o.Value
				}
			}
			if v == "" && (f.Custom || len(f.Options) == 0) {
				v = text
			}
		}
		if v == "" && len(f.Options) > 0 {
			v = f.Options[0].Value
		}
		if v == "" {
			v = "ok"
		}
		if f.Type == "multiselect" {
			out[f.Key] = []string{v}
		} else {
			out[f.Key] = v
		}
	}
	return out
}
