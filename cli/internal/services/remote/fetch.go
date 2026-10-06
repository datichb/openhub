package remote

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/remote"
)

// Errors of a fetch.
var (
	ErrNotFinished = errors.New("the pipeline of this session is still running")
	ErrNoArtifacts = errors.New("the pipeline left no artifacts (oh-run job missing or expired)")
)

// FetchOptions control a fetch.
type FetchOptions struct {
	// NoImport only downloads the artifacts (no local session).
	NoImport bool
}

// FetchResult is a fetched remote session.
type FetchResult struct {
	SessionID string
	Summary   remote.Summary
	Journal   []beadswire.JournalEntry
	Dir       string // local copy of the artifacts
	Imported  bool
	Location  string // where the session continues locally
	Ref       domain.RemoteRef
}

// artifactNames are the files kept from the run job artifacts.
var artifactNames = map[string]bool{remote.JournalFile: true, remote.SummaryFile: true, remote.ExportFile: true}

// Fetch downloads the artifacts of a finished remote session and imports the
// session into a local server group so that it can be resumed (P5-T16). The
// session continues in a worktree of the pushed branch (the project base
// directory when nothing was pushed). The Beads journal is not replayed
// here (PlanReplay / ApplyReplay, with confirmation).
func (s *Service) Fetch(ctx context.Context, sid string, opts FetchOptions) (*FetchResult, error) {
	tr, err := s.TrackSession(ctx, sid)
	if err != nil {
		return nil, err
	}
	ref := tr.Ref
	if Pending(ref) {
		return nil, ErrNotFinished
	}
	c, _, err := s.clientFor(ctx, ref.Target)
	if err != nil {
		return nil, err
	}
	rid := fmt.Sprint(ref.RunnerID)
	jobs, err := c.PipelineJobs(ctx, rid, ref.Pipeline)
	if err != nil {
		return nil, err
	}
	var job int64
	for _, j := range jobs {
		if j.Name == "oh-run" && j.ID > job {
			job = j.ID
		}
	}
	if job == 0 {
		return nil, ErrNoArtifacts
	}
	data, err := c.JobArtifacts(ctx, rid, job)
	if err != nil {
		return nil, fmt.Errorf("%w (%v)", ErrNoArtifacts, err)
	}
	dir := s.RemoteDir(sid)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := extractArtifacts(data, dir); err != nil {
		return nil, err
	}
	res := &FetchResult{SessionID: sid, Dir: dir}
	sdata, err := os.ReadFile(filepath.Join(dir, remote.SummaryFile))
	if err != nil {
		return nil, fmt.Errorf("%w: no summary.json", ErrNoArtifacts)
	}
	if err := json.Unmarshal(sdata, &res.Summary); err != nil {
		return nil, fmt.Errorf("summary.json: %w", err)
	}
	if res.Journal, err = ReadJournal(filepath.Join(dir, remote.JournalFile)); err != nil {
		return nil, err
	}

	sess, err := s.Sessions.Get(ctx, sid)
	if err != nil {
		return nil, err
	}
	res.Location = sess.LaunchPath
	if !opts.NoImport && sess.Runtime == "remote" {
		if err := s.importSession(ctx, sess, ref, res); err != nil {
			return res, err
		}
	}

	// Records: cost, tokens, outputs, MR, status.
	if sess, err = s.Sessions.Get(ctx, sid); err == nil {
		sm := res.Summary
		sess.Cost, sess.TokensIn, sess.TokensOut = sm.Cost, sm.TokensIn, sm.TokensOut
		sess.TokensReasoning, sess.TokensCacheRead = sm.TokensReasoning, sm.TokensCacheRead
		if sm.Model != "" {
			sess.Model = sm.Model
		}
		if len(sm.Outputs) > 0 {
			sess.Outputs = sm.Outputs
		}
		if err := s.Sessions.Update(ctx, sess); err != nil {
			return res, err
		}
	}
	ref.MRURL = res.Summary.MRURL
	if res.Summary.Error != "" {
		ref.Error = res.Summary.Error
	}
	ref.Status, ref.UpdatedAt = domain.RemoteFetched, s.now().UTC()
	if len(res.Journal) == 0 {
		ref.Status = domain.RemoteResolved
	}
	if err := s.Remote.SetRemoteRef(ctx, sid, ref); err != nil {
		return res, err
	}
	res.Ref = ref
	return res, nil
}

// importSession recreates the session locally (worktree of the pushed branch).
func (s *Service) importSession(ctx context.Context, sess *domain.Session, ref domain.RemoteRef, res *FetchResult) error {
	edata, err := os.ReadFile(filepath.Join(res.Dir, remote.ExportFile))
	if err != nil {
		return fmt.Errorf("%w: no session.export", ErrNoArtifacts)
	}
	var ex remote.Export
	if err := json.Unmarshal(edata, &ex); err != nil {
		return fmt.Errorf("session.export: %w", err)
	}
	if s.Adopt == nil {
		return errors.New("remote: import not wired")
	}
	if err := s.ensureBundle(ctx, ref, sess.BundleHash); err != nil {
		return err
	}
	loc := sess.LaunchPath
	if res.Summary.Commit != "" && ref.Branch != "" {
		if err := s.git().FetchBranch(ctx, sess.LaunchPath, ref.Branch); err != nil {
			return fmt.Errorf("fetching the branch %s: %w", ref.Branch, err)
		}
		if s.Worktree != nil {
			wt, err := s.Worktree(sess.LaunchPath, ref.Branch)
			if err != nil {
				return fmt.Errorf("worktree of %s: %w", ref.Branch, err)
			}
			loc = wt
		}
	}
	trs := make([][]byte, len(ex.Sessions))
	for i, t := range ex.Sessions {
		trs[i] = t
	}
	if err := s.Adopt(ctx, sess.ID, trs, loc); err != nil {
		return err
	}
	res.Imported, res.Location = true, loc
	return nil
}

// ensureBundle downloads the bundle of the session when it is not on the
// machine any more (purged).
func (s *Service) ensureBundle(ctx context.Context, ref domain.RemoteRef, hash string) error {
	if s.BundlesDir == "" {
		return nil
	}
	dir := filepath.Join(s.BundlesDir, hash)
	if _, err := os.Stat(filepath.Join(dir, "bundle.json")); err == nil {
		return nil
	}
	c, _, err := s.clientFor(ctx, ref.Target)
	if err != nil {
		return err
	}
	rc, err := c.DownloadGenericPackage(ctx, fmt.Sprint(ref.RunnerID), remote.PackageBundle, hash, "bundle.tar.gz")
	if err != nil {
		return fmt.Errorf("downloading the session bundle: %w", err)
	}
	defer rc.Close()
	tmp, err := os.MkdirTemp(s.BundlesDir, ".fetch-*")
	if err != nil {
		return err
	}
	if err := remote.UnpackDir(rc, tmp); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	return os.Rename(tmp, dir)
}

// extractArtifacts keeps oh-out/{journal.jsonl,summary.json,session.export}.
func extractArtifacts(data []byte, dir string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNoArtifacts, err)
	}
	for _, f := range zr.File {
		name := path.Base(f.Name)
		if path.Dir(f.Name) != remote.OutDir || !artifactNames[name] || f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		content, err := io.ReadAll(io.LimitReader(rc, remote.MaxUnpack))
		rc.Close()
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			return err
		}
	}
	return nil
}

// ReadJournal reads a Beads journal (missing file: empty), in seq order.
func ReadJournal(p string) ([]beadswire.JournalEntry, error) {
	f, err := os.Open(p)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []beadswire.JournalEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 8<<20)
	for sc.Scan() {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		var e beadswire.JournalEntry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			return nil, fmt.Errorf("journal.jsonl: %w", err)
		}
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	return out, sc.Err()
}
