package checkpoint

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Verification run by oh at a checkpoint (R3, E1): the build, tests and type
// check of the project (`verify: { from: stack }`, bricks.VerifyRuns) or the
// commands of the workflow document (`cmd`), never a command of the agent.
//
//	asked (paused) ── verification started ── card shows the result
//	call runs ── same state of the work? ─┬─ result reused
//	                                      └─ run again (bounded wait)
//	failed ── nothing unlocked, ErrVerifyFailed with an excerpt
//	         └─ the user overrides at the paused checkpoint (traced)
//
// The commands run without a shell, with a cleaned environment, a timeout
// and a bounded output. A result is tied to the state of the work verified
// (HEAD, uncommitted changes): it is reused only for the same state.

// Verification errors (VerifyError matches them).
var (
	ErrVerifyFailed  = errors.New("checkpoint verification failed")
	ErrVerifyPending = errors.New("checkpoint verification running")
)

// VerifyError is a checkpoint refused by its verification (message for the
// agent: what failed and an excerpt of the output).
type VerifyError struct {
	ID      string
	Label   string
	Result  domain.CheckpointVerify
	Pending bool
}

func (e *VerifyError) Error() string {
	if e.Pending {
		return i18n.Tf("cmd.daemon.workflow.verify_pending", e.ID, e.Label)
	}
	msg := i18n.Tf("cmd.daemon.workflow.verify_failed", e.ID, e.Label, VerifySummary(e.Result))
	if e.Result.Excerpt != "" {
		msg += "\n" + e.Result.Excerpt
	}
	return msg
}

// Is matches ErrVerifyFailed or ErrVerifyPending.
func (e *VerifyError) Is(target error) bool {
	if e.Pending {
		return target == ErrVerifyPending
	}
	return target == ErrVerifyFailed
}

// Bounds of a verification.
const (
	// verifyWait bounds the wait of a workflow_checkpoint call for a
	// verification still running (the call is then refused as pending:
	// below the 30 s of the daemon client).
	verifyWait = 20 * time.Second
	// verifyOutputMax is the output kept per command (its end).
	verifyOutputMax = 64 * 1024
	// excerptLines and excerptBytes bound the excerpt.
	excerptLines = 30
	excerptBytes = 3000
)

// ExecFunc runs a verification command in dir with env, and returns its
// exit code and output (bounded by the caller). err is set when the
// command could not run.
type ExecFunc func(ctx context.Context, dir string, args, env []string, out *TailBuffer) (exit int, err error)

// verifier is the in-process state of the verifications: one run at a time
// per session, checkpoint and state of the work.
type verifier struct {
	mu       sync.Mutex
	inflight map[string]bool
}

func (s *Service) verifyWait() time.Duration {
	if s.VerifyWait > 0 {
		return s.VerifyWait
	}
	return verifyWait
}

// verifyDir is the folder of the verification of c ("" = unreachable).
func verifyDir(sess *domain.Session, v *sessionspec.VerifyDef) string {
	if sess.LaunchPath == "" {
		return ""
	}
	dir := sess.LaunchPath
	if v.In != "" {
		dir = filepath.Join(dir, filepath.FromSlash(v.In))
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return ""
	}
	return dir
}

// StartVerify starts the verification of checkpoint c of sess in the
// background, unless it already ran or runs for the current state of the
// work (called when a paused checkpoint is asked: the card shows it).
func (s *Service) StartVerify(sess *domain.Session, c sessionspec.CheckpointDef) {
	if c.Verify == nil || s.States == nil {
		return
	}
	ctx := context.Background()
	fp := s.fingerprint(ctx, sess)
	if r, ok := s.currentVerify(ctx, sess.ID, c, fp); ok && (r.Done() || !s.stale(r, c)) {
		return
	}
	s.launchVerify(sess, c, fp)
}

// verification returns the verification of c for the current state of the
// work, running it when needed and waiting at most wait for its end
// (ErrVerifyPending after that).
func (s *Service) verification(ctx context.Context, sess *domain.Session, c sessionspec.CheckpointDef, wait time.Duration) (domain.CheckpointVerify, error) {
	fp := s.fingerprint(ctx, sess)
	r, ok := s.currentVerify(ctx, sess.ID, c, fp)
	if !ok || (!r.Done() && s.stale(r, c)) {
		s.launchVerify(sess, c, fp)
	}
	deadline := time.Now().Add(wait)
	for {
		if r, ok = s.currentVerify(ctx, sess.ID, c, fp); ok && r.Done() {
			return r, nil
		}
		if time.Now().After(deadline) {
			return r, ErrVerifyPending
		}
		select {
		case <-ctx.Done():
			return r, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// currentVerify returns the stored verification of c when it is for the
// state fp of the work.
func (s *Service) currentVerify(ctx context.Context, sessionID string, c sessionspec.CheckpointDef, fp string) (domain.CheckpointVerify, bool) {
	st, err := s.state(ctx, sessionID)
	if err != nil {
		return domain.CheckpointVerify{}, false
	}
	r, ok := st.Verify[c.ID]
	return r, ok && r.Fingerprint == fp
}

// stale reports whether a running verification outlived its timeout (the
// process that ran it stopped).
func (s *Service) stale(r domain.CheckpointVerify, c sessionspec.CheckpointDef) bool {
	return s.now().After(r.Started.Add(verifyTimeout(c.Verify) + 30*time.Second))
}

func verifyTimeout(v *sessionspec.VerifyDef) time.Duration {
	if v == nil || v.TimeoutSec <= 0 {
		return 10 * time.Minute
	}
	return time.Duration(v.TimeoutSec) * time.Second
}

// launchVerify records the verification as running and runs it in the
// background (once per session, checkpoint and state of the work).
func (s *Service) launchVerify(sess *domain.Session, c sessionspec.CheckpointDef, fp string) {
	key := sess.ID + "\x00" + c.ID + "\x00" + fp
	s.verifier.mu.Lock()
	if s.verifier.inflight[key] {
		s.verifier.mu.Unlock()
		return
	}
	if s.verifier.inflight == nil {
		s.verifier.inflight = map[string]bool{}
	}
	s.verifier.inflight[key] = true
	s.verifier.mu.Unlock()
	started := s.now()
	ctx := context.Background()
	_, err := s.update(ctx, sess.ID, func(st *domain.CheckpointState) error {
		if st.Verify == nil {
			st.Verify = map[string]domain.CheckpointVerify{}
		}
		st.Verify[c.ID] = domain.CheckpointVerify{Status: domain.VerifyRunning, Expect: c.Verify.Expect, Fingerprint: fp, Started: started}
		return nil
	})
	if err != nil {
		s.verifier.mu.Lock()
		delete(s.verifier.inflight, key)
		s.verifier.mu.Unlock()
		return
	}
	go func() {
		defer func() {
			s.verifier.mu.Lock()
			delete(s.verifier.inflight, key)
			s.verifier.mu.Unlock()
		}()
		r := s.runVerify(ctx, sess, c)
		r.Fingerprint, r.Started, r.Expect = fp, started, c.Verify.Expect
		done := s.now()
		r.Finished = &done
		_, _ = s.update(ctx, sess.ID, func(st *domain.CheckpointState) error {
			if cur, ok := st.Verify[c.ID]; ok && (cur.Fingerprint != fp || !cur.Started.Equal(started)) {
				return errUnchanged // a newer run replaced it
			}
			if st.Verify == nil {
				st.Verify = map[string]domain.CheckpointVerify{}
			}
			st.Verify[c.ID] = r
			kind := domain.CheckpointVerified
			if !r.OK() {
				kind = domain.CheckpointVerifyFail
			}
			s.record(st, kind, c.ID, "oh", VerifySummary(r))
			return nil
		})
	}()
}

// runVerify runs the verification of c (no state change).
func (s *Service) runVerify(ctx context.Context, sess *domain.Session, c sessionspec.CheckpointDef) domain.CheckpointVerify {
	v := c.Verify
	dir := verifyDir(sess, v)
	if dir == "" {
		// The location is not on this machine (remote runner) or is gone:
		// nothing to verify here, shown on the card.
		return domain.CheckpointVerify{Status: domain.VerifyNone, Reason: i18n.T("tui.checkpoint.verify.no_location")}
	}
	if len(v.MustTouch) > 0 {
		paths, err := s.changedPaths(ctx, sess)
		if err != nil || !touches(paths, v.MustTouch) {
			return domain.CheckpointVerify{Status: domain.VerifyFailed,
				Reason: i18n.Tf("tui.checkpoint.verify.must_touch", strings.Join(v.MustTouch, ", "))}
		}
	}
	runs := verifyRuns(sess.LaunchPath, dir, v)
	if len(runs) == 0 {
		return domain.CheckpointVerify{Status: domain.VerifyNone, Reason: i18n.T("tui.checkpoint.verify.no_command")}
	}
	ctx, cancel := context.WithTimeout(ctx, verifyTimeout(v))
	defer cancel()
	run := s.Exec
	if run == nil {
		run = execVerify
	}
	env := verifyEnv(os.Environ())
	r := domain.CheckpointVerify{}
	var failed *TailBuffer
	for _, cmd := range runs {
		out := &TailBuffer{Max: verifyOutputMax}
		start := time.Now()
		exit, err := run(ctx, dir, cmd.Args, env, out)
		res := domain.VerifyCommandResult{Kind: cmd.Kind, Cmd: cmd.String(), Exit: exit, Millis: time.Since(start).Milliseconds()}
		if ctx.Err() != nil {
			res.TimedOut = true
			r.Commands = append(r.Commands, res)
			r.Status, r.Reason = domain.VerifyFailed, i18n.Tf("tui.checkpoint.verify.timeout", verifyTimeout(v))
			r.Excerpt = excerpt(out.Bytes())
			return r
		}
		if err != nil && exit == 0 {
			exit, res.Exit = -1, -1
			out.WriteString("\n" + err.Error())
		}
		r.Commands = append(r.Commands, res)
		if exit != 0 {
			failed = out
			break
		}
	}
	switch {
	case v.Expect == sessionspec.VerifyFail && failed != nil:
		r.Status, r.Excerpt = domain.VerifyPassed, excerpt(failed.Bytes())
	case v.Expect == sessionspec.VerifyFail:
		r.Status, r.Reason = domain.VerifyFailed, i18n.T("tui.checkpoint.verify.expected_failure")
	case failed != nil:
		r.Status, r.Excerpt = domain.VerifyFailed, excerpt(failed.Bytes())
	default:
		r.Status = domain.VerifyPassed
	}
	return r
}

// verifyRuns are the commands of a verification: the stack commands of the
// folder, then the commands of the workflow document.
func verifyRuns(project, dir string, v *sessionspec.VerifyDef) []bricks.VerifyRun {
	var runs []bricks.VerifyRun
	if v.From == sessionspec.VerifyFromStack {
		rel := filepath.ToSlash(filepath.Clean(v.In))
		if v.In == "" {
			rel = "."
		}
		runs = bricks.VerifyRuns(dir, bricks.DetectSignals(project), rel, v.Kinds)
	}
	for _, c := range v.Cmd {
		if args := strings.Fields(c); len(args) > 0 {
			runs = append(runs, bricks.VerifyRun{Args: args})
		}
	}
	return runs
}

// verifyEnvKeys are the variables a verification keeps (toolchains, locale,
// caches); everything else is dropped (tokens, credentials, the variables
// of oh and of the tool).
var verifyEnvKeys = map[string]bool{
	"PATH": true, "HOME": true, "USER": true, "LOGNAME": true, "SHELL": true, "TMPDIR": true, "TZ": true,
	"LANG": true, "LC_ALL": true, "LC_CTYPE": true, "LC_MESSAGES": true,
	"XDG_CACHE_HOME": true, "XDG_CONFIG_HOME": true, "XDG_DATA_HOME": true, "SSL_CERT_FILE": true, "SSL_CERT_DIR": true,
	"GOPATH": true, "GOROOT": true, "GOCACHE": true, "GOMODCACHE": true, "GOFLAGS": true, "GOPROXY": true, "GOPRIVATE": true,
	"GONOSUMDB": true, "GONOPROXY": true, "GOTOOLCHAIN": true, "GOTMPDIR": true,
	"CARGO_HOME": true, "RUSTUP_HOME": true, "JAVA_HOME": true, "GRADLE_USER_HOME": true, "MAVEN_HOME": true, "M2_HOME": true,
	"NVM_DIR": true, "NVM_BIN": true, "VOLTA_HOME": true, "PNPM_HOME": true, "BUN_INSTALL": true,
	"VIRTUAL_ENV": true, "PYENV_ROOT": true, "UV_CACHE_DIR": true, "POETRY_HOME": true, "PIPX_HOME": true,
}

// verifyEnv returns the environment of a verification.
func verifyEnv(parent []string) []string {
	out := make([]string, 0, len(verifyEnvKeys)+3)
	for _, kv := range parent {
		k, _, _ := strings.Cut(kv, "=")
		if verifyEnvKeys[k] {
			out = append(out, kv)
		}
	}
	return append(out, "CI=true", "NO_COLOR=1", "TERM=dumb")
}

// execVerify runs a command in its own process group, killed with it on
// timeout; stdin is empty.
func execVerify(ctx context.Context, dir string, args, env []string, out *TailBuffer) (int, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = dir, env, out, out
	cmd.Stdin = bytes.NewReader(nil)
	cmd.WaitDelay = 5 * time.Second
	setProcessGroup(cmd)
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	if err != nil {
		return -1, err
	}
	return 0, nil
}

// TailBuffer keeps the last Max bytes written.
type TailBuffer struct {
	Max int
	mu  sync.Mutex
	buf []byte
	cut bool
}

func (b *TailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if b.Max > 0 && len(b.buf) > b.Max {
		b.buf = append([]byte(nil), b.buf[len(b.buf)-b.Max:]...)
		b.cut = true
	}
	return len(p), nil
}

// WriteString writes s.
func (b *TailBuffer) WriteString(s string) { _, _ = b.Write([]byte(s)) }

// Bytes returns the bytes kept.
func (b *TailBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.buf...)
}

// excerpt returns the end of an output (excerptLines lines, excerptBytes bytes).
func excerpt(out []byte) string {
	text := strings.TrimRight(strings.ToValidUTF8(string(out), "?"), "\n ")
	lines := strings.Split(text, "\n")
	if len(lines) > excerptLines {
		lines = lines[len(lines)-excerptLines:]
	}
	text = strings.Join(lines, "\n")
	if len(text) > excerptBytes {
		text = strings.ToValidUTF8(text[len(text)-excerptBytes:], "")
	}
	return text
}

// VerifySummary is the one-line result of a verification.
func VerifySummary(r domain.CheckpointVerify) string {
	var cmds []string
	for _, c := range r.Commands {
		mark := "✓"
		if c.Exit != 0 || c.TimedOut {
			mark = fmt.Sprintf("✗ (%d)", c.Exit)
		}
		cmds = append(cmds, c.Cmd+" "+mark)
	}
	parts := []string{i18n.T("tui.checkpoint.verify.status." + firstNonEmpty(r.Status, domain.VerifyRunning))}
	if r.Expect == sessionspec.VerifyFail {
		parts[0] += " " + i18n.T("tui.checkpoint.verify.expect_fail")
	}
	if r.Reason != "" {
		parts = append(parts, r.Reason)
	}
	if len(cmds) > 0 {
		parts = append(parts, strings.Join(cmds, " · "))
	}
	return strings.Join(parts, " — ")
}

// fingerprint identifies the state of the work of a session: HEAD, the
// uncommitted changes and the untracked files ("" when git is not there).
func (s *Service) fingerprint(ctx context.Context, sess *domain.Session) string {
	if s.Fingerprint != nil {
		return s.Fingerprint(ctx, sess.LaunchPath)
	}
	return GitFingerprint(ctx, sess.LaunchPath)
}

// GitFingerprint is the git fingerprint of the work in dir.
func GitFingerprint(ctx context.Context, dir string) string {
	if dir == "" {
		return ""
	}
	h := sha256.New()
	for _, args := range [][]string{
		{"rev-parse", "-q", "--verify", "HEAD"},
		{"diff", "HEAD", "--", ".", ":(exclude).beads"},
		{"status", "--porcelain", "-uall", "--", ".", ":(exclude).beads"},
	} {
		out, _ := gitutil.Command(ctx, dir, args...).Output()
		h.Write(out)
		h.Write([]byte{0})
	}
	out, _ := gitutil.Command(ctx, dir, "ls-files", "-o", "--exclude-standard", "-z").Output()
	for _, f := range strings.Split(string(out), "\x00") {
		if f == "" {
			continue
		}
		if fi, err := os.Stat(filepath.Join(dir, f)); err == nil {
			fmt.Fprintf(h, "%s %d %d\n", f, fi.Size(), fi.ModTime().UnixNano())
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// changedPaths are the paths the session changed: committed since it
// started (StartRef), uncommitted or untracked.
func (s *Service) changedPaths(ctx context.Context, sess *domain.Session) ([]string, error) {
	if s.Changed != nil {
		return s.Changed(ctx, sess.LaunchPath, sess.StartRef)
	}
	return GitChanged(ctx, sess.LaunchPath, sess.StartRef)
}

// GitChanged lists the paths changed in dir since the commit since ("" =
// HEAD), untracked files included.
func GitChanged(ctx context.Context, dir, since string) ([]string, error) {
	if dir == "" {
		return nil, errors.New("no location")
	}
	if since == "" {
		since = "HEAD"
	}
	diff, err := gitutil.Command(ctx, dir, "diff", "--name-only", since, "--", ".", ":(exclude).beads").Output()
	if err != nil {
		return nil, err
	}
	untracked, err := gitutil.Command(ctx, dir, "ls-files", "-o", "--exclude-standard").Output()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, l := range strings.Split(string(diff)+"\n"+string(untracked), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out, nil
}

func touches(paths, patterns []string) bool {
	for _, p := range paths {
		for _, pat := range patterns {
			if sessionspec.PathMatch(pat, p) {
				return true
			}
		}
	}
	return false
}

// verifyGate checks the verification of c before it passes (tryPass): a
// failure refuses it, unless the user overrode it at the paused checkpoint.
func (s *Service) verifyGate(ctx context.Context, sess *domain.Session, c sessionspec.CheckpointDef, needs bool) error {
	st, err := s.state(ctx, sess.ID)
	if err != nil {
		return err
	}
	ap, approved := st.Approved[c.ID]
	if needs && !approved {
		return ErrNotApproved
	}
	label := c.LabelFor(i18n.Locale())
	r, err := s.verification(ctx, sess, c, s.verifyWait())
	if errors.Is(err, ErrVerifyPending) {
		return &VerifyError{ID: c.ID, Label: label, Result: r, Pending: true}
	}
	if err != nil {
		return err
	}
	if r.OK() {
		return nil
	}
	if approved && ap.Override {
		_, err = s.update(ctx, sess.ID, func(st *domain.CheckpointState) error {
			s.record(st, domain.CheckpointOverridden, c.ID, ap.By, firstNonEmpty(ap.Message, VerifySummary(r)))
			return nil
		})
		return err
	}
	_, _ = s.update(ctx, sess.ID, func(st *domain.CheckpointState) error {
		// The validation was given on a failed verification: the agent
		// fixes the work and asks again.
		delete(st.Approved, c.ID)
		if st.Waiting == c.ID {
			st.Waiting = ""
		}
		return nil
	})
	return &VerifyError{ID: c.ID, Label: label, Result: r}
}

// VerifyResult returns the verification of a checkpoint of a session (the
// card), ok false when it has none.
func (s *Service) VerifyResult(ctx context.Context, sessionID, id string) (domain.CheckpointVerify, bool) {
	st, err := s.state(ctx, sessionID)
	if err != nil {
		return domain.CheckpointVerify{}, false
	}
	r, ok := st.Verify[id]
	return r, ok
}
