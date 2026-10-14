package clock

import (
	"os"
	"testing"
)

func TestNowParis(t *testing.T) {
	os.Setenv("TZ", "Europe/Paris")
	defer os.Unsetenv("TZ")
	if Now().IsZero() {
		t.Fatal("zero")
	}
}
