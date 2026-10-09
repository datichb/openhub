package selfupdate

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stubCosign(t *testing.T, found bool, verifyErr error) *[]string {
	t.Helper()
	var calls []string
	oldLook, oldRun := lookCosign, runCosign
	lookCosign = func() (string, error) {
		if !found {
			return "", errors.New("not found")
		}
		return "/usr/bin/cosign", nil
	}
	runCosign = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(args, " "))
		return []byte("cosign output"), verifyErr
	}
	t.Cleanup(func() { lookCosign, runCosign = oldLook, oldRun })
	return &calls
}

func TestVerifySignatureWithoutBundle(t *testing.T) {
	calls := stubCosign(t, true, nil)
	rel := &Release{TagName: "v5.0.0"}
	assert.NoError(t, verifySignature(rel, []byte("sums"), nil), "no bundle: warning only")
	assert.Empty(t, *calls)

	t.Setenv(requireSignatureEnvVar, "1")
	assert.ErrorIs(t, verifySignature(rel, []byte("sums"), nil), ErrSignature)
}

func TestVerifySignatureWithoutCosign(t *testing.T) {
	stubCosign(t, false, nil)
	rel := &Release{TagName: "v5.0.0"}
	bundle := &Asset{Name: "checksums.txt.sigstore.json", BrowserDownloadURL: "https://github.com/x"}
	assert.NoError(t, verifySignature(rel, []byte("sums"), bundle), "no cosign: warning only")

	t.Setenv(requireSignatureEnvVar, "1")
	assert.ErrorIs(t, verifySignature(rel, []byte("sums"), bundle), ErrSignature)
}

func TestVerifySignatureRejectsUntrustedBundleURL(t *testing.T) {
	stubCosign(t, true, nil)
	rel := &Release{TagName: "v5.0.0"}
	bundle := &Asset{Name: "checksums.txt.sigstore.json", BrowserDownloadURL: "http://evil.example/x"}
	assert.Error(t, verifySignature(rel, []byte("sums"), bundle))
}

// The bundle download goes through the trusted-host check: the tests swap the
// asset URL for a local server by allowing its host for the test.
func withTrustedServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"bundle":true}`))
	}))
	t.Cleanup(srv.Close)
	old := allowedDownloadHosts
	allowedDownloadHosts = append(append([]string{}, old...), strings.TrimPrefix(srv.URL, "https://"))
	oldTransport := http.DefaultTransport
	http.DefaultTransport = srv.Client().Transport
	t.Cleanup(func() { allowedDownloadHosts, http.DefaultTransport = old, oldTransport })
	return srv
}

func TestVerifySignatureRunsCosign(t *testing.T) {
	srv := withTrustedServer(t)
	rel := &Release{TagName: "v5.0.0"}
	bundle := &Asset{Name: "checksums.txt.sigstore.json", BrowserDownloadURL: srv.URL + "/sig"}

	calls := stubCosign(t, true, nil)
	require.NoError(t, verifySignature(rel, []byte("sums"), bundle))
	require.Len(t, *calls, 1)
	assert.Contains(t, (*calls)[0], "verify-blob --bundle ")
	assert.Contains(t, (*calls)[0], "--certificate-oidc-issuer "+cosignOIDCIssuer)
	assert.Contains(t, (*calls)[0], "--certificate-identity-regexp "+cosignIdentityRegexp)

	stubCosign(t, true, errors.New("exit status 1"))
	assert.ErrorIs(t, verifySignature(rel, []byte("sums"), bundle), ErrSignature, "a rejected signature always stops the update")
}

func TestParseChecksums(t *testing.T) {
	body := []byte("abc123  oh_darwin_arm64.tar.gz\ndef456 oh_linux_amd64.tar.gz\n")
	h, err := parseChecksums(body, "oh_linux_amd64.tar.gz")
	require.NoError(t, err)
	assert.Equal(t, "def456", h)
	_, err = parseChecksums(body, "missing")
	assert.Error(t, err)
}
