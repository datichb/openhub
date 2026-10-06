package ohruntime

import (
	"context"
	"time"
)

// PrepareEstimate tells what preparing a group would cost now (launch
// form: image cached, or build to expect).
type PrepareEstimate struct {
	// Ready is true when nothing has to be built.
	Ready bool
	// Image is the reference of the image used (when known).
	Image string
	// Steps lists what remains to build, in order (e.g. "base", "layer").
	Steps []string
	// Duration is the expected preparation time (0 = unknown: first build).
	Duration time.Duration
}

// Estimator is implemented by runtimes that can tell, without preparing
// anything, what a preparation would do.
type Estimator interface {
	Estimate(ctx context.Context, g Group) (PrepareEstimate, error)
}
