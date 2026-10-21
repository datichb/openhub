package views

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fill sets every exported field reachable from v to a non-zero value that
// depends only on the order of the walk: two fills give two equal values.
// Pointers point to a value, slices hold two elements, maps two entries.
func fill(v reflect.Value, n *int, depth int) {
	if depth > 8 {
		return
	}
	*n++
	switch v.Kind() {
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		fill(v.Elem(), n, depth+1)
	case reflect.Struct:
		if v.Type() == reflect.TypeOf(time.Time{}) {
			v.Set(reflect.ValueOf(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)))
			return
		}
		for i := range v.NumField() {
			if v.Field(i).CanSet() {
				fill(v.Field(i), n, depth+1)
			}
		}
	case reflect.Slice:
		s := reflect.MakeSlice(v.Type(), 2, 2)
		for i := range 2 {
			fill(s.Index(i), n, depth+1)
		}
		v.Set(s)
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return
		}
		m := reflect.MakeMap(v.Type())
		for range 2 {
			key := reflect.New(v.Type().Key()).Elem()
			fill(key, n, depth+1)
			elem := reflect.New(v.Type().Elem()).Elem()
			fill(elem, n, depth+1)
			m.SetMapIndex(key, elem)
		}
		v.Set(m)
	case reflect.String:
		v.SetString(fmt.Sprintf("%s-%d", v.Type().Name(), *n))
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(int64(*n))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(uint64(*n))
	case reflect.Float32, reflect.Float64:
		v.SetFloat(float64(*n) + 0.5)
	}
}

// mutate changes everything reachable from v: pointed values, slice
// elements, map entries (and adds one), strings, numbers, booleans.
func mutate(v reflect.Value, depth int) {
	if depth > 8 {
		return
	}
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			mutate(v.Elem(), depth+1)
		}
	case reflect.Struct:
		if v.Type() == reflect.TypeOf(time.Time{}) {
			v.Set(reflect.ValueOf(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)))
			return
		}
		for i := range v.NumField() {
			if v.Field(i).CanSet() {
				mutate(v.Field(i), depth+1)
			}
		}
	case reflect.Slice:
		for i := range v.Len() {
			mutate(v.Index(i), depth+1)
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			elem := reflect.New(v.Type().Elem()).Elem()
			elem.Set(v.MapIndex(key))
			mutate(elem, depth+1)
			v.SetMapIndex(key, elem)
		}
		if v.Type().Key().Kind() == reflect.String && !v.IsNil() {
			fresh := reflect.New(v.Type().Elem()).Elem()
			fill(fresh, new(int), depth+1)
			v.SetMapIndex(reflect.ValueOf("added by the test").Convert(v.Type().Key()), fresh)
		}
	case reflect.String:
		v.SetString(v.String() + " (changed)")
	case reflect.Bool:
		v.SetBool(!v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(v.Int() + 1000)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(v.Uint() + 1000)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(v.Float() + 1000)
	}
}

// checkDeepCopy fills a value, copies it, changes every field of the copy
// and checks that the original did not move.
func checkDeepCopy[T any](t *testing.T, copyOf func(*T) T) {
	t.Helper()
	build := func() *T {
		var x T
		n := 0
		fill(reflect.ValueOf(&x).Elem(), &n, 0)
		return &x
	}
	orig, pristine := build(), build()
	require.Equal(t, pristine, orig, "the fixture is deterministic")

	cp := copyOf(orig)
	require.Equal(t, *pristine, cp, "the copy equals the original")

	mutate(reflect.ValueOf(&cp).Elem(), 0)
	require.NotEqual(t, *pristine, cp, "the test changed the copy")
	assert.Equal(t, pristine, orig, "changing the copy must leave the original intact (aliased pointer, slice or map)")
}

func TestDeepCopy(t *testing.T) {
	t.Run("config", func(t *testing.T) {
		checkDeepCopy(t, deepCopyConfig)
	})
	t.Run("project", func(t *testing.T) {
		checkDeepCopy(t, deepCopyProject)
	})
	t.Run("team config", func(t *testing.T) {
		checkDeepCopy(t, deepCopyTeamConfig)
	})
	t.Run("string map", func(t *testing.T) {
		assert.Nil(t, cloneStringMap(nil), "nil map stays nil")

		orig := map[string]string{"a": "1", "b": "2"}
		cp := cloneStringMap(orig)
		cp["a"] = "changed"
		cp["c"] = "new"
		assert.Equal(t, map[string]string{"a": "1", "b": "2"}, orig, "changing the copy leaves the original intact")
	})
}
