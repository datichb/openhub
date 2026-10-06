package runsvc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
)

type estimatingRuntime struct {
	*fakeRuntime
	got ohruntime.Group
}

func (r *estimatingRuntime) Estimate(_ context.Context, g ohruntime.Group) (ohruntime.PrepareEstimate, error) {
	r.got = g
	return ohruntime.PrepareEstimate{Steps: []string{"layer"}, Duration: time.Minute}, nil
}

func TestPrepareEstimate(t *testing.T) {
	f := newRTFixture(t)
	ctx := context.Background()

	// The fixture runtime cannot estimate.
	_, ok, err := f.svc.PrepareEstimate(ctx, f.request(f.project))
	require.NoError(t, err)
	assert.False(t, ok)

	er := &estimatingRuntime{fakeRuntime: f.rt}
	f.svc.Runtimes[f.rt.Kind()] = er
	req := f.request(f.project)
	est, ok, err := f.svc.PrepareEstimate(ctx, req)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, time.Minute, est.Duration)
	assert.Equal(t, "Dockerfile.dev", er.got.Dockerfile)
	assert.Equal(t, f.project, er.got.ProjectDir)
	assert.NotNil(t, er.got.Tool, "the adapter tool decides the image")
	assert.Empty(t, f.rt.prepares, "nothing prepared")

	req.Runtime = ""
	_, ok, err = f.svc.PrepareEstimate(ctx, req)
	require.NoError(t, err)
	assert.False(t, ok, "local: nothing to estimate")
}
