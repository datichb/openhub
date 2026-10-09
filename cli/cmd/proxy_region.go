package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// checkBedrockRegion refuses a Bedrock region that is not an AWS region
// name (audit v5, S-21: the region names the host the credential is sent
// to) and warns about a region where Bedrock is not known to run. "" is
// accepted (default chain).
func checkBedrockRegion(warn io.Writer, region string) error {
	if region == "" {
		return nil
	}
	if credproxy.CheckRegion(region) != nil {
		return errors.New(i18n.Tf("cmd.proxy.region.invalid", region))
	}
	if !credproxy.KnownBedrockRegion(region) && warn != nil {
		fmt.Fprintln(warn, i18n.Tf("cmd.proxy.region.unknown", region))
	}
	return nil
}
