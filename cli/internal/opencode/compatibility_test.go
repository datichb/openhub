package opencode

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckCompatibilityInRange(t *testing.T) {
	result := CheckCompatibility("2.0.0", "1.17.13")
	assert.True(t, result.Compatible)
	assert.Empty(t, result.Warning)
}

func TestCheckCompatibilityMinBoundary(t *testing.T) {
	result := CheckCompatibility("2.0.0", "1.15.0")
	assert.True(t, result.Compatible)
	assert.Empty(t, result.Warning)
}

func TestCheckCompatibilityMaxBoundary(t *testing.T) {
	result := CheckCompatibility("2.0.0", "1.99.99")
	assert.True(t, result.Compatible)
	assert.Empty(t, result.Warning)
}

func TestCheckCompatibilityTooOld(t *testing.T) {
	result := CheckCompatibility("2.0.0", "1.14.9")
	assert.False(t, result.Compatible)
	assert.Contains(t, result.Warning, "trop ancien")
	assert.Contains(t, result.Warning, "1.15.0")
}

func TestCheckCompatibilityTooNew(t *testing.T) {
	result := CheckCompatibility("2.0.0", "2.0.0")
	assert.False(t, result.Compatible)
	assert.Contains(t, result.Warning, "pas été testé")
}

func TestCheckCompatibilityEmptyOpencodeVersion(t *testing.T) {
	result := CheckCompatibility("2.0.0", "")
	assert.False(t, result.Compatible)
	assert.Contains(t, result.Warning, "inconnue")
}

func TestCheckCompatibilityUnknownOhVersion(t *testing.T) {
	// If oh version is not in the matrix, assume compatible
	result := CheckCompatibility("3.5.0", "1.17.13")
	assert.True(t, result.Compatible)
	assert.Empty(t, result.Warning)
}

func TestCheckCompatibilitySnapshotVersion(t *testing.T) {
	result := CheckCompatibility("2.0.0-SNAPSHOT-abc123", "1.17.13")
	assert.True(t, result.Compatible)
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
		assert.Equal(t, tt.expected, majorMinor(tt.input), "input=%s", tt.input)
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected semver
	}{
		{"1.17.13", semver{1, 17, 13}},
		{"v2.0.0", semver{2, 0, 0}},
		{"0.42.0", semver{0, 42, 0}},
		{"1.15.0-beta", semver{1, 15, 0}},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, parseVersion(tt.input), "input=%s", tt.input)
	}
}

func TestSemverLessThan(t *testing.T) {
	assert.True(t, semver{1, 14, 9}.lessThan(semver{1, 15, 0}))
	assert.False(t, semver{1, 15, 0}.lessThan(semver{1, 15, 0}))
	assert.False(t, semver{1, 17, 13}.lessThan(semver{1, 15, 0}))
	assert.True(t, semver{0, 99, 99}.lessThan(semver{1, 0, 0}))
	assert.True(t, semver{1, 99, 99}.lessThan(semver{2, 0, 0}))
}
