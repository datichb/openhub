package workflow

import (
	"errors"
	"fmt"

	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// ErrRemoteNotAllowed is returned when the workflow does not list `remote`
// in runtime.allowed.
var ErrRemoteNotAllowed = errors.New("the workflow does not allow the remote runtime")

// RemoteForbiddenError is returned when a checkpoint waits for the user in
// the session mode and its remote policy is forbid.
type RemoteForbiddenError struct{ Checkpoint string }

func (e *RemoteForbiddenError) Error() string {
	return fmt.Sprintf("checkpoint %s waits for the user and is forbidden remotely (remote: forbid)", e.Checkpoint)
}

// RemoteCheckpoint is the remote handling of a checkpoint that waits for the
// user in the session mode.
type RemoteCheckpoint struct {
	ID        string          `json:"id"`
	Label     string          `json:"label,omitempty"`
	Policy    wf.RemotePolicy `json:"policy"` // auto | defer
	Mandatory bool            `json:"mandatory,omitempty"`
}

// RemotePlan checks that the resolution can run remotely (P5-T04): `remote`
// is allowed and no checkpoint that waits for the user in the mode is
// `remote: forbid`. It returns the checkpoints the policy responder of the
// job handles (in declaration order); the others pass by the mode.
func (r *Resolution) RemotePlan(lang string) ([]RemoteCheckpoint, error) {
	if !r.Spec.AllowsRuntime(wf.RuntimeRemote) {
		return nil, ErrRemoteNotAllowed
	}
	var out []RemoteCheckpoint
	for _, id := range r.WaitingCheckpoints() {
		cp, _ := r.Spec.Checkpoints.Get(id)
		pol := cp.RemotePolicy()
		if pol == wf.RemoteForbid {
			return nil, &RemoteForbiddenError{Checkpoint: id}
		}
		out = append(out, RemoteCheckpoint{ID: id, Label: cp.Label.Text(lang), Policy: pol, Mandatory: cp.IsMandatory()})
	}
	return out, nil
}
