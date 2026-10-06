package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

type memRemote map[string]domain.RemoteRef

func (m memRemote) GetRemoteRef(_ context.Context, id string) (*domain.RemoteRef, error) {
	r, ok := m[id]
	if !ok {
		return nil, nil
	}
	return &r, nil
}
func (m memRemote) SetRemoteRef(_ context.Context, id string, r domain.RemoteRef) error {
	m[id] = r
	return nil
}
func (m memRemote) ListRemote(context.Context) (map[string]domain.RemoteRef, error) { return m, nil }

func TestHeartbeatTicketsOfPendingRemoteSessions(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	script := "#!/bin/sh\necho \"$PWD $@\" >> " + log + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bd"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	project := t.TempDir()

	heartbeatTickets(context.Background(), domain.RemoteRef{ProjectDir: project, Tickets: []string{"bd-1", "bd-2"}})
	data, err := os.ReadFile(log)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, 2)
	assert.True(t, strings.HasSuffix(lines[0], "heartbeat bd-1"), lines[0])
	assert.Contains(t, lines[0], filepath.Base(project), "run in the project folder")

	m := memRemote{"s1": {Status: domain.RemoteReady}}
	assert.False(t, anyPendingRemote(context.Background(), m))
	m["s2"] = domain.RemoteRef{Status: domain.RemoteRunning}
	assert.True(t, anyPendingRemote(context.Background(), m))
}
