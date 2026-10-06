package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gateway"
	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/remote"
)

// Replay of the Beads journal of a remote session (P5-T17): every write the
// job recorded is checked again (allow-list of the workflow, refused
// options, tickets of the session), compared with the current version of
// its ticket (revision of the snapshot sent), and applied on the machine
// only after confirmation. A ticket changed on the machine meanwhile is a
// conflict, resolved per ticket: keep local, apply remote, or merge notes.

// Resolution of a conflict.
type Resolution string

// Resolutions.
const (
	KeepLocal   Resolution = "keep_local"
	ApplyRemote Resolution = "apply_remote"
	MergeNotes  Resolution = "merge_notes"
)

// Valid reports a known resolution.
func (r Resolution) Valid() bool { return r == KeepLocal || r == ApplyRemote || r == MergeNotes }

// Item states.
const (
	ItemPending = "pending"
	ItemApplied = "applied"
	ItemSkipped = "skipped" // kept local
	ItemRefused = "refused" // not allowed (checked again on the machine)
	ItemFailed  = "failed"
)

// ReplayItem is one journal entry.
type ReplayItem struct {
	Seq         int      `json:"seq"`
	Argv        []string `json:"argv"`
	Command     string   `json:"command"`
	Tickets     []string `json:"tickets,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	State       string   `json:"state"`
	Reason      string   `json:"reason,omitempty"`
	Created     string   `json:"created,omitempty"` // real id of a created issue
}

// FieldChange is a field changed remotely on a conflicting ticket.
type FieldChange struct {
	Field    string `json:"field"`
	Snapshot string `json:"snapshot"`
	Local    string `json:"local"`
	Remote   string `json:"remote"`
}

// Conflict is a ticket changed on the machine since the session was sent,
// and written by the session.
type Conflict struct {
	Ticket     string        `json:"ticket"`
	Title      string        `json:"title,omitempty"`
	Fields     []FieldChange `json:"fields,omitempty"`
	Notes      []string      `json:"notes,omitempty"` // notes and comments added remotely
	Entries    []int         `json:"entries"`
	Resolution Resolution    `json:"resolution,omitempty"`
}

// ReplayPlan is the replay of a session journal.
type ReplayPlan struct {
	SessionID string       `json:"session_id"`
	Items     []ReplayItem `json:"items"`
	Conflicts []Conflict   `json:"conflicts,omitempty"`
}

// Pending counts the entries left to apply.
func (p *ReplayPlan) Pending() int {
	n := 0
	for _, it := range p.Items {
		if it.State == ItemPending {
			n++
		}
	}
	return n
}

// Unresolved lists the conflicts without resolution.
func (p *ReplayPlan) Unresolved() []string {
	var out []string
	for _, c := range p.Conflicts {
		if !c.Resolution.Valid() {
			out = append(out, c.Ticket)
		}
	}
	return out
}

// ErrUnresolved is returned when conflicts have no resolution.
var ErrUnresolved = errors.New("conflicts without resolution")

// replayState is kept in <remote dir>/replay.json (idempotent replay).
type replayState struct {
	Items        map[int]ReplayItem    `json:"items"`
	Placeholders map[string]string     `json:"placeholders,omitempty"`
	Resolutions  map[string]Resolution `json:"resolutions,omitempty"`
}

// refusedFlags change the database or directory of bd (gateway policy).
var refusedFlags = map[string]bool{"--db": true, "--database": true, "-C": true, "--directory": true, "--global": true,
	"--cpu-profile": true, "--mem-profile": true, "--dolt-auto-commit": true}

// fileFlags name job files, absent from the machine (stdin "-" excepted).
var fileFlags = map[string]bool{"--body-file": true, "--design-file": true, "--file": true, "-f": true, "--graph": true,
	"--output": true, "-o": true, "--input": true, "-i": true}

// valueFlags take a value.
var valueFlags = map[string]bool{
	"--status": true, "-s": true, "--assignee": true, "-a": true, "--title": true, "--priority": true, "-p": true,
	"--type": true, "-t": true, "--label": true, "-l": true, "--parent": true, "--description": true, "-d": true,
	"--notes": true, "--note": true, "--reason": true, "-r": true, "--actor": true, "--design": true,
	"--acceptance": true, "--external-ref": true, "--estimate": true, "--deps": true, "--id": true,
	"--body-file": true, "--design-file": true, "--file": true, "-f": true, "--output": true, "-o": true, "--input": true, "-i": true,
}

var idLikeRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*-[A-Za-z0-9.]+$`)

type bdArgs struct {
	cmd   string
	pos   []string
	flags map[string]string
}

func parseBd(argv []string) bdArgs {
	a := bdArgs{flags: map[string]string{}}
	for i := 0; i < len(argv); i++ {
		v := argv[i]
		if strings.HasPrefix(v, "-") && v != "-" {
			name, val, has := strings.Cut(v, "=")
			if !has && valueFlags[name] && i+1 < len(argv) {
				val = argv[i+1]
				i++
			}
			a.flags[name] = val
			continue
		}
		if a.cmd == "" {
			a.cmd = v
			continue
		}
		a.pos = append(a.pos, v)
	}
	return a
}

// allowed applies the workflow allow-list (one word, or "dep add").
func allowed(a bdArgs, allow []string) bool {
	if allow == nil {
		allow = gateway.DefaultBeadsAllow
	}
	two := a.cmd
	if len(a.pos) > 0 {
		two += " " + a.pos[0]
	}
	for _, x := range allow {
		if x == a.cmd || x == two {
			return true
		}
	}
	return false
}

func (s *Service) loadEnvelope(sid string) (*remote.Manifest, *remote.Snapshot, error) {
	dir := s.RemoteDir(sid)
	var m remote.Manifest
	var snap remote.Snapshot
	for name, v := range map[string]any{remote.ManifestFile: &m, remote.SnapshotFile: &snap} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, nil, fmt.Errorf("envelope of the session (%s): %w", name, err)
		}
		if err := json.Unmarshal(data, v); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", name, err)
		}
	}
	return &m, &snap, nil
}

func (s *Service) loadReplayState(sid string) replayState {
	st := replayState{Items: map[int]ReplayItem{}, Placeholders: map[string]string{}, Resolutions: map[string]Resolution{}}
	if data, err := os.ReadFile(filepath.Join(s.RemoteDir(sid), "replay.json")); err == nil {
		_ = json.Unmarshal(data, &st)
	}
	if st.Items == nil {
		st.Items = map[int]ReplayItem{}
	}
	if st.Placeholders == nil {
		st.Placeholders = map[string]string{}
	}
	if st.Resolutions == nil {
		st.Resolutions = map[string]Resolution{}
	}
	return st
}

func (s *Service) saveReplayState(sid string, st replayState) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.RemoteDir(sid), "replay.json"), data, 0o600)
}

// PlanReplay computes the replay of the journal of a fetched session: what
// will be applied, refused, and the conflicts to resolve.
func (s *Service) PlanReplay(ctx context.Context, sid string) (*ReplayPlan, error) {
	ref, err := s.Remote.GetRemoteRef(ctx, sid)
	if err != nil {
		return nil, err
	}
	if ref == nil {
		return nil, ErrNotRemote
	}
	man, snap, err := s.loadEnvelope(sid)
	if err != nil {
		return nil, err
	}
	journal, err := ReadJournal(filepath.Join(s.RemoteDir(sid), remote.JournalFile))
	if err != nil {
		return nil, err
	}
	st := s.loadReplayState(sid)
	plan := &ReplayPlan{SessionID: sid}
	placeholders := map[string]bool{}
	for _, e := range journal {
		if e.Placeholder != "" {
			placeholders[e.Placeholder] = true
		}
	}
	touched := map[string][]int{}
	for _, e := range journal {
		it := classify(e, man.BeadsAllow, snap, placeholders)
		if prev, ok := st.Items[e.Seq]; ok && prev.State != ItemPending && prev.State != ItemFailed {
			it.State, it.Reason, it.Created = prev.State, prev.Reason, prev.Created
		}
		if it.State == ItemPending {
			for _, t := range it.Tickets {
				if _, inSnap := snap.Issues[t]; inSnap {
					touched[t] = append(touched[t], it.Seq)
				}
			}
		}
		plan.Items = append(plan.Items, it)
	}
	if len(touched) == 0 {
		return plan, nil
	}
	ids := make([]string, 0, len(touched))
	for t := range touched {
		ids = append(ids, t)
	}
	sort.Strings(ids)
	recs, err := s.beads().Show(ctx, ref.ProjectDir, ids...)
	if err != nil {
		return nil, fmt.Errorf("reading the tickets on the machine: %w", err)
	}
	current := map[string]json.RawMessage{}
	for _, r := range recs {
		if h, err := headOf(r); err == nil {
			current[h.ID] = r
		}
	}
	for _, t := range ids {
		cur, ok := current[t]
		if !ok {
			continue
		}
		h, _ := headOf(cur)
		if h.version() == snap.Revisions[t] {
			continue
		}
		c := conflictOf(t, snap.Issues[t], cur, journal, touched[t])
		c.Resolution = st.Resolutions[t]
		plan.Conflicts = append(plan.Conflicts, c)
	}
	return plan, nil
}

// classify checks an entry again on the machine.
func classify(e beadswire.JournalEntry, allow []string, snap *remote.Snapshot, placeholders map[string]bool) ReplayItem {
	a := parseBd(e.Argv)
	it := ReplayItem{Seq: e.Seq, Argv: e.Argv, Command: a.cmd, Placeholder: e.Placeholder, State: ItemPending}
	refuse := func(why string) ReplayItem {
		it.State, it.Reason = ItemRefused, why
		return it
	}
	if a.cmd == "" {
		return refuse("no command")
	}
	if !allowed(a, allow) {
		return refuse("not allowed by the workflow (beads.allow)")
	}
	for f, v := range a.flags {
		if refusedFlags[f] || strings.HasPrefix(f, "-C") && len(f) > 2 {
			return refuse("option " + f + " refused")
		}
		if fileFlags[f] && v != "-" {
			return refuse("option " + f + " names a file of the job")
		}
	}
	for _, p := range a.pos {
		switch _, inSnap := snap.Issues[p]; {
		case inSnap, placeholders[p]:
			it.Tickets = append(it.Tickets, p)
		case idLikeRe.MatchString(p) && a.cmd != "create" && a.cmd != "q":
			return refuse("ticket " + p + " is not a ticket of the session")
		}
	}
	return it
}

func fieldOf(raw json.RawMessage, k string) string {
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	s, _ := m[k].(string)
	return s
}

// conflictOf compares the ticket at sending time, now, and after the
// session's writes.
func conflictOf(t string, snapRaw, curRaw json.RawMessage, journal []beadswire.JournalEntry, seqs []int) Conflict {
	c := Conflict{Ticket: t, Title: fieldOf(curRaw, "title"), Entries: seqs}
	after := map[string]string{}
	for _, k := range []string{"status", "assignee", "title"} {
		after[k] = fieldOf(snapRaw, k)
	}
	in := map[int]bool{}
	for _, s := range seqs {
		in[s] = true
	}
	for _, e := range journal {
		if !in[e.Seq] {
			continue
		}
		a := parseBd(e.Argv)
		switch a.cmd {
		case "close":
			after["status"] = "closed"
		case "reopen":
			after["status"] = "open"
		case "unclaim":
			after["status"], after["assignee"] = "open", ""
		case "update":
			if _, ok := a.flags["--claim"]; ok {
				after["status"] = "in_progress"
			}
			for _, f := range [][2]string{{"--status", "status"}, {"-s", "status"}, {"--assignee", "assignee"}, {"-a", "assignee"}, {"--title", "title"}} {
				if v, ok := a.flags[f[0]]; ok {
					after[f[1]] = v
				}
			}
			for _, f := range []string{"--notes", "--note"} {
				if v, ok := a.flags[f]; ok {
					c.Notes = append(c.Notes, v)
				}
			}
		case "note", "comment", "comments":
			text := strings.Join(a.pos[1:], " ")
			if len(a.pos) > 1 && a.pos[0] == "add" && len(a.pos) > 2 { // comments add <id> text
				text = strings.Join(a.pos[2:], " ")
			}
			if text == "" && e.Stdin != nil {
				text = string(e.Stdin)
			}
			if text != "" {
				c.Notes = append(c.Notes, text)
			}
		}
	}
	for _, k := range []string{"status", "assignee", "title"} {
		snapV, curV := fieldOf(snapRaw, k), fieldOf(curRaw, k)
		if after[k] != snapV {
			c.Fields = append(c.Fields, FieldChange{Field: k, Snapshot: snapV, Local: curV, Remote: after[k]})
		}
	}
	return c
}

// notesOnly reduces an entry to its notes and comments (MergeNotes), nil
// when it has none.
func notesOnly(argv []string) []string {
	a := parseBd(argv)
	switch a.cmd {
	case "note", "comment", "comments":
		return argv
	case "update":
		for _, f := range []string{"--notes", "--note"} {
			if v, ok := a.flags[f]; ok && len(a.pos) > 0 {
				return []string{"update", a.pos[0], f, v}
			}
		}
	}
	return nil
}

// ApplyReplay applies the pending entries (after the user's confirmation),
// with a resolution for each conflict (resolutions given here override the
// saved ones). Entries are applied in their order; created issues replace
// their placeholder in the following entries.
func (s *Service) ApplyReplay(ctx context.Context, sid string, res map[string]Resolution) (*ReplayPlan, error) {
	plan, err := s.PlanReplay(ctx, sid)
	if err != nil {
		return nil, err
	}
	st := s.loadReplayState(sid)
	for t, r := range res {
		if !r.Valid() {
			return nil, fmt.Errorf("unknown resolution %q for %s", r, t)
		}
		st.Resolutions[t] = r
	}
	conflicting := map[string]Resolution{}
	for i := range plan.Conflicts {
		plan.Conflicts[i].Resolution = st.Resolutions[plan.Conflicts[i].Ticket]
		conflicting[plan.Conflicts[i].Ticket] = plan.Conflicts[i].Resolution
	}
	if u := plan.Unresolved(); len(u) > 0 {
		return plan, fmt.Errorf("%w: %s", ErrUnresolved, strings.Join(u, ", "))
	}
	ref, err := s.Remote.GetRemoteRef(ctx, sid)
	if err != nil || ref == nil {
		return plan, err
	}
	b := s.beads()
	for i := range plan.Items {
		it := &plan.Items[i]
		if it.State != ItemPending {
			st.Items[it.Seq] = *it
			continue
		}
		argv := it.Argv
		for _, t := range it.Tickets {
			switch conflicting[t] {
			case KeepLocal:
				argv = nil
			case MergeNotes:
				argv = notesOnly(argv)
			}
		}
		if argv == nil {
			it.State, it.Reason = ItemSkipped, "kept local"
			st.Items[it.Seq] = *it
			_ = s.saveReplayState(sid, st)
			continue
		}
		argv = substitute(argv, st.Placeholders)
		if it.Placeholder != "" && !contains(argv, "--json") {
			argv = append(argv, "--json")
		}
		var stdin []byte
		if e := journalEntry(s.RemoteDir(sid), it.Seq); e != nil {
			stdin = e.Stdin
		}
		out, err := b.Exec(ctx, ref.ProjectDir, argv, stdin)
		if err != nil {
			it.State, it.Reason = ItemFailed, err.Error()
		} else {
			it.State, it.Reason = ItemApplied, ""
			if it.Placeholder != "" {
				if id := createdID(out); id != "" {
					st.Placeholders[it.Placeholder], it.Created = id, id
				}
			}
		}
		st.Items[it.Seq] = *it
		if err := s.saveReplayState(sid, st); err != nil {
			return plan, err
		}
	}
	if err := s.saveReplayState(sid, st); err != nil {
		return plan, err
	}
	failed := 0
	for _, it := range plan.Items {
		if it.State == ItemFailed || it.State == ItemPending {
			failed++
		}
	}
	if failed == 0 {
		ref.Status, ref.UpdatedAt = domain.RemoteResolved, s.now().UTC()
		if err := s.Remote.SetRemoteRef(ctx, sid, *ref); err != nil {
			return plan, err
		}
	}
	return plan, nil
}

func substitute(argv []string, ph map[string]string) []string {
	out := make([]string, len(argv))
	for i, a := range argv {
		if id, ok := ph[a]; ok {
			a = id
		}
		out[i] = a
	}
	return out
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// createdID reads the id printed by `bd create --json` (object or array).
func createdID(out []byte) string {
	var one struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(out, &one) == nil && one.ID != "" {
		return one.ID
	}
	var many []struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(out, &many) == nil && len(many) > 0 {
		return many[0].ID
	}
	return ""
}

func journalEntry(dir string, seq int) *beadswire.JournalEntry {
	j, err := ReadJournal(filepath.Join(dir, remote.JournalFile))
	if err != nil {
		return nil
	}
	for i := range j {
		if j[i].Seq == seq {
			return &j[i]
		}
	}
	return nil
}
