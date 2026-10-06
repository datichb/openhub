package workflow

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// Publication (P2-T04, O14), restoration (P2-T06) and archiving: each runs
// as a team-state transaction — pull, rebuild from disk (next version,
// revalidation), commit, push; rebuilt when another member pushed first;
// queued when the remote cannot be reached.

// Operation kinds (teamstate.QueuedOp.Kind).
const (
	OpPublish = "publish"
	OpRestore = "restore"
	OpArchive = "archive"
)

var (
	// ErrUnknownVersion is returned by Restore for a version not in history.
	ErrUnknownVersion = errors.New("unknown workflow version")
	// ErrNotPublished is returned by Archive / History for an id never
	// published in the scope.
	ErrNotPublished = errors.New("workflow not published")
	// ErrSharedPrompt refuses to overwrite a prompt template used by another
	// published workflow.
	ErrSharedPrompt = errors.New("prompt template shared with another workflow")
)

// Publication is the result of Publish, Restore or Archive.
type Publication struct {
	Kind    string    `json:"kind"`
	Ref     wf.Ref    `json:"ref"`
	Version int       `json:"version"`
	By      string    `json:"by"`
	At      time.Time `json:"at"`
	Message string    `json:"message,omitempty"`
	// RestoredFrom is the history version a restore republished.
	RestoredFrom int `json:"restored_from,omitempty"`
	// Impact compares the new version with the previous one.
	Impact ImpactReport `json:"impact"`
	// Diagnostics are the warnings of the revalidation (and the errors of
	// published workflows built on this one).
	Diagnostics wf.Diagnostics `json:"diagnostics,omitempty"`
	// Queued: the remote could not be reached; the operation is replayed by
	// FlushQueue (draft kept).
	Queued bool `json:"queued,omitempty"`
}

// Publish publishes the current member's draft id ("<id>", "team:<id>",
// "project:<id>"; bare: project draft first, then team) as the next
// version, with message.
func (s *Service) Publish(ctx context.Context, c Context, draftID, message string) (*Publication, error) {
	ts, err := s.requireTeam(ctx, c)
	if err != nil {
		return nil, err
	}
	scope, id, err := s.draftScope(ts, draftID)
	if err != nil {
		return nil, err
	}
	return s.run(ctx, ts, teamstate.QueuedOp{Kind: OpPublish, Scope: scope.String(), ID: id, Member: ts.Member, Message: message})
}

// Restore republishes version of id as a new version (event
// workflow.restored).
func (s *Service) Restore(ctx context.Context, c Context, id string, version int) (*Publication, error) {
	ts, err := s.requireTeam(ctx, c)
	if err != nil {
		return nil, err
	}
	scope, bare, err := s.publishedScope(ts, id)
	if err != nil {
		return nil, err
	}
	msg := i18n.Tf("teamstate.workflow.restore_message", version)
	return s.run(ctx, ts, teamstate.QueuedOp{Kind: OpRestore, Scope: scope.String(), ID: bare, Member: ts.Member, Message: msg, Version: version})
}

// Archive withdraws a published workflow: its file moves to history (it can
// be restored), its lock entry is removed.
func (s *Service) Archive(ctx context.Context, c Context, id, message string) (*Publication, error) {
	ts, err := s.requireTeam(ctx, c)
	if err != nil {
		return nil, err
	}
	scope, bare, err := s.publishedScope(ts, id)
	if err != nil {
		return nil, err
	}
	return s.run(ctx, ts, teamstate.QueuedOp{Kind: OpArchive, Scope: scope.String(), ID: bare, Member: ts.Member, Message: message})
}

// FlushQueue replays the operations queued while offline, oldest first. It
// stops at the first one that still cannot reach the remote; an operation
// that fails otherwise (validation) is dropped and its error returned.
func (s *Service) FlushQueue(ctx context.Context, c Context) ([]*Publication, []error) {
	ts, err := s.requireTeam(ctx, c)
	if err != nil {
		return nil, []error{err}
	}
	ops, err := ts.Repo.Queue()
	if err != nil {
		return nil, []error{err}
	}
	var pubs []*Publication
	var errs []error
	for _, op := range ops {
		if op.Member != ts.Member {
			continue
		}
		opTS := ts
		if scope, err := teamstate.ParseScope(op.Scope); err == nil && scope.IsProject() && ts.Project != scope.Project {
			// Replayed from another context (TUI synchronization): load the
			// project layer of the operation.
			cp := *ts
			cp.Project = scope.Project
			opTS = &cp
		}
		p, err := s.apply(ctx, opTS, op)
		if errors.Is(err, teamstate.ErrOffline) {
			break
		}
		_ = ts.Repo.Dequeue(op.Scope, op.ID)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s %s:%s: %w", op.Kind, op.Scope, op.ID, err))
			continue
		}
		pubs = append(pubs, p)
	}
	return pubs, errs
}

// run applies op, queuing it when the remote cannot be reached.
func (s *Service) run(ctx context.Context, ts *TeamState, op teamstate.QueuedOp) (*Publication, error) {
	p, err := s.apply(ctx, ts, op)
	if !errors.Is(err, teamstate.ErrOffline) {
		return p, err
	}
	if qerr := ts.Repo.Enqueue(op); qerr != nil {
		return nil, fmt.Errorf("%w (queue: %v)", err, qerr)
	}
	scope, _ := teamstate.ParseScope(op.Scope)
	return &Publication{Kind: op.Kind, Ref: wf.Ref{Layer: scope.Layer(), ID: op.ID}, By: op.Member, Message: op.Message, Queued: true}, nil
}

// apply runs op as one transaction.
func (s *Service) apply(ctx context.Context, ts *TeamState, op teamstate.QueuedOp) (*Publication, error) {
	scope, err := teamstate.ParseScope(op.Scope)
	if err != nil {
		return nil, err
	}
	var pub *Publication
	err = ts.Repo.Transact(ctx, func(ctx context.Context, tx *teamstate.Tx) (teamstate.TxResult, error) {
		pub = nil
		p, msg, err := s.build(ctx, ts, tx, scope, op)
		if err != nil {
			return teamstate.TxResult{}, err
		}
		pub = p
		return teamstate.TxResult{Message: msg}, nil
	})
	if err != nil {
		return nil, err
	}
	return pub, nil
}

// build writes one operation in the transaction and revalidates it.
func (s *Service) build(_ context.Context, ts *TeamState, tx *teamstate.Tx, scope teamstate.WorkflowScope, op teamstate.QueuedOp) (*Publication, string, error) {
	if err := tx.CheckPublish(op.Member); err != nil {
		return nil, "", err
	}
	ref := wf.Ref{Layer: scope.Layer(), ID: op.ID}
	lock, err := tx.Lock()
	if err != nil {
		return nil, "", err
	}
	cur, hasCur := lock.Get(scope, op.ID)
	history, err := tx.Repo().ListHistory(scope, op.ID)
	if err != nil {
		return nil, "", err
	}

	// Previous version, as the team sees it now (after the pull).
	before, err := s.txCatalog(ts, tx)
	if err != nil {
		return nil, "", err
	}
	var from *wf.Resolved
	if hasCur {
		from, _ = wf.Check(before.docs, ref, nil, before.env)
	}

	now := time.Now().UTC()
	pub := &Publication{Kind: op.Kind, Ref: ref, By: op.Member, At: now, Message: op.Message}
	if hasCur {
		if err := archiveCurrent(tx, scope, op.ID, cur); err != nil {
			return nil, "", err
		}
	}

	if op.Kind == OpArchive {
		if !hasCur {
			return nil, "", fmt.Errorf("%w: %s", ErrNotPublished, ref)
		}
		rel, _ := teamstate.PublishedRel(scope, op.ID)
		if err := tx.Remove(rel); err != nil {
			return nil, "", err
		}
		lock.Delete(scope, op.ID)
		if err := tx.WriteLock(lock); err != nil {
			return nil, "", err
		}
		pub.Version = cur.Version
		after, err := s.txCatalog(ts, tx)
		if err != nil {
			return nil, "", err
		}
		pub.Diagnostics = dependents(after, ref)
		if err := s.event(tx, scope, teamstate.EventWorkflowArchived, pub); err != nil {
			return nil, "", err
		}
		return pub, fmt.Sprintf("workflow: archive %s:%s v%d by %s", scope, op.ID, cur.Version, op.Member), nil
	}

	// Content of the new version.
	var yaml, prompt []byte
	switch op.Kind {
	case OpPublish:
		rel, _ := teamstate.DraftRel(scope, op.Member, op.ID)
		if yaml, err = tx.ReadFile(rel); err != nil {
			return nil, "", fmt.Errorf("%w: %s (%s)", ErrNoDraft, ref, op.Member)
		}
		prel, _ := teamstate.DraftPromptRel(scope, op.Member, op.ID)
		prompt, _ = tx.ReadFile(prel)
	case OpRestore:
		var h *teamstate.HistoryVersion
		for i := range history {
			if history[i].Version == op.Version {
				h = &history[i]
			}
		}
		if h == nil {
			return nil, "", fmt.Errorf("%w: %s v%d", ErrUnknownVersion, ref, op.Version)
		}
		if yaml, err = readAbs(tx, h.Path); err != nil {
			return nil, "", err
		}
		if h.Prompt != "" {
			prompt, _ = readAbs(tx, h.Prompt)
		}
		pub.RestoredFrom = op.Version
	default:
		return nil, "", fmt.Errorf("unknown operation %q", op.Kind)
	}

	version := cur.Version
	for _, h := range history {
		version = max(version, h.Version)
	}
	version++
	pub.Version = version
	yaml = setVersion(yaml, version)

	rel, _ := teamstate.PublishedRel(scope, op.ID)
	if err := tx.WriteFile(rel, yaml); err != nil {
		return nil, "", err
	}
	if len(prompt) > 0 {
		if err := writeOwnPrompt(tx, scope, op.ID, yaml, prompt, lock); err != nil {
			return nil, "", err
		}
	}
	entry, err := tx.Seal(scope, op.ID, teamstate.LockMeta{Version: version, PublishedBy: op.Member, PublishedAt: now, Message: op.Message})
	if err != nil {
		return nil, "", err
	}

	// Revalidation against the team-state after the pull.
	after, err := s.txCatalog(ts, tx)
	if err != nil {
		return nil, "", err
	}
	for _, d := range after.diags {
		if d.Source == filepath.Join(tx.Path(), rel) {
			return nil, "", &InvalidError{Ref: ref.String(), Diagnostics: wf.Diagnostics{d}}
		}
	}
	to, diags := wf.Check(after.docs, ref, nil, after.env)
	diags.Sort()
	if to == nil || diags.HasErrors() {
		return nil, "", &InvalidError{Ref: ref.String(), Diagnostics: diags}
	}
	diags = append(diags, dependents(after, ref)...)
	pub.Diagnostics = diags
	var fromSpec *wf.Spec
	if from != nil {
		fromSpec = from.Spec
	}
	pub.Impact = impactOf(fromSpec, to.Spec, after.env.Agents)
	if after.bricks != nil {
		prev := teamBricksUsed(fromSpec, before.env.Agents, after.bricks.Team)
		pub.Impact.NewBricks = missing(teamBricksUsed(to.Spec, after.env.Agents, after.bricks.Team), prev)
	}
	if fromSpec != nil && cur.PromptHash != entry.PromptHash && samePrompt(fromSpec.Prompt, to.Spec.Prompt) {
		pub.Impact.add(ImpactInfo, "prompt_changed", "prompt")
	}

	event := teamstate.EventWorkflowPublished
	verb := "publish"
	if op.Kind == OpPublish {
		drel, _ := teamstate.DraftRel(scope, op.Member, op.ID)
		prel, _ := teamstate.DraftPromptRel(scope, op.Member, op.ID)
		if err := tx.Remove(drel); err != nil {
			return nil, "", err
		}
		if err := tx.Remove(prel); err != nil {
			return nil, "", err
		}
	} else {
		event, verb = teamstate.EventWorkflowRestored, fmt.Sprintf("restore v%d as", op.Version)
	}
	if err := s.event(tx, scope, event, pub); err != nil {
		return nil, "", err
	}
	return pub, fmt.Sprintf("workflow: %s %s:%s v%d by %s", verb, scope, op.ID, version, op.Member), nil
}

// archiveCurrent copies the published version to history (document, prompt
// template, lock entry).
func archiveCurrent(tx *teamstate.Tx, scope teamstate.WorkflowScope, id string, cur teamstate.LockEntry) error {
	rel, _ := teamstate.PublishedRel(scope, id)
	data, err := tx.ReadFile(rel)
	if err != nil {
		return fmt.Errorf("reading the published version: %w", err)
	}
	hrel, err := teamstate.HistoryRel(scope, id, cur.Version)
	if err != nil {
		return err
	}
	if err := tx.WriteFile(hrel, data); err != nil {
		return err
	}
	if prel, err := tx.Repo().PromptOfFile(scope, data); err == nil && prel != "" {
		if pdata, err := tx.ReadFile(prel); err == nil {
			hp, _ := teamstate.HistoryPromptRel(scope, id, cur.Version)
			if err := tx.WriteFile(hp, pdata); err != nil {
				return err
			}
		}
	}
	meta, err := teamstate.MarshalLockEntry(cur)
	if err != nil {
		return err
	}
	mrel, _ := teamstate.HistoryMetaRel(scope, id, cur.Version)
	return tx.WriteFile(mrel, meta)
}

// writeOwnPrompt writes the prompt copy of a draft or history version to the
// template the document references, unless another published workflow uses
// that template.
func writeOwnPrompt(tx *teamstate.Tx, scope teamstate.WorkflowScope, id string, yaml, prompt []byte, lock *teamstate.WorkflowLock) error {
	target, err := tx.Repo().PromptOfFile(scope, yaml)
	if err != nil || target == "" {
		return err // prompt.text: the copy is not used
	}
	for _, other := range lock.IDs(scope) {
		if other == id {
			continue
		}
		orel, _ := teamstate.PublishedRel(scope, other)
		odata, err := tx.ReadFile(orel)
		if err != nil {
			continue
		}
		if p, _ := tx.Repo().PromptOfFile(scope, odata); p == target {
			return fmt.Errorf("%w: %s (%s)", ErrSharedPrompt, target, other)
		}
	}
	return tx.WriteFile(target, prompt)
}

// txCatalog loads the hub and the published team-state layers inside a
// transaction (no drafts: a publication never depends on a draft).
func (s *Service) txCatalog(ts *TeamState, tx *teamstate.Tx) (*catalog, error) {
	cat, err := s.loadHub()
	if err != nil {
		return nil, err
	}
	cat.diags = append(cat.diags, tx.LoadWorkflowScopes(cat.docs, ts.scopes()...)...)
	cat.env.Prompts = teamstate.PromptSource{Repo: tx.Repo(), Fallback: cat.env.Prompts}
	cat.team = ts
	if err := s.useTeamBricks(cat, ts); err != nil {
		return nil, err
	}
	return cat, nil
}

// dependents reports the errors of the published workflows built on ref
// (warnings: they are the authors' to fix, the publication goes on).
func dependents(cat *catalog, ref wf.Ref) wf.Diagnostics {
	var out wf.Diagnostics
	for _, other := range cat.docs.Refs() {
		if other == ref || other.Layer.Rank() <= ref.Layer.Rank() {
			continue
		}
		r, diags := wf.Check(cat.docs, other, nil, cat.env)
		uses := r == nil && other.ID == ref.ID
		if r != nil {
			for _, c := range r.Chain {
				uses = uses || c == ref
			}
		}
		if !uses || !diags.HasErrors() {
			continue
		}
		for _, d := range diags.Errors() {
			d.Severity = wf.SeverityWarning
			d.Message = i18n.Tf("teamstate.workflow.dependent_broken", other.String(), d.Message)
			out = append(out, d)
		}
	}
	return out
}

// event appends the workflow event of a publication.
func (s *Service) event(tx *teamstate.Tx, scope teamstate.WorkflowScope, kind string, p *Publication) error {
	data := map[string]any{"workflow": p.Ref.ID, "scope": scope.String(), "version": p.Version}
	if p.Message != "" {
		data["message"] = p.Message
	}
	if p.RestoredFrom > 0 {
		data["restored_from"] = p.RestoredFrom
	}
	return tx.AppendEvent(teamstate.Event{Timestamp: p.At, Actor: p.By, Type: kind, Project: scope.EventProject(), Data: data})
}

var (
	reVersionLine = regexp.MustCompile(`(?m)^version:[^\n]*$`)
	reIDLine      = regexp.MustCompile(`(?m)^id:[^\n]*$`)
)

// setVersion writes `version: v` in a top-level block document (replacing
// the line, else after `id:`), keeping the rest of the text untouched.
func setVersion(data []byte, v int) []byte {
	line := []byte("version: " + strconv.Itoa(v))
	if reVersionLine.Match(data) {
		return reVersionLine.ReplaceAll(data, line)
	}
	if loc := reIDLine.FindIndex(data); loc != nil {
		out := append([]byte{}, data[:loc[1]]...)
		out = append(out, '\n')
		out = append(out, line...)
		return append(out, data[loc[1]:]...)
	}
	return append(append(line, '\n'), data...)
}

func readAbs(tx *teamstate.Tx, abs string) ([]byte, error) {
	rel, err := filepath.Rel(tx.Path(), abs)
	if err != nil {
		return nil, err
	}
	return tx.ReadFile(rel)
}

// requireTeam returns the team-state of c or ErrNoTeamState.
func (s *Service) requireTeam(ctx context.Context, c Context) (*TeamState, error) {
	ts, err := s.teamState(ctx, c)
	if err != nil {
		return nil, err
	}
	if ts == nil || ts.Member == "" {
		return nil, ErrNoTeamState
	}
	return ts, nil
}

// draftScope finds the scope of the current member's draft id.
func (s *Service) draftScope(ts *TeamState, id string) (teamstate.WorkflowScope, string, error) {
	if ref, err := wf.ParseRef(id); err == nil {
		scope, err := ts.scopeOf(ref.Layer)
		return scope, ref.ID, err
	}
	drafts, err := ts.Repo.ListDrafts(ts.Member, ts.scopes()...)
	if err != nil {
		return teamstate.WorkflowScope{}, "", err
	}
	for i := len(drafts) - 1; i >= 0; i-- { // project scope last in scopes(): most specific first
		if drafts[i].ID == id {
			return drafts[i].Scope, id, nil
		}
	}
	return teamstate.WorkflowScope{}, "", fmt.Errorf("%w: %s", ErrNoDraft, id)
}

// publishedScope finds the scope where id is published (project first).
func (s *Service) publishedScope(ts *TeamState, id string) (teamstate.WorkflowScope, string, error) {
	if ref, err := wf.ParseRef(id); err == nil {
		scope, err := ts.scopeOf(ref.Layer)
		return scope, ref.ID, err
	}
	lock, err := ts.Repo.ReadWorkflowLock()
	if err != nil {
		return teamstate.WorkflowScope{}, "", err
	}
	scopes := ts.scopes()
	for i := len(scopes) - 1; i >= 0; i-- {
		if _, ok := lock.Get(scopes[i], id); ok {
			return scopes[i], id, nil
		}
		if h, _ := ts.Repo.ListHistory(scopes[i], id); len(h) > 0 {
			return scopes[i], id, nil
		}
	}
	return teamstate.WorkflowScope{}, "", fmt.Errorf("%w: %s", ErrNotPublished, id)
}
