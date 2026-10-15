package workflow

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// Checks of the team bricks (ADR-054): the rules of the hub catalogue
// (oh skill check), the frontmatter of the agents and the replacement of a
// hub brick, applied to the hub merged with the team catalogue.
//
// Two levels: strict for a brick being written with oh brick (draft,
// publication: a skill description is a « Utiliser … » trigger of 200
// characters at most, as in the hub), lenient for the published catalogue
// (oh doctor, oh brick list: the description rules are warnings, as in
// oh skill check).

// Brick check codes (messages "teamstate.workflow.brick.<code>").
const (
	DiagBrickFrontmatter   = "brick_frontmatter"
	DiagBrickNoFrontmatter = "brick_no_frontmatter"
	DiagBrickIDMismatch    = "brick_id_mismatch"
	DiagBrickNoDescription = "brick_no_description"
	DiagBrickBadMode       = "brick_bad_mode"
	DiagBrickUnknownKey    = "brick_unknown_key"
	DiagBrickPermBase      = "brick_permission_base"
	DiagBrickShadows       = "brick_shadows"
)

// agentKeys are the frontmatter keys of an agent read by oh (bricks.
// AgentFrontmatter), plus extends: for a team agent.
var agentKeys = []string{"id", "label", "description", "mode", "model", "permission_base", "permission", "skills",
	"native_skills", "verify", "stack_skills", "skills_from_callers", "project_instructions", "extends"}

// BrickProblems are the findings of a check, by team brick key
// ("agent:<id>", "skill:<ref>" of the team file); "" holds the findings of
// no brick (catalog.lock).
type BrickProblems map[string]wf.Diagnostics

// Errors counts the bricks with at least one error.
func (p BrickProblems) Errors() int {
	n := 0
	for k, ds := range p {
		if k != "" && ds.HasErrors() {
			n++
		}
	}
	return n
}

// checkCatalog merges the hub with the team catalogue teamDir (with
// candidate in place of its published version, when not nil) and checks
// the team bricks. A candidate is merged in a temporary directory.
func (s *Service) checkCatalog(teamDir string, candidate *teamBrick, strict bool) (*Bricks, BrickProblems, error) {
	cacheRoot := s.bricksCacheDir()
	if candidate != nil {
		tmp, err := os.MkdirTemp("", "oh-brick-check-")
		if err != nil {
			return nil, nil, err
		}
		defer os.RemoveAll(tmp)
		cacheRoot = tmp
	}
	b, err := mergeBricks(s.HubDir, teamDir, cacheRoot, candidate)
	if err != nil {
		return nil, nil, err
	}
	list, _, err := readTeamBricks(teamDir)
	if err != nil {
		return nil, nil, err
	}
	if candidate != nil {
		list = append(list, *candidate)
	}
	keyOf := map[string]string{} // source file → team key
	data := map[string][]byte{}
	for _, tb := range list {
		if tb.kind != "annex" {
			keyOf[tb.src] = teamstate.BrickKey(tb.kind, tb.id)
			data[tb.src] = tb.data
		}
	}
	placed := map[string]string{} // key in the merged directory → team key
	for key, src := range b.Sources {
		placed[key] = keyOf[src]
	}

	out := BrickProblems{}
	for _, d := range b.Diagnostics {
		key := keyOf[d.Source]
		if candidate != nil && d.Source == candidate.src {
			d.Severity = wf.SeverityError
		}
		out[key] = append(out[key], d)
	}
	for key, hub := range b.Shadows {
		if tk := placed[key]; tk != "" {
			out[tk] = append(out[tk], wf.Diagnostic{Severity: wf.SeverityWarning, Code: DiagBrickShadows, Source: b.Sources[key], Path: tk,
				Message: i18n.Tf("teamstate.workflow.brick."+DiagBrickShadows, tk, hub)})
		}
	}
	for key, src := range b.Sources {
		if id, ok := strings.CutPrefix(key, "agent:"); ok {
			tk := placed[key]
			out[tk] = append(out[tk], checkAgentFront(id, src, data[src], b.Dir)...)
		}
	}
	problems, err := bundle.CheckSkills(b.Dir)
	if err != nil {
		return nil, nil, err
	}
	for _, p := range problems {
		key := "skill:" + p.Ref
		if p.Kind == bundle.SkillMissingAgentSkill {
			key = "agent:" + p.Detail
		}
		tk := placed[key]
		if tk == "" {
			continue // a hub brick
		}
		sev := wf.SeverityWarning
		switch {
		case strict && (p.Kind == bundle.SkillNoDescription || p.Kind == bundle.SkillNotTrigger || p.Kind == bundle.SkillMissingAgentSkill):
			sev = wf.SeverityError
		case !strict && p.Kind == bundle.SkillLongDescription:
			if n, _ := strconv.Atoi(p.Detail); n > bundle.MaxSkillDescriptionLen {
				sev = wf.SeverityError
			}
		case p.Error:
			sev = wf.SeverityError
		}
		out[tk] = append(out[tk], wf.Diagnostic{Severity: sev, Code: string(p.Kind), Source: b.Sources[key], Path: tk,
			Message: i18n.Tf("cmd.skill.check.kind."+string(p.Kind), p.Ref, p.Detail)})
	}
	for k := range out {
		out[k].Sort()
	}
	return b, out, nil
}

func brickMsg(code string, args ...any) string {
	return i18n.Tf("teamstate.workflow.brick."+code, args...)
}

// checkAgentFront checks the frontmatter of a team agent: readable, id
// equal to the file name, description, mode, known keys, base permission
// file present.
func checkAgentFront(id, src string, data []byte, dir string) wf.Diagnostics {
	diag := func(sev wf.Severity, code string, args ...any) wf.Diagnostic {
		return wf.Diagnostic{Severity: sev, Code: code, Source: src, Path: "agent:" + id,
			Message: i18n.Tf("teamstate.workflow.brick."+code, args...)}
	}
	fm, err := bricks.ParseAgentFrontmatterFromBytes(data)
	if err != nil {
		return wf.Diagnostics{diag(wf.SeverityError, DiagBrickFrontmatter, err)}
	}
	var out wf.Diagnostics
	if fm.ID != id {
		out = append(out, diag(wf.SeverityError, DiagBrickIDMismatch, fm.ID, id+".md"))
	}
	if strings.TrimSpace(fm.Description) == "" {
		out = append(out, diag(wf.SeverityError, DiagBrickNoDescription))
	}
	if fm.Mode != "primary" && fm.Mode != "subagent" {
		out = append(out, diag(wf.SeverityError, DiagBrickBadMode, fm.Mode))
	}
	raw, _ := splitFrontmatter(data)
	var keys map[string]any
	_ = yaml.Unmarshal(raw, &keys)
	var unknown []string
	for k := range keys {
		if !slices.Contains(agentKeys, k) {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(unknown)
	for _, k := range unknown {
		out = append(out, diag(wf.SeverityError, DiagBrickUnknownKey, k))
	}
	if fm.PermissionBase != "" {
		if teamstate.ValidBrickFamily(fm.PermissionBase) != nil {
			out = append(out, diag(wf.SeverityError, DiagBrickPermBase, fm.PermissionBase))
		} else if _, err := os.Stat(filepath.Join(dir, "permissions", fm.PermissionBase+".yaml")); err != nil {
			out = append(out, diag(wf.SeverityError, DiagBrickPermBase, fm.PermissionBase))
		}
	}
	return out
}

// CheckTeamBricks checks the published team catalogue of c with the
// lenient rules (oh doctor, oh brick list). Without team-state or team
// bricks: nil, nil.
func (s *Service) CheckTeamBricks(ctx context.Context, c Context) (*Bricks, BrickProblems, error) {
	ts, err := s.teamState(ctx, c)
	if err != nil || ts == nil || ts.Repo == nil || s.HubDir == "" {
		return nil, nil, err
	}
	teamDir := filepath.Join(ts.Repo.Path(), teamstate.CatalogDirName)
	if !hasFiles(teamDir) {
		return nil, nil, nil
	}
	return s.checkCatalog(teamDir, nil, false)
}

// BrickWorkspace is what oh brick list shows.
type BrickWorkspace struct {
	Bricks []BrickEntry               `json:"bricks"`
	Drafts []teamstate.BrickDraftInfo `json:"drafts,omitempty"`
	Member string                     `json:"member,omitempty"`
	// TeamState: the context has a team-state (team catalogue editable).
	TeamState bool `json:"team_state"`
	// Sealed: catalog.lock exists; Unsealed: bricks loaded without it.
	Sealed   bool     `json:"sealed"`
	Unsealed []string `json:"unsealed,omitempty"`
	// Integrity lists the refused team bricks and the catalog.lock problems.
	Integrity wf.Diagnostics `json:"integrity,omitempty"`
	// Problems are the findings of the team bricks, by team key.
	Problems BrickProblems `json:"problems,omitempty"`
}

// BrickWorkspace returns the bricks of c with their origin, the member's
// draft bricks and the findings of the team catalogue.
func (s *Service) BrickWorkspace(ctx context.Context, c Context) (*BrickWorkspace, error) {
	entries, err := s.BrickCatalog(ctx, c)
	if err != nil {
		return nil, err
	}
	w := &BrickWorkspace{Bricks: entries}
	ts, err := s.teamState(ctx, c)
	if err != nil {
		return nil, err
	}
	if ts == nil || ts.Repo == nil {
		return w, nil
	}
	w.TeamState, w.Member = true, ts.Member
	if ts.Member != "" {
		w.Drafts, _ = teamstate.ListBrickDrafts(ts.Repo.Path(), ts.Member)
	}
	b, probs, err := s.CheckTeamBricks(ctx, c)
	if err != nil || b == nil {
		return w, err
	}
	w.Sealed, w.Unsealed, w.Problems = b.Sealed, b.Unsealed, probs
	w.Integrity = append(w.Integrity, b.Diagnostics...)
	for i := range w.Bricks {
		e := &w.Bricks[i]
		if e.TeamID == "" {
			continue
		}
		for _, d := range probs[string(e.Kind)+":"+e.TeamID] {
			switch {
			case d.Code == DiagBrickShadows:
			case d.Severity == wf.SeverityError:
				e.Errors++
			default:
				e.Warnings++
			}
		}
	}
	return w, nil
}
