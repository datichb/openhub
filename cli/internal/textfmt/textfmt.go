// Package textfmt is the single place where oh formats what it shows: ages
// and dates, durations, costs, sizes, token counts, and the cutting of a text
// to a length or a width. The CLI, the TUI and the services call it instead
// of keeping their own copy, so that one value reads the same everywhere.
//
// The words and the layouts of dates come from the locales (keys fmt.*).
// Every date is shown in the local time zone. The clock is injectable (see
// SetNow, AgoAt and AgeAt).
//
// The package imports no screen and no command: only i18n and the text
// width library already used by the TUI.
package textfmt

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mattn/go-runewidth"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// ellipsis ends a text that was cut.
const ellipsis = "…"

var clock atomic.Pointer[func() time.Time]

// Now is the current time of the package (time.Now unless SetNow replaced it).
func Now() time.Time {
	if f := clock.Load(); f != nil {
		return (*f)()
	}
	return time.Now()
}

// SetNow replaces the clock of the package and returns the function that puts
// the previous one back. It is meant for tests:
//
//	t.Cleanup(textfmt.SetNow(func() time.Time { return fixed }))
func SetNow(f func() time.Time) (restore func()) {
	prev := clock.Swap(&f)
	return func() { clock.Store(prev) }
}

// ── Ages and durations ───────────────────────────────────────────────────────

// Ago is the time elapsed since t, as « just now », « 3 min ago », « 5 h ago »
// or « 2 d ago » ("" for the zero time). A time in the future reads as « just
// now ».
func Ago(t time.Time) string { return AgoAt(Now(), t) }

// AgoAt is Ago relative to now.
func AgoAt(now, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return i18n.T("fmt.ago.now")
	case d < time.Hour:
		return i18n.Tf("fmt.ago.min", int(d.Minutes()))
	case d < 24*time.Hour:
		return i18n.Tf("fmt.ago.hours", int(d.Hours()))
	}
	return i18n.Tf("fmt.ago.days", int(d.Hours()/24))
}

// Age is the compact form of Ago, for a column: « 3 min », « 5 h », « 2 d »
// ("" for the zero time, « 0 min » under a minute or in the future).
func Age(t time.Time) string { return AgeAt(Now(), t) }

// AgeAt is Age relative to now.
func AgeAt(now, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := max(now.Sub(t), 0)
	switch {
	case d < time.Hour:
		return i18n.Tf("fmt.age.min", int(d.Minutes()))
	case d < 24*time.Hour:
		return i18n.Tf("fmt.age.hours", int(d.Hours()))
	}
	return i18n.Tf("fmt.age.days", int(d.Hours()/24))
}

// Duration is a length of time: « 45s » under a minute, then « 5m », « 2h »
// and « 2h5m » (no day: 50 hours read « 50h »). A negative duration is zero.
func Duration(d time.Duration) string {
	d = max(d, 0)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	h, m := int(d.Hours()), int(d.Minutes())%60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh%dm", h, m)
}

// ── Dates ────────────────────────────────────────────────────────────────────

// Date is the day of t in local time (« 2026-10-23 »; "" for the zero time).
func Date(t time.Time) string { return layout(t, "fmt.date") }

// DateTime is the day and the minute of t in local time (« 2026-10-23 14:05 »).
func DateTime(t time.Time) string { return layout(t, "fmt.datetime") }

// ShortDateTime is the day and the minute of t without the year, for a list
// of recent items (« 10-23 14:05 »; « 23/10 14:05 » in French).
func ShortDateTime(t time.Time) string { return layout(t, "fmt.shortdatetime") }

// Clock is the hour and the minute of t in local time (« 14:05 »).
func Clock(t time.Time) string { return layout(t, "fmt.clock") }

// ClockSec is Clock with the seconds (« 14:05:09 »).
func ClockSec(t time.Time) string { return layout(t, "fmt.clocksec") }

func layout(t time.Time, key string) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format(i18n.T(key))
}

// ── Costs, sizes, counts ─────────────────────────────────────────────────────

// Cost is an amount in dollars to the cent (« $0.12 »; « 0.12 $ » in French).
// An amount above zero and under a cent reads « <$0.01 » (« <0.01 $ ») rather
// than « $0.00 », which would say that nothing was spent; zero stays « $0.00 ».
func Cost(usd float64) string {
	if usd > 0 && usd < 0.01 {
		return i18n.T("fmt.cost_floor")
	}
	return i18n.Tf("fmt.cost", usd)
}

// CostPrecise is Cost to four decimals, for a trace where one call costs a
// fraction of a cent (« $0.0123 »).
func CostPrecise(usd float64) string { return i18n.Tf("fmt.cost_precise", usd) }

// Bytes is a size in powers of 1024: « 820 B », « 2.1 KiB », « 1.5 MiB »,
// « 3.0 GiB » (« 820 o », « 2.1 Kio », « 1.5 Mio », « 3.0 Gio » in French).
func Bytes(n int64) string {
	const kib = 1024
	switch {
	case n >= kib*kib*kib:
		return i18n.Tf("fmt.bytes.gib", float64(n)/(kib*kib*kib))
	case n >= kib*kib:
		return i18n.Tf("fmt.bytes.mib", float64(n)/(kib*kib))
	case n >= kib:
		return i18n.Tf("fmt.bytes.kib", float64(n)/kib)
	}
	return i18n.Tf("fmt.bytes.b", max(n, 0))
}

// Tokens is a count of tokens: « 950 », « 1.5K », « 2.3M ».
func Tokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

// OnOff is « enabled » or « disabled » (« activé » or « désactivé »).
func OnOff(on bool) string {
	if on {
		return i18n.T("fmt.on")
	}
	return i18n.T("fmt.off")
}

// ── Cutting a text ───────────────────────────────────────────────────────────

// Clip cuts s to n runes at most, the last one being « … » when s was cut
// (n counts the ellipsis). It counts runes, not display columns: use
// ClipWidth to fit a column. n < 1 gives "".
func Clip(s string, n int) string {
	if n < 1 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + ellipsis
}

// ClipLine is the first line of s, cut by Clip (no trimming: a text that
// starts with a newline gives "").
func ClipLine(s string, n int) string {
	first, _, _ := strings.Cut(s, "\n")
	return Clip(first, n)
}

// ClipWidth cuts s so that it takes w display columns at most (a wide rune
// such as a Chinese character or an emoji takes two, a combining mark none),
// the last column being « … » when s was cut. w < 1 gives "".
func ClipWidth(s string, w int) string {
	if w < 1 {
		return ""
	}
	return runewidth.Truncate(s, w, ellipsis)
}

// ClipLeft keeps the end of s (a path), cut to n runes at most, with « … »
// first when s was cut (n counts the ellipsis). n < 1 gives "".
func ClipLeft(s string, n int) string {
	if n < 1 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return ellipsis + string(r[len(r)-n+1:])
}

// ClipBytes keeps the first n bytes of s (cut inside a rune, the broken rune
// is dropped) and adds « … » when s was cut: the bound of a stored field,
// not of a display. The ellipsis comes on top of the n bytes.
func ClipBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:max(n, 0)], "") + ellipsis
}

// ClipBytesLeft keeps the last n bytes of s with « … » first when s was cut
// (the end of an output says why it failed). The broken rune, if any, is
// dropped; the ellipsis comes on top of the n bytes.
func ClipBytesLeft(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return ellipsis + strings.ToValidUTF8(s[len(s)-max(n, 0):], "")
}

// FirstLine is the first line of s once its surrounding blanks are removed.
func FirstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return s
}
