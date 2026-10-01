package teamstate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	toml "github.com/pelletier/go-toml/v2"
)

// regexCache stores compiled regex patterns to avoid recompilation on each CheckAll call.
var regexCache sync.Map // pattern string → *regexp.Regexp

// getCompiledRegex returns a cached compiled regex, compiling it on first use.
func getCompiledRegex(pattern string) (*regexp.Regexp, error) {
	if cached, ok := regexCache.Load(pattern); ok {
		return cached.(*regexp.Regexp), nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	regexCache.Store(pattern, re)
	return re, nil
}

// PolicyType represents the type of a policy rule.
type PolicyType string

const (
	PolicyTypeRegex            PolicyType = "regex"
	PolicyTypeBoolean          PolicyType = "boolean"
	PolicyTypeLimit            PolicyType = "limit"
	PolicyTypeForbiddenPattern PolicyType = "forbidden_pattern"
)

// PolicyEnforcement defines what happens when a policy is violated.
type PolicyEnforcement string

const (
	EnforcementDisabled PolicyEnforcement = "disabled"
	EnforcementWarn     PolicyEnforcement = "warn"
	EnforcementRefuse   PolicyEnforcement = "refuse"
)

// enforcementStrictness returns a numeric strictness level for enforcement ordering.
// Higher value = stricter. Used by mergePolicies to ensure overrides can only tighten.
func enforcementStrictness(e PolicyEnforcement) int {
	switch e {
	case EnforcementDisabled:
		return 0
	case EnforcementWarn:
		return 1
	case EnforcementRefuse:
		return 2
	default:
		return 1
	}
}

// Policy represents a single team policy rule.
type Policy struct {
	Name        string            `toml:"-"` // derived from TOML key
	Type        PolicyType        `toml:"type"`
	Target      string            `toml:"target,omitempty"`   // what to check: branch_name, commit_message, review, tests, wip_tickets...
	Rule        string            `toml:"rule,omitempty"`     // regex pattern
	Enabled     bool              `toml:"enabled,omitempty"`  // for boolean type
	Max         int               `toml:"max,omitempty"`      // for limit type
	Unit        string            `toml:"unit,omitempty"`     // for limit type (e.g. "lines")
	Patterns    []string          `toml:"patterns,omitempty"` // for forbidden_pattern type
	Scope       string            `toml:"scope,omitempty"`    // diff_only | all_files | modified_files | per_feature_branch
	Enforcement PolicyEnforcement `toml:"enforcement"`
	Message     string            `toml:"message,omitempty"`
}

// PolicyResult holds the outcome of a single policy check.
type PolicyResult struct {
	Name         string
	Passed       bool
	NotEvaluable bool // true when the context is insufficient to evaluate the policy
	Enforcement  PolicyEnforcement
	Message      string
	Details      string // additional context (e.g. which pattern matched)
}

// PolicyContext provides the data needed to evaluate policies.
type PolicyContext struct {
	BranchName        string   // current branch name
	CommitMessage     string   // commit message to validate
	DiffLines         []string // lines from the diff (added lines only)
	ModifiedFiles     []string // file paths modified
	ModifiedFileLines []string // full content lines of modified files (for scope: modified_files)
	AllFileLines      []string // full content lines of all tracked files (for scope: all_files)
	MemberID          string   // who is performing the action
	ActiveClaims      int      // number of active claims for this member
	// Boolean policy context fields — callers set these based on the current state.
	HasReview   bool // true if at least one review/approval exists
	HasTests    bool // true if test files are present or modified
	HasCoverage bool // true if coverage threshold is met
}

// policiesFile is the TOML structure of policies.toml.
type policiesFile struct {
	Policies map[string]Policy `toml:"policies"`
}

// LoadPolicies reads and merges global policies with project-specific overrides.
// If project is empty, only global policies are returned.
func (r *Repo) LoadPolicies(project string) ([]Policy, error) {
	global, err := r.loadPoliciesFromFile(filepath.Join(r.path, "policies.toml"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No policies configured
		}
		return nil, fmt.Errorf("loading global policies: %w", err)
	}

	// Apply project overrides if specified
	if project != "" {
		overridePath := filepath.Join(r.path, "projects", project, "policies-override.toml")
		overrides, err := r.loadPoliciesFromFile(overridePath)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("loading project overrides: %w", err)
		}
		if overrides != nil {
			global = mergePolicies(global, overrides)
		}
	}

	// Convert map to slice with names
	policies := make([]Policy, 0, len(global))
	for name, p := range global {
		p.Name = name
		policies = append(policies, p)
	}
	return policies, nil
}

// SavePolicies writes the given policies map to policies.toml in the team-state repo.
// If the file already exists, it is overwritten.
func (r *Repo) SavePolicies(ctx context.Context, policies map[string]Policy) error {
	return r.withWriteLock(ctx, func(ctx context.Context) error {
		// Clear Name fields before marshaling (Name is derived from the TOML key)
		clean := make(map[string]Policy, len(policies))
		for k, p := range policies {
			p.Name = ""
			clean[k] = p
		}
		pf := policiesFile{Policies: clean}
		data, err := toml.Marshal(pf)
		if err != nil {
			return fmt.Errorf("marshaling policies.toml: %w", err)
		}
		path := filepath.Join(r.path, "policies.toml")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
		return r.commitAndPush(ctx, "policies: update", "policies.toml")
	})
}

// CheckPolicy evaluates a single policy against the provided context.
// Disabled policies always pass.
func CheckPolicy(p Policy, ctx PolicyContext) PolicyResult {
	result := PolicyResult{
		Name:        p.Name,
		Enforcement: p.Enforcement,
		Message:     p.Message,
		Passed:      true,
	}

	// Disabled policies are never evaluated.
	if p.Enforcement == EnforcementDisabled {
		return result
	}

	switch p.Type {
	case PolicyTypeRegex:
		result = checkRegex(p, ctx, result)
	case PolicyTypeBoolean:
		result = checkBoolean(p, ctx, result)
	case PolicyTypeLimit:
		result = checkLimit(p, ctx, result)
	case PolicyTypeForbiddenPattern:
		result = checkForbiddenPattern(p, ctx, result)
	}

	return result
}

// CheckAll evaluates all policies for a project against the given context.
// Returns only violations (passed=false). Disabled policies are skipped.
func (r *Repo) CheckAll(project string, ctx PolicyContext) ([]PolicyResult, error) {
	policies, err := r.LoadPolicies(project)
	if err != nil {
		return nil, err
	}

	var violations []PolicyResult
	for _, p := range policies {
		if p.Enforcement == EnforcementDisabled {
			continue
		}
		result := CheckPolicy(p, ctx)
		if !result.Passed || result.NotEvaluable {
			violations = append(violations, result)
		}
	}
	return violations, nil
}

// HasRefuseViolations returns true if any violation has enforcement = refuse.
func HasRefuseViolations(results []PolicyResult) bool {
	for _, r := range results {
		if !r.Passed && !r.NotEvaluable && r.Enforcement == EnforcementRefuse {
			return true
		}
	}
	return false
}

// loadPoliciesFromFile reads a policies TOML file and returns the map.
func (r *Repo) loadPoliciesFromFile(path string) (map[string]Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var pf policiesFile
	if err := toml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrPolicyFileInvalid, err)
	}
	if pf.Policies == nil {
		pf.Policies = make(map[string]Policy)
	}
	return pf.Policies, nil
}

// mergePolicies applies overrides on top of global policies.
// Overrides can only make enforcement stricter (disabled < warn < refuse), never more permissive.
func mergePolicies(global, overrides map[string]Policy) map[string]Policy {
	merged := make(map[string]Policy, len(global))
	for k, v := range global {
		merged[k] = v
	}
	for k, override := range overrides {
		if base, exists := merged[k]; exists {
			// Only allow stricter enforcement
			if enforcementStrictness(override.Enforcement) > enforcementStrictness(base.Enforcement) {
				base.Enforcement = override.Enforcement
			}
			// Allow overriding message
			if override.Message != "" {
				base.Message = override.Message
			}
			merged[k] = base
		}
		// New policies from override are added as-is
		if _, exists := merged[k]; !exists {
			merged[k] = override
		}
	}
	return merged
}

// --- Target resolution helpers ---

// resolveRegexTarget determines what a regex policy checks.
func resolveRegexTarget(p Policy) string {
	if p.Target != "" {
		return p.Target
	}
	switch {
	case strings.Contains(p.Name, "branch"):
		return "branch_name"
	case strings.Contains(p.Name, "commit"):
		return "commit_message"
	default:
		return "file_path"
	}
}

// resolveBooleanTarget determines what a boolean policy checks.
func resolveBooleanTarget(p Policy) string {
	if p.Target != "" {
		return p.Target
	}
	switch {
	case strings.Contains(p.Name, "review"):
		return "review"
	case strings.Contains(p.Name, "test"):
		return "tests"
	case strings.Contains(p.Name, "coverage"):
		return "coverage"
	default:
		return ""
	}
}

// resolveLimitTarget determines what a limit policy checks.
func resolveLimitTarget(p Policy) string {
	if p.Target != "" {
		return p.Target
	}
	if strings.Contains(p.Name, "wip") || strings.Contains(p.Name, "ticket") {
		return "wip_tickets"
	}
	return ""
}

// --- Check functions ---

func checkRegex(p Policy, ctx PolicyContext, result PolicyResult) PolicyResult {
	if p.Rule == "" {
		return result
	}

	re, err := getCompiledRegex(p.Rule)
	if err != nil {
		result.Passed = false
		result.Details = fmt.Sprintf("invalid regex: %s", err)
		return result
	}

	target := resolveRegexTarget(p)

	var value string
	var applicable bool
	switch target {
	case "branch_name":
		value = ctx.BranchName
		applicable = ctx.BranchName != ""
		if !applicable {
			result.NotEvaluable = true
			result.Details = "branch name not available in this context"
			return result
		}
	case "commit_message":
		value = ctx.CommitMessage
		applicable = true
	case "file_path":
		if len(ctx.ModifiedFiles) == 0 {
			return result
		}
		for _, f := range ctx.ModifiedFiles {
			if re.MatchString(f) {
				return result // pass
			}
		}
		if p.Scope == "per_feature_branch" {
			result.Passed = false
			result.Details = "no file matching pattern found in branch"
		}
		return result
	default:
		return result
	}

	if !applicable {
		return result
	}

	if !re.MatchString(value) {
		result.Passed = false
		result.Details = fmt.Sprintf("value %q does not match %s", value, p.Rule)
	}
	return result
}

func checkBoolean(p Policy, ctx PolicyContext, result PolicyResult) PolicyResult {
	if !p.Enabled {
		return result
	}

	target := resolveBooleanTarget(p)

	switch target {
	case "review":
		// HasReview requires external API integration (GitLab/GitHub) which
		// is not available in all contexts. Mark as not evaluable when false
		// to avoid false violations.
		if !ctx.HasReview {
			result.NotEvaluable = true
			result.Details = "review status cannot be determined in this context"
			return result
		}
	case "tests":
		if !ctx.HasTests {
			result.Passed = false
			result.Details = fmt.Sprintf("boolean check failed: %s is not satisfied", p.Name)
		}
		return result
	case "coverage":
		// Coverage requires running tests with coverage reporting, which is
		// not available in this context.
		if !ctx.HasCoverage {
			result.NotEvaluable = true
			result.Details = "coverage data cannot be determined in this context"
			return result
		}
	default:
		return result
	}

	return result
}

func checkLimit(p Policy, ctx PolicyContext, result PolicyResult) PolicyResult {
	if p.Max <= 0 {
		return result
	}

	target := resolveLimitTarget(p)

	if target == "wip_tickets" {
		if ctx.ActiveClaims >= p.Max {
			result.Passed = false
			result.Details = fmt.Sprintf("active claims: %d (max: %d)", ctx.ActiveClaims, p.Max)
		}
	}
	return result
}

func checkForbiddenPattern(p Policy, ctx PolicyContext, result PolicyResult) PolicyResult {
	if len(p.Patterns) == 0 {
		return result
	}

	var linesToCheck []string
	switch p.Scope {
	case "diff_only":
		linesToCheck = ctx.DiffLines
	case "modified_files":
		if len(ctx.ModifiedFileLines) > 0 {
			linesToCheck = ctx.ModifiedFileLines
		} else if len(ctx.ModifiedFiles) == 0 {
			result.NotEvaluable = true
			result.Details = "no modified files context available"
			return result
		} else {
			// Fallback: use DiffLines when full file content is not available.
			linesToCheck = ctx.DiffLines
		}
	case "all_files":
		if len(ctx.AllFileLines) > 0 {
			linesToCheck = ctx.AllFileLines
		} else {
			result.NotEvaluable = true
			result.Details = "no file content context available (all_files requires project context)"
			return result
		}
	default:
		linesToCheck = ctx.DiffLines
	}

	for _, line := range linesToCheck {
		for _, pattern := range p.Patterns {
			if strings.Contains(line, pattern) {
				result.Passed = false
				result.Details = fmt.Sprintf("forbidden pattern %q found", pattern)
				return result
			}
		}
	}
	return result
}
