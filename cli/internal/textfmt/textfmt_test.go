package textfmt

import (
	"testing"
	"time"

	"github.com/mattn/go-runewidth"
	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// paris is a zone that is neither UTC nor the zone of the machine running the test.
var paris = time.FixedZone("CEST", 2*3600)

// fixed is 2026-10-23 12:00:00 UTC, i.e. 14:00 in paris.
var fixed = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

// inZone makes the local zone paris for the test.
func inZone(t *testing.T) {
	t.Helper()
	prev := time.Local
	time.Local = paris
	t.Cleanup(func() { time.Local = prev })
}

func TestAgoAndAge(t *testing.T) {
	tests := []struct {
		name         string
		t            time.Time
		enAgo, enAge string
		frAgo, frAge string
	}{
		{"zero", time.Time{}, "", "", "", ""},
		{"now", fixed, "just now", "0 min", "à l'instant", "0 min"},
		{"seconds", fixed.Add(-59 * time.Second), "just now", "0 min", "à l'instant", "0 min"},
		{"one minute", fixed.Add(-time.Minute), "1 min ago", "1 min", "il y a 1 min", "1 min"},
		{"minutes", fixed.Add(-42*time.Minute - 30*time.Second), "42 min ago", "42 min", "il y a 42 min", "42 min"},
		{"just under an hour", fixed.Add(-59*time.Minute - 59*time.Second), "59 min ago", "59 min", "il y a 59 min", "59 min"},
		{"one hour", fixed.Add(-time.Hour), "1 h ago", "1 h", "il y a 1 h", "1 h"},
		{"hours", fixed.Add(-23*time.Hour - 59*time.Minute), "23 h ago", "23 h", "il y a 23 h", "23 h"},
		{"one day", fixed.Add(-24 * time.Hour), "1 d ago", "1 d", "il y a 1 j", "1 j"},
		{"day and a half", fixed.Add(-36 * time.Hour), "1 d ago", "1 d", "il y a 1 j", "1 j"},
		{"days", fixed.Add(-10 * 24 * time.Hour), "10 d ago", "10 d", "il y a 10 j", "10 j"},
		{"future seconds", fixed.Add(30 * time.Second), "just now", "0 min", "à l'instant", "0 min"},
		{"future hours", fixed.Add(5 * time.Hour), "just now", "0 min", "à l'instant", "0 min"},
		{"other zone", fixed.In(paris).Add(-3 * time.Hour), "3 h ago", "3 h", "il y a 3 h", "3 h"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(SetNow(func() time.Time { return fixed }))
			i18ntest.Set(t, "en")
			assert.Equal(t, tt.enAgo, Ago(tt.t), "Ago en")
			assert.Equal(t, tt.enAge, Age(tt.t), "Age en")
			assert.Equal(t, tt.enAgo, AgoAt(fixed, tt.t), "AgoAt en")
			assert.Equal(t, tt.enAge, AgeAt(fixed, tt.t), "AgeAt en")
			i18ntest.Set(t, "fr")
			assert.Equal(t, tt.frAgo, Ago(tt.t), "Ago fr")
			assert.Equal(t, tt.frAge, Age(tt.t), "Age fr")
		})
	}
}

func TestSetNowRestores(t *testing.T) {
	before := Now()
	restore := SetNow(func() time.Time { return fixed })
	assert.True(t, Now().Equal(fixed))
	restore()
	assert.WithinDuration(t, before, Now(), time.Minute)
	assert.False(t, Now().Equal(fixed))
}

func TestDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{-5 * time.Minute, "0s"},
		{0, "0s"},
		{45 * time.Second, "45s"},
		{59*time.Second + 900*time.Millisecond, "59s"},
		{time.Minute, "1m"},
		{5*time.Minute + 59*time.Second, "5m"},
		{59 * time.Minute, "59m"},
		{time.Hour, "1h"},
		{2*time.Hour + 5*time.Minute, "2h5m"},
		{2*time.Hour + 59*time.Second, "2h"},
		{50 * time.Hour, "50h"},
		{50*time.Hour + 30*time.Minute, "50h30m"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, Duration(tt.d), tt.d.String())
	}
}

func TestDates(t *testing.T) {
	inZone(t)
	// 2026-10-23 22:30:09 UTC is already the 24th at 00:30:09 in paris.
	at := time.Date(2026, 10, 23, 22, 30, 9, 0, time.UTC)
	shortDateTime := map[string]string{"en": "10-24 00:30", "fr": "24/10 00:30"}
	for _, lang := range []string{"en", "fr"} {
		t.Run(lang, func(t *testing.T) {
			i18ntest.Set(t, lang)
			assert.Equal(t, "2026-10-24", Date(at))
			assert.Equal(t, "2026-10-24 00:30", DateTime(at))
			assert.Equal(t, shortDateTime[lang], ShortDateTime(at))
			assert.Equal(t, "00:30", Clock(at))
			assert.Equal(t, "00:30:09", ClockSec(at))
			assert.Equal(t, "2026-10-24", Date(at.In(time.UTC)), "the zone of the value does not matter")
		})
	}

	t.Run("zero", func(t *testing.T) {
		for _, f := range []func(time.Time) string{Date, DateTime, ShortDateTime, Clock, ClockSec} {
			assert.Empty(t, f(time.Time{}))
		}
	})
}

func TestCost(t *testing.T) {
	i18ntest.Set(t, "en")
	assert.Equal(t, "$0.12", Cost(0.123))
	assert.Equal(t, "$0.00", Cost(0))
	assert.Equal(t, "$12.35", Cost(12.345678))
	assert.Equal(t, "$0.0123", CostPrecise(0.01234))
	i18ntest.Set(t, "fr")
	assert.Equal(t, "0.12 $", Cost(0.123))
	assert.Equal(t, "0.0123 $", CostPrecise(0.01234))
}

// A cost above zero and under a cent shows a floor, not « $0.00 »; zero and
// a cent are shown as is, and CostPrecise keeps its four decimals.
func TestCostFloor(t *testing.T) {
	tests := []struct {
		usd    float64
		en, fr string
	}{
		{0, "$0.00", "0.00 $"},
		{0.000001, "<$0.01", "<0.01 $"},
		{0.004, "<$0.01", "<0.01 $"},
		{0.006, "<$0.01", "<0.01 $"},
		{0.0099, "<$0.01", "<0.01 $"},
		{0.01, "$0.01", "0.01 $"},
		{0.0149, "$0.01", "0.01 $"},
		{-0.5, "$-0.50", "-0.50 $"},
	}
	for _, tt := range tests {
		i18ntest.Set(t, "en")
		assert.Equal(t, tt.en, Cost(tt.usd), "en %v", tt.usd)
		i18ntest.Set(t, "fr")
		assert.Equal(t, tt.fr, Cost(tt.usd), "fr %v", tt.usd)
	}
	i18ntest.Set(t, "en")
	assert.Equal(t, "$0.0040", CostPrecise(0.004), "CostPrecise has no floor")
}

func TestBytes(t *testing.T) {
	tests := []struct {
		n      int64
		en, fr string
	}{
		{-1, "0 B", "0 o"},
		{0, "0 B", "0 o"},
		{820, "820 B", "820 o"},
		{1023, "1023 B", "1023 o"},
		{1024, "1.0 KiB", "1.0 Kio"},
		{2150, "2.1 KiB", "2.1 Kio"},
		{1<<20 - 1, "1024.0 KiB", "1024.0 Kio"},
		{3 << 19, "1.5 MiB", "1.5 Mio"},
		{3 << 30, "3.0 GiB", "3.0 Gio"},
	}
	for _, tt := range tests {
		i18ntest.Set(t, "en")
		assert.Equal(t, tt.en, Bytes(tt.n), "en %d", tt.n)
		i18ntest.Set(t, "fr")
		assert.Equal(t, tt.fr, Bytes(tt.n), "fr %d", tt.n)
	}
}

func TestTokens(t *testing.T) {
	for n, want := range map[int64]string{
		0: "0", 950: "950", 999: "999", 1000: "1.0K", 1500: "1.5K", 999_999: "1000.0K", 1_000_000: "1.0M", 2_340_000: "2.3M",
	} {
		assert.Equal(t, want, Tokens(n), "%d", n)
	}
}

func TestOnOff(t *testing.T) {
	i18ntest.Set(t, "en")
	assert.Equal(t, "enabled", OnOff(true))
	assert.Equal(t, "disabled", OnOff(false))
	i18ntest.Set(t, "fr")
	assert.Equal(t, "activé", OnOff(true))
	assert.Equal(t, "désactivé", OnOff(false))
}

func TestClip(t *testing.T) {
	tests := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{"short", "abc", 5, "abc"},
		{"exact", "abcde", 5, "abcde"},
		{"cut", "abcdef", 5, "abcd…"},
		{"runes not bytes", "éééééé", 5, "éééé…"},
		{"runes not bytes, fits", "ééééé", 5, "ééééé"},
		{"wide runes count one", "日本語日本語", 4, "日本語…"},
		{"one", "abc", 1, "…"},
		{"empty", "", 3, ""},
		{"zero", "abc", 0, ""},
		{"negative", "abc", -2, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { assert.Equal(t, tt.want, Clip(tt.s, tt.n)) })
	}
}

func TestClipLine(t *testing.T) {
	assert.Equal(t, "first", ClipLine("first\nsecond", 10))
	assert.Equal(t, "fir…", ClipLine("first\nsecond", 4))
	assert.Equal(t, "", ClipLine("\nsecond", 10), "no trimming")
	assert.Equal(t, "single", ClipLine("single", 10))
}

func TestClipWidth(t *testing.T) {
	tests := []struct {
		name  string
		s     string
		w     int
		want  string
		width int // display width expected for the result (-1: not checked)
	}{
		{"ascii fits", "hello", 5, "hello", 5},
		{"ascii cut", "hello world", 6, "hello…", 6},
		{"wide fits", "日本語", 6, "日本語", 6},
		{"wide cut leaves no half character", "日本語日本語", 6, "日本…", 5},
		{"wide cut at an even column", "日本語日本語", 7, "日本語…", 7},
		{"emoji", "ab😀😀😀", 5, "ab😀…", 5},
		{"combining marks take no column", "e\u0301e\u0301e\u0301e\u0301", 4, "e\u0301e\u0301e\u0301e\u0301", 4},
		{"combining marks kept with their base", "e\u0301e\u0301e\u0301e\u0301e\u0301", 4, "e\u0301e\u0301e\u0301…", 4},
		{"zero", "abc", 0, "", 0},
		{"negative", "abc", -1, "", 0},
		{"empty", "", 4, "", 0},
		{"one column", "abc", 1, "…", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClipWidth(tt.s, tt.w)
			assert.Equal(t, tt.want, got)
			if tt.w > 0 {
				assert.LessOrEqual(t, runewidth.StringWidth(got), tt.w)
			}
			assert.Equal(t, tt.width, runewidth.StringWidth(got))
		})
	}
}

func TestClipLeft(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"/a/b/c", 10, "/a/b/c"},
		{"/a/b/c", 6, "/a/b/c"},
		{"/a/b/c/d", 6, "…b/c/d"},
		{"/home/éric/projets", 8, "…projets"},
		{"abc", 1, "…"},
		{"abc", 0, ""},
		{"", 3, ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, ClipLeft(tt.s, tt.n), "%q %d", tt.s, tt.n)
	}
}

func TestClipBytes(t *testing.T) {
	assert.Equal(t, "abc", ClipBytes("abc", 3))
	assert.Equal(t, "ab…", ClipBytes("abcdef", 2))
	assert.Equal(t, "é…", ClipBytes("éé", 2), "two bytes keep one é")
	assert.Equal(t, "…", ClipBytes("éé", 1), "the broken rune is dropped")
	assert.Equal(t, "…", ClipBytes("abc", 0))
}

func TestClipBytesLeft(t *testing.T) {
	assert.Equal(t, "abc", ClipBytesLeft("abc", 3))
	assert.Equal(t, "…ef", ClipBytesLeft("abcdef", 2))
	assert.Equal(t, "…é", ClipBytesLeft("éé", 2))
	assert.Equal(t, "…", ClipBytesLeft("éé", 1), "the broken rune is dropped")
}

func TestFirstLine(t *testing.T) {
	assert.Equal(t, "first", FirstLine("first\nsecond"))
	assert.Equal(t, "first", FirstLine("\n\nfirst\nsecond"), "the blanks around the text are removed first")
	assert.Equal(t, "first", FirstLine("  first\nsecond\n"))
	assert.Equal(t, "first  ", FirstLine("first  \nsecond"), "the end of the first line is kept as is")
	assert.Equal(t, "single", FirstLine("single"))
	assert.Equal(t, "", FirstLine(" \n "))
	assert.Equal(t, "", FirstLine(""))
}
