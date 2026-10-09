package provider

import (
	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// RegionProblem checks a Bedrock region with the rule of the credential
// proxy (audit v5, S-21: the region names the host the credential is sent
// to), as it is typed or configured. msg is the translated message ("" =
// nothing to say); invalid refuses the value, a well-formed region where
// Bedrock is not known only warns. "" is accepted (default chain).
func RegionProblem(region string) (msg string, invalid bool) {
	if region == "" {
		return "", false
	}
	if credproxy.CheckRegion(region) != nil {
		return i18n.Tf("cmd.proxy.region.invalid", region), true
	}
	if !credproxy.KnownBedrockRegion(region) {
		return i18n.Tf("cmd.proxy.region.unknown", region), false
	}
	return "", false
}
