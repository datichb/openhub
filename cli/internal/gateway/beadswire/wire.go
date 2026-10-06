// Package beadswire is the wire format between the fake `bd` installed in
// oh container images and the Beads gateway of the oh daemon (P4-T07), plus
// the journal entries of the remote mode (P5-T12). It has no dependency so
// that the fake bd stays small.
package beadswire

import "time"

// Environment variables read by the fake bd.
const (
	// EnvURL is the gateway base URL seen from the runtime
	// (http://host.docker.internal:<port>/oh-gateway), set per server group.
	EnvURL = "OH_GATEWAY_URL"
	// EnvToken is the session gateway token (ohg_…), set per session.
	EnvToken = "OH_GATEWAY_TOKEN"
	// EnvMode selects the backend: "gateway" (default) or "journal" (remote
	// runs: reads from a snapshot, writes to a journal — P5-T12).
	EnvMode = "OH_BD_MODE"
	// EnvSnapshot and EnvJournal locate the files of the journal mode.
	EnvSnapshot = "OH_BD_SNAPSHOT"
	EnvJournal  = "OH_BD_JOURNAL"
)

// Modes of the fake bd.
const (
	ModeGateway = "gateway"
	ModeJournal = "journal"
)

// Prefix is the first path segment of the gateways on the daemon listener.
const Prefix = "oh-gateway"

// ExecPath is the path of the Beads command endpoint under the gateway URL.
const ExecPath = "/beads/v1/exec"

// TokenPrefix starts every gateway token.
const TokenPrefix = "ohg_"

// MaxStdin bounds the standard input forwarded by the fake bd.
const MaxStdin = 4 << 20

// ExecRequest is the body of POST <gateway>/beads/v1/exec.
type ExecRequest struct {
	Argv  []string `json:"argv"`            // arguments after "bd"
	Cwd   string   `json:"cwd"`             // working directory in the runtime
	Stdin []byte   `json:"stdin,omitempty"` // forwarded only for --stdin or "-"
}

// ExecResponse relays the outcome of the real bd. A refused command has a
// non-empty Error, no output and ExitCode 1.
type ExecResponse struct {
	Stdout   []byte `json:"stdout,omitempty"`
	Stderr   []byte `json:"stderr,omitempty"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// JournalEntry is one Beads command recorded by the journal mode, replayed
// on the machine when a remote session comes back (P5-T17).
type JournalEntry struct {
	Time time.Time `json:"time"`
	Argv []string  `json:"argv"`
	Cwd  string    `json:"cwd"`
	// Seq orders the entries (1, 2…), Stdin is the input of --stdin / "-",
	// Placeholder the temporary id answered for a `create` (replaced by the
	// real id at replay). Added in phase 5 (P5-T12).
	Seq         int    `json:"seq,omitempty"`
	Stdin       []byte `json:"stdin,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
}
