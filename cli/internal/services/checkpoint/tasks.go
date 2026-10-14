package checkpoint

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Breakdown read by oh (sweep-fanout): a checkpoint with `tasks:` is called
// with the breakdown (`tasks` of workflow_checkpoint), and oh, not the
// agent, says what it is:
//
//	asked ── read and checked ─┬─ problems ── no card, the call is refused with them
//	                           └─ card: the breakdown as oh read it (ShownTasks)
//	call ── read and checked again, the same as on the card? ─ passed:
//	        recorded (CheckpointState.Tasks, output `record: tasks`)
//
// The check: 1 to max tasks, each with a title, a domain of the list (when
// the workflow gives one) and a file scope whose every entry is a path, a
// folder or a pattern of the repository, or a new file in an existing
// folder (never absolute, never outside); the files of two tasks that may
// run at the same time (every task of a fan-out, the parallel ones
// otherwise) never meet.

// ErrTasksInvalid refuses a checkpoint whose breakdown oh cannot take.
var ErrTasksInvalid = errors.New("the breakdown of the checkpoint is refused by oh")

// TasksError lists what oh refuses in a breakdown.
type TasksError struct {
	ID       string
	Problems []string
}

func (e *TasksError) Error() string {
	return i18n.Tf("cmd.daemon.workflow.tasks_refused", e.ID, "- "+strings.Join(e.Problems, "\n- "))
}

// Is matches ErrTasksInvalid.
func (e *TasksError) Is(target error) bool { return target == ErrTasksInvalid }

// DataTasks is the key of the breakdown read by oh in a checkpoint decision.
const DataTasks = "tasks"

// RecordTasks is the record kind of the breakdown (workflow.RecordTasks).
const RecordTasks = "tasks"

// Bounds of a task.
const (
	maxTaskTitle       = 200
	maxTaskDescription = 4000
	maxTaskFiles       = 50
	maxTaskFileLen     = 300
)

// tasksOf reads the breakdown of a call: a list of {title, description,
// files, domain, parallel} (files: a list, or a text with one entry per
// line or comma). Problems are the entries oh cannot read.
func tasksOf(raw any) (tasks []domain.PlanTask, problems []string) {
	list, ok := raw.([]any)
	if !ok || len(list) == 0 {
		return nil, []string{i18n.T("cmd.daemon.workflow.tasks_none")}
	}
	for i, e := range list {
		m, ok := e.(map[string]any)
		if !ok {
			problems = append(problems, i18n.Tf("cmd.daemon.workflow.tasks_entry", i+1))
			continue
		}
		t := domain.PlanTask{ID: fmt.Sprintf("t%d", i+1)}
		t.Title = strings.TrimSpace(str(m["title"]))
		t.Description = strings.TrimSpace(str(m["description"]))
		t.Domain = strings.TrimSpace(str(m["domain"]))
		t.Parallel, _ = m["parallel"].(bool)
		t.Files = filesOf(m["files"])
		tasks = append(tasks, t)
	}
	return tasks, problems
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func filesOf(v any) []string {
	var raw []string
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			raw = append(raw, str(e))
		}
	case string:
		raw = strings.FieldsFunc(x, func(r rune) bool { return r == '\n' || r == ',' })
	}
	var out []string
	for _, f := range raw {
		if f = strings.TrimSpace(f); f != "" && !slices.Contains(out, f) {
			out = append(out, f)
		}
	}
	return out
}

// checkTasks reads and checks the breakdown given with a call of c in sess:
// the tasks as oh records them, or the problems.
func (s *Service) checkTasks(ctx context.Context, sess *domain.Session, c sessionspec.CheckpointDef, raw any) (tasks []domain.PlanTask, problems []string) {
	tasks, problems = tasksOf(raw)
	if len(tasks) == 0 {
		return nil, problems
	}
	if limit := c.Tasks.Max; limit > 0 && len(tasks) > limit {
		problems = append(problems, i18n.Tf("cmd.daemon.workflow.tasks_too_many", len(tasks), limit))
	}
	tracked, err := s.trackedFiles(ctx, sess.LaunchPath)
	if err != nil {
		return nil, append(problems, i18n.Tf("cmd.daemon.workflow.tasks_unreadable", err))
	}
	covered := make([][]string, len(tasks))
	for i, t := range tasks {
		at := t.ID
		if t.Title != "" {
			at += " (" + clipRunes(t.Title, 60) + ")"
		}
		switch {
		case t.Title == "":
			problems = append(problems, i18n.Tf("cmd.daemon.workflow.tasks_title", t.ID))
		case utf8.RuneCountInString(t.Title) > maxTaskTitle || strings.ContainsAny(t.Title, "\n\r"):
			problems = append(problems, i18n.Tf("cmd.daemon.workflow.tasks_title_long", at, maxTaskTitle))
		}
		if utf8.RuneCountInString(t.Description) > maxTaskDescription {
			problems = append(problems, i18n.Tf("cmd.daemon.workflow.tasks_description_long", at, maxTaskDescription))
		}
		if d := c.Tasks.Domains; len(d) > 0 && !slices.Contains(d, t.Domain) {
			problems = append(problems, i18n.Tf("cmd.daemon.workflow.tasks_domain", at, firstNonEmpty(t.Domain, "—"), strings.Join(d, ", ")))
		}
		switch {
		case len(t.Files) == 0:
			problems = append(problems, i18n.Tf("cmd.daemon.workflow.tasks_files", at))
			continue
		case len(t.Files) > maxTaskFiles:
			problems = append(problems, i18n.Tf("cmd.daemon.workflow.tasks_files_many", at, maxTaskFiles))
			continue
		}
		for j, f := range t.Files {
			clean, ok := scopeEntry(f)
			if !ok {
				problems = append(problems, i18n.Tf("cmd.daemon.workflow.tasks_file_invalid", at, f))
				continue
			}
			tasks[i].Files[j] = clean
			matched := matchScope(tracked, clean)
			if len(matched) == 0 {
				if !newFile(sess.LaunchPath, clean) {
					problems = append(problems, i18n.Tf("cmd.daemon.workflow.tasks_file_unknown", at, clean))
					continue
				}
				matched = []string{clean}
			}
			covered[i] = append(covered[i], matched...)
		}
	}
	fanout := sess.Fanout != nil && sess.Fanout.Role == domain.RoleParent
	for i := range tasks {
		for j := i + 1; j < len(tasks); j++ {
			if !fanout && (!tasks[i].Parallel || !tasks[j].Parallel) {
				continue
			}
			if shared := intersect(covered[i], covered[j]); len(shared) > 0 {
				problems = append(problems, i18n.Tf("cmd.daemon.workflow.tasks_overlap", tasks[i].ID, tasks[j].ID, listed(shared)))
			}
		}
	}
	if len(problems) > 0 {
		return nil, problems
	}
	return tasks, nil
}

func clipRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

// scopeEntry cleans an entry of a file scope: relative, inside the
// repository, one line, no option.
func scopeEntry(f string) (string, bool) {
	f = strings.TrimPrefix(strings.TrimSpace(filepath.ToSlash(f)), "./")
	dir := strings.HasSuffix(f, "/")
	if f == "" || len(f) > maxTaskFileLen || strings.ContainsAny(f, "\n\r\x00\\") || strings.HasPrefix(f, "-") || path.IsAbs(f) {
		return "", false
	}
	clean := path.Clean(f)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean == ".git" || strings.HasPrefix(clean, ".git/") {
		return "", false
	}
	if dir {
		clean += "/"
	}
	return clean, true
}

// InScope reports whether path p is in the scope entry e: the pattern
// (*, **), the file itself, or a file under the folder.
func InScope(p, e string) bool {
	if strings.HasSuffix(e, "/") {
		return strings.HasPrefix(p, e)
	}
	return p == e || strings.HasPrefix(p, e+"/") || (strings.ContainsAny(e, "*?[") && sessionspec.PathMatch(e, p))
}

// InTaskScope reports whether path p is in one of the entries of a scope.
func InTaskScope(p string, scope []string) bool {
	for _, e := range scope {
		if InScope(p, e) {
			return true
		}
	}
	return false
}

func matchScope(files []string, e string) []string {
	var out []string
	for _, f := range files {
		if InScope(f, e) {
			out = append(out, f)
		}
	}
	return out
}

// newFile reports whether e names a file to create: no pattern, its folder
// exists in the repository (dir).
func newFile(dir, e string) bool {
	if dir == "" || strings.ContainsAny(e, "*?[") || strings.HasSuffix(e, "/") {
		return false
	}
	parent := path.Dir(e)
	fi, err := os.Stat(filepath.Join(dir, filepath.FromSlash(parent)))
	return err == nil && fi.IsDir()
}

func intersect(a, b []string) []string {
	set := map[string]bool{}
	for _, x := range a {
		set[x] = true
	}
	var out []string
	for _, x := range b {
		if set[x] && !slices.Contains(out, x) {
			out = append(out, x)
		}
	}
	slices.Sort(out)
	return out
}

// trackedFiles lists the files of the repository in dir (git ls-files).
func (s *Service) trackedFiles(ctx context.Context, dir string) ([]string, error) {
	if s.Files != nil {
		return s.Files(ctx, dir)
	}
	if dir == "" {
		return nil, errors.New("no location")
	}
	out, err := gitAs(ctx, dir, s.Account, "ls-files", "-z", "--cached", "--others", "--exclude-standard").Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range strings.Split(string(out), "\x00") {
		if f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

// RenderTasks is the breakdown as oh recorded it, one task per line (card,
// output `record: tasks`, message to the agent).
func RenderTasks(tasks []domain.PlanTask) string {
	var b strings.Builder
	for i, t := range tasks {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(t.ID)
		if t.Domain != "" {
			b.WriteString(" [" + t.Domain + "]")
		}
		b.WriteString(" " + t.Title + " — " + strings.Join(t.Files, ", "))
		if t.Parallel {
			b.WriteString(" " + i18n.T("cmd.daemon.workflow.tasks_parallel"))
		}
	}
	return b.String()
}

// sameTasks reports whether two breakdowns are the same.
func sameTasks(a, b []domain.PlanTask) bool {
	return slices.EqualFunc(a, b, func(x, y domain.PlanTask) bool {
		return x.ID == y.ID && x.Title == y.Title && x.Description == y.Description && x.Domain == y.Domain &&
			x.Parallel == y.Parallel && slices.Equal(x.Files, y.Files)
	})
}

// tasksAsked reads the breakdown of a checkpoint asked (Asked, Evaluate):
// ok false when oh refuses it (the call then runs and is refused with the
// problems: no card for a breakdown oh cannot take).
func (s *Service) tasksAsked(ctx context.Context, sess *domain.Session, c sessionspec.CheckpointDef, input map[string]any) ([]domain.PlanTask, bool) {
	if c.Tasks == nil {
		return nil, true
	}
	tasks, problems := s.checkTasks(ctx, sess, c, input[DataTasks])
	return tasks, len(problems) == 0
}

// tasksReached checks the breakdown of a call of c before it passes: the
// same as on the card when the checkpoint waited for the user.
func (s *Service) tasksReached(ctx context.Context, sess *domain.Session, c sessionspec.CheckpointDef, raw any) ([]domain.PlanTask, error) {
	if c.Tasks == nil {
		return nil, nil
	}
	tasks, problems := s.checkTasks(ctx, sess, c, raw)
	if len(problems) > 0 {
		return nil, &TasksError{ID: c.ID, Problems: problems}
	}
	if s.needsDecision(ctx, sess, c) {
		st, err := s.state(ctx, sess.ID)
		if err != nil {
			return nil, err
		}
		if !sameTasks(st.ShownTasks, tasks) {
			return nil, &TasksError{ID: c.ID, Problems: []string{i18n.T("cmd.daemon.workflow.tasks_changed")}}
		}
	}
	return tasks, nil
}

// recordTasks records the breakdown of checkpoint id once it passed.
func (s *Service) recordTasks(ctx context.Context, sessionID, id string, tasks []domain.PlanTask) {
	if len(tasks) == 0 {
		return
	}
	_, _ = s.update(ctx, sessionID, func(st *domain.CheckpointState) error {
		st.Tasks, st.ShownTasks = tasks, nil
		s.record(st, domain.CheckpointTasks, id, "oh", fmt.Sprintf("%d", len(tasks)))
		return nil
	})
	_, _ = s.Record(ctx, sessionID, RecordTasks, RenderTasks(tasks))
}

// scopeCheck refuses the verification of a child of a fan-out whose task
// has a file scope (sweep-fanout) when the child changed files outside it:
// the scopes of the tasks are disjoint, a change outside would meet the
// work of another child.
func (s *Service) scopeCheck(ctx context.Context, sess *domain.Session) (domain.CheckpointVerify, bool) {
	f := sess.Fanout
	if f == nil || f.Role != domain.RoleChild || f.Task == nil || len(f.Task.Files) == 0 {
		return domain.CheckpointVerify{}, true
	}
	paths, err := s.changedPaths(ctx, sess)
	if err != nil {
		return domain.CheckpointVerify{Status: domain.VerifyFailed, Reason: i18n.Tf("tui.checkpoint.verify.task_scope_unreadable", err)}, false
	}
	var outside []string
	for _, p := range paths {
		if !InTaskScope(p, f.Task.Files) {
			outside = append(outside, p)
		}
	}
	if len(outside) == 0 {
		return domain.CheckpointVerify{}, true
	}
	return domain.CheckpointVerify{Status: domain.VerifyFailed,
		Reason: i18n.Tf("tui.checkpoint.verify.task_scope", f.Task.ID, listed(outside), strings.Join(f.Task.Files, ", "))}, false
}
