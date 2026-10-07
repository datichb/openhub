package workflow

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func decodeStrict(t *testing.T, data []byte) (*Spec, error) {
	t.Helper()
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var s Spec
	if err := dec.Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

func loadFullSpec(t *testing.T) *Spec {
	t.Helper()
	data, err := os.ReadFile("testdata/spec_v1_full.yaml")
	if err != nil {
		t.Fatal(err)
	}
	s, err := decodeStrict(t, data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return s
}

func TestSpec_DecodeFullExample(t *testing.T) {
	s := loadFullSpec(t)

	if s.APIVersion != APIVersionV1 || s.Kind != KindWorkflow || s.ID != "ticket" || s.Version != 3 {
		t.Fatalf("header = %q %q %q %d", s.APIVersion, s.Kind, s.ID, s.Version)
	}
	if !s.Category.Valid() || !s.Risk.Valid() || !s.Isolation.Valid() {
		t.Fatalf("enums: %q %q %q", s.Category, s.Risk, s.Isolation)
	}
	if s.CodeMode == nil || *s.CodeMode {
		t.Fatalf("code_mode = %v, want explicit false", s.CodeMode)
	}
	if got := s.EntryAgent(); got != "orchestrator-dev" {
		t.Fatalf("entry = %q", got)
	}
	if got := s.Description.Text("en"); got != "Implement a detailed Beads ticket" {
		t.Fatalf("description en = %q", got)
	}

	if pc, ok := s.Preconditions.Get("context"); !ok || pc.OnFail != OnFailSuggest || len(pc.Check.PathExists) != 2 {
		t.Fatalf("preconditions = %+v", s.Preconditions)
	}
	if got := s.Inputs.Keys(); !reflect.DeepEqual(got, []string{"ticket", "branch", "publish", "type"}) {
		t.Fatalf("input order = %v", got)
	}
	ticket, _ := s.Inputs.Get("ticket")
	if ticket.Type != InputBeadsID || !ticket.Required || ticket.Picker == nil || !ticket.Picker.Multi {
		t.Fatalf("ticket input = %+v", ticket)
	}
	enum, _ := s.Inputs.Get("type")
	if enum.Type != InputEnum || len(enum.Values) != 3 || enum.Default != "security" {
		t.Fatalf("enum input = %+v", enum)
	}

	if got := s.Agents.Keys(); !reflect.DeepEqual(got, []string{"orchestrator-dev", "developer", "reviewer", "documentarian"}) {
		t.Fatalf("agent order = %v", got)
	}
	od, _ := s.Agents.Get("orchestrator-dev")
	if od.Role != RoleWorkflow || len(od.Calls) != 3 {
		t.Fatalf("orchestrator-dev = %+v", od)
	}
	dev, _ := s.Agents.Get("developer")
	if dev.After != "cp-1" {
		t.Fatalf("developer.after = %q", dev.After)
	}

	if got := s.Checkpoints.Keys(); !reflect.DeepEqual(got, []string{"cp-1", "cp-2"}) {
		t.Fatalf("checkpoint order = %v", got)
	}
	cp2, _ := s.Checkpoints.Get("cp-2")
	if cp2.Mandatory == nil || !*cp2.Mandatory || cp2.Remote != RemoteDefer || cp2.Mode[ModeAuto] != BehaviorPause {
		t.Fatalf("cp-2 = %+v", cp2)
	}
	if got := cp2.Label.Text("en"); got != "Commit ou correction" {
		t.Fatalf("cp-2 label = %q", got)
	}

	if s.CircuitBreaker == nil || s.CircuitBreaker.MaxConsecutiveSubagents == nil || *s.CircuitBreaker.MaxConsecutiveSubagents != 12 {
		t.Fatalf("circuit breaker = %+v", s.CircuitBreaker)
	}
	if len(s.Plugins) != 2 || s.Plugins[0].ID != "context-mode" || s.Plugins[1].Options["verbose"] != true {
		t.Fatalf("plugins = %+v", s.Plugins)
	}
	if s.Runtime == nil || len(s.Runtime.Allowed) != 3 || !s.Runtime.Allowed[2].Valid() {
		t.Fatalf("runtime = %+v", s.Runtime)
	}
	if len(s.Outputs) != 3 || s.Outputs[1].Type != OutputMergeRequest {
		t.Fatalf("outputs = %+v", s.Outputs)
	}
	if s.Limits == nil || s.Limits.BudgetUSD == nil || *s.Limits.BudgetUSD != 5 {
		t.Fatalf("limits = %+v", s.Limits)
	}
}

func TestSpec_RoundTrip(t *testing.T) {
	s := loadFullSpec(t)
	out, err := yaml.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	again, err := decodeStrict(t, out)
	if err != nil {
		t.Fatalf("re-decode: %v\n%s", err, out)
	}
	if !reflect.DeepEqual(s, again) {
		t.Fatalf("round trip differs:\n%s", out)
	}
}

func TestSpec_StrictRejectsUnknownFields(t *testing.T) {
	cases := map[string]string{
		"top level":     "apiVersion: oh/v1\nkind: Workflow\nid: x\nfoo: 1\n",
		"inside input":  "apiVersion: oh/v1\nkind: Workflow\nid: x\ninputs:\n  a: { type: string, colour: red }\n",
		"inside agent":  "apiVersion: oh/v1\nkind: Workflow\nid: x\nagents:\n  dev: { role: workflow, rol: x }\n",
		"inside plugin": "apiVersion: oh/v1\nkind: Workflow\nid: x\nplugins:\n  - { id: p, opts: {} }\n",
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := decodeStrict(t, []byte(doc))
			if err == nil || !strings.Contains(err.Error(), "line ") {
				t.Fatalf("want an error with a line number, got %v", err)
			}
		})
	}
}

func TestSpec_RejectsDuplicateKeys(t *testing.T) {
	doc := "apiVersion: oh/v1\nkind: Workflow\nid: x\ncheckpoints:\n  cp-1: {}\n  cp-1: {}\n"
	if _, err := decodeStrict(t, []byte(doc)); err == nil {
		t.Fatal("duplicate checkpoint key accepted")
	}
}

func TestSpec_EntryDefaultsToConductor(t *testing.T) {
	s, err := decodeStrict(t, []byte("apiVersion: oh/v1\nkind: Workflow\nid: quick\n"))
	if err != nil {
		t.Fatal(err)
	}
	if s.EntryAgent() != DefaultEntryAgent {
		t.Fatalf("entry = %q", s.EntryAgent())
	}
	if s.Inputs.Len() != 0 || s.Agents.Keys() != nil {
		t.Fatal("empty maps expected")
	}
}

func TestLocalizedText_Fallback(t *testing.T) {
	plain := LocalizedText{Default: "Bonjour"}
	if plain.Text("en") != "Bonjour" {
		t.Fatal("plain form must be used for every language")
	}
	m := LocalizedText{ByLang: map[string]string{"fr": "Bonjour"}}
	if m.Text("en") != "Bonjour" || m.Text("fr") != "Bonjour" {
		t.Fatal("fallback to the available language expected")
	}
	if (LocalizedText{}).Text("fr") != "" || !(LocalizedText{}).IsZero() {
		t.Fatal("empty text expected")
	}
}

func TestOrderedMap_SetDelete(t *testing.T) {
	var m OrderedMap[int]
	m.Set("b", 1)
	m.Set("a", 2)
	m.Set("b", 3)
	m.Delete("a")
	m.Set("c", 4)
	if got := m.Keys(); !reflect.DeepEqual(got, []string{"b", "c"}) {
		t.Fatalf("keys = %v", got)
	}
	if v, _ := m.Get("b"); v != 3 {
		t.Fatalf("b = %d", v)
	}
}

func TestEnums_Rank(t *testing.T) {
	if RiskRead.Rank() >= RiskWrite.Rank() || RiskWrite.Rank() >= RiskPublish.Rank() {
		t.Fatal("risk ranks must be ordered")
	}
	if Risk("nope").Valid() || InputType("file").Valid() || RemotePolicy("x").Valid() {
		t.Fatal("unknown enum accepted")
	}
}
