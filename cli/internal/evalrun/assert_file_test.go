package evalrun

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

// R4: file_matches reads the files changed in the clone (the spec written
// under docs/wiki/specs/); it needs the clone, so Rescore keeps its result.
func TestFileMatches(t *testing.T) {
	files := map[string]string{
		"docs/wiki/specs/percent.md": "## Glossaire\n- **Part**\n## Critères d'acceptation\n- CA-1 total zéro · Test : TestPercent_Zero\n",
		"calc.go":                    "package calc\n",
	}
	ev := Evidence{Changed: []string{"calc.go", "docs/wiki/specs/percent.md", "docs/wiki/specs/gone.md"}, ReadFile: func(rel string) ([]byte, error) {
		if s, ok := files[rel]; ok {
			return []byte(s), nil
		}
		return nil, errors.New("not found")
	}}
	specs := []string{"docs/wiki/specs/**"}
	r := one(t, Assertion{Type: AssertFileMatches, Paths: specs, Pattern: `(?m)^[^\n]*z[ée]ro[^\n]*Test[A-Z]\w*`}, ev)
	assert.True(t, r.Pass, r.Detail)
	assert.Equal(t, "docs/wiki/specs/percent.md", r.Detail)
	r = one(t, Assertion{Type: AssertFileMatches, Paths: specs, Pattern: "plafon"}, ev)
	assert.False(t, r.Pass)
	assert.Equal(t, "not found in docs/wiki/specs/percent.md", r.Detail)
	r = one(t, Assertion{Type: AssertFileMatches, Paths: []string{"docs/adr/**"}, Pattern: "x"}, ev)
	assert.Equal(t, "no changed file matches docs/adr/**", r.Detail)
	assert.True(t, one(t, Assertion{Type: AssertFileMatches, Paths: specs, Pattern: "package", Absent: true}, ev).Pass, "calc.go is not a spec")
	assert.False(t, one(t, Assertion{Type: AssertFileMatches, Paths: specs, Pattern: "x"}, Evidence{}).Pass, "no clone")
	assert.Equal(t, "docs/wiki/specs/** holds /x/", Assertion{Type: AssertFileMatches, Paths: specs, Pattern: "x"}.Label())

	require.Error(t, Assertion{Type: AssertFileMatches, Pattern: "x"}.Validate(), "paths")
	require.Error(t, Assertion{Type: AssertFileMatches, Paths: specs}.Validate(), "pattern")
	assert.True(t, cloneAssertion(AssertFileMatches))
}

// R4: answers.sequence answers the first asks of a checkpoint in order,
// then answers.checkpoints applies.
func TestAnswerSequence(t *testing.T) {
	a := Answers{Checkpoints: map[string]string{"cp-spec": "approve"}, Sequence: map[string][]string{"cp-spec": {"reject: ajoute une règle"}}}
	d := domain.Decision{Kind: domain.DecisionCheckpoint, Payload: domain.DecisionPayload{Data: map[string]any{"checkpoint": "cp-spec"}}}
	assert.Equal(t, Answer{Decision: "reject", Message: "ajoute une règle"}, a.DecideAt(d, 0))
	assert.Equal(t, Answer{Decision: "approve"}, a.DecideAt(d, 1))
	assert.Equal(t, Answer{Decision: "approve"}, a.Decide(d), "no count: the checkpoints answer")
}
