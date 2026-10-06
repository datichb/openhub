package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/remote"
)

// Beads is the machine's bd (D9: Beads stays on the machine).
type Beads interface {
	// Show returns the `bd show --json` records of ids.
	Show(ctx context.Context, dir string, ids ...string) ([]json.RawMessage, error)
	// Children returns the `bd children --json` records of id.
	Children(ctx context.Context, dir, id string) ([]json.RawMessage, error)
	// Claim runs `bd update <id> --claim`.
	Claim(ctx context.Context, dir, id string) error
	// Unclaim runs `bd unclaim <id>`.
	Unclaim(ctx context.Context, dir, id string) error
}

// BdCLI runs the bd binary.
type BdCLI struct{}

func (BdCLI) run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := beads.BdCommand(ctx, append([]string{"-C", dir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("bd %s: %s", args[0], msg)
	}
	return out, nil
}

func decodeRecords(out []byte) ([]json.RawMessage, error) {
	var recs []json.RawMessage
	if err := json.Unmarshal(out, &recs); err != nil {
		return nil, fmt.Errorf("decoding bd output: %w", err)
	}
	return recs, nil
}

// Show implements Beads.
func (b BdCLI) Show(ctx context.Context, dir string, ids ...string) ([]json.RawMessage, error) {
	out, err := b.run(ctx, dir, append(append([]string{"show"}, ids...), "--json")...)
	if err != nil {
		return nil, err
	}
	return decodeRecords(out)
}

// Children implements Beads.
func (b BdCLI) Children(ctx context.Context, dir, id string) ([]json.RawMessage, error) {
	out, err := b.run(ctx, dir, "children", id, "--json")
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return nil, nil
	}
	return decodeRecords(out)
}

// Claim implements Beads.
func (b BdCLI) Claim(ctx context.Context, dir, id string) error {
	_, err := b.run(ctx, dir, "update", id, "--claim", "--json")
	return err
}

// Unclaim implements Beads.
func (b BdCLI) Unclaim(ctx context.Context, dir, id string) error {
	_, err := b.run(ctx, dir, "unclaim", id)
	return err
}

// issueHead is the part of a bd record used by oh.
type issueHead struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Assignee     string `json:"assignee"`
	UpdatedAt    string `json:"updated_at"`
	Revision     string `json:"revision"`
	Dependencies []struct {
		ID          string `json:"id"`
		DependsOnID string `json:"depends_on_id"`
	} `json:"dependencies"`
}

func headOf(raw json.RawMessage) (issueHead, error) {
	var h issueHead
	err := json.Unmarshal(raw, &h)
	if err == nil && h.ID == "" {
		err = errors.New("bd record without id")
	}
	return h, err
}

// Revision is the version of a record compared at replay: bd's revision,
// else its update time.
func (h issueHead) version() string {
	if h.Revision != "" {
		return h.Revision
	}
	return h.UpdatedAt
}

// TakeSnapshot reads the tickets, their dependencies and their children
// (P5-T06).
func TakeSnapshot(ctx context.Context, b Beads, dir string, tickets []string, now time.Time) (*remote.Snapshot, error) {
	snap := &remote.Snapshot{Schema: remote.SnapshotSchema, TakenAt: now.UTC(), Requested: append([]string{}, tickets...),
		Issues: map[string]json.RawMessage{}, Revisions: map[string]string{}, Children: map[string][]string{}}
	if len(tickets) == 0 {
		return snap, nil
	}
	recs, err := b.Show(ctx, dir, tickets...)
	if err != nil {
		return nil, err
	}
	related := map[string]bool{}
	add := func(raw json.RawMessage) error {
		h, err := headOf(raw)
		if err != nil {
			return err
		}
		snap.Issues[h.ID] = raw
		snap.Revisions[h.ID] = h.version()
		return nil
	}
	for _, raw := range recs {
		if err := add(raw); err != nil {
			return nil, err
		}
		h, _ := headOf(raw)
		for _, d := range h.Dependencies {
			for _, id := range []string{d.ID, d.DependsOnID} {
				if id != "" && id != h.ID {
					related[id] = true
				}
			}
		}
	}
	for _, t := range tickets {
		kids, err := b.Children(ctx, dir, t)
		if err != nil {
			return nil, err
		}
		for _, raw := range kids {
			h, err := headOf(raw)
			if err != nil {
				return nil, err
			}
			snap.Children[t] = append(snap.Children[t], h.ID)
			related[h.ID] = true
		}
	}
	var extra []string
	for id := range related {
		if _, ok := snap.Issues[id]; !ok {
			extra = append(extra, id)
		}
	}
	sort.Strings(extra)
	if len(extra) > 0 {
		recs, err := b.Show(ctx, dir, extra...)
		if err != nil {
			return nil, err
		}
		for _, raw := range recs {
			if err := add(raw); err != nil {
				return nil, err
			}
		}
	}
	return snap, nil
}

// Git is the git of the project on the machine.
type Git interface {
	RemoteURL(ctx context.Context, dir string) (string, error)
	CurrentBranch(ctx context.Context, dir string) (string, error)
	Head(ctx context.Context, dir string) (string, error)
	// FetchHead fetches branch from origin and returns its commit.
	FetchHead(ctx context.Context, dir, branch string) (string, error)
	// IsAncestor reports whether ancestor is reachable from commit.
	IsAncestor(ctx context.Context, dir, ancestor, commit string) (bool, error)
	Dirty(ctx context.Context, dir string) (bool, error)
	// ShowFile returns path at commit.
	ShowFile(ctx context.Context, dir, commit, path string) ([]byte, error)
}

// GitCLI runs git.
type GitCLI struct{}

func (GitCLI) git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(stderr.String()+" "+err.Error()))
	}
	return strings.TrimSpace(string(out)), nil
}

// RemoteURL implements Git.
func (g GitCLI) RemoteURL(ctx context.Context, dir string) (string, error) {
	return g.git(ctx, dir, "remote", "get-url", "origin")
}

// CurrentBranch implements Git ("" when detached).
func (g GitCLI) CurrentBranch(ctx context.Context, dir string) (string, error) {
	b, err := g.git(ctx, dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "", nil //nolint:nilerr // detached HEAD: no branch
	}
	return b, nil
}

// Head implements Git.
func (g GitCLI) Head(ctx context.Context, dir string) (string, error) {
	return g.git(ctx, dir, "rev-parse", "HEAD")
}

// FetchHead implements Git.
func (g GitCLI) FetchHead(ctx context.Context, dir, branch string) (string, error) {
	if _, err := g.git(ctx, dir, "fetch", "--quiet", "origin", "refs/heads/"+branch); err != nil {
		return "", err
	}
	return g.git(ctx, dir, "rev-parse", "FETCH_HEAD")
}

// IsAncestor implements Git.
func (g GitCLI) IsAncestor(ctx context.Context, dir, ancestor, commit string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "merge-base", "--is-ancestor", ancestor, commit)
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		return false, nil
	}
	return err == nil, err
}

// Dirty implements Git.
func (g GitCLI) Dirty(ctx context.Context, dir string) (bool, error) {
	out, err := g.git(ctx, dir, "status", "--porcelain", "--untracked-files=no")
	return out != "", err
}

// ShowFile implements Git.
func (g GitCLI) ShowFile(ctx context.Context, dir, commit, path string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "show", commit+":"+path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git show %s:%s: %s", commit, path, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}
