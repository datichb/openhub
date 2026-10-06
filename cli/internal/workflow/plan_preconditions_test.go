package workflow

import (
	"reflect"
	"testing"
	"testing/fstest"
)

const hubPlan = vHeader + `risk: plan
entry: { agent: auditor }
beads: { allow: [show, create] }
preconditions:
  wiki:
    label: { fr: Pas de wiki, en: No wiki }
    check: { path_exists: [docs/wiki/index.md, ONBOARDING.md] }
    suggest: { workflow: other, resume: true }
  config:
    check: { path_exists: [oh.toml] }
    on_fail: block
`

func TestRiskPlan(t *testing.T) {
	cat := catalogOf(t, map[string]string{"hub:x": hubPlan, "hub:other": "apiVersion: oh/v1\nkind: Workflow\nid: other\nrisk: write\n"})
	_, diags := Check(cat, Ref{LayerHub, "x"}, nil, testEnv())
	if len(diags) != 0 {
		t.Fatalf("plan workflow: %v", diags)
	}
	if !(RiskRead.Rank() < RiskPlan.Rank() && RiskPlan.Rank() < RiskWrite.Rank()) {
		t.Fatal("plan must rank between read and write")
	}
	for _, c := range []struct {
		risk  string
		codes []string
	}{
		{"write", []string{"loosening"}},
		{"read", nil},
	} {
		cat := catalogOf(t, map[string]string{
			"hub:x":     hubPlan,
			"hub:other": "apiVersion: oh/v1\nkind: Workflow\nid: other\nrisk: write\n",
			"team:x":    "apiVersion: oh/v1\nkind: Workflow\nid: x\nextends: hub:x\nrisk: " + c.risk + "\n",
		})
		r, diags := ResolveSpec(cat, Ref{LayerTeam, "x"}, nil)
		if got := diags.Codes(); len(got)+len(c.codes) > 0 && !reflect.DeepEqual(got, c.codes) {
			t.Fatalf("risk %s: codes = %v", c.risk, got)
		}
		if c.codes == nil && r.Spec.Risk != RiskRead {
			t.Fatalf("hardened risk = %s", r.Spec.Risk)
		}
	}
}

func TestPreconditionsPatch(t *testing.T) {
	cat := catalogOf(t, map[string]string{
		"hub:x":     hubPlan,
		"hub:other": "apiVersion: oh/v1\nkind: Workflow\nid: other\nrisk: write\n",
		"team:x": `apiVersion: oh/v1
kind: Workflow
id: x
extends: hub:x
preconditions:
  wiki: { check: { path_exists: [docs/README.md] } }
  config: { disabled: true }
  ghost: { disabled: true }
  extra: { check: { path_exists: [Makefile] } }
`,
	})
	r, diags := ResolveSpec(cat, Ref{LayerTeam, "x"}, nil)
	if got := diags.Codes(); !reflect.DeepEqual(got, []string{"patch_unknown_precondition"}) {
		t.Fatalf("codes = %v", got)
	}
	if got := r.Spec.Preconditions.Keys(); !reflect.DeepEqual(got, []string{"wiki", "extra"}) {
		t.Fatalf("keys = %v", got)
	}
	wiki, _ := r.Spec.Preconditions.Get("wiki")
	if !reflect.DeepEqual(wiki.Check.PathExists, []string{"docs/README.md"}) || wiki.Suggest == nil || wiki.Suggest.Workflow != "other" {
		t.Fatalf("wiki merged = %+v", wiki)
	}
	hub, _ := cat.Lookup(Ref{LayerHub, "x"})
	if hubWiki, _ := hub.Spec.Preconditions.Get("wiki"); len(hubWiki.Check.PathExists) != 2 {
		t.Fatal("parent document modified")
	}
}

func TestEvaluatePreconditions(t *testing.T) {
	cat := catalogOf(t, map[string]string{"hub:x": hubPlan})
	doc, _ := cat.Lookup(Ref{LayerHub, "x"})
	got := EvaluatePreconditions(doc.Spec, fstest.MapFS{"ONBOARDING.md": {Data: []byte("x")}})
	if len(got) != 2 || !got[0].OK || got[1].OK || !got[1].Block || got[0].Suggest != "other" || !got[0].Resume {
		t.Fatalf("results = %+v", got)
	}
	failed := FailedPreconditions(got)
	if len(failed) != 1 || failed[0].ID != "config" {
		t.Fatalf("failed = %+v", failed)
	}
	got = EvaluatePreconditions(doc.Spec, fstest.MapFS{"docs/wiki/index.md": {}, "oh.toml": {}})
	if len(FailedPreconditions(got)) != 0 {
		t.Fatalf("results = %+v", got)
	}
	if got[0].Label.Text("en") != "No wiki" {
		t.Fatalf("label = %q", got[0].Label.Text("en"))
	}
}
