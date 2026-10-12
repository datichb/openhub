package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gateway"
	"github.com/datichb/openhub/cli/internal/gateway/bdargs"
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

// replayFlags are the options a replayed entry may carry (long names of bd,
// as bdargs reads them): any other option is refused (S-04), even one bd
// knows.
var replayFlags = map[string]bool{
	// global
	"json": true, "quiet": true, "verbose": true, "actor": true,
	// fields
	"status": true, "assignee": true, "title": true, "priority": true, "type": true, "label": true, "labels": true,
	"parent": true, "description": true, "notes": true, "append-notes": true, "reason": true, "design": true,
	"acceptance": true, "external-ref": true, "estimate": true, "deps": true, "add-label": true, "remove-label": true,
	"claim": true, "force": true, "all": true,
	// standard input only (job files are refused)
	"body-file": true, "design-file": true, "file": true, "output": true, "input": true, "stdin": true,
}

// idFlags hold ticket ids (comma separated, "type:id" for --deps).
var idFlags = map[string]bool{"parent": true, "deps": true}

// parseEntry reads a journal entry as bd does (the gateway's analyser,
// bdargs); reason says why it cannot be replayed.
func parseEntry(argv []string) (c bdargs.Cmd, reason string) {
	c, err := bdargs.Parse(argv)
	var pe *bdargs.Error
	switch {
	case err == nil:
	case errors.As(err, &pe) && pe.Kind == bdargs.UnknownFlag:
		return c, "unknown option " + pe.Token + " refused (" + bdargs.GeneratedFor + ")"
	case errors.As(err, &pe) && pe.Kind == bdargs.UnknownCommand:
		return c, "unknown command " + pe.Token + " refused (" + bdargs.GeneratedFor + ")"
	default:
		return c, "arguments refused: " + err.Error()
	}
	if c.Name() == "" || c.Help() {
		return c, "no command"
	}
	return c, ""
}

// ErrJournalUnauthenticated is returned when the envelope kept on the
// machine or the journal fetched from the job does not carry a valid seal
// of the session's journal key (S-04): nothing is replayed.
var ErrJournalUnauthenticated = errors.New("the Beads journal of the session is not authenticated")

// loadSealed reads the envelope kept on the machine and the fetched
// journal, and checks their seals with the journal key of the session
// (keychain of the machine).
func (s *Service) loadSealed(ctx context.Context, sid string) (*remote.Manifest, *remote.Snapshot, []beadswire.JournalEntry, error) {
	dir := s.RemoteDir(sid)
	read := func(name string) ([]byte, error) {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("envelope of the session (%s): %w", name, err)
		}
		return data, nil
	}
	mdata, err := read(remote.ManifestFile)
	if err != nil {
		return nil, nil, nil, err
	}
	sdata, err := read(remote.SnapshotFile)
	if err != nil {
		return nil, nil, nil, err
	}
	key := ""
	if s.Secrets != nil {
		key, _ = s.Secrets.Get(ctx, remote.JournalKeySecret(sid))
	}
	if key == "" {
		return nil, nil, nil, fmt.Errorf("%w: no journal key on this machine", ErrJournalUnauthenticated)
	}
	emac, _ := os.ReadFile(filepath.Join(dir, envelopeSealFile))
	if !remote.CheckSeal(key, "envelope", sid, strings.TrimSpace(string(emac)), mdata, sdata) {
		return nil, nil, nil, fmt.Errorf("%w: the envelope kept on the machine was modified", ErrJournalUnauthenticated)
	}
	var m remote.Manifest
	var snap remote.Snapshot
	if err := json.Unmarshal(mdata, &m); err != nil {
		return nil, nil, nil, fmt.Errorf("%s: %w", remote.ManifestFile, err)
	}
	if err := json.Unmarshal(sdata, &snap); err != nil {
		return nil, nil, nil, fmt.Errorf("%s: %w", remote.SnapshotFile, err)
	}
	jdata, err := os.ReadFile(filepath.Join(dir, remote.JournalFile))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil, err
	}
	var sum remote.Summary
	if data, err := os.ReadFile(filepath.Join(dir, remote.SummaryFile)); err == nil {
		_ = json.Unmarshal(data, &sum)
	}
	if !remote.CheckSeal(key, "journal", sid, sum.JournalMAC, jdata) {
		return nil, nil, nil, fmt.Errorf("%w: journal.jsonl does not match the seal of the job", ErrJournalUnauthenticated)
	}
	journal, err := parseJournal(jdata)
	if err != nil {
		return nil, nil, nil, err
	}
	return &m, &snap, journal, nil
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
	plan, _, err := s.planReplay(ctx, sid)
	return plan, err
}

func (s *Service) planReplay(ctx context.Context, sid string) (*ReplayPlan, []beadswire.JournalEntry, error) {
	ref, err := s.Remote.GetRemoteRef(ctx, sid)
	if err != nil {
		return nil, nil, err
	}
	if ref == nil {
		return nil, nil, ErrNotRemote
	}
	if ref.Status == domain.RemoteResolved {
		// Replayed already (the journal key is gone): the outcome only.
		st := s.loadReplayState(sid)
		plan := &ReplayPlan{SessionID: sid}
		seqs := make([]int, 0, len(st.Items))
		for seq := range st.Items {
			seqs = append(seqs, seq)
		}
		sort.Ints(seqs)
		for _, seq := range seqs {
			if it := st.Items[seq]; it.State != ItemPending && it.State != ItemFailed {
				plan.Items = append(plan.Items, it)
			}
		}
		return plan, nil, nil
	}
	man, snap, journal, err := s.loadSealed(ctx, sid)
	if err != nil {
		return nil, nil, err
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
	closing := s.closingStatuses(ctx, ref.ProjectDir)
	for _, e := range journal {
		it := classify(e, man.BeadsAllow, snap, placeholders, closing)
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
		return plan, journal, nil
	}
	ids := make([]string, 0, len(touched))
	for t := range touched {
		ids = append(ids, t)
	}
	sort.Strings(ids)
	recs, err := s.beads().Show(ctx, ref.ProjectDir, ids...)
	if err != nil {
		return nil, nil, fmt.Errorf("reading the tickets on the machine: %w", err)
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
	return plan, journal, nil
}

// closingStatuses asks bd on the machine, once, which statuses of the
// project's configuration close a ticket (unreadable: every status closes).
func (s *Service) closingStatuses(ctx context.Context, dir string) func(string) bool {
	var out []byte
	asked := false
	return func(status string) bool {
		if !asked {
			asked = true
			out, _ = s.beads().Exec(ctx, dir, []string{"statuses", "--json"}, nil)
		}
		return gateway.ClosingStatus(out, status)
	}
}

// classify checks an entry again on the machine. closing tells whether a
// status of the project's own configuration closes a ticket.
func classify(e beadswire.JournalEntry, allow []string, snap *remote.Snapshot, placeholders map[string]bool, closing func(status string) bool) ReplayItem {
	it := ReplayItem{Seq: e.Seq, Argv: e.Argv, Placeholder: e.Placeholder, State: ItemPending}
	refuse := func(why string) ReplayItem {
		it.State, it.Reason = ItemRefused, why
		return it
	}
	c, why := parseEntry(e.Argv)
	it.Command = c.Key()
	if why != "" {
		return refuse(why)
	}
	if !gateway.BeadsAllowed(c, allow) {
		return refuse("not allowed by the workflow (beads.allow)")
	}
	for _, f := range c.Flags {
		switch {
		case f.Name == "actor":
			// set by the Beads gateway of the job; rewritten on replay
		case gateway.ForbiddenFlag(f.Name):
			return refuse("option --" + f.Name + " refused")
		case f.Name == "id":
			return refuse("option --id refused (the machine chooses the id)")
		case gateway.IsFileFlag(f.Name, f.Value) && f.Explicit && f.Value != "-":
			return refuse("option --" + f.Name + " names a file of the job")
		case !replayFlags[f.Name]:
			return refuse("option --" + f.Name + " refused (not replayed)")
		}
	}
	if op, ok := gateway.TicketOp(c); ok && op.Kind == gateway.OpClose && (op.Status == "" || closing == nil || closing(op.Status)) &&
		!gateway.BeadsAllowed(bdargs.Cmd{Path: []string{"close"}}, allow) {
		return refuse("closing a ticket is not allowed by the workflow (beads.allow)")
	}
	ids, ok := ticketArgs(c)
	if !ok {
		return refuse("unexpected arguments")
	}
	session := func(id string) bool {
		_, inSnap := snap.Issues[id]
		return inSnap || placeholders[id]
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !session(id) {
			return refuse("ticket " + id + " is not a ticket of the session")
		}
		if !seen[id] {
			seen[id] = true
			it.Tickets = append(it.Tickets, id)
		}
	}
	return it
}

// ticketArgs returns every ticket id an entry names, by position (per
// command) and in the options that hold ids; ok is false when the
// arguments do not have the shape of the command.
func ticketArgs(c bdargs.Cmd) (ids []string, ok bool) {
	args := c.Args
	switch c.Key() {
	case "create", "q":
		if len(args) > 1 {
			return nil, false // the title is one argument
		}
	case "note", "comment", "comments add":
		if len(args) == 0 {
			return nil, false
		}
		ids = append(ids, args[0])
	case "comments":
		if len(args) != 1 {
			return nil, false
		}
		ids = append(ids, args[0])
	case "dep add", "dep remove":
		if len(args) < 2 {
			return nil, false
		}
		ids = append(ids, args...)
	case "label add", "label remove": // ids… label
		if len(args) < 2 {
			return nil, false
		}
		ids = append(ids, args[:len(args)-1]...)
	case "dep", "label":
		return nil, false
	default:
		ids = append(ids, args...)
	}
	for _, f := range c.Flags {
		if !idFlags[f.Name] {
			continue
		}
		for _, x := range strings.Split(f.Value, ",") {
			x = strings.TrimSpace(x)
			if _, id, typed := strings.Cut(x, ":"); typed {
				x = strings.TrimSpace(id)
			}
			if x == "" {
				return nil, false
			}
			ids = append(ids, x)
		}
	}
	return ids, true
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
		c2, why := parseEntry(e.Argv)
		if why != "" {
			continue
		}
		switch c2.Key() {
		case "close":
			after["status"] = "closed"
		case "reopen":
			after["status"] = "open"
		case "unclaim":
			after["status"], after["assignee"] = "open", ""
		case "update":
			if c2.Bool("claim") {
				after["status"] = "in_progress"
			}
			for _, f := range []string{"status", "assignee", "title"} {
				if v, ok := c2.Value(f); ok {
					after[f] = v
				}
			}
			for _, f := range []string{"notes", "append-notes"} {
				if v, ok := c2.Value(f); ok {
					c.Notes = append(c.Notes, v)
				}
			}
		case "note", "comment", "comments add":
			text := ""
			if len(c2.Args) > 1 {
				text = strings.Join(c2.Args[1:], " ")
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
func notesOnly(c bdargs.Cmd) *bdargs.Cmd {
	switch c.Key() {
	case "note", "comment", "comments add":
		return &c
	case "update":
		out := bdargs.Cmd{Path: c.Path, Args: c.Args}
		for _, f := range c.Flags {
			if f.Name == "notes" || f.Name == "append-notes" {
				out.Flags = append(out.Flags, f)
			}
		}
		if len(out.Flags) > 0 && len(out.Args) > 0 {
			return &out
		}
	}
	return nil
}

// ApplyReplay applies the pending entries (after the user's confirmation),
// with a resolution for each conflict (resolutions given here override the
// saved ones). Entries are applied in their order; created issues replace
// their placeholder in the following entries.
func (s *Service) ApplyReplay(ctx context.Context, sid string, res map[string]Resolution) (*ReplayPlan, error) {
	plan, journal, err := s.planReplay(ctx, sid)
	if err != nil {
		return nil, err
	}
	stdinOf := map[int][]byte{}
	for _, e := range journal {
		stdinOf[e.Seq] = e.Stdin
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
	if err != nil || ref == nil || ref.Status == domain.RemoteResolved {
		return plan, err
	}
	b := s.beads()
	human := gateway.ResolveHuman
	if s.Human != nil {
		human = s.Human
	}
	person := human(ctx, ref.ProjectDir)
	for i := range plan.Items {
		it := &plan.Items[i]
		if it.State != ItemPending {
			st.Items[it.Seq] = *it
			continue
		}
		// bd runs the canonical form of what was checked (bdargs).
		parsed, why := parseEntry(it.Argv)
		cmd := &parsed
		if why != "" {
			cmd = nil
		}
		for _, t := range it.Tickets {
			if cmd == nil {
				break
			}
			switch conflicting[t] {
			case KeepLocal:
				cmd = nil
			case MergeNotes:
				cmd = notesOnly(*cmd)
			}
		}
		if cmd == nil {
			it.State, it.Reason = ItemSkipped, "kept local"
			st.Items[it.Seq] = *it
			_ = s.saveReplayState(sid, st)
			continue
		}
		run := substitute(*cmd, st.Placeholders)
		run = replayActor(run, person)
		if it.Placeholder != "" && !run.Bool("json") {
			run.Flags = append(run.Flags, bdargs.Flag{Name: "json", Value: "true"})
		}
		argv := run.Argv()
		out, err := b.Exec(ctx, ref.ProjectDir, argv, stdinOf[it.Seq])
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
		if s.Secrets != nil {
			_ = s.Secrets.Delete(ctx, remote.JournalKeySecret(sid))
		}
	}
	return plan, nil
}

// substitute replaces the placeholders of created issues by their id, in
// the arguments and in the options that hold ids.
// replayActor sets the actor of a replayed write on the machine: the person
// of the machine and the oh agent recorded by the job, marked remote. A
// claim keeps bd's own actor (the person), since bd makes it the assignee.
func replayActor(c bdargs.Cmd, human string) bdargs.Cmd {
	agent := ""
	flags := make([]bdargs.Flag, 0, len(c.Flags)+1)
	for _, f := range c.Flags {
		if f.Name == "actor" {
			agent = gateway.ActorAgent(f.Value)
			continue
		}
		flags = append(flags, f)
	}
	c.Flags = flags
	if op, ok := gateway.TicketOp(c); ok && op.Kind == gateway.OpClaim {
		return c
	}
	if agent == "" {
		agent = "session"
	}
	c.Flags = append(c.Flags, bdargs.Flag{Name: "actor", Value: gateway.Actor(human, agent+" (remote)"), Explicit: true})
	return c
}

func substitute(c bdargs.Cmd, ph map[string]string) bdargs.Cmd {
	out := bdargs.Cmd{Path: c.Path, Args: make([]string, len(c.Args)), Flags: make([]bdargs.Flag, len(c.Flags))}
	for i, a := range c.Args {
		if id, ok := ph[a]; ok {
			a = id
		}
		out.Args[i] = a
	}
	for i, f := range c.Flags {
		if idFlags[f.Name] {
			parts := strings.Split(f.Value, ",")
			for j, x := range parts {
				typ, id, typed := strings.Cut(strings.TrimSpace(x), ":")
				if !typed {
					id, typ = typ, ""
				}
				if newID, ok := ph[strings.TrimSpace(id)]; ok {
					if typed {
						parts[j] = typ + ":" + newID
					} else {
						parts[j] = newID
					}
				}
			}
			f.Value = strings.Join(parts, ",")
		}
		out.Flags[i] = f
	}
	return out
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
