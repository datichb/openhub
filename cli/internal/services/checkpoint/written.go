package checkpoint

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Files written by the session (B1, `record: written`): each time a
// checkpoint passes, oh reads the files the session changed under the paths
// of the output (committed since it started, uncommitted or untracked) and
// records them (the postmortem of incident, the decision of spike), or, with
// `match:`, the first group of the regular expression in the first of them
// that matches (the verdict of the decision).

// recordWritten records the outputs `record: written` of a session. A
// failure is only logged: the checkpoint has passed.
func (s *Service) recordWritten(ctx context.Context, sess *domain.Session, wf *sessionspec.WorkflowRuntime) {
	var defs []sessionspec.OutputDef
	for _, o := range wf.Outputs {
		if o.Record == RecordWritten && len(o.Paths) > 0 {
			defs = append(defs, o)
		}
	}
	if len(defs) == 0 || s.SessionOutputs == nil {
		return
	}
	changed, err := s.changedPaths(ctx, sess)
	if err != nil {
		slog.Debug("checkpoint: written files not read", "session", sess.ID, "error", err)
		return
	}
	sort.Strings(changed)
	for _, o := range defs {
		files := writtenUnder(sess.LaunchPath, changed, o.Paths)
		v := ""
		if o.Match == "" {
			v = strings.Join(files, ",")
		} else {
			v = matchIn(sess.LaunchPath, files, o.Match)
		}
		if v == "" || fmt.Sprint(sess.Outputs[o.ID]) == v {
			continue
		}
		if err := s.SessionOutputs.SetSessionOutput(ctx, sess.ID, o.ID, v); err != nil {
			slog.Debug("checkpoint: written output not recorded", "session", sess.ID, "output", o.ID, "error", err)
		}
	}
}

// writtenUnder returns the changed paths under patterns that are regular
// files of dir (a deleted file is not written).
func writtenUnder(dir string, changed, patterns []string) []string {
	var out []string
	for _, p := range changed {
		if !touches([]string{p}, patterns) {
			continue
		}
		if _, ok := readSpec(dir, p); ok {
			out = append(out, p)
		}
	}
	return out
}

// matchIn returns the first group of expr in the first file that matches.
func matchIn(dir string, files []string, expr string) string {
	re, err := regexp.Compile(expr)
	if err != nil || re.NumSubexp() < 1 {
		return ""
	}
	for _, p := range files {
		data, ok := readSpec(dir, p)
		if !ok {
			continue
		}
		if m := re.FindSubmatch(data); m != nil {
			if v := strings.TrimSpace(string(m[1])); v != "" {
				return v
			}
		}
	}
	return ""
}
