package remote

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/remote"
	"github.com/datichb/openhub/cli/internal/remote/gitlab"
)

// recordImage records the digest of the image a pipeline built (ref.ImageBuilt),
// read from the dotenv artifact of its image job once that job succeeded:
// the next sessions of the same image run it by this digest instead of
// rebuilding it. It reports whether ref changed; a failure only means the
// next session rebuilds the image.
func (s *Service) recordImage(ctx context.Context, c *gitlab.Client, ref *domain.RemoteRef, jobs []gitlab.Job) bool {
	if !ref.ImageBuilt || ref.ImageDigest != "" {
		return false
	}
	rid := fmt.Sprint(ref.RunnerID)
	if jobs == nil {
		var err error
		if jobs, err = c.PipelineJobs(ctx, rid, ref.Pipeline); err != nil {
			slog.Warn("remote: image digest not recorded", "pipeline", ref.Pipeline, "err", err)
			return false
		}
	}
	var job int64
	for _, j := range jobs {
		if j.Name == "oh-image" && j.Status == "success" && j.ID > job {
			job = j.ID
		}
	}
	if job == 0 {
		return false
	}
	data, err := c.JobArtifacts(ctx, rid, job)
	if err != nil {
		slog.Warn("remote: image digest not recorded", "pipeline", ref.Pipeline, "err", err)
		return false
	}
	digest, err := imageDigestOf(data, ref.Image)
	if err != nil {
		slog.Warn("remote: image digest not recorded", "pipeline", ref.Pipeline, "err", err)
		return false
	}
	ref.ImageDigest = digest
	return true
}

// imageDigestOf reads the digest of image in the artifacts (zip) of an image
// job: the VarRunImage line of ImageEnvFile, which must name the repository
// of image.
func imageDigestOf(artifacts []byte, image string) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(artifacts), int64(len(artifacts)))
	if err != nil {
		return "", err
	}
	for _, f := range zr.File {
		if f.Name != remote.ImageEnvFile {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		content, err := io.ReadAll(io.LimitReader(rc, 64<<10))
		_ = rc.Close()
		if err != nil {
			return "", err
		}
		prefix := pinnedImage(image, "")
		sc := bufio.NewScanner(bytes.NewReader(content))
		for sc.Scan() {
			k, v, ok := strings.Cut(strings.TrimSpace(sc.Text()), "=")
			if !ok || k != remote.VarRunImage {
				continue
			}
			d, found := strings.CutPrefix(v, prefix)
			if !found || !digestRe.MatchString(d) {
				return "", fmt.Errorf("%s: %s is not %s pinned by digest", remote.ImageEnvFile, remote.VarRunImage, image)
			}
			return d, nil
		}
		return "", fmt.Errorf("%s: no %s", remote.ImageEnvFile, remote.VarRunImage)
	}
	return "", fmt.Errorf("no %s in the image job artifacts", remote.ImageEnvFile)
}
