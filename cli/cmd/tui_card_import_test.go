package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/beads/bdtest"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// fakeBdForCards puts a bd on the PATH that keeps the tickets it creates in a
// file and answers list --external-ref from it; every call is logged. With
// failCreate the creation fails.
func fakeBdForCards(t *testing.T, failCreate bool) (creates func() int) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell script")
	}
	dir := t.TempDir()
	log, db := filepath.Join(dir, "calls.log"), filepath.Join(dir, "db")
	create := `n=$(( $(wc -l < "` + db + `" 2>/dev/null || echo 0) + 1 ))
  printf '%s\tbt-%s\topen\n' "$ref" "$n" >> "` + db + `"
  echo "{\"id\":\"bt-$n\"}"`
	if failCreate {
		create = `echo "boom" >&2; exit 1`
	}
	script := `#!/bin/sh
echo "$*" >> "` + log + `"
ref=""; prev=""
for a in "$@"; do if [ "$prev" = "--external-ref" ]; then ref="$a"; fi; prev="$a"; done
case " $* " in
*" list "*)
  found=$(grep "^$ref	" "` + db + `" 2>/dev/null | head -1)
  if [ -z "$found" ]; then echo "[]"; else
    id=$(echo "$found" | cut -f2)
    echo "[{\"id\":\"$id\",\"title\":\"t\",\"status\":\"open\",\"external_ref\":\"$ref\"}]"
  fi ;;
*" create "*)
  ` + create + ` ;;
esac
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bd"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return func() int {
		b, _ := os.ReadFile(log)
		n := 0
		for _, l := range strings.Split(string(b), "\n") {
			if strings.Contains(l, " create ") {
				n++
			}
		}
		return n
	}
}

// The issue of a card is imported into Beads when the launch is confirmed, not
// when the form opens nor when its recap is shown: a form closed leaves
// nothing. The form and the recap work on a stand-in ticket and say so.
func TestCardImportWaitsForTheConfirmation(t *testing.T) {
	a, p := dryRunHub(t)
	creates := fakeBdForCards(t, false)
	require.NoError(t, os.MkdirAll(filepath.Join(p.Path, ".beads"), 0o755))
	ticket := views.TicketContext{ID: "623", Title: "Export CSV", ProjectPath: p.Path, ProjectID: p.ID}

	cfg, err := launchFormConfig(t.Context(), a, p, tuiLaunchRequest{WorkflowID: "ticket", Import: &ticket, Card: &tuiCardClaim{Ticket: "623"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"gitlab-623"}, cfg.Tickets, "the form runs on a stand-in named like the link")
	assert.True(t, cfg.AtOptions)

	recap, err := cfg.Recap(t.Context(), views.LaunchChoices{Runtime: "local", Location: "base", Inputs: map[string]string{}, Tickets: cfg.Tickets})
	require.NoError(t, err)
	var rows []string
	for _, r := range recap.Rows {
		rows = append(rows, r.Label+": "+r.Value)
	}
	assert.Contains(t, strings.Join(rows, "\n"), "Beads ticket: #623 will be imported as a Beads ticket when launched (linked as gitlab-623)")
	assert.Zero(t, creates(), "the form and its recap created nothing; closing the form leaves nothing")
}

// Confirming creates the ticket once: a launch that stops after the import
// (here the runtime is unavailable), confirmed again, finds the ticket.
func TestCardImportOnConfirmationIsIdempotent(t *testing.T) {
	a, p := dryRunHub(t)
	creates := fakeBdForCards(t, false)
	require.NoError(t, os.MkdirAll(filepath.Join(p.Path, ".beads"), 0o755))
	ticket := views.TicketContext{ID: "623", Title: "Export CSV", ProjectPath: p.Path, ProjectID: p.ID}
	cfg, err := launchFormConfig(t.Context(), a, p, tuiLaunchRequest{WorkflowID: "ticket", Import: &ticket})
	require.NoError(t, err)

	choices := views.LaunchChoices{Runtime: "container", Location: "base", Inputs: map[string]string{}, Tickets: cfg.Tickets}
	err = cfg.Launch(t.Context(), choices)
	require.Error(t, err, "the container runtime is not available here: the launch stops after the import")
	assert.NotContains(t, err.Error(), "could not be imported")
	assert.Equal(t, 1, creates(), "one ticket created at the confirmation")

	require.Error(t, cfg.Launch(t.Context(), choices))
	assert.Equal(t, 1, creates(), "confirmed again: the ticket is found, not created twice")

	// A form opened later on the same issue sees the ticket the first one made.
	again, err := launchFormConfig(t.Context(), a, p, tuiLaunchRequest{WorkflowID: "ticket", Import: &ticket})
	require.NoError(t, err)
	assert.Equal(t, []string{"bt-1"}, again.Tickets, "the form runs on the ticket the issue already is")
	recap, err := again.Recap(t.Context(), views.LaunchChoices{Runtime: "local", Location: "base", Inputs: map[string]string{}, Tickets: again.Tickets})
	require.NoError(t, err)
	assert.Equal(t, "#623 is ticket bt-1", recap.Rows[0].Value)
	assert.Equal(t, 1, creates())
}

// An import that fails after the confirmation stops the launch before
// anything starts, and the message says so.
func TestCardImportFailureStopsTheLaunch(t *testing.T) {
	a, p := dryRunHub(t)
	creates := fakeBdForCards(t, true)
	require.NoError(t, os.MkdirAll(filepath.Join(p.Path, ".beads"), 0o755))
	ticket := views.TicketContext{ID: "623", Title: "Export CSV", ProjectPath: p.Path, ProjectID: p.ID}
	cfg, err := launchFormConfig(t.Context(), a, p, tuiLaunchRequest{WorkflowID: "ticket", Import: &ticket})
	require.NoError(t, err)

	err = cfg.Launch(t.Context(), views.LaunchChoices{Runtime: "local", Location: "base", Inputs: map[string]string{}, Tickets: cfg.Tickets})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Issue #623 could not be imported into Beads, nothing was launched")
	assert.Contains(t, err.Error(), "boom", "what bd said")
	assert.Equal(t, 1, creates(), "the creation was attempted at the confirmation, and only then")
}

// With the real bd: confirming twice leaves a single ticket linked to the
// issue.
func TestCardImportOnConfirmationRealBd(t *testing.T) {
	bdtest.RequireRealBd(t)
	a, p := dryRunHub(t)
	dir := bdtest.InitedRepo(t)
	t.Setenv("BEADS_DIR", "")
	ticket := views.TicketContext{ID: "623", Title: "Export CSV", ProjectPath: dir, ProjectID: p.ID}
	cfg, err := launchFormConfig(t.Context(), a, p, tuiLaunchRequest{WorkflowID: "ticket", Import: &ticket})
	require.NoError(t, err)
	all, err := beads.ListAll(dir)
	require.NoError(t, err)
	require.Empty(t, all, "opening the form created nothing")

	choices := views.LaunchChoices{Runtime: "container", Location: "base", Inputs: map[string]string{}, Tickets: cfg.Tickets}
	require.Error(t, cfg.Launch(t.Context(), choices))
	require.Error(t, cfg.Launch(t.Context(), choices))
	all, err = beads.ListAll(dir)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, "gitlab-623", all[0].ExternalRef)
}
