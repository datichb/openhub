package teamstate

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeName_Valid(t *testing.T) {
	tests := []string{
		"conventions",
		"my-page",
		"ticket_BD-42",
		"2026-07-api-migration",
		"page.with.dots",
	}
	for _, name := range tests {
		result, err := SafeName(name)
		assert.NoError(t, err, "name: %q", name)
		assert.Equal(t, name, result)
	}
}

func TestSafeName_PathTraversal(t *testing.T) {
	tests := []string{
		"../etc/passwd",
		"..%2fetc%2fpasswd",
		"page/../secret",
		"..",
	}
	for _, name := range tests {
		_, err := SafeName(name)
		assert.Error(t, err, "name: %q should be rejected", name)
		assert.True(t, errors.Is(err, ErrUnsafeName), "name: %q", name)
	}
}

func TestSafeName_PathSeparator(t *testing.T) {
	tests := []string{
		"wiki/page",
		"projects\\claims",
		"/absolute",
		"nested/deep/path",
	}
	for _, name := range tests {
		_, err := SafeName(name)
		assert.Error(t, err, "name: %q should be rejected", name)
		assert.True(t, errors.Is(err, ErrUnsafeName), "name: %q", name)
	}
}

func TestSafeName_Empty(t *testing.T) {
	_, err := SafeName("")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsafeName))
}

func TestSafeName_NullByte(t *testing.T) {
	_, err := SafeName("page\x00evil")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsafeName))
}
