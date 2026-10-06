// Package platform holds the types shared with the former session platform
// abstraction (ADR-036): provider credentials and the statistics read by
// metrics and dashboards (StatsProvider). Sessions themselves run through
// the tool adapters (internal/adapters) and the RunService (internal/runsvc):
// opencode V1 and its launch path were removed in oh v5 (D3, P3-T30).
package platform

// Credentials holds resolved provider credentials.
type Credentials struct {
	BearerToken string // Bedrock bearer token
	APIKey      string // Provider API key (Anthropic, OpenRouter)
	AWSProfile  string // AWS profile override
	AWSRegion   string // AWS region override
}
