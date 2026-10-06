package domain

import (
	"context"
	"time"
)

// RemoteStatus is the lifecycle of a remote session (v5 phase 5).
type RemoteStatus string

const (
	RemoteSent     RemoteStatus = "sent"     // pipeline triggered
	RemoteRunning  RemoteStatus = "running"  // job started
	RemoteReady    RemoteStatus = "ready"    // finished: artifacts to fetch
	RemoteFailed   RemoteStatus = "failed"   // pipeline failed or canceled
	RemoteFetched  RemoteStatus = "fetched"  // artifacts fetched, session imported
	RemoteResolved RemoteStatus = "resolved" // Beads journal replayed
)

// RemoteRef locates a remote session: GitLab target, pipeline and packages
// (sessions.remote_ref, migration v39). No secret.
type RemoteRef struct {
	Target      string       `json:"target"`            // remote target name (hub.toml)
	URL         string       `json:"url"`               // GitLab instance
	RunnerID    int64        `json:"runner_project_id"` // oh-runner project
	ProjectPath string       `json:"project_path"`      // target project (GitLab path)
	ProjectDir  string       `json:"project_dir"`       // project base directory on the machine (Beads)
	ProjectID   int64        `json:"project_id"`        // target project (GitLab ID)
	Ref         string       `json:"ref"`               // branch the job starts from
	Commit      string       `json:"commit,omitempty"`  // commit at sending time
	Branch      string       `json:"branch,omitempty"`  // branch pushed by the job
	Pipeline    int64        `json:"pipeline"`          // pipeline ID
	PipelineURL string       `json:"pipeline_url"`      // web page of the pipeline
	BundleURL   string       `json:"bundle_url"`        // generic package of the bundle
	SessionURL  string       `json:"session_url"`       // generic package of the envelope
	Image       string       `json:"image"`             // project image of the job
	Tickets     []string     `json:"tickets,omitempty"` // Beads tickets reserved for the session
	TeamID      string       `json:"team_id,omitempty"` // team-state of the claims ("" = none)
	Status      RemoteStatus `json:"status"`            //
	MRURL       string       `json:"mr_url,omitempty"`  // merge request opened by the job
	Error       string       `json:"error,omitempty"`   // last failure
	SentAt      time.Time    `json:"sent_at"`           //
	UpdatedAt   time.Time    `json:"updated_at"`        //
}

// RemoteStore keeps the remote reference of sessions.
type RemoteStore interface {
	// GetRemoteRef returns the reference of a session (nil if local).
	GetRemoteRef(ctx context.Context, sessionID string) (*RemoteRef, error)
	// SetRemoteRef writes the reference of a session.
	SetRemoteRef(ctx context.Context, sessionID string, ref RemoteRef) error
	// ListRemote returns the sessions that have a reference, by session ID.
	ListRemote(ctx context.Context) (map[string]RemoteRef, error)
}
