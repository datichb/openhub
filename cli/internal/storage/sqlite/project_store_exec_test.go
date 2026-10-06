package sqlite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

func TestProjectStore_ExecConfigRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ctx := context.Background()

	p := &domain.Project{ID: "p1", Name: "p1", Path: "/tmp/p1", Status: domain.ProjectStatusActive}
	require.NoError(t, ps.Create(ctx, p))
	got, err := ps.Get(ctx, "p1")
	require.NoError(t, err)
	assert.Nil(t, got.ExecConfig, "no setting: nil")

	got.ExecConfig = &domain.ProjectExecConfig{
		Dockerfile: "docker/dev.Dockerfile", BuildArgs: map[string]string{"NODE": "22"},
		Volumes: []string{"node_modules"}, DefaultWorkflow: "ticket", DefaultRuntime: "container",
	}
	require.NoError(t, ps.Update(ctx, got))
	again, err := ps.Get(ctx, "p1")
	require.NoError(t, err)
	assert.Equal(t, got.ExecConfig, again.ExecConfig)

	list, err := ps.List(ctx, "")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "ticket", list[0].ExecConfig.DefaultWorkflow)

	// An emptied config is stored as nothing.
	again.ExecConfig = &domain.ProjectExecConfig{}
	require.NoError(t, ps.Update(ctx, again))
	cleared, err := ps.Get(ctx, "p1")
	require.NoError(t, err)
	assert.Nil(t, cleared.ExecConfig)
}

func TestProjectStore_ExecConfigUnreadable(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ctx := context.Background()
	require.NoError(t, ps.Create(ctx, &domain.Project{ID: "p1", Name: "p1", Path: "/tmp/p1", Status: domain.ProjectStatusActive}))
	_, err := s.DB().ExecContext(ctx, `UPDATE projects SET exec_config = '{not json' WHERE id = 'p1'`)
	require.NoError(t, err)
	got, err := ps.Get(ctx, "p1")
	require.NoError(t, err)
	assert.Nil(t, got.ExecConfig)
}
