//go:build container

package container

import (
	"context"
	"testing"
)

// Real engines: `make test-container` (Colima and/or Podman running).
func TestRealDetect(t *testing.T) {
	for _, k := range []EngineKind{EngineColima, EnginePodman, EngineDocker} {
		t.Run(string(k), func(t *testing.T) {
			rt := New(Options{Engine: k})
			av, err := rt.Available(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if !av.OK {
				t.Skipf("%s unavailable: %s", k, av.Message())
			}
			e, _ := rt.Engine(context.Background())
			t.Logf("%s %s via %v, host %s, details %v", e.Kind, e.Version, e.Command(), e.Host, e.Details)
			if e.Version == "" || rt.HostAddress() == "" {
				t.Fatalf("incomplete engine: %+v", e)
			}
		})
	}
}
