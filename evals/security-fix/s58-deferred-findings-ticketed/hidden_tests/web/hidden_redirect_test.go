package web

import "testing"

func TestHiddenSafeNextRefusesOtherSites(t *testing.T) {
	for _, in := range []string{"https://evil.example/x", "//evil.example/x", "/\\evil.example/x", "http:evil.example", "javascript:alert(1)"} {
		if got := SafeNext(in); got != "/" {
			t.Errorf("SafeNext(%q) = %q, want /", in, got)
		}
	}
	for _, in := range []string{"/compte", "/factures?annee=2026"} {
		if got := SafeNext(in); got != in {
			t.Errorf("SafeNext(%q) = %q, want it kept", in, got)
		}
	}
}
