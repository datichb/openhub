package teamstate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/datichb/openhub/cli/internal/gitremote"
)

// Identity of a project in the team-state: projects/<id>/project.toml records
// the git remote of the project, so that what the team-state keeps under
// projects/<id>/ (claims, events, workflows) is tied to a repository and no
// longer to a name or to the URL of a merge request. A project that is created
// again on a machine gets a new ID: its former claims are found again by the
// remote.
//
// The remote is recorded as the repository designates it, host and path
// ("git.example.com/group/app"): without scheme, user, token or ".git", so that
// two spellings of the same remote (https, ssh, scp-like) are the same identity
// and nothing secret reaches the team-state.

// ProjectIdentityFile is the name of the file, in projects/<id>/.
const ProjectIdentityFile = "project.toml"

// ErrProjectIdentityNotFound is returned when a project has no identity file.
var ErrProjectIdentityNotFound = errors.New("project identity not found")

// ErrProjectIdentityInvalid is returned for an identity that cannot be
// recorded (no ID, no remote, a remote that is a local path).
var ErrProjectIdentityInvalid = errors.New("project identity invalid")

// ProjectIdentity is the content of projects/<id>/project.toml.
type ProjectIdentity struct {
	// ID is the ID of the project; it is the name of the folder.
	ID string `toml:"id"`
	// Name is the name of the project when it was recorded (display only).
	Name string `toml:"name,omitempty"`
	// Remote is the repository of the project: "host/path".
	Remote string `toml:"remote"`
	// RecordedBy is the member who recorded it; RecordedAt when.
	RecordedBy string  `toml:"recorded_by,omitempty"`
	RecordedAt Instant `toml:"recorded_at,omitempty"`
}

// NewProjectIdentity builds the identity of project id from a git remote in any
// spelling (https, ssh, scp-like).
func NewProjectIdentity(id, name, remote, by string, now time.Time) (ProjectIdentity, error) {
	if _, err := SafeName(id); err != nil {
		return ProjectIdentity{}, fmt.Errorf("%w: project ID: %w", ErrProjectIdentityInvalid, err)
	}
	host, path, ok := gitremote.Split(remote)
	if !ok || host == "" || path == "" {
		return ProjectIdentity{}, fmt.Errorf("%w: %q is not a git remote", ErrProjectIdentityInvalid, redactRemote(remote))
	}
	return ProjectIdentity{ID: id, Name: name, Remote: host + "/" + path, RecordedBy: by, RecordedAt: InstantOf(now)}, nil
}

// redactRemote drops the user (and any token) of a remote before it reaches an
// error message.
func redactRemote(remote string) string {
	if host, path, ok := gitremote.Split(remote); ok {
		return host + "/" + path
	}
	return "(local path)"
}

// Matches reports whether remote, in any spelling, is the repository of the
// project.
func (i ProjectIdentity) Matches(remote string) bool {
	return gitremote.Same(i.Remote, remote)
}

// ProjectIdentityRel is the repo-relative path of the identity of a project.
func ProjectIdentityRel(project string) (string, error) {
	if _, err := SafeName(project); err != nil {
		return "", fmt.Errorf("invalid project name: %w", err)
	}
	return filepath.Join("projects", project, ProjectIdentityFile), nil
}

func (r *Repo) readProjectIdentity(project string) (*ProjectIdentity, error) {
	rel, err := ProjectIdentityRel(project)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(r.path, rel))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrProjectIdentityNotFound
		}
		return nil, fmt.Errorf("reading %s: %w", rel, err)
	}
	var id ProjectIdentity
	if err := toml.Unmarshal(data, &id); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", rel, err)
	}
	if id.ID == "" {
		id.ID = project
	}
	return &id, nil
}

// ReadProjectIdentity reads the identity of project from the clone. A project
// without one gives ErrProjectIdentityNotFound.
func (r *Repo) ReadProjectIdentity(ctx context.Context, project string) (*ProjectIdentity, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.readProjectIdentity(project)
}

// ListProjectIdentities reads the identity of every project of the clone that
// has one, by ID. A file that cannot be read is left out and its project is
// named in unreadable, with the error, so that the doctor can tell.
func (r *Repo) ListProjectIdentities(ctx context.Context) (found []ProjectIdentity, unreadable map[string]error, err error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	entries, err := os.ReadDir(filepath.Join(r.path, "projects"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id, err := r.readProjectIdentity(e.Name())
		switch {
		case errors.Is(err, ErrProjectIdentityNotFound):
			continue
		case err != nil:
			if unreadable == nil {
				unreadable = map[string]error{}
			}
			unreadable[e.Name()] = err
		default:
			found = append(found, *id)
		}
	}
	sort.Slice(found, func(a, b int) bool { return found[a].ID < found[b].ID })
	return found, unreadable, nil
}

// FindProjectByRemote returns the identities whose remote is the given one, in
// any spelling: the project IDs a repository has been known under in the
// team-state.
func (r *Repo) FindProjectByRemote(ctx context.Context, remote string) ([]ProjectIdentity, error) {
	all, _, err := r.ListProjectIdentities(ctx)
	if err != nil {
		return nil, err
	}
	var out []ProjectIdentity
	for _, id := range all {
		if id.Matches(remote) {
			out = append(out, id)
		}
	}
	return out, nil
}

// WriteProjectIdentity records the identity of a project in one commit and one
// push; it replaces the file when there is one. changed is false when the file
// already held this identity (nothing is committed).
func (r *Repo) WriteProjectIdentity(ctx context.Context, id ProjectIdentity) (changed bool, err error) {
	err = r.withWriteLock(ctx, func(ctx context.Context) error {
		changed, err = r.writeProjectIdentity(ctx, id, true)
		return err
	})
	if err != nil {
		return false, err
	}
	return changed, nil
}

// EnsureProjectIdentity records the identity of a project when it has none and
// leaves an existing file alone, whatever it holds (the doctor tells when it is
// not the remote of the project). One commit and one push; none when the file
// exists.
func (r *Repo) EnsureProjectIdentity(ctx context.Context, id ProjectIdentity) (created bool, err error) {
	err = r.withWriteLock(ctx, func(ctx context.Context) error {
		created, err = r.writeProjectIdentity(ctx, id, false)
		return err
	})
	if err != nil {
		return false, err
	}
	return created, nil
}

// writeProjectIdentity is the body of the two writers, within the transaction
// of ctx.
func (r *Repo) writeProjectIdentity(ctx context.Context, id ProjectIdentity, replace bool) (bool, error) {
	if id.Remote == "" {
		return false, fmt.Errorf("%w: no remote", ErrProjectIdentityInvalid)
	}
	rel, err := ProjectIdentityRel(id.ID)
	if err != nil {
		return false, err
	}
	have, err := r.readProjectIdentity(id.ID)
	switch {
	case err == nil && !replace:
		return false, nil
	case err == nil && have.Remote == id.Remote && have.Name == id.Name:
		return false, nil
	case err != nil && !errors.Is(err, ErrProjectIdentityNotFound) && !replace:
		return false, nil // a file that cannot be read is the doctor's to tell
	}
	if id.RecordedAt == "" {
		id.RecordedAt = InstantOf(time.Now())
	}
	data, err := toml.Marshal(id)
	if err != nil {
		return false, fmt.Errorf("marshaling %s: %w", rel, err)
	}
	if err := writeRel(r.path, rel, data); err != nil {
		return false, fmt.Errorf("writing %s: %w", rel, err)
	}
	msg := fmt.Sprintf("project: identity of %s (%s)", id.ID, strings.TrimSpace(id.Remote))
	return true, r.commitAndPush(ctx, msg, rel)
}

// ProjectIdentityStore reads and writes the identity of the projects.
type ProjectIdentityStore interface {
	ReadProjectIdentity(ctx context.Context, project string) (*ProjectIdentity, error)
	ListProjectIdentities(ctx context.Context) (found []ProjectIdentity, unreadable map[string]error, err error)
	FindProjectByRemote(ctx context.Context, remote string) ([]ProjectIdentity, error)
	WriteProjectIdentity(ctx context.Context, id ProjectIdentity) (changed bool, err error)
	EnsureProjectIdentity(ctx context.Context, id ProjectIdentity) (created bool, err error)
}

var _ ProjectIdentityStore = (*Repo)(nil)
