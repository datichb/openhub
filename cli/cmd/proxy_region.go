package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/datichb/openhub/cli/internal/provider"
)

// checkBedrockRegion refuses a Bedrock region that is not an AWS region
// name (audit v5, S-21: the region names the host the credential is sent
// to) and warns about a region where Bedrock is not known to run. "" is
// accepted (default chain). Same rule as the TUI and oh init as typed
// (provider.RegionProblem).
func checkBedrockRegion(warn io.Writer, region string) error {
	msg, invalid := provider.RegionProblem(region)
	if invalid {
		return errors.New(msg)
	}
	if msg != "" && warn != nil {
		fmt.Fprintln(warn, msg)
	}
	return nil
}
