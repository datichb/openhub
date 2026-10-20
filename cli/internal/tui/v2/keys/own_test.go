package keys_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
	_ "github.com/datichb/openhub/cli/internal/tui/v2/views" // declares the actions of the views
)

// TestOwnActions: the actions of the views (keys.Own) are translated, have
// one ID each and keep the convention (no action of the convention on a
// letter of a view: « a ajouter », « f filtrer »…).
func TestOwnActions(t *testing.T) {
	prev := i18n.Locale()
	t.Cleanup(func() { i18n.SetLocale(prev) })
	owned := keys.Owned()
	assert.NotEmpty(t, owned)
	ids := map[string]bool{}
	for _, a := range owned {
		assert.False(t, ids[a.ID], "two actions %s", a.ID)
		ids[a.ID] = true
		for _, l := range []string{"en", "fr"} {
			i18n.SetLocale(l)
			assert.NotEqual(t, a.LabelKey(), a.Label(), "%s: %s", l, a.ID)
		}
		assert.Empty(t, keys.CheckHintLine(a.Hint()), a.ID)
	}
}
