package bricks

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

// R7: the test files of a stack and the reason a test written first fails.

func TestTestFilePatterns(t *testing.T) {
	g := TestFilePatterns(Signals{"golang": {"."}})
	assert.Contains(t, g, "*_test.go")
	assert.Contains(t, g, "**/tests/**", "usual test folders")
	assert.NotContains(t, g, "test_*.py", "only the stacks of the project")
	assert.True(t, slices.IsSorted(g))

	js := TestFilePatterns(Signals{"javascript": {"."}, "typescript": {"."}})
	assert.Contains(t, js, "*.test.*")
	assert.Contains(t, js, "*.spec.*")
	assert.Contains(t, js, "**/__tests__/**")

	none := TestFilePatterns(Signals{})
	for _, p := range []string{"*_test.go", "test_*.py", "*.spec.*", "**/src/test/**"} {
		assert.Contains(t, none, p, "no signal: every stack")
	}
}

func TestRedFailure(t *testing.T) {
	cases := []struct {
		name string
		args []string
		exit int
		out  string
		want string
	}{
		{"pass", []string{"go", "test", "./..."}, 0, "ok  x", ""},
		{"go assertion", []string{"go", "test", "./..."}, 1, "--- FAIL: TestAdd\nFAIL\tx/calc\t0.01s\n", ""},
		{"go build failed", []string{"go", "test", "./..."}, 1, "# x/calc [x/calc.test]\n./calc_test.go:5:9: undefined: Add\nFAIL\tx/calc [build failed]\n", RedCompile},
		{"go setup failed", []string{"go", "test", "./..."}, 1, "FAIL\tx/calc [setup failed]\n", RedCompile},
		{"make wraps go", []string{"make", "test"}, 2, "FAIL\tx/calc [build failed]\nmake: *** [test] Error 1\n", RedCompile},
		{"cargo", []string{"cargo", "test"}, 101, "error[E0425]: cannot find function `add`\nerror: could not compile `calc`\n", RedCompile},
		{"tsc", []string{"npm", "test"}, 2, "src/calc.test.ts(3,10): error TS2305: Module has no exported member 'add'.\n", RedCompile},
		{"pytest failed", []string{"python3", "-m", "pytest"}, 1, "FAILED tests/test_calc.py::test_add\n", ""},
		{"pytest collection", []string{"uv", "run", "--no-sync", "pytest"}, 2, "ERROR collecting tests/test_calc.py\nImportError\n", RedCollect},
		{"pytest none", []string{".venv/bin/pytest"}, 5, "no tests ran\n", RedNoTests},
		{"pytest behind make", []string{"make", "test"}, 2, "ERROR collecting tests/test_calc.py\n", RedCollect},
		{"jest", []string{"npm", "test"}, 1, "Tests: 1 failed, 1 total\n", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, RedFailure(c.args, c.exit, []byte(c.out)))
		})
	}
}
