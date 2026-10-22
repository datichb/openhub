package adapters

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTool is an adapter that only answers Detect.
type fakeTool struct {
	ToolAdapter
	name string
	info ToolInfo
	err  error
}

func (f *fakeTool) Name() string { return f.name }

func (f *fakeTool) Detect(context.Context) (ToolInfo, error) { return f.info, f.err }

func factoryOf(a ToolAdapter, built *atomic.Int32) Factory {
	return func(string) ToolAdapter {
		if built != nil {
			built.Add(1)
		}
		return a
	}
}

// An adapter is built the first time it is asked for, once, with the cache
// folder of the registry; a former name still finds it.
func TestRegistryGetBuildsOnceAndFollowsAliases(t *testing.T) {
	var built atomic.Int32
	var cacheSeen string
	tool := &fakeTool{name: "alpha"}
	r := &Registry{CacheDir: "/cache"}
	r.Register("alpha", func(dir string) ToolAdapter { cacheSeen = dir; built.Add(1); return tool }, "alpha-v1", "alpha-old")

	got, ok := r.Get("alpha")
	require.True(t, ok)
	assert.Same(t, tool, got)
	assert.Equal(t, "/cache", cacheSeen)

	for _, name := range []string{"alpha", "alpha-v1", "alpha-old"} {
		again, ok := r.Get(name)
		require.True(t, ok, name)
		assert.Same(t, tool, again, name)
	}
	assert.EqualValues(t, 1, built.Load(), "built once, whatever the name")

	_, ok = r.Get("beta")
	assert.False(t, ok)
	var empty Registry
	_, ok = empty.Get("alpha")
	assert.False(t, ok, "an empty registry knows nothing")
}

func TestRegistryNamesKeepTheOrderOfPreference(t *testing.T) {
	r := &Registry{}
	assert.Empty(t, r.Names())
	r.Register("b", factoryOf(&fakeTool{name: "b"}, nil))
	r.Register("a", factoryOf(&fakeTool{name: "a"}, nil))
	assert.Equal(t, []string{"b", "a"}, r.Names())

	names := r.Names()
	names[0] = "changed"
	assert.Equal(t, []string{"b", "a"}, r.Names(), "the caller gets a copy")
}

// Detect returns the first adapter whose tool is installed and supported;
// those before it are skipped.
func TestRegistryDetectPicksTheFirstUsableAdapter(t *testing.T) {
	missing := &fakeTool{name: "first", info: ToolInfo{Name: "first"}, err: ErrToolNotInstalled}
	good := &fakeTool{name: "second", info: ToolInfo{Name: "second", Version: "2.1.0"}}
	other := &fakeTool{name: "third", info: ToolInfo{Name: "third"}}
	r := &Registry{}
	r.Register("first", factoryOf(missing, nil))
	r.Register("second", factoryOf(good, nil))
	r.Register("third", factoryOf(other, nil))

	a, info, err := r.Detect(context.Background())
	require.NoError(t, err)
	assert.Same(t, good, a)
	assert.Equal(t, "2.1.0", info.Version)
}

// When no tool is usable, the answer is the one of the preferred adapter: its
// names, so that the message can say which tool is missing, and its error.
func TestRegistryDetectReportsThePreferredAdaptersError(t *testing.T) {
	unsupported := &UnsupportedVersionError{Tool: "Alpha", Found: "9.0.0", Min: "2.0.0", Max: "2.9.0"}
	first := &fakeTool{name: "first", info: ToolInfo{Name: "first", DisplayName: "Alpha", Version: "9.0.0"}, err: unsupported}
	second := &fakeTool{name: "second", info: ToolInfo{Name: "second"}, err: ErrToolNotInstalled}
	r := &Registry{}
	r.Register("first", factoryOf(first, nil))
	r.Register("second", factoryOf(second, nil))

	a, info, err := r.Detect(context.Background())
	assert.Nil(t, a)
	var ue *UnsupportedVersionError
	require.ErrorAs(t, err, &ue)
	assert.Equal(t, "Alpha", info.DisplayName)
	assert.Equal(t, "Alpha 9.0.0 is not supported (requires 2.0.0 to 2.9.0)", err.Error())

	empty := &Registry{}
	a, _, err = empty.Detect(context.Background())
	assert.Nil(t, a)
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrToolNotInstalled), "no adapter at all is not a missing tool")
}

// A leftover plan is empty when nothing is to be removed; its diff and apply
// work without a function behind them.
func TestLegacyPlan(t *testing.T) {
	assert.True(t, (&LegacyPlan{}).Empty())
	assert.False(t, (&LegacyPlan{}).Leftovers())
	for name, p := range map[string]*LegacyPlan{
		"a file":          {Items: []LegacyItem{{Rel: ".legacy-dir", Dir: true, Count: 3}}},
		"a config key":    {Removed: []string{"agent.coder"}},
		"the config file": {DeleteConfig: true},
	} {
		assert.False(t, p.Empty(), name)
		assert.True(t, p.Leftovers(), name)
	}
	assert.False(t, (&LegacyPlan{Kept: []string{"model"}, ConfigUntouched: "unreadable"}).Leftovers(), "kept keys are not leftovers")

	p := &LegacyPlan{}
	assert.Empty(t, p.Diff())
	assert.NoError(t, p.Apply())

	applied := 0
	p = &LegacyPlan{DiffFunc: func() string { return "-a\n+b" }, ApplyFunc: func() error { applied++; return errors.New("disk full") }}
	assert.Equal(t, "-a\n+b", p.Diff())
	assert.EqualError(t, p.Apply(), "disk full")
	assert.Equal(t, 1, applied)
}
