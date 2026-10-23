package doctor

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

func results(rs ...Result) func(context.Context) []Result {
	return func(context.Context) []Result { return rs }
}

func TestNewRegistryRefusesMistakes(t *testing.T) {
	run := results(Result{OK: true})
	_, err := NewRegistry(Check{ID: "", Name: "x", Run: run})
	assert.Error(t, err, "no ID")
	_, err = NewRegistry(Check{ID: "a", Run: run}, Check{ID: "a", Run: run})
	assert.Error(t, err, "repeated ID")
	_, err = NewRegistry(Check{ID: "a"})
	assert.Error(t, err, "no Run")
	reg, err := NewRegistry(Check{ID: "a", Run: run}, Check{ID: "b", Run: run})
	require.NoError(t, err)
	assert.Len(t, reg.Checks(), 2)
	_, ok := reg.Get("b")
	assert.True(t, ok)
	_, ok = reg.Get("c")
	assert.False(t, ok)
}

func TestCheckResultsCompletesTheResults(t *testing.T) {
	fix := &Fix{Open: func() {}}
	c := Check{
		ID: "db", Name: "Database",
		Run: results(
			Result{Detail: "ok", OK: true},
			Result{ID: "db.other", Name: "Other", Detail: "boom", Optional: true},
			Result{Detail: "broken"},
		),
		Fix: func(r Result) *Fix {
			if r.OK {
				return nil
			}
			return fix
		},
	}
	got := c.Results(context.Background())
	require.Len(t, got, 3)

	assert.Equal(t, "Database", got[0].Name, "the result takes the name of its check")
	assert.Equal(t, "db", got[0].ID, "and its identifier")
	assert.Nil(t, got[0].Fix, "a passed result has no repair")

	assert.Equal(t, "Other", got[1].Name, "an own name is kept")
	assert.Equal(t, "db.other", got[1].ID, "so is an own identifier")
	assert.True(t, got[1].OK && got[1].Warn, "an optional failure is a warning")
	assert.Nil(t, got[1].Fix, "which is not repaired")

	assert.False(t, got[2].OK)
	assert.Same(t, fix, got[2].Fix, "a failed result carries the repair of its check")
}

func TestRegistryRunKeepsTheOrderAndFixFor(t *testing.T) {
	fix := &Fix{Plan: func(context.Context) (string, error) { return "plan", nil }}
	reg, err := NewRegistry(
		Check{ID: "a", Name: "A", Run: results(Result{OK: true}, Result{Name: "A2", OK: true})},
		Check{ID: "b", Name: "B", Run: results(Result{}), Fix: func(Result) *Fix { return fix }},
	)
	require.NoError(t, err)
	got := reg.Run(context.Background())
	var names []string
	for _, r := range got {
		names = append(names, r.Name)
	}
	assert.Equal(t, []string{"A", "A2", "B"}, names, "registry order, then the order of each check")

	assert.Same(t, fix, got[2].Fix)
	assert.Same(t, fix, reg.FixFor(Result{ID: "b"}), "a result built apart gets the repair of its check")
	assert.Nil(t, reg.FixFor(Result{ID: "a"}))
	assert.Nil(t, reg.FixFor(Result{ID: "zzz"}))
}

func TestCheckRunsWithTheContextItIsGiven(t *testing.T) {
	type key struct{}
	var seen any
	c := Check{ID: "k", Run: func(ctx context.Context) []Result {
		seen = ctx.Value(key{})
		return nil
	}}
	c.Results(context.WithValue(context.Background(), key{}, "from the caller"))
	assert.Equal(t, "from the caller", seen)
}

// A migrated check end to end: registry → check → program of the PATH →
// result with a name, an identifier and no repair.
func TestBinaryCheckEndToEnd(t *testing.T) {
	i18ntest.Set(t, "en")
	if runtime.GOOS == "windows" {
		t.Skip("the fake program is a shell script")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\necho 'fakebd 1.2.3'\necho 'second line'\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fakebd"), []byte(script), 0o755))
	t.Setenv("PATH", dir)

	reg, err := NewRegistry(
		OS(), GoRuntime(),
		Binary("fakebd", "fakebd"),
		Binary("missing", "no-such-program"),
		OptionalBinary("opt", "no-such-program", "optional tool", "brew install tool"),
		OptionalBinary("optfound", "fakebd", "optional fake", "unused"),
	)
	require.NoError(t, err)
	got := reg.Run(context.Background())
	require.Len(t, got, 6)

	assert.Equal(t, runtime.GOOS+"/"+runtime.GOARCH, got[0].Detail)
	assert.Equal(t, runtime.Version(), got[1].Detail)

	assert.Equal(t, Result{ID: "fakebd", Name: "fakebd", Detail: "fakebd 1.2.3", OK: true}, got[2], "first line of the version")

	assert.False(t, got[3].OK, "a required program that is missing fails")
	assert.Equal(t, "not found in PATH", got[3].Detail)

	assert.True(t, got[4].OK, "an optional program that is missing passes")
	assert.Contains(t, got[4].Detail, "brew install tool")
	assert.Equal(t, "optional tool", got[4].Name)

	assert.Equal(t, "fakebd 1.2.3", got[5].Detail)
}
