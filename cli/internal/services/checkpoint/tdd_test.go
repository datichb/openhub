package checkpoint

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// french sets the language of the messages checked.
func french(t *testing.T) {
	prev := i18n.Locale()
	i18n.SetLocale("fr")
	t.Cleanup(func() { i18n.SetLocale(prev) })
}

// R7: cp-seams records where the changes are counted from, cp-red checks
// the test files and that the tests fail for the right reason.

// redWorkflow is the TDD part of the ticket workflow: cp-seams then cp-red.
func redWorkflow(outside string) *sessionspec.WorkflowRuntime {
	return &sessionspec.WorkflowRuntime{ID: "ticket", DefaultMode: "auto", Method: "tdd", Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-seams", Behaviors: map[string]string{"auto": "auto"}, Base: true},
		{ID: "cp-red", Behaviors: map[string]string{"auto": "auto"}, Verify: &sessionspec.VerifyDef{From: "stack", Kinds: []string{"build", "test", "typecheck"},
			Expect: "fail", TimeoutSec: 120, MustTouchTests: true, OutsideTests: outside, Since: "cp-seams"}},
	}}
}

func newRedService(t *testing.T, outside string) (*Service, context.Context, *fakeExec, *fakeHead, *[]string, *string) {
	t.Helper()
	french(t)
	svc, ctx := newTestService(t)
	writeBundle(t, svc.BundlesDir, "h_red", redWorkflow(outside))
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module x\n")
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_v", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h_red", Mode: "auto", State: domain.RunActive, LaunchPath: dir, StartRef: "c0"}))
	ex := &fakeExec{exits: map[string]int{}}
	h := &fakeHead{head: "c1"}
	changed, since := []string{}, ""
	svc.Exec = ex.run
	svc.Fingerprint = func(context.Context, string) string { return h.head + strings.Join(changed, ",") }
	svc.Head = func(context.Context, string) (string, bool, error) { return h.head, false, nil }
	svc.Changed = func(_ context.Context, _, from string) ([]string, error) { since = from; return changed, nil }
	svc.VerifyWait = 2 * time.Second
	return svc, ctx, ex, h, &changed, &since
}

func TestRedChecks(t *testing.T) {
	svc, ctx, ex, h, changed, since := newRedService(t, sessionspec.OutsideTestsWarn)
	_, err := svc.Reached(ctx, "ses_v", Call{ID: "cp-seams"})
	require.NoError(t, err)
	st, _ := svc.State(ctx, "ses_v")
	assert.Equal(t, "c1", st.Bases["cp-seams"], "the commit at cp-seams")

	h.head = "c2" // a commit since: the changes are still counted from cp-seams
	*changed = []string{"calc.go"}
	_, err = svc.Reached(ctx, "ses_v", Call{ID: "cp-red"})
	require.ErrorIs(t, err, ErrVerifyFailed, "no test file")
	assert.Contains(t, err.Error(), "aucun fichier de test modifié depuis cp-seams")
	assert.Equal(t, "c1", *since)
	assert.Equal(t, 0, ex.count(), "nothing run")

	*changed = []string{"calc.go", "calc_test.go"}
	_, err = svc.Reached(ctx, "ses_v", Call{ID: "cp-red"})
	require.ErrorIs(t, err, ErrVerifyFailed, "the tests pass")
	assert.Contains(t, err.Error(), "un échec était attendu")
	assert.Equal(t, []string{"go build ./...", "go test ./..."}, ex.runs)

	ex.runs = nil
	ex.exits["go build ./..."] = 1
	*changed = []string{"calc.go", "calc_test.go", "b.go"}
	_, err = svc.Reached(ctx, "ses_v", Call{ID: "cp-red"})
	require.ErrorIs(t, err, ErrVerifyFailed, "the code does not build")
	assert.Contains(t, err.Error(), "go build ./... échoue")
	assert.Equal(t, []string{"go build ./..."}, ex.runs, "the tests are not run")

	ex.exits["go build ./..."] = 0
	ex.exits["go test ./..."] = 1
	ex.out = "# x [x.test]\n./calc_test.go:5:9: undefined: Add\nFAIL\tx [build failed]\n"
	*changed = []string{"calc.go", "calc_test.go", "c.go"}
	_, err = svc.Reached(ctx, "ses_v", Call{ID: "cp-red"})
	require.ErrorIs(t, err, ErrVerifyFailed, "the tests do not compile")
	assert.Contains(t, err.Error(), "les tests ne compilent pas")
	assert.Contains(t, err.Error(), "undefined: Add", "with the excerpt")

	ex.out = "--- FAIL: TestAdd (0.00s)\n    calc_test.go:7: got 0, want 3\nFAIL\tx\t0.01s\n"
	*changed = []string{"calc.go", "calc_test.go"}
	res, err := svc.Reached(ctx, "ses_v", Call{ID: "cp-red"})
	require.NoError(t, err, "red: the test runs and fails")
	assert.Contains(t, res.Verify, "fichiers modifiés hors tests : calc.go", "outside_tests: warn")
	r, _ := svc.VerifyResult(ctx, "ses_v", "cp-red")
	assert.Equal(t, domain.VerifyPassed, r.Status)
	assert.Contains(t, r.Excerpt, "--- FAIL: TestAdd")
}

func TestRedOutsideRefused(t *testing.T) {
	svc, ctx, ex, _, changed, since := newRedService(t, sessionspec.OutsideTestsRefuse)
	ex.exits["go test ./..."] = 1
	*changed = []string{"calc_test.go", "calc.go"}
	_, err := svc.Reached(ctx, "ses_v", Call{ID: "cp-red"})
	require.ErrorIs(t, err, ErrVerifyFailed)
	assert.Contains(t, err.Error(), "seuls les tests sont acceptés")
	assert.Equal(t, "c0", *since, "cp-seams not passed: from the start of the session")

	*changed = []string{"calc_test.go", "testdata/in.txt"}
	_, err = svc.Reached(ctx, "ses_v", Call{ID: "cp-red"})
	require.NoError(t, err, "only test files")
}

func TestRedHelpers(t *testing.T) {
	assert.Equal(t, "a, b", listed([]string{"a", "b"}))
	assert.Equal(t, "1, 2, 3, 4, 5… (+2)", listed([]string{"1", "2", "3", "4", "5", "6", "7"}))
	tests, outside := splitTests([]string{"a/b_test.go", "a/b.go", "web/x.spec.ts", "tests/t.py"}, []string{"*_test.go", "*.spec.*", "**/tests/**"})
	assert.Equal(t, []string{"a/b_test.go", "web/x.spec.ts", "tests/t.py"}, tests)
	assert.Equal(t, []string{"a/b.go"}, outside)
}

// R7 with real git and go (no model): a test that fails passes cp-red, a
// test that does not compile, a suite that passes and a diff without test
// file are refused.
func TestRedRealGo(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not found")
	}
	french(t)
	svc, ctx := newTestService(t)
	writeBundle(t, svc.BundlesDir, "h_red", redWorkflow(sessionspec.OutsideTestsWarn))
	dir := t.TempDir()
	gittest.Run(t, dir, "init", "-q")
	writeFile(t, dir, "go.mod", "module calc\n\ngo 1.21\n")
	writeFile(t, dir, "calc.go", "package calc\n\n// Add is a stub (cp-seams).\nfunc Add(a, b int) int { return 0 }\n")
	gittest.Run(t, dir, "add", ".")
	gittest.Run(t, dir, "commit", "-qm", "init")
	start := gittest.Run(t, dir, "rev-parse", "HEAD")
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_v", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h_red", Mode: "auto", State: domain.RunActive, LaunchPath: dir, StartRef: start}))
	svc.VerifyWait = 2 * time.Minute

	_, err := svc.Reached(ctx, "ses_v", Call{ID: "cp-seams"})
	require.NoError(t, err)
	st, _ := svc.State(ctx, "ses_v")
	assert.Equal(t, start, st.Bases["cp-seams"])

	_, err = svc.Reached(ctx, "ses_v", Call{ID: "cp-red"})
	require.ErrorIs(t, err, ErrVerifyFailed, "no test file")

	writeFile(t, dir, "calc_test.go", "package calc\n\nimport \"testing\"\n\nfunc TestSub(t *testing.T) {\n\tif Sub(3, 1) != 2 {\n\t\tt.Fatal(\"sub\")\n\t}\n}\n")
	_, err = svc.Reached(ctx, "ses_v", Call{ID: "cp-red"})
	require.ErrorIs(t, err, ErrVerifyFailed, "the test does not compile")
	assert.Contains(t, err.Error(), "les tests ne compilent pas")

	writeFile(t, dir, "calc_test.go", "package calc\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif got := Add(1, 2); got != 3 {\n\t\tt.Fatalf(\"got %d, want 3\", got)\n\t}\n}\n")
	res, err := svc.Reached(ctx, "ses_v", Call{ID: "cp-red"})
	require.NoError(t, err, "red for the right reason")
	assert.NotContains(t, res.Verify, "hors tests", "only a test file since cp-seams")

	writeFile(t, dir, "calc.go", "package calc\n\n// Add returns a + b.\nfunc Add(a, b int) int { return a + b }\n")
	_, err = svc.Reached(ctx, "ses_v", Call{ID: "cp-red"})
	require.ErrorIs(t, err, ErrVerifyFailed, "implemented: the tests pass")
	assert.Contains(t, err.Error(), "un échec était attendu")
	r, _ := svc.VerifyResult(ctx, "ses_v", "cp-red")
	assert.Contains(t, r.Reason, "fichiers modifiés hors tests : calc.go")
}
