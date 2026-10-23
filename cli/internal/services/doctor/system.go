package doctor

import (
	"context"
	"os/exec"
	"runtime"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// The checks of the machine: they depend on nothing of oh.

// OS reports the operating system and architecture.
func OS() Check {
	return Check{ID: "os", Name: i18n.T("cmd.doctor.check.os"), Run: func(context.Context) []Result {
		return []Result{{Detail: runtime.GOOS + "/" + runtime.GOARCH, OK: true}}
	}}
}

// GoRuntime reports the Go version oh was built with.
func GoRuntime() Check {
	return Check{ID: "go", Name: i18n.T("cmd.doctor.check.go"), Run: func(context.Context) []Result {
		return []Result{{Detail: runtime.Version(), OK: true}}
	}}
}

// Binary requires a program in the PATH and reports its version.
func Binary(id, name string) Check {
	return Check{ID: id, Name: name, Run: func(context.Context) []Result {
		path, err := exec.LookPath(name)
		if err != nil {
			return []Result{{Detail: i18n.T("cmd.doctor.not_found")}}
		}
		return []Result{{Detail: versionLine(path), OK: true}}
	}}
}

// OptionalBinary recommends a program: when missing, the check passes with
// an install hint.
func OptionalBinary(id, name, label, installHint string) Check {
	return Check{ID: id, Name: label, Run: func(context.Context) []Result {
		path, err := exec.LookPath(name)
		if err != nil {
			return []Result{{Detail: i18n.Tf("cmd.doctor.optional_missing", installHint), OK: true}}
		}
		return []Result{{Detail: versionLine(path), OK: true}}
	}}
}

// versionLine is the first line printed by `path --version`, or the path
// when the program does not tell.
func versionLine(path string) string {
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return path
	}
	version := string(out)
	if i := strings.IndexByte(version, '\n'); i > 0 {
		version = version[:i]
	}
	return version
}
