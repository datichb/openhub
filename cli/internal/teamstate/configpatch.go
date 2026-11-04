package teamstate

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
)

// ErrConfigConflict is returned when a change to config.toml touches a
// setting that a colleague changed since the copy it started from: nothing is
// written, the caller reloads and edits again. Keys are the settings
// (tracker.type, mcp.gitlab.url, models.families.opus…).
type ErrConfigConflict struct {
	Keys []string
}

func (e *ErrConfigConflict) Error() string {
	return "config.toml was changed meanwhile by a colleague: " + strings.Join(e.Keys, ", ")
}

// IsConfigConflict reports whether err is an ErrConfigConflict and returns it.
func IsConfigConflict(err error) (*ErrConfigConflict, bool) {
	var c *ErrConfigConflict
	if errors.As(err, &c) {
		return c, true
	}
	return nil, false
}

// UpdateConfig changes config.toml of the team-state: patch receives the
// configuration as it is on disk after the pull of the transaction, and
// changes only the settings of its caller. When the push is refused (another
// member pushed first) the transaction pulls again and patch runs again on the
// version of the remote: the changes of the others are never replaced by an
// older copy. An unreadable config.toml is an error, not an empty
// configuration: nothing is written over it.
//
// The result is validated (lease settings, mappings of the tracker) before it
// is written.
func (r *Repo) UpdateConfig(ctx context.Context, patch func(*TeamConfig) error) error {
	return r.Transact(ctx, func(_ context.Context, tx *Tx) (TxResult, error) {
		cfg, err := tx.Config()
		if err != nil {
			return TxResult{}, err
		}
		if err := patch(cfg); err != nil {
			return TxResult{}, err
		}
		if err := cfg.Claim.Validate(); err != nil {
			return TxResult{}, err
		}
		if err := cfg.Tracker.ValidateMappings(cfg.Board); err != nil {
			return TxResult{}, err
		}
		if err := tx.WriteConfig(cfg); err != nil {
			return TxResult{}, err
		}
		return TxResult{Message: "config: update team config"}, nil
	})
}

// UpdateConfigFrom writes the changes between base (the configuration as the
// caller read it) and edited to config.toml, on top of the latest version
// (UpdateConfig). A setting changed on both sides since base gives an
// ErrConfigConflict and writes nothing. Nothing changed: nothing is done.
func (r *Repo) UpdateConfigFrom(ctx context.Context, base, edited *TeamConfig) error {
	p := DiffConfig(base, edited)
	if p.Empty() {
		return nil
	}
	return r.UpdateConfig(ctx, p.Apply)
}

// ConfigPatch is the difference between two configurations, setting by
// setting. Applied to a third one, it changes the settings it holds and
// refuses to overwrite one that is no more what the patch started from.
type ConfigPatch struct {
	ops []configOp
}

type pathStep struct {
	field int           // index of a struct field (when !isKey)
	key   reflect.Value // key of a map entry (when isKey)
	isKey bool
}

type configOp struct {
	key         string
	path        []pathStep
	base, next  reflect.Value
	basePresent bool
	nextPresent bool
}

// DiffConfig is the patch that turns base into edited. Structs are compared
// field by field, maps entry by entry (and the entries that are structs field
// by field); slices and pointers to scalars are settings of their own.
func DiffConfig(base, edited *TeamConfig) ConfigPatch {
	var ops []configOp
	b, e := TeamConfig{}, TeamConfig{}
	if base != nil {
		b = *base
	}
	if edited != nil {
		e = *edited
	}
	diffValue(reflect.ValueOf(&b).Elem(), reflect.ValueOf(&e).Elem(), nil, "", &ops)
	return ConfigPatch{ops: ops}
}

// Empty reports whether the patch changes nothing.
func (p ConfigPatch) Empty() bool { return len(p.ops) == 0 }

// Keys are the settings the patch changes, sorted.
func (p ConfigPatch) Keys() []string {
	keys := make([]string, len(p.ops))
	for i, op := range p.ops {
		keys[i] = op.key
	}
	sort.Strings(keys)
	return keys
}

// Apply changes cfg. A setting whose value in cfg is neither the one the patch
// started from nor the one it sets was changed by someone else: the whole
// patch is refused with an ErrConfigConflict and cfg is left as it is.
func (p ConfigPatch) Apply(cfg *TeamConfig) error {
	root := reflect.ValueOf(cfg).Elem()
	var conflicts []string
	var todo []configOp
	for _, op := range p.ops {
		cur, present := getAt(root, op.path)
		switch {
		case sameState(cur, present, op.next, op.nextPresent):
			// already as wanted
		case sameState(cur, present, op.base, op.basePresent):
			todo = append(todo, op)
		default:
			conflicts = append(conflicts, op.key)
		}
	}
	if len(conflicts) > 0 {
		sort.Strings(conflicts)
		return &ErrConfigConflict{Keys: conflicts}
	}
	for _, op := range todo {
		setAt(root, op.path, op.next, op.nextPresent)
	}
	return nil
}

// CloneConfig is a copy of cfg that shares no map, slice nor pointer with it.
func CloneConfig(cfg *TeamConfig) *TeamConfig {
	if cfg == nil {
		return nil
	}
	c := cloneValue(reflect.ValueOf(cfg).Elem())
	out := c.Interface().(TeamConfig)
	return &out
}

func sameState(a reflect.Value, aPresent bool, b reflect.Value, bPresent bool) bool {
	if aPresent != bPresent {
		return false
	}
	if !aPresent {
		return true
	}
	return equalValues(a, b)
}

func tomlName(f reflect.StructField) (string, bool) {
	tag := f.Tag.Get("toml")
	name, _, _ := strings.Cut(tag, ",")
	if name == "-" {
		return "", false
	}
	if name == "" {
		name = strings.ToLower(f.Name)
	}
	return name, true
}

func joinKey(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func isStructLike(t reflect.Type) bool {
	return t.Kind() == reflect.Struct || (t.Kind() == reflect.Pointer && t.Elem().Kind() == reflect.Struct)
}

func derefOrZero(v reflect.Value) reflect.Value {
	if v.Kind() != reflect.Pointer {
		return v
	}
	if v.IsNil() {
		return reflect.Zero(v.Type().Elem())
	}
	return v.Elem()
}

func diffValue(before, after reflect.Value, path []pathStep, key string, ops *[]configOp) {
	if isStructLike(after.Type()) {
		b, a := derefOrZero(before), derefOrZero(after)
		for i := 0; i < a.NumField(); i++ {
			f := a.Type().Field(i)
			name, ok := tomlName(f)
			if !f.IsExported() || !ok {
				continue
			}
			diffValue(b.Field(i), a.Field(i), appendStep(path, pathStep{field: i}), joinKey(key, name), ops)
		}
		return
	}
	if after.Kind() == reflect.Map && after.Type().Key().Kind() == reflect.String {
		diffMap(before, after, path, key, ops)
		return
	}
	if !equalValues(before, after) {
		*ops = append(*ops, configOp{
			key: key, path: path, base: cloneValue(before), next: cloneValue(after),
			basePresent: true, nextPresent: true,
		})
	}
}

func diffMap(before, after reflect.Value, path []pathStep, key string, ops *[]configOp) {
	seen := map[string]bool{}
	var names []string
	for _, m := range []reflect.Value{before, after} {
		for _, k := range m.MapKeys() {
			if !seen[k.String()] {
				seen[k.String()] = true
				names = append(names, k.String())
			}
		}
	}
	sort.Strings(names)
	elem := after.Type().Elem()
	for _, name := range names {
		k := reflect.ValueOf(name).Convert(after.Type().Key())
		step := appendStep(path, pathStep{key: k, isKey: true})
		bv, av := before.MapIndex(k), after.MapIndex(k)
		sub := joinKey(key, name)
		if bv.IsValid() && av.IsValid() {
			if isStructLike(elem) {
				diffValue(bv, av, step, sub, ops)
			} else if !equalValues(bv, av) {
				*ops = append(*ops, configOp{key: sub, path: step, base: cloneValue(bv), next: cloneValue(av), basePresent: true, nextPresent: true})
			}
			continue
		}
		op := configOp{key: sub, path: step, basePresent: bv.IsValid(), nextPresent: av.IsValid()}
		if bv.IsValid() {
			op.base = cloneValue(bv)
		}
		if av.IsValid() {
			op.next = cloneValue(av)
		}
		*ops = append(*ops, op)
	}
}

func appendStep(path []pathStep, s pathStep) []pathStep {
	return append(append([]pathStep(nil), path...), s)
}

// getAt reads the value at path; present is false for a map entry that does
// not exist (the value is then the zero value of its type).
func getAt(v reflect.Value, path []pathStep) (reflect.Value, bool) {
	for i, s := range path {
		v = derefOrZero(v)
		if s.isKey {
			if v.IsNil() {
				return zeroAt(v.Type().Elem(), path[i+1:]), false
			}
			e := v.MapIndex(s.key)
			if !e.IsValid() {
				return zeroAt(v.Type().Elem(), path[i+1:]), false
			}
			v = e
			continue
		}
		v = v.Field(s.field)
	}
	return v, true
}

// zeroAt is the zero value found at path under a value of type t.
func zeroAt(t reflect.Type, path []pathStep) reflect.Value {
	for _, s := range path {
		if t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		if s.isKey {
			t = t.Elem()
		} else {
			t = t.Field(s.field).Type
		}
	}
	return reflect.Zero(t)
}

// setAt sets the value at path (present false: deletes the map entry). Maps
// and pointers on the way are created.
func setAt(v reflect.Value, path []pathStep, val reflect.Value, present bool) {
	if len(path) == 0 {
		v.Set(cloneValue(val))
		return
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
	s := path[0]
	if !s.isKey {
		setAt(v.Field(s.field), path[1:], val, present)
		return
	}
	if len(path) == 1 {
		if !present {
			if !v.IsNil() {
				v.SetMapIndex(s.key, reflect.Value{})
			}
			return
		}
		if v.IsNil() {
			v.Set(reflect.MakeMap(v.Type()))
		}
		v.SetMapIndex(s.key, cloneValue(val))
		return
	}
	if v.IsNil() {
		v.Set(reflect.MakeMap(v.Type()))
	}
	elem := reflect.New(v.Type().Elem()).Elem()
	if cur := v.MapIndex(s.key); cur.IsValid() {
		elem.Set(cloneValue(cur))
	}
	setAt(elem, path[1:], val, present)
	v.SetMapIndex(s.key, elem)
}

// equalValues compares two values of a configuration the way config.toml
// holds them: an empty list or map is an absent one, a nil pointer to a
// struct is an empty struct.
func equalValues(a, b reflect.Value) bool {
	switch a.Kind() {
	case reflect.Slice:
		if a.Len() != b.Len() {
			return false
		}
		for i := 0; i < a.Len(); i++ {
			if !equalValues(a.Index(i), b.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Map:
		if a.Len() != b.Len() {
			return false
		}
		for _, k := range a.MapKeys() {
			bv := b.MapIndex(k)
			if !bv.IsValid() || !equalValues(a.MapIndex(k), bv) {
				return false
			}
		}
		return true
	case reflect.Pointer:
		if a.Type().Elem().Kind() == reflect.Struct {
			return equalValues(derefOrZero(a), derefOrZero(b))
		}
		if a.IsNil() || b.IsNil() {
			return a.IsNil() && b.IsNil()
		}
		return equalValues(a.Elem(), b.Elem())
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			if !a.Type().Field(i).IsExported() {
				continue
			}
			if !equalValues(a.Field(i), b.Field(i)) {
				return false
			}
		}
		return true
	default:
		return a.Interface() == b.Interface()
	}
}

// cloneValue returns a copy of v that shares no pointer, slice or map with it.
func cloneValue(v reflect.Value) reflect.Value {
	if !v.IsValid() {
		return v
	}
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}
		p := reflect.New(v.Type().Elem())
		p.Elem().Set(cloneValue(v.Elem()))
		return p
	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		s := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			s.Index(i).Set(cloneValue(v.Index(i)))
		}
		return s
	case reflect.Map:
		if v.IsNil() {
			return v
		}
		m := reflect.MakeMapWithSize(v.Type(), v.Len())
		for _, k := range v.MapKeys() {
			m.SetMapIndex(k, cloneValue(v.MapIndex(k)))
		}
		return m
	case reflect.Struct:
		c := reflect.New(v.Type()).Elem()
		c.Set(v)
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				c.Field(i).Set(cloneValue(v.Field(i)))
			}
		}
		return c
	default:
		return v
	}
}
