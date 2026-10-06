package workflow

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/datichb/openhub/cli/internal/teamstate"
	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// Version is one published version of a workflow (P2-T06).
type Version struct {
	Ref     wf.Ref              `json:"ref"`
	Version int                 `json:"version"`
	Current bool                `json:"current,omitempty"`
	Entry   teamstate.LockEntry `json:"entry"`
	// Path is the YAML file of the version (published file or history copy).
	Path string `json:"path"`
}

// History lists the versions of id ("<id>", "team:<id>", "project:<id>"),
// most recent first; the current published version comes first when the
// workflow is not archived.
func (s *Service) History(ctx context.Context, c Context, id string) ([]Version, error) {
	ts, err := s.requireTeam(ctx, c)
	if err != nil {
		return nil, err
	}
	scope, bare, err := s.publishedScope(ts, id)
	if err != nil {
		return nil, err
	}
	ref := wf.Ref{Layer: scope.Layer(), ID: bare}
	lock, err := ts.Repo.ReadWorkflowLock()
	if err != nil {
		return nil, err
	}
	var out []Version
	if cur, ok := lock.Get(scope, bare); ok {
		rel, _ := teamstate.PublishedRel(scope, bare)
		out = append(out, Version{Ref: ref, Version: cur.Version, Current: true, Entry: cur, Path: filepath.Join(ts.Repo.Path(), rel)})
	}
	hist, err := ts.Repo.ListHistory(scope, bare)
	if err != nil {
		return nil, err
	}
	for i := len(hist) - 1; i >= 0; i-- {
		h := hist[i]
		if len(out) > 0 && out[0].Current && h.Version == out[0].Version {
			continue
		}
		e := h.Entry
		e.Version = h.Version
		out = append(out, Version{Ref: ref, Version: h.Version, Entry: e, Path: h.Path})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNotPublished, ref)
	}
	return out, nil
}
