package workflow

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Human approver of a checkpoint (CP-R9, step 1, ADR-055). `approver:`
// names the people expected to validate the checkpoint: a git user.name
// (the identity oh gives a person in Beads, ADR-052) or an e-mail address
// (git user.email), one or a list. Only a team layer declares it: neither
// the hub (shipped workflows), nor a project layer, nor the launch options
// (`checkpoint_approver_layer`). A project layer may not remove it either:
// neither by disabling the checkpoint nor by making it pause less in a mode
// (`loosening`), as for a mandatory checkpoint.
//
// On this machine oh cannot enforce it: the card shows the approver, a
// validation by someone else is warned and traced (`approver_mismatch`),
// never refused (services/checkpoint). Step 2 (a remote session that only
// the approver may unblock) is not done.

// MaxApprovers caps the identities of one checkpoint.
const MaxApprovers = 20

// maxApproverLen caps one identity.
const maxApproverLen = 100

// Approvers are the identities expected to validate a checkpoint.
type Approvers []string

// UnmarshalYAML accepts one identity or a list.
func (a *Approvers) UnmarshalYAML(unmarshal func(any) error) error {
	node, err := captureNode(unmarshal)
	if err != nil {
		return err
	}
	switch node.Kind {
	case yaml.ScalarNode:
		if node.Tag == "!!null" {
			*a = nil
			return nil
		}
		*a = Approvers{node.Value}
		return nil
	case yaml.SequenceNode:
		var l []string
		if err := unmarshal(&l); err != nil {
			return err
		}
		*a = l
		return nil
	default:
		return nodeError(node, "expected_approver")
	}
}

// String lists the identities ("alice, bob@example.com").
func (a Approvers) String() string { return strings.Join(a, ", ") }

// approverLayer is the only layer that may write `approver:`.
const approverLayer = LayerTeam

// approverPaths are the recorded paths of `approver:` of checkpoint k.
func (v *validator) approverPaths(k string) []string {
	base := "checkpoints." + k + ".approver"
	var at []string
	for p := range v.r.Origins {
		if p == base || strings.HasPrefix(p, base+"[") {
			at = append(at, p)
		}
	}
	slices.Sort(at)
	return at
}

// checkpointsApprover validates `approver:` of each checkpoint: identities
// on one line, once each, at most MaxApprovers, written by a team layer
// only; warned on a checkpoint that never pauses (nobody is asked).
func (v *validator) checkpointsApprover() {
	s := v.s
	for _, k := range s.Checkpoints.Keys() {
		cp, _ := s.Checkpoints.Get(k)
		base := "checkpoints." + k + ".approver"
		v.approverLayer(k, base)
		if len(cp.Approver) == 0 || cp.Disabled {
			continue
		}
		if len(cp.Approver) > MaxApprovers {
			v.err("checkpoint_approver_max", base, k, MaxApprovers)
		}
		seen := map[string]bool{}
		for i, id := range cp.Approver {
			at := fmt.Sprintf("%s[%d]", base, i)
			key := sessionspec.NormalizeIdentity(id)
			switch {
			case !validIdentity(id):
				v.err("checkpoint_approver_invalid", at, id, k)
			case seen[key]:
				v.warn("checkpoint_approver_duplicate", at, id, k)
			}
			seen[key] = true
		}
		pauses := false
		for _, mode := range s.AllowedModes() {
			if b := cp.Behavior(mode); b == BehaviorPause || b == BehaviorConditional {
				pauses = true
				break
			}
		}
		if !pauses && cp.When.IsZero() {
			v.warn("checkpoint_approver_no_pause", base, k)
		}
		if cp.RemotePolicy() == RemoteAuto {
			// Passed by the remote policy in a job, never by the approver
			// (traced as out of approver; step 2 will change it).
			v.warn("checkpoint_approver_remote_auto", base, k)
		}
	}
}

// approverLayer reports an `approver:` written by another layer than the
// team (in a full definition or in a patch).
func (v *validator) approverLayer(k, base string) {
	at := v.approverPaths(k)
	if len(at) == 0 {
		cp, _ := v.s.Checkpoints.Get(k)
		if len(cp.Approver) == 0 {
			return
		}
		at = []string{base}
	}
	for _, p := range at {
		o, ok := v.r.Origins.Of(p)
		if !ok {
			// No document recorded (in-memory): the workflow itself.
			o = Origin{Layer: v.r.Ref.Layer, ID: v.r.Ref.ID}
		}
		if o.Layer != approverLayer {
			v.err("checkpoint_approver_layer", base, o.Ref().String())
			return
		}
	}
}

// validIdentity reports whether id may name a person: not blank, on one
// printable line, at most maxApproverLen characters.
func validIdentity(id string) bool {
	t := strings.TrimSpace(id)
	if t == "" || len([]rune(t)) > maxApproverLen {
		return false
	}
	for _, r := range t {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

// patchApprover applies `approver:` of a patch to an inherited checkpoint:
// a team layer replaces it; any other layer keeps the inherited value (the
// validation reports the layer, checkpoint_approver_layer).
func (p *patcher) patchApprover(base string, old *CheckpointSpec, v CheckpointSpec) {
	if !p.has(base + ".approver") {
		return
	}
	if p.doc.Source.Layer == approverLayer {
		old.Approver = cloneSlice(v.Approver)
	}
}

// approverHeld reports whether old has an approver that this patch may not
// loosen (any layer but the team's).
func (p *patcher) approverHeld(old CheckpointSpec) bool {
	return len(old.Approver) > 0 && p.doc.Source.Layer != approverLayer
}
