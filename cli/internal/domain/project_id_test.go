package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/domain"
)

func TestNewProjectID(t *testing.T) {
	assert.Regexp(t, `^my-cool-project-[0-9a-f]{8}$`, domain.NewProjectID("My cool project"))
	assert.Regexp(t, `^project-[0-9a-f]{8}$`, domain.NewProjectID("日本語"))
	assert.NotEqual(t, domain.NewProjectID("x"), domain.NewProjectID("x"))
	slug, ok := domain.ProjectIDSlug(domain.NewProjectID("T SRU"))
	assert.True(t, ok)
	assert.Equal(t, "t-sru", slug, "the slug of an ID is the slug of the name")
}

func TestProjectSlug(t *testing.T) {
	assert.Equal(t, "hello-world", domain.ProjectSlug("  Hello World! "))
	assert.Equal(t, "project", domain.ProjectSlug("---"))
	long := domain.ProjectSlug(strings.Repeat("a", 40) + "-b")
	assert.Equal(t, strings.Repeat("a", 32), long)
	assert.Equal(t, "ab", domain.ProjectSlug("ab"+strings.Repeat(" ", 3)+"-"), "no trailing dash")
}

func TestProjectIDSlug(t *testing.T) {
	for id, want := range map[string]string{"t-sru-b267fbf1": "t-sru", "a-00000000": "a"} {
		slug, ok := domain.ProjectIDSlug(id)
		assert.True(t, ok, id)
		assert.Equal(t, want, slug, id)
	}
	for _, id := range []string{"88280c5b", "t-sru", "t-sru-B267FBF1", "t-sru-b267fbf", "-b267fbf1", ""} {
		_, ok := domain.ProjectIDSlug(id)
		assert.False(t, ok, id)
	}
}
