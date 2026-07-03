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
