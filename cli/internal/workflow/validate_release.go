package workflow

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
)

// R8: preconditions on the repository (E7) and publishing workflows
// (risk: publish, operations tag and mr, E9).

// IsZero reports whether c declares no test.
func (c PreconditionCheck) IsZero() bool {
	return len(c.PathExists) == 0 && !c.GitClean && len(c.Branch) == 0 && len(c.Tools) == 0
}

// reToolName is a command looked up in the PATH: a name, never a path nor
// arguments.
var reToolName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)

// preconditionRepoChecks checks the branch patterns and the tools of a
// precondition.
func (v *validator) preconditionRepoChecks(base string, c PreconditionCheck) {
	for i, p := range c.Branch {
		if _, err := path.Match(p, ""); strings.TrimSpace(p) == "" || err != nil {
			v.err("precondition_branch_invalid", fmt.Sprintf("%s.check.branch[%d]", base, i), p)
		}
	}
	for i, t := range c.Tools {
		if !reToolName.MatchString(t) {
			v.err("precondition_tool_invalid", fmt.Sprintf("%s.check.tools[%d]", base, i), t)
		}
	}
}

// IsPublishOp reports whether op publishes (push, tag, mr).
func IsPublishOp(op UnlockOp) bool { return slices.Contains(PublishOps, op) }

// publish checks the publishing rules:
//   - tag and mr are unlocked by a `risk: publish` workflow only (push keeps
//     its warning, unlock_push_without_publish);
//   - a `risk: publish` workflow never runs remotely: `remote` is not in
//     runtime.allowed, and each checkpoint that unlocks a publishing
//     operation says `remote: forbid`;
//   - that checkpoint waits for the user in every mode (auto included): a
//     release is never published by the mode.
func (v *validator) publish() {
	s := v.s
	publish := s.Risk == RiskPublish
	if publish && s.AllowsRuntime(RuntimeRemote) {
		v.err("publish_remote_runtime", "runtime.allowed", s.ID)
	}
	for _, k := range s.Checkpoints.Keys() {
		cp, _ := s.Checkpoints.Get(k)
		if cp.Disabled {
			continue
		}
		base := "checkpoints." + k
		publishing := false
		for i, op := range cp.Unlocks {
			if !IsPublishOp(op) {
				continue
			}
			publishing = true
			if !publish && op != UnlockPush {
				v.err("unlock_requires_publish", fmt.Sprintf("%s.unlocks[%d]", base, i), k, string(op))
			}
		}
		if !publish || !publishing {
			continue
		}
		if cp.Remote != RemoteForbid {
			v.err("publish_checkpoint_remote", base+".remote", k)
		}
		for _, mode := range s.AllowedModes() {
			if cp.Behavior(mode) != BehaviorPause {
				v.err("publish_checkpoint_pause", base+".mode."+mode, k, mode)
			}
		}
	}
}
