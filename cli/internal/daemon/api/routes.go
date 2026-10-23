package api

import (
	"net/url"
	"strings"
)

// Routes of the daemon API, without the Prefix. A segment in braces is a
// path parameter (see Route).
const (
	RouteHealth          = "/health"
	RouteGrants          = "/grants"
	RouteGrantsSecret    = "/grants/secret"
	RouteGrantsPending   = "/grants/pending"
	RouteOwnerGrants     = "/owners/{owner}/grants"
	RouteUsage           = "/usage"
	RouteServerTouch     = "/servers/{group}/touch"
	RouteShutdown        = "/shutdown"
	RouteClientHeartbeat = "/clients/heartbeat"
	RouteClient          = "/clients/{id}"
	RouteClients         = "/clients"
	RouteGroupPolicy     = "/groups/{group}/policy"
	RouteProxyListeners  = "/proxy/listeners"
	RouteGatewayGrants   = "/gateway/grants"
	RouteHookToken       = "/gateway/hook-token"
	RouteStream          = "/stream"
	RouteSessionDecided  = "/sessions/{id}/decided"

	RouteWorkflowStatus     = "/workflow/status"
	RouteWorkflowCheckpoint = "/workflow/checkpoint"
	RouteWorkflowOutputs    = "/workflow/outputs"
	RouteWorkflowReport     = "/workflow/report"
	RouteWorkflowReports    = "/workflow/reports"
	RouteWorkflowRules      = "/workflow/rules"

	RouteWikiIndex   = "/wiki/index"
	RouteWikiPage    = "/wiki/page"
	RouteWikiPropose = "/wiki/propose"
	RouteWikiPending = "/wiki/pending"
	RouteWikiRefused = "/wiki/refused"
	RouteWikiApply   = "/wiki/apply"
	RouteWikiCheck   = "/wiki/check"
)

// Route fills the parameters of a route pattern, escaped for a path:
// Route(RouteClient, "id", x) is "/clients/<x>". params are name/value pairs.
func Route(pattern string, params ...string) string {
	for i := 0; i+1 < len(params); i += 2 {
		pattern = strings.ReplaceAll(pattern, "{"+params[i]+"}", url.PathEscape(params[i+1]))
	}
	return pattern
}
