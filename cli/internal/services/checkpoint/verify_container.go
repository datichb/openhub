package checkpoint

import (
	"context"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Verification of a session that works in a container (fanout-container):
// it runs where the agents run, in a container of the image of the project
// with the location of the session mounted (for the merge checkpoint of a
// fan-out: the integration worktree), never on the machine instead. The
// state of the work (fingerprint, changes, scope) is still read by oh on
// the machine, where the repository is.

// ContainerExecFunc prepares the container where the verifications of sess
// run (its location, sess.LaunchPath, mounted) and returns how a command
// runs there, with what to call once they are done. An error: the
// verification cannot run (engine stopped, image not built…).
type ContainerExecFunc func(ctx context.Context, sess *domain.Session) (run ExecFunc, done func(), err error)

// inContainer reports whether the work of sess runs in a container.
func inContainer(sess *domain.Session) bool {
	return sessionspec.RuntimeKind(sess.Runtime) == sessionspec.RuntimeContainer
}

// containerVerifyEnv is the environment given to a verification in a
// container: the variables of the image (PATH, toolchains) and of the
// container (HOME), nothing from the machine.
var containerVerifyEnv = []string{"CI=true", "NO_COLOR=1", "TERM=dumb"}

// containerRunner returns how the commands of a verification of sess run in
// a container; ok false with the result to record when they cannot.
func (s *Service) containerRunner(ctx context.Context, sess *domain.Session) (run ExecFunc, done func(), failed domain.CheckpointVerify, ok bool) {
	if s.ContainerExec == nil {
		return nil, nil, containerUnavailable(i18n.T("tui.checkpoint.verify.container_none")), false
	}
	run, done, err := s.ContainerExec(ctx, sess)
	if err != nil {
		return nil, nil, containerUnavailable(err.Error()), false
	}
	if done == nil {
		done = func() {}
	}
	return run, done, domain.CheckpointVerify{}, true
}

// containerUnavailable is the result of a verification that cannot run in
// the container of its session.
func containerUnavailable(why string) domain.CheckpointVerify {
	return domain.CheckpointVerify{Status: domain.VerifyUnavailable, Reason: i18n.Tf("tui.checkpoint.verify.container_unavailable", why)}
}

// reusable reports whether a stored result can stand for the current state
// of the work: a verification that could not run is tried again (the
// engine may be back), one still running only when it outlived its
// timeout.
func (s *Service) reusable(r domain.CheckpointVerify, c sessionspec.CheckpointDef) bool {
	if r.Status == domain.VerifyUnavailable {
		return false
	}
	return r.Done() || !s.stale(r, c)
}
