package teamstate

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

// Team brick catalogue under governance (ADR-054): published bricks,
// drafts, history and catalog.lock.
//
//	catalog/agents/<family>/<id>.md                 published agents (loaded)
//	catalog/skills/<ref>.md                         published skills (loaded)
//	catalog/drafts/<member>/agents/<family>/<id>.md drafts (never loaded)
//	catalog/drafts/<member>/skills/<ref>.md
//	catalog/history/agents/<id>/<v>.md              previous versions
//	catalog/history/skills/<ref>/<v>.md             (+ <v>.lock.toml)
//	catalog.lock                                    integrity of the published bricks
//
// Every name is checked before it becomes a path: kebab-case segments only,
// nothing outside catalog/.
const (
	BricksLockFile     = "catalog.lock"
	BrickDraftsDirName = "drafts"
	BrickHistoryDir    = "history"
	BrickKindAgent     = "agent"
	BrickKindSkill     = "skill"
	// TeamAgentFamily is the default family folder of a team agent.
	TeamAgentFamily = "team"
	// maxSkillDepth bounds the folders of a skill ref (category/…/name).
	maxSkillDepth = 4
)

// ErrInvalidBrickID is returned for a brick name that is not a safe path.
var ErrInvalidBrickID = fmt.Errorf("invalid brick id")

var brickSegment = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// ValidBrickID checks the id of a brick: an agent id (one kebab-case
// segment) or a skill ref (1 to 4 kebab-case segments separated by "/",
// not under templates/, the annexes folder).
func ValidBrickID(kind, id string) error {
	switch kind {
	case BrickKindAgent:
		if !brickSegment.MatchString(id) {
			return fmt.Errorf("%w: agent %q", ErrInvalidBrickID, id)
		}
	case BrickKindSkill:
		parts := strings.Split(id, "/")
		if len(parts) > maxSkillDepth || parts[0] == "templates" {
			return fmt.Errorf("%w: skill %q", ErrInvalidBrickID, id)
		}
		for _, p := range parts {
			if !brickSegment.MatchString(p) {
				return fmt.Errorf("%w: skill %q", ErrInvalidBrickID, id)
			}
		}
	default:
		return fmt.Errorf("%w: kind %q", ErrInvalidBrickID, kind)
	}
	return nil
}

// ValidBrickFamily checks the family folder of a team agent.
func ValidBrickFamily(family string) error {
	if !brickSegment.MatchString(family) {
		return fmt.Errorf("%w: family %q", ErrInvalidBrickID, family)
	}
	return nil
}

// BrickKey identifies a brick ("agent:<id>", "skill:<ref>").
func BrickKey(kind, id string) string { return kind + ":" + id }

// brickRel returns the file of a brick under base (relative to the repo
// root): base/agents/<family>/<id>.md or base/skills/<ref>.md.
func brickRel(base, kind, id, family string) (string, error) {
	if err := ValidBrickID(kind, id); err != nil {
		return "", err
	}
	if kind == BrickKindAgent {
		if err := ValidBrickFamily(family); err != nil {
			return "", err
		}
		return filepath.Join(base, "agents", family, id+".md"), nil
	}
	return filepath.Join(base, "skills", filepath.FromSlash(id)+".md"), nil
}

// PublishedBrickRel is the published file of a brick.
func PublishedBrickRel(kind, id, family string) (string, error) {
	return brickRel(CatalogDirName, kind, id, family)
}

// DraftBrickRel is the draft of a brick by member.
func DraftBrickRel(member, kind, id, family string) (string, error) {
	if _, err := SafeName(member); err != nil {
		return "", err
	}
	return brickRel(filepath.Join(CatalogDirName, BrickDraftsDirName, member), kind, id, family)
}

// BrickHistoryRel is catalog/history/<kind>s/<id>/<v>.md.
func BrickHistoryRel(kind, id string, version int) (string, error) {
	if err := ValidBrickID(kind, id); err != nil {
		return "", err
	}
	if version <= 0 {
		return "", fmt.Errorf("invalid brick version %d", version)
	}
	return filepath.Join(CatalogDirName, BrickHistoryDir, kind+"s", filepath.FromSlash(id), strconv.Itoa(version)+".md"), nil
}

// BrickHistoryMetaRel is the lock entry of a history version.
func BrickHistoryMetaRel(kind, id string, version int) (string, error) {
	rel, err := BrickHistoryRel(kind, id, version)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(rel, ".md") + ".lock.toml", nil
}

// BricksLock is catalog.lock: the version and hash of every published team
// brick. When it exists, only bricks whose file matches their entry are
// loaded; without it (catalogue committed by hand before ADR-054) the bricks
// are loaded and reported as not sealed.
//
//	[agents.hotfixer]
//	version = 2
//	path = "catalog/agents/team/hotfixer.md"
//	hash = "sha256:…"
//	published_by = "alice"
//	published_at = 2026-10-15T10:00:00Z
//	message = "…"
//
//	[skills."team/hotfix-protocol"]
//	…
type BricksLock struct {
	Agents map[string]BrickLockEntry `toml:"agents,omitempty"`
	Skills map[string]BrickLockEntry `toml:"skills,omitempty"`
}

// BrickLockEntry is the lock of one published brick.
type BrickLockEntry struct {
	Version     int       `toml:"version" json:"version"`
	Path        string    `toml:"path" json:"path"`
	Hash        string    `toml:"hash" json:"hash"`
	PublishedBy string    `toml:"published_by" json:"published_by"`
	PublishedAt time.Time `toml:"published_at" json:"published_at"`
	Message     string    `toml:"message,omitempty" json:"message,omitempty"`
}

func (l *BricksLock) entries(kind string) map[string]BrickLockEntry {
	if kind == BrickKindAgent {
		if l.Agents == nil {
			l.Agents = map[string]BrickLockEntry{}
		}
		return l.Agents
	}
	if l.Skills == nil {
		l.Skills = map[string]BrickLockEntry{}
	}
	return l.Skills
}

// Get returns the entry of a brick.
func (l *BricksLock) Get(kind, id string) (BrickLockEntry, bool) {
	if l == nil {
		return BrickLockEntry{}, false
	}
	e, ok := l.entries(kind)[id]
	return e, ok
}

// Set records the entry of a brick.
func (l *BricksLock) Set(kind, id string, e BrickLockEntry) { l.entries(kind)[id] = e }

// Keys returns the locked bricks ("agent:<id>", "skill:<ref>"), sorted.
func (l *BricksLock) Keys() []string {
	if l == nil {
		return nil
	}
	var out []string
	for id := range l.Agents {
		out = append(out, BrickKey(BrickKindAgent, id))
	}
	for id := range l.Skills {
		out = append(out, BrickKey(BrickKindSkill, id))
	}
	sort.Strings(out)
	return out
}

// CheckPath reports whether the recorded path of an entry is the published
// file of its brick (a lock entry never points outside catalog/).
func (e BrickLockEntry) CheckPath(kind, id string) error {
	clean := path.Clean(filepath.ToSlash(e.Path))
	if clean != filepath.ToSlash(e.Path) {
		return fmt.Errorf("%w: lock path %q", ErrUnsafeName, e.Path)
	}
	family := ""
	if kind == BrickKindAgent {
		parts := strings.Split(clean, "/")
		if len(parts) != 4 {
			return fmt.Errorf("%w: lock path %q", ErrUnsafeName, e.Path)
		}
		family = parts[2]
	}
	want, err := PublishedBrickRel(kind, id, family)
	if err != nil {
		return err
	}
	if filepath.ToSlash(want) != clean {
		return fmt.Errorf("%w: lock path %q", ErrUnsafeName, e.Path)
	}
	return nil
}

// ReadBricksLock reads catalog.lock of the repo at root; ok is false when
// the file does not exist (catalogue never sealed).
func ReadBricksLock(root string) (l *BricksLock, ok bool, err error) {
	data, err := os.ReadFile(filepath.Join(root, BricksLockFile))
	if err != nil {
		if os.IsNotExist(err) {
			return &BricksLock{}, false, nil
		}
		return nil, true, fmt.Errorf("reading %s: %w", BricksLockFile, err)
	}
	l = &BricksLock{}
	if err := toml.Unmarshal(data, l); err != nil {
		return nil, true, fmt.Errorf("parsing %s: %w", BricksLockFile, err)
	}
	return l, true, nil
}

// MarshalBricksLock encodes catalog.lock.
func MarshalBricksLock(l *BricksLock) ([]byte, error) {
	data, err := toml.Marshal(l)
	if err != nil {
		return nil, fmt.Errorf("marshaling %s: %w", BricksLockFile, err)
	}
	return data, nil
}

// MarshalBrickLockEntry encodes the <v>.lock.toml of a history version.
func MarshalBrickLockEntry(e BrickLockEntry) ([]byte, error) {
	data, err := toml.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("marshaling brick lock entry: %w", err)
	}
	return data, nil
}

// BrickHistoryVersion is a previous version of a brick.
type BrickHistoryVersion struct {
	Version int
	Entry   BrickLockEntry // zero when the metadata file is missing
	Path    string         // absolute path of the copy
}

// ListBrickHistory returns the previous versions of a brick, oldest first.
// It only reads files (usable inside a transaction).
func ListBrickHistory(root, kind, id string) ([]BrickHistoryVersion, error) {
	first, err := BrickHistoryRel(kind, id, 1)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, filepath.Dir(first))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []BrickHistoryVersion
	for _, e := range entries {
		v, err := strconv.Atoi(strings.TrimSuffix(e.Name(), ".md"))
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || err != nil || v <= 0 {
			continue
		}
		h := BrickHistoryVersion{Version: v, Path: filepath.Join(dir, e.Name())}
		if data, err := os.ReadFile(filepath.Join(dir, strconv.Itoa(v)+".lock.toml")); err == nil {
			_ = toml.Unmarshal(data, &h.Entry)
		}
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// Brick events, appended to the team events (projects/_team/events/).
const (
	EventBrickPublished = "brick.published"
	EventBrickAdopted   = "brick.adopted"
)

// BrickDraftInfo is a draft brick of a member.
type BrickDraftInfo struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Family string `json:"family,omitempty"`
	Rel    string `json:"path"` // relative to the repo root
}

// ListBrickDrafts returns the draft bricks of member under root, sorted by
// kind then id. Files whose names break the rules are skipped.
func ListBrickDrafts(root, member string) ([]BrickDraftInfo, error) {
	if _, err := SafeName(member); err != nil {
		return nil, err
	}
	base := filepath.Join(CatalogDirName, BrickDraftsDirName, member)
	var out []BrickDraftInfo
	agents, _ := filepath.Glob(filepath.Join(root, base, "agents", "*", "*.md"))
	for _, p := range agents {
		id := strings.TrimSuffix(filepath.Base(p), ".md")
		family := filepath.Base(filepath.Dir(p))
		rel, err := DraftBrickRel(member, BrickKindAgent, id, family)
		if err != nil || filepath.Join(root, rel) != p {
			continue
		}
		out = append(out, BrickDraftInfo{Kind: BrickKindAgent, ID: id, Family: family, Rel: rel})
	}
	skillRoot := filepath.Join(root, base, "skills")
	_ = filepath.WalkDir(skillRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".md" || !d.Type().IsRegular() {
			return nil //nolint:nilerr // unreadable or odd entries are skipped
		}
		r, _ := filepath.Rel(skillRoot, p)
		id := filepath.ToSlash(strings.TrimSuffix(r, ".md"))
		rel, err := DraftBrickRel(member, BrickKindSkill, id, "")
		if err != nil {
			return nil //nolint:nilerr // invalid names are skipped
		}
		out = append(out, BrickDraftInfo{Kind: BrickKindSkill, ID: id, Rel: rel})
		return nil
	})
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// WriteBrickDraftLocal writes the draft of a brick by member (a draft of the
// same agent in another family is removed). It returns the files to commit.
func (r *Repo) WriteBrickDraftLocal(member, kind, id, family string, data []byte) ([]string, error) {
	rel, err := DraftBrickRel(member, kind, id, family)
	if err != nil {
		return nil, err
	}
	var files []string
	if kind == BrickKindAgent {
		drafts, err := ListBrickDrafts(r.path, member)
		if err != nil {
			return nil, err
		}
		for _, d := range drafts {
			if d.Kind == kind && d.ID == id && d.Rel != rel {
				if err := os.Remove(filepath.Join(r.path, d.Rel)); err != nil && !os.IsNotExist(err) {
					return nil, err
				}
				files = append(files, d.Rel)
			}
		}
	}
	if err := writeRel(r.path, rel, data); err != nil {
		return nil, err
	}
	return append(files, rel), nil
}
