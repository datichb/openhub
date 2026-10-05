package workflow

import (
	"gopkg.in/yaml.v3"
)

// OrderedMap is a YAML mapping that keeps the declaration order of its keys.
//
// Order is meaningful in oh/v1 documents: inputs are shown in the launch form
// in declaration order, checkpoints are passed in declaration order, and
// agents are listed in declaration order.
type OrderedMap[T any] struct {
	keys   []string
	values map[string]T
}

// Keys returns the keys in declaration order.
func (m *OrderedMap[T]) Keys() []string {
	if m == nil {
		return nil
	}
	return append([]string(nil), m.keys...)
}

// Len returns the number of entries.
func (m *OrderedMap[T]) Len() int {
	if m == nil {
		return 0
	}
	return len(m.keys)
}

// Get returns the value for key.
func (m *OrderedMap[T]) Get(key string) (T, bool) {
	var zero T
	if m == nil || m.values == nil {
		return zero, false
	}
	v, ok := m.values[key]
	return v, ok
}

// Set inserts or replaces key. A new key is appended at the end.
func (m *OrderedMap[T]) Set(key string, value T) {
	if m.values == nil {
		m.values = make(map[string]T)
	}
	if _, ok := m.values[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

// Delete removes key if present.
func (m *OrderedMap[T]) Delete(key string) {
	if m == nil || m.values == nil {
		return
	}
	if _, ok := m.values[key]; !ok {
		return
	}
	delete(m.values, key)
	for i, k := range m.keys {
		if k == key {
			m.keys = append(m.keys[:i], m.keys[i+1:]...)
			break
		}
	}
}

// UnmarshalYAML decodes a mapping and records its key order.
//
// It uses the callback form of yaml.v3's unmarshaler on purpose: values are
// decoded by the caller's decoder, so strict mode (KnownFields) and line
// numbers in errors are preserved. yaml.v3 itself rejects duplicate keys.
func (m *OrderedMap[T]) UnmarshalYAML(unmarshal func(any) error) error {
	node, err := captureNode(unmarshal)
	if err != nil {
		return err
	}
	if node.Kind == yaml.ScalarNode && node.Tag == "!!null" {
		*m = OrderedMap[T]{}
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return nodeError(node, "expected_mapping")
	}
	var values map[string]T
	if err := unmarshal(&values); err != nil {
		return err
	}
	out := OrderedMap[T]{values: values}
	for i := 0; i+1 < len(node.Content); i += 2 {
		out.keys = append(out.keys, node.Content[i].Value)
	}
	*m = out
	return nil
}

// MarshalYAML encodes the entries in declaration order.
func (m OrderedMap[T]) MarshalYAML() (any, error) {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, k := range m.keys {
		var val yaml.Node
		if err := val.Encode(m.values[k]); err != nil {
			return nil, err
		}
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k},
			&val,
		)
	}
	return node, nil
}

// IsZero lets `omitempty` drop empty maps.
func (m OrderedMap[T]) IsZero() bool { return len(m.keys) == 0 }

// nodeCapture records the raw node through the callback unmarshaler, which
// otherwise has no way to expose it.
type nodeCapture struct{ node *yaml.Node }

func (c *nodeCapture) UnmarshalYAML(n *yaml.Node) error {
	c.node = n
	return nil
}

func captureNode(unmarshal func(any) error) (*yaml.Node, error) {
	var c nodeCapture
	if err := unmarshal(&c); err != nil {
		return nil, err
	}
	return c.node, nil
}
