package runsvc

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionctx"
	"github.com/datichb/openhub/cli/internal/sessionresults"
	"github.com/datichb/openhub/cli/internal/worktree"
)

// Handoff brief (OnExistingKeep): what a new session needs to know about
// the stopped session whose work it continues.

// handoffListMax bounds each list of the brief.
const handoffListMax = 30

// HandoffBrief describes the work of a session for the session continuing
// it: objective, results, decisions taken, commits of its branch and
// changed files. Read once the session is stopped (its results are saved).
func (s *Service) HandoffBrief(ctx context.Context, sessionID, projectPath string) string {
	sess, err := s.Session(ctx, sessionID)
	if err != nil {
		return ""
	}
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	title := ""
	if sess.Title != nil {
		title = *sess.Title
	}
	branch := sess.WorktreeBranch
	if branch == "" && sess.LaunchPath != "" {
		branch, _ = worktree.CurrentBranch(sess.LaunchPath)
	}
	line("# %s", i18n.Tf("cmd.run.handoff.title", sess.ID))
	line("%s", i18n.Tf("cmd.run.handoff.intro", sess.LaunchPath, orDash(branch)))
	line("")
	line("- %s", i18n.Tf("cmd.run.handoff.objective", orDash(sess.WorkflowID), sess.WorkflowVersion, orDash(strings.Join(sess.Tickets, ", ")), orDash(title)))
	if sum, _, err := sessionresults.Load(s.SessionsDir, sess.ID); err == nil {
		line("- %s", i18n.Tf("cmd.run.handoff.results", len(sum.Files), sum.Additions, sum.Deletions, sum.Cost))
	}
	if ds := s.decisionsTaken(ctx, sess); len(ds) > 0 {
		line("")
		line("## %s", i18n.T("cmd.run.handoff.decisions"))
		writeList(&b, ds)
	}
	if sess.LaunchPath != "" && worktree.IsGitRepo(sess.LaunchPath) {
		base := worktree.DetectBaseBranch(projectPath)
		if commits, err := gitLines(sess.LaunchPath, "log", "--oneline", "--no-decorate", "-n", fmt.Sprint(handoffListMax+1), base+"..HEAD"); err == nil && len(commits) > 0 {
			line("")
			line("## %s", i18n.Tf("cmd.run.handoff.commits", base))
			writeList(&b, commits)
		}
		if files := changedFiles(sess.LaunchPath); len(files) > 0 {
			line("")
			line("## %s", i18n.T("cmd.run.handoff.files"))
			writeList(&b, files)
		}
	}
	return strings.TrimSpace(b.String())
}

func orDash(v string) string {
	if v == "" {
		return "—"
	}
	return v
}

func writeList(b *strings.Builder, items []string) {
	for i, it := range items {
		if i == handoffListMax {
			fmt.Fprintf(b, "- … (+%d)\n", len(items)-handoffListMax)
			return
		}
		fmt.Fprintf(b, "- %s\n", it)
	}
}

// decisionsTaken lists the decisions answered in a session (not the ones
// closed because it ended).
func (s *Service) decisionsTaken(ctx context.Context, sess *domain.Session) []string {
	if s.Decisions == nil {
		return nil
	}
	list, err := s.Decisions.ListSince(ctx, sess.StartedAt)
	if err != nil {
		return nil
	}
	var out []string
	for _, d := range list {
		if d.SessionID != sess.ID || d.ResolvedAt == nil || d.ResolvedBy == domain.ResolvedByGone {
			continue
		}
		what := d.Payload.Title
		if what == "" {
			what = strings.TrimSpace(d.Payload.Action + " " + strings.Join(d.Payload.Resources, " "))
		}
		answer := ""
		if r := d.Resolution; r != nil {
			answer = r.Decision
			if len(r.Answer) > 0 {
				keys := make([]string, 0, len(r.Answer))
				for k := range r.Answer {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				parts := make([]string, len(keys))
				for i, k := range keys {
					parts[i] = fmt.Sprintf("%s=%v", k, r.Answer[k])
				}
				answer = strings.TrimSpace(answer + " " + strings.Join(parts, ", "))
			}
			if r.Message != "" {
				answer = strings.TrimSpace(answer + " — " + r.Message)
			}
		}
		out = append(out, fmt.Sprintf("%s %s → %s", d.Kind, orDash(what), orDash(answer)))
	}
	return out
}

// changedFiles lists the files of a directory changed since its branch
// left the base branch, uncommitted changes and new files included.
func changedFiles(dir string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(f string) {
		if f != "" && !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	if base := gitutil.BaseRef(dir); base != "" {
		if files, err := gitLines(dir, "diff", "--name-only", "--no-renames", base, "--"); err == nil {
			for _, f := range files {
				add(f)
			}
		}
	}
	if st, err := gitLines(dir, "status", "--porcelain", "--untracked-files=all"); err == nil {
		for _, l := range st {
			if len(l) > 3 {
				add(l[3:])
			}
		}
	}
	return out
}

// setHandoffContext gives the brief of the continued session to the entry
// agent (session context entry; a synthetic message without the
// capability). It is also kept in the session folder (handoff.md).
func (s *Service) setHandoffContext(ctx context.Context, h adapters.ServerHandle, sessionID, brief string) {
	if s.SessionsDir != "" {
		dir := filepath.Join(s.SessionsDir, sessionID)
		if err := os.MkdirAll(dir, 0o700); err == nil {
			_ = os.WriteFile(filepath.Join(dir, "handoff.md"), []byte(brief+"\n"), 0o600)
		}
	}
	w := &sessionctx.Writer{Adapter: s.Adapter, Dir: s.SessionsDir}
	if _, err := w.Set(ctx, h, sessionID, sessionctx.Entry{Key: sessionctx.KeyHandoff,
		Value: map[string]any{"text": brief}, Fallback: brief}); err != nil {
		slog.Warn("runsvc: handoff brief not set", "session", sessionID, "error", err)
	}
}
