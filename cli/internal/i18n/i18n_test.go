package i18n

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestT(t *testing.T) {
	tests := []struct {
		name   string
		locale string
		key    string
		want   string
	}{
		{"english", "en", "help.flag.help", "Help"},
		{"english section", "en", "help.section.other", "OTHER"},
		{"french", "fr", "help.flag.help", "Aide"},
		{"french section", "fr", "help.section.other", "AUTRES"},
		{"a locale with no messages falls back to english", "ja", "help.flag.help", "Help"},
		{"an unknown key is returned as is", "en", "unknown.key.xyz", "unknown.key.xyz"},
		{"an unknown key is returned as is in french", "fr", "unknown.key.xyz", "unknown.key.xyz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := Locale()
			t.Cleanup(func() { SetLocale(prev) })
			SetLocale(tt.locale)
			assert.Equal(t, tt.want, T(tt.key))
		})
	}

	t.Run("Tf formats the message", func(t *testing.T) {
		prev := Locale()
		t.Cleanup(func() { SetLocale(prev) })
		SetLocale("en")
		assert.Equal(t, "Exporting to backup.tar.gz...\n", Tf("cmd.export.exporting_to", "backup.tar.gz"))
	})
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
