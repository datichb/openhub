package views

import "sync/atomic"

// LayoutLevel is the density of the TUI for the terminal size (N20).
type LayoutLevel int32

const (
	// LevelFull: banner, footer and spacers of the landings, 8-row palette.
	LevelFull LayoutLevel = iota
	// LevelCompact: one-line banner, no footer nor spacers, 3-row palette.
	LevelCompact
	// LevelTooSmall: only the « terminal too small » message is shown.
	LevelTooSmall
)

// Thresholds of the layout levels (terminal columns and rows).
const (
	MinCols     = 60
	MinRows     = 16
	CompactRows = 32
)

// Values of the `[cli] compact` setting.
const (
	CompactAuto   = "auto"
	CompactAlways = "always"
	CompactNever  = "never"
)

// LevelFor returns the layout level of a cols×rows terminal; pref is the
// `[cli] compact` setting ("" = auto). A terminal under MinCols×MinRows is
// always too small.
func LevelFor(cols, rows int, pref string) LayoutLevel {
	if cols < MinCols || rows < MinRows {
		return LevelTooSmall
	}
	switch pref {
	case CompactAlways:
		return LevelCompact
	case CompactNever:
		return LevelFull
	}
	if rows < CompactRows {
		return LevelCompact
	}
	return LevelFull
}

var currentLevel atomic.Int32

// SetLayoutLevel records the level chosen by the shell for the terminal.
func SetLayoutLevel(l LayoutLevel) { currentLevel.Store(int32(l)) }

// CurrentLayoutLevel is the level chosen by the shell (LevelFull before the
// first draw and outside the shell).
func CurrentLayoutLevel() LayoutLevel { return LayoutLevel(currentLevel.Load()) }

// compactLayout reports whether the landings use their compact form.
func compactLayout() bool { return CurrentLayoutLevel() != LevelFull }
