package workflow

import (
	"bytes"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

// DocEdit edits a workflow document as a YAML tree (TUI editor, P2-T14):
// comments, key order and the presence of each field (what a patch writes)
// are kept; only the edited keys change.
type DocEdit struct {
	doc *yaml.Node // document node
}

// ParseDocEdit reads a document (empty: a new mapping).
func ParseDocEdit(data []byte) (*DocEdit, error) {
	var doc yaml.Node
	if len(bytes.TrimSpace(data)) > 0 {
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, err
		}
	}
	if doc.Kind == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("the workflow document is not a YAML mapping")
	}
	return &DocEdit{doc: &doc}, nil
}

func (e *DocEdit) root() *yaml.Node { return e.doc.Content[0] }

// editLookup returns the value node of key in mapping m.
func lookup(m *yaml.Node, key string) (int, *yaml.Node) {
	if m == nil || m.Kind != yaml.MappingNode {
		return -1, nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return i, m.Content[i+1]
		}
	}
	return -1, nil
}

// Node returns the node at path (nil when absent).
func (e *DocEdit) Node(path ...string) *yaml.Node {
	n := e.root()
	for _, k := range path {
		if _, n = lookup(n, k); n == nil {
			return nil
		}
	}
	return n
}

// Has reports whether path is written in the document.
func (e *DocEdit) Has(path ...string) bool { return e.Node(path...) != nil }

// Scalar returns the scalar value at path.
func (e *DocEdit) Scalar(path ...string) (string, bool) {
	n := e.Node(path...)
	if n == nil || n.Kind != yaml.ScalarNode {
		return "", false
	}
	return n.Value, true
}

// Strings returns the scalar items of the sequence at path (a scalar is a
// one-item list).
func (e *DocEdit) Strings(path ...string) ([]string, bool) {
	n := e.Node(path...)
	switch {
	case n == nil:
		return nil, false
	case n.Kind == yaml.ScalarNode:
		return []string{n.Value}, true
	case n.Kind != yaml.SequenceNode:
		return nil, false
	}
	out := []string{}
	for _, it := range n.Content {
		if it.Kind == yaml.ScalarNode {
			out = append(out, it.Value)
		}
	}
	return out, true
}

// Keys returns the keys of the mapping at path, in document order.
func (e *DocEdit) Keys(path ...string) []string {
	n := e.Node(path...)
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	var out []string
	for i := 0; i+1 < len(n.Content); i += 2 {
		out = append(out, n.Content[i].Value)
	}
	return out
}

// Set writes value at path, creating the missing mappings. An existing
// value keeps its comments; a mapping in flow style stays in flow style.
func (e *DocEdit) Set(value any, path ...string) error {
	if len(path) == 0 {
		return errors.New("empty path")
	}
	var v yaml.Node
	if err := v.Encode(value); err != nil {
		return err
	}
	m := e.root()
	for i, k := range path {
		_, n := lookup(m, k)
		last := i == len(path)-1
		switch {
		case last && n != nil:
			v.HeadComment, v.LineComment, v.FootComment = n.HeadComment, n.LineComment, n.FootComment
			if m.Style&yaml.FlowStyle != 0 || n.Style&yaml.FlowStyle != 0 {
				flow(&v)
			}
			*n = v
			return nil
		case last:
			if m.Style&yaml.FlowStyle != 0 {
				flow(&v)
			}
			m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}, &v)
			return nil
		case n == nil:
			n = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Style: m.Style & yaml.FlowStyle}
			m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}, n)
		case n.Kind != yaml.MappingNode:
			if n.Kind == yaml.ScalarNode && n.Tag == "!!null" {
				*n = yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			} else {
				return fmt.Errorf("%v is not a mapping", path[:i+1])
			}
		}
		m = n
	}
	return nil
}

// editFlow puts a collection in flow style (inside a flow mapping).
func flow(n *yaml.Node) {
	if n.Kind == yaml.MappingNode || n.Kind == yaml.SequenceNode {
		n.Style |= yaml.FlowStyle
	}
}

// Unset removes path; mappings left empty are removed too.
func (e *DocEdit) Unset(path ...string) {
	if len(path) == 0 {
		return
	}
	parents := []*yaml.Node{e.root()}
	for _, k := range path[:len(path)-1] {
		_, n := lookup(parents[len(parents)-1], k)
		if n == nil {
			return
		}
		parents = append(parents, n)
	}
	for i := len(path) - 1; i >= 0; i-- {
		m := parents[i]
		idx, _ := lookup(m, path[i])
		if idx < 0 {
			return
		}
		m.Content = append(m.Content[:idx], m.Content[idx+2:]...)
		if len(m.Content) > 0 || i == 0 {
			return
		}
	}
}

// Rename renames the last key of path (an agent, a checkpoint, an input),
// keeping its place and value.
func (e *DocEdit) Rename(to string, path ...string) error {
	if len(path) == 0 {
		return errors.New("empty path")
	}
	m := e.root()
	if len(path) > 1 {
		m = e.Node(path[:len(path)-1]...)
	}
	if _, n := lookup(m, to); n != nil {
		return fmt.Errorf("%q already exists", to)
	}
	idx, _ := lookup(m, path[len(path)-1])
	if idx < 0 {
		return fmt.Errorf("%v not found", path)
	}
	m.Content[idx].Value = to
	return nil
}

// Bytes renders the document (2-space indentation).
func (e *DocEdit) Bytes() ([]byte, error) {
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(e.doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
