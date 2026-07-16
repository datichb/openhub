package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func testPickerItems() []PickerItem {
	return []PickerItem{
		{ID: "1", Label: "Alpha", Category: "Group A", Description: "First item"},
		{ID: "2", Label: "Beta", Category: "Group A", Description: "Second item"},
		{ID: "3", Label: "Gamma", Category: "Group B", Description: "Third item"},
		{ID: "4", Label: "Delta", Category: "Group B", Description: "Fourth item"},
		{ID: "5", Label: "Epsilon", Category: "Group C", Description: "Fifth item"},
	}
}

func TestFilterPickerItems_EmptyFilter_ReturnsAll(t *testing.T) {
	items := testPickerItems()
	result := filterPickerItems(items, "")
	assert.Equal(t, 5, len(result))
	assert.Equal(t, "Alpha", result[0].Label)
}

func TestFilterPickerItems_EmptyFilter_ReturnsCopy(t *testing.T) {
	items := testPickerItems()
	result := filterPickerItems(items, "")
	// Modify result — should not affect original
	result[0].Label = "Modified"
	assert.Equal(t, "Alpha", items[0].Label)
}

func TestFilterPickerItems_MatchesLabel(t *testing.T) {
	items := testPickerItems()
	result := filterPickerItems(items, "alpha")
	assert.Equal(t, 1, len(result))
	assert.Equal(t, "Alpha", result[0].Label)
}

func TestFilterPickerItems_CaseInsensitive(t *testing.T) {
	items := testPickerItems()
	result := filterPickerItems(items, "BETA")
	assert.Equal(t, 1, len(result))
	assert.Equal(t, "Beta", result[0].Label)
}

func TestFilterPickerItems_MatchesDescription(t *testing.T) {
	items := testPickerItems()
	result := filterPickerItems(items, "Third")
	assert.Equal(t, 1, len(result))
	assert.Equal(t, "Gamma", result[0].Label)
}

func TestFilterPickerItems_MatchesCategory(t *testing.T) {
	items := testPickerItems()
	result := filterPickerItems(items, "Group B")
	assert.Equal(t, 2, len(result))
	assert.Equal(t, "Gamma", result[0].Label)
	assert.Equal(t, "Delta", result[1].Label)
}

func TestFilterPickerItems_NoMatch(t *testing.T) {
	items := testPickerItems()
	result := filterPickerItems(items, "zzz")
	assert.Equal(t, 0, len(result))
}

func TestFilterPickerItems_PartialMatch(t *testing.T) {
	items := testPickerItems()
	result := filterPickerItems(items, "a")
	// Matches: Alpha, Beta, Delta, Gamma (all contain "a" in label/desc/category)
	assert.True(t, len(result) >= 3)
}

func TestPickerHints_SingleSelect(t *testing.T) {
	hints := pickerHints(false, false)
	assert.Contains(t, hints, "enter select")
	assert.NotContains(t, hints, "space toggle")
}

func TestPickerHints_MultiSelect(t *testing.T) {
	hints := pickerHints(true, false)
	assert.Contains(t, hints, "space toggle")
	assert.Contains(t, hints, "* toggle all")
}

func TestPickerHints_Filtering(t *testing.T) {
	hints := pickerHints(false, true)
	assert.Contains(t, hints, "esc cancel")
	assert.NotContains(t, hints, "navigate")
}
