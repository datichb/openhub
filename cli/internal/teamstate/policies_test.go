package teamstate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPolicies_GlobalOnly(t *testing.T) {
	dir := t.TempDir()
	repo := &Repo{path: dir}

	content := `
[policies.branch_naming]
type = "regex"
rule = "^(feat|fix)/[a-z0-9-]+"
enforcement = "refuse"
message = "Invalid branch name"

[policies.commit_format]
type = "regex"
rule = "^(feat|fix): .+"
enforcement = "warn"

[policies.max_ticket_wip]
type = "limit"
max = 2
enforcement = "warn"
message = "Too many tickets in progress"
`
	if err := os.WriteFile(filepath.Join(dir, "policies.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	policies, err := repo.LoadPolicies("")
	if err != nil {
		t.Fatalf("LoadPolicies failed: %v", err)
	}

	if len(policies) != 3 {
		t.Fatalf("expected 3 policies, got %d", len(policies))
	}

	// Check that names are set
	found := map[string]bool{}
	for _, p := range policies {
		found[p.Name] = true
	}
	for _, name := range []string{"branch_naming", "commit_format", "max_ticket_wip"} {
		if !found[name] {
			t.Errorf("policy %q not found", name)
		}
	}
}

func TestLoadPolicies_WithOverride(t *testing.T) {
	dir := t.TempDir()
	repo := &Repo{path: dir}

	// Create global policies
	global := `
[policies.tests_required]
type = "boolean"
enabled = true
enforcement = "warn"
message = "Tests should pass"

[policies.branch_naming]
type = "regex"
rule = "^feat/"
enforcement = "warn"
`
	if err := os.WriteFile(filepath.Join(dir, "policies.toml"), []byte(global), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create project override (stricter)
	overrideDir := filepath.Join(dir, "projects", "T-SRU")
	if err := os.MkdirAll(overrideDir, 0o755); err != nil {
		t.Fatal(err)
	}
	override := `
[policies.tests_required]
enforcement = "refuse"
message = "Tests MUST pass on T-SRU"
`
	if err := os.WriteFile(filepath.Join(overrideDir, "policies-override.toml"), []byte(override), 0o644); err != nil {
		t.Fatal(err)
	}

	policies, err := repo.LoadPolicies("T-SRU")
	if err != nil {
		t.Fatalf("LoadPolicies with override failed: %v", err)
	}

	// Find tests_required
	var testsPolicy *Policy
	for i := range policies {
		if policies[i].Name == "tests_required" {
			testsPolicy = &policies[i]
			break
		}
	}
	if testsPolicy == nil {
		t.Fatal("tests_required policy not found")
	}

	// Should be upgraded to refuse
	if testsPolicy.Enforcement != EnforcementRefuse {
		t.Errorf("expected enforcement=refuse after override, got %q", testsPolicy.Enforcement)
	}
	if testsPolicy.Message != "Tests MUST pass on T-SRU" {
		t.Errorf("expected overridden message, got %q", testsPolicy.Message)
	}
}

func TestOverride_CannotWeaken(t *testing.T) {
	dir := t.TempDir()
	repo := &Repo{path: dir}

	global := `
[policies.branch_naming]
type = "regex"
rule = "^feat/"
enforcement = "refuse"
message = "strict"
`
	if err := os.WriteFile(filepath.Join(dir, "policies.toml"), []byte(global), 0o644); err != nil {
		t.Fatal(err)
	}

	overrideDir := filepath.Join(dir, "projects", "lax")
	if err := os.MkdirAll(overrideDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Try to weaken from refuse to warn
	override := `
[policies.branch_naming]
enforcement = "warn"
`
	if err := os.WriteFile(filepath.Join(overrideDir, "policies-override.toml"), []byte(override), 0o644); err != nil {
		t.Fatal(err)
	}

	policies, err := repo.LoadPolicies("lax")
	if err != nil {
		t.Fatalf("LoadPolicies failed: %v", err)
	}

	for _, p := range policies {
		if p.Name == "branch_naming" {
			if p.Enforcement != EnforcementRefuse {
				t.Errorf("override weakened enforcement from refuse to %q", p.Enforcement)
			}
			return
		}
	}
	t.Error("branch_naming policy not found")
}

func TestCheckPolicy_Regex_BranchNaming(t *testing.T) {
	p := Policy{
		Name:        "branch_naming",
		Type:        PolicyTypeRegex,
		Rule:        `^(feat|fix|chore)/[a-z0-9-]+$`,
		Enforcement: EnforcementRefuse,
		Message:     "bad branch",
	}

	tests := []struct {
		branch string
		pass   bool
	}{
		{"feat/my-feature", true},
		{"fix/bug-123", true},
		{"chore/cleanup", true},
		{"main", false},
		{"feature/My-Feature", false},
		{"feat/", false},
	}

	for _, tc := range tests {
		ctx := PolicyContext{BranchName: tc.branch}
		result := CheckPolicy(p, ctx)
		if result.Passed != tc.pass {
			t.Errorf("branch=%q: expected passed=%v, got %v (details: %s)",
				tc.branch, tc.pass, result.Passed, result.Details)
		}
	}
}

func TestCheckPolicy_Regex_CommitFormat(t *testing.T) {
	p := Policy{
		Name:        "commit_format",
		Type:        PolicyTypeRegex,
		Rule:        `^(feat|fix|docs|refactor|test|chore)(\(.+\))?: .+`,
		Enforcement: EnforcementRefuse,
	}

	tests := []struct {
		msg  string
		pass bool
	}{
		{"feat: add login page", true},
		{"fix(auth): resolve timeout", true},
		{"docs: update readme", true},
		{"WIP save progress", false},
		{"fixed a bug", false},
		{"", false},
	}

	for _, tc := range tests {
		ctx := PolicyContext{CommitMessage: tc.msg}
		result := CheckPolicy(p, ctx)
		if result.Passed != tc.pass {
			t.Errorf("msg=%q: expected passed=%v, got %v", tc.msg, tc.pass, result.Passed)
		}
	}
}

func TestCheckPolicy_Limit_MaxWIP(t *testing.T) {
	p := Policy{
		Name:        "max_ticket_wip",
		Type:        PolicyTypeLimit,
		Max:         2,
		Enforcement: EnforcementWarn,
		Message:     "too many",
	}

	tests := []struct {
		claims int
		pass   bool
	}{
		{0, true},
		{1, true},
		{2, false}, // >= max
		{3, false},
	}

	for _, tc := range tests {
		ctx := PolicyContext{ActiveClaims: tc.claims}
		result := CheckPolicy(p, ctx)
		if result.Passed != tc.pass {
			t.Errorf("claims=%d: expected passed=%v, got %v", tc.claims, tc.pass, result.Passed)
		}
	}
}

func TestCheckPolicy_ForbiddenPattern(t *testing.T) {
	p := Policy{
		Name:        "custom_no_console_log",
		Type:        PolicyTypeForbiddenPattern,
		Patterns:    []string{"console.log", "console.warn"},
		Scope:       "diff_only",
		Enforcement: EnforcementWarn,
	}

	tests := []struct {
		diff []string
		pass bool
	}{
		{[]string{"const x = 1", "return x"}, true},
		{[]string{"console.log('debug')", "const x = 1"}, false},
		{[]string{"// console.warn is bad", "x = console.warn(y)"}, false},
		{nil, true},
		{[]string{}, true},
	}

	for i, tc := range tests {
		ctx := PolicyContext{DiffLines: tc.diff}
		result := CheckPolicy(p, ctx)
		if result.Passed != tc.pass {
			t.Errorf("test %d: expected passed=%v, got %v (details: %s)",
				i, tc.pass, result.Passed, result.Details)
		}
	}
}

func TestCheckPolicy_Boolean_Disabled(t *testing.T) {
	p := Policy{
		Name:        "review_required",
		Type:        PolicyTypeBoolean,
		Enabled:     false,
		Enforcement: EnforcementRefuse,
	}

	ctx := PolicyContext{}
	result := CheckPolicy(p, ctx)
	if !result.Passed {
		t.Error("disabled boolean policy should pass")
	}
}

func TestHasRefuseViolations(t *testing.T) {
	results := []PolicyResult{
		{Name: "a", Passed: false, Enforcement: EnforcementWarn},
		{Name: "b", Passed: true, Enforcement: EnforcementRefuse},
	}
	if HasRefuseViolations(results) {
		t.Error("should not have refuse violations (b passed)")
	}

	results = append(results, PolicyResult{Name: "c", Passed: false, Enforcement: EnforcementRefuse})
	if !HasRefuseViolations(results) {
		t.Error("should have refuse violations (c failed)")
	}
}

func TestLoadPolicies_NoFile(t *testing.T) {
	dir := t.TempDir()
	repo := &Repo{path: dir}

	policies, err := repo.LoadPolicies("")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if policies != nil {
		t.Fatalf("expected nil policies, got %d", len(policies))
	}
}

func TestCheckAll(t *testing.T) {
	dir := t.TempDir()
	repo := &Repo{path: dir}

	content := `
[policies.branch_naming]
type = "regex"
rule = "^feat/"
enforcement = "refuse"
message = "bad branch"

[policies.max_ticket_wip]
type = "limit"
max = 3
enforcement = "warn"
message = "too many WIP"
`
	if err := os.WriteFile(filepath.Join(dir, "policies.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := PolicyContext{
		BranchName:   "fix/something", // violates branch_naming
		ActiveClaims: 4,               // violates max_wip
	}

	violations, err := repo.CheckAll("", ctx)
	if err != nil {
		t.Fatalf("CheckAll failed: %v", err)
	}
	if len(violations) != 2 {
		t.Fatalf("expected 2 violations, got %d", len(violations))
	}
	if !HasRefuseViolations(violations) {
		t.Error("expected at least one refuse violation")
	}
}
