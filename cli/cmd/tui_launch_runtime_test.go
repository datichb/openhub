package cmd

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/i18n"
	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	"github.com/datichb/openhub/cli/internal/runtime/container"
)

func TestRuntimeStatusText(t *testing.T) {
	av := ohruntime.Availability{OK: true, Engine: "colima", Version: "28.1", Details: []string{"mount=virtiofs"}}
	engine := i18n.Tf("tui.launch.runtime.engine", "colima 28.1 · mount=virtiofs")

	got := runtimeStatusText(av, ohruntime.PrepareEstimate{Ready: true, Image: "oh-dev/app:9f3c"}, true, nil)
	assert.Equal(t, engine+" · "+i18n.Tf("tui.launch.runtime.cached", "oh-dev/app:9f3c"), got)

	got = runtimeStatusText(av, ohruntime.PrepareEstimate{Steps: []string{container.StepBase, container.StepLayer}}, true, nil)
	assert.Contains(t, got, i18n.T("tui.launch.runtime.build_first"))

	got = runtimeStatusText(av, ohruntime.PrepareEstimate{Steps: []string{container.StepLayer}, Duration: 40 * time.Second}, true, nil)
	assert.Contains(t, got, i18n.Tf("tui.launch.runtime.layer_eta", "~40 s"))

	got = runtimeStatusText(av, ohruntime.PrepareEstimate{Steps: []string{container.StepBase, container.StepLayer}, Duration: 190 * time.Second}, true, nil)
	assert.Contains(t, got, "~3 min")

	got = runtimeStatusText(av, ohruntime.PrepareEstimate{}, false, errors.New("dev Dockerfile x not found"))
	assert.Contains(t, got, "not found")
	assert.Equal(t, engine, runtimeStatusText(av, ohruntime.PrepareEstimate{}, false, nil))
}

func TestApproxDuration(t *testing.T) {
	assert.Equal(t, "~5 s", approxDuration(time.Second))
	assert.Equal(t, "~45 s", approxDuration(44*time.Second))
	assert.Equal(t, "~2 min", approxDuration(100*time.Second))
}
