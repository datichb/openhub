package credproxy

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// S-21 (audit v5): the region never sends the credential elsewhere.
func TestBedrockRegion(t *testing.T) {
	for _, r := range []string{"eu-west-1", "us-gov-west-1", "ap-southeast-5", "il-central-1", "xx-newregion-12"} {
		assert.NoError(t, CheckRegion(r), r)
	}
	for _, r := range []string{"x.evil.com#", "evil.com", "eu-west-1.evil.com", "eu-west-1#", "EU-WEST-1", "eu_west_1", "eu-west", "eu-west-1/x", "", "eu-west-1 "} {
		assert.ErrorIs(t, CheckRegion(r), ErrRegion, r)
		if r != "" {
			_, err := BedrockUpstream(r, BearerAuth{Token: "k"})
			assert.Error(t, err, r)
		}
	}
	up, err := BedrockUpstream("", BearerAuth{})
	require.NoError(t, err)
	assert.Equal(t, "https://bedrock-runtime.us-east-1.amazonaws.com", up.BaseURL)
	assert.True(t, KnownBedrockRegion("eu-west-1"))
	assert.False(t, KnownBedrockRegion("xx-newregion-12"))
}

func TestCheckUpstream(t *testing.T) {
	for _, tc := range []struct {
		provider, base string
		ok             bool
	}{
		{ProviderBedrock, "https://bedrock-runtime.eu-west-1.amazonaws.com", true},
		{ProviderBedrock, "https://bedrock-runtime.x.evil.com#.amazonaws.com", false},
		{ProviderBedrock, "https://bedrock-runtime.x.evil.com", false},
		{ProviderBedrock, "http://bedrock-runtime.eu-west-1.amazonaws.com", false},
		{ProviderBedrock, "https://u:p@bedrock-runtime.eu-west-1.amazonaws.com", false},
		{ProviderAnthropic, "https://api.anthropic.com/v1", true},
		{ProviderAnthropic, "https://api.anthropic.com.evil.com/v1", false},
		{ProviderOpenRouter, "https://openrouter.ai/api/v1", true},
		{ProviderOpenAI, "https://api.openai.com/v1", true},
		{ProviderOpenAI, "http://127.0.0.1:5555", true}, // loopback relay (tests)
		{"other", "https://example.com", false},
	} {
		err := checkUpstream(tc.provider, tc.base)
		assert.Equal(t, tc.ok, err == nil, "%s %s: %v", tc.provider, tc.base, err)
	}
	p := New()
	_, err := p.Issue(Grant{SessionID: "s", Provider: ProviderBedrock, Upstream: Upstream{BaseURL: "https://bedrock-runtime.x.evil.com", Auth: BearerAuth{}}})
	assert.Error(t, err, "refused when issued")
}

// S-22 (audit v5): OpenRouter cannot route a session limited to some
// models to another one.
func TestOpenRouterRoutingRefused(t *testing.T) {
	var s seen
	up := upstream(t, &s, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{}`)) })
	p := startProxy(t)
	tok, err := p.Issue(Grant{SessionID: "s", Provider: ProviderOpenRouter, AllowedModels: []string{"anthropic/claude-haiku-*"},
		Upstream: Upstream{BaseURL: up.URL, Auth: BearerAuth{Token: "REAL"}}})
	require.NoError(t, err)
	post := func(body string) int {
		req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderOpenRouter)+"/chat/completions", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		return resp.StatusCode
	}
	assert.Equal(t, http.StatusOK, post(`{"model":"anthropic/claude-haiku-4.5","messages":[]}`))
	assert.Equal(t, http.StatusOK, post(`{"model":"anthropic/claude-haiku-4.5","models":["anthropic/claude-haiku-4"],"messages":[]}`))
	s.mu.Lock()
	s.body = ""
	s.mu.Unlock()
	for _, body := range []string{
		`{"model":"anthropic/claude-haiku-4.5","models":["openai/gpt-5"]}`,
		`{"model":"anthropic/claude-haiku-4.5","route":"fallback","models":["openai/gpt-5"]}`,
		`{"model":"anthropic/claude-haiku-4.5","preset":"expensive"}`,
		`{"model":"anthropic/claude-haiku-4.5@preset/expensive"}`,
		`{"model":"anthropic/claude-haiku-4.5","Models":["openai/gpt-5"]}`,
		`{"model":"anthropic/claude-haiku-4.5"}{"model":"openai/gpt-5"}`,
		`{"model":"anthropic/claude-haiku-4.5","models":"openai/gpt-5"}`,
	} {
		assert.Contains(t, []int{http.StatusBadRequest, http.StatusForbidden}, post(body), body)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	assert.Empty(t, s.body, "nothing reached OpenRouter")
}
