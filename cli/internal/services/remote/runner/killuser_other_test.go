//go:build !linux

package runner

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKillUserUnsupported(t *testing.T) {
	require.NoError(t, KillUser(nil))
	require.Error(t, KillUser(&User{UID: 10001}), "never push while the agent may still run")
}
