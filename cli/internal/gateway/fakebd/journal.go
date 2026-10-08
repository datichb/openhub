package fakebd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/remote"
)

// journalBackend is the remote mode (P5-T12): Beads stays on the machine, so
// the job reads from the snapshot sent with the session and records every
// other command in the journal, replayed on the machine when the session is
// fetched (P5-T17). In the oh runner the allow-list (beads.allow) is applied
// before, by the Beads gateway of the job's oh daemon, which runs this
// binary as its bd.
//
// Reads: show, list, ready, children, search, count. Writes are recorded and
// applied to the snapshot copy (status, assignee) so that later reads see
// them; `create` answers a placeholder id.
type journalBackend struct {
	snapshot, journal string
	now               func() time.Time
}

// readCommands are answered from the snapshot.
var readCommands = map[string]bool{"show": true, "list": true, "ready": true, "children": true, "search": true, "count": true}

// flagsWithValue take a value (`--status x` or `--status=x`).
var flagsWithValue = map[string]bool{
	"--status": true, "-s": true, "--assignee": true, "-a": true, "--title": true, "--priority": true, "-p": true,
	"--type": true, "-t": true, "--label": true, "-l": true, "--parent": true, "--description": true, "-d": true,
	"--notes": true, "--note": true, "--reason": true, "-r": true, "--actor": true, "--design": true,
	"--acceptance": true, "--body-file": true, "--design-file": true, "--file": true, "-f": true, "--external-ref": true,
	"--limit": true, "-n": true, "--estimate": true, "--deps": true, "--id": true,
}

// parsed is a bd command line.
type parsed struct {
	cmd   string
	pos   []string // positional arguments after the command
	flags map[string]string
	json  bool
}

func parseArgs(argv []string) parsed {
	p := parsed{flags: map[string]string{}}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if a == "--json" {
			p.json = true
			continue
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			name, val, has := strings.Cut(a, "=")
			if !has && flagsWithValue[name] && i+1 < len(argv) {
				val = argv[i+1]
				i++
			}
			p.flags[name] = val
			continue
		}
		if p.cmd == "" {
			p.cmd = a
			continue
		}
		p.pos = append(p.pos, a)
	}
	return p
}

func (p parsed) flag(names ...string) (string, bool) {
	for _, n := range names {
		if v, ok := p.flags[n]; ok {
			return v, true
		}
	}
	return "", false
}

func (j journalBackend) Exec(req beadswire.ExecRequest) (beadswire.ExecResponse, error) {
	if j.snapshot == "" || j.journal == "" {
		return beadswire.ExecResponse{}, fmt.Errorf("the journal mode needs %s and %s", beadswire.EnvSnapshot, beadswire.EnvJournal)
	}
	p := parseArgs(req.Argv)
	switch {
	case p.cmd == "" || p.cmd == "help" || has(req.Argv, "--help", "-h"):
		return text("bd (oh remote session): Beads stays on your machine. Reads come from the snapshot sent with the session " +
			"(show, list, ready, children, search, count); other commands are recorded and applied when the session is fetched.\n"), nil
	case has(req.Argv, "--version", "-V") || p.cmd == "version":
		return text("bd (oh journal mode)\n"), nil
	case req.GitHook && p.cmd == "hooks":
		return beadswire.ExecResponse{}, nil // Beads git hooks: nothing to sync in a remote run
	}
	unlock, err := lockFile(j.journal + ".lock")
	if err != nil {
		return beadswire.ExecResponse{}, err
	}
	defer unlock()
	snap, err := j.load()
	if err != nil {
		return beadswire.ExecResponse{}, err
	}
	if readCommands[p.cmd] {
		return j.read(snap, p)
	}
	return j.write(snap, p, req)
}

func has(argv []string, names ...string) bool {
	for _, a := range argv {
		for _, n := range names {
			if a == n {
				return true
			}
		}
	}
	return false
}

func text(s string) beadswire.ExecResponse { return beadswire.ExecResponse{Stdout: []byte(s)} }

func fail(msg string) beadswire.ExecResponse {
	return beadswire.ExecResponse{Stderr: []byte(msg + "\n"), ExitCode: 1}
}

func (j journalBackend) load() (*remote.Snapshot, error) {
	data, err := os.ReadFile(j.snapshot)
	if err != nil {
		return nil, fmt.Errorf("reading the Beads snapshot: %w", err)
	}
	var s remote.Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("decoding the Beads snapshot: %w", err)
	}
	if s.Issues == nil {
		s.Issues = map[string]json.RawMessage{}
	}
	return &s, nil
}

func (j journalBackend) save(s *remote.Snapshot) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := j.snapshot + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, j.snapshot)
}

// issue is a decoded snapshot record (all fields kept).
type issue map[string]any

func (i issue) str(k string) string { s, _ := i[k].(string); return s }

func decode(raw json.RawMessage) issue {
	var m issue
	_ = json.Unmarshal(raw, &m)
	return m
}

func sortedIDs(s *remote.Snapshot) []string {
	ids := make([]string, 0, len(s.Issues))
	for id := range s.Issues {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func render(p parsed, list []issue) beadswire.ExecResponse {
	if p.json {
		if list == nil {
			list = []issue{}
		}
		data, _ := json.MarshalIndent(list, "", "  ")
		return text(string(data) + "\n")
	}
	var b strings.Builder
	for _, i := range list {
		fmt.Fprintf(&b, "%s [%s] %s\n", i.str("id"), i.str("status"), i.str("title"))
		if p.cmd == "show" {
			for _, k := range []string{"description", "acceptance_criteria", "design", "notes"} {
				if v := i.str(k); v != "" {
					fmt.Fprintf(&b, "\n%s:\n%s\n", strings.ToUpper(k[:1])+strings.ReplaceAll(k[1:], "_", " "), v)
				}
			}
		}
	}
	return text(b.String())
}

func (j journalBackend) read(s *remote.Snapshot, p parsed) (beadswire.ExecResponse, error) {
	var out []issue
	switch p.cmd {
	case "show":
		if len(p.pos) == 0 {
			return fail("bd show: an issue id is required"), nil
		}
		for _, id := range p.pos {
			raw, ok := s.Issues[id]
			if !ok {
				return fail(fmt.Sprintf("bd show: %s is not in the snapshot of this remote session (only the tickets of the session, their dependencies and children are available)", id)), nil
			}
			out = append(out, decode(raw))
		}
	case "children":
		if len(p.pos) == 0 {
			return fail("bd children: an issue id is required"), nil
		}
		for _, id := range s.Children[p.pos[0]] {
			if raw, ok := s.Issues[id]; ok {
				out = append(out, decode(raw))
			}
		}
	case "list", "search", "count":
		status, _ := p.flag("--status", "-s")
		query := strings.ToLower(strings.Join(p.pos, " "))
		for _, id := range sortedIDs(s) {
			i := decode(s.Issues[id])
			if status != "" && i.str("status") != status {
				continue
			}
			if status == "" && p.cmd == "list" && i.str("status") == "closed" && !has(keys(p.flags), "--all") {
				continue
			}
			if query != "" && !strings.Contains(strings.ToLower(i.str("title")+" "+i.str("description")), query) {
				continue
			}
			out = append(out, i)
		}
		if p.cmd == "count" {
			if p.json {
				return text(fmt.Sprintf("{\"count\": %d}\n", len(out))), nil
			}
			return text(fmt.Sprintf("%d\n", len(out))), nil
		}
	case "ready":
		for _, id := range sortedIDs(s) {
			i := decode(s.Issues[id])
			if i.str("status") != "open" || blocked(s, i) {
				continue
			}
			out = append(out, i)
		}
	}
	return render(p, out), nil
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// blocked reports an open blocker in the snapshot.
func blocked(s *remote.Snapshot, i issue) bool {
	deps, _ := i["dependencies"].([]any)
	for _, d := range deps {
		dm, _ := d.(map[string]any)
		if t, _ := dm["dependency_type"].(string); t != "" && t != "blocks" {
			continue
		}
		id, _ := dm["id"].(string)
		if raw, ok := s.Issues[id]; ok && decode(raw).str("status") != "closed" {
			return true
		}
	}
	return false
}

func (j journalBackend) write(s *remote.Snapshot, p parsed, req beadswire.ExecRequest) (beadswire.ExecResponse, error) {
	seq, err := nextSeq(j.journal)
	if err != nil {
		return beadswire.ExecResponse{}, err
	}
	now := time.Now
	if j.now != nil {
		now = j.now
	}
	e := beadswire.JournalEntry{Time: now().UTC(), Argv: req.Argv, Cwd: req.Cwd, Seq: seq, Stdin: req.Stdin}
	msg := fmt.Sprintf("recorded: bd %s (applied to Beads on the machine when the session is fetched)\n", p.cmd)
	var answer []issue
	switch p.cmd {
	case "create", "q":
		e.Placeholder = fmt.Sprintf("pending-%d", seq)
		title := strings.Join(p.pos, " ")
		if t, ok := p.flag("--title"); ok {
			title = t
		}
		rec := issue{"id": e.Placeholder, "title": title, "status": "open"}
		data, _ := json.Marshal(rec)
		s.Issues[e.Placeholder] = data
		answer = []issue{rec}
		msg = fmt.Sprintf("Created issue: %s (recorded; the real id is assigned when the session is fetched)\n", e.Placeholder)
		if p.cmd == "q" {
			msg = e.Placeholder + "\n"
		}
	default:
		for _, id := range p.pos {
			raw, ok := s.Issues[id]
			if !ok {
				continue
			}
			i := decode(raw)
			apply(i, p)
			data, _ := json.Marshal(i)
			s.Issues[id] = data
			answer = append(answer, i)
			if p.cmd != "update" && p.cmd != "close" && p.cmd != "reopen" && p.cmd != "unclaim" {
				break // dep, comment, note…: the first id is the issue
			}
		}
	}
	line, err := json.Marshal(e)
	if err != nil {
		return beadswire.ExecResponse{}, err
	}
	f, err := os.OpenFile(j.journal, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return beadswire.ExecResponse{}, fmt.Errorf("writing the Beads journal: %w", err)
	}
	_, werr := f.Write(append(line, '\n'))
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return beadswire.ExecResponse{}, fmt.Errorf("writing the Beads journal: %w", werr)
	}
	if err := j.save(s); err != nil {
		return beadswire.ExecResponse{}, err
	}
	if p.json && answer != nil {
		return render(p, answer), nil
	}
	return text(msg), nil
}

// apply reflects a write on the snapshot copy (later reads of the session).
func apply(i issue, p parsed) {
	switch p.cmd {
	case "close":
		i["status"] = "closed"
		if r, ok := p.flag("--reason", "-r"); ok {
			i["close_reason"] = r
		}
	case "reopen":
		i["status"] = "open"
	case "unclaim":
		i["status"], i["assignee"] = "open", ""
	case "update":
		if _, ok := p.flags["--claim"]; ok {
			i["status"] = "in_progress"
		}
		if v, ok := p.flag("--status", "-s"); ok {
			i["status"] = v
		}
		if v, ok := p.flag("--assignee", "-a"); ok {
			i["assignee"] = v
		}
		if v, ok := p.flag("--title"); ok {
			i["title"] = v
		}
		if v, ok := p.flag("--notes", "--note"); ok {
			i["notes"] = strings.TrimSpace(i.str("notes") + "\n" + v)
		}
	case "note":
		if len(p.pos) > 1 {
			i["notes"] = strings.TrimSpace(i.str("notes") + "\n" + strings.Join(p.pos[1:], " "))
		}
	}
}

// nextSeq counts the journal lines (the caller holds the lock).
func nextSeq(path string) (int, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	return bytes.Count(data, []byte("\n")) + 1, nil
}
