// Package bdschema is the contract between oh and the JSON output of bd:
// the record schema of the bd version oh was built for (schema_bd.json,
// `bd schema` of that version) and the fields oh reads from it. A newer bd
// that drops or renames one of these fields would make oh misread tickets
// silently: `oh doctor` and the tests compare the installed bd with it.
package bdschema

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// Regenerate with: bd schema > internal/beads/bdschema/schema_bd.json
//
//go:embed schema_bd.json
var embedded []byte

// Schema is the output of `bd schema`.
type Schema struct {
	Version int             `json:"schema_version"`
	Types   map[string]Type `json:"types"`
}

// Type is the JSON Schema of one record type (issue, dependency).
type Type struct {
	Properties map[string]Prop `json:"properties"`
}

// Prop is one property; Type is a JSON type name or a list of them (none
// for a reference or a boolean schema such as `true`).
type Prop struct {
	Type json.RawMessage `json:"type"`
}

// UnmarshalJSON accepts boolean schemas (`"metadata": true`).
func (p *Prop) UnmarshalJSON(data []byte) error {
	var obj struct {
		Type json.RawMessage `json:"type"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		var b bool
		if json.Unmarshal(data, &b) == nil {
			*p = Prop{}
			return nil
		}
		return err
	}
	p.Type = obj.Type
	return nil
}

// Types are the JSON type names of a property ("string", "integer"…;
// none for a reference).
func (p Prop) Types() []string {
	var one string
	if json.Unmarshal(p.Type, &one) == nil {
		return []string{one}
	}
	var many []string
	_ = json.Unmarshal(p.Type, &many)
	return many
}

// Parse reads the output of `bd schema`.
func Parse(data []byte) (*Schema, error) {
	var s Schema
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("bd schema: %w", err)
	}
	if len(s.Types) == 0 {
		return nil, fmt.Errorf("bd schema: no record type")
	}
	return &s, nil
}

// Embedded is the schema of the bd version oh was built for.
func Embedded() *Schema {
	s, err := Parse(embedded)
	if err != nil {
		panic(err)
	}
	return s
}

// Used are the record fields oh reads from bd's JSON output, by record
// type, with the JSON types oh accepts (the decoders of internal/beads,
// services/remote and the journal mode of the fake bd).
var Used = map[string]map[string][]string{
	"issue": {
		"id": {"string"}, "title": {"string"}, "status": {"string"}, "priority": {"integer", "string"},
		"issue_type": {"string"}, "labels": {"array"}, "external_ref": {"string"}, "description": {"string"},
		"acceptance_criteria": {"string"}, "notes": {"string"}, "design": {"string"}, "estimated_minutes": {"integer"},
		"assignee": {"string"}, "owner": {"string"}, "close_reason": {"string"}, "dependencies": {"array"},
		"updated_at": {"string"},
	},
	"dependency": {
		"id": {"string"}, "depends_on_id": {"string"},
	},
}

// ViewFields are added by bd commands to the records they print (show,
// list, children) and are not part of the record schema: oh reads parent
// and revision (replay of remote sessions), and dependency_type in the
// dependencies printed by show.
var ViewFields = []string{"parent", "revision", "dependency_type", "dependency_count", "dependent_count", "comment_count"}

// IsViewField reports a field added by commands (see ViewFields).
func IsViewField(name string) bool {
	for _, f := range ViewFields {
		if f == name {
			return true
		}
	}
	return false
}

// Missing lists the fields oh reads that s lacks or types differently
// ("issue.priority (boolean)").
func Missing(s *Schema) []string {
	var out []string
	for typ, fields := range Used {
		t, ok := s.Types[typ]
		for name, accepted := range fields {
			if !ok {
				out = append(out, typ+"."+name)
				continue
			}
			p, ok := t.Properties[name]
			if !ok {
				out = append(out, typ+"."+name)
				continue
			}
			if got := p.Types(); len(got) > 0 && !overlap(got, accepted) {
				out = append(out, fmt.Sprintf("%s.%s (%s)", typ, name, strings.Join(got, "|")))
			}
		}
	}
	sort.Strings(out)
	return out
}

func overlap(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
		}
	}
	return false
}

// Uncovered lists the JSON fields of a decoder struct that are neither in
// Used[typ] nor view fields: a struct of oh that reads a new field of bd
// must declare it in Used, so that `oh doctor` checks it.
func Uncovered(v any, typ string) []string {
	var out []string
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	for i := 0; i < t.NumField(); i++ {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		if _, ok := Used[typ][name]; !ok && !IsViewField(name) {
			out = append(out, name)
		}
	}
	return out
}
