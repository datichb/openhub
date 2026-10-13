package checkpoint

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// SCR: the session rules re-applied as the session moves on keep opening
// its scratch folders (and only its own).
func TestRulesKeepScratch(t *testing.T) {
	svc, ctx := newTestService(t)
	wf := TestWorkflow
	wf.Scratch = []sessionspec.ScratchDef{{Owner: "debugger", Readers: []string{"developer"}}}
	writeBundle(t, svc.BundlesDir, "h_scratch", &wf)
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_s", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h_scratch", Mode: "semi-auto", State: domain.RunActive}))
	rules, err := svc.Rules(ctx, "ses_s")
	require.NoError(t, err)
	assert.Contains(t, rules, sessionspec.PermissionRule{Action: bundle.ActionOutsideDir,
		Resource: filepath.ToSlash(bundle.ScratchDir(svc.SessionsDir, "ses_s", "debugger")) + "/*", Effect: sessionspec.EffectAllow})
	assert.Subset(t, rules, bundle.SessionRules(&wf, "semi-auto", &domain.CheckpointState{}))
}
