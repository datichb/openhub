package opencodev2

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
)

// DefaultNPMRegistry publishes the opencode V2 native binaries
// (@opencode/cli-<os>-<arch>[-musl]); V2 has no GitHub release assets.
const DefaultNPMRegistry = "https://registry.npmjs.org"

// LinuxTool provides the opencode Linux binary installed in container images,
// pinned to the version of the adapter.
type LinuxTool struct {
	Ver      string
	CacheDir string       // binaries are cached under <CacheDir>/tools/opencode/<ver>/<target>/
	Registry string       // default DefaultNPMRegistry
	HTTP     *http.Client // default: 10 min timeout
	// Pins is the table of pinned integrities (nil = the one embedded in oh,
	// an empty table pins nothing: tests).
	Pins IntegrityPins
	// Strict, when it returns true (execution.strict_isolation), refuses a
	// version that is not pinned instead of warning about it.
	Strict func() bool
}

// ContainerTool returns the container tool matching the adapter version.
func (a *Adapter) ContainerTool() ohruntime.Tool {
	return &LinuxTool{Ver: a.Ver, CacheDir: a.CacheDir, Strict: a.StrictPins}
}

var _ ohruntime.Tool = (*LinuxTool)(nil)

// Name is the command name of the tool inside the image.
func (t *LinuxTool) Name() string { return "opencode" }

// Version is the pinned tool version.
func (t *LinuxTool) Version() string { return t.Ver }

// npmTarget returns the npm package suffix for an architecture and libc.
func npmTarget(arch, libc string) (string, error) {
	var a string
	switch arch {
	case "arm64", "aarch64":
		a = "arm64"
	case "amd64", "x86_64", "x64":
		a = "x64"
	default:
		return "", fmt.Errorf("unsupported architecture %q", arch)
	}
	target := "linux-" + a
	switch libc {
	case "", "glibc", "gnu":
	case "musl":
		target += "-musl"
	default:
		return "", fmt.Errorf("unsupported libc %q", libc)
	}
	return target, nil
}

type npmVersion struct {
	Dist struct {
		Tarball   string `json:"tarball"`
		Integrity string `json:"integrity"`
	} `json:"dist"`
}

// binarySumSuffix names the file kept next to the cached binary, with the
// SHA-256 of the binary computed when it was downloaded (S-38).
const binarySumSuffix = ".sha256"

// binaryPinSuffix names the file kept next to the cached binary of a pinned
// version, with the pinned integrity its archive was checked against: a
// binary cached before the pin, or under another value, is downloaded again.
const binaryPinSuffix = ".integrity"

// LinuxBinary returns the path of the cached Linux binary, downloading it
// from the npm registry when missing. The archive is checked against the
// registry integrity (sha512) before extraction; the SHA-256 of the extracted
// binary is kept next to it and checked again before each reuse: a binary that
// no longer matches (modified, truncated, replaced) is downloaded again.
//
// For a version listed in pinned_integrity.json the integrity announced by npm
// must be the pinned one: the registry response alone is not trusted (S-38).
// For another version the integrity comes from the same npm response as the
// tarball, which protects against a corrupted or swapped archive, not against
// a compromised registry entry: oh warns about it, and refuses the version
// when execution.strict_isolation is set (see pins.go to pin a version).
func (t *LinuxTool) LinuxBinary(ctx context.Context, arch, libc string) (string, error) {
	if t.Ver == "" {
		return "", errors.New("opencode version unknown (adapter not detected)")
	}
	target, err := npmTarget(arch, libc)
	if err != nil {
		return "", err
	}
	pkg := "@opencode/cli-" + target
	pin, err := t.pin(target)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(t.CacheDir, "tools", "opencode", t.Ver, target)
	bin := filepath.Join(dir, "opencode")
	if cachedBinaryIntact(bin, pin) {
		return bin, nil
	}
	// A cached binary that fails the check is never used: not even when the
	// download below fails.
	_ = os.Remove(bin)
	_ = os.Remove(bin + binarySumSuffix)
	_ = os.Remove(bin + binaryPinSuffix)
	reg := t.Registry
	if reg == "" {
		reg = DefaultNPMRegistry
	}
	hc := t.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Minute}
	}
	var meta npmVersion
	if err := getJSON(ctx, hc, strings.TrimRight(reg, "/")+"/"+url.PathEscape(pkg)+"/"+url.PathEscape(t.Ver), &meta); err != nil {
		return "", fmt.Errorf("npm metadata of %s@%s: %w", pkg, t.Ver, err)
	}
	want, ok := strings.CutPrefix(meta.Dist.Integrity, "sha512-")
	if !ok || meta.Dist.Tarball == "" {
		return "", fmt.Errorf("npm metadata of %s@%s: no sha512 integrity", pkg, t.Ver)
	}
	if pin != "" && meta.Dist.Integrity != pin {
		return "", fmt.Errorf("%s@%s: the integrity announced by the npm registry is not the one pinned in oh (a compromised or mirrored registry?): refusing to download", pkg, t.Ver)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, "download-*.tgz")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	h := sha512.New()
	if err := download(ctx, hc, meta.Dist.Tarball, io.MultiWriter(tmp, h)); err != nil {
		return "", fmt.Errorf("downloading %s@%s: %w", pkg, t.Ver, err)
	}
	if got := base64.StdEncoding.EncodeToString(h.Sum(nil)); got != want {
		return "", fmt.Errorf("%s@%s: integrity mismatch", pkg, t.Ver)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	if err := extractFile(tmp, "package/bin/opencode", bin); err != nil {
		return "", fmt.Errorf("extracting %s@%s: %w", pkg, t.Ver, err)
	}
	sum, err := fileSHA256(bin)
	if err != nil {
		return "", fmt.Errorf("fingerprint of %s@%s: %w", pkg, t.Ver, err)
	}
	if pin != "" {
		if err := writeFileAtomic(bin+binaryPinSuffix, []byte(pin+"\n")); err != nil {
			return "", fmt.Errorf("pinned integrity of %s@%s: %w", pkg, t.Ver, err)
		}
	}
	if err := writeFileAtomic(bin+binarySumSuffix, []byte(sum+"\n")); err != nil {
		return "", fmt.Errorf("fingerprint of %s@%s: %w", pkg, t.Ver, err)
	}
	return bin, nil
}

// pin returns the pinned integrity of the version for a target. A version
// that is not pinned has none ("" and no error): it is accepted with a
// warning, or refused under strict isolation.
func (t *LinuxTool) pin(target string) (string, error) {
	pins := t.Pins
	if pins == nil {
		var err error
		if pins, err = PinnedIntegrity(); err != nil {
			return "", err
		}
	}
	if integrity, ok := pins.lookup(t.Ver, target); ok {
		return integrity, nil
	}
	if t.Strict != nil && t.Strict() {
		return "", fmt.Errorf("opencode %s for %s is not pinned in oh (no integrity in pinned_integrity.json) and execution.strict_isolation is set: refusing it. Use a pinned version, or turn strict isolation off", t.Ver, target)
	}
	slog.Warn("opencode Linux binary not pinned: its integrity comes from the same npm response as the package, so a compromised registry could supply both",
		"package", "@opencode/cli-"+target, "version", t.Ver, "hint", "set execution.strict_isolation to refuse unpinned versions")
	return "", nil
}

// cachedBinaryIntact reports whether bin is a regular, non-empty file whose
// SHA-256 is the one recorded when it was downloaded. Without that record
// (binary cached by an earlier version of oh, or interrupted download) the
// binary is not trusted. For a pinned version it must also have been
// downloaded under the pinned integrity.
func cachedBinaryIntact(bin, pin string) bool {
	st, err := os.Lstat(bin)
	if err != nil || !st.Mode().IsRegular() || st.Size() == 0 {
		return false
	}
	if pin != "" {
		if got, err := os.ReadFile(bin + binaryPinSuffix); err != nil || strings.TrimSpace(string(got)) != pin {
			return false
		}
	}
	want, err := os.ReadFile(bin + binarySumSuffix)
	if err != nil {
		return false
	}
	got, err := fileSHA256(bin)
	return err == nil && strings.TrimSpace(string(want)) == got
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeFileAtomic(path string, data []byte) error {
	part := path + ".part"
	if err := os.WriteFile(part, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(part, path); err != nil {
		os.Remove(part)
		return err
	}
	return nil
}

func getJSON(ctx context.Context, hc *http.Client, u string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func download(ctx context.Context, hc *http.Client, u string, w io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	_, err = io.Copy(w, resp.Body)
	return err
}

// extractFile writes one regular file of a .tgz to dest (atomically, 0755).
func extractFile(r io.Reader, name, dest string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("%s not found in archive", name)
		}
		if err != nil {
			return err
		}
		if hdr.Name != name || hdr.Typeflag != tar.TypeReg {
			continue
		}
		part := dest + ".part"
		f, err := os.OpenFile(part, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			os.Remove(part)
			return err
		}
		if err := f.Close(); err != nil {
			os.Remove(part)
			return err
		}
		return os.Rename(part, dest)
	}
}
