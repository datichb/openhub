package sessionspec

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModelRefRoundTrip(t *testing.T) {
	cases := []string{
		"amazon-bedrock/eu.anthropic.claude-haiku-4-5-20251001-v1:0",
		"anthropic/claude-sonnet-4-5#high",
	}
	for _, c := range cases {
		assert.Equal(t, c, ParseModelRef(c).String())
	}
	bare := ParseModelRef("claude-sonnet-4-5")
	assert.Equal(t, "", bare.Provider)
	assert.Equal(t, "claude-sonnet-4-5", bare.Model)
	assert.Equal(t, "", ModelRef{}.String())
}

func TestNewSessionID(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		id := NewSessionID()
		assert.True(t, strings.HasPrefix(id, "ses_"))
		assert.Len(t, id, 30)
		assert.False(t, seen[id])
		seen[id] = true
	}
}

func TestGroupKeyString(t *testing.T) {
	g := GroupKey{BundleHash: "0123456789abcdef0123", ProjectID: "my proj/x", Runtime: RuntimeLocal}
	assert.Equal(t, "my_proj_x-0123456789ab-local", g.String())
	assert.Equal(t, "none--container", GroupKey{Runtime: RuntimeContainer}.String())
	g.Config = "fedcba9876543210"
	assert.Equal(t, "my_proj_x-0123456789ab-fedcba9876-local", g.String())
}

func TestBundleIDs(t *testing.T) {
	b := BundleSpec{
		Agents: []AgentDef{{ID: "a"}, {ID: "b"}},
		Skills: []SkillDef{{ID: "s"}},
	}
	assert.Equal(t, []string{"a", "b"}, b.AgentIDs())
	assert.Equal(t, []string{"s"}, b.SkillIDs())
}

func TestWithBundleRoot(t *testing.T) {
	b := BundleSpec{Root: "/r", Agents: []AgentDef{{ID: "a", Body: "read " + BundleRootVar + "/skills/x/t.md and " + BundleRootVar + "/y"}}}
	got := b.WithBundleRoot(b.Root)
	assert.Equal(t, "read /r/skills/x/t.md and /r/y", got.Agents[0].Body)
	assert.Contains(t, b.Agents[0].Body, BundleRootVar, "the original is not modified")
	assert.Equal(t, got, got.WithBundleRoot("/other"), "idempotent once expanded")
	assert.Equal(t, b, b.WithBundleRoot(""))
}
