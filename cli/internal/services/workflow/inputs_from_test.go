package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// QB2: inputs declared with `from:` are computed at launch by the service
// (oh run review-feedback reads the MR discussions itself, not only the
// former alias): a given value wins, then the computed one, then the default.
func TestComputeInputs(t *testing.T) {
	sp := specOf(t, `apiVersion: oh/v1
kind: Workflow
id: x
risk: write
inputs:
  mr: { type: string, required: true }
  branch: { type: branch, required: true, from: "gitlab.mr_source_branch(mr)" }
  base: { type: branch, default: main, from: "gitlab.mr_target_branch(mr)" }
  feedback: { type: text, required: true, from: "gitlab.mr_discussions(mr)" }
`)
	var calls []string
	src := func(name, out string, err error) InputSource {
		return func(_ context.Context, c Context, arg string) (string, error) {
			calls = append(calls, name+"("+arg+")@"+c.ProjectID)
			return out, err
		}
	}
	svc := &Service{InputSources: map[string]InputSource{
		"gitlab.mr_source_branch": src("source", "feat/x", nil),
		"gitlab.mr_target_branch": src("target", "develop", nil),
		"gitlab.mr_discussions":   src("discussions", "--- Discussion 1 ---", nil),
	}}
	values := map[string]any{"mr": "!12", "branch": "given"}
	require.NoError(t, svc.ComputeInputs(context.Background(), Context{ProjectID: "p"}, sp, values))
	assert.Equal(t, map[string]any{"mr": "!12", "branch": "given", "base": "develop", "feedback": "--- Discussion 1 ---"}, values)
	assert.Equal(t, []string{"target(!12)@p", "discussions(!12)@p"}, calls, "a given value is not computed")

	// Nothing to compute from: left to the required-input checks.
	values = map[string]any{}
	require.NoError(t, svc.ComputeInputs(context.Background(), Context{}, sp, values))
	assert.Empty(t, values)

	// A failing source: error without default, default kept otherwise.
	svc.InputSources["gitlab.mr_discussions"] = src("discussions", "", errors.New("no unresolved discussion"))
	err := svc.ComputeInputs(context.Background(), Context{}, sp, map[string]any{"mr": "!12"})
	assert.ErrorContains(t, err, "no unresolved discussion")
	svc.InputSources["gitlab.mr_discussions"] = src("discussions", "x", nil)
	svc.InputSources["gitlab.mr_target_branch"] = src("target", "", errors.New("down"))
	values = map[string]any{"mr": "!12"}
	require.NoError(t, svc.ComputeInputs(context.Background(), Context{}, sp, values))
	assert.NotContains(t, values, "base", "the default applies")

	// No source on this machine.
	err = (&Service{}).ComputeInputs(context.Background(), Context{}, sp, map[string]any{"mr": "!12"})
	assert.Error(t, err)
}
