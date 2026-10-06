package workflow

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// NewDraft describes the starting text of a new workflow (`oh workflow
// new`, catalogue `n`): an empty skeleton, a patch extending a workflow, or
// a copy of one.
type NewDraft struct {
	ID    string
	Layer wf.Layer
	// Extends is the reference the new workflow extends.
	Extends string
	// Copy is the reference whose text is copied.
	Copy string
}

// ErrExists is returned when the layer already has a draft or a published
// document of the id (edit it instead).
var ErrExists = errors.New("workflow already exists in this layer")

// NewDraftText returns the starting text of a new workflow; nothing is
// written (SaveDraft validates and saves it).
func (s *Service) NewDraftText(ctx context.Context, c Context, n NewDraft) (*Text, error) {
	if err := teamstate.ValidWorkflowID(n.ID); err != nil {
		return nil, fmt.Errorf("%s", i18n.Tf("cmd.workflow.new.bad_id", n.ID))
	}
	if n.Extends != "" && n.Copy != "" {
		return nil, fmt.Errorf("%s", i18n.T("cmd.workflow.new.extends_and_copy"))
	}
	if t, err := s.EditText(ctx, c, n.Layer, n.ID); err == nil {
		return nil, &kindError{kind: ErrExists, msg: i18n.Tf("cmd.workflow.new.exists", t.Ref.String())}
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	t := &Text{Ref: wf.Ref{Layer: n.Layer, ID: n.ID}, Draft: true}
	switch {
	case n.Copy != "":
		src, err := s.DocumentText(ctx, c, n.Copy)
		if err != nil {
			return nil, err
		}
		t.YAML, t.Prompt = CopyDocument(src.YAML, n.ID), src.Prompt
	case n.Extends != "":
		if _, err := wf.ParseRef(n.Extends); err != nil {
			return nil, fmt.Errorf("%s", i18n.Tf("cmd.workflow.new.bad_extends", n.Extends))
		}
		if !s.Has(ctx, c, n.Extends) {
			return nil, fmt.Errorf("%s", i18n.Tf("cmd.workflow.show.unknown", n.Extends))
		}
		t.YAML = fmt.Appendf(nil, i18n.T("cmd.workflow.new.template_extends"), n.ID, n.Extends)
	default:
		t.YAML = fmt.Appendf(nil, i18n.T("cmd.workflow.new.template_empty"), n.ID)
	}
	return t, nil
}

var (
	reTopID      = regexp.MustCompile(`(?m)^id:[^\n]*$`)
	reTopVersion = regexp.MustCompile(`(?m)^version:[^\n]*\n?`)
)

// CopyDocument renames a copied document and drops its published version.
func CopyDocument(data []byte, id string) []byte {
	out := reTopID.ReplaceAll(data, []byte("id: "+id))
	return reTopVersion.ReplaceAll(out, nil)
}

// kindError is a localized message matching a sentinel error (errors.Is).
type kindError struct {
	kind error
	msg  string
}

func (e *kindError) Error() string { return e.msg }
func (e *kindError) Unwrap() error { return e.kind }
