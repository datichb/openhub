package web

import "testing"

func TestSafeNext(t *testing.T) {
	if got := SafeNext(""); got != "/" {
		t.Fatalf("SafeNext(\"\") = %q", got)
	}
	if got := SafeNext("/compte"); got != "/compte" {
		t.Fatalf("SafeNext(/compte) = %q", got)
	}
}
