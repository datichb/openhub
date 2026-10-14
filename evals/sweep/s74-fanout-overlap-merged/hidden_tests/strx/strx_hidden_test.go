package strx

import "testing"

func TestHiddenStrx(t *testing.T) {
	if Reverse("abc") != "cba" || Shout("ok") != "OK!" {
		t.Fatal("strx behaviour changed")
	}
}
