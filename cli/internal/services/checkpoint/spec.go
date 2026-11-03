package checkpoint

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/wiki"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/specdoc"
)

// Specs written by the session (workflow spec): `verify.spec` makes oh
// read the specs the session changed under its paths and check them
// (internal/specdoc) before the checkpoint passes:
//
//	no spec changed under the paths ─────────────── refused
//	frontmatter, confidence tags (wiki format) ──── refused, each problem listed
//	glossary, criteria with test points ─ missing ── refused, each problem listed
//	breakdown (convergence) ─ a criterion uncovered,
//	                          a slice outside the spec ── refused
//	otherwise ────────────────────────────────────── passed (then the commands)
//
// A spec without its risks and edge cases passes, with a warning (specdoc.Advise)
// in the result: shown on the card of the checkpoint and to the agent.

// maxSpecSize bounds the size of a spec read.
const maxSpecSize = 1 << 20

// specCheck checks the specs of verification v: a failed result when they
// are missing or incomplete (ok false), else the warnings about them (note).
// The specs checked are recorded in the output `record: spec` of the session.
func (s *Service) specCheck(ctx context.Context, sess *domain.Session, v *sessionspec.VerifyDef) (failed domain.CheckpointVerify, note string, ok bool) {
	c := v.Spec
	if c == nil {
		return domain.CheckpointVerify{}, "", true
	}
	paths, err := s.changesFor(ctx, sess, v)
	if err != nil {
		return domain.CheckpointVerify{Status: domain.VerifyFailed,
			Reason: i18n.Tf("tui.checkpoint.verify.spec_none", strings.Join(c.Paths, ", "))}, "", false
	}
	var specs, problems, warnings []string
	for _, p := range paths {
		if !strings.EqualFold(filepath.Ext(p), ".md") || !touches([]string{p}, c.Paths) {
			continue
		}
		data, readable := readSpec(sess.LaunchPath, p)
		if !readable {
			continue // deleted, or not a regular file
		}
		specs = append(specs, p)
		// The format of the wiki first: frontmatter, its keys
		// and values, the confidence tags (wiki.FormatIssues).
		for _, is := range wiki.FormatIssues(strings.TrimPrefix(p, wiki.Dir+"/"), string(data)) {
			at := p
			if is.Line > 1 {
				at = fmt.Sprintf("%s:%d", p, is.Line)
			}
			problems = append(problems, fmt.Sprintf("%s : %s", at, is.Message))
		}
		for _, pb := range specdoc.Check(data, c.Breakdown) {
			msg := i18n.T("tui.checkpoint.verify.spec." + pb.Code)
			if pb.Item != "" {
				msg = i18n.Tf("tui.checkpoint.verify.spec."+pb.Code, pb.Item)
			}
			problems = append(problems, fmt.Sprintf("%s : %s", p, msg))
		}
		for _, w := range specdoc.Advise(data) {
			warnings = append(warnings, fmt.Sprintf("%s : %s", p, i18n.T("tui.checkpoint.verify.spec."+w.Code)))
		}
	}
	switch {
	case len(specs) == 0:
		return domain.CheckpointVerify{Status: domain.VerifyFailed,
			Reason: i18n.Tf("tui.checkpoint.verify.spec_none", strings.Join(c.Paths, ", "))}, "", false
	case len(problems) > 0:
		return domain.CheckpointVerify{Status: domain.VerifyFailed,
			Reason:  i18n.Tf("tui.checkpoint.verify.spec_problems", len(problems), strings.Join(specs, ", ")),
			Excerpt: strings.Join(problems, "\n")}, "", false
	}
	if _, err := s.Record(ctx, sess.ID, RecordSpec, specs...); err != nil {
		slog.Debug("checkpoint: spec output not recorded", "session", sess.ID, "error", err)
	}
	if len(warnings) > 0 {
		note = i18n.Tf("tui.checkpoint.verify.spec_warnings", strings.Join(warnings, " ; "))
	}
	return domain.CheckpointVerify{}, note, true
}

// readSpec reads the regular file rel of dir (a link is not followed).
func readSpec(dir, rel string) ([]byte, bool) {
	p := filepath.Join(dir, filepath.FromSlash(rel))
	fi, err := os.Lstat(p)
	if err != nil || !fi.Mode().IsRegular() {
		return nil, false
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxSpecSize))
	return data, err == nil
}
