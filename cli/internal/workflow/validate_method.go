package workflow

import (
	"slices"
	"strings"
)

// Validation of the method of a session (R7): the `method` input and the
// checkpoints declared for a method, and the R7 fields of `verify:`.

// methodValues are the values the `method` input may offer.
func methodValues() []string {
	out := []string{string(MethodStandard)}
	for _, m := range Methods {
		out = append(out, string(m))
	}
	return out
}

func (v *validator) methods() {
	s := v.s
	in, hasInput := s.Inputs.Get(MethodInput)
	if hasInput {
		base := "inputs." + MethodInput
		if in.Type != InputEnum {
			v.err("method_input_type", base+".type", string(in.Type))
		}
		for _, val := range in.Values {
			v.enum(base+".values", val, slices.Contains(methodValues(), val), methodValues()...)
		}
	}
	afters := map[string][]string{}
	for _, id := range s.Agents.Keys() {
		if a, _ := s.Agents.Get(id); a.After != "" {
			afters[a.After] = append(afters[a.After], id)
		}
	}
	for _, k := range s.Checkpoints.Keys() {
		cp, _ := s.Checkpoints.Get(k)
		if cp.Method == "" || cp.Disabled {
			continue
		}
		base := "checkpoints." + k
		names := make([]string, len(Methods))
		for i, m := range Methods {
			names[i] = string(m)
		}
		v.enum(base+".method", string(cp.Method), cp.Method.Valid(), names...)
		if !hasInput || in.Type != InputEnum || !slices.Contains(in.Values, string(cp.Method)) {
			v.err("checkpoint_method_no_input", base+".method", k, string(cp.Method), MethodInput)
		}
		if cp.IsMandatory() || len(cp.Unlocks) > 0 {
			// Outside the method the checkpoint does not exist: it may
			// only add a step, never hold a decision or a lock (CP-R7).
			v.err("checkpoint_method_mandatory", base+".method", k)
		}
		for _, a := range afters[k] {
			v.err("checkpoint_method_after", "agents."+a+".after", a, k)
		}
	}
}

// verifyTests checks the R7 fields of the verification of checkpoint id.
func (v *validator) verifyTests(base, id string, cp CheckpointSpec) {
	vs := cp.Verify
	if vs.OutsideTests != "" {
		v.enum(base+".outside_tests", string(vs.OutsideTests), vs.OutsideTests == OutsideTestsWarn || vs.OutsideTests == OutsideTestsRefuse,
			string(OutsideTestsWarn), string(OutsideTestsRefuse))
		if !vs.MustTouchTests {
			v.warn("verify_outside_tests_ignored", base+".outside_tests")
		}
	}
	if vs.Since == "" {
		return
	}
	keys := v.s.Checkpoints.Keys()
	at := slices.Index(keys, id)
	i := slices.Index(keys, vs.Since)
	if i < 0 || i >= at {
		v.err("verify_since_unknown", base+".since", vs.Since, strings.Join(keys[:max(at, 0)], ", "))
		return
	}
	if from, _ := v.s.Checkpoints.Get(vs.Since); from.Method != "" && from.Method != cp.Method {
		// Outside its method the checkpoint does not exist.
		v.err("verify_since_method", base+".since", vs.Since, string(from.Method), id)
	}
}
