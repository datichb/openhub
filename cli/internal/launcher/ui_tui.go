package launcher

// TUILaunchUI implements LaunchUI for the TUI dashboard.
// It wraps the shell's SuspendAndExec and toast notification capabilities.
type TUILaunchUI struct {
	suspendFn func(func() error) error
	toastFn   func(msg string, success bool)
}

// NewTUIUI creates a TUI launch UI.
//   - suspendFn: shell.SuspendAndExec — pauses the TUI, runs the callback, resumes.
//   - toastFn: shell.ShowToastMsg — shows a toast notification after the session.
func NewTUIUI(suspendFn func(func() error) error, toastFn func(msg string, success bool)) *TUILaunchUI {
	return &TUILaunchUI{
		suspendFn: suspendFn,
		toastFn:   toastFn,
	}
}

func (t *TUILaunchUI) Confirm(_ string) (bool, error) {
	// TUI sessions skip confirmation — the user already clicked a button.
	return true, nil
}

func (t *TUILaunchUI) Notify(msg string, level Level) {
	if t.toastFn != nil {
		t.toastFn(msg, level == LevelSuccess || level == LevelInfo)
	}
}

// SuspendAndExec returns the shell's suspend function.
func (t *TUILaunchUI) SuspendAndExec() func(func() error) error {
	return t.suspendFn
}
