package workflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/teamstate"
	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// Editing of the team brick catalogue (ADR-054, oh brick): drafts of a
// member validated when saved, published by a team-state transaction with
// the governance of the workflows. Hub bricks are read-only: a team brick
// replaces one only when it says so (extends: hub:<id>).

var (
	// ErrUnknownBrick is returned for a brick found nowhere.
	ErrUnknownBrick = errors.New("unknown brick")
	// ErrAmbiguousBrick is returned for a bare id naming an agent and a skill.
	ErrAmbiguousBrick = errors.New("ambiguous brick")
	// ErrBrickExists refuses to create a brick the team already has.
	ErrBrickExists = errors.New("brick already exists")
	// ErrBrickInHub refuses to create, without extends, a brick the hub has.
	ErrBrickInHub = errors.New("brick exists in the hub")
	// ErrBrickNotInHub refuses extends for a brick the hub does not have.
	ErrBrickNotInHub = errors.New("no hub brick to extend")
	// ErrBricksUnsealed refuses a publication while catalog.lock is absent
	// and bricks committed by hand exist (oh brick publish --adopt first).
	ErrBricksUnsealed = errors.New("team catalogue not sealed")
	// ErrBricksSealed refuses an adoption once catalog.lock exists.
	ErrBricksSealed = errors.New("team catalogue already sealed")
)

// BrickRef identifies a brick: "agent:<id>" or "skill:<ref>".
type BrickRef struct {
	Kind BrickKind `json:"kind"`
	ID   string    `json:"id"`
}

func (r BrickRef) String() string { return string(r.Kind) + ":" + r.ID }

// ParseBrickRef reads "agent:<id>", "skill:<ref>" or a bare id (Kind "").
func ParseBrickRef(s string) BrickRef {
	for _, k := range []BrickKind{BrickAgent, BrickSkill} {
		if id, ok := strings.CutPrefix(s, string(k)+":"); ok {
			return BrickRef{Kind: k, ID: id}
		}
	}
	return BrickRef{ID: s}
}

// brickLoc is where a brick lives.
type brickLoc struct {
	Ref       BrickRef
	Family    string // agent family in the team catalogue
	Draft     string // draft of the member, relative to the team-state ("" = none)
	Published string // published file, relative to the team-state ("" = none)
	Hub       string // hub file, absolute ("" = none)
}

func (l *brickLoc) found() bool { return l.Draft != "" || l.Published != "" || l.Hub != "" }

// lookupBrick finds arg in the drafts of the member, the team catalogue and
// the hub. A bare id may name an agent or a skill, not both.
func (s *Service) lookupBrick(ts *TeamState, arg string) (*brickLoc, error) {
	ref := ParseBrickRef(arg)
	kinds := []BrickKind{ref.Kind}
	if ref.Kind == "" {
		kinds = []BrickKind{BrickAgent, BrickSkill}
	}
	var found []*brickLoc
	for _, k := range kinds {
		if teamstate.ValidBrickID(string(k), ref.ID) != nil {
			if ref.Kind != "" {
				return nil, fmt.Errorf("%w: %s", teamstate.ErrInvalidBrickID, arg)
			}
			continue
		}
		if l := s.locateBrick(ts, BrickRef{Kind: k, ID: ref.ID}); l.found() {
			found = append(found, l)
		}
	}
	switch len(found) {
	case 0:
		return nil, fmt.Errorf("%w: %s", ErrUnknownBrick, arg)
	case 1:
		return found[0], nil
	}
	return nil, fmt.Errorf("%w: %s (agent:%[2]s, skill:%[2]s)", ErrAmbiguousBrick, ref.ID)
}

// locateBrick fills the locations of a valid ref.
func (s *Service) locateBrick(ts *TeamState, ref BrickRef) *brickLoc {
	l := &brickLoc{Ref: ref}
	kind := string(ref.Kind)
	if ts != nil && ts.Repo != nil {
		root := ts.Repo.Path()
		if ref.Kind == BrickAgent {
			if m, _ := filepath.Glob(filepath.Join(root, teamstate.CatalogDirName, "agents", "*", ref.ID+".md")); len(m) > 0 && regular(m[0]) {
				l.Family = filepath.Base(filepath.Dir(m[0]))
				l.Published, _ = teamstate.PublishedBrickRel(kind, ref.ID, l.Family)
			}
		} else if rel, _ := teamstate.PublishedBrickRel(kind, ref.ID, ""); regular(filepath.Join(root, rel)) {
			l.Published = rel
		}
		if ts.Member != "" {
			drafts, _ := teamstate.ListBrickDrafts(root, ts.Member)
			for _, d := range drafts {
				if d.Kind == kind && d.ID == ref.ID && regular(filepath.Join(root, d.Rel)) {
					l.Draft = d.Rel
					if l.Family == "" || d.Family != "" {
						l.Family = d.Family
					}
				}
			}
		}
	}
	l.Hub = s.hubBrick(ref)
	return l
}

// hubBrick returns the hub file of ref ("" when the hub has none).
func (s *Service) hubBrick(ref BrickRef) string {
	if s.HubDir == "" {
		return ""
	}
	if ref.Kind == BrickAgent {
		files, _ := bricks.FindAgentFiles(s.HubDir)
		return files[ref.ID]
	}
	p := filepath.Join(s.HubDir, "skills", filepath.FromSlash(ref.ID)+".md")
	if regular(p) {
		return p
	}
	return ""
}

// hubSkillByName returns the ref of the hub skill named name ("" = none):
// a team skill replaces a hub skill of the same name.
func (s *Service) hubSkillByName(name string) string {
	hubSkills, _ := listFiles(filepath.Join(s.HubDir, "skills"), ".md")
	for _, rel := range hubSkills {
		ref := filepath.ToSlash(strings.TrimSuffix(rel, ".md"))
		if !strings.HasPrefix(ref, "templates/") && filepath.Base(ref) == name {
			return ref
		}
	}
	return ""
}

func regular(p string) bool {
	st, err := os.Lstat(p)
	return err == nil && st.Mode().IsRegular()
}

// BrickText is the text of a brick to edit.
type BrickText struct {
	Ref    BrickRef `json:"ref"`
	Family string   `json:"family,omitempty"`
	// Origin is draft, team or hub.
	Origin string `json:"origin"`
	Path   string `json:"path"`
	Data   []byte `json:"-"`
}

// BrickText returns the text of arg: the member's draft, else the
// published team brick, else the hub brick (read-only).
func (s *Service) BrickText(ctx context.Context, c Context, arg string) (*BrickText, error) {
	ts, err := s.teamState(ctx, c)
	if err != nil {
		return nil, err
	}
	l, err := s.lookupBrick(ts, arg)
	if err != nil {
		return nil, err
	}
	t := &BrickText{Ref: l.Ref, Family: l.Family}
	switch {
	case l.Draft != "":
		t.Origin, t.Path = "draft", filepath.Join(ts.Repo.Path(), l.Draft)
	case l.Published != "":
		t.Origin, t.Path = string(OriginTeam), filepath.Join(ts.Repo.Path(), l.Published)
	default:
		t.Origin, t.Path = string(OriginHub), l.Hub
	}
	if t.Data, err = os.ReadFile(t.Path); err != nil {
		return nil, err
	}
	return t, nil
}

// EditableBrickText is BrickText for an edit: a hub brick is refused
// (ErrHubReadOnly; create a team brick that extends it).
func (s *Service) EditableBrickText(ctx context.Context, c Context, arg string) (*BrickText, error) {
	if _, err := s.requireTeam(ctx, c); err != nil {
		return nil, err
	}
	t, err := s.BrickText(ctx, c, arg)
	if err != nil {
		return nil, err
	}
	if t.Origin == string(OriginHub) {
		return t, fmt.Errorf("%w: %s", ErrHubReadOnly, t.Ref)
	}
	return t, nil
}

// NewBrick describes a brick to create.
type NewBrick struct {
	Ref    BrickRef
	Family string // agent family ("" = team)
	// Extends starts from the hub brick of the same id (agent) or name
	// (skill) and replaces it, explicitly.
	Extends bool
}

// NewBrickText returns the starting text of a new team brick: a valid
// template, or the hub brick with extends: hub:<id> when n.Extends.
func (s *Service) NewBrickText(ctx context.Context, c Context, n NewBrick) (*BrickText, error) {
	ts, err := s.requireTeam(ctx, c)
	if err != nil {
		return nil, err
	}
	kind := string(n.Ref.Kind)
	if err := teamstate.ValidBrickID(kind, n.Ref.ID); err != nil {
		return nil, err
	}
	if n.Family == "" {
		n.Family = teamstate.TeamAgentFamily
	}
	if n.Ref.Kind == BrickAgent {
		if err := teamstate.ValidBrickFamily(n.Family); err != nil {
			return nil, err
		}
	} else {
		n.Family = ""
	}
	if l := s.locateBrick(ts, n.Ref); l.Draft != "" || l.Published != "" {
		return nil, fmt.Errorf("%w: %s", ErrBrickExists, n.Ref)
	}
	hubRef, hubFile := n.Ref.ID, s.hubBrick(n.Ref)
	if n.Ref.Kind == BrickSkill {
		if hubRef = s.hubSkillByName(filepath.Base(n.Ref.ID)); hubRef != "" {
			hubFile = filepath.Join(s.HubDir, "skills", filepath.FromSlash(hubRef)+".md")
		}
	}
	t := &BrickText{Ref: n.Ref, Family: n.Family, Origin: "draft"}
	switch {
	case hubFile != "" && !n.Extends:
		return nil, fmt.Errorf("%w: %s (hub:%s)", ErrBrickInHub, n.Ref, hubRef)
	case n.Extends && hubFile == "":
		return nil, fmt.Errorf("%w: %s", ErrBrickNotInHub, n.Ref)
	case n.Extends:
		data, err := os.ReadFile(hubFile)
		if err != nil {
			return nil, err
		}
		t.Data = withExtends(data, "hub:"+hubRef)
	case n.Ref.Kind == BrickAgent:
		t.Data = agentTemplate(n.Ref.ID)
	default:
		t.Data = skillTemplate(filepath.Base(n.Ref.ID))
	}
	return t, nil
}

// withExtends adds extends: <ref> at the top of the frontmatter.
func withExtends(data []byte, ref string) []byte {
	if !strings.HasPrefix(string(data), "---\n") {
		return data
	}
	return []byte("---\nextends: " + ref + "\n" + strings.TrimPrefix(string(data), "---\n"))
}

func titleOf(id string) string {
	words := strings.Split(id, "-")
	if words[0] != "" {
		words[0] = strings.ToUpper(words[0][:1]) + words[0][1:]
	}
	return strings.Join(words, " ")
}

func agentTemplate(id string) []byte {
	title := titleOf(id)
	return []byte(`---
id: ` + id + `
label: ` + title + `
description: "À compléter : rôle de l'agent en une phrase."
mode: subagent
permission:
  question: allow
  skill: allow
  read: allow
  glob: allow
  grep: allow
  edit: deny
  write: deny
  bash:
    "*": deny
skills: [shared/universal-guardrails]
native_skills: []
---

# ` + title + `

Tu es l'agent ` + id + ` de l'équipe. À compléter : mission, entrées, livrables, limites.
`)
}

func skillTemplate(name string) []byte {
	return []byte(`---
name: ` + name + `
description: "Utiliser quand … (à compléter : déclencheur, 200 caractères au plus)."
---

# ` + titleOf(name) + `

## Quand l'utiliser

À compléter.

## Règles

À compléter.
`)
}

// BrickInput is a brick to save as a draft.
type BrickInput struct {
	Ref    BrickRef
	Family string // agent family ("" = the published one, else team)
	Data   []byte
}

// BrickDraft is a saved draft brick.
type BrickDraft struct {
	Ref         BrickRef       `json:"ref"`
	Member      string         `json:"member"`
	Path        string         `json:"path"`
	Diagnostics wf.Diagnostics `json:"diagnostics,omitempty"`
	Pushed      bool           `json:"pushed"`
	PushError   string         `json:"push_error,omitempty"`
}

// candidate returns the team brick of in, published in the catalogue of ts.
func (s *Service) candidate(ts *TeamState, in *BrickInput, src string) (*teamBrick, error) {
	kind := string(in.Ref.Kind)
	if err := teamstate.ValidBrickID(kind, in.Ref.ID); err != nil {
		return nil, err
	}
	if in.Ref.Kind == BrickAgent && in.Family == "" {
		in.Family = s.locateBrick(ts, in.Ref).Family
		if in.Family == "" {
			in.Family = teamstate.TeamAgentFamily
		}
	}
	if in.Ref.Kind != BrickAgent {
		in.Family = ""
	}
	rel, err := teamstate.PublishedBrickRel(kind, in.Ref.ID, in.Family)
	if err != nil {
		return nil, err
	}
	if src == "" {
		src = filepath.Join(ts.Repo.Path(), rel)
	}
	return &teamBrick{kind: kind, id: in.Ref.ID, src: src, rel: rel, data: in.Data, extends: frontmatterExtends(in.Data)}, nil
}

// CheckBrick validates a brick against the hub merged with the published
// team catalogue (strict rules).
func (s *Service) CheckBrick(ctx context.Context, c Context, in BrickInput) (wf.Diagnostics, error) {
	ts, err := s.requireTeam(ctx, c)
	if err != nil {
		return nil, err
	}
	diags, _, err := s.checkBrick(ts, &in, "")
	return diags, err
}

// checkBrick validates in (read from src, "" = its published path) and
// returns the hub brick it replaces ("hub:<id>", "" = none).
func (s *Service) checkBrick(ts *TeamState, in *BrickInput, src string) (wf.Diagnostics, string, error) {
	cand, err := s.candidate(ts, in, src)
	if err != nil {
		return nil, "", err
	}
	if fm, _ := splitFrontmatter(in.Data); fm == nil {
		return wf.Diagnostics{{Severity: wf.SeverityError, Code: DiagBrickNoFrontmatter, Source: cand.src, Path: in.Ref.String(),
			Message: brickMsg(DiagBrickNoFrontmatter)}}, "", nil
	}
	b, probs, err := s.checkCatalog(filepath.Join(ts.Repo.Path(), teamstate.CatalogDirName), cand, true)
	if err != nil {
		return nil, "", err
	}
	shadows := ""
	for key, hub := range b.Shadows {
		if b.Sources[key] == cand.src {
			shadows = hub
		}
	}
	return probs[in.Ref.String()], shadows, nil
}

// SaveBrickDraft validates and saves a draft brick of the current member.
// A draft with errors is not saved (*InvalidError).
func (s *Service) SaveBrickDraft(ctx context.Context, c Context, in BrickInput) (*BrickDraft, error) {
	ts, err := s.requireTeam(ctx, c)
	if err != nil {
		return nil, err
	}
	diags, _, err := s.checkBrick(ts, &in, "")
	if err != nil {
		return nil, err
	}
	if diags.HasErrors() {
		return nil, &InvalidError{Ref: in.Ref.String(), Diagnostics: diags}
	}
	files, err := ts.Repo.WriteBrickDraftLocal(ts.Member, string(in.Ref.Kind), in.Ref.ID, in.Family, in.Data)
	if err != nil {
		return nil, err
	}
	d := &BrickDraft{Ref: in.Ref, Member: ts.Member, Path: filepath.Join(ts.Repo.Path(), files[len(files)-1]), Diagnostics: diags, Pushed: true}
	if err := ts.Repo.CommitAndPush(ctx, fmt.Sprintf("brick: draft %s by %s", in.Ref, ts.Member), files...); err != nil {
		d.Pushed, d.PushError = false, err.Error()
	}
	return d, nil
}
