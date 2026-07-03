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
