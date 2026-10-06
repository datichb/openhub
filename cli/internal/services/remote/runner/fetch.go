package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/remote"
)

// Fetched is the session sent by the machine.
type Fetched struct {
	Manifest remote.Manifest
	Snapshot remote.Snapshot
	Bundle   *bundle.Bundle
}

// download reads a generic package file of oh-runner with the job token.
func download(ctx context.Context, hc *http.Client, rawURL, jobToken string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("JOB-TOKEN", jobToken)
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", redactURL(rawURL), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading %s: HTTP %d", redactURL(rawURL), resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, remote.MaxUnpack))
}

func redactURL(raw string) string {
	if u, err := url.Parse(raw); err == nil {
		u.RawQuery, u.User = "", nil
		return u.String()
	}
	return "package"
}

// packageVersion is the version segment of a generic package URL
// (…/packages/generic/<name>/<version>/<file>).
func packageVersion(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	dir, _ := path.Split(u.Path)
	return path.Base(strings.TrimSuffix(dir, "/"))
}

// Fetch downloads and checks the envelope and the bundle: the envelope must
// hash to its package version, the bundle version must be the manifest's
// bundle hash. The bundle is unpacked under bundlesDir/<hash>.
func Fetch(ctx context.Context, j *Job, bundlesDir string) (*Fetched, error) {
	hc := &http.Client{Timeout: 10 * time.Minute}
	env, err := download(ctx, hc, j.SessionURL, j.Secrets.JobToken)
	if err != nil {
		return nil, err
	}
	if sum := remote.SHA256(env); sum != packageVersion(j.SessionURL) {
		return nil, fmt.Errorf("session envelope: hash %s does not match its package version", sum[:12])
	}
	files, err := remote.UnpackFiles(bytes.NewReader(env))
	if err != nil {
		return nil, err
	}
	f := &Fetched{}
	if err := json.Unmarshal(files[remote.ManifestFile], &f.Manifest); err != nil {
		return nil, fmt.Errorf("session manifest: %w", err)
	}
	if f.Manifest.Schema != remote.ManifestSchema {
		return nil, fmt.Errorf("session manifest schema %d, this oh reads %d", f.Manifest.Schema, remote.ManifestSchema)
	}
	if f.Manifest.SessionID != j.SessionID {
		return nil, fmt.Errorf("session manifest of %s, pipeline of %s", f.Manifest.SessionID, j.SessionID)
	}
	if data, ok := files[remote.SnapshotFile]; ok {
		if err := json.Unmarshal(data, &f.Snapshot); err != nil {
			return nil, fmt.Errorf("beads snapshot: %w", err)
		}
	}
	if packageVersion(j.BundleURL) != f.Manifest.BundleHash {
		return nil, fmt.Errorf("bundle package %s is not the bundle of the session", packageVersion(j.BundleURL))
	}
	data, err := download(ctx, hc, j.BundleURL, j.Secrets.JobToken)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(bundlesDir, f.Manifest.BundleHash)
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	if err := remote.UnpackDir(bytes.NewReader(data), dir); err != nil {
		return nil, fmt.Errorf("unpacking the bundle: %w", err)
	}
	b, err := bundle.Load(bundlesDir, f.Manifest.BundleHash)
	if err != nil {
		return nil, fmt.Errorf("loading the bundle: %w", err)
	}
	Relocate(b)
	f.Bundle = b
	return f, nil
}

// Relocate points the paths recorded in a bundle built on the machine to
// its directory in the job.
func Relocate(b *bundle.Bundle) {
	b.Spec.Root = b.Dir
	b.Spec.SkillsDir = filepath.Join(b.Dir, "skills")
	for i := range b.Spec.Skills {
		b.Spec.Skills[i].Dir = filepath.Join(b.Spec.SkillsDir, b.Spec.Skills[i].ID)
	}
}
