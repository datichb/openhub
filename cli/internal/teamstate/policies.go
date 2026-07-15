package teamstate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// PolicyType represents the type of a policy rule.
type PolicyType string

const (
	PolicyTypeRegex           PolicyType = "regex"
	PolicyTypeBoolean         PolicyType = "boolean"
	PolicyTypeLimit           PolicyType = "limit"
	PolicyTypeForbiddenPattern PolicyType = "forbidden_pattern"
)

// PolicyEnforcement defines what happens when a policy is violated.
type PolicyEnforcement string

const (
	EnforcementRefuse PolicyEnforcement = "refuse"
	EnforcementWarn   PolicyEnforcement = "warn"
)

// Policy represents a single team policy rule.
type Policy struct {
	Name        string           `toml:"-"`          // derived from TOML key
	Type        PolicyType       `toml:"type"`
	Rule        string           `toml:"rule,omitempty"`        // regex pattern
	Enabled     bool             `toml:"enabled,omitempty"`     // for boolean type
	Max         int              `toml:"max,omitempty"`         // for limit type
	Unit        string           `toml:"unit,omitempty"`        // for limit type (e.g. "lines")
	Patterns    []string         `toml:"patterns,omitempty"`    // for forbidden_pattern type
	Scope       string           `toml:"scope,omitempty"`       // diff_only | all_files | modified_files | per_feature_branch
	Enforcement PolicyEnforcement `toml:"enforcement"`
	Message     string           `toml:"message,omitempty"`
}

// PolicyResult holds the outcome of a single policy check.
type PolicyResult struct {
	Name        string
	Passed      bool
	Enforcement PolicyEnforcement
	Message     string
	Details     string // additional context (e.g. which pattern matched)
}

// PolicyContext provides the data needed to evaluate policies.
type PolicyContext struct {
	BranchName    string   // current branch name
	CommitMessage string   // commit message to validate
	DiffLines     []string // lines from the diff (added lines only)
	ModifiedFiles []string // file paths modified
	MemberID      string   // who is performing the action
	ActiveClaims  int      // number of active claims for this member
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
func (r *Repo) SavePolicies(policies map[string]Policy) error {
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
	return os.WriteFile(path, data, 0o644)
}

// CheckPolicy evaluates a single policy against the provided context.
func CheckPolicy(p Policy, ctx PolicyContext) PolicyResult {
	result := PolicyResult{
		Name:        p.Name,
		Enforcement: p.Enforcement,
		Message:     p.Message,
		Passed:      true,
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
// Returns only violations (passed=false).
func (r *Repo) CheckAll(project string, ctx PolicyContext) ([]PolicyResult, error) {
	policies, err := r.LoadPolicies(project)
	if err != nil {
		return nil, err
	}

	var violations []PolicyResult
	for _, p := range policies {
		result := CheckPolicy(p, ctx)
		if !result.Passed {
			violations = append(violations, result)
		}
	}
	return violations, nil
}

// HasRefuseViolations returns true if any violation has enforcement = refuse.
func HasRefuseViolations(results []PolicyResult) bool {
	for _, r := range results {
		if !r.Passed && r.Enforcement == EnforcementRefuse {
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
// Overrides can only make enforcement stricter (warn -> refuse), never more permissive.
func mergePolicies(global, overrides map[string]Policy) map[string]Policy {
	merged := make(map[string]Policy, len(global))
	for k, v := range global {
		merged[k] = v
	}
	for k, override := range overrides {
		if base, exists := merged[k]; exists {
			// Only allow stricter enforcement
			if base.Enforcement == EnforcementWarn && override.Enforcement == EnforcementRefuse {
				base.Enforcement = EnforcementRefuse
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

// --- Check functions ---

func checkRegex(p Policy, ctx PolicyContext, result PolicyResult) PolicyResult {
	if p.Rule == "" {
		return result
	}

	re, err := regexp.Compile(p.Rule)
	if err != nil {
		result.Passed = false
		result.Details = fmt.Sprintf("invalid regex: %s", err)
		return result
	}

	// Determine what to check based on common policy names
	var value string
	var applicable bool
	switch {
	case strings.Contains(p.Name, "branch"):
		value = ctx.BranchName
		applicable = ctx.BranchName != ""
	case strings.Contains(p.Name, "commit"):
		value = ctx.CommitMessage
		// Empty commit message is always a violation if the policy applies
		applicable = true
	default:
		// Generic: check if at least one modified file matches
		if len(ctx.ModifiedFiles) == 0 {
			return result // no context, skip
		}
		for _, f := range ctx.ModifiedFiles {
			if re.MatchString(f) {
				return result // pass
			}
		}
		// For per_feature_branch scope, require at least one match
		if p.Scope == "per_feature_branch" {
			result.Passed = false
			result.Details = "no file matching pattern found in branch"
		}
		return result
	}

	if !applicable {
		return result // context not relevant for this check
	}

	if !re.MatchString(value) {
		result.Passed = false
		result.Details = fmt.Sprintf("value %q does not match %s", value, p.Rule)
	}
	return result
}

func checkBoolean(p Policy, ctx PolicyContext, result PolicyResult) PolicyResult {
	// Boolean policies are checked contextually — the CLI layer handles
	// the actual verification (e.g. review_required checks if a review exists).
	// Here we just confirm the policy is enabled.
	if !p.Enabled {
		// Policy disabled — always passes
		result.Passed = true
	}
	return result
}

func checkLimit(p Policy, ctx PolicyContext, result PolicyResult) PolicyResult {
	if p.Max <= 0 {
		return result
	}

	// Check which limit to apply
	switch {
	case strings.Contains(p.Name, "wip") || strings.Contains(p.Name, "ticket"):
		if ctx.ActiveClaims >= p.Max {
			result.Passed = false
			result.Details = fmt.Sprintf("active claims: %d (max: %d)", ctx.ActiveClaims, p.Max)
		}
	case p.Unit == "lines" && strings.Contains(p.Name, "file_length"):
		// File length check is done at file level — skip here
		// The CLI/agent handles per-file checks
	}
	return result
}

func checkForbiddenPattern(p Policy, ctx PolicyContext, result PolicyResult) PolicyResult {
	if len(p.Patterns) == 0 {
		return result
	}

	// Choose lines to check based on scope
	var linesToCheck []string
	switch p.Scope {
	case "diff_only":
		linesToCheck = ctx.DiffLines
	case "all_files", "modified_files":
		// Would need file content — handled by the agent, not here
		return result
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
