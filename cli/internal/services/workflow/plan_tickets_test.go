package workflow

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTickets struct {
	epics map[string][]string
	err   error
}

func (f fakeTickets) IsEpic(id string) (bool, error) {
	if id == "unknown" {
		return false, errors.New("not found")
	}
	_, ok := f.epics[id]
	return ok, nil
}

func (f fakeTickets) OpenChildren(epic string) ([]string, error) { return f.epics[epic], f.err }

// A24: an epic stands for its open children, each ticket once.
func TestExpandEpics(t *testing.T) {
	src := fakeTickets{epics: map[string][]string{"pt-c3b": {"pt-c3b.1", "pt-c3b.2"}, "pt-e": nil}}
	got, exps, err := ExpandEpics(src, []string{"pt-1", "pt-c3b", "pt-c3b.1", "unknown"})
	require.NoError(t, err)
	assert.Equal(t, []string{"pt-1", "pt-c3b.1", "pt-c3b.2", "unknown"}, got)
	assert.Equal(t, []EpicExpansion{{Epic: "pt-c3b", Children: []string{"pt-c3b.1", "pt-c3b.2"}}}, exps)

	_, _, err = ExpandEpics(src, []string{"pt-e"})
	assert.ErrorIs(t, err, ErrEmptyEpic)

	got, exps, err = ExpandEpics(nil, []string{"pt-c3b"})
	require.NoError(t, err)
	assert.Equal(t, []string{"pt-c3b"}, got)
	assert.Empty(t, exps)
}
