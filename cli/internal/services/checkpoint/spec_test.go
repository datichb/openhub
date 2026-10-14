package checkpoint

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// R4: cp-spec checks the spec written under docs/wiki/specs/ (glossary,
// criteria with test points), cp-converge its breakdown; nothing else runs.

func specWorkflow() *sessionspec.WorkflowRuntime {
	paths := []string{"docs/wiki/specs/**"}
	return &sessionspec.WorkflowRuntime{ID: "spec", DefaultMode: "auto", Outputs: []sessionspec.OutputDef{{ID: "spec", Type: "path", Record: RecordSpec}}, Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-spec", Behaviors: map[string]string{"auto": "auto"}, Verify: &sessionspec.VerifyDef{Expect: "pass", TimeoutSec: 60,
			Spec: &sessionspec.SpecCheckDef{Paths: paths}}},
		{ID: "cp-converge", Behaviors: map[string]string{"auto": "auto"}, Verify: &sessionspec.VerifyDef{Expect: "pass", TimeoutSec: 60,
			Spec: &sessionspec.SpecCheckDef{Paths: paths, Breakdown: true}}},
	}}
}

func TestSpecChecks(t *testing.T) {
	french(t)
	svc, ctx := newTestService(t)
	writeBundle(t, svc.BundlesDir, "h_spec", specWorkflow())
	dir := t.TempDir()
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_s", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h_spec", Mode: "auto", State: domain.RunActive, LaunchPath: dir, StartRef: "c0"}))
	ex := &fakeExec{exits: map[string]int{}}
	var changed []string
	state := 0
	svc.Exec = ex.run
	svc.Fingerprint = func(context.Context, string) string { state++; return strings.Repeat("x", state) }
	svc.Changed = func(context.Context, string, string) ([]string, error) { return changed, nil }
	svc.VerifyWait = 2 * time.Second
	spec := filepath.Join(dir, "docs", "wiki", "specs", "percent.md")
	write := func(s string) {
		require.NoError(t, os.MkdirAll(filepath.Dir(spec), 0o755))
		require.NoError(t, os.WriteFile(spec, []byte(s), 0o644))
	}

	changed = []string{"calc.go"}
	_, err := svc.Reached(ctx, "ses_s", Call{ID: "cp-spec"})
	require.ErrorIs(t, err, ErrVerifyFailed, "no spec written")
	assert.Contains(t, err.Error(), "aucune spec modifiée sous docs/wiki/specs/**")

	changed = []string{"docs/wiki/specs/percent.md"}
	write("# Spec\n## Critères d'acceptation\n- CA-1 total nul : erreur\n- CA-2 plafond · Test : TestPercent_Cap\n")
	_, err = svc.Reached(ctx, "ses_s", Call{ID: "cp-spec"})
	require.ErrorIs(t, err, ErrVerifyFailed)
	assert.Contains(t, err.Error(), "spec incomplète : 2 problème(s)")
	assert.Contains(t, err.Error(), "docs/wiki/specs/percent.md : pas de section « Glossaire »")
	assert.Contains(t, err.Error(), "CA-1 ne nomme pas son point de test")

	body := "# Spec\n## Glossaire\n- **Part** : la valeur\n## Critères d'acceptation\n- CA-1 total nul · Test : TestPercent_Zero\n- CA-2 plafond · Test : TestPercent_Cap\n"
	write(body)
	_, err = svc.Reached(ctx, "ses_s", Call{ID: "cp-spec"})
	require.NoError(t, err)
	r, ok := svc.VerifyResult(ctx, "ses_s", "cp-spec")
	require.True(t, ok)
	assert.Equal(t, domain.VerifyPassed, r.Status)
	assert.Equal(t, 0, ex.count(), "no command run")
	sess, err := svc.Sessions.Get(ctx, "ses_s")
	require.NoError(t, err)
	assert.Equal(t, "docs/wiki/specs/percent.md", sess.Outputs["spec"], "R5: the spec checked is recorded by oh")

	write(body + "## Découpage\n- T1 · Couvre : CA-1\n- T2 · la doc\n")
	_, err = svc.Reached(ctx, "ses_s", Call{ID: "cp-converge"})
	require.ErrorIs(t, err, ErrVerifyFailed, "a gap between the breakdown and the spec")
	assert.Contains(t, err.Error(), "tranche hors spec, elle ne couvre aucun critère : T2 · la doc")
	assert.Contains(t, err.Error(), "CA-2 n'est couvert par aucune tranche du découpage")

	write(body + "## Découpage\n- T1 · Couvre : CA-1\n- T2 · Couvre : CA-2\n")
	_, err = svc.Reached(ctx, "ses_s", Call{ID: "cp-converge"})
	require.NoError(t, err)

	// A link is not followed.
	require.NoError(t, os.Remove(spec))
	require.NoError(t, os.Symlink(filepath.Join(dir, "elsewhere.md"), spec))
	_, err = svc.Reached(ctx, "ses_s", Call{ID: "cp-spec"})
	require.ErrorIs(t, err, ErrVerifyFailed)
	assert.Contains(t, err.Error(), "aucune spec modifiée")
}
