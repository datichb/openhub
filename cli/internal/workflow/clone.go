package workflow

// Deep copies of the oh/v1 document, used by the resolver so that parent
// documents are never mutated.

// Clone returns an independent deep copy of s.
func (s *Spec) Clone() *Spec {
	if s == nil {
		return nil
	}
	c := *s
	c.Label = s.Label.clone()
	c.Description = s.Description.clone()
	c.CodeMode = clonePtr(s.CodeMode)
	c.Entry = clonePtr(s.Entry)
	c.Inputs = s.Inputs.cloneWith(func(in Input) Input {
		in.Default = cloneAny(in.Default)
		in.Label = in.Label.clone()
		in.Help = in.Help.clone()
		in.Values = cloneSlice(in.Values)
		in.Picker = clonePtr(in.Picker)
		return in
	})
	c.Prompt = clonePtr(s.Prompt)
	c.Agents = s.Agents.cloneWith(func(a AgentRef) AgentRef {
		a.Calls = cloneSlice(a.Calls)
		return a
	})
	c.Preconditions = s.Preconditions.cloneWith(func(pc Precondition) Precondition {
		pc.Label = pc.Label.clone()
		pc.Check.PathExists = cloneSlice(pc.Check.PathExists)
		pc.Suggest = clonePtr(pc.Suggest)
		return pc
	})
	c.Checkpoints = s.Checkpoints.cloneWith(func(cp CheckpointSpec) CheckpointSpec {
		cp.Label = cp.Label.clone()
		cp.Description = cp.Description.clone()
		cp.Mode = cloneMap(cp.Mode)
		cp.Mandatory = clonePtr(cp.Mandatory)
		return cp
	})
	if s.Modes != nil {
		m := *s.Modes
		m.Allowed = cloneSlice(m.Allowed)
		c.Modes = &m
	}
	if s.CircuitBreaker != nil {
		cb := CircuitBreaker{MaxConsecutiveSubagents: clonePtr(s.CircuitBreaker.MaxConsecutiveSubagents)}
		c.CircuitBreaker = &cb
	}
	if s.Models != nil {
		m := Models{Default: s.Models.Default, Agents: cloneMap(s.Models.Agents)}
		c.Models = &m
	}
	if s.Skills != nil {
		sk := SkillSelection{Extra: cloneSlice(s.Skills.Extra), Deny: cloneSlice(s.Skills.Deny)}
		c.Skills = &sk
	}
	if s.Plugins != nil {
		c.Plugins = make([]PluginRef, len(s.Plugins))
		for i, p := range s.Plugins {
			c.Plugins[i] = PluginRef{ID: p.ID}
			if p.Options != nil {
				c.Plugins[i].Options, _ = cloneAny(p.Options).(map[string]any)
			}
		}
	}
	c.MCP = cloneSlice(s.MCP)
	if s.Beads != nil {
		c.Beads = &BeadsAccess{Allow: cloneSlice(s.Beads.Allow)}
	}
	if s.Runtime != nil {
		c.Runtime = &RuntimeSpec{Default: s.Runtime.Default, Allowed: cloneSlice(s.Runtime.Allowed)}
	}
	if s.Outputs != nil {
		c.Outputs = make([]Output, len(s.Outputs))
		for i, o := range s.Outputs {
			o.Label = o.Label.clone()
			c.Outputs[i] = o
		}
	}
	if s.Limits != nil {
		c.Limits = &Limits{BudgetUSD: clonePtr(s.Limits.BudgetUSD)}
	}
	return &c
}

func (t LocalizedText) clone() LocalizedText {
	return LocalizedText{Default: t.Default, ByLang: cloneMap(t.ByLang)}
}

func (m OrderedMap[T]) cloneWith(f func(T) T) OrderedMap[T] {
	if len(m.keys) == 0 {
		return OrderedMap[T]{}
	}
	out := OrderedMap[T]{keys: cloneSlice(m.keys), values: make(map[string]T, len(m.values))}
	for k, v := range m.values {
		out.values[k] = f(v)
	}
	return out
}

func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func cloneSlice[T any](s []T) []T {
	if s == nil {
		return nil
	}
	return append(make([]T, 0, len(s)), s...)
}

func cloneMap[K comparable, V any](m map[K]V) map[K]V {
	if m == nil {
		return nil
	}
	out := make(map[K]V, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// cloneAny copies the YAML value shapes (maps, lists, scalars).
func cloneAny(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[k] = cloneAny(e)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = cloneAny(e)
		}
		return out
	}
	return v
}
