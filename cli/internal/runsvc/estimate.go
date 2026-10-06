package runsvc

import (
	"context"

	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// PrepareEstimate tells what preparing the runtime of a request would cost
// now (launch form: image cached or build to expect), without preparing
// anything. ok is false when the runtime cannot tell (local, no estimator).
func (s *Service) PrepareEstimate(ctx context.Context, req StartRequest) (est ohruntime.PrepareEstimate, ok bool, err error) {
	kind := runtimeKind(req.Runtime)
	if kind == sessionspec.RuntimeLocal {
		return est, false, nil
	}
	rt, err := s.runtime(kind)
	if err != nil {
		return est, false, err
	}
	e, isEst := rt.(ohruntime.Estimator)
	if !isEst {
		return est, false, nil
	}
	est, err = e.Estimate(ctx, s.runtimeGroup(req, sessionspec.GroupKey{}, ""))
	return est, err == nil, err
}
