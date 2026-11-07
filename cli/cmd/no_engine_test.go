package cmd

import (
	"context"
	"errors"
	"os/exec"

	"github.com/datichb/openhub/cli/internal/runtime/container"
)

// noEngineRunner is the container engine tooling of a machine that has none.
// The tests of this package never ask the real engines of the machine they run
// on (colima, podman and docker answer in seconds, or not at all, and the
// result then depends on the machine): the detection of an engine finds
// nothing, at once. The tests of the detection itself are the ones of the
// package container, with its own fake runner.
type noEngineRunner struct{}

var errNoEngine = errors.New("no container engine in the tests of ./cmd")

func (noEngineRunner) LookPath(name string) (string, error) {
	return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
}

func (noEngineRunner) Run(context.Context, string, ...string) ([]byte, error) {
	return nil, errNoEngine
}

func (noEngineRunner) Stream(context.Context, func(string), string, ...string) error {
	return errNoEngine
}

var _ container.Runner = noEngineRunner{}

func init() { containerRunner = noEngineRunner{} }
