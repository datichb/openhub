package shell

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// The same message is not shown twice in a row (a failing refresh, a repeated
// log line); another message, another level, or later, is.
func TestRepeatedToastIsNotShownAgain(t *testing.T) {
	s := &Shell{}
	assert.False(t, s.repeatedToast("pull failed", ToastError))
	assert.True(t, s.repeatedToast("pull failed", ToastError), "same text, same level, right after")
	assert.False(t, s.repeatedToast("pull failed", ToastWarning), "another level")
	assert.False(t, s.repeatedToast("other", ToastError))

	s.toastMu.Lock()
	for k := range s.recentToasts {
		s.recentToasts[k] = time.Now().Add(-2 * toastRepeatWindow)
	}
	s.toastMu.Unlock()
	assert.False(t, s.repeatedToast("pull failed", ToastError), "after the window")
}
