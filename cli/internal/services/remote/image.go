package remote

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/datichb/openhub/cli/internal/runtime/container"
)

// imageLayerVersion changes whenever the thin oh layer of the job image
// changes (Dockerfile of ciconfig, oh runner install).
const imageLayerVersion = "1"

// tagLen is the length of the image tags (hash prefix).
const tagLen = 20

var slugCleanRe = regexp.MustCompile(`[^a-z0-9._-]+`)

// imageSlug is the container repository name of a target project.
func imageSlug(projectPath string) string {
	s := strings.ToLower(strings.Trim(projectPath, "/"))
	s = slugCleanRe.ReplaceAllString(strings.ReplaceAll(s, "/", "-"), "-")
	return strings.Trim(s, "-._")
}

func hashParts(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		fmt.Fprintf(h, "%d:%s\x00", len(p), p)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// jobImage is the image of a remote session.
type jobImage struct {
	Dockerfile string // path in the project ("" = oh default base)
	Base       string // registry reference of the base image
	Image      string // registry reference of the job image
	BaseRepo   string // container repository names (relative to oh-runner)
	ImageRepo  string
	Tag        string
}

// ErrDockerfileNotCommitted is returned when the dev Dockerfile is not in
// the commit the job starts from.
var ErrDockerfileNotCommitted = errors.New("the dev Dockerfile is not committed at the branch the job starts from")

// planImage computes the job image of a project at commit: the base hashes
// the dev Dockerfile as committed, the image adds the oh layer and the oh
// binary identity (release version or development binary hash).
func planImage(ctx context.Context, g Git, projectDir, configured, commit, registryPrefix, projectPath, ohIdentity, arch string) (*jobImage, error) {
	abs, err := container.DetectDockerfile(projectDir, configured)
	if err != nil {
		return nil, err
	}
	img := &jobImage{}
	content := []byte(container.DefaultBaseDockerfile)
	if abs != "" {
		rel, err := filepath.Rel(projectDir, abs)
		if err != nil || strings.HasPrefix(rel, "..") {
			return nil, fmt.Errorf("dev Dockerfile %s is outside the project", abs)
		}
		img.Dockerfile = filepath.ToSlash(rel)
		if content, err = g.ShowFile(ctx, projectDir, commit, img.Dockerfile); err != nil {
			return nil, fmt.Errorf("%w (%s)", ErrDockerfileNotCommitted, img.Dockerfile)
		}
	}
	base := hashParts("base", img.Dockerfile, string(content))
	full := hashParts("job", base, imageLayerVersion, ohIdentity, arch)
	slug := imageSlug(projectPath)
	prefix := strings.TrimRight(registryPrefix, "/")
	img.BaseRepo, img.ImageRepo = slug+"-base", slug
	img.Tag = full[:tagLen]
	img.Base = prefix + "/" + img.BaseRepo + ":" + base[:tagLen]
	img.Image = prefix + "/" + img.ImageRepo + ":" + img.Tag
	return img, nil
}
