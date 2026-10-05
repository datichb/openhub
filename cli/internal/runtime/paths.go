package ohruntime

import (
	"path/filepath"
	"sort"
	"strings"
)

// Mapping maps a host directory to a directory of the runtime.
type Mapping struct {
	Host  string `json:"host"`
	Inner string `json:"inner"`
}

// PathMap translates paths between the machine and the runtime. The longest
// matching prefix wins. A nil map is the identity (local runtime).
type PathMap []Mapping

// ToInner translates a host path. ok is false when no mapping covers it.
func (m PathMap) ToInner(host string) (string, bool) {
	if m == nil {
		return host, true
	}
	return translate(m, host, func(x Mapping) (string, string) { return x.Host, x.Inner })
}

// ToHost translates a runtime path back to the machine.
func (m PathMap) ToHost(inner string) (string, bool) {
	if m == nil {
		return inner, true
	}
	return translate(m, inner, func(x Mapping) (string, string) { return x.Inner, x.Host })
}

// Covers reports whether a host path is visible in the runtime.
func (m PathMap) Covers(host string) bool {
	_, ok := m.ToInner(host)
	return ok
}

func translate(m PathMap, p string, side func(Mapping) (from, to string)) (string, bool) {
	p = filepath.Clean(p)
	sorted := append(PathMap(nil), m...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, _ := side(sorted[i])
		b, _ := side(sorted[j])
		return len(a) > len(b)
	})
	for _, x := range sorted {
		from, to := side(x)
		from = filepath.Clean(from)
		if p == from {
			return filepath.Clean(to), true
		}
		if rel, ok := strings.CutPrefix(p, from+string(filepath.Separator)); ok {
			return filepath.Join(to, rel), true
		}
		if from == string(filepath.Separator) {
			return filepath.Join(to, p), true
		}
	}
	return "", false
}
