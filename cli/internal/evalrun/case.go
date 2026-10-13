// Package evalrun runs behavioural evaluations of the shipped workflows with
// a real model (`oh eval`, P0.5): each case launches a workflow without
// interface on a throwaway clone of its fixture, answers its decisions as the
// case says, reads the session trace (`oh session trace`), then checks
// assertions on the trace and on the clone (hidden tests). Reports are
// written in JSON and Markdown; a reference measure (baseline) can be
// recorded and compared.
//
// The package holds no tool or model code: the launch of a pass is an
// Executor (cmd/eval.go), so the runner, the assertions and the reports are
// tested without network.
package evalrun

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Case file names inside a case directory.
const (
	CaseFile       = "case.yaml"
	FixtureDir     = "fixture"
	HiddenTestsDir = "hidden_tests"
	// ReportsOff is the value of Case.Reports that turns the reports off.
	ReportsOff = "off"
)

// Case is one evaluation scenario (evals/<target>/<case>/case.yaml).
type Case struct {
	ID    string `yaml:"id" json:"id"`
	Title string `yaml:"title" json:"title"`
	// Target is the directory the case belongs to (evals/<target>/…).
	Target   string `yaml:"-" json:"target"`
	Workflow string `yaml:"workflow" json:"workflow"`
	// Active: false keeps the case out of the runs (InactiveReason says why).
	Active         *bool  `yaml:"active" json:"-"`
	InactiveReason string `yaml:"inactive_reason" json:"inactive_reason,omitempty"`
	Mode           string `yaml:"mode" json:"mode,omitempty"`
	// Inputs are the workflow inputs; Text goes to its first free text input.
	Inputs  map[string]string `yaml:"inputs" json:"inputs,omitempty"`
	Text    string            `yaml:"text" json:"text,omitempty"`
	Tickets []string          `yaml:"tickets" json:"tickets,omitempty"`
	// Beads initializes a Beads database in the clone with these tickets.
	Beads *BeadsSetup `yaml:"beads" json:"beads,omitempty"`
	// Setup are shell commands run in the clone after the fixture commit
	// (branches, seeded bugs…); each is committed when it changes files.
	Setup   []string `yaml:"setup" json:"setup,omitempty"`
	Timeout Duration `yaml:"timeout" json:"timeout,omitempty"`
	// BudgetUSD caps the cost of one pass (0 = the run default).
	BudgetUSD float64 `yaml:"budget_usd" json:"budget_usd,omitempty"`
	Answers   Answers `yaml:"answers" json:"answers"`
	// Assertions are checked on the trace and the clone of every pass.
	Assertions []Assertion `yaml:"assertions" json:"assertions"`
	// HiddenTests is the command run by the hidden_tests assertion
	// (default: per stack, see HiddenTestCommand).
	HiddenTests string `yaml:"hidden_tests" json:"hidden_tests,omitempty"`
	// Reports: "off" makes the daemon of the pass keep no report
	// (workflow_report answers "not recorded": the agents must fall back on
	// their whole report); "" keeps them.
	Reports string `yaml:"reports" json:"reports,omitempty"`

	// Dir is the case directory.
	Dir string `yaml:"-" json:"-"`
}

// BeadsSetup describes the Beads database of a case.
type BeadsSetup struct {
	Prefix  string       `yaml:"prefix" json:"prefix"`
	Tickets []BeadTicket `yaml:"tickets" json:"tickets"`
}

// BeadTicket is a ticket created in the clone before the launch.
type BeadTicket struct {
	ID          string   `yaml:"id" json:"id"`
	Title       string   `yaml:"title" json:"title"`
	Type        string   `yaml:"type" json:"type,omitempty"`
	Description string   `yaml:"description" json:"description,omitempty"`
	Acceptance  string   `yaml:"acceptance" json:"acceptance,omitempty"`
	Labels      []string `yaml:"labels" json:"labels,omitempty"`
	Parent      string   `yaml:"parent" json:"parent,omitempty"`
}

// Answers tell how a pass answers the decisions of its session.
type Answers struct {
	// Checkpoints by id ("*" = any other): approve | fix | reject | other,
	// optionally with a message after a colon ("fix: add a test").
	Checkpoints map[string]string `yaml:"checkpoints" json:"checkpoints,omitempty"`
	// Permission: once (default) | reject.
	Permission string `yaml:"permission" json:"permission,omitempty"`
	// Question: "first" (default: first option) or a free answer.
	Question string `yaml:"question" json:"question,omitempty"`
	// MaxDecisions bounds the decisions answered in one pass (default 40).
	MaxDecisions int `yaml:"max_decisions" json:"max_decisions,omitempty"`
	// Wiki: how the wiki proposals on the card of a commit checkpoint are
	// decided before the checkpoint is answered: accept | refuse | pending
	// (default: left as they are, so they stay queued).
	Wiki string `yaml:"wiki" json:"wiki,omitempty"`
}

// Duration is a time.Duration read from "15m".
type Duration time.Duration

// UnmarshalYAML implements yaml.Unmarshaler.
func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	v, err := time.ParseDuration(n.Value)
	if err != nil {
		return fmt.Errorf("duration %q: %w", n.Value, err)
	}
	*d = Duration(v)
	return nil
}

// MarshalText implements encoding.TextMarshaler.
func (d Duration) MarshalText() ([]byte, error) { return []byte(time.Duration(d).String()), nil }

// IsActive reports whether the case runs.
func (c *Case) IsActive() bool { return c.Active == nil || *c.Active }

// Key identifies a case in reports: <target>/<id>.
func (c *Case) Key() string { return c.Target + "/" + c.ID }

// DefaultTimeout is the time a pass may take when the case sets none.
const DefaultTimeout = 20 * time.Minute

// TimeoutOrDefault returns the timeout of a pass.
func (c *Case) TimeoutOrDefault() time.Duration {
	if c.Timeout > 0 {
		return time.Duration(c.Timeout)
	}
	return DefaultTimeout
}

// LoadCase reads a case directory.
func LoadCase(dir string) (*Case, error) {
	data, err := os.ReadFile(filepath.Join(dir, CaseFile))
	if err != nil {
		return nil, err
	}
	var c Case
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Join(dir, CaseFile), err)
	}
	c.Dir = dir
	c.Target = filepath.Base(filepath.Dir(dir))
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Join(dir, CaseFile), err)
	}
	return &c, nil
}

// Validate checks a case.
func (c *Case) Validate() error {
	var errs []error
	if c.ID == "" {
		errs = append(errs, errors.New("id is required"))
	}
	if c.Workflow == "" {
		errs = append(errs, errors.New("workflow is required"))
	}
	if !c.IsActive() && c.InactiveReason == "" {
		errs = append(errs, errors.New("an inactive case says why (inactive_reason)"))
	}
	if st, err := os.Stat(filepath.Join(c.Dir, FixtureDir)); err != nil || !st.IsDir() {
		errs = append(errs, fmt.Errorf("%s/ is missing", FixtureDir))
	}
	if len(c.Assertions) == 0 {
		errs = append(errs, errors.New("at least one assertion"))
	}
	for i, a := range c.Assertions {
		if err := a.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("assertion %d (%s): %w", i+1, a.Type, err))
		}
		if a.Type == AssertHiddenTests {
			if st, err := os.Stat(filepath.Join(c.Dir, HiddenTestsDir)); err != nil || !st.IsDir() {
				errs = append(errs, fmt.Errorf("assertion %d: %s/ is missing", i+1, HiddenTestsDir))
			}
		}
	}
	for id, v := range c.Answers.Checkpoints {
		if _, _, err := ParseChoice(v); err != nil {
			errs = append(errs, fmt.Errorf("answers.checkpoints.%s: %w", id, err))
		}
	}
	switch c.Answers.Permission {
	case "", "once", "reject":
	default:
		errs = append(errs, fmt.Errorf("answers.permission %q (once, reject)", c.Answers.Permission))
	}
	switch c.Answers.Wiki {
	case "", "accept", "refuse", "pending":
	default:
		errs = append(errs, fmt.Errorf("answers.wiki %q (accept, refuse, pending)", c.Answers.Wiki))
	}
	switch c.Reports {
	case "", ReportsOff:
	default:
		errs = append(errs, fmt.Errorf("reports %q (off)", c.Reports))
	}
	return errors.Join(errs...)
}

// LoadCases reads every case under root (root/<target>/<case>/case.yaml),
// sorted by key.
func LoadCases(root string) ([]*Case, error) {
	matches, err := filepath.Glob(filepath.Join(root, "*", "*", CaseFile))
	if err != nil {
		return nil, err
	}
	var (
		out  []*Case
		errs []error
	)
	seen := map[string]string{}
	for _, m := range matches {
		c, err := LoadCase(filepath.Dir(m))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if prev, dup := seen[c.ID]; dup {
			errs = append(errs, fmt.Errorf("case id %s used twice (%s, %s)", c.ID, prev, c.Dir))
			continue
		}
		seen[c.ID] = c.Dir
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return caseLess(out[i].ID, out[j].ID) })
	return out, errors.Join(errs...)
}

// caseLess orders S2 before S10.
func caseLess(a, b string) bool {
	na, nb := numSuffix(a), numSuffix(b)
	pa, pb := strings.TrimRight(a, "0123456789"), strings.TrimRight(b, "0123456789")
	if pa != pb || na < 0 || nb < 0 {
		return a < b
	}
	return na < nb
}

func numSuffix(s string) int {
	i := len(s)
	for i > 0 && s[i-1] >= '0' && s[i-1] <= '9' {
		i--
	}
	if i == len(s) {
		return -1
	}
	n := 0
	for _, r := range s[i:] {
		n = n*10 + int(r-'0')
	}
	return n
}

// Select keeps the cases whose id is listed (all when ids is empty).
func Select(cases []*Case, ids []string) ([]*Case, error) {
	if len(ids) == 0 {
		return cases, nil
	}
	by := map[string]*Case{}
	for _, c := range cases {
		by[strings.ToLower(c.ID)] = c
	}
	var out []*Case
	for _, id := range ids {
		c, ok := by[strings.ToLower(strings.TrimSpace(id))]
		if !ok {
			return nil, fmt.Errorf("unknown case %q", id)
		}
		out = append(out, c)
	}
	return out, nil
}
