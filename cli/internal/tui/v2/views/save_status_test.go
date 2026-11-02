package views

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// One delay for every page that saves on its own: the hub and the projects
// do not differ any more (500 ms in one, 200 ms in the others before 5.1).
func TestSaveDelayIsOneForTheHubAndTheProjects(t *testing.T) {
	assert.Equal(t, "500ms", SaveDelay.String())
}

func TestSaveStatusTexts(t *testing.T) {
	i18ntest.Set(t, "fr")
	assert.Equal(t, "enregistré", saveStatusSaved.plain())
	assert.Equal(t, "non publié — Ctrl+S", saveStatusUnpublished.plain())
	assert.Equal(t, "enregistrement…", saveStatusPending.plain())
	assert.Equal(t, "non enregistré", saveStatusFailed.plain())
	i18ntest.Set(t, "en")
	assert.Equal(t, "saved", saveStatusSaved.plain())
	assert.Equal(t, "not published — Ctrl+S", saveStatusUnpublished.plain())
}

func TestSaveBadgeShowsTheStatusInTheTitle(t *testing.T) {
	i18ntest.Set(t, "fr")
	page := widgets.NewListDetailPage("Équipe", "Détail")
	var b saveBadge
	b.attach(page, "Équipe")
	assert.Equal(t, " Équipe ", page.Left.GetTitle(), "a page nobody edited says nothing")
	b.settle(false)
	assert.Equal(t, " Équipe ", page.Left.GetTitle())

	b.set(saveStatusSaved)
	assert.Contains(t, page.Left.GetTitle(), "enregistré")

	b.set(saveStatusUnpublished)
	assert.Contains(t, page.Left.GetTitle(), "non publié — Ctrl+S")
	assert.NotContains(t, page.Left.GetTitle(), "enregistré")

	b.settle(false)
	assert.Contains(t, page.Left.GetTitle(), "enregistré", "published: saved")
	b.settle(true)
	b.retitle("Équipe alpha")
	assert.Contains(t, page.Left.GetTitle(), "Équipe alpha")
	assert.Contains(t, page.Left.GetTitle(), "non publié")

	b.detach()
	b.set(saveStatusSaved) // no page: nothing to draw, no panic
}
