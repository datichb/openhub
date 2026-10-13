package gateway

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
)

// The Beads database of a session is fixed when its grant is issued (audit
// v5, S-08): bd finds its workspace from the working directory (a `.beads`
// in a sub-directory, `.beads/redirect`, the configuration files), all of
// which an agent can write. The gateway resolves the workspace once with
// bd itself (`bd where`, so that worktrees and redirects set up by the user
// keep working), then runs every command with BEADS_DIR set to it and
// refuses them when the files that choose the database changed since.

// baseFiles choose the database of a Beads workspace.
var baseFiles = []string{"redirect", "metadata.json", "config.yaml"}

// Base is the Beads workspace of a grant.
type Base struct {
	Dir   string            `json:"dir"`             // "" = no Beads workspace when the session started
	Files map[string]string `json:"files,omitempty"` // base file → SHA-256 ("" = absent)
}

// journalBase is the base of the job daemon of a remote runner: its bd is
// the fake bd in journal mode (cmd/runner.go, environment of the daemon),
// reading a snapshot and writing a journal, without any workspace.
const journalBase = "journal"

func journalMode() bool { return os.Getenv(beadswire.EnvMode) == beadswire.ModeJournal }

// baseTimeout bounds `bd where` when a grant is issued.
const baseTimeout = 20 * time.Second

// ResolveBase asks bd for the workspace of location (nil when bd cannot be
// run: the base is then resolved at the first command).
func (b *Beads) ResolveBase(ctx context.Context, location string) *Base {
	if journalMode() {
		return &Base{Dir: journalBase}
	}
	bin, err := b.binary()
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, baseTimeout)
	defer cancel()
	var out bytes.Buffer
	for attempt := 1; ; attempt++ {
		out.Reset()
		cmd := exec.CommandContext(ctx, bin, "where", "--json")
		cmd.Dir = location
		cmd.Env = beadsEnv(os.Environ(), "", false)
		cmd.Stdout = &out
		err := cmd.Run() // bd exits 1 without a workspace, with a JSON answer
		if attempt >= startAttempts || !transientStart(err) || sleepCtx(ctx, time.Duration(attempt)*50*time.Millisecond) != nil {
			break
		}
	}
	var w struct {
		Path  string `json:"path"`
		Error string `json:"error"`
	}
	switch err := json.Unmarshal(out.Bytes(), &w); {
	case err != nil:
		return nil // bd failed: resolved again at the first command
	case w.Error == "no_beads_directory":
		return &Base{}
	case w.Path == "":
		return nil
	}
	dir := filepath.Clean(w.Path)
	return &Base{Dir: dir, Files: hashBaseFiles(dir)}
}

// startAttempts bounds the runs of `bd where` that could not start.
const startAttempts = 3

// transientStart reports a start failure worth retrying: bd being written
// or replaced (ETXTBSY), or the process limit reached under load (EAGAIN).
func transientStart(err error) bool {
	return errors.Is(err, syscall.ETXTBSY) || errors.Is(err, syscall.EAGAIN)
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func hashBaseFiles(dir string) map[string]string {
	m := make(map[string]string, len(baseFiles))
	for _, f := range baseFiles {
		m[f] = hashFile(filepath.Join(dir, f))
	}
	return m
}

// hashFile is the SHA-256 of a file, "" when absent ("?" when unreadable).
func hashFile(p string) string {
	data, err := os.ReadFile(p)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return ""
	case err != nil:
		return "?"
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// check refuses a command when the workspace is missing or a base file
// changed since the grant was issued.
func (b *Base) check() error {
	if b.Dir == journalBase {
		return nil
	}
	if b.Dir == "" {
		return refuse("cmd.gateway.beads.no_base")
	}
	if st, err := os.Stat(b.Dir); err != nil || !st.IsDir() {
		return refuse("cmd.gateway.beads.base_changed", b.Dir)
	}
	for _, f := range baseFiles {
		if hashFile(filepath.Join(b.Dir, f)) != b.Files[f] {
			return refuse("cmd.gateway.beads.base_changed", filepath.Join(b.Dir, f))
		}
	}
	return nil
}

// base returns the workspace of a grant, resolved now for grants issued by
// an older daemon (and remembered).
func (b *Beads) base(ctx context.Context, g Grant) *Base {
	if g.Base != nil {
		return g.Base
	}
	base := b.ResolveBase(ctx, g.Location)
	if base != nil && b.Store != nil {
		b.Store.setBase(g, base)
	}
	return base
}
