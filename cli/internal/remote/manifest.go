package remote

import (
	"encoding/json"
	"time"
)

// ManifestSchema is the version of the session envelope format.
const ManifestSchema = 3 // 2: journal key (S-04); 3: journal key sealed for the target (N21)

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
	ReportsFile = "reports.json"
	WikiFile    = "wiki.json"
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
	// JournalKeySealed is the journal key of the session (base64), which
	// authenticates its Beads journal (S-04), encrypted for the target
	// (SealJournalKey): only the run job, which holds the private key of
	// oh-runner (VarJournalKey), opens it, in memory. The runner seals the
	// journal with it (Summary.JournalMAC) and the machine, which keeps the
	// key in its keychain, checks the seal before any replay. Members who
	// read the package registry of oh-runner never see the key.
	JournalKeySealed string `json:"journal_key_sealed,omitempty"`
	// LegacyJournalKey is the clear key of schema 2 envelopes: refused.
	LegacyJournalKey string `json:"journal_key,omitempty"`
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

// SummarySchema is the version of summary.json.
const SummarySchema = 2 // 2: verifications run by oh in the job (R3-remote)

// Outcomes of a remote session.
const (
	OutcomeCompleted = "completed" // the turn ended
	OutcomeDeferred  = "deferred"  // stopped at a remote: defer checkpoint (MR ready, continue locally)
	OutcomeQuestion  = "question"  // an agent question has no default answer: continue locally
	OutcomeFailed    = "failed"    // error, budget, circuit breaker or job failure
)

// Summary is the result of a remote session (summary.json artifact).
type Summary struct {
	Schema    int       `json:"schema"`
	SessionID string    `json:"session_id"`
	Outcome   string    `json:"outcome"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`

	// Deferred is the checkpoint the session stopped at (OutcomeDeferred);
	// Question the pending question (OutcomeQuestion).
	Deferred *SummaryDecision `json:"deferred,omitempty"`
	Question *SummaryDecision `json:"question,omitempty"`
	// Decisions are the answers of the policy responder.
	Decisions []SummaryDecision `json:"decisions,omitempty"`
	// Verifications are the last verification run by oh in the job for
	// each verified checkpoint (R3-remote), by checkpoint id.
	Verifications []SummaryVerification `json:"verifications,omitempty"`

	Branch     string `json:"branch,omitempty"`
	BaseCommit string `json:"base_commit,omitempty"`
	Commit     string `json:"commit,omitempty"` // pushed commit ("" = nothing pushed)
	MRURL      string `json:"mr_url,omitempty"`

	Cost            float64 `json:"cost"`
	TokensIn        int64   `json:"tokens_in"`
	TokensOut       int64   `json:"tokens_out"`
	TokensReasoning int64   `json:"tokens_reasoning,omitempty"`
	TokensCacheRead int64   `json:"tokens_cache_read,omitempty"`
	Model           string  `json:"model,omitempty"`

	Outputs        map[string]any `json:"outputs,omitempty"`
	JournalEntries int            `json:"journal_entries"`
	// JournalMAC seals journal.jsonl (JournalSeal with the session's
	// journal key).
	JournalMAC string `json:"journal_mac,omitempty"`
	// Sessions are the exported tool sessions, parents first.
	Sessions []string `json:"sessions,omitempty"`
	Text     string   `json:"text,omitempty"` // last text of the assistant
}

// SummaryDecision is a decision of the session and its answer.
type SummaryDecision struct {
	ID        string    `json:"id"`                  // checkpoint id or decision id
	Kind      string    `json:"kind"`                // checkpoint | permission | question | error | budget | circuit
	Label     string    `json:"label,omitempty"`     // checkpoint label, permission action, question title
	Resources []string  `json:"resources,omitempty"` // permission resources
	Answer    string    `json:"answer"`              // approved | rejected | deferred | pending | stopped
	At        time.Time `json:"at"`
}

// SummaryVerification is the verification of a checkpoint run by oh in the
// job, where the work is (same commands, account of the agents, filtered
// environment, timeout and bounded output as on the machine).
type SummaryVerification struct {
	Checkpoint string                 `json:"checkpoint"`
	Status     string                 `json:"status"` // pass | fail | none | unavailable | running
	Expect     string                 `json:"expect,omitempty"`
	Reason     string                 `json:"reason,omitempty"`
	Excerpt    string                 `json:"excerpt,omitempty"`
	Commands   []SummaryVerifyCommand `json:"commands,omitempty"`
	Started    time.Time              `json:"started"`
	Finished   *time.Time             `json:"finished,omitempty"`
}

// SummaryVerifyCommand is a command of a verification and its result.
type SummaryVerifyCommand struct {
	Kind     string `json:"kind,omitempty"`
	Cmd      string `json:"cmd"`
	Exit     int    `json:"exit"`
	TimedOut bool   `json:"timed_out,omitempty"`
	Millis   int64  `json:"ms"`
}

// ExportSchema is the version of session.export.
const ExportSchema = 1

// Export is the session.export artifact: the transcripts of the session and
// of its sub-agent sessions, parents first (import order).
type Export struct {
	Schema   int               `json:"schema"`
	Sessions []json.RawMessage `json:"sessions"`
}

// ReportsSchema is the version of reports.json.
const ReportsSchema = 1

// Reports is the reports.json artifact: the reports the agents filed with
// workflow_report in the job, imported into the session when it is fetched
// (cleaned and bounded again: the artifact is not trusted).
type Reports struct {
	Schema  int           `json:"schema"`
	Reports []ReportEntry `json:"reports"`
}

// ReportEntry is a report of reports.json.
type ReportEntry struct {
	Seq         int       `json:"seq"`
	Agent       string    `json:"agent"`
	ToolSession string    `json:"tool_session,omitempty"`
	Title       string    `json:"title"`
	CreatedAt   time.Time `json:"created_at"`
	Body        string    `json:"body"`
}

// WikiSchema is the version of wiki.json.
const WikiSchema = 1

// Wiki is the wiki.json artifact: the proposals the agents filed with
// wiki_propose in the job (never applied there), queued on the machine when
// the session is fetched, to be validated there (cleaned and bounded again:
// the artifact is not trusted).
type Wiki struct {
	Schema    int         `json:"schema"`
	Proposals []WikiEntry `json:"proposals"`
}

// WikiEntry is a proposal of wiki.json.
type WikiEntry struct {
	ID          string    `json:"id"` // wp-<n> in the job
	Agent       string    `json:"agent"`
	ToolSession string    `json:"tool_session,omitempty"`
	Page        string    `json:"page"`
	Section     string    `json:"section,omitempty"`
	Content     string    `json:"content"`
	Tag         string    `json:"tag"`
	Source      string    `json:"source,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
