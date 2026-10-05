// Package sessionspec defines the tool-agnostic model of an agentic session:
// what to run (bundle), where (location, runtime), with which provider and
// session-scoped rules. Adapters translate it to a concrete tool (opencode V1/V2…).
//
// This package has no dependency on any tool implementation.
package sessionspec

import (
	"crypto/rand"
	"fmt"
	"strings"
)

// RuntimeKind is where the tool server runs.
type RuntimeKind string

const (
	RuntimeLocal     RuntimeKind = "local"
	RuntimeContainer RuntimeKind = "container"
	RuntimeRemote    RuntimeKind = "remote"
)

// AttachPref selects how an interactive client is opened for a session.
type AttachPref string

const (
	AttachAuto     AttachPref = "auto"
	AttachITerm    AttachPref = "iterm"
	AttachTerminal AttachPref = "terminal"
	AttachTmux     AttachPref = "tmux"
	AttachBrowser  AttachPref = "browser"
	AttachSuspend  AttachPref = "suspend"
	AttachNone     AttachPref = "none"
)

// IsolationLevel is the "closed world" guarantee an adapter can provide.
type IsolationLevel string

const (
	IsolationFull    IsolationLevel = "full"
	IsolationPartial IsolationLevel = "partial"
	IsolationNone    IsolationLevel = "none"
)

// Effect of a permission rule.
type Effect string

const (
	EffectAllow Effect = "allow"
	EffectAsk   Effect = "ask"
	EffectDeny  Effect = "deny"
)

// Neutral permission actions. Adapters map them to tool-specific names.
const (
	ActionAll      = "*"
	ActionShell    = "shell"
	ActionEdit     = "edit"
	ActionRead     = "read"
	ActionSubagent = "subagent"
	ActionSkill    = "skill"
	ActionQuestion = "question"
	ActionWebFetch = "webfetch"
	ActionWebSrch  = "websearch"
)

// PermissionRule is an ordered rule; the last matching rule wins.
type PermissionRule struct {
	Action   string `json:"action"`
	Resource string `json:"resource"`
	Effect   Effect `json:"effect"`
}

// ModelRef identifies a model as "provider/model[#variant]".
type ModelRef struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Variant  string `json:"variant,omitempty"`
}

// String renders the reference in "provider/model[#variant]" form.
func (m ModelRef) String() string {
	if m.Provider == "" && m.Model == "" {
		return ""
	}
	s := m.Provider + "/" + m.Model
	if m.Variant != "" {
		s += "#" + m.Variant
	}
	return s
}

// ParseModelRef parses "provider/model[#variant]". A bare model keeps an empty provider.
func ParseModelRef(s string) ModelRef {
	var ref ModelRef
	if i := strings.LastIndex(s, "#"); i >= 0 {
		ref.Variant = s[i+1:]
		s = s[:i]
	}
	if i := strings.Index(s, "/"); i >= 0 {
		ref.Provider, ref.Model = s[:i], s[i+1:]
	} else {
		ref.Model = s
	}
	return ref
}

// ProviderSpec describes how the tool reaches the LLM provider.
// Secrets never appear here: the tool only receives a session token issued by
// the credential proxy (BaseURL points to that proxy).
type ProviderSpec struct {
	ID           string `json:"id"`                  // tool provider id, e.g. "amazon-bedrock"
	Region       string `json:"region,omitempty"`    // required for Bedrock
	BaseURL      string `json:"base_url,omitempty"`  // credential proxy endpoint
	SessionToken string `json:"-"`                   // proxy token handed to the tool (not persisted)
	TokenEnv     string `json:"token_env,omitempty"` // env var the tool reads the token from
}

// AgentDef is an assembled agent ready to be rendered.
type AgentDef struct {
	ID          string           `json:"id"`
	Description string           `json:"description"`
	Mode        string           `json:"mode"` // primary | subagent | all
	Body        string           `json:"body"` // assembled prompt (skills inlined)
	Model       *ModelRef        `json:"model,omitempty"`
	Permissions []PermissionRule `json:"permissions,omitempty"`
	Hidden      bool             `json:"hidden,omitempty"`
}

// SkillDef points to a skill directory (SKILL.md + resources) inside the bundle.
type SkillDef struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Dir         string `json:"dir"`
}

// MCPServerDef declares an MCP server available in the session.
type MCPServerDef struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"` // local | remote
	Command     []string          `json:"command,omitempty"`
	URL         string            `json:"url,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

// PluginDef is a tool plugin directory shipped in the bundle.
type PluginDef struct {
	ID      string         `json:"id"`
	Dir     string         `json:"dir"`
	Options map[string]any `json:"options,omitempty"`
}

// BundleSpec is the compiled, tool-agnostic content of a session.
type BundleSpec struct {
	Hash          string              `json:"hash"`
	Root          string              `json:"root"`
	EntryAgent    string              `json:"entry_agent"`
	Agents        []AgentDef          `json:"agents"`
	Skills        []SkillDef          `json:"skills"`
	SkillsDir     string              `json:"skills_dir,omitempty"`
	MCP           []MCPServerDef      `json:"mcp,omitempty"`
	Plugins       []PluginDef         `json:"plugins,omitempty"`
	Permissions   []PermissionRule    `json:"permissions,omitempty"` // global rules
	SubagentGraph map[string][]string `json:"subagent_graph,omitempty"`
	MaxDepth      int                 `json:"max_depth"`
	CodeMode      bool                `json:"code_mode"`
	Isolation     IsolationLevel      `json:"isolation"`
	DefaultModel  *ModelRef           `json:"default_model,omitempty"`
}

// BundleRootVar stands for the bundle root directory in agent bodies (paths
// to skill annexes). Adapters expand it with WithBundleRoot when they render
// or install the agents; the bundle hash covers the unexpanded text.
const BundleRootVar = "{{oh.bundle}}"

// WithBundleRoot returns a copy of b whose agent bodies reference root (the
// bundle directory as seen by the tool: b.Root locally, a mount point in a
// container). Idempotent.
func (b BundleSpec) WithBundleRoot(root string) BundleSpec {
	if root == "" {
		return b
	}
	out := b
	out.Agents = append([]AgentDef(nil), b.Agents...)
	for i := range out.Agents {
		out.Agents[i].Body = strings.ReplaceAll(out.Agents[i].Body, BundleRootVar, root)
	}
	return out
}

// AgentIDs returns the IDs of all agents in the bundle.
func (b BundleSpec) AgentIDs() []string {
	ids := make([]string, 0, len(b.Agents))
	for _, a := range b.Agents {
		ids = append(ids, a.ID)
	}
	return ids
}

// HasAgent reports whether the bundle contains the agent.
func (b BundleSpec) HasAgent(id string) bool {
	for _, a := range b.Agents {
		if a.ID == id {
			return true
		}
	}
	return false
}

// SkillIDs returns the IDs of all skills in the bundle.
func (b BundleSpec) SkillIDs() []string {
	ids := make([]string, 0, len(b.Skills))
	for _, s := range b.Skills {
		ids = append(ids, s.ID)
	}
	return ids
}

// BudgetLimit is an optional spending cap.
type BudgetLimit struct {
	MaxUSD float64 `json:"max_usd"`
}

// GroupKey identifies a server group: sessions sharing the same bundle,
// project and runtime share one tool server.
type GroupKey struct {
	BundleHash string      `json:"bundle_hash"`
	ProjectID  string      `json:"project_id"`
	Runtime    RuntimeKind `json:"runtime"`
	// Config fingerprints what a running server is bound to besides its
	// bundle (exact project id, provider, region, credential source), so that
	// changing any of them never reuses a server started with the old ones.
	Config string `json:"config,omitempty"`
}

// String renders a stable, filesystem-safe key.
func (g GroupKey) String() string {
	h := g.BundleHash
	if len(h) > 12 {
		h = h[:12]
	}
	p := g.ProjectID
	if p == "" {
		p = "none"
	}
	if g.Config != "" {
		c := g.Config
		if len(c) > 10 {
			c = c[:10]
		}
		return fmt.Sprintf("%s-%s-%s-%s", sanitize(p), h, c, g.Runtime)
	}
	return fmt.Sprintf("%s-%s-%s", sanitize(p), h, g.Runtime)
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// SessionSpec is everything needed to start one session.
type SessionSpec struct {
	SessionID    string            `json:"session_id"`
	Title        string            `json:"title,omitempty"`
	Group        GroupKey          `json:"group"`
	ProjectID    string            `json:"project_id,omitempty"`
	Location     string            `json:"location"`
	EntryAgent   string            `json:"entry_agent"`
	Mode         string            `json:"mode,omitempty"`
	Prompt       string            `json:"prompt,omitempty"`
	Inputs       map[string]any    `json:"inputs,omitempty"`
	Model        *ModelRef         `json:"model,omitempty"`
	Provider     ProviderSpec      `json:"provider"`
	SessionEnv   map[string]string `json:"-"`
	SessionRules []PermissionRule  `json:"session_rules,omitempty"`
	Instructions map[string]string `json:"instructions,omitempty"`
	Runtime      RuntimeKind       `json:"runtime"`
	Attach       AttachPref        `json:"attach"`
	Budget       *BudgetLimit      `json:"budget,omitempty"`
}

// sessionIDAlphabet matches the characters opencode uses in its own IDs.
const sessionIDAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZabcdefghjkmnpqrstvwxyz"

// NewSessionID returns a tool-compatible session ID ("ses_" + 26 chars).
func NewSessionID() string {
	buf := make([]byte, 26)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Sprintf("sessionspec: crypto/rand failed: %v", err))
	}
	for i, v := range buf {
		buf[i] = sessionIDAlphabet[int(v)%len(sessionIDAlphabet)]
	}
	return "ses_" + string(buf)
}
