package cmd

import (
	"errors"
	"fmt"
	"strings"
)

// Exit codes of oh (U-05), documented in docs/reference/cli.*.md. Every code
// is defined here; no command writes a number of its own, except `oh beads`
// which passes the code of bd through.
const (
	ExitOK      = 0 // success
	ExitFailure = 1 // the command failed
	ExitUsage   = 2 // wrong usage: unknown command or option, removed command
	ExitPending = 3 // waiting: a session waits for a decision (checkpoint)
)

// ExitError ends oh with Code without printing anything more (the command
// already reported the failure, or a proxied command printed its own).
type ExitError struct{ Code int }

func (e *ExitError) Error() string { return fmt.Sprintf("exit status %d", e.Code) }

// CodedError is an error that is printed as usual and ends oh with Code.
type CodedError struct {
	Code int
	Err  error
}

func (e *CodedError) Error() string { return e.Err.Error() }
func (e *CodedError) Unwrap() error { return e.Err }

// withExitCode gives err the exit code code (nil stays nil).
func withExitCode(code int, err error) error {
	if err == nil {
		return nil
	}
	return &CodedError{Code: code, Err: err}
}

// ExitCode returns the process exit code for an error returned by Execute.
func ExitCode(err error) int {
	var exit *ExitError
	if errors.As(err, &exit) && exit.Code > 0 {
		return exit.Code
	}
	var coded *CodedError
	if errors.As(err, &coded) && coded.Code > 0 {
		return coded.Code
	}
	if err != nil {
		return ExitFailure
	}
	return ExitOK
}

// usageError marks the errors cobra raises for a wrong command line: an
// unknown command or option. Arguments refused by a command stay failures.
func usageError(err error) error {
	if err == nil {
		return nil
	}
	var coded *CodedError
	if errors.As(err, &coded) {
		return err
	}
	if strings.HasPrefix(err.Error(), "unknown command ") {
		return withExitCode(ExitUsage, err)
	}
	return err
}
