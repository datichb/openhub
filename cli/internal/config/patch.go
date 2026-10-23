package config

import "reflect"

// Patch returns a function that copies onto a value the settings that differ
// between before and after, and nothing else. A view that edits a copy of a
// section (or of the whole Config) keeps the copy it started from and saves
// Patch(started, edited) with Update: the file is read as it is, the fields the
// view changed take its values, the fields it did not touch keep whatever the
// file holds, even if another part of oh changed them since the view loaded.
//
// Settings are compared one by one down the structs; a setting that is a
// pointer, a slice or a map is compared and copied as a whole (so a changed
// element replaces the list). Unexported fields are left alone.
func Patch[T any](before, after T) func(*T) {
	var ops []patchOp
	diffValues(reflect.ValueOf(&before).Elem(), reflect.ValueOf(&after).Elem(), nil, &ops)
	return func(dst *T) {
		root := reflect.ValueOf(dst).Elem()
		for _, op := range ops {
			root.FieldByIndex(op.path).Set(cloneValue(op.value))
		}
	}
}

type patchOp struct {
	path  []int
	value reflect.Value
}

func diffValues(before, after reflect.Value, path []int, ops *[]patchOp) {
	if after.Kind() == reflect.Struct && hasExportedFields(after.Type()) {
		for i := 0; i < after.NumField(); i++ {
			if !after.Type().Field(i).IsExported() {
				continue
			}
			sub := append(append([]int(nil), path...), i)
			diffValues(before.Field(i), after.Field(i), sub, ops)
		}
		return
	}
	if len(path) == 0 { // the root is a leaf: nothing to address
		return
	}
	if !reflect.DeepEqual(before.Interface(), after.Interface()) {
		*ops = append(*ops, patchOp{path: path, value: cloneValue(after)})
	}
}

func hasExportedFields(t reflect.Type) bool {
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).IsExported() {
			return true
		}
	}
	return false
}

// cloneValue returns a copy of v that shares no pointer, slice or map with it.
func cloneValue(v reflect.Value) reflect.Value {
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
