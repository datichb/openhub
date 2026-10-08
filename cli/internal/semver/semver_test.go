package semver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	tests := []struct {
		input    string
		expected Version
	}{
		{"1.17.13", Version{1, 17, 13}},
		{"v2.0.0", Version{2, 0, 0}},
		{"0.42.0", Version{0, 42, 0}},
		{"1.15.0-beta", Version{1, 15, 0}},
		{"v0.1.2-rc1", Version{0, 1, 2}},
		{"3", Version{3, 0, 0}},
		{"2.1", Version{2, 1, 0}},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, Parse(tt.input), "input=%s", tt.input)
	}
}

func TestLessThan(t *testing.T) {
	assert.True(t, Version{1, 14, 9}.LessThan(Version{1, 15, 0}))
	assert.False(t, Version{1, 15, 0}.LessThan(Version{1, 15, 0}))
	assert.False(t, Version{1, 17, 13}.LessThan(Version{1, 15, 0}))
	assert.True(t, Version{0, 99, 99}.LessThan(Version{1, 0, 0}))
	assert.True(t, Version{1, 99, 99}.LessThan(Version{2, 0, 0}))
}

func TestAtLeast(t *testing.T) {
	assert.True(t, Version{1, 15, 0}.AtLeast(Version{1, 15, 0}))  // equal
	assert.True(t, Version{1, 17, 0}.AtLeast(Version{1, 15, 0}))  // greater
	assert.False(t, Version{1, 14, 9}.AtLeast(Version{1, 15, 0})) // less
}

func TestIsAtLeast(t *testing.T) {
	assert.True(t, IsAtLeast("0.42.0", "0.42.0"))
	assert.True(t, IsAtLeast("0.42.1", "0.42.0"))
	assert.False(t, IsAtLeast("0.41.9", "0.42.0"))
	assert.True(t, IsAtLeast("1.0.0", "0.42.0"))
}

func TestMajorMinor(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"2.0.0", "2.0"},
		{"2.0.1", "2.0"},
		{"1.17.13", "1.17"},
		{"v2.0.0", "2.0"},
		{"2.0.0-SNAPSHOT-abc", "2.0"},
		{"dev", "dev"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, MajorMinor(tt.input), "input=%s", tt.input)
	}
}

func TestFromOutput(t *testing.T) {
	for in, want := range map[string]string{"1.17.13\n": "1.17.13", "tool v2.0.20\n": "2.0.20", "weird": "weird"} {
		assert.Equal(t, want, FromOutput(in), "input=%q", in)
	}
}

// A1: pre-releases and `git describe` builds are ordered, so oh never offers
// to "update" to an older version (5.0.0-test → 4.2.0).
func TestCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"5.0.0-test", "4.2.0", 1},
		{"5.0.0", "4.2.0", 1},
		{"v5.0.0", "5.0.0", 0},
		{"5.0.0-test", "5.0.0", -1},
		{"5.0.0-rc.1", "5.0.0-rc.2", -1},
		{"5.0.0-rc.2", "5.0.0-rc.10", -1},
		{"5.0.0-alpha", "5.0.0-alpha.1", -1},
		{"5.0.0-1", "5.0.0-alpha", -1},
		{"5.0.0-beta", "5.0.0-alpha", 1},
		{"v4.2.0-195-gfe9ed932", "4.2.0", 1},
		{"v4.2.0-195-gfe9ed932-dirty", "4.2.0", 1},
		{"v4.2.0-dirty", "4.2.0", 1},
		{"v4.2.0-195-gfe9ed932", "5.0.0", -1},
		{"v4.2.0-195-gfe9ed932", "v4.2.0-200-gabcdef1", -1},
		{"5.0.0-rc.1-3-gabc1234", "5.0.0-rc.1", 1},
		{"5.0.0-rc.1-3-gabc1234", "5.0.0", -1},
		{"5.0.0+build.7", "5.0.0", 0},
	}
	for _, tt := range tests {
		got, ok := Compare(tt.a, tt.b)
		assert.True(t, ok, "%s vs %s", tt.a, tt.b)
		assert.Equal(t, tt.want, got, "%s vs %s", tt.a, tt.b)
		back, _ := Compare(tt.b, tt.a)
		assert.Equal(t, -tt.want, back, "%s vs %s", tt.b, tt.a)
	}
}

func TestCompareInvalid(t *testing.T) {
	for _, s := range []string{"dev", "fe9ed93", "", "5.0", "5.0.x", "5.0.0-"} {
		_, ok := Compare(s, "4.2.0")
		assert.False(t, ok, s)
	}
}
