package daemon

import (
	"github.com/datichb/openhub/cli/internal/daemon/api"
	dclient "github.com/datichb/openhub/cli/internal/daemon/client"
)

// Temporary aliases (v5.1, lot C5): the transport types, the routes, the
// sentinel errors and the client moved to daemon/api and daemon/client so
// that the packages below cmd no longer import the whole daemon. cmd (and
// the code of the daemon itself) still use these names; they are removed
// with the migration of cmd (lot C8) — use daemon/client and daemon/api.

// apiPrefix is the version prefix of the routes (api.Prefix).
const apiPrefix = api.Prefix

// CapabilityHeader carries the capability on privileged requests.
const CapabilityHeader = api.CapabilityHeader

type (
	// Paths: use api.Paths.
	Paths = api.Paths

	// Client: use client.Client.
	Client = dclient.Client
	// EnsureOptions: use client.EnsureOptions.
	EnsureOptions = dclient.EnsureOptions

	Health               = api.Health
	GrantRequest         = api.GrantRequest
	GrantResponse        = api.GrantResponse
	SecretRequest        = api.SecretRequest
	UsageResponse        = api.UsageResponse
	AuthRejected         = api.AuthRejected
	PendingGrant         = api.PendingGrant
	ListenRequest        = api.ListenRequest
	ListenResponse       = api.ListenResponse
	GatewayGrantRequest  = api.GatewayGrantRequest
	GatewayGrantResponse = api.GatewayGrantResponse

	ClientKind       = api.ClientKind
	HeartbeatRequest = api.HeartbeatRequest
	QuitPolicy       = api.QuitPolicy
	PolicyRequest    = api.PolicyRequest
	StreamEvent      = api.StreamEvent
	AttachedClient   = api.AttachedClient

	WorkflowCheckpointRequest = api.WorkflowCheckpointRequest
	WorkflowOutputRequest     = api.WorkflowOutputRequest
	WorkflowRulesRequest      = api.WorkflowRulesRequest
	WorkflowReportRequest     = api.WorkflowReportRequest
	WorkflowReport            = api.WorkflowReport
	HookPermissionRequest     = api.HookPermissionRequest
	HookPermissionResponse    = api.HookPermissionResponse

	WikiProposeRequest = api.WikiProposeRequest
	WikiApplyRequest   = api.WikiApplyRequest
	WikiProposal       = api.WikiProposal
	WikiRefusal        = api.WikiRefusal
	WikiPageInfo       = api.WikiPageInfo
	WikiIndex          = api.WikiIndex
	WikiPage           = api.WikiPage

	APIError             = api.APIError
	UntrustedSocketError = api.UntrustedSocketError
)

const (
	ClientAttach   = api.ClientAttach
	ClientPresence = api.ClientPresence

	PolicySleepWhenIdle = api.PolicySleepWhenIdle
	PolicyBackground    = api.PolicyBackground
	PolicyStopNow       = api.PolicyStopNow
)

// Sentinel errors: use api.ErrXxx.
var (
	ErrNotRunning      = api.ErrNotRunning
	ErrUnsupported     = api.ErrUnsupported
	ErrUntrustedSocket = api.ErrUntrustedSocket
)

// DefaultPaths: use api.DefaultPaths.
func DefaultPaths() Paths { return api.DefaultPaths() }

// GroupLockPath: use api.GroupLockPath.
func GroupLockPath(serversDir, group string) string { return api.GroupLockPath(serversDir, group) }

// ProxyURLPath: use api.ProxyURLPath.
func ProxyURLPath(serversDir, group string) string { return api.ProxyURLPath(serversDir, group) }

// Constructors: use client.NewClient, client.InProcessMode and client.Ensure.
var (
	NewClient     = dclient.NewClient
	InProcessMode = dclient.InProcessMode
	Ensure        = dclient.Ensure
)
