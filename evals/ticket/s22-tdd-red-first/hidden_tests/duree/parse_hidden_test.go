package duree

import (
	"testing"
	"time"
)

func TestHiddenParse(t *testing.T) {
	ok := map[string]time.Duration{
		"1h30": 90 * time.Minute,
		"2h":   2 * time.Hour,
		"45m":  45 * time.Minute,
		"2h05": 125 * time.Minute,
		"0m":   0,
	}
	for in, want := range ok {
		got, err := Parse(in)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %v, %v ; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "1x", "h30", "1h75", "-2h", "1h3"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) : erreur attendue", in)
		}
	}
}

func TestHiddenRoundTrip(t *testing.T) {
	for _, d := range []time.Duration{0, 5 * time.Minute, time.Hour, 61 * time.Minute, 10*time.Hour + 59*time.Minute} {
		got, err := Parse(Format(d))
		if err != nil || got != d {
			t.Errorf("Parse(Format(%v)) = %v, %v", d, got, err)
		}
	}
}
