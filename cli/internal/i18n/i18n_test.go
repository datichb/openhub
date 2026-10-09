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

// keyOfScan is the former KeyOf: a scan of every message.
func keyOfScan(s string) string {
	for _, locale := range []string{"en", "fr"} {
		best := ""
		for k, v := range loadLocale(locale) {
			if v == s && (best == "" || k < best) {
				best = k
			}
		}
		if best != "" {
			return best
		}
	}
	return ""
}

// N13: the reverse index gives the same key as the scan.
func TestKeyOfMatchesTheScan(t *testing.T) {
	n := 0
	for _, locale := range []string{"en", "fr"} {
		for _, v := range loadLocale(locale) {
			if n++; n%7 != 0 {
				continue
			}
			if got, want := KeyOf(v), keyOfScan(v); got != want {
				t.Fatalf("KeyOf(%q) = %q, want %q", v, got, want)
			}
		}
	}
	if KeyOf("not a message of oh") != "" || KeyOf("") != "" {
		t.Fatal("unknown text has no key")
	}
}

func BenchmarkKeyOf(b *testing.B) {
	msg := T("cmd.repair.long")
	for b.Loop() {
		KeyOf(msg)
	}
}
