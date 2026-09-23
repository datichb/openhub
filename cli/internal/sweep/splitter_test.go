package sweep

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/datichb/openhub/cli/internal/llm"
	"github.com/datichb/openhub/cli/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock LLM Completer ---

type mockCompleter struct {
	responses []string // responses to return in order
	calls     int      // track how many times Complete was called
	err       error    // optional error to return
}

func (m *mockCompleter) Complete(_ context.Context, req llm.Request) (*llm.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	idx := m.calls
	m.calls++
	if idx < len(m.responses) {
		return &llm.Response{Content: m.responses[idx]}, nil
	}
	return nil, fmt.Errorf("mock completer: no more responses (call %d)", idx)
}

// --- Manual strategy tests ---

func TestManualSplit(t *testing.T) {
	s := NewSplitter(nil)
	tasks, err := s.Split(context.Background(), SplitOpts{
		Strategy:     StrategyManual,
		Goal:         "Add tests",
		ManualTasks:  []string{"pkg/auth", "pkg/api", "pkg/config"},
		BranchPrefix: "sweep/",
	})
	require.NoError(t, err)
	assert.Len(t, tasks, 3)

	// Verify first task
	assert.Equal(t, "sweep-pkg-auth", tasks[0].ID)
	assert.Equal(t, task.KindSweep, tasks[0].Kind)
	assert.Equal(t, "pkg/auth", tasks[0].Label)
	assert.Equal(t, "sweep/sweep-pkg-auth", tasks[0].BranchName)
	assert.Equal(t, "pkg/auth", tasks[0].Metadata["scope"])
	assert.Equal(t, "manual", tasks[0].Metadata["strategy"])
	assert.Equal(t, "Add tests", tasks[0].Metadata["goal"])

	// Verify all tasks are sweep kind and mergeable
	for _, tsk := range tasks {
		assert.Equal(t, task.KindSweep, tsk.Kind)
		assert.True(t, tsk.IsMergeable())
	}
}

func TestManualSplit_Empty(t *testing.T) {
	s := NewSplitter(nil)
	_, err := s.Split(context.Background(), SplitOpts{
		Strategy:    StrategyManual,
		Goal:        "Add tests",
		ManualTasks: []string{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--sweep-tasks is required")
}

func TestManualSplit_EmptyGoal(t *testing.T) {
	s := NewSplitter(nil)
	_, err := s.Split(context.Background(), SplitOpts{
		Strategy:    StrategyManual,
		Goal:        "",
		ManualTasks: []string{"a"},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "goal is required")
}

func TestManualSplit_WhitespaceItems(t *testing.T) {
	s := NewSplitter(nil)
	tasks, err := s.Split(context.Background(), SplitOpts{
		Strategy:     StrategyManual,
		Goal:         "Refactor",
		ManualTasks:  []string{"  pkg/auth  ", "", "  pkg/api  "},
		BranchPrefix: "sweep/",
	})
	require.NoError(t, err)
	assert.Len(t, tasks, 2) // empty item filtered out
	assert.Equal(t, "sweep-pkg-auth", tasks[0].ID)
	assert.Equal(t, "sweep-pkg-api", tasks[1].ID)
}

// --- By-file strategy tests ---

func TestByFileSplit(t *testing.T) {
	// Create temp directory with test files
	tmpDir := t.TempDir()
	for _, name := range []string{"a.go", "b.go", "c.go", "d.go", "e.txt"} {
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, name), []byte("package test"), 0o644))
	}

	s := NewSplitter(nil)
	tasks, err := s.Split(context.Background(), SplitOpts{
		Strategy:     StrategyByFile,
		Goal:         "Lint fix",
		IncludeGlobs: []string{"*.go"},
		MaxTasks:     2,
		ProjectPath:  tmpDir,
		BranchPrefix: "sweep/",
	})
	require.NoError(t, err)
	assert.Len(t, tasks, 2) // 4 files / 2 batches = 2 tasks

	for _, tsk := range tasks {
		assert.Equal(t, task.KindSweep, tsk.Kind)
		assert.NotEmpty(t, tsk.Metadata["scope"])
		assert.Equal(t, "by-file", tsk.Metadata["strategy"])
	}
}

func TestByFileSplit_NoInclude(t *testing.T) {
	s := NewSplitter(nil)
	_, err := s.Split(context.Background(), SplitOpts{
		Strategy:    StrategyByFile,
		Goal:        "Lint",
		ProjectPath: t.TempDir(),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--sweep-include is required")
}

func TestByFileSplit_ExcludeGlobs(t *testing.T) {
	tmpDir := t.TempDir()
	for _, name := range []string{"main.go", "main_test.go", "util.go", "util_test.go"} {
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, name), []byte("package test"), 0o644))
	}

	s := NewSplitter(nil)
	tasks, err := s.Split(context.Background(), SplitOpts{
		Strategy:     StrategyByFile,
		Goal:         "Add docs",
		IncludeGlobs: []string{"*.go"},
		ExcludeGlobs: []string{"*_test.go"},
		MaxTasks:     10,
		ProjectPath:  tmpDir,
		BranchPrefix: "sweep/",
	})
	require.NoError(t, err)
	// Only main.go and util.go should be included
	totalFiles := 0
	for _, tsk := range tasks {
		files := len(tsk.Metadata["scope"])
		if files > 0 {
			totalFiles++
		}
	}
	assert.GreaterOrEqual(t, totalFiles, 1)
	// Verify no test files in scope
	for _, tsk := range tasks {
		assert.NotContains(t, tsk.Metadata["scope"], "_test.go")
	}
}

func TestByFileSplit_NoMatches(t *testing.T) {
	s := NewSplitter(nil)
	_, err := s.Split(context.Background(), SplitOpts{
		Strategy:     StrategyByFile,
		Goal:         "Fix",
		IncludeGlobs: []string{"*.xyz"},
		ProjectPath:  t.TempDir(),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no files matched")
}

// --- LLM strategy tests ---

func TestLLMSplit(t *testing.T) {
	mock := &mockCompleter{
		responses: []string{`{
			"tasks": [
				{"id": "sweep-auth", "label": "Auth package", "scope": "pkg/auth", "description": "Fix auth"},
				{"id": "sweep-api", "label": "API package", "scope": "pkg/api", "description": "Fix api"}
			]
		}`},
	}

	s := NewSplitter(mock)
	tasks, err := s.Split(context.Background(), SplitOpts{
		Strategy:     StrategyLLM,
		Goal:         "Fix all the things",
		MaxTasks:     10,
		ProjectPath:  "/tmp/test",
		BranchPrefix: "sweep/",
	})
	require.NoError(t, err)
	assert.Len(t, tasks, 2)
	assert.Equal(t, "sweep-auth", tasks[0].ID)
	assert.Equal(t, "Auth package", tasks[0].Label)
	assert.Equal(t, "pkg/auth", tasks[0].Metadata["scope"])
	assert.Equal(t, task.KindSweep, tasks[0].Kind)
	assert.Equal(t, 1, mock.calls)
}

func TestLLMSplit_WithCodeFences(t *testing.T) {
	mock := &mockCompleter{
		responses: []string{"```json\n" + `{"tasks": [{"id": "sweep-one", "label": "One", "scope": "src", "description": "Do stuff"}]}` + "\n```"},
	}

	s := NewSplitter(mock)
	tasks, err := s.Split(context.Background(), SplitOpts{
		Strategy:     StrategyLLM,
		Goal:         "Stuff",
		MaxTasks:     10,
		ProjectPath:  "/tmp/test",
		BranchPrefix: "sweep/",
	})
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "sweep-one", tasks[0].ID)
}

func TestLLMSplit_MalformedRetry(t *testing.T) {
	mock := &mockCompleter{
		responses: []string{
			"this is not json",                                                                       // first attempt: invalid
			`{"tasks": [{"id": "sweep-ok", "label": "OK", "scope": "src", "description": "Fixed"}]}`, // second attempt: valid
		},
	}

	s := NewSplitter(mock)
	tasks, err := s.Split(context.Background(), SplitOpts{
		Strategy:     StrategyLLM,
		Goal:         "Fix",
		MaxTasks:     10,
		ProjectPath:  "/tmp/test",
		BranchPrefix: "sweep/",
	})
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "sweep-ok", tasks[0].ID)
	assert.Equal(t, 2, mock.calls) // needed 2 attempts
}

func TestLLMSplit_AllRetriesFail(t *testing.T) {
	mock := &mockCompleter{
		responses: []string{
			"not json 1",
			"not json 2",
			"not json 3",
		},
	}

	s := NewSplitter(mock)
	_, err := s.Split(context.Background(), SplitOpts{
		Strategy:     StrategyLLM,
		Goal:         "Fix",
		MaxTasks:     10,
		ProjectPath:  "/tmp/test",
		BranchPrefix: "sweep/",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed after")
	assert.Equal(t, 3, mock.calls)
}

func TestLLMSplit_DuplicateIDs(t *testing.T) {
	mock := &mockCompleter{
		responses: []string{`{
			"tasks": [
				{"id": "sweep-same", "label": "A", "scope": "a", "description": "A"},
				{"id": "sweep-same", "label": "B", "scope": "b", "description": "B"}
			]
		}`},
	}

	s := NewSplitter(mock)
	_, err := s.Split(context.Background(), SplitOpts{
		Strategy:     StrategyLLM,
		Goal:         "Fix",
		MaxTasks:     10,
		ProjectPath:  "/tmp/test",
		BranchPrefix: "sweep/",
	})
	// First attempt fails due to duplicate, retries
	assert.Error(t, err)
}

func TestLLMSplit_NilCompleter(t *testing.T) {
	s := NewSplitter(nil)
	_, err := s.Split(context.Background(), SplitOpts{
		Strategy:     StrategyLLM,
		Goal:         "Fix",
		ProjectPath:  "/tmp",
		BranchPrefix: "sweep/",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LLM completer is required")
}

// --- MaxTasks cap test ---

func TestSplit_MaxTasksCap(t *testing.T) {
	s := NewSplitter(nil)
	tasks, err := s.Split(context.Background(), SplitOpts{
		Strategy:     StrategyManual,
		Goal:         "Test",
		ManualTasks:  []string{"a", "b", "c", "d", "e"},
		MaxTasks:     3,
		BranchPrefix: "sweep/",
	})
	require.NoError(t, err)
	assert.Len(t, tasks, 3)
}

// --- Unknown strategy test ---

func TestSplit_UnknownStrategy(t *testing.T) {
	s := NewSplitter(nil)
	_, err := s.Split(context.Background(), SplitOpts{
		Strategy: Strategy("nope"),
		Goal:     "Test",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown sweep strategy")
}

// --- Config tests ---

func TestDefaultSweepConfig(t *testing.T) {
	cfg := DefaultSweepConfig()
	assert.Equal(t, 10, cfg.MaxSplits)
	assert.Equal(t, "sweep/", cfg.BranchPrefix)
	assert.Equal(t, "none", cfg.VerifyStrategy)
	assert.Empty(t, cfg.VerifyCmd)
}

func TestIsValidStrategy(t *testing.T) {
	assert.True(t, IsValidStrategy(StrategyManual))
	assert.True(t, IsValidStrategy(StrategyByFile))
	assert.True(t, IsValidStrategy(StrategyByPackage))
	assert.True(t, IsValidStrategy(StrategyLLM))
	assert.False(t, IsValidStrategy(Strategy("invalid")))
}

func TestIsValidVerifyStrategy(t *testing.T) {
	assert.True(t, IsValidVerifyStrategy(VerifyNone))
	assert.True(t, IsValidVerifyStrategy(VerifyTests))
	assert.True(t, IsValidVerifyStrategy(VerifyBuild))
	assert.True(t, IsValidVerifyStrategy(VerifyLint))
	assert.True(t, IsValidVerifyStrategy(VerifyAll))
	assert.True(t, IsValidVerifyStrategy(VerifyCustom))
	assert.False(t, IsValidVerifyStrategy(VerifyStrategy("bogus")))
}

// --- Slugify tests ---

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"sweep-pkg/auth", "sweep-pkg-auth"},
		{"sweep_some_thing", "sweep-some-thing"},
		{"SWEEP-UPPER", "sweep-upper"},
		{"sweep--double--dash", "sweep-double-dash"},
		{"sweep/path/to/pkg", "sweep-path-to-pkg"},
		{"  spaces  ", "spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, slugify(tt.input))
		})
	}
}

// --- Prompt tests ---

func TestBuildSweepPrompt(t *testing.T) {
	tsk := task.Task{
		ID:          "sweep-auth",
		Kind:        task.KindSweep,
		Label:       "Auth tests",
		Description: "Add unit tests for the auth package",
		Metadata: map[string]string{
			"scope":    "pkg/auth",
			"strategy": "by-package",
		},
	}

	prompt := BuildSweepPrompt(tsk, "Add missing tests")
	assert.Contains(t, prompt, "[SWEEP MODE]")
	assert.Contains(t, prompt, "Add missing tests")
	assert.Contains(t, prompt, "Add unit tests for the auth package")
	assert.Contains(t, prompt, "pkg/auth")
	assert.Contains(t, prompt, "by-package")
}

func TestBuildSweepPrompt_MinimalTask(t *testing.T) {
	tsk := task.Task{
		ID:   "sweep-simple",
		Kind: task.KindSweep,
	}

	prompt := BuildSweepPrompt(tsk, "Simple goal")
	assert.Contains(t, prompt, "[SWEEP MODE]")
	assert.Contains(t, prompt, "Simple goal")
	assert.NotContains(t, prompt, "Ta sous-tâche")
	assert.NotContains(t, prompt, "Scope")
}
