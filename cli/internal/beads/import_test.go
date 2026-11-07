package beads

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/beads/bdtest"
	"github.com/datichb/openhub/cli/internal/gateway/bdargs"
)

// fakeBd puts a bd on the PATH that keeps the tickets it creates in a file of
// its own and answers list --external-ref from it; every call is logged.
func fakeBd(t *testing.T) (calls func() []string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell script")
	}
	// A machine under load can take longer than the 10 s of a bd command to
	// start the script: the process is then killed and the error has no message.
	patientBd(t)
	dir := t.TempDir()
	log, db := filepath.Join(dir, "calls.log"), filepath.Join(dir, "db")
	script := `#!/bin/sh
echo "$*" >> "` + log + `"
case " $* " in
*" list "*)
  ref=""; prev=""
  for a in "$@"; do if [ "$prev" = "--external-ref" ]; then ref="$a"; fi; prev="$a"; done
  found=$(grep "^$ref	" "` + db + `" 2>/dev/null | head -1)
  if [ -z "$found" ]; then echo "[]"; else
    id=$(echo "$found" | cut -f2); st=$(echo "$found" | cut -f3)
    echo "[{\"id\":\"$id\",\"title\":\"t\",\"status\":\"$st\",\"external_ref\":\"$ref\"}]"
  fi ;;
*" create "*)
  ref=""; prev=""
  for a in "$@"; do if [ "$prev" = "--external-ref" ]; then ref="$a"; fi; prev="$a"; done
  n=$(( $(wc -l < "` + db + `" 2>/dev/null || echo 0) + 1 ))
  printf '%s\tbt-%s\topen\n' "$ref" "$n" >> "` + db + `"
  echo "{\"id\":\"bt-$n\"}" ;;
esac
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bd"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return func() []string {
		b, _ := os.ReadFile(log)
		return strings.Split(strings.TrimSpace(string(b)), "\n")
	}
}

// The import is idempotent: the second call finds the ticket the first one
// created and creates nothing.
func TestImportFromTrackerCreatesOnce(t *testing.T) {
	calls := fakeBd(t)
	id, created, err := ImportFromTracker("/p", ProviderGitLab, "623", "Export CSV", "From the tracker")
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, "bt-1", id)

	again, created, err := ImportFromTracker("/p", ProviderGitLab, "623", "Export CSV", "From the tracker")
	require.NoError(t, err)
	assert.False(t, created, "a ticket already linked to the issue is reused")
	assert.Equal(t, id, again)

	other, created, err := ImportFromTracker("/p", ProviderGitLab, "624", "Another", "")
	require.NoError(t, err)
	assert.True(t, created)
	assert.NotEqual(t, id, other, "another issue, another ticket")

	var creates int
	for _, c := range calls() {
		if strings.Contains(c, " create ") {
			creates++
		}
	}
	assert.Equal(t, 2, creates)
	assert.Contains(t, calls()[1], `create [gitlab-623] Export CSV --external-ref gitlab-623 --description=From the tracker --json`)
}

func TestImportFromTrackerNeedsAReference(t *testing.T) {
	fakeBd(t)
	_, _, err := ImportFromTracker("/p", "", "623", "x", "")
	require.Error(t, err)
	_, _, err = ImportFromTracker("/p", ProviderGitLab, "", "x", "")
	require.Error(t, err)
	_, err = FindByExternalRef("/p", "")
	require.Error(t, err)
}

// What the import asks of bd is in the table of the bd gateway: the options
// are known to the version oh is generated for (a refusal there would break
// the import on the day bd drops one).
func TestImportArgsAreInTheBdTable(t *testing.T) {
	t.Parallel()
	create, err := bdargs.Parse(importCreateArgs("/p", "gitlab-623", "Export CSV", "- starts with a dash"))
	require.NoError(t, err)
	assert.Equal(t, []string{"create"}, create.Path)
	assert.Equal(t, []string{"gitlab-623"}, create.Values("external-ref"))
	assert.Equal(t, []string{"- starts with a dash"}, create.Values("description"))

	list, err := bdargs.Parse([]string{"-C", "/p", "--readonly", "list", "--external-ref", "gitlab-623", "--all", "--json", "--flat", "-n", "0"})
	require.NoError(t, err)
	assert.Equal(t, []string{"list"}, list.Path)
	assert.Equal(t, []string{"gitlab-623"}, list.Values("external-ref"))
}

// Contract with the real bd: the filter is exact, the import creates one
// ticket for an issue however many times it is asked, closed ones included.
func TestImportFromTrackerRealBd(t *testing.T) {
	bdtest.RequireRealBd(t)
	patientBd(t)
	dir := bdtest.InitedRepo(t)
	t.Setenv("BEADS_DIR", "")

	id, created, err := ImportFromTracker(dir, ProviderGitLab, "623", "Export CSV", "Imported from the tracker.")
	require.NoError(t, err)
	assert.True(t, created)
	assert.NotEmpty(t, id)

	again, created, err := ImportFromTracker(dir, ProviderGitLab, "623", "Export CSV", "")
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, id, again)

	longer, created, err := ImportFromTracker(dir, ProviderGitLab, "62", "Another issue", "")
	require.NoError(t, err)
	assert.True(t, created, "gitlab-62 is not gitlab-623")
	assert.NotEqual(t, id, longer)

	found, err := FindByExternalRef(dir, "gitlab-623")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "[gitlab-623] Export CSV", found.Title)
	assert.Equal(t, "gitlab-623", found.ExternalRef)
	none, err := FindByExternalRef(dir, "gitlab-9")
	require.NoError(t, err)
	assert.Nil(t, none)
}
