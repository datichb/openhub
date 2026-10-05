package credproxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixedNow() time.Time { return time.Date(2015, 8, 30, 12, 36, 0, 0, time.UTC) }

// AWS SigV4 test suite: "get-vanilla".
func TestSigV4AWSTestVector(t *testing.T) {
	auth := &SigV4Auth{
		Credentials: credentials.NewStaticCredentialsProvider("AKIDEXAMPLE", "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY", ""),
		Region:      "us-east-1",
		Service:     "service",
		Now:         fixedNow,
	}
	req, _ := http.NewRequest(http.MethodGet, "https://example.amazonaws.com/", http.NoBody)
	require.NoError(t, auth.Apply(req, nil))
	assert.Equal(t, "20150830T123600Z", req.Header.Get("X-Amz-Date"))
	assert.Equal(t,
		"AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=5fa00fa31553b73ebf1942676e86291e8372ff2a2260956d9b8aae1d763fbf31",
		req.Header.Get("Authorization"))
}

func TestSigV4ThroughProxy(t *testing.T) {
	var s seen
	up := upstream(t, &s, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "{}") })
	p := startProxy(t)
	auth := &SigV4Auth{
		Credentials: credentials.NewStaticCredentialsProvider("AKIDHOST", "secret", "session-token"),
		Region:      "eu-west-1",
	}
	tok, err := p.Issue(Grant{Provider: ProviderBedrock, Upstream: Upstream{BaseURL: up.URL, Auth: auth}})
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderBedrock)+"/model/eu.anthropic.claude-haiku-4-5-20251001-v1%3A0/converse-stream", strings.NewReader(`{"messages":[]}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("X-Amz-Security-Token", "tool-forged")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	s.mu.Lock()
	defer s.mu.Unlock()
	authz := s.headers.Get("Authorization")
	assert.True(t, strings.HasPrefix(authz, "AWS4-HMAC-SHA256 Credential=AKIDHOST/"), authz)
	assert.Contains(t, authz, "/eu-west-1/bedrock/aws4_request")
	assert.NotEmpty(t, s.headers.Get("X-Amz-Date"))
	assert.Equal(t, "session-token", s.headers.Get("X-Amz-Security-Token"), "host session token, not the tool's")
	assert.NotContains(t, authz, tok)
}

func TestSigV4MissingCredentials(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://x/", http.NoBody)
	assert.Error(t, (&SigV4Auth{}).Apply(req, nil))

	failing := aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{}, fmt.Errorf("expired SSO session")
	})
	err := (&SigV4Auth{Credentials: failing, Region: "eu-west-1"}).Apply(req, nil)
	assert.ErrorContains(t, err, "expired SSO session")
}

func TestNewSigV4FromProfileUnknownProfile(t *testing.T) {
	t.Setenv("AWS_CONFIG_FILE", t.TempDir()+"/none")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", t.TempDir()+"/none")
	_, err := NewSigV4FromProfile(context.Background(), "does-not-exist", "eu-west-1")
	assert.Error(t, err)
}
