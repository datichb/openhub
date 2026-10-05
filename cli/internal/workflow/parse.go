package workflow

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Source identifies where a document comes from.
type Source struct {
	Layer Layer `json:"layer"`
	// Path is the file the document was read from (display only).
	Path string `json:"path,omitempty"`
	// Draft marks an unpublished document (phase 2).
	Draft bool `json:"draft,omitempty"`
}

func (s Source) String() string {
	if s.Path != "" {
		return s.Path
	}
	return string(s.Layer)
}

// Document is a parsed oh/v1 file: the decoded Spec plus the position of
// every field present in the source. Presence matters for patches: a field
// written in the file overrides the parent even when it holds a zero value
// (`required: false`, `mandatory: false`).
type Document struct {
	Spec   *Spec
	Source Source
	index  nodeIndex
}

// Ref returns the reference of the document in its layer.
func (d *Document) Ref() Ref { return Ref{Layer: d.Source.Layer, ID: d.Spec.ID} }

// Has reports whether path is written in the document.
func (d *Document) Has(path string) bool {
	_, ok := d.index.pos[path]
	return ok
}

// Pos returns the position of path, or of its closest written parent.
func (d *Document) Pos(path string) Pos {
	for p := path; p != ""; p = parentPath(p) {
		if pos, ok := d.index.pos[p]; ok {
			return pos
		}
	}
	return Pos{}
}

// Paths returns every written path (document order).
func (d *Document) Paths() []string { return append([]string(nil), d.index.order...) }

// ParseFile reads and parses a workflow file.
func ParseFile(path string, layer Layer) (*Document, Diagnostics) {
	data, err := os.ReadFile(path)
	if err != nil {
		d := errDiag("read_failed", "", err)
		d.Source = path
		return nil, Diagnostics{d}
	}
	return Parse(data, Source{Layer: layer, Path: path})
}

// ParseFS reads and parses a workflow file from fsys.
func ParseFS(fsys fs.FS, name string, src Source) (*Document, Diagnostics) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		d := errDiag("read_failed", "", err)
		d.Source = src.String()
		return nil, Diagnostics{d}
	}
	return Parse(data, src)
}

// Parse decodes one oh/v1 document in strict mode: unknown fields,
// duplicate keys, wrong types and several documents in one file are
// refused. Every finding carries its line and, when known, its column.
//
// The returned document is nil only when the source is not YAML at all;
// otherwise it holds whatever could be decoded, even with errors.
func Parse(data []byte, src Source) (*Document, Diagnostics) {
	var diags Diagnostics
	finish := func(doc *Document) (*Document, Diagnostics) {
		for i := range diags {
			diags[i].Source = src.String()
			if diags[i].Pos.IsZero() && diags[i].Path != "" && doc != nil {
				diags[i].Pos = doc.Pos(diags[i].Path)
			}
		}
		diags.Sort()
		return doc, diags
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		diags = append(diags, yamlMessageDiags(err, nodeIndex{})...)
		return finish(nil)
	}
	if root.Kind == 0 || len(root.Content) == 0 {
		diags = append(diags, errDiag("empty_document", ""))
		return finish(nil)
	}
	top := root.Content[0]
	if top.Kind != yaml.MappingNode {
		d := errDiag("expected_mapping", "")
		d.Pos = Pos{Line: top.Line, Col: top.Column}
		diags = append(diags, d)
		return finish(nil)
	}

	doc := &Document{Spec: &Spec{}, Source: src, index: buildIndex(&root)}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(doc.Spec); err != nil {
		diags = append(diags, yamlMessageDiags(err, doc.index)...)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		d := errDiag("multiple_documents", "")
		if err == nil {
			d.Pos = Pos{Line: extra.Line, Col: extra.Column}
		}
		diags = append(diags, d)
	}

	s := doc.Spec
	switch {
	case !doc.Has("apiVersion"):
		diags = append(diags, errDiag("field_required", "apiVersion", "apiVersion"))
	case s.APIVersion != APIVersionV1:
		diags = append(diags, errDiag("api_version", "apiVersion", s.APIVersion, APIVersionV1))
	}
	switch {
	case !doc.Has("kind"):
		diags = append(diags, errDiag("field_required", "kind", "kind"))
	case s.Kind != KindWorkflow:
		diags = append(diags, errDiag("kind", "kind", s.Kind, KindWorkflow))
	}
	if !doc.Has("id") {
		diags = append(diags, errDiag("field_required", "id", "id"))
	}
	return finish(doc)
}

// ---------------------------------------------------------------------------
// Position index
// ---------------------------------------------------------------------------

type nodeEntry struct {
	path      string
	key       bool // mapping key (false: value or sequence item)
	scalar    bool
	line, col int
	value     string
}

type nodeIndex struct {
	pos     map[string]Pos // path → position of its key (or sequence item)
	order   []string
	entries []nodeEntry
}

func buildIndex(root *yaml.Node) nodeIndex {
	idx := nodeIndex{pos: map[string]Pos{}}
	var walk func(n *yaml.Node, prefix string)
	walk = func(n *yaml.Node, prefix string) {
		switch n.Kind {
		case yaml.DocumentNode:
			for _, c := range n.Content {
				walk(c, prefix)
			}
		case yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				k, v := n.Content[i], n.Content[i+1]
				p := joinPath(prefix, k.Value)
				idx.add(p, Pos{Line: k.Line, Col: k.Column})
				idx.entries = append(idx.entries, nodeEntry{path: p, key: true, line: k.Line, col: k.Column, value: k.Value})
				idx.addValue(p, v)
				walk(v, p)
			}
		case yaml.SequenceNode:
			for i, item := range n.Content {
				p := prefix + "[" + strconv.Itoa(i) + "]"
				idx.add(p, Pos{Line: item.Line, Col: item.Column})
				idx.addValue(p, item)
				walk(item, p)
			}
		}
	}
	walk(root, "")
	return idx
}

func (idx *nodeIndex) add(path string, pos Pos) {
	if _, ok := idx.pos[path]; !ok {
		idx.order = append(idx.order, path)
	}
	idx.pos[path] = pos
}

func (idx *nodeIndex) addValue(path string, n *yaml.Node) {
	e := nodeEntry{path: path, line: n.Line, col: n.Column, scalar: n.Kind == yaml.ScalarNode}
	if e.scalar {
		e.value = n.Value
	}
	idx.entries = append(idx.entries, e)
}

// find returns the first entry on line that matches.
func (idx nodeIndex) find(line int, match func(nodeEntry) bool) (nodeEntry, bool) {
	for _, e := range idx.entries {
		if e.line == line && match(e) {
			return e, true
		}
	}
	return nodeEntry{}, false
}

// findLast returns the last (most nested) entry on line that matches.
func (idx nodeIndex) findLast(line int, match func(nodeEntry) bool) (nodeEntry, bool) {
	for i := len(idx.entries) - 1; i >= 0; i-- {
		if e := idx.entries[i]; e.line == line && match(e) {
			return e, true
		}
	}
	return nodeEntry{}, false
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

// parentPath strips the last segment ("a.b[2]" → "a.b", "a.b" → "a").
func parentPath(p string) string {
	if strings.HasSuffix(p, "]") {
		if i := strings.LastIndex(p, "["); i >= 0 {
			return p[:i]
		}
	}
	if i := strings.LastIndex(p, "."); i >= 0 {
		return p[:i]
	}
	return ""
}

// ---------------------------------------------------------------------------
// yaml.v3 messages → diagnostics
// ---------------------------------------------------------------------------

var (
	reYAMLLine      = regexp.MustCompile(`^(?:yaml: )?line (\d+)(?:, column (\d+))?: (.*)$`)
	reUnknownField  = regexp.MustCompile(`^field (\S+) not found in type \S+$`)
	reDuplicateKey  = regexp.MustCompile(`^mapping key "(.*)" already defined at line (\d+)$`)
	reCannotDecode  = regexp.MustCompile("^cannot unmarshal !!(\\w+)(?: `(.*)`)? into (.+)$")
	reOwnCode       = regexp.MustCompile(`^oh:([a-z_]+)$`)
	reTypeErrorHead = regexp.MustCompile(`^yaml: unmarshal errors:\n`)
)

func yamlMessageDiags(err error, idx nodeIndex) Diagnostics {
	var msgs []string
	var te *yaml.TypeError
	if errors.As(err, &te) {
		msgs = te.Errors
	} else {
		msgs = []string{reTypeErrorHead.ReplaceAllString(err.Error(), "")}
	}
	out := make(Diagnostics, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, yamlMessageDiag(strings.TrimSpace(m), idx))
	}
	return out
}

func yamlMessageDiag(msg string, idx nodeIndex) Diagnostic {
	m := reYAMLLine.FindStringSubmatch(msg)
	if m == nil {
		return errDiag("syntax", "", strings.TrimPrefix(msg, "yaml: "))
	}
	line, _ := strconv.Atoi(m[1])
	col, _ := strconv.Atoi(m[2])
	rest := m[3]

	var d Diagnostic
	var entry nodeEntry
	var found bool
	switch {
	case reUnknownField.MatchString(rest):
		field := reUnknownField.FindStringSubmatch(rest)[1]
		d = errDiag("unknown_field", "", field)
		entry, found = idx.find(line, func(e nodeEntry) bool { return e.key && e.value == field })
	case reDuplicateKey.MatchString(rest):
		sm := reDuplicateKey.FindStringSubmatch(rest)
		d = errDiag("duplicate_key", "", sm[1], sm[2])
		// The duplicate is not in the index (yaml keeps one); find it by value.
		entry, found = idx.find(line, func(e nodeEntry) bool { return e.key && e.value == sm[1] })
	case reCannotDecode.MatchString(rest):
		sm := reCannotDecode.FindStringSubmatch(rest)
		tag, value := sm[1], sm[2]
		if value == "" {
			// Non-scalar value: yaml.v3 only gives its tag.
			display := map[string]string{"map": "mapping", "seq": "list"}[tag]
			if display == "" {
				display = tag
			}
			d = errDiag("invalid_type", "", display, friendlyType(sm[3]))
			entry, found = idx.findLast(line, func(e nodeEntry) bool { return !e.key && !e.scalar })
			break
		}
		d = errDiag("invalid_type", "", value, friendlyType(sm[3]))
		prefix := strings.TrimSuffix(value, "...")
		entry, found = idx.find(line, func(e nodeEntry) bool {
			return !e.key && e.scalar && (e.value == value || (prefix != value && strings.HasPrefix(e.value, prefix)))
		})
	case reOwnCode.MatchString(rest):
		code := reOwnCode.FindStringSubmatch(rest)[1]
		d = errDiag(code, "")
		entry, found = idx.find(line, func(e nodeEntry) bool { return !e.key && (col == 0 || e.col == col) })
	default:
		d = errDiag("syntax", "", rest)
	}
	d.Pos = Pos{Line: line, Col: col}
	if found {
		d.Path = entry.path
		if col == 0 {
			d.Pos.Col = entry.col
		}
	}
	return d
}

// friendlyType turns a Go type from a yaml.v3 message into a schema word.
func friendlyType(goType string) string {
	switch {
	case strings.HasPrefix(goType, "[]"):
		return "list"
	case strings.HasPrefix(goType, "map["), strings.HasPrefix(goType, "workflow."):
		switch goType {
		case "workflow.LocalizedText":
			return "text"
		case "workflow.Risk", "workflow.Isolation", "workflow.Category", "workflow.RemotePolicy",
			"workflow.InputType", "workflow.OutputType", "workflow.Runtime", "workflow.AgentRole",
			"workflow.AgentMode", "workflow.CheckpointBehavior":
			return "string"
		}
		return "mapping"
	case strings.HasPrefix(goType, "int"), strings.HasPrefix(goType, "uint"):
		return "integer"
	case strings.HasPrefix(goType, "float"):
		return "number"
	}
	return goType
}

// nodeError reports a schema error at node through yaml.v3's error list, so
// that decoding goes on and every error is collected.
func nodeError(node *yaml.Node, code string) error {
	return &yaml.TypeError{Errors: []string{fmt.Sprintf("line %d, column %d: oh:%s", node.Line, node.Column, code)}}
}
