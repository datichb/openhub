package opencodev2

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
}

// ContainerTool returns the container tool matching the adapter version.
func (a *Adapter) ContainerTool() ohruntime.Tool {
	return &LinuxTool{Ver: a.Ver, CacheDir: a.CacheDir}
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

// LinuxBinary returns the path of the cached Linux binary, downloading it
// from the npm registry when missing. The archive is checked against the
// registry integrity (sha512) before extraction.
func (t *LinuxTool) LinuxBinary(ctx context.Context, arch, libc string) (string, error) {
	if t.Ver == "" {
		return "", errors.New("opencode version unknown (adapter not detected)")
	}
	target, err := npmTarget(arch, libc)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(t.CacheDir, "tools", "opencode", t.Ver, target)
	bin := filepath.Join(dir, "opencode")
	if st, err := os.Stat(bin); err == nil && st.Mode().IsRegular() && st.Size() > 0 {
		return bin, nil
	}
	reg := t.Registry
	if reg == "" {
		reg = DefaultNPMRegistry
	}
	hc := t.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Minute}
	}
	pkg := "@opencode/cli-" + target
	var meta npmVersion
	if err := getJSON(ctx, hc, strings.TrimRight(reg, "/")+"/"+url.PathEscape(pkg)+"/"+url.PathEscape(t.Ver), &meta); err != nil {
		return "", fmt.Errorf("npm metadata of %s@%s: %w", pkg, t.Ver, err)
	}
	want, ok := strings.CutPrefix(meta.Dist.Integrity, "sha512-")
	if !ok || meta.Dist.Tarball == "" {
		return "", fmt.Errorf("npm metadata of %s@%s: no sha512 integrity", pkg, t.Ver)
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
	return bin, nil
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
