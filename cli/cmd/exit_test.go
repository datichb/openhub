package cmd

import (
	"bytes"
	"errors"
	"os/exec"
	"testing"
)

// QB1: oh doctor exits non-zero when a check fails (it always exited 0).
func TestDoctorExitCode(t *testing.T) {
	ok := check{"ok", single(func() (string, bool) { return "fine", true })}
	bad := check{"bad", single(func() (string, bool) { return "broken", false })}
	var out bytes.Buffer
	if err := runDoctorChecks(&out, []check{ok, ok}); err != nil || ExitCode(err) != 0 {
		t.Fatalf("all passed: err = %v", err)
	}
	err := runDoctorChecks(&out, []check{ok, bad})
	if ExitCode(err) != 1 {
		t.Fatalf("failed check: err = %v, code %d", err, ExitCode(err))
	}
	var exit *ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("err = %T, want *ExitError (nothing more printed)", err)
	}
}

// QB1: oh beads passes the exit code of bd through.
func TestBdExitCodePassedThrough(t *testing.T) {
	err := bdExit(exec.Command("sh", "-c", "exit 3").Run())
	if ExitCode(err) != 3 {
		t.Fatalf("code = %d (%v), want 3", ExitCode(err), err)
	}
	if err := bdExit(exec.Command("sh", "-c", "exit 0").Run()); err != nil {
		t.Fatalf("err = %v", err)
	}
	if ExitCode(errors.New("boom")) != 1 || ExitCode(nil) != 0 {
		t.Fatal("generic codes")
	}
}
