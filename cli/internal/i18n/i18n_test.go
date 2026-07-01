package i18n

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestT_English(t *testing.T) {
	SetLocale("en")
	assert.Equal(t, "OpenHub", T("app.name"))
	assert.Equal(t, "Loading...", T("tui.spinner.loading"))
}

func TestT_French(t *testing.T) {
	SetLocale("fr")
	assert.Equal(t, "OpenHub", T("app.name"))
	assert.Equal(t, "Chargement...", T("tui.spinner.loading"))
}

func TestT_FallbackToEnglish(t *testing.T) {
	SetLocale("fr")
	// Key that only exists in English or both
	assert.Equal(t, "OpenHub", T("app.name"))
}

func TestT_UnknownKey(t *testing.T) {
	SetLocale("en")
	assert.Equal(t, "unknown.key.xyz", T("unknown.key.xyz"))
}

func TestT_UnknownLocale(t *testing.T) {
	SetLocale("ja")
	// Should fallback to English
	assert.Equal(t, "Loading...", T("tui.spinner.loading"))
}

func TestTf_Format(t *testing.T) {
	SetLocale("en")
	result := Tf("error.project_not_found", "my-app")
	assert.Equal(t, "Project not found: my-app", result)
}
