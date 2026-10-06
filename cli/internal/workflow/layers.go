package workflow

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Layer is where a workflow document lives. Layers are ordered from the
// most general (hub) to the most specific (session options).
type Layer string

const (
	LayerHub     Layer = "hub"     // shipped with oh
	LayerTeam    Layer = "team"    // team-state (or solo space)
	LayerProject Layer = "project" // team-state, per project
	// LayerSession holds launch options (mode, runtime, inputs); never stored.
	LayerSession Layer = "session"
)

// Rank orders layers (hub < team < project < session); 0 for unknown values.
func (l Layer) Rank() int {
	switch l {
	case LayerHub:
		return 1
	case LayerTeam:
		return 2
	case LayerProject:
		return 3
	case LayerSession:
		return 4
	}
	return 0
}

// DocumentLayers are the layers that hold workflow files, most general first.
var DocumentLayers = []Layer{LayerHub, LayerTeam, LayerProject}

// IsDocumentLayer reports whether l can hold workflow files.
func (l Layer) IsDocumentLayer() bool {
	return l == LayerHub || l == LayerTeam || l == LayerProject
}

// Ref names a workflow in a layer: "hub:ticket", "team:ticket-hotfix".
type Ref struct {
	Layer Layer
	ID    string
}

func (r Ref) String() string { return string(r.Layer) + ":" + r.ID }

// ParseRef parses "<layer>:<id>".
func ParseRef(s string) (Ref, error) {
	layer, id, ok := strings.Cut(s, ":")
	if !ok || id == "" || !Layer(layer).IsDocumentLayer() {
		return Ref{}, fmt.Errorf("invalid workflow reference %q (want hub:<id>, team:<id> or project:<id>)", s)
	}
	return Ref{Layer: Layer(layer), ID: id}, nil
}

// Catalog gives access to the workflow documents of every layer.
type Catalog interface {
	Lookup(ref Ref) (*Document, bool)
}

// MemCatalog is an in-memory Catalog.
type MemCatalog struct {
	docs map[Ref]*Document
}

// NewMemCatalog returns an empty catalog.
func NewMemCatalog() *MemCatalog { return &MemCatalog{docs: map[Ref]*Document{}} }

// Lookup implements Catalog.
func (c *MemCatalog) Lookup(ref Ref) (*Document, bool) {
	d, ok := c.docs[ref]
	return d, ok
}

// Add registers doc under its layer and id; a second document with the same
// reference is refused.
func (c *MemCatalog) Add(doc *Document) Diagnostics {
	ref := doc.Ref()
	if prev, ok := c.docs[ref]; ok {
		d := errDiag("duplicate_workflow", "id", ref.String(), prev.Source.String())
		d.Source = doc.Source.String()
		d.Pos = doc.Pos("id")
		return Diagnostics{d}
	}
	c.docs[ref] = doc
	return nil
}

// Put registers doc, replacing a document with the same reference.
func (c *MemCatalog) Put(doc *Document) { c.docs[doc.Ref()] = doc }

// HasWorkflow implements WorkflowCatalog: id exists in some layer.
func (c *MemCatalog) HasWorkflow(id string) bool {
	for r := range c.docs {
		if r.ID == id {
			return true
		}
	}
	return false
}

// Refs returns every reference, by layer then id.
func (c *MemCatalog) Refs() []Ref {
	out := make([]Ref, 0, len(c.docs))
	for r := range c.docs {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Layer != out[j].Layer {
			return out[i].Layer.Rank() < out[j].Layer.Rank()
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// LoadFS parses every *.yaml / *.yml file at the root of dir in fsys into the
// catalog, under layer. A file must be named after the workflow id. Files
// with errors are reported and not added.
func (c *MemCatalog) LoadFS(fsys fs.FS, dir string, layer Layer) Diagnostics {
	return c.load(fsys, dir, layer, func(name string) string { return name })
}

// LoadDir is LoadFS on a directory of the file system; sources are reported
// with their full path.
func (c *MemCatalog) LoadDir(dir string, layer Layer) Diagnostics {
	return c.load(os.DirFS(dir), ".", layer, func(name string) string {
		return filepath.Join(dir, filepath.FromSlash(name))
	})
}

func (c *MemCatalog) load(fsys fs.FS, dir string, layer Layer, display func(string) string) Diagnostics {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		d := errDiag("read_failed", "", err)
		d.Source = display(dir)
		return Diagnostics{d}
	}
	var diags Diagnostics
	for _, e := range entries {
		ext := path.Ext(e.Name())
		if e.IsDir() || (ext != ".yaml" && ext != ".yml") {
			continue
		}
		name := path.Join(dir, e.Name())
		doc, ds := ParseFS(fsys, name, Source{Layer: layer, Path: display(name)})
		diags = append(diags, ds...)
		if doc == nil || ds.HasErrors() {
			continue
		}
		if want := strings.TrimSuffix(e.Name(), ext); doc.Spec.ID != want {
			d := errDiag("id_filename_mismatch", "id", doc.Spec.ID, e.Name())
			d.Source = display(name)
			d.Pos = doc.Pos("id")
			diags = append(diags, d)
			continue
		}
		diags = append(diags, c.Add(doc)...)
	}
	return diags
}
