package selfupdate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Release signatures (Cosign keyless, Sigstore). The checksums file of a
// release is signed by the release workflow of the repository; the archive is
// verified against the checksums, the checksums against the signature.
//
// oh carries no Sigstore client: the signature is verified with the cosign
// CLI when it is installed. Without cosign the update goes on with the
// checksum alone and a warning, unless OH_REQUIRE_SIGNATURE=1 (refused).
// A signature that cosign rejects always stops the update.

const (
	cosignOIDCIssuer       = "https://token.actions.githubusercontent.com"
	cosignIdentityRegexp   = `^https://github\.com/datichb/openhub/\.github/workflows/release\.yml@refs/tags/v.*$`
	requireSignatureEnvVar = "OH_REQUIRE_SIGNATURE"
	cosignTimeout          = 2 * time.Minute
)

// ErrSignature is returned when the signature of a release is invalid, or
// required and impossible to verify.
var ErrSignature = errors.New("release signature")

// lookCosign and runCosign are variables for the tests.
var (
	lookCosign = func() (string, error) { return exec.LookPath("cosign") }
	runCosign  = func(ctx context.Context, bin string, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, bin, args...).CombinedOutput()
	}
)

func signatureRequired() bool { return os.Getenv(requireSignatureEnvVar) == "1" }

// verifySignature checks the checksums file of a release (sums, as
// downloaded) against its signature bundle. bundle is nil when the release
// has none.
func verifySignature(release *Release, sums []byte, bundle *Asset) error {
	if bundle == nil {
		if signatureRequired() {
			return fmt.Errorf("%w: release %s has no signature bundle (%s=1)", ErrSignature, release.TagName, requireSignatureEnvVar)
		}
		slog.Warn("release has no cosign signature bundle — checksum verified, signature not verified", "release", release.TagName)
		return nil
	}
	bin, err := lookCosign()
	if err != nil {
		if signatureRequired() {
			return fmt.Errorf("%w: cosign is not installed and %s=1", ErrSignature, requireSignatureEnvVar)
		}
		slog.Warn("cosign is not installed — checksum verified, signature not verified (install cosign, or set "+requireSignatureEnvVar+"=1 to require it)", "release", release.TagName)
		return nil
	}
	dir, err := os.MkdirTemp("", "oh-signature-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	sumsPath := filepath.Join(dir, "checksums.txt")
	if err := os.WriteFile(sumsPath, sums, 0o600); err != nil {
		return err
	}
	sig, err := downloadTo(dir, bundle)
	if err != nil {
		return fmt.Errorf("downloading signature bundle: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cosignTimeout)
	defer cancel()
	out, err := runCosign(ctx, bin, "verify-blob",
		"--bundle", sig,
		"--certificate-oidc-issuer", cosignOIDCIssuer,
		"--certificate-identity-regexp", cosignIdentityRegexp,
		sumsPath)
	if err != nil {
		return fmt.Errorf("%w: cosign rejected the signature of %s: %v: %s", ErrSignature, release.TagName, err, out)
	}
	slog.Debug("release signature verified", "release", release.TagName)
	return nil
}

func downloadTo(dir string, a *Asset) (string, error) {
	if err := validateDownloadURL(a.BrowserDownloadURL); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "asset-*")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := downloadAsset(a, f, nil); err != nil {
		return "", err
	}
	return f.Name(), nil
}
