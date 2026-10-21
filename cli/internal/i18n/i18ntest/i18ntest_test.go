package i18ntest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

func TestLanguageDoesNotOutliveTheTest(t *testing.T) {
	i18n.SetLocale("en")
	t.Cleanup(func() { i18n.SetLocale("en") })

	t.Run("Set", func(t *testing.T) {
		i18ntest.Set(t, "fr")
		assert.Equal(t, "fr", i18n.Locale(), "the test runs in the requested language")
	})
	assert.Equal(t, "en", i18n.Locale(), "Set puts the previous language back")

	t.Run("Restore", func(t *testing.T) {
		i18ntest.Restore(t)
		i18n.SetLocale("fr")
	})
	assert.Equal(t, "en", i18n.Locale(), "Restore undoes a change made by the code under test")
}
