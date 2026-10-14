package duree

import (
	"testing"
	"time"
)

func TestFormat(t *testing.T) {
	cases := map[time.Duration]string{
		90 * time.Minute:  "1h30",
		2 * time.Hour:     "2h",
		45 * time.Minute:  "45m",
		125 * time.Minute: "2h05",
	}
	for d, want := range cases {
		if got := Format(d); got != want {
			t.Errorf("Format(%v) = %q, want %q", d, got, want)
		}
	}
}
