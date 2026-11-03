package shell

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// A view closes the overlay it showed, and only that one.
func TestCloseOverlayOnlyClosesItsOwn(t *testing.T) {
	homeView := &testView{id: "home", title: "Home"}
	s := New(Config{ProjectName: "test", Views: []views.View{homeView}, HomeViewID: "home"})
	s.NavigateHome("home", views.ModeHub)
	var _ views.OverlayCloser = s
	assert.Nil(t, s.CurrentOverlay())

	s.ShowScrollableModal("decision", "body", nil)
	first := s.CurrentOverlay()
	require.NotNil(t, first)

	// Closed by the user, another overlay takes the place: the first one is no
	// longer the one shown.
	s.closeWindows()
	s.ShowInlineForm(views.InlineFormConfig{Title: "other", Fields: []views.FormField{{Key: "k", Label: "k", Type: views.FieldText}}})
	assert.False(t, s.CloseOverlay(first), "another overlay replaced it")
	assert.True(t, s.pages.HasPage("inline-overlay"))

	second := s.CurrentOverlay()
	assert.True(t, s.CloseOverlay(second))
	assert.False(t, s.pages.HasPage("inline-overlay"))
	assert.Equal(t, s.content, s.app.GetFocus())
	assert.False(t, s.CloseOverlay(second))
	assert.False(t, s.CloseOverlay(nil))
}

// An overlay opened over the one a view follows (a form asked from the card)
// is part of it: it closes with it.
func TestCloseOverlayClosesWhatWasOpenedOverIt(t *testing.T) {
	homeView := &testView{id: "home", title: "Home"}
	s := New(Config{ProjectName: "test", Views: []views.View{homeView}, HomeViewID: "home"})
	s.NavigateHome("home", views.ModeHub)

	s.ShowScrollableModal("decision", "body", nil)
	card := s.CurrentOverlay()
	s.ShowInlineForm(views.InlineFormConfig{Title: "other", Fields: []views.FormField{{Key: "k", Label: "k", Type: views.FieldText}}})
	require.True(t, s.pages.HasPage("sub-overlay"))
	assert.Same(t, card, s.CurrentOverlay(), "the overlay followed is the card")

	assert.True(t, s.CloseOverlay(card))
	assert.False(t, s.pages.HasPage("inline-overlay"))
	assert.False(t, s.pages.HasPage("sub-overlay"))
	assert.Equal(t, s.content, s.app.GetFocus())
}
