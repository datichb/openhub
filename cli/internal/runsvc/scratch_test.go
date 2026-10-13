package runsvc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// SCR: a session of a workflow with scratch folders gets them (0700), their
// path in the session context before the first prompt, and the session
// rules that open them for this session only.
func TestStartSessionScratch(t *testing.T) {
	f := newRTFixture(t)
	ad := &ctxAdapter{rtAdapter: f.ad}
	f.svc.Adapter = ad
	f.svc.SessionsDir = filepath.Join(f.root, "sessions")
	f.b.Spec.Workflow = &sessionspec.WorkflowRuntime{ID: "debug", Scratch: []sessionspec.ScratchDef{{Owner: "debugger", Readers: []string{"developer"}}}}
	req := f.request(f.project)
	req.Runtime, req.Prompt = "", "go"
	res, err := f.svc.StartSession(context.Background(), req)
	require.NoError(t, err)

	dir := bundle.ScratchDir(f.svc.SessionsDir, res.SessionID, "debugger")
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())

	require.Len(t, ad.calls, 2)
	assert.True(t, strings.HasPrefix(ad.calls[0], "set oh.scratch="), ad.calls[0])
	assert.Contains(t, ad.calls[0], `"debugger":"`+dir+`"`)
	assert.Equal(t, "prompt", ad.calls[1])

	created := f.ad.created[len(f.ad.created)-1]
	assert.Subset(t, created.SessionRules, bundle.ScratchSessionRules(f.b.Spec.Workflow, f.svc.SessionsDir, res.SessionID))
	assert.Contains(t, created.SessionRules, sessionspec.PermissionRule{Action: bundle.ActionOutsideDir, Resource: filepath.ToSlash(dir) + "/*", Effect: sessionspec.EffectAllow})
}
