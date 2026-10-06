package remote

import (
	"encoding/json"
	"time"
)

// ManifestSchema is the version of the session envelope format.
const ManifestSchema = 1

// Files of the session envelope (oh-session package).
const (
	ManifestFile = "manifest.json"
	SnapshotFile = "beads-snapshot.json"
)

// Artifacts of the run job (in OutDir).
const (
	JournalFile = "journal.jsonl"
	SummaryFile = "summary.json"
	ExportFile  = "session.export"
)

// WorkRoot is where the job clones the target project
// (WorkRoot/<project name>). The prompt is rendered with this location.
const WorkRoot = "/tmp/oh-work"

// Manifest describes a remote session for the job. No secret.
type Manifest struct {
	Schema     int       `json:"schema"`
	SessionID  string    `json:"session_id"`
	OhVersion  string    `json:"oh_version"`
	SentAt     time.Time `json:"sent_at"`
	Target     string    `json:"target"`
	BundleHash string    `json:"bundle_hash"`

	Workflow ManifestWorkflow `json:"workflow"`
	Mode     string           `json:"mode"`
	Title    string           `json:"title"`
	// EntryAgent starts the session; Prompt is rendered for WorkDir.
	EntryAgent string         `json:"entry_agent"`
	Prompt     string         `json:"prompt"`
	Inputs     map[string]any `json:"inputs,omitempty"`

	// Project is the target project; the job clones Ref (at Commit when
	// set) into WorkDir and pushes Branch.
	Project ManifestProject `json:"project"`
	WorkDir string          `json:"work_dir"`
	Ref     string          `json:"ref"`
	Commit  string          `json:"commit,omitempty"`
	Branch  string          `json:"branch"`

	// Provider of the jobs (the key is the OH_LLM_KEY CI variable).
	Provider      string   `json:"provider"`
	AllowedModels []string `json:"allowed_models,omitempty"`
	MaxTokens     int64    `json:"max_tokens,omitempty"`

	// BeadsAllow is the workflow beads.allow (nil = read-only default).
	BeadsAllow []string `json:"beads_allow"`
	// Tickets are the Beads tickets of the session (in the snapshot).
	Tickets []string `json:"tickets,omitempty"`
	// Checkpoints are the checkpoints that wait for the user in Mode, with
	// their remote policy (auto | defer), answered by the policy responder.
	Checkpoints []ManifestCheckpoint `json:"checkpoints,omitempty"`
	// Team is where the runner publishes the progress of the claims.
	Team *ManifestTeam `json:"team,omitempty"`
}

// ManifestWorkflow identifies the workflow.
type ManifestWorkflow struct {
	ID      string `json:"id"`
	Layer   string `json:"layer,omitempty"`
	Version int    `json:"version,omitempty"`
	Risk    string `json:"risk,omitempty"`
}

// ManifestProject is the target project.
type ManifestProject struct {
	ID       int64  `json:"id"`        // GitLab ID (selects OH_PROJECT_TOKEN_<id>)
	Path     string `json:"path"`      // GitLab full path
	Name     string `json:"name"`      // oh project name
	OhID     string `json:"oh_id"`     // oh project ID (team-state claims)
	CloneURL string `json:"clone_url"` // HTTPS clone URL
}

// ManifestCheckpoint is the remote handling of a checkpoint.
type ManifestCheckpoint struct {
	ID        string `json:"id"`
	Label     string `json:"label,omitempty"`
	Policy    string `json:"policy"` // auto | defer
	Mandatory bool   `json:"mandatory,omitempty"`
}

// ManifestTeam is the team-state of the claims.
type ManifestTeam struct {
	ID       string `json:"id"`
	Repo     string `json:"repo"` // HTTPS URL of the team-state repository
	MemberID string `json:"member_id"`
}

// SnapshotSchema is the version of the Beads snapshot format.
const SnapshotSchema = 1

// Snapshot is the Beads state sent with a remote session: the tickets of
// the session, their dependencies and children, as `bd show --json` prints
// them (served by the fake bd in journal mode), and their revision at
// sending time (checked when the journal is replayed).
type Snapshot struct {
	Schema    int                        `json:"schema"`
	TakenAt   time.Time                  `json:"taken_at"`
	Requested []string                   `json:"requested"`
	Issues    map[string]json.RawMessage `json:"issues"`
	Revisions map[string]string          `json:"revisions"`
	// Children lists the child ids of each issue (bd children).
	Children map[string][]string `json:"children,omitempty"`
}
