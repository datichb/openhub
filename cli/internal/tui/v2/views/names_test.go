package views

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

func TestNamesShowTheNameAndKeepAnUnknownIDReadable(t *testing.T) {
	i18ntest.Set(t, "en")
	n := NewNames(map[string]string{"88280c5b": "transparence-sru"}, []teamstate.Member{
		{ID: "bdatiche", DisplayName: "Benjamin"}, {ID: "anon"},
	})
	assert.Equal(t, "transparence-sru", n.Project("88280c5b"))
	assert.Equal(t, "transparence-sru", n.ProjectLabel("88280c5b"))
	assert.True(t, n.KnowsProject("88280c5b"))

	assert.Equal(t, "deadbeef", n.Project("deadbeef"), "an unknown project is its ID")
	assert.Equal(t, "deadbeef (unknown project)", n.ProjectLabel("deadbeef"), "…marked as unknown where it is displayed")
	assert.False(t, n.KnowsProject("deadbeef"))
	assert.Empty(t, n.ProjectLabel(""))

	assert.Equal(t, "Benjamin", n.Member("bdatiche"))
	assert.Equal(t, "anon", n.Member("anon"), "a member without a display name is their ID")
	assert.Equal(t, "gone", n.Member("gone"), "a member who left is their ID")

	i18ntest.Set(t, "fr")
	assert.Equal(t, "deadbeef (projet inconnu)", n.ProjectLabel("deadbeef"))
}

func TestNamesWithoutASourceAnswerWithTheID(t *testing.T) {
	var f NamesFunc
	n := f.Read(context.Background(), nil)
	assert.Equal(t, "p1", n.Project("p1"))
	assert.Equal(t, "alice", n.Member("alice"))
	assert.Equal(t, "Bob", NamesFunc(func(context.Context, teamstate.MemberReader) Names {
		return NewNames(nil, []teamstate.Member{{ID: "bob", DisplayName: "Bob"}})
	}).Read(context.Background(), nil).Member("bob"))
}
