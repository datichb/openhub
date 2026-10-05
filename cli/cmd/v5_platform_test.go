package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/domain"
)

func TestTaskStatusMapping(t *testing.T) {
	cases := map[domain.RunState]string{
		domain.RunActive: "running", domain.RunWaiting: "running", domain.RunPreparing: "running",
		domain.RunIdle: "idle", domain.RunSleeping: "idle",
		domain.RunStopped: "completed", domain.RunCompleted: "completed", domain.RunFailed: "failed",
	}
	for in, want := range cases {
		assert.Equal(t, want, taskStatus(in), string(in))
	}
}
