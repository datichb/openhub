package workflow

// Effective values: what an unset field means once the workflow is resolved.

// DefaultModes are the modes available when `modes.allowed` is not set.
var DefaultModes = []string{ModeManual, ModeSemiAuto, ModeAuto}

// AllowedModes returns `modes.allowed`, or DefaultModes when unset.
func (s *Spec) AllowedModes() []string {
	if s.Modes != nil && len(s.Modes.Allowed) > 0 {
		return s.Modes.Allowed
	}
	return DefaultModes
}

// DefaultMode returns `modes.default`, or the first allowed mode.
func (s *Spec) DefaultMode() string {
	if s.Modes != nil && s.Modes.Default != "" {
		return s.Modes.Default
	}
	if allowed := s.AllowedModes(); len(allowed) > 0 {
		return allowed[0]
	}
	return ""
}

// AllowedRuntimes returns `runtime.allowed`; unset means the default runtime
// only (local when no default is set).
func (s *Spec) AllowedRuntimes() []Runtime {
	if s.Runtime != nil && len(s.Runtime.Allowed) > 0 {
		return s.Runtime.Allowed
	}
	return []Runtime{s.DefaultRuntime()}
}

// DefaultRuntime returns `runtime.default`, or local.
func (s *Spec) DefaultRuntime() Runtime {
	if s.Runtime != nil && s.Runtime.Default != "" {
		return s.Runtime.Default
	}
	return RuntimeLocal
}

// AllowsRuntime reports whether r is in the allowed runtimes.
func (s *Spec) AllowsRuntime(r Runtime) bool {
	for _, a := range s.AllowedRuntimes() {
		if a == r {
			return true
		}
	}
	return false
}

// IsMandatory reports whether the checkpoint cannot be removed or relaxed.
func (c CheckpointSpec) IsMandatory() bool { return c.Mandatory != nil && *c.Mandatory }

// RemotePolicy returns `remote`; unset means defer (wait for the user).
func (c CheckpointSpec) RemotePolicy() RemotePolicy {
	if c.Remote == "" {
		return RemoteDefer
	}
	return c.Remote
}

// Behavior returns the behavior for mode; unset means pause.
func (c CheckpointSpec) Behavior(mode string) CheckpointBehavior {
	if b, ok := c.Mode[mode]; ok && b != "" {
		return b
	}
	return BehaviorPause
}

// Rank orders remote policies by strictness (auto < defer < forbid).
func (p RemotePolicy) Rank() int {
	switch p {
	case RemoteAuto:
		return 1
	case RemoteDefer:
		return 2
	case RemoteForbid:
		return 3
	}
	return 0
}

// Valid reports whether b is a known checkpoint behavior.
func (b CheckpointBehavior) Valid() bool {
	switch b {
	case BehaviorPause, BehaviorAuto, BehaviorSkip, BehaviorConditional:
		return true
	}
	return false
}

// Strictness orders behaviors by how much they involve the user
// (skip < auto < conditional < pause).
func (b CheckpointBehavior) Strictness() int {
	switch b {
	case BehaviorSkip:
		return 0
	case BehaviorAuto:
		return 1
	case BehaviorConditional:
		return 2
	case BehaviorPause:
		return 3
	}
	return 0
}

// Valid reports whether r is a known agent role.
func (r AgentRole) Valid() bool {
	return r == RoleWorkflow || r == RoleIndependent || r == RoleDisabled
}

// Valid reports whether m is a known agent mode.
func (m AgentMode) Valid() bool { return m == ModePrimary || m == ModeSubagent }

// WaitingCheckpoints lists the checkpoints that wait for the user in a mode
// (pause, conditional, or a mandatory one set to skip), in declaration order.
func (s *Spec) WaitingCheckpoints(mode string) []string {
	var out []string
	for _, id := range s.Checkpoints.Keys() {
		cp, _ := s.Checkpoints.Get(id)
		if cp.Disabled {
			continue
		}
		switch b := cp.Behavior(mode); {
		case b == BehaviorPause, b == BehaviorConditional, b == BehaviorSkip && cp.IsMandatory():
			out = append(out, id)
		}
	}
	return out
}
