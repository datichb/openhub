package workflow

import "strings"

// checkpointVerify merges `verify:` and `when:` of a checkpoint patch (R3).
// Both only harden: removing or weakening an inherited verification, or
// dropping an inherited condition, is a loosening and keeps the parent value.
func (p *patcher) checkpointVerify(base string, old *CheckpointSpec, v CheckpointSpec) {
	if p.has(base + ".verify") {
		if !v.Verify.Covers(old.Verify) {
			p.loosening(base+".verify", verifyText(v.Verify), verifyText(old.Verify))
		} else {
			old.Verify = v.Verify
		}
	}
	if p.has(base + ".when") {
		if !v.When.Covers(old.When) {
			p.loosening(base+".when", whenText(v.When), whenText(old.When))
		} else {
			old.When = v.When
		}
	}
}

func verifyText(v *VerifySpec) string {
	if v == nil {
		return "none"
	}
	var parts []string
	if v.From != "" {
		kinds := make([]string, 0, len(v.EffectiveKinds()))
		for _, k := range v.EffectiveKinds() {
			kinds = append(kinds, string(k))
		}
		parts = append(parts, "from: "+string(v.From)+" ("+strings.Join(kinds, ",")+")")
	}
	if len(v.Cmd) > 0 {
		parts = append(parts, "cmd: "+strings.Join(v.Cmd, "; "))
	}
	parts = append(parts, "expect: "+string(v.ExpectOrPass()))
	if v.In != "" {
		parts = append(parts, "in: "+v.In)
	}
	if len(v.MustTouch) > 0 {
		parts = append(parts, "must_touch: "+strings.Join(v.MustTouch, ","))
	}
	parts = append(parts, "timeout: "+v.TimeoutOrDefault().String())
	return "{" + strings.Join(parts, ", ") + "}"
}

func whenText(w *CheckpointWhen) string {
	if w.IsZero() {
		return "none"
	}
	var parts []string
	add := func(k string, vs []string) {
		if len(vs) > 0 {
			parts = append(parts, k+": "+strings.Join(vs, ","))
		}
	}
	add("entry", w.Entry)
	add("labels", w.Labels)
	add("paths", w.Paths)
	add("outputs", w.Outputs)
	return "{" + strings.Join(parts, ", ") + "}"
}
