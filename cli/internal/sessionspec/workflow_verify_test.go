package sessionspec

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathMatch(t *testing.T) {
	for _, c := range []struct {
		pattern, name string
		want          bool
	}{
		{"*_test.go", "pkg/calc/calc_test.go", true},
		{"*_test.go", "calc.go", false},
		{"docs/specs/**", "docs/specs/a/b.md", true},
		{"docs/specs/**", "docs/other.md", false},
		{"**/migrations/*.sql", "db/migrations/001.sql", true},
		{"**/migrations/*.sql", "migrations/001.sql", true},
		{"src/*.ts", "src/a/b.ts", false},
		{"calc.go", "calc.go", true},
	} {
		assert.Equal(t, c.want, PathMatch(c.pattern, c.name), "%s ~ %s", c.pattern, c.name)
	}
}

// CP-R7: when: only adds a pause.
func TestEffectiveBehavior(t *testing.T) {
	when := &WhenDef{Labels: []string{"ux"}}
	yes, no := WhenFacts{Labels: []string{"UX"}}, WhenFacts{}
	auto := CheckpointDef{Behaviors: map[string]string{"m": CheckpointAuto}, When: when}
	assert.Equal(t, CheckpointPause, auto.EffectiveBehavior("m", yes))
	assert.Equal(t, CheckpointAuto, auto.EffectiveBehavior("m", no))
	cond := CheckpointDef{Behaviors: map[string]string{"m": CheckpointConditional}, When: when}
	assert.Equal(t, CheckpointPause, cond.EffectiveBehavior("m", yes))
	assert.Equal(t, CheckpointAuto, cond.EffectiveBehavior("m", no), "evaluated by oh: no pause when false")
	noWhen := CheckpointDef{Behaviors: map[string]string{"m": CheckpointConditional}}
	assert.Equal(t, CheckpointConditional, noWhen.EffectiveBehavior("m", no), "not evaluable: it asks")
	mandatory := CheckpointDef{Behaviors: map[string]string{"m": CheckpointConditional}, When: when, Mandatory: true}
	assert.Equal(t, CheckpointConditional, mandatory.EffectiveBehavior("m", no), "a mandatory checkpoint keeps its pause")
	unlocking := CheckpointDef{Behaviors: map[string]string{"m": CheckpointConditional}, When: when, Unlocks: []string{UnlockCommit}}
	assert.Equal(t, CheckpointConditional, unlocking.EffectiveBehavior("m", no))
	pause := CheckpointDef{Behaviors: map[string]string{"m": CheckpointPause}, When: when}
	assert.Equal(t, CheckpointPause, pause.EffectiveBehavior("m", no), "never removes a pause")
	skip := CheckpointDef{Behaviors: map[string]string{"m": CheckpointSkip}, When: when}
	assert.Equal(t, CheckpointSkip, skip.EffectiveBehavior("m", yes))

	ok, why := (&WhenDef{Entry: []string{"debugger"}, Paths: []string{"*.sql"}, Outputs: []string{"mr"}}).Holds(WhenFacts{Paths: []string{"db/1.sql"}})
	assert.True(t, ok)
	assert.Equal(t, "path: db/1.sql", why)
}
