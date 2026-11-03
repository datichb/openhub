package views

import "strings"

// CheckedInputShell is implemented by shells whose input can refuse a value:
// check returns the reason of the refusal ("" = accepted), shown while the
// input stays open with what was typed. Without it (fakes), askChecked
// checks after the input closed.
type CheckedInputShell interface {
	ShowCheckedInputModal(title, currentValue string, check func(value string) string, onConfirm func(value string))
}

// askChecked asks a text that must pass check before the input closes: a
// required reason or message is refused with its explanation, not after the
// window (and the card it was asked from) is gone.
func askChecked(sh ShellAccess, title, currentValue string, check func(string) string, onConfirm func(string)) {
	if c, ok := sh.(CheckedInputShell); ok {
		c.ShowCheckedInputModal(title, currentValue, check, onConfirm)
		return
	}
	sh.ShowInputModal(title, currentValue, func(value string) {
		if msg := check(value); msg != "" {
			sh.ShowToastMsg(msg, false)
			return
		}
		onConfirm(value)
	})
}

// requiredText is the check of a text that cannot be empty.
func requiredText(missing string) func(string) string {
	return func(value string) string {
		if strings.TrimSpace(value) == "" {
			return missing
		}
		return ""
	}
}
