package workflow

import (
	"fmt"
	"slices"
)

// Method of a session (R7): chosen at launch with the enum input `method`,
// it keeps the checkpoints declared for it (`checkpoints.<id>.method`) and
// drops them from the sessions of another method.
//
//	inputs:
//	  method: { type: enum, values: [standard, tdd], default: standard }
//	checkpoints:
//	  cp-seams: { method: tdd, mode: { semi-auto: pause } }
//	  cp-red:
//	    method: tdd
//	    verify: { from: stack, expect: fail, must_touch_tests: true, since: cp-seams }
//
// The method is evaluated by oh at launch (CP-R7): the bundle of a session
// holds only the checkpoints of its method, so a session of another method
// is the same as without them.

// Method is a way of working of a session.
type Method string

const (
	// MethodStandard is the method of a session without a method input,
	// or launched with `method: standard`: no method checkpoint.
	MethodStandard Method = "standard"
	// MethodTDD: the developer presents the seams (cp-seams), writes the
	// tests first and oh checks they fail (cp-red), then implements.
	MethodTDD Method = "tdd"
)

// MethodInput is the input that chooses the method of a session.
const MethodInput = "method"

// Methods lists the methods a checkpoint may be declared for.
var Methods = []Method{MethodTDD}

// Valid reports whether m is a method a checkpoint may be declared for.
func (m Method) Valid() bool { return slices.Contains(Methods, m) }

// Standard reports whether m is the standard method ("" included).
func (m Method) Standard() bool { return m == "" || m == MethodStandard }

// MethodOf returns the method of a session given its inputs: the `method`
// input when given, else its default (standard when the workflow has none).
func (s *Spec) MethodOf(inputs map[string]any) Method {
	in, ok := s.Inputs.Get(MethodInput)
	if !ok || in.Type != InputEnum {
		return MethodStandard
	}
	v, given := inputs[MethodInput]
	if !given {
		v = in.Default
	}
	if v == nil {
		return MethodStandard
	}
	if m := Method(fmt.Sprint(v)); !m.Standard() {
		return m
	}
	return MethodStandard
}

// HasMethod reports whether a checkpoint of s is declared for m.
func (s *Spec) HasMethod(m Method) bool {
	for _, k := range s.Checkpoints.Keys() {
		if cp, _ := s.Checkpoints.Get(k); cp.Method != "" && cp.Method == m {
			return true
		}
	}
	return false
}

// ForMethod returns s as a session of method m sees it: the checkpoints of
// another method are dropped (s itself is never changed; s is returned as
// is when nothing is dropped).
func (s *Spec) ForMethod(m Method) *Spec {
	if s == nil {
		return nil
	}
	if m.Standard() {
		m = ""
	}
	var drop []string
	for _, k := range s.Checkpoints.Keys() {
		if cp, _ := s.Checkpoints.Get(k); cp.Method != "" && cp.Method != m {
			drop = append(drop, k)
		}
	}
	if len(drop) == 0 {
		return s
	}
	c := s.Clone()
	for _, k := range drop {
		c.Checkpoints.Delete(k)
	}
	return c
}
