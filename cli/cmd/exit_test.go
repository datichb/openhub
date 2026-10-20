package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
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

// U-05: the exit codes are named, in one place.
func TestExitCodesAreDistinct(t *testing.T) {
	if ExitOK != 0 || ExitFailure != 1 || ExitUsage != 2 || ExitPending != 3 {
		t.Fatalf("codes = %d %d %d %d, want 0 1 2 3", ExitOK, ExitFailure, ExitUsage, ExitPending)
	}
}

func TestExitCodeOfErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"none", nil, ExitOK},
		{"plain", errors.New("boom"), ExitFailure},
		{"exit error", &ExitError{Code: ExitUsage}, ExitUsage},
		{"coded", withExitCode(ExitPending, errors.New("waits")), ExitPending},
		{"wrapped", fmt.Errorf("ctx: %w", withExitCode(ExitUsage, errors.New("x"))), ExitUsage},
		{"joined", errors.Join(withExitCode(ExitPending, errors.New("a"))), ExitPending},
	}
	for _, tc := range cases {
		if got := ExitCode(tc.err); got != tc.want {
			t.Errorf("%s: code %d, want %d", tc.name, got, tc.want)
		}
	}
	if withExitCode(ExitPending, nil) != nil {
		t.Error("nil error must stay nil")
	}
}

// U-05: a headless launch exits 3 when it only waits for a decision, 1 as
// soon as a session failed.
func TestHeadlessExitCodes(t *testing.T) {
	waits := errors.New("session s1 waits for a decision")
	failed := errors.New("session s2 failed")
	if err := headlessError(nil, 0); err != nil {
		t.Fatalf("no error: %v", err)
	}
	if got := ExitCode(headlessError([]error{waits}, 1)); got != ExitPending {
		t.Errorf("pending only: code %d, want %d", got, ExitPending)
	}
	if got := ExitCode(headlessError([]error{waits, waits}, 2)); got != ExitPending {
		t.Errorf("two pending: code %d, want %d", got, ExitPending)
	}
	if got := ExitCode(headlessError([]error{failed}, 0)); got != ExitFailure {
		t.Errorf("failure: code %d, want %d", got, ExitFailure)
	}
	mixed := headlessError([]error{waits, failed}, 1)
	if got := ExitCode(mixed); got != ExitFailure {
		t.Errorf("failure and pending: code %d, want %d (the failure wins)", got, ExitFailure)
	}
	if !strings.Contains(headlessError([]error{waits}, 1).Error(), "waits") {
		t.Error("the message of the pending session is kept")
	}
}

// U-05: the removed commands exit non-zero with their message.
func TestRemovedCommandsExitUsage(t *testing.T) {
	for _, name := range []string{"deploy", "sync"} {
		c, _, err := rootCmd.Find([]string{name})
		if err != nil || c.Name() != name {
			t.Fatalf("%s: %v", name, err)
		}
		var errOut bytes.Buffer
		c.SetErr(&errOut)
		err = c.RunE(c, []string{"--check"})
		if ExitCode(err) != ExitUsage {
			t.Errorf("oh %s: code %d, want %d", name, ExitCode(err), ExitUsage)
		}
		if !strings.Contains(errOut.String(), "oh "+name) {
			t.Errorf("oh %s: message not printed: %q", name, errOut.String())
		}
		c.SetErr(nil)
	}
}

// U-05: an optional check that fails is a warning and keeps the exit code
// at 0; a mandatory one gives 1.
func TestDoctorOptionalChecksAreWarnings(t *testing.T) {
	optional := check{"docker", func() []views.DoctorCheck {
		return []views.DoctorCheck{{Detail: "engine stopped", Optional: true}}
	}}
	mandatory := check{"git", single(func() (string, bool) { return "missing", false })}
	var out bytes.Buffer
	if err := runDoctorChecks(&out, []check{optional}); ExitCode(err) != ExitOK {
		t.Fatalf("optional failure: code %d, out %s", ExitCode(err), out.String())
	}
	if !strings.Contains(out.String(), theme.IconWarning) {
		t.Errorf("an optional failure shows as a warning: %q", out.String())
	}
	out.Reset()
	if err := runDoctorChecks(&out, []check{optional, mandatory}); ExitCode(err) != ExitFailure {
		t.Fatalf("mandatory failure: code %d", ExitCode(err))
	}
}

// U-05: the container engine is optional unless execution.runtime is container.
func TestContainerRequired(t *testing.T) {
	a := &app.App{Config: &config.Config{}}
	if containerRequired(a) || containerRequired(nil) {
		t.Error("not required by default")
	}
	a.Config.Execution.Runtime = "container"
	if !containerRequired(a) {
		t.Error("required when the runtime is container")
	}
}

// U-05: a wrong command line is a usage error.
func TestUsageErrors(t *testing.T) {
	if ExitCode(usageError(errors.New(`unknown command "bogus" for "oh"`))) != ExitUsage {
		t.Error("unknown command")
	}
	if ExitCode(usageError(errors.New("boom"))) != ExitFailure {
		t.Error("another error stays a failure")
	}
	c, _, _ := rootCmd.Find([]string{"doctor"})
	flagErr := c.FlagErrorFunc()(c, errors.New("unknown flag: --bogus"))
	if ExitCode(flagErr) != ExitUsage {
		t.Errorf("flag error: code %d", ExitCode(flagErr))
	}
}
