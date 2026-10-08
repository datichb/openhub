package cmd

import (
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/workflow"
)

func parseTestSpec(t *testing.T, yaml string) *workflow.Spec {
	t.Helper()
	doc, diags := workflow.Parse([]byte(yaml), workflow.Source{Layer: workflow.LayerHub, Path: "t.yaml"})
	if doc == nil || len(diags.Errors()) > 0 {
		t.Fatalf("parse: %v", diags)
	}
	return doc.Spec
}

const headlessSpecBase = `apiVersion: oh/v1
kind: Workflow
id: wf
category: develop
description: test
risk: write
prompt: { text: "go" }
agents:
  conductor: { role: workflow, mode: primary }
`

// A25: a mandatory checkpoint that pauses in every mode is reported as
// such (no "choose another mode"); otherwise the modes that work are named.
func TestHeadlessCheckpointsMessage(t *testing.T) {
	useLocale(t, "en")
	always := parseTestSpec(t, headlessSpecBase+`checkpoints:
  cp-2:
    label: Commit
    mandatory: true
    mode: { manuel: pause, semi-auto: pause, auto: pause }
`)
	err := headlessCheckpointsError(always, "semi-auto")
	if err == nil || strings.Contains(err.Error(), "another mode") || !strings.Contains(err.Error(), "every mode") || !strings.Contains(err.Error(), "cp-2") {
		t.Fatalf("always pausing: %v", err)
	}

	some := parseTestSpec(t, headlessSpecBase+`checkpoints:
  cp-1:
    label: Start
    mode: { manuel: pause, semi-auto: pause, auto: auto }
`)
	err = headlessCheckpointsError(some, "manuel")
	if err == nil || !strings.Contains(err.Error(), "auto") || strings.Contains(err.Error(), "every mode") {
		t.Fatalf("auto passes: %v", err)
	}
	if err := headlessCheckpointsError(some, "auto"); err != nil {
		t.Fatalf("auto mode: %v", err)
	}
}
