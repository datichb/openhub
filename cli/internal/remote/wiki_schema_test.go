package remote

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// p6-debts: the proposals that replace a line travel apart (an oh of
// schema 1 leaves them out instead of adding them); a newer schema is
// refused.
func TestWikiSchema2(t *testing.T) {
	w := NewWiki([]WikiEntry{{ID: "wp-1", Content: "a"}, {ID: "wp-2", Content: "b", Replaces: "old"}, {ID: "wp-10", Content: "c"}})
	assert.Equal(t, 2, w.Schema)
	require.Len(t, w.Proposals, 2)
	require.Len(t, w.Replacements, 1)
	data, err := json.Marshal(w)
	require.NoError(t, err)

	// What an oh of schema 1 reads: the proposals key only.
	var v1 struct {
		Proposals []struct {
			ID, Content string
		} `json:"proposals"`
	}
	require.NoError(t, json.Unmarshal(data, &v1))
	assert.Len(t, v1.Proposals, 2, "the replacement is not read as a line to add")

	var back Wiki
	require.NoError(t, json.Unmarshal(data, &back))
	all, err := back.Entries()
	require.NoError(t, err)
	ids := []string{}
	for _, e := range all {
		ids = append(ids, e.ID)
	}
	assert.Equal(t, []string{"wp-1", "wp-2", "wp-10"}, ids)
	assert.Equal(t, "old", all[1].Replaces)

	_, err = Wiki{Schema: 3}.Entries()
	assert.Error(t, err)
	old, err := Wiki{Schema: 1, Proposals: []WikiEntry{{ID: "wp-1"}}}.Entries()
	require.NoError(t, err)
	assert.Len(t, old, 1, "schema 1 still read")
	assert.NotNil(t, NewWiki(nil).Proposals)
}
