package evalrun

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
)

// Assertion types.
const (
	AssertRan                  = "ran"                     // a shell command was seen (before an event)
	AssertCheckpointOrder      = "checkpoint_order"        // checkpoints reached in this order
	AssertNoQuestionInSubagent = "no_question_in_subagent" // no subagent asked the user with the question tool
	AssertSkillLoadedBefore    = "skill_loaded_before"     // a skill was loaded (before an event)
	AssertChangedWithin        = "changed_within"          // the clone changed only within paths
	AssertHiddenTests          = "hidden_tests"            // the hidden tests pass on the clone
	AssertNoDeniedLoop         = "no_denied_loop"          // a refused call is not retried in a loop
	AssertTextMatches          = "text_matches"            // an agent wrote (or never wrote) a text
)

// Assertion is a check of a case (`type:` and its parameters).
type Assertion struct {
	Type string `yaml:"type" json:"type"`
	// Name labels the assertion in reports (default: generated).
	Name string `yaml:"name" json:"name,omitempty"`

	// ran: Command is a shell pattern (`*` = any text), refused calls left
	// out; Absent inverts it (the command must not run before the anchor).
	Command string  `yaml:"command" json:"command,omitempty"`
	Absent  bool    `yaml:"absent" json:"absent,omitempty"`
	Before  *Anchor `yaml:"before" json:"before,omitempty"`
	// Agent restricts ran, skill_loaded_before and text_matches to an agent.
	Agent string `yaml:"agent" json:"agent,omitempty"`

	// checkpoint_order.
	Order []string `yaml:"order" json:"order,omitempty"`

	// no_question_in_subagent: Agents restricts the subagents checked
	// (default all); IncludeEntry also checks the entry agent.
	Agents       []string `yaml:"agents" json:"agents,omitempty"`
	IncludeEntry bool     `yaml:"include_entry" json:"include_entry,omitempty"`

	// skill_loaded_before.
	Skill string `yaml:"skill" json:"skill,omitempty"`

	// changed_within: Paths are globs (`**` = any directories); MaxFiles and
	// MinFiles bound the number of changed files.
	Paths    []string `yaml:"paths" json:"paths,omitempty"`
	MaxFiles int      `yaml:"max_files" json:"max_files,omitempty"`
	MinFiles int      `yaml:"min_files" json:"min_files,omitempty"`

	// no_denied_loop: a refused call (permission rule, permission
	// rejected, unknown skill) repeated more than MaxRepeats times (default
	// 2) fails.
	MaxRepeats int `yaml:"max_repeats" json:"max_repeats,omitempty"`

	// text_matches: Pattern is a regular expression (case-insensitive);
	// Questions also looks at the questions asked to the user (title,
	// fields and options).
	Pattern   string `yaml:"pattern" json:"pattern,omitempty"`
	Questions bool   `yaml:"questions" json:"questions,omitempty"`
}

// Anchor selects the first event of a kind in a trace.
type Anchor struct {
	Checkpoint       string `yaml:"checkpoint" json:"checkpoint,omitempty"`               // the checkpoint was reached (asked or passed)
	CheckpointPassed string `yaml:"checkpoint_passed" json:"checkpoint_passed,omitempty"` // the checkpoint was passed
	Subagent         string `yaml:"subagent" json:"subagent,omitempty"`                   // a delegation to this agent
	Tool             string `yaml:"tool" json:"tool,omitempty"`                           // a call of this tool (edit, write…)
	Text             string `yaml:"text" json:"text,omitempty"`                           // a text matching this expression
	Command          string `yaml:"command" json:"command,omitempty"`                     // a shell command matching this pattern
	Agent            string `yaml:"agent" json:"agent,omitempty"`                         // restricts Tool, Text and Command
	Arg              string `yaml:"arg" json:"arg,omitempty"`                             // restricts Tool: pattern of its path or command
}

func (a *Anchor) String() string {
	switch {
	case a == nil:
		return "the end"
	case a.Checkpoint != "":
		return "checkpoint " + a.Checkpoint
	case a.CheckpointPassed != "":
		return "checkpoint " + a.CheckpointPassed + " passed"
	case a.Subagent != "":
		return "delegation to " + a.Subagent
	case a.Tool != "" && a.Arg != "":
		return "first " + a.Tool + " " + a.Arg
	case a.Tool != "":
		return "first " + a.Tool
	case a.Text != "":
		return "text /" + a.Text + "/"
	case a.Command != "":
		return "command " + a.Command
	}
	return "?"
}

func (a *Anchor) validate() error {
	if a == nil {
		return nil
	}
	n := 0
	for _, s := range []string{a.Checkpoint, a.CheckpointPassed, a.Subagent, a.Tool, a.Text, a.Command} {
		if s != "" {
			n++
		}
	}
	if n != 1 {
		return errors.New("before: exactly one of checkpoint, checkpoint_passed, subagent, tool, text, command")
	}
	if a.Text != "" {
		if _, err := regexp.Compile("(?i)" + a.Text); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks the parameters of an assertion.
func (a Assertion) Validate() error {
	if err := a.Before.validate(); err != nil {
		return err
	}
	switch a.Type {
	case AssertRan:
		if a.Command == "" {
			return errors.New("command is required")
		}
	case AssertCheckpointOrder:
		if len(a.Order) < 1 {
			return errors.New("order lists the checkpoints")
		}
	case AssertNoQuestionInSubagent, AssertNoDeniedLoop, AssertHiddenTests:
	case AssertSkillLoadedBefore:
		if a.Skill == "" {
			return errors.New("skill is required")
		}
	case AssertChangedWithin:
		for _, p := range a.Paths {
			if _, err := path.Match(strings.ReplaceAll(p, "**", "*"), ""); err != nil {
				return fmt.Errorf("path %q: %w", p, err)
			}
		}
	case AssertTextMatches:
		if _, err := regexp.Compile("(?i)" + a.Pattern); err != nil || a.Pattern == "" {
			return fmt.Errorf("pattern: a regular expression (%v)", err)
		}
	default:
		return fmt.Errorf("unknown assertion type %q", a.Type)
	}
	return nil
}

// Label is the name of an assertion in reports.
func (a Assertion) Label() string {
	if a.Name != "" {
		return a.Name
	}
	switch a.Type {
	case AssertRan:
		s := "ran " + a.Command
		if a.Absent {
			s = "never ran " + a.Command
		}
		if a.Before != nil {
			s += " before " + a.Before.String()
		}
		return s
	case AssertCheckpointOrder:
		return "checkpoints " + strings.Join(a.Order, " → ")
	case AssertSkillLoadedBefore:
		s := "skill " + a.Skill
		if a.Before != nil {
			s += " before " + a.Before.String()
		}
		return s
	case AssertChangedWithin:
		if len(a.Paths) == 0 && a.MaxFiles == 0 && a.MinFiles == 0 {
			return "no file changed"
		}
		return "changed within " + strings.Join(a.Paths, ", ")
	case AssertTextMatches:
		if a.Absent {
			return "never wrote /" + a.Pattern + "/"
		}
		return "wrote /" + a.Pattern + "/"
	}
	return a.Type
}

// Evidence is what the assertions of a pass look at.
type Evidence struct {
	Events []sessionsvc.TraceEvent
	// Changed are the files of the clone changed since the fixture
	// (committed or not), relative to its root.
	Changed []string
	// HiddenTests runs the hidden tests on the clone (nil: not available).
	HiddenTests func() (ok bool, detail string, err error)
}

// AssertionResult is the outcome of one assertion on one pass.
type AssertionResult struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail,omitempty"`
}

// Check evaluates the assertions of a case.
func Check(asserts []Assertion, ev Evidence) []AssertionResult {
	out := make([]AssertionResult, len(asserts))
	for i, a := range asserts {
		pass, detail := check(a, ev)
		out[i] = AssertionResult{Type: a.Type, Name: a.Label(), Pass: pass, Detail: detail}
	}
	return out
}

func check(a Assertion, ev Evidence) (pass bool, detail string) {
	switch a.Type {
	case AssertRan:
		return checkRan(a, ev.Events)
	case AssertCheckpointOrder:
		return checkCheckpointOrder(a, ev.Events)
	case AssertNoQuestionInSubagent:
		return checkNoQuestion(a, ev.Events)
	case AssertSkillLoadedBefore:
		return checkSkillBefore(a, ev.Events)
	case AssertChangedWithin:
		return checkChangedWithin(a, ev.Changed)
	case AssertHiddenTests:
		if ev.HiddenTests == nil {
			return false, "hidden tests not available"
		}
		ok, detail, err := ev.HiddenTests()
		if err != nil {
			return false, err.Error()
		}
		return ok, detail
	case AssertNoDeniedLoop:
		return checkNoDeniedLoop(a, ev.Events)
	case AssertTextMatches:
		return checkText(a, ev.Events)
	}
	return false, "unknown assertion type " + a.Type
}

// anchorIndex returns the index of the first event matching the anchor
// (-1 when it never happened; len(events) for no anchor).
func anchorIndex(a *Anchor, evs []sessionsvc.TraceEvent) int {
	if a == nil {
		return len(evs)
	}
	var re *regexp.Regexp
	if a.Text != "" {
		re = regexp.MustCompile("(?i)" + a.Text)
	}
	for i, e := range evs {
		if a.Agent != "" && e.Agent != a.Agent && (a.Tool != "" || a.Text != "" || a.Command != "") {
			continue
		}
		switch {
		case a.Checkpoint != "" && reachesCheckpoint(e, a.Checkpoint):
			return i
		case a.CheckpointPassed != "" && e.Kind == sessionsvc.TraceCheckpoint && e.Checkpoint == a.CheckpointPassed && e.Event == "passed":
			return i
		case a.Subagent != "" && e.Kind == sessionsvc.TraceSubagent && e.Subagent == a.Subagent:
			return i
		case a.Tool != "" && (e.Kind == sessionsvc.TraceTool || e.Kind == sessionsvc.TraceSubagent || e.Kind == sessionsvc.TraceSkill) && e.Tool == a.Tool &&
			(a.Arg == "" || wildcard(a.Arg, mainArg(e))):
			return i
		case re != nil && e.Kind == sessionsvc.TraceText && re.MatchString(e.Text):
			return i
		case a.Command != "" && isShell(e) && shellMatch(a.Command, command(e)):
			return i
		}
	}
	return -1
}

// reachesCheckpoint: the checkpoint was asked (decision, timeline) or
// passed by the mode.
func reachesCheckpoint(e sessionsvc.TraceEvent, id string) bool {
	switch e.Kind {
	case sessionsvc.TraceDecision:
		return e.DecisionKind == "checkpoint" && e.Checkpoint == id
	case sessionsvc.TraceCheckpoint:
		return e.Checkpoint == id && (e.Event == "passed" || e.Event == "waiting" || e.Event == "refused")
	}
	return false
}

// mainArg is the path or command of a tool call.
func mainArg(e sessionsvc.TraceEvent) string {
	for _, k := range []string{"filePath", "path", "file", "command", "pattern"} {
		if v, ok := e.Input[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func isShell(e sessionsvc.TraceEvent) bool {
	return e.Kind == sessionsvc.TraceTool && e.Tool == "shell"
}

func command(e sessionsvc.TraceEvent) string {
	s, _ := e.Input["command"].(string)
	return strings.TrimSpace(s)
}

// shellMatch matches a shell pattern against a command or any of its
// chained parts (a && b; a | b).
func shellMatch(pattern, cmd string) bool {
	if wildcard(pattern, cmd) {
		return true
	}
	for _, part := range regexp.MustCompile(`&&|\|\||;|\||\n`).Split(cmd, -1) {
		if wildcard(pattern, strings.TrimSpace(part)) {
			return true
		}
	}
	return false
}

func wildcard(pattern, s string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == s
	}
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = s[len(parts[0]):]
	for i := 1; i < len(parts)-1; i++ {
		k := strings.Index(s, parts[i])
		if k < 0 {
			return false
		}
		s = s[k+len(parts[i]):]
	}
	return strings.HasSuffix(s, parts[len(parts)-1])
}

func checkRan(a Assertion, evs []sessionsvc.TraceEvent) (pass bool, detail string) {
	anchor := anchorIndex(a.Before, evs)
	limit := anchor
	if limit < 0 {
		limit = len(evs) // the anchor never happened: the whole trace
	}
	first := -1
	for i := 0; i < len(evs); i++ {
		e := evs[i]
		// A call refused by oh (permission rules) did not run.
		if isShell(e) && !e.Denied && (a.Agent == "" || e.Agent == a.Agent) && shellMatch(a.Command, command(e)) {
			first = i
			break
		}
	}
	seenBefore := first >= 0 && first < limit
	if a.Absent {
		if seenBefore {
			return false, fmt.Sprintf("ran %q (event %d) before %s", command(evs[first]), evs[first].Seq, a.Before.String())
		}
		return true, ""
	}
	switch {
	case first < 0:
		return false, "command never ran"
	case !seenBefore:
		return false, fmt.Sprintf("first run at event %d, after %s (event %d)", evs[first].Seq, a.Before.String(), evs[anchor].Seq)
	case anchor < 0:
		return true, a.Before.String() + " never happened"
	}
	return true, ""
}

func checkCheckpointOrder(a Assertion, evs []sessionsvc.TraceEvent) (pass bool, detail string) {
	last, lastID := -1, ""
	for _, id := range a.Order {
		at := -1
		for i, e := range evs {
			if reachesCheckpoint(e, id) {
				at = i
				break
			}
		}
		if at < 0 {
			return false, "checkpoint " + id + " never reached"
		}
		if at < last {
			return false, fmt.Sprintf("checkpoint %s reached before %s", id, lastID)
		}
		last, lastID = at, id
	}
	return true, ""
}

// agentsOf completes the agent of events that do not tell it (decisions of
// traces written before oh named them) with the last agent seen in their
// tool session.
func agentsOf(evs []sessionsvc.TraceEvent) []string {
	out := make([]string, len(evs))
	last := map[string]string{}
	for i, e := range evs {
		out[i] = e.Agent
		if e.Agent == "" {
			out[i] = last[e.ToolSession]
		} else if e.ToolSession != "" {
			last[e.ToolSession] = e.Agent
		}
	}
	return out
}

func checkNoQuestion(a Assertion, evs []sessionsvc.TraceEvent) (pass bool, detail string) {
	agents := agentsOf(evs)
	want := map[string]bool{}
	for _, ag := range a.Agents {
		want[ag] = true
	}
	var bad []string
	seen := map[string]bool{}
	for i, e := range evs {
		// The question tool call: its decision is the same question.
		asked := (e.Kind == sessionsvc.TraceTool && e.Tool == "question") || (e.Kind == sessionsvc.TraceDecision && e.DecisionKind == "question")
		if !asked || (!e.InSubagent && !a.IncludeEntry) {
			continue
		}
		who := agents[i]
		if len(want) > 0 && who != "" && !want[who] {
			continue
		}
		if who == "" {
			who = e.ToolSession
		}
		if e.Kind == sessionsvc.TraceDecision && seen[e.ToolSession] {
			continue
		}
		seen[e.ToolSession] = e.Kind == sessionsvc.TraceTool
		bad = append(bad, fmt.Sprintf("%s (event %d)", who, e.Seq))
	}
	if len(bad) > 0 {
		return false, "question asked by " + strings.Join(bad, ", ")
	}
	return true, ""
}

// skillMatches: skill ids may carry their family (developer/dev-standards).
func skillMatches(got, want string) bool {
	return got == want || strings.HasSuffix(got, "/"+want) || strings.HasSuffix(want, "/"+got)
}

func checkSkillBefore(a Assertion, evs []sessionsvc.TraceEvent) (pass bool, detail string) {
	anchor := anchorIndex(a.Before, evs)
	first := -1
	for i, e := range evs {
		if e.Kind == sessionsvc.TraceSkill && skillMatches(e.Skill, a.Skill) && (a.Agent == "" || e.Agent == a.Agent) {
			first = i
			break
		}
	}
	switch {
	case first < 0:
		return false, "skill never loaded"
	case anchor < 0:
		// The phase never came: loading the skill is enough.
		return true, a.Before.String() + " never happened"
	case first > anchor:
		return false, fmt.Sprintf("loaded at event %d, after %s (event %d)", evs[first].Seq, a.Before.String(), evs[anchor].Seq)
	}
	return true, ""
}

func checkChangedWithin(a Assertion, changed []string) (pass bool, detail string) {
	// Without paths, max_files or min_files: nothing may change.
	restrict := len(a.Paths) > 0 || (a.MaxFiles == 0 && a.MinFiles == 0)
	var outside []string
	for _, f := range changed {
		if !restrict {
			break
		}
		ok := false
		for _, p := range a.Paths {
			if globMatch(p, f) {
				ok = true
				break
			}
		}
		if !ok {
			outside = append(outside, f)
		}
	}
	switch {
	case len(outside) > 0:
		return false, "changed outside: " + strings.Join(outside, ", ")
	case a.MaxFiles > 0 && len(changed) > a.MaxFiles:
		return false, fmt.Sprintf("%d files changed (max %d)", len(changed), a.MaxFiles)
	case len(changed) < a.MinFiles:
		return false, fmt.Sprintf("%d files changed (min %d)", len(changed), a.MinFiles)
	}
	return true, fmt.Sprintf("%d file(s) changed", len(changed))
}

// globMatch matches a slash path against a glob where `**` spans
// directories.
func globMatch(pattern, name string) bool {
	if !strings.Contains(pattern, "**") {
		ok, _ := path.Match(pattern, name)
		return ok
	}
	i := strings.Index(pattern, "**")
	prefix, rest := pattern[:i], strings.TrimPrefix(pattern[i+2:], "/")
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	tail := strings.TrimPrefix(name, prefix)
	if rest == "" {
		return true
	}
	segs := strings.Split(tail, "/")
	for k := range segs {
		if globMatch(rest, strings.Join(segs[k:], "/")) {
			return true
		}
	}
	return false
}

func checkNoDeniedLoop(a Assertion, evs []sessionsvc.TraceEvent) (pass bool, detail string) {
	limit := a.MaxRepeats
	if limit <= 0 {
		limit = 2
	}
	counts := map[string]int{}
	for _, e := range evs {
		denied := e.Denied || (e.Granted != nil && !*e.Granted) || (e.Kind == sessionsvc.TraceSkill && e.Status == "failed")
		if !denied {
			continue
		}
		key := e.ToolSession + "\x00" + e.Tool + e.Action + "\x00" + inputKey(e) + e.Skill
		counts[key]++
	}
	var loops []string
	for k, n := range counts {
		if n > limit {
			parts := strings.SplitN(k, "\x00", 3)
			loops = append(loops, fmt.Sprintf("%s %q ×%d", parts[1], parts[2], n))
		}
	}
	sort.Strings(loops)
	if len(loops) > 0 {
		return false, "refused call retried: " + strings.Join(loops, "; ")
	}
	return true, ""
}

func inputKey(e sessionsvc.TraceEvent) string {
	if c := command(e); c != "" {
		return c
	}
	if len(e.Resources) > 0 {
		return strings.Join(e.Resources, " ")
	}
	for _, k := range []string{"filePath", "path", "agent", "name"} {
		if v, ok := e.Input[k].(string); ok {
			return v
		}
	}
	return ""
}

func checkText(a Assertion, evs []sessionsvc.TraceEvent) (pass bool, detail string) {
	re := regexp.MustCompile("(?i)" + a.Pattern)
	agents := agentsOf(evs)
	for i, e := range evs {
		text := ""
		switch {
		case e.Kind == sessionsvc.TraceText && (a.Agent == "" || e.Agent == a.Agent):
			text = e.Text
		case a.Questions && e.Kind == sessionsvc.TraceDecision && e.DecisionKind == "question" &&
			(a.Agent == "" || agents[i] == a.Agent || (agents[i] == "" && !e.InSubagent)):
			data, _ := json.Marshal(e.Data["fields"])
			text = e.Title + " " + e.Text + " " + string(data)
		case a.Questions && e.Kind == sessionsvc.TraceTool && e.Tool == "question" && (a.Agent == "" || e.Agent == a.Agent):
			data, _ := json.Marshal(e.Input)
			text = string(data)
		default:
			continue
		}
		if re.MatchString(text) {
			if a.Absent {
				return false, fmt.Sprintf("written by %s (event %d)", e.Agent, e.Seq)
			}
			return true, ""
		}
	}
	if a.Absent {
		return true, ""
	}
	return false, "never written"
}
