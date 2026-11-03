package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// checkedShell is a shell whose input checks the value itself.
type checkedShell struct {
	mockShell
	checked func(string) string
	title   string
}

func (s *checkedShell) ShowCheckedInputModal(title, _ string, check func(string) string, onConfirm func(string)) {
	s.title = title
	s.checked = check
	_ = onConfirm
}

// A shell that checks the input is given the check (the input stays open
// when it refuses); any other shell gets it applied after the input.
func TestAskCheckedUsesTheCheckOfTheShell(t *testing.T) {
	sh := &checkedShell{}
	asked := 0
	askChecked(sh, "reason", "", requiredText("a reason is needed"), func(string) { asked++ })
	assert.Equal(t, "reason", sh.title)
	assert.Equal(t, "a reason is needed", sh.checked("  "))
	assert.Equal(t, "", sh.checked("because"))
	assert.Zero(t, asked)
}

type plainInputShell struct {
	mockShell
	answer func(string)
	toasts []string
}

func (s *plainInputShell) ShowInputModal(_, _ string, onConfirm func(string)) { s.answer = onConfirm }
func (s *plainInputShell) ShowToastMsg(msg string, _ bool)                    { s.toasts = append(s.toasts, msg) }

func TestAskCheckedFallsBackToCheckingAfterTheInput(t *testing.T) {
	sh := &plainInputShell{}
	var got []string
	askChecked(sh, "reason", "", requiredText("a reason is needed"), func(v string) { got = append(got, v) })
	sh.answer("   ")
	assert.Empty(t, got, "an empty text is refused")
	assert.Equal(t, []string{"a reason is needed"}, sh.toasts)
	sh.answer("because")
	assert.Equal(t, []string{"because"}, got)
}
