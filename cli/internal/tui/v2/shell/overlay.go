package shell

// CurrentOverlay implements views.OverlayCloser: the overlay shown now (nil
// when none).
func (s *Shell) CurrentOverlay() any {
	if !s.pages.HasPage(windowPages[0]) {
		return nil
	}
	return s.pages.GetPage(windowPages[0])
}

// CloseOverlay implements views.OverlayCloser: it closes the overlay (and
// its sub-overlay) when it is still the one shown. Event loop only.
func (s *Shell) CloseOverlay(token any) bool {
	if token == nil || s.CurrentOverlay() != token {
		return false
	}
	s.closeWindows()
	return true
}
