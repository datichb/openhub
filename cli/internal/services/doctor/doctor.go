// Package doctor is the registry of the health checks of oh (oh doctor and
// the Doctor view of the TUI): a check has an identifier, a translated name,
// a function that returns its results and, for the results that can be
// repaired, a Fix. The command line and the TUI run the same registry; the
// repairs are plain functions, never a command of the command line.
package doctor

import (
	"context"
	"fmt"
)

// Result is the outcome of a check. Warn marks a passed check that deserves
// attention (newer oh release…).
type Result struct {
	// ID identifies the result for the repairs (Check.Fix); a result without
	// one takes the identifier of its check.
	ID     string
	Name   string
	Detail string
	OK     bool
	Warn   bool
	// Optional marks a check of something oh can do without (container
	// engine, bd…): when it fails it is reported as a warning and does not
	// change the exit code of oh doctor (U-05). See Normalized.
	Optional bool
	// Fix repairs what the check found (oh doctor --fix); nil: nothing to
	// repair. Set by Check.Results from Check.Fix.
	Fix *Fix
}

// Normalized turns the failure of an optional check into a warning.
func (r Result) Normalized() Result {
	if r.Optional && !r.OK {
		r.OK, r.Warn = true, true
	}
	return r
}

// Fix is the repair of a result: Plan describes it for the confirmation, then
// Apply repairs and returns its report (both may take time: callers run them
// off their event loop). Open replaces both when the repair is a screen of
// the caller (the clean-up view); it runs on the caller's loop.
type Fix struct {
	Plan  func(ctx context.Context) (string, error)
	Apply func(ctx context.Context) (string, error)
	Open  func()
}

// Check is one entry of the registry.
type Check struct {
	// ID is stable and unique in a registry (it does not follow the language).
	ID string
	// Name is the translated label shown while the check runs; a result
	// without a name takes it.
	Name string
	// Run returns the results of the check (most return one).
	Run func(ctx context.Context) []Result
	// Fix returns the repair of a result of this check (nil: none). Optional.
	Fix func(Result) *Fix
}

// Results runs the check and completes its results: name and identifier
// when empty, optional failures turned into warnings, repair attached.
func (c Check) Results(ctx context.Context) []Result {
	if c.Run == nil {
		return nil
	}
	res := c.Run(ctx)
	for i := range res {
		if res[i].Name == "" {
			res[i].Name = c.Name
		}
		if res[i].ID == "" {
			res[i].ID = c.ID
		}
		res[i] = res[i].Normalized()
		if res[i].Fix == nil && c.Fix != nil {
			res[i].Fix = c.Fix(res[i])
		}
	}
	return res
}

// Registry is the list of the checks, in display order.
type Registry struct {
	checks []Check
}

// NewRegistry builds a registry; an empty or repeated identifier, or a check
// without Run, is an error (a mistake of the code that builds the list).
func NewRegistry(checks ...Check) (*Registry, error) {
	seen := map[string]bool{}
	for _, c := range checks {
		switch {
		case c.ID == "":
			return nil, fmt.Errorf("doctor: check %q has no ID", c.Name)
		case seen[c.ID]:
			return nil, fmt.Errorf("doctor: check ID %q is used twice", c.ID)
		case c.Run == nil:
			return nil, fmt.Errorf("doctor: check %q has no Run", c.ID)
		}
		seen[c.ID] = true
	}
	return &Registry{checks: append([]Check(nil), checks...)}, nil
}

// Checks lists the checks in display order.
func (r *Registry) Checks() []Check { return append([]Check(nil), r.checks...) }

// Get finds a check by identifier.
func (r *Registry) Get(id string) (Check, bool) {
	for _, c := range r.checks {
		if c.ID == id {
			return c, true
		}
	}
	return Check{}, false
}

// Run runs every check and returns their results in order.
func (r *Registry) Run(ctx context.Context) []Result {
	var out []Result
	for _, c := range r.checks {
		out = append(out, c.Results(ctx)...)
	}
	return out
}

// FixFor returns the repair of a result, as the registry gives it (nil:
// none): the one attached by Run, or the one of its check when the result
// was built apart.
func (r *Registry) FixFor(res Result) *Fix {
	if res.Fix != nil {
		return res.Fix
	}
	for _, c := range r.checks {
		if c.ID == res.ID && c.Fix != nil {
			return c.Fix(res)
		}
	}
	return nil
}
