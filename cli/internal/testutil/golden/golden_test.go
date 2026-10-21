package golden

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recorder collects the failures of Assert instead of failing the real test.
type recorder struct {
	testing.TB
	errors []string
}

func (r *recorder) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func TestAssert(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.MkdirAll("testdata/sub", 0o755), "creating testdata")
	require.NoError(t, os.WriteFile("testdata/page.golden", []byte("one\ntwo\nthree\n"), 0o644), "writing the golden file")
	require.NoError(t, os.WriteFile("testdata/sub/data.golden", []byte{'{', '}', '\n'}, 0o644), "writing the golden file of a folder")

	check := func(name string, got any) *recorder {
		r := &recorder{TB: t}
		switch v := got.(type) {
		case string:
			Assert(r, name, v)
		case []byte:
			Assert(r, name, v)
		}
		return r
	}

	assert.Empty(t, check("page", "one\ntwo\nthree\n").errors, "same string: no failure")
	assert.Empty(t, check("sub/data", []byte("{}\n")).errors, "same bytes, name with a folder: no failure")

	r := check("page", "one\n2\nthree\n")
	require.Len(t, r.errors, 1, "different output: one failure")
	assert.Contains(t, r.errors[0], "testdata/page.golden", "the failure names the golden file")
	assert.Contains(t, r.errors[0], "-two", "the diff shows the expected line")
	assert.Contains(t, r.errors[0], "+2", "the diff shows the line obtained")
	assert.Contains(t, r.errors[0], "-run '^TestAssert$' -update", "the failure tells how to regenerate")
	kept, err := os.ReadFile("testdata/page.golden")
	require.NoError(t, err, "reading the golden file")
	assert.Equal(t, "one\ntwo\nthree\n", string(kept), "a failure does not rewrite the golden file")

	r = check("absent", "x")
	require.Len(t, r.errors, 1, "missing golden file: one failure")
	assert.Contains(t, r.errors[0], "testdata/absent.golden", "the failure names the missing file")
	assert.Contains(t, r.errors[0], "-update", "the failure tells how to create it")

	r = check("../escape", "x")
	require.Len(t, r.errors, 1, "a name outside testdata is refused")

	assert.False(t, Updating(), "a normal run does not update")
	*update = true
	t.Cleanup(func() { *update = false })
	assert.True(t, Updating(), "the flag -update is reported")
	assert.Empty(t, check("page", "rewritten\n").errors, "update: no failure")
	assert.Empty(t, check("new/deep", []byte("created")).errors, "update: no failure for a new file")
	*update = false
	got, err := os.ReadFile(filepath.Join("testdata", "page.golden"))
	require.NoError(t, err, "reading the rewritten file")
	assert.Equal(t, "rewritten\n", string(got), "update rewrites the golden file")
	got, err = os.ReadFile(filepath.Join("testdata", "new", "deep.golden"))
	require.NoError(t, err, "reading the new file")
	assert.Equal(t, "created", string(got), "update creates the file and its folders")
	assert.Empty(t, check("page", "rewritten\n").errors, "the rewritten file now matches")
}
