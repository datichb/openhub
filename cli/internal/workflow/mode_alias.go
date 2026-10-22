package workflow

import "strings"

// ModeManualAlias is the English spelling of the manual mode. It is accepted
// wherever a mode is written or typed and read back as ModeManual, the only
// name stored, compared and shown (U-04).
const ModeManualAlias = "manual"

// CanonicalMode returns the stored name of a mode: ModeManual for
// ModeManualAlias, the mode itself otherwise (custom modes are kept).
func CanonicalMode(mode string) string {
	if mode == ModeManualAlias {
		return ModeManual
	}
	return mode
}

// canonicalModes reads the English alias as the canonical mode in what a
// document writes: `modes.default`, `modes.allowed` and the keys of
// `checkpoints.<id>.mode`. The position index follows, so that diagnostics
// and Has() speak of the canonical name. A checkpoint that writes both
// spellings is reported (the second one would be lost).
func (d *Document) canonicalModes() Diagnostics {
	s := d.Spec
	var diags Diagnostics
	if s.Modes != nil {
		s.Modes.Default = CanonicalMode(s.Modes.Default)
		for i, m := range s.Modes.Allowed {
			s.Modes.Allowed[i] = CanonicalMode(m)
		}
	}
	for _, id := range s.Checkpoints.Keys() {
		cp, _ := s.Checkpoints.Get(id)
		b, ok := cp.Mode[ModeManualAlias]
		if !ok {
			continue
		}
		if _, both := cp.Mode[ModeManual]; both {
			diags = append(diags, errDiag("duplicate_entry", "checkpoints."+id+".mode."+ModeManualAlias, ModeManualAlias))
			delete(cp.Mode, ModeManualAlias)
		} else {
			delete(cp.Mode, ModeManualAlias)
			cp.Mode[ModeManual] = b
		}
		s.Checkpoints.Set(id, cp)
		d.index.rename("checkpoints."+id+".mode."+ModeManualAlias, "checkpoints."+id+".mode."+ModeManual)
	}
	d.index.canonicalModeValues()
	return diags
}

// rename moves a written path (key position and entries) to another name.
func (idx *nodeIndex) rename(from, to string) {
	pos, ok := idx.pos[from]
	if !ok {
		return
	}
	delete(idx.pos, from)
	if _, exists := idx.pos[to]; exists {
		for i, p := range idx.order {
			if p == from {
				idx.order = append(idx.order[:i], idx.order[i+1:]...)
				break
			}
		}
		return
	}
	idx.pos[to] = pos
	for i, p := range idx.order {
		if p == from {
			idx.order[i] = to
		}
	}
	for i := range idx.entries {
		if idx.entries[i].path == from {
			idx.entries[i].path = to
			if idx.entries[i].key {
				idx.entries[i].value = ModeManual
			}
		}
	}
}

// canonicalModeValues rewrites the scalar values of modes.default and
// modes.allowed[i] in the entries.
func (idx *nodeIndex) canonicalModeValues() {
	for i := range idx.entries {
		e := &idx.entries[i]
		if e.scalar && e.value == ModeManualAlias && (e.path == "modes.default" || strings.HasPrefix(e.path, "modes.allowed[")) {
			e.value = ModeManual
		}
	}
}
