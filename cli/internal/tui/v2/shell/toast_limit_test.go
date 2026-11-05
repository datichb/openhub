package shell

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// watchToastPages records the most pages the shell ever holds above its
// number when the test starts (each toast is one page). Call before the first
// toast; max is read through the returned function, on the event loop.
func (s *simShell) watchToastPages() (max func() int) {
	most, base := 0, 0
	s.loop(func() {
		base = s.pages.GetPageCount()
		s.pages.SetChangedFunc(func() {
			if n := s.pages.GetPageCount() - base; n > most {
				most = n
			}
		})
	})
	return func() (n int) {
		s.loop(func() { n = most })
		return n
	}
}

func (s *simShell) toastSlotsInUse() (slots []int) {
	s.loop(func() {
		for _, id := range s.activeToastIDs {
			slots = append(slots, s.toastSlots[id])
		}
	})
	return slots
}

// A burst of toasts never shows more than maxVisibleToasts at once: the
// oldest ones make room, and none is lost from the notification centre.
func TestToastBurstIsLimited(t *testing.T) {
	const burst = 12
	require.Greater(t, burst, maxVisibleToasts)
	s := newKeyShell(t, &keyView{id: "home", focusList: true})
	most := s.watchToastPages()

	var wg sync.WaitGroup
	for i := range burst {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.ShowToast(fmt.Sprintf("burst message %02d", i), ToastError) // errors stay on screen
		}()
	}
	wg.Wait()
	require.Eventually(t, func() bool { return s.notifications.Len() == burst && s.toastCount() == maxVisibleToasts },
		5*time.Second, 10*time.Millisecond, "every toast is recorded, five are shown")
	time.Sleep(100 * time.Millisecond) // nothing more comes

	assert.Equal(t, maxVisibleToasts, s.toastCount())
	assert.LessOrEqual(t, most(), maxVisibleToasts, "never more than the limit at a given time")
	assert.Equal(t, maxVisibleToasts, strings.Count(s.screenText(), "burst message"), "five on the screen")
	slots := s.toastSlotsInUse()
	assert.ElementsMatch(t, []int{0, 1, 2, 3, 4}, slots, "each toast has a row of its own")
}

// The newest toast replaces the oldest, in the row it freed.
func TestToastLimitReplacesTheOldest(t *testing.T) {
	s := newKeyShell(t, &keyView{id: "home", focusList: true})
	for i := 1; i <= maxVisibleToasts+2; i++ {
		msg := fmt.Sprintf("message %d", i)
		s.ShowToast(msg, ToastError)
		require.Eventually(t, func() bool { return strings.Contains(s.screenText(), msg) }, 5*time.Second, 10*time.Millisecond)
	}
	screen := s.screenText()
	for _, gone := range []string{"message 1 ", "message 2 "} {
		assert.NotContains(t, screen, gone)
	}
	for i := 3; i <= maxVisibleToasts+2; i++ {
		assert.Contains(t, screen, fmt.Sprintf("message %d ", i))
	}
	assert.Equal(t, maxVisibleToasts, s.toastCount())
	assert.ElementsMatch(t, []int{0, 1, 2, 3, 4}, s.toastSlotsInUse())
	assert.Equal(t, maxVisibleToasts+2, s.notifications.Len(), "the replaced ones stay in the notification centre")
}

// A toast closed in the middle leaves its row to the next one instead of
// the next one drawn over its neighbour.
func TestToastTakesTheFreedRow(t *testing.T) {
	s := newKeyShell(t, &keyView{id: "home", focusList: true})
	for i := 1; i <= 3; i++ {
		msg := fmt.Sprintf("row test %d", i)
		s.ShowToast(msg, ToastError)
		require.Eventually(t, func() bool { return strings.Contains(s.screenText(), msg) }, 5*time.Second, 10*time.Millisecond)
	}
	s.loop(func() { s.dismissToast(s.activeToastIDs[0]) })
	s.ShowToast("row test 4", ToastError)
	require.Eventually(t, func() bool { return strings.Contains(s.screenText(), "row test 4") }, 5*time.Second, 10*time.Millisecond)
	assert.ElementsMatch(t, []int{0, 1, 2}, s.toastSlotsInUse(), "three toasts, three different rows")
}
