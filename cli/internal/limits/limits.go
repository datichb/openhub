// Package limits resolves the optional session restrictions (I6): maximum
// of working sessions, budget per session and per day, memory cap of the
// tool servers, model allow-list. They are off by default and set in
// cascade: hub (hub.toml [limits]) → team (config.toml [limits.recommended]
// and [limits.enforced]) → project (preferences) → workflow (limits:).
//
// The most specific value wins; a value enforced by the team is a ceiling
// that no other level can loosen. Budgets are soft caps: they are checked on
// the cost the tool reports, between agent steps.
package limits

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Limits is one level of restrictions (zero values = not set).
type Limits struct {
	MaxActiveSessions int      `mapstructure:"max_active_sessions" toml:"max_active_sessions,omitempty" json:"max_active_sessions,omitempty"`
	SessionBudgetUSD  float64  `mapstructure:"session_budget_usd" toml:"session_budget_usd,omitempty" json:"session_budget_usd,omitempty"`
	DailyBudgetUSD    float64  `mapstructure:"daily_budget_usd" toml:"daily_budget_usd,omitempty" json:"daily_budget_usd,omitempty"`
	MemoryMB          int      `mapstructure:"memory_mb" toml:"memory_mb,omitempty" json:"memory_mb,omitempty"`
	Models            []string `mapstructure:"models" toml:"models,omitempty" json:"models,omitempty"`
}

// IsZero reports whether no restriction is set.
func (l Limits) IsZero() bool {
	return l.MaxActiveSessions == 0 && l.SessionBudgetUSD == 0 && l.DailyBudgetUSD == 0 && l.MemoryMB == 0 && len(l.Models) == 0
}

// TeamLimits are the team restrictions: recommended values (a default the
// hub, project or workflow may change) and enforced ceilings.
type TeamLimits struct {
	Recommended Limits `toml:"recommended,omitempty" json:"recommended,omitempty"`
	Enforced    Limits `toml:"enforced,omitempty" json:"enforced,omitempty"`
}

// Origin tells which level set a resolved value.
type Origin string

const (
	OriginNone         Origin = ""
	OriginWorkflow     Origin = "workflow"
	OriginProject      Origin = "project"
	OriginHub          Origin = "hub"
	OriginTeam         Origin = "team"
	OriginTeamEnforced Origin = "team-enforced"
)

// Field names (keys of Resolved.Origins, of `oh budget set` and of the
// project preference).
const (
	FieldMaxActive     = "max_active_sessions"
	FieldSessionBudget = "session_budget_usd"
	FieldDailyBudget   = "daily_budget_usd"
	FieldMemory        = "memory_mb"
	FieldModels        = "models"
)

// Fields lists the fields in display order.
var Fields = []string{FieldMaxActive, FieldSessionBudget, FieldDailyBudget, FieldMemory, FieldModels}

// Input gathers the levels of a launch.
type Input struct {
	Hub       Limits
	Team      *TeamLimits
	Project   Limits
	Workflow  Limits // only SessionBudgetUSD and Models are read
	ProjectID string
}

// Resolved is the effective restrictions of a session.
type Resolved struct {
	Limits
	Origins   map[string]Origin `json:"origins,omitempty"`
	ProjectID string            `json:"project_id,omitempty"`
}

// Resolve applies the cascade. The memory cap is a machine setting (hub and
// team only); the daily budget and the maximum of working sessions apply to
// all the sessions of the machine when set by the hub or the team, to the
// project's sessions when set by the project.
func Resolve(in Input) Resolved {
	var rec, enf Limits
	if in.Team != nil {
		rec, enf = in.Team.Recommended, in.Team.Enforced
	}
	wf := Limits{SessionBudgetUSD: in.Workflow.SessionBudgetUSD, Models: in.Workflow.Models}
	proj := in.Project
	proj.MemoryMB = 0
	levels := []struct {
		o Origin
		l Limits
	}{{OriginWorkflow, wf}, {OriginProject, proj}, {OriginHub, in.Hub}, {OriginTeam, rec}}

	r := Resolved{Origins: map[string]Origin{}, ProjectID: in.ProjectID}
	pickInt := func(field string, get func(Limits) int, cap int) int {
		v, o := 0, OriginNone
		for _, lv := range levels {
			if x := get(lv.l); x > 0 {
				v, o = x, lv.o
				break
			}
		}
		if cap > 0 && (v == 0 || cap < v) {
			v, o = cap, OriginTeamEnforced
		}
		if o != OriginNone {
			r.Origins[field] = o
		}
		return v
	}
	pickFloat := func(field string, get func(Limits) float64, cap float64) float64 {
		v, o := 0.0, OriginNone
		for _, lv := range levels {
			if x := get(lv.l); x > 0 {
				v, o = x, lv.o
				break
			}
		}
		if cap > 0 && (v == 0 || cap < v) {
			v, o = cap, OriginTeamEnforced
		}
		if o != OriginNone {
			r.Origins[field] = o
		}
		return v
	}
	r.MaxActiveSessions = pickInt(FieldMaxActive, func(l Limits) int { return l.MaxActiveSessions }, enf.MaxActiveSessions)
	r.MemoryMB = pickInt(FieldMemory, func(l Limits) int { return l.MemoryMB }, enf.MemoryMB)
	r.SessionBudgetUSD = pickFloat(FieldSessionBudget, func(l Limits) float64 { return l.SessionBudgetUSD }, enf.SessionBudgetUSD)
	r.DailyBudgetUSD = pickFloat(FieldDailyBudget, func(l Limits) float64 { return l.DailyBudgetUSD }, enf.DailyBudgetUSD)

	for _, lv := range levels {
		if len(lv.l.Models) > 0 {
			r.Models, r.Origins[FieldModels] = slices.Clone(lv.l.Models), lv.o
			break
		}
	}
	if len(enf.Models) > 0 {
		kept := CoveredModels(r.Models, enf.Models)
		if len(kept) == 0 {
			kept = slices.Clone(enf.Models)
		}
		if len(r.Models) == 0 || len(kept) != len(r.Models) {
			r.Origins[FieldModels] = OriginTeamEnforced
		}
		r.Models = kept
	}
	return r
}

// CoveredModels keeps the patterns of list that some ceiling pattern covers
// (a ceiling "eu.anthropic.*" covers "eu.anthropic.claude-haiku-*").
func CoveredModels(list, ceiling []string) []string {
	var out []string
	for _, p := range list {
		for _, c := range ceiling {
			if Match(c, p) {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// ModelAllowed reports whether a model id matches the allow-list (empty =
// any model).
func ModelAllowed(patterns []string, model string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, p := range patterns {
		if Match(p, model) {
			return true
		}
	}
	return false
}

// Match is a whole-value wildcard match ('*' any run, '?' one character).
func Match(pattern, s string) bool {
	p, v := []rune(pattern), []rune(s)
	pi, si, star, mark := 0, 0, -1, 0
	for si < len(v) {
		switch {
		case pi < len(p) && (p[pi] == '?' || p[pi] == v[si]):
			pi++
			si++
		case pi < len(p) && p[pi] == '*':
			star, mark = pi, si
			pi++
		case star >= 0:
			pi = star + 1
			mark++
			si = mark
		default:
			return false
		}
	}
	for pi < len(p) && p[pi] == '*' {
		pi++
	}
	return pi == len(p)
}

// Scope of the daily budget and of the maximum of working sessions.
const ScopeGlobal = "global"

// ProjectScope is the scope of a project's sessions.
func ProjectScope(id string) string { return "project:" + id }

// DailyScope is the scope the daily budget counts.
func (r Resolved) DailyScope() string { return r.scope(FieldDailyBudget) }

// ActiveScope is the scope the maximum of working sessions counts.
func (r Resolved) ActiveScope() string { return r.scope(FieldMaxActive) }

func (r Resolved) scope(field string) string {
	if r.Origins[field] == OriginProject && r.ProjectID != "" {
		return ProjectScope(r.ProjectID)
	}
	return ScopeGlobal
}

// ScopeProject returns the project of a scope ("" = all projects).
func ScopeProject(scope string) string {
	id, _ := strings.CutPrefix(scope, "project:")
	if id == scope {
		return ""
	}
	return id
}

// SessionScope is the budget raise scope of a session.
func SessionScope(id string) string { return "session:" + id }

// FileName is the per-session file of the resolved restrictions
// (~/.oh/sessions/<id>/limits.json), read by the daemon.
const FileName = "limits.json"

// Save writes the resolved restrictions of a session (removed when none).
func Save(sessionsDir, sessionID string, r Resolved) error {
	p := filepath.Join(sessionsDir, sessionID, FileName)
	if r.IsZero() {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

// Load reads the restrictions of a session (zero when none were set).
func Load(sessionsDir, sessionID string) (Resolved, error) {
	var r Resolved
	data, err := os.ReadFile(filepath.Join(sessionsDir, sessionID, FileName))
	if errors.Is(err, os.ErrNotExist) {
		return r, nil
	}
	if err != nil {
		return r, err
	}
	err = json.Unmarshal(data, &r)
	return r, err
}

// Set changes one field of a level from its text value ("" or "0" clears
// it; models are comma-separated).
func (l *Limits) Set(field, value string) error {
	value = strings.TrimSpace(value)
	off := value == "" || value == "0" || value == "off"
	switch field {
	case FieldMaxActive, FieldMemory:
		n := 0
		if !off {
			v, err := strconv.Atoi(value)
			if err != nil || v < 0 {
				return fmt.Errorf("%s: a positive integer is expected, got %q", field, value)
			}
			n = v
		}
		if field == FieldMaxActive {
			l.MaxActiveSessions = n
		} else {
			l.MemoryMB = n
		}
	case FieldSessionBudget, FieldDailyBudget:
		f := 0.0
		if !off {
			v, err := strconv.ParseFloat(value, 64)
			if err != nil || v < 0 || math.IsInf(v, 0) || math.IsNaN(v) {
				return fmt.Errorf("%s: a positive amount in USD is expected, got %q", field, value)
			}
			f = v
		}
		if field == FieldSessionBudget {
			l.SessionBudgetUSD = f
		} else {
			l.DailyBudgetUSD = f
		}
	case FieldModels:
		l.Models = nil
		if !off {
			for _, m := range strings.Split(value, ",") {
				if m = strings.TrimSpace(m); m != "" {
					l.Models = append(l.Models, m)
				}
			}
		}
	default:
		return fmt.Errorf("unknown restriction %q (%s)", field, strings.Join(Fields, ", "))
	}
	return nil
}

// Value renders one field ("" when not set).
func (l Limits) Value(field string) string {
	switch field {
	case FieldMaxActive:
		return intStr(l.MaxActiveSessions)
	case FieldMemory:
		return intStr(l.MemoryMB)
	case FieldSessionBudget:
		return floatStr(l.SessionBudgetUSD)
	case FieldDailyBudget:
		return floatStr(l.DailyBudgetUSD)
	case FieldModels:
		return strings.Join(l.Models, ", ")
	}
	return ""
}

func intStr(v int) string {
	if v == 0 {
		return ""
	}
	return strconv.Itoa(v)
}

func floatStr(v float64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// BaselineFile is the usage a tool session reported when oh created it
// without spending anything (a fork copies the history and its cost):
// the ledger counts what is spent above it (v5 finalisation, Q3-6).
const BaselineFile = "usage-baseline.json"

// UsageBaseline is that reported usage.
type UsageBaseline struct {
	CostUSD   float64 `json:"cost_usd"`
	TokensIn  int64   `json:"tokens_in"`
	TokensOut int64   `json:"tokens_out"`
}

// SaveBaseline writes the usage baseline of a session (nothing when zero).
func SaveBaseline(sessionsDir, sessionID string, b UsageBaseline) error {
	if b == (UsageBaseline{}) {
		return nil
	}
	p := filepath.Join(sessionsDir, sessionID, BaselineFile)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(b)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

// LoadBaseline reads the usage baseline of a session (zero when none).
func LoadBaseline(sessionsDir, sessionID string) UsageBaseline {
	var b UsageBaseline
	if sessionsDir == "" {
		return b
	}
	if data, err := os.ReadFile(filepath.Join(sessionsDir, sessionID, BaselineFile)); err == nil {
		_ = json.Unmarshal(data, &b)
	}
	return b
}
