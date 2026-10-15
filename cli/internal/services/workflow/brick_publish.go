package workflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/teamstate"
	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// Publication of team bricks (ADR-054): a team-state transaction (pull,
// rebuild, commit, push, rebuilt when another member pushed first), the
// governance of the workflows ([governance] publish), a strict
// revalidation against the catalogue after the pull, the previous version
// kept in catalog/history/ and the new one sealed in catalog.lock.

// BrickPreview is what a publication would do.
type BrickPreview struct {
	Ref         BrickRef       `json:"ref"`
	Family      string         `json:"family,omitempty"`
	Current     int            `json:"current,omitempty"` // 0 = first publication
	Next        int            `json:"next"`
	Diagnostics wf.Diagnostics `json:"diagnostics,omitempty"`
	// Shadows is the hub brick the brick replaces ("" = none).
	Shadows string `json:"shadows,omitempty"`
	// Unsealed lists the bricks committed by hand that block a first
	// publication (adopt them first).
	Unsealed []string `json:"unsealed,omitempty"`
}

// BrickPublication is the result of PublishBrick or AdoptBricks.
type BrickPublication struct {
	Ref         BrickRef       `json:"ref"`
	Version     int            `json:"version,omitempty"`
	By          string         `json:"by"`
	At          time.Time      `json:"at"`
	Message     string         `json:"message,omitempty"`
	Shadows     string         `json:"shadows,omitempty"`
	Diagnostics wf.Diagnostics `json:"diagnostics,omitempty"`
	// Adopted lists the bricks sealed by AdoptBricks.
	Adopted []string `json:"adopted,omitempty"`
}

// PreviewBrick checks the member's draft of arg as PublishBrick would.
func (s *Service) PreviewBrick(ctx context.Context, c Context, arg string) (*BrickPreview, error) {
	ts, err := s.requireTeam(ctx, c)
	if err != nil {
		return nil, err
	}
	l, err := s.lookupBrick(ts, arg)
	if err != nil {
		return nil, err
	}
	if l.Draft == "" {
		return nil, fmt.Errorf("%w: %s", ErrNoDraft, l.Ref)
	}
	root := ts.Repo.Path()
	p := &BrickPreview{Ref: l.Ref, Family: l.Family}
	lock, sealed, err := teamstate.ReadBricksLock(root)
	if err != nil {
		return nil, err
	}
	if !sealed {
		p.Unsealed = unsealedOthers(root, l.Ref)
	}
	if e, ok := lock.Get(string(l.Ref.Kind), l.Ref.ID); ok {
		p.Current = e.Version
	}
	p.Next = nextBrickVersion(root, l.Ref, p.Current)
	data, err := readRel(root, l.Draft)
	if err != nil {
		return nil, err
	}
	in := BrickInput{Ref: l.Ref, Family: l.Family, Data: data}
	if p.Diagnostics, p.Shadows, err = s.checkBrick(ts, &in, filepath.Join(root, l.Draft)); err != nil {
		return nil, err
	}
	return p, nil
}

// PublishBrick publishes the member's draft of arg as the next version.
func (s *Service) PublishBrick(ctx context.Context, c Context, arg, message string) (*BrickPublication, error) {
	ts, err := s.requireTeam(ctx, c)
	if err != nil {
		return nil, err
	}
	var pub *BrickPublication
	err = ts.Repo.Transact(ctx, func(_ context.Context, tx *teamstate.Tx) (teamstate.TxResult, error) {
		pub = nil
		p, err := s.buildBrickPublication(ts, tx, arg, message)
		if err != nil {
			return teamstate.TxResult{}, err
		}
		pub = p
		return teamstate.TxResult{Message: fmt.Sprintf("brick: publish %s v%d by %s", p.Ref, p.Version, p.By)}, nil
	})
	if err != nil {
		return nil, err
	}
	return pub, nil
}

func (s *Service) buildBrickPublication(ts *TeamState, tx *teamstate.Tx, arg, message string) (*BrickPublication, error) {
	if err := tx.CheckPublish(ts.Member); err != nil {
		return nil, err
	}
	root := tx.Path()
	l, err := s.lookupBrick(ts, arg)
	if err != nil {
		return nil, err
	}
	if l.Draft == "" {
		return nil, fmt.Errorf("%w: %s", ErrNoDraft, l.Ref)
	}
	kind, id := string(l.Ref.Kind), l.Ref.ID
	lock, sealed, err := teamstate.ReadBricksLock(root)
	if err != nil {
		return nil, err
	}
	if !sealed {
		if others := unsealedOthers(root, l.Ref); len(others) > 0 {
			return nil, fmt.Errorf("%w: %s", ErrBricksUnsealed, strings.Join(others, ", "))
		}
	}
	data, err := tx.ReadFile(l.Draft)
	if err != nil {
		return nil, err
	}
	in := BrickInput{Ref: l.Ref, Family: l.Family, Data: data}
	diags, shadows, err := s.checkBrick(ts, &in, filepath.Join(root, l.Draft))
	if err != nil {
		return nil, err
	}
	if diags.HasErrors() {
		return nil, &InvalidError{Ref: l.Ref.String(), Diagnostics: diags}
	}
	rel, err := teamstate.PublishedBrickRel(kind, id, in.Family)
	if err != nil {
		return nil, err
	}

	cur, hasCur := lock.Get(kind, id)
	version := nextBrickVersion(root, l.Ref, cur.Version)
	if hasCur && cur.CheckPath(kind, id) == nil {
		if err := archiveBrick(tx, kind, id, cur); err != nil {
			return nil, err
		}
		if filepath.ToSlash(cur.Path) != filepath.ToSlash(rel) {
			if err := tx.Remove(filepath.FromSlash(cur.Path)); err != nil {
				return nil, err
			}
		}
	}
	if err := tx.WriteFile(rel, data); err != nil {
		return nil, err
	}
	now := time.Now().UTC().Truncate(time.Second)
	lock.Set(kind, id, teamstate.BrickLockEntry{Version: version, Path: filepath.ToSlash(rel), Hash: teamstate.HashContent(data),
		PublishedBy: ts.Member, PublishedAt: now, Message: message})
	if err := writeBricksLock(tx, lock); err != nil {
		return nil, err
	}
	if err := tx.Remove(l.Draft); err != nil {
		return nil, err
	}
	pub := &BrickPublication{Ref: l.Ref, Version: version, By: ts.Member, At: now, Message: message, Shadows: shadows, Diagnostics: diags}
	ev := map[string]any{"brick": l.Ref.String(), "version": version}
	if message != "" {
		ev["message"] = message
	}
	if pub.Shadows != "" {
		ev["replaces"] = pub.Shadows
	}
	if err := tx.AppendEvent(teamstate.Event{Timestamp: now, Actor: ts.Member, Type: teamstate.EventBrickPublished,
		Project: teamstate.TeamEventsProject, Data: ev}); err != nil {
		return nil, err
	}
	return pub, nil
}

// AdoptBricks seals, as they are, the bricks of a catalogue committed by
// hand before ADR-054 (catalog.lock absent): each must load (no refused
// brick) and pass the lenient checks of oh doctor. One transaction, one
// event.
func (s *Service) AdoptBricks(ctx context.Context, c Context, message string) (*BrickPublication, error) {
	ts, err := s.requireTeam(ctx, c)
	if err != nil {
		return nil, err
	}
	var pub *BrickPublication
	err = ts.Repo.Transact(ctx, func(_ context.Context, tx *teamstate.Tx) (teamstate.TxResult, error) {
		pub = nil
		if err := tx.CheckPublish(ts.Member); err != nil {
			return teamstate.TxResult{}, err
		}
		root := tx.Path()
		lock, sealed, err := teamstate.ReadBricksLock(root)
		if err != nil {
			return teamstate.TxResult{}, err
		}
		if sealed {
			return teamstate.TxResult{}, ErrBricksSealed
		}
		teamDir := filepath.Join(root, teamstate.CatalogDirName)
		_, probs, err := s.checkCatalog(teamDir, nil, false)
		if err != nil {
			return teamstate.TxResult{}, err
		}
		var blocking wf.Diagnostics
		for _, ds := range probs {
			for _, d := range ds {
				if d.Severity == wf.SeverityError || (isBrickDiag(d) && d.Code != DiagBrickShadows) {
					d.Severity = wf.SeverityError
					blocking = append(blocking, d)
				}
			}
		}
		if len(blocking) > 0 {
			blocking.Sort()
			return teamstate.TxResult{}, &InvalidError{Ref: teamstate.CatalogDirName, Diagnostics: blocking}
		}
		list, _, err := readTeamBricks(teamDir)
		if err != nil {
			return teamstate.TxResult{}, err
		}
		now := time.Now().UTC().Truncate(time.Second)
		p := &BrickPublication{By: ts.Member, At: now, Message: message}
		for _, b := range list {
			if b.kind == "annex" {
				continue
			}
			ref := BrickRef{Kind: BrickKind(b.kind), ID: b.id}
			lock.Set(b.kind, b.id, teamstate.BrickLockEntry{Version: nextBrickVersion(root, ref, 0), Path: filepath.ToSlash(b.rel),
				Hash: teamstate.HashContent(b.data), PublishedBy: ts.Member, PublishedAt: now, Message: message})
			p.Adopted = append(p.Adopted, ref.String())
		}
		if err := writeBricksLock(tx, lock); err != nil {
			return teamstate.TxResult{}, err
		}
		ev := map[string]any{"adopted": p.Adopted}
		if message != "" {
			ev["message"] = message
		}
		if err := tx.AppendEvent(teamstate.Event{Timestamp: now, Actor: ts.Member, Type: teamstate.EventBrickAdopted,
			Project: teamstate.TeamEventsProject, Data: ev}); err != nil {
			return teamstate.TxResult{}, err
		}
		pub = p
		return teamstate.TxResult{Message: fmt.Sprintf("brick: seal %d bricks by %s", len(p.Adopted), ts.Member)}, nil
	})
	if err != nil {
		return nil, err
	}
	return pub, nil
}

// BrickVersion is one published version of a brick.
type BrickVersion struct {
	Ref     BrickRef                 `json:"ref"`
	Version int                      `json:"version"`
	Current bool                     `json:"current,omitempty"`
	Entry   teamstate.BrickLockEntry `json:"entry"`
	Path    string                   `json:"path"`
}

// BrickHistory lists the versions of a team brick, most recent first.
func (s *Service) BrickHistory(ctx context.Context, c Context, arg string) ([]BrickVersion, error) {
	ts, err := s.requireTeam(ctx, c)
	if err != nil {
		return nil, err
	}
	l, err := s.lookupBrick(ts, arg)
	if err != nil {
		return nil, err
	}
	root := ts.Repo.Path()
	lock, _, err := teamstate.ReadBricksLock(root)
	if err != nil {
		return nil, err
	}
	kind := string(l.Ref.Kind)
	var out []BrickVersion
	if cur, ok := lock.Get(kind, l.Ref.ID); ok {
		out = append(out, BrickVersion{Ref: l.Ref, Version: cur.Version, Current: true, Entry: cur, Path: filepath.Join(root, filepath.FromSlash(cur.Path))})
	}
	hist, err := teamstate.ListBrickHistory(root, kind, l.Ref.ID)
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
		out = append(out, BrickVersion{Ref: l.Ref, Version: h.Version, Entry: e, Path: h.Path})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNotPublished, l.Ref)
	}
	return out, nil
}

// archiveBrick copies the published version to history (file and lock
// entry).
func archiveBrick(tx *teamstate.Tx, kind, id string, cur teamstate.BrickLockEntry) error {
	data, err := tx.ReadFile(filepath.FromSlash(cur.Path))
	if err != nil {
		return fmt.Errorf("reading the published version: %w", err)
	}
	hrel, err := teamstate.BrickHistoryRel(kind, id, cur.Version)
	if err != nil {
		return err
	}
	if err := tx.WriteFile(hrel, data); err != nil {
		return err
	}
	meta, err := teamstate.MarshalBrickLockEntry(cur)
	if err != nil {
		return err
	}
	mrel, _ := teamstate.BrickHistoryMetaRel(kind, id, cur.Version)
	return tx.WriteFile(mrel, meta)
}

func writeBricksLock(tx *teamstate.Tx, lock *teamstate.BricksLock) error {
	data, err := teamstate.MarshalBricksLock(lock)
	if err != nil {
		return err
	}
	return tx.WriteFile(teamstate.BricksLockFile, data)
}

// nextBrickVersion is one more than the current and every history version.
func nextBrickVersion(root string, ref BrickRef, current int) int {
	v := current
	hist, _ := teamstate.ListBrickHistory(root, string(ref.Kind), ref.ID)
	for _, h := range hist {
		v = max(v, h.Version)
	}
	return v + 1
}

// unsealedOthers lists the published bricks other than ref of a catalogue
// without catalog.lock.
func unsealedOthers(root string, ref BrickRef) []string {
	list, _, err := readTeamBricks(filepath.Join(root, teamstate.CatalogDirName))
	if err != nil {
		return nil
	}
	var out []string
	for _, b := range list {
		if b.kind != "annex" && (b.kind != string(ref.Kind) || b.id != ref.ID) {
			out = append(out, teamstate.BrickKey(b.kind, b.id))
		}
	}
	return out
}

func readRel(root, rel string) ([]byte, error) {
	if !regular(filepath.Join(root, rel)) {
		return nil, errors.New("not a regular file: " + rel)
	}
	return os.ReadFile(filepath.Join(root, rel))
}
