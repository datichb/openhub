// Package i18ntest keeps the language of the messages of one test from
// leaking into the next ones.
package i18ntest

import (
	"testing"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// Set switches the messages to lang for the test and puts the previous
// language back when it ends.
//
//	i18ntest.Set(t, "fr")
func Set(t testing.TB, lang string) {
	t.Helper()
	Restore(t)
	i18n.SetLocale(lang)
}

// Restore puts the current language back when the test ends: for a test that
// runs code which changes it (the init wizard, the startup, the language step).
func Restore(t testing.TB) {
	t.Helper()
	prev := i18n.Locale()
	t.Cleanup(func() { i18n.SetLocale(prev) })
}
