package credproxy

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

// Bedrock regions (audit v5, S-21): the region names the host the real
// credential is sent to (`bedrock-runtime.<region>.amazonaws.com`); it is
// checked when it is configured, when a grant is issued and on every
// request, so that a value such as `x.evil.com#` never reaches the network.

var regionPattern = regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-\d{1,2}$`)

// ErrRegion is an AWS region name that is not one.
var ErrRegion = errors.New("credproxy: invalid AWS region")

// knownBedrockRegions are the regions of Bedrock known to this oh; another
// well-formed region is accepted with a warning (new regions).
var knownBedrockRegions = map[string]bool{
	"us-east-1": true, "us-east-2": true, "us-west-1": true, "us-west-2": true, "us-gov-east-1": true, "us-gov-west-1": true,
	"ca-central-1": true, "ca-west-1": true, "mx-central-1": true, "sa-east-1": true,
	"eu-west-1": true, "eu-west-2": true, "eu-west-3": true, "eu-central-1": true, "eu-central-2": true,
	"eu-north-1": true, "eu-south-1": true, "eu-south-2": true,
	"ap-northeast-1": true, "ap-northeast-2": true, "ap-northeast-3": true, "ap-south-1": true, "ap-south-2": true,
	"ap-southeast-1": true, "ap-southeast-2": true, "ap-southeast-3": true, "ap-southeast-4": true,
	"ap-southeast-5": true, "ap-southeast-7": true, "ap-east-2": true,
	"me-central-1": true, "me-south-1": true, "il-central-1": true, "af-south-1": true,
}

// CheckRegion refuses a value that is not an AWS region name.
func CheckRegion(region string) error {
	if !regionPattern.MatchString(region) {
		return fmt.Errorf("%w: %q", ErrRegion, region)
	}
	return nil
}

// KnownBedrockRegion reports whether Bedrock is known to run in region.
func KnownBedrockRegion(region string) bool { return knownBedrockRegions[region] }

// BedrockUpstream returns the Bedrock runtime endpoint for a region ("" =
// us-east-1); an invalid region is refused.
func BedrockUpstream(region string, auth Auth) (Upstream, error) {
	if region == "" {
		region = "us-east-1"
	}
	if err := CheckRegion(region); err != nil {
		return Upstream{}, err
	}
	up := Upstream{BaseURL: "https://bedrock-runtime." + region + ".amazonaws.com", Auth: auth}
	if err := checkUpstream(ProviderBedrock, up.BaseURL); err != nil {
		return Upstream{}, err
	}
	return up, nil
}

// providerHosts are the hosts of the providers with a fixed endpoint.
var providerHosts = map[string]string{
	ProviderAnthropic:  "api.anthropic.com",
	ProviderOpenRouter: "openrouter.ai",
	ProviderOpenAI:     "api.openai.com",
}

// checkUpstream checks the endpoint of a grant: HTTPS to the provider's
// host (Bedrock: bedrock-runtime.<valid region>.amazonaws.com). A loopback
// host is accepted (tests and local relays): it never leaves the machine.
func checkUpstream(provider, base string) error {
	u, err := url.Parse(base)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("credproxy: invalid upstream %q", base)
	}
	host := u.Hostname()
	if ip := net.ParseIP(host); (ip != nil && ip.IsLoopback()) || host == "localhost" {
		return nil
	}
	if u.Scheme != "https" {
		return fmt.Errorf("credproxy: upstream %q is not HTTPS", base)
	}
	switch want, fixed := providerHosts[provider]; {
	case fixed:
		if host != want {
			return fmt.Errorf("credproxy: upstream host %q is not the one of %s", host, provider)
		}
	case provider == ProviderBedrock:
		region, ok := strings.CutPrefix(host, "bedrock-runtime.")
		if region, ok2 := strings.CutSuffix(region, ".amazonaws.com"); !ok || !ok2 || CheckRegion(region) != nil {
			return fmt.Errorf("credproxy: upstream host %q is not a Bedrock endpoint", host)
		}
	default:
		return fmt.Errorf("credproxy: unknown provider %q", provider)
	}
	return nil
}
