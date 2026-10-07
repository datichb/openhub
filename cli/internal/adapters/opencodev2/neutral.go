package opencodev2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/adapters/opencodev2/deploycleanup"
)

// Neutral capabilities of the adapter (D19): what oh asks the tool through
// the adapters interfaces, without naming it.

var (
	_ adapters.Pairer         = (*Adapter)(nil)
	_ adapters.OutcomeReader  = (*Adapter)(nil)
	_ adapters.ProviderMapper = (*Adapter)(nil)
	_ adapters.LinuxInstaller = (*Adapter)(nil)
	_ adapters.DataLocator    = (*Adapter)(nil)
)

// PairURL implements adapters.Pairer (opencode pairing code).
func (a *Adapter) PairURL(ctx context.Context, h adapters.ServerHandle) (string, error) {
	c := NewClient(h.URL, h.Password)
	code, err := c.Pair(ctx)
	if err != nil {
		return "", err
	}
	return c.PairURL(code.Code), nil
}

// Outcome implements adapters.OutcomeReader.
func (a *Adapter) Outcome(ctx context.Context, h adapters.ServerHandle, sessionID string) (string, error) {
	s, err := NewClient(h.URL, h.Password).GetSession(ctx, sessionID)
	if err != nil {
		return "", err
	}
	return s.Outcome, nil
}

// ProviderID implements adapters.ProviderMapper: the opencode provider of a
// hub provider.
func (a *Adapter) ProviderID(hubProvider string) string { return ProviderID(hubProvider) }

// ProviderID maps a hub provider name to the opencode provider id.
func ProviderID(hubProvider string) string {
	if hubProvider == "bedrock" {
		return BedrockProvider
	}
	return hubProvider
}

// BedrockProvider is the opencode provider id of Amazon Bedrock.
const BedrockProvider = "amazon-bedrock"

// legacyVersionEnv is the variable that gave the opencode version of remote
// job images before v5 finalisation (read during v5.0).
const legacyVersionEnv = "OH_OPENCODE_VERSION"

// InstallLinux implements adapters.LinuxInstaller: the opencode Linux binary
// (npm release, glibc or musl) into o.BinDir.
func (a *Adapter) InstallLinux(ctx context.Context, o adapters.LinuxInstall) (adapters.LinuxInstalled, error) {
	ver := o.Version
	if ver == "" {
		ver = os.Getenv(legacyVersionEnv)
	}
	ver = strings.TrimPrefix(ver, "v")
	if ver == "" {
		return adapters.LinuxInstalled{}, fmt.Errorf("%s version unknown (build argument of the oh layer)", DisplayName)
	}
	tool := &LinuxTool{Ver: ver, CacheDir: o.CacheDir}
	src, err := tool.LinuxBinary(ctx, o.Arch, o.Libc)
	if err != nil {
		return adapters.LinuxInstalled{}, fmt.Errorf("%s %s for linux/%s (%s): %w", DisplayName, ver, o.Arch, o.Libc, err)
	}
	dst := filepath.Join(o.BinDir, Command)
	data, err := os.ReadFile(src)
	if err != nil {
		return adapters.LinuxInstalled{}, err
	}
	if err := os.WriteFile(dst, data, 0o755); err != nil {
		return adapters.LinuxInstalled{}, err
	}
	return adapters.LinuxInstalled{Version: ver, Binary: dst}, nil
}

// UserDataDirs implements adapters.DataLocator (XDG data and config).
func (a *Adapter) UserDataDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{filepath.Join(home, ".local", "share", "opencode"), filepath.Join(home, ".config", "opencode")}
}

// ProjectFiles implements adapters.DataLocator: what `oh deploy` (oh v4)
// wrote in a project.
func (a *Adapter) ProjectFiles() []string { return []string{".opencode", "opencode.json"} }

var _ adapters.LegacyCleaner = (*Adapter)(nil)

// ScanLegacy implements adapters.LegacyCleaner (files of `oh deploy`:
// .opencode/, opencode.json).
func (a *Adapter) ScanLegacy(dir string, o adapters.LegacyOptions) (*adapters.LegacyPlan, error) {
	p, err := deploycleanup.Scan(dir, deploycleanup.Options{AgentIDs: o.AgentIDs, InstructionFiles: o.InstructionFiles})
	if err != nil {
		return nil, err
	}
	out := &adapters.LegacyPlan{Dir: p.Dir, ConfigFile: deploycleanup.ConfigFile, Removed: p.Removed, Kept: p.Kept,
		ConfigUntouched: p.ConfigUntouched, DeleteConfig: p.DeleteConfig, DiffFunc: p.Diff, ApplyFunc: p.Apply}
	for _, it := range p.Items {
		out.Items = append(out.Items, adapters.LegacyItem{Rel: it.Rel, Dir: it.Dir, Count: it.Count, Link: it.Link})
	}
	return out, nil
}

var _ adapters.SessionContextSetter = (*Adapter)(nil)

// maxContextValue is the largest instruction entry opencode accepts (2.0.20).
const maxContextValue = 256 << 10

// SetSessionContext implements adapters.SessionContextSetter (instruction
// entries, S8). A route missing on this server (404/405) is ErrUnsupported.
func (a *Adapter) SetSessionContext(ctx context.Context, h adapters.ServerHandle, sessionID, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(data) > maxContextValue {
		return fmt.Errorf("session context %s: %d bytes, more than %d", key, len(data), maxContextValue)
	}
	return contextError(NewClient(h.URL, h.Password).PutInstructionEntry(ctx, sessionID, key, json.RawMessage(data)))
}

// ClearSessionContext implements adapters.SessionContextSetter.
func (a *Adapter) ClearSessionContext(ctx context.Context, h adapters.ServerHandle, sessionID, key string) error {
	err := NewClient(h.URL, h.Password).DeleteInstructionEntry(ctx, sessionID, key)
	var ae *APIError
	if errors.As(err, &ae) && ae.Status == http.StatusNotFound && ae.Tag != "" {
		return nil // already absent
	}
	return contextError(err)
}

func contextError(err error) error {
	var ae *APIError
	if errors.As(err, &ae) && (ae.Status == http.StatusMethodNotAllowed || (ae.Status == http.StatusNotFound && ae.Tag == "")) {
		return fmt.Errorf("%w: %v", adapters.ErrUnsupported, err)
	}
	return err
}
