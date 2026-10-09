package credproxy

// Provider upstreams (provider IDs of the tool).
const (
	ProviderBedrock    = "amazon-bedrock"
	ProviderAnthropic  = "anthropic"
	ProviderOpenRouter = "openrouter"
	ProviderOpenAI     = "openai"
)

// AnthropicUpstream returns the Anthropic API endpoint (x-api-key auth).
func AnthropicUpstream(apiKey string) Upstream {
	return Upstream{BaseURL: "https://api.anthropic.com/v1", Auth: HeaderAuth{Name: "x-api-key", Value: apiKey}}
}

// OpenRouterUpstream returns the OpenRouter API endpoint (Bearer auth).
func OpenRouterUpstream(apiKey string) Upstream {
	return Upstream{BaseURL: "https://openrouter.ai/api/v1", Auth: BearerAuth{Token: apiKey}}
}

// OpenAIUpstream returns the OpenAI API endpoint (Bearer auth).
func OpenAIUpstream(apiKey string) Upstream {
	return Upstream{BaseURL: "https://api.openai.com/v1", Auth: BearerAuth{Token: apiKey}}
}
