package evalrun

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/workflow"
	"github.com/datichb/openhub/cli/internal/workflow/hubcat"
)

// CheckHub checks the hub of a variant before any pass runs: the skill
// catalogue (oh skill check, errors only), then each workflow validated
// against the hub and its bundle composed, as a session launch does.
func CheckHub(hub string, workflows []string) error {
	var errs []string
	probs, err := bundle.CheckSkills(hub)
	if err != nil {
		return err
	}
	for _, p := range probs {
		if p.Error {
			errs = append(errs, fmt.Sprintf("skill %s: %s %s", p.Ref, p.Kind, p.Detail))
		}
	}
	cat, diags := hubcat.LoadWorkflows(hub)
	if err := diags.Err(); err != nil {
		errs = append(errs, err.Error())
	}
	hc, err := hubcat.New(hub)
	if err != nil {
		return err
	}
	env := hc.Env()
	env.Skills = bundle.NewSkillCatalog(hub)
	out, err := os.MkdirTemp("", "ohe-check-")
	if err != nil {
		return err
	}
	defer func() { _ = RemoveAll(out) }()
	seen := map[string]bool{}
	sorted := append([]string(nil), workflows...)
	sort.Strings(sorted)
	for _, id := range sorted {
		if seen[id] {
			continue
		}
		seen[id] = true
		r, diags := workflow.Check(cat, workflow.Ref{Layer: workflow.LayerHub, ID: id}, nil, env)
		if err := diags.Err(); err != nil {
			errs = append(errs, "workflow "+id+": "+err.Error())
			continue
		}
		if _, err := bundle.Build(bundle.Request{HubDir: hub, OutDir: out, Spec: r.Spec}); err != nil {
			errs = append(errs, "workflow "+id+": bundle: "+err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}
