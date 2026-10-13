package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/procinfo"
)

// Hook tokens (audit v5, S-06, DC24): the hook mode of the gateway (`bd
// hooks run` whatever beads.allow says) is granted only with a one-time
// token. The daemon issues it on its socket to a fake bd that git runs
// from the hook of the session location: an ancestor of the calling
// process runs that hook file, and an ancestor of it is git. The token is
// bound to the session holder and the hook, valid for one command and a
// short time.

// hookTokenTTL is the validity of a hook token.
const hookTokenTTL = 30 * time.Second

type hookTicket struct {
	session, holder, hook string
	exp                   time.Time
}

// IssueHookToken returns a one-time token for hook, for the holder of g.
func (s *Store) IssueHookToken(g Grant, hook string) string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("gateway: crypto/rand failed: %v", err))
	}
	tok := "ohh_" + hex.EncodeToString(b)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hooks == nil {
		s.hooks = map[string]hookTicket{}
	}
	now := time.Now()
	for h, t := range s.hooks {
		if now.After(t.exp) {
			delete(s.hooks, h)
		}
	}
	s.hooks[hashToken(tok)] = hookTicket{session: g.SessionID, holder: g.Holder, hook: hook, exp: now.Add(hookTokenTTL)}
	return tok
}

// consumeHookToken uses a hook token: true when it was issued for the
// holder of g and hook, and has not expired nor been used.
func (s *Store) consumeHookToken(tok string, g Grant, hook string) bool {
	if tok == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	h := hashToken(tok)
	t, ok := s.hooks[h]
	if !ok {
		return false
	}
	delete(s.hooks, h)
	return t.session == g.SessionID && t.holder == g.Holder && t.hook == hook && time.Now().Before(t.exp)
}

// ErrNotFromGit: the process asking for a hook token is not run by git
// from the hook of the session location.
var ErrNotFromGit = errors.New("not run by git from the Beads hook of the session")

// maxHookDepth bounds the walk up the process tree (fake bd → timeout →
// hook shell → git → …).
const maxHookDepth = 12

// CheckHookCaller checks that pid is run by git from the hook of location
// (read: procinfo.Get; procinfo.ErrUnsupported is returned as is).
func CheckHookCaller(ctx context.Context, pid int, location, hook string, read func(int) (procinfo.Process, error)) error {
	hookFile, top, err := hookPath(ctx, location, hook)
	if err != nil {
		return err
	}
	inHook := false
	for depth, cur := 0, pid; depth < maxHookDepth && cur > 1; depth++ {
		p, err := read(cur)
		switch {
		case errors.Is(err, procinfo.ErrUnsupported):
			return err
		case err != nil && p.PPID <= 0:
			return fmt.Errorf("%w: %v", ErrNotFromGit, err)
		}
		switch {
		case !inHook && runsFile(p.Args, hookFile, top):
			inHook = true
		case inHook && isGit(p.Args):
			return nil
		}
		cur = p.PPID
	}
	return ErrNotFromGit
}

// hookPath is the hook file of location (hooks directory of git, which
// follows core.hooksPath) and the top of its work tree.
func hookPath(ctx context.Context, location, hook string) (file, top string, err error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// Not gitutil.Command: the hooks path of the repository is what is
	// asked (rev-parse runs nothing of it); as its owner in a remote job.
	cmd := exec.CommandContext(ctx, "git", "-C", location, "rev-parse", "--show-toplevel", "--git-path", "hooks")
	gitutil.AsOwner(cmd, location)
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrNotFromGit, err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		return "", "", ErrNotFromGit
	}
	top = resolved(lines[0])
	dir := lines[1]
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(location, dir)
	}
	return resolved(filepath.Join(dir, hook)), top, nil
}

func resolved(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

// runsFile reports whether a command line runs file (interpreter, script).
func runsFile(args []string, file, top string) bool {
	for i, a := range args {
		if i > 2 {
			break
		}
		if a == "" || strings.HasPrefix(a, "-") {
			continue
		}
		if !filepath.IsAbs(a) {
			a = filepath.Join(top, a)
		}
		if resolved(a) == file {
			return true
		}
	}
	return false
}

func isGit(args []string) bool {
	if len(args) == 0 {
		return false
	}
	base := strings.ToLower(filepath.Base(args[0]))
	return base == "git" || base == "git.exe"
}
