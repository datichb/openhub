package prettylog

import "testing"

// U-09: the logs have no color on a terminal either when NO_COLOR is set.
func TestColorDisabled(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	if colorDisabled(true) {
		t.Error("a terminal without NO_COLOR is colored")
	}
	if !colorDisabled(false) {
		t.Error("a pipe is not colored")
	}
	t.Setenv("NO_COLOR", "1")
	if !colorDisabled(true) {
		t.Error("NO_COLOR=1 turns the colors of a terminal off")
	}
}
