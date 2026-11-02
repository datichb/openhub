package remote

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/remote"
)

// statusesBeads answers `bd statuses --json` like bd 1.3 does (built-in and
// custom statuses), or fails.
type statusesBeads struct {
	*fakeBeads
	out string
	err error
}

func (b statusesBeads) Exec(ctx context.Context, dir string, argv []string, stdin []byte) ([]byte, error) {
	if len(argv) > 0 && argv[0] == "statuses" {
		return []byte(b.out), b.err
	}
	return b.fakeBeads.Exec(ctx, dir, argv, stdin)
}

// The snapshot sent with the session carries the statuses of the project's
// Beads configuration: the bd of the job refuses any other one.
func TestSnapshotKeepsConfiguredStatuses(t *testing.T) {
	ctx := context.Background()
	beads := func(out string, err error) statusesBeads {
		return statusesBeads{fakeBeads: &fakeBeads{issues: map[string]map[string]any{"bd-42": {"id": "bd-42", "status": "open", "revision": "r1"}}}, out: out, err: err}
	}
	for _, tc := range []struct {
		name string
		b    statusesBeads
		want []string
	}{
		{"built-in and custom", beads(`{"built_in_statuses":[{"name":"open"},{"name":"in_progress"},{"name":"closed"}],`+
			`"custom_statuses":[{"name":"review","category":"wip"},{"name":"shipped","category":"done"}]}`, nil),
			[]string{"open", "in_progress", "closed", "review", "shipped"}},
		{"custom only: the built-in ones of bd", beads(`{"custom_statuses":[{"name":"review","category":"wip"}]}`, nil),
			[]string{"open", "in_progress", "blocked", "deferred", "closed", "pinned", "hooked", "review"}},
		{"no custom status", beads(`{"built_in_statuses":[{"name":"open"},{"name":"closed"}]}`, nil), []string{"open", "closed"}},
		{"bd cannot say: not known, not checked", beads("", errors.New("bd failed")), nil},
		{"unreadable answer: not known, not checked", beads("not json", nil), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snap, err := TakeSnapshot(ctx, tc.b, "/p", []string{"bd-42"}, time.Now())
			require.NoError(t, err)
			assert.Equal(t, tc.want, snap.Statuses)
			data, err := json.Marshal(snap)
			require.NoError(t, err)
			var back remote.Snapshot
			require.NoError(t, json.Unmarshal(data, &back))
			assert.Equal(t, tc.want, back.Statuses)
		})
	}
}
