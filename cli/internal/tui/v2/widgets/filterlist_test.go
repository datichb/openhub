package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterableList(t *testing.T) {
	items := []FilterItem{
		{MainText: "Start session", SecondaryText: "launch session"},
		{MainText: "Board view", SecondaryText: "kanban"},
		{MainText: "Status check", SecondaryText: "system health"},
	}
	fl := NewFilterableList(items, nil)
	require.Equal(t, 3, fl.list.GetItemCount(), "all items listed")

	tests := []struct {
		query string
		want  []string
	}{
		{"board", []string{"Board view"}},
		{"s", []string{"Start session", "Status check"}},
		{"SESSION", []string{"Start session"}},
		{"", []string{"Start session", "Board view", "Status check"}},
	}
	for _, tc := range tests {
		t.Run("query "+tc.query, func(t *testing.T) {
			fl.filter(tc.query)
			var got []string
			for _, it := range fl.visible {
				got = append(got, it.MainText)
			}
			assert.Equal(t, tc.want, got, "visible items (case-insensitive)")
		})
	}
}
