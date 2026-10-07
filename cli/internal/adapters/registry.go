package adapters

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// The registry of tool adapters (D19): the composition root
// (cmd/v5_adapters.go) registers the implementations, everything else asks
// the registry for a neutral ToolAdapter.

// ErrToolNotInstalled is returned by Detect when the tool binary is missing.
var ErrToolNotInstalled = errors.New("tool binary not found")

// UnsupportedVersionError is returned by Detect for a tool release outside
// the range this oh version supports.
type UnsupportedVersionError struct {
	Tool            string // displayed tool name
	Found, Min, Max string
}

func (e *UnsupportedVersionError) Error() string {
	return fmt.Sprintf("%s %s is not supported (requires %s to %s)", e.Tool, e.Found, e.Min, e.Max)
}

// Factory builds an adapter; cacheDir is where it may keep discovery results.
type Factory func(cacheDir string) ToolAdapter

// Registry holds the adapters by name, in order of preference.
type Registry struct {
	CacheDir string

	mu        sync.Mutex
	names     []string
	factories map[string]Factory
	legacy    map[string]string // former adapter name → current name
	built     map[string]ToolAdapter
}

// Register adds an adapter (aliases: former names still recorded in the
// server table).
func (r *Registry) Register(name string, f Factory, aliases ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.factories == nil {
		r.factories, r.legacy, r.built = map[string]Factory{}, map[string]string{}, map[string]ToolAdapter{}
	}
	r.names = append(r.names, name)
	r.factories[name] = f
	for _, a := range aliases {
		r.legacy[a] = name
	}
}

// Names returns the registered adapter names, in order of preference.
func (r *Registry) Names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.names...)
}

// Get returns the adapter named name (as recorded for a server group), built
// once; false when no such adapter is registered.
func (r *Registry) Get(name string) (ToolAdapter, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n, ok := r.legacy[name]; ok {
		name = n
	}
	if a, ok := r.built[name]; ok {
		return a, true
	}
	f, ok := r.factories[name]
	if !ok {
		return nil, false
	}
	a := f(r.CacheDir)
	r.built[name] = a
	return a, true
}

// Detect returns the first registered adapter whose tool is installed and
// supported, with what Detect found. Without any, what the preferred adapter
// found (at least its names) and its error (ErrToolNotInstalled,
// *UnsupportedVersionError…).
func (r *Registry) Detect(ctx context.Context) (ToolAdapter, ToolInfo, error) {
	var (
		first     error
		firstInfo ToolInfo
	)
	for _, name := range r.Names() {
		a, _ := r.Get(name)
		info, err := a.Detect(ctx)
		if err == nil {
			return a, info, nil
		}
		if first == nil {
			first, firstInfo = err, info
		}
	}
	if first == nil {
		first = errors.New("no tool adapter registered")
	}
	return nil, firstInfo, first
}

// ── Optional capabilities (neutral; implemented by the adapters) ──────────

// Pairer opens a session in a browser or on a phone (S11): a one-time
// pairing URL for the server.
type Pairer interface {
	PairURL(ctx context.Context, h ServerHandle) (string, error)
}

// OutcomeReader reports how the last turn of a session ended (succeeded,
// failed, interrupted…; "" when unknown).
type OutcomeReader interface {
	Outcome(ctx context.Context, h ServerHandle, sessionID string) (string, error)
}

// ProviderMapper translates a hub provider (bedrock, anthropic…) to the
// provider id of the tool.
type ProviderMapper interface {
	ProviderID(hubProvider string) string
}

// LinuxInstaller installs the tool for Linux in an image or a job (remote
// runners): the binary for arch/libc into binDir.
type LinuxInstaller interface {
	InstallLinux(ctx context.Context, o LinuxInstall) (LinuxInstalled, error)
}

// LinuxInstall describes a Linux installation of the tool.
type LinuxInstall struct {
	Version  string // "" = from the build environment of the image
	Arch     string // amd64 | arm64
	Libc     string // glibc | musl
	BinDir   string
	CacheDir string
}

// LinuxInstalled is what InstallLinux did.
type LinuxInstalled struct {
	Version string
	Binary  string // path of the installed binary
}

// DataLocator lists where the tool keeps data outside oh (purge).
type DataLocator interface {
	// UserDataDirs are the tool directories of the user (data, config).
	UserDataDirs() []string
	// ProjectFiles are the files and folders of a former oh deployment in a
	// project, relative to the project.
	ProjectFiles() []string
}

// LegacyCleaner removes what the former deployment of oh (v4: tool files
// written into each project) left (`oh migrate deploy-cleanup`).
type LegacyCleaner interface {
	ScanLegacy(dir string, o LegacyOptions) (*LegacyPlan, error)
}

// LegacyOptions are what oh knows about the former deployments.
type LegacyOptions struct {
	AgentIDs         []string // the hub agents (their blocks in the tool config)
	InstructionFiles []string // the extra instruction files of hub.toml
}

// LegacyItem is a file or folder to remove.
type LegacyItem struct {
	Rel   string `json:"path"`
	Dir   bool   `json:"dir,omitempty"`
	Count int    `json:"count,omitempty"` // files in a folder
	Link  bool   `json:"link,omitempty"`  // symbolic link (worktree)
}

// LegacyPlan is the cleanup of one project.
type LegacyPlan struct {
	Dir   string       `json:"dir"`
	Items []LegacyItem `json:"items,omitempty"`
	// ConfigFile is the tool configuration file of the project (relative).
	ConfigFile string `json:"config_file,omitempty"`
	// Removed lists the configuration keys removed (paths); Kept the keys
	// oh wrote but the user changed since.
	Removed []string `json:"removed,omitempty"`
	Kept    []string `json:"kept,omitempty"`
	// ConfigUntouched explains why the configuration is not cleaned:
	// "no_deploy_state", "unreadable" ("" = it is, or nothing to do).
	ConfigUntouched string `json:"config_untouched,omitempty"`
	// DeleteConfig: nothing of the user remains in the configuration file.
	DeleteConfig bool `json:"delete_config,omitempty"`

	DiffFunc  func() string `json:"-"`
	ApplyFunc func() error  `json:"-"`
}

// Empty reports whether nothing is left to remove.
func (p *LegacyPlan) Empty() bool {
	return len(p.Items) == 0 && len(p.Removed) == 0 && !p.DeleteConfig
}

// Leftovers reports whether former deployment files remain (Doctor).
func (p *LegacyPlan) Leftovers() bool { return !p.Empty() }

// Diff is the change of the configuration file ("" = none).
func (p *LegacyPlan) Diff() string {
	if p.DiffFunc == nil {
		return ""
	}
	return p.DiffFunc()
}

// Apply removes the leftovers.
func (p *LegacyPlan) Apply() error {
	if p.ApplyFunc == nil {
		return nil
	}
	return p.ApplyFunc()
}
