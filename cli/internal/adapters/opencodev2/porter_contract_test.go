//go:build integration

package opencodev2

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Remote runs (P5-T14, T16): a session exported from the job's server is
// imported on the machine's server at another location, with the same id.
func TestContractSessionExportImport(t *testing.T) {
	cfg := `{"agents":{"build":{"disabled":true},"plan":{"disabled":true},"general":{"disabled":true},"explore":{"disabled":true},` +
		`"pinger":{"mode":"primary","description":"contract agent"}}}`
	job := startLiveServer(t, cfg, "pinger")
	machine := startLiveServer(t, cfg, "pinger")
	ctx := context.Background()
	id := sessionspec.NewSessionID()
	_, err := job.client.CreateSession(ctx, CreateSessionRequest{ID: id, Title: "remote", Agent: "pinger", Location: &Location{Directory: job.project}})
	require.NoError(t, err)

	a := &Adapter{}
	hJob := adapters.ServerHandle{URL: job.URL, Password: job.Password}
	hMachine := adapters.ServerHandle{URL: machine.URL, Password: machine.Password}
	data, err := a.ExportSession(ctx, hJob, id)
	require.NoError(t, err)
	var tr struct {
		Info     map[string]any `json:"info"`
		Messages []any          `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(data, &tr))
	assert.Equal(t, id, tr.Info["id"])

	got, err := a.ImportSession(ctx, hMachine, data, machine.project)
	require.NoError(t, err)
	assert.Equal(t, id, got)
	s, err := machine.client.GetSession(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, machine.project, s.Location.Directory)
	assert.Equal(t, "remote", s.Title)

	_, err = a.ImportSession(ctx, hMachine, data, machine.project)
	assert.ErrorIs(t, err, adapters.ErrSessionExists)
}
