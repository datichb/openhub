package shell

import (
	"time"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// FlashLevel defines the visual style of a status bar flash.
type FlashLevel int

const (
	// FlashSuccess shows a success flash (Jade).
	FlashSuccess FlashLevel = iota
	// FlashError shows an error flash (Ruby).
	FlashError
	// FlashWarning shows a warning flash (Amber).
	FlashWarning
)

// FlashStatus briefly shows a message in the status bar left section,
// then reverts to the previous content after the given duration.
func (s *Shell) FlashStatus(msg string, level FlashLevel, duration time.Duration) {
	icon, colorHex := flashStyle(level)
	flash := theme.ColorTag(colorHex) + icon + " " + msg + theme.TagColor

	// Store current view name for restoration
	var restore string
	if cur := s.router.Current(); cur != nil {
		restore = cur.Title()
	}

	s.statusBar.SetView(flash)

	time.AfterFunc(duration, func() {
		s.app.QueueUpdateDraw(func() {
			if cur := s.router.Current(); cur != nil {
				s.statusBar.SetView(cur.Title())
			} else {
				s.statusBar.SetView(restore)
			}
		})
	})
}

func flashStyle(level FlashLevel) (string, string) {
	switch level {
	case FlashSuccess:
		return theme.IconSuccess, theme.SuccessHex
	case FlashError:
		return theme.IconError, theme.ErrorHex
	case FlashWarning:
		return theme.IconWarning, theme.WarningHex
	default:
		return theme.IconDot, theme.AccentHex
	}
}
