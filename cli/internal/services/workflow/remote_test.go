package workflow

import (
	"errors"
	"testing"

	wf "github.com/datichb/openhub/cli/internal/workflow"
)

func remoteRes(t *testing.T, yaml, mode string) *Resolution {
	t.Helper()
	doc, diags := wf.Parse([]byte(yaml), wf.Source{Layer: wf.LayerHub, Path: "t.yaml"})
	if diags.HasErrors() {
		t.Fatalf("parse: %v", diags.Err())
	}
	return &Resolution{Resolved: &wf.Resolved{Spec: doc.Spec, Mode: mode}}
}

const remoteBase = `apiVersion: oh/v1
kind: Workflow
id: t
description: test
category: develop
risk: write
modes: { default: semi-auto, allowed: [manuel, semi-auto] }
agents:
  lead: { role: workflow }
entry: { agent: lead }
prompt: { text: "go" }
`

func TestRemotePlan(t *testing.T) {
	_, err := remoteRes(t, remoteBase, "semi-auto").RemotePlan("fr")
	if !errors.Is(err, ErrRemoteNotAllowed) {
		t.Fatalf("local only: %v", err)
	}

	y := remoteBase + `runtime: { default: local, allowed: [local, remote] }
checkpoints:
  cp-1: { label: { fr: Démarrer, en: Start }, mode: { manuel: pause, semi-auto: auto }, remote: forbid }
  cp-2: { label: Commit, mandatory: true, mode: { manuel: pause, semi-auto: pause }, remote: defer }
  cp-3: { label: Doc, mode: { manuel: pause, semi-auto: pause }, remote: auto }
`
	plan, err := remoteRes(t, y, "semi-auto").RemotePlan("en")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 || plan[0].ID != "cp-2" || plan[0].Policy != wf.RemoteDefer || !plan[0].Mandatory ||
		plan[1].ID != "cp-3" || plan[1].Policy != wf.RemoteAuto {
		t.Fatalf("plan = %+v", plan)
	}

	// In manuel, cp-1 pauses and is forbidden remotely.
	_, err = remoteRes(t, y, "manuel").RemotePlan("fr")
	var fe *RemoteForbiddenError
	if !errors.As(err, &fe) || fe.Checkpoint != "cp-1" {
		t.Fatalf("manuel: %v", err)
	}
}
