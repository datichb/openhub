package container

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEstimate(t *testing.T) {
	st := &fakeStore{images: map[string]string{}}
	r := newImageEngine(t, st)
	tool := &fakeTool{ver: "2.0.20", bin: writeBin(t)}
	g := testGroup(t, tool)
	rt := newImageRuntime(t, r)
	ctx := context.Background()

	est, err := rt.Estimate(ctx, g)
	require.NoError(t, err)
	assert.False(t, est.Ready)
	assert.Equal(t, []string{StepBase, StepLayer}, est.Steps)
	assert.Zero(t, est.Duration, "first build: unknown")

	img, err := rt.EnsureImage(ctx, g)
	require.NoError(t, err)
	runs := len(r.called("docker run"))
	est, err = rt.Estimate(ctx, g)
	require.NoError(t, err)
	assert.True(t, est.Ready)
	assert.Equal(t, img.Ref, est.Image)
	assert.Len(t, r.called("docker run"), runs, "no container started (libc read from the probe cache)")

	// A new tool version: only the layer is rebuilt, with a known duration.
	rt.recordBuild(g.ProjectID, "dev", 12*time.Second)
	rt.recordBuild(g.ProjectID, "base", 90*time.Second)
	tool.ver = "2.0.21"
	est, err = rt.Estimate(ctx, g)
	require.NoError(t, err)
	assert.False(t, est.Ready)
	assert.Equal(t, []string{StepLayer}, est.Steps)
	assert.Equal(t, 12*time.Second, est.Duration)
	assert.NotEqual(t, img.Ref, est.Image)

	// Another Dockerfile content: base and layer, both known.
	g.BuildArgs = map[string]string{"NODE": "22"}
	est, err = rt.Estimate(ctx, g)
	require.NoError(t, err)
	assert.Equal(t, []string{StepBase, StepLayer}, est.Steps)
	assert.Equal(t, 102*time.Second, est.Duration)
	assert.Len(t, r.called("docker build"), 2, "estimating builds nothing")
}

func TestEnsureImageRecordsBuildTimes(t *testing.T) {
	st := &fakeStore{images: map[string]string{}}
	rt := newImageRuntime(t, newImageEngine(t, st))
	g := testGroup(t, &fakeTool{ver: "2.0.20", bin: writeBin(t)})
	_, err := rt.EnsureImage(context.Background(), g)
	require.NoError(t, err)
	times := rt.loadBuildTimes(g.ProjectID)
	assert.Positive(t, times.Base)
	assert.Positive(t, times.Layer)
}
