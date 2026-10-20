package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/mcpresolve"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// URL of an MCP service imposed by the team (audit v5, S-31): the token of
// the service is sent to that URL, so it is used only once the member
// approved it with `oh mcp trust` (stored in the personal hub.toml as
// mcp.<service>.trusted_url). See internal/mcpresolve.

func init() {
	mcpCmd.AddCommand(mcpTrustCmd())
	mcpCmd.AddCommand(mcpUntrustCmd())
}

// mcpPinServices are the services that have a URL and a personal entry in
// hub.toml, so a URL to pin.
var mcpPinServices = []string{"figma", "gitlab", "gslides", "jira"}

func completeMCPPinServices(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return mcpPinServices, cobra.ShellCompDirectiveNoFileComp
}

func mcpTrustCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "trust <service>",
		Short:             i18n.T("cmd.mcp.trust.short"),
		Long:              i18n.T("cmd.mcp.trust.long"),
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeMCPPinServices,
		RunE:              runMCPTrust,
	}
	cmd.Flags().BoolP("yes", "y", false, i18n.T("cmd.mcp.trust.flags.yes"))
	cmd.Flags().StringP("project", "p", "", i18n.T("cmd.mcp.trust.flags.project"))
	_ = cmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
	return cmd
}

func mcpUntrustCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "untrust <service>",
		Short:             i18n.T("cmd.mcp.untrust.short"),
		Long:              i18n.T("cmd.mcp.untrust.long"),
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeMCPPinServices,
		RunE:              runMCPUntrust,
	}
}

// pinService returns the lower-case name of a service that has a URL to pin.
func pinService(name string) (string, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, s := range mcpPinServices {
		if s == name {
			return name, nil
		}
	}
	return "", errors.New(i18n.Tf("cmd.mcp.trust.unsupported", name, strings.Join(mcpPinServices, ", ")))
}

// ─── Team state ──────────────────────────────────────────────────────────────

// sharedMCPForTeam returns the MCP configuration the team shares (nil
// without team, or while the team-state is not cloned).
func sharedMCPForTeam(tc config.ResolvedTeamConfig) map[string]teamstate.SharedMCPConfig {
	if !tc.Enabled || tc.StatePath == "" {
		return nil
	}
	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
	if !repo.IsCloned() {
		return nil
	}
	cfg, err := repo.LoadConfig()
	if err != nil || cfg == nil {
		return nil
	}
	return cfg.MCP
}

// mcpTeamOf returns the team of --project, else of the project of the
// current directory, else of the hub.
func mcpTeamOf(cmd *cobra.Command, a *app.App) (config.ResolvedTeamConfig, error) {
	if id, _ := cmd.Flags().GetString("project"); id != "" {
		p, err := resolveProject(cmd.Context(), a, id, projectRead)
		if err != nil {
			return config.ResolvedTeamConfig{}, err
		}
		return resolvedTeamConfig(a, p), nil
	}
	if p, err := resolveActiveProject(a); err == nil && p != nil {
		return resolvedTeamConfig(a, p), nil
	}
	return resolvedTeamConfig(a, nil), nil
}

// ─── oh mcp trust ────────────────────────────────────────────────────────────

// lookupIPs resolves a host name (a variable for the tests).
var lookupIPs = func(ctx context.Context, host string) ([]net.IP, error) {
	return net.DefaultResolver.LookupIP(ctx, "ip", host)
}

// trustProposal is what `oh mcp trust` shows before it pins a URL.
type trustProposal struct {
	Service string
	Raw     string // as the team wrote it
	URL     string // normalized: what is pinned
	Pinned  string // URL pinned now ("" none)
	Addrs   []net.IP
	DNSErr  error
}

// Sensitive reports whether the host resolves to a private, loopback or
// link-local address.
func (p *trustProposal) Sensitive() bool {
	for _, ip := range p.Addrs {
		if mcpresolve.AddrKind(ip) != "" {
			return true
		}
	}
	return false
}

// planTrust checks the URL the team imposes on a service and resolves its
// host; it pins nothing.
func planTrust(ctx context.Context, service string, shared *teamstate.SharedMCPConfig, cur config.MCPServerConfig) (*trustProposal, error) {
	if shared == nil || !shared.IsURLEnforced() || shared.URL == "" {
		return nil, errors.New(i18n.Tf("cmd.mcp.trust.not_enforced", service))
	}
	norm, err := mcpresolve.NormalizeURL(shared.URL)
	if err != nil {
		return nil, errors.New(i18n.Tf("cmd.mcp.trust.refused", service, shared.URL, urlProblem(err)))
	}
	p := &trustProposal{Service: service, Raw: shared.URL, URL: norm, Pinned: cur.TrustedURL}
	p.Addrs, p.DNSErr = resolveHost(ctx, norm)
	return p, nil
}

// urlProblem is the localized reason NormalizeURL refused a URL.
func urlProblem(err error) string {
	if errors.Is(err, mcpresolve.ErrNotHTTPS) {
		return i18n.T("cmd.mcp.url.not_https")
	}
	return i18n.T("cmd.mcp.url.bad")
}

// resolveHost resolves the host of a URL (an address is not looked up).
func resolveHost(ctx context.Context, rawURL string) ([]net.IP, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil {
		return []net.IP{ip}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return lookupIPs(ctx, u.Hostname())
}

// hostLines describes the addresses of a host, the sensitive ones flagged.
func hostLines(addrs []net.IP, dnsErr error) []string {
	if dnsErr != nil {
		return []string{i18n.Tf("cmd.mcp.trust.resolve_failed", dnsErr)}
	}
	var out []string
	for _, ip := range addrs {
		line := ip.String()
		if kind := mcpresolve.AddrKind(ip); kind != "" {
			line += "  ⚠ " + i18n.T("cmd.mcp.trust.kind."+kind)
		}
		out = append(out, line)
	}
	return out
}

func printTrustProposal(out io.Writer, p *trustProposal) {
	fmt.Fprintf(out, "%s\n", i18n.Tf("cmd.mcp.trust.proposed", theme.Bold.Render(p.Service), theme.Bold.Render(p.URL)))
	if p.Raw != p.URL {
		fmt.Fprintf(out, "  %s\n", i18n.Tf("cmd.mcp.trust.written_as", p.Raw))
	}
	fmt.Fprintf(out, "%s\n", i18n.T("cmd.mcp.trust.resolves"))
	for _, l := range hostLines(p.Addrs, p.DNSErr) {
		fmt.Fprintf(out, "  %s\n", l)
	}
	if p.Sensitive() {
		fmt.Fprintf(out, "\n%s %s\n", theme.WarningStyle.Render(theme.IconWarning), i18n.Tf("cmd.mcp.trust.warn_private", p.Service))
	}
	if p.Pinned != "" && !mcpresolve.SameURL(p.Pinned, p.URL) {
		fmt.Fprintf(out, "\n%s\n", i18n.Tf("cmd.mcp.trust.replaces", p.Pinned))
	}
}

// askYes asks a question and reads the answer (o/oui/y/yes).
func askYes(in io.Reader, out io.Writer, question string) bool {
	fmt.Fprintf(out, "\n%s ", question)
	line, _ := bufio.NewReader(in).ReadString('\n')
	return confirmed(line, false)
}

func runMCPTrust(cmd *cobra.Command, args []string) error {
	service, err := pinService(args[0])
	if err != nil {
		return err
	}
	a := MustApp()
	tc, err := mcpTeamOf(cmd, a)
	if err != nil {
		return err
	}
	var shared *teamstate.SharedMCPConfig
	if s, ok := sharedMCPForTeam(tc)[service]; ok {
		shared = &s
	}
	yes, _ := cmd.Flags().GetBool("yes")
	return trustURL(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), service, shared, *a.Config.MCPServer(service), yes, stdinIsTerminal())
}

// trustURL shows the URL the team imposes on a service, asks for a
// confirmation (a terminal, or yes) and pins the URL.
func trustURL(ctx context.Context, in io.Reader, out io.Writer, service string, shared *teamstate.SharedMCPConfig, cur config.MCPServerConfig, yes, terminal bool) error {
	p, err := planTrust(ctx, service, shared, cur)
	if err != nil {
		return err
	}
	printTrustProposal(out, p)
	if p.Pinned != "" && mcpresolve.SameURL(p.Pinned, p.URL) {
		fmt.Fprintf(out, "\n%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.mcp.trust.already", p.Service, p.Pinned))
		return nil
	}
	if !yes {
		if !terminal {
			return errors.New(i18n.T("cmd.mcp.trust.no_terminal"))
		}
		if !askYes(in, out, i18n.Tf("cmd.mcp.trust.confirm", p.Service, p.URL)) {
			fmt.Fprintf(out, "%s\n", i18n.T("cmd.mcp.trust.cancelled"))
			return nil
		}
	}
	if err := pinMCPURL(p.Service, p.URL); err != nil {
		return err
	}
	fmt.Fprintf(out, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.mcp.trust.done", theme.Bold.Render(p.Service), p.URL))
	return nil
}

// pinMCPURL stores the approved URL in the personal hub.toml.
func pinMCPURL(service, normalized string) error {
	return config.Update(func(c *config.Config) error {
		s := c.MCPServer(service)
		if s == nil {
			return errors.New(i18n.Tf("cmd.mcp.trust.unsupported", service, strings.Join(mcpPinServices, ", ")))
		}
		s.TrustedURL = normalized
		return nil
	})
}

func runMCPUntrust(cmd *cobra.Command, args []string) error {
	service, err := pinService(args[0])
	if err != nil {
		return err
	}
	var previous string
	if err := config.Update(func(c *config.Config) error {
		s := c.MCPServer(service)
		previous, s.TrustedURL = s.TrustedURL, ""
		return nil
	}); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.config.save_failed"), err)
	}
	out := cmd.OutOrStdout()
	if previous == "" {
		fmt.Fprintf(out, "%s\n", i18n.Tf("cmd.mcp.untrust.none", service))
		return nil
	}
	fmt.Fprintf(out, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.mcp.untrust.done", theme.Bold.Render(service), previous))
	return nil
}

// ─── Warnings (oh mcp status, oh doctor, TUI) ────────────────────────────────

// enforcedURLNotice is a URL imposed by the team that is not used.
type enforcedURLNotice struct {
	Service string
	URL     string // as the team wrote it
	Pinned  string
	Reason  mcpresolve.PendingReason
}

// Message is the localized warning, with the command that fixes it.
func (n enforcedURLNotice) Message() string {
	switch n.Reason {
	case mcpresolve.PendingInvalid:
		_, err := mcpresolve.NormalizeURL(n.URL)
		return i18n.Tf("cmd.mcp.enforced.invalid", n.Service, n.URL, urlProblem(err))
	case mcpresolve.PendingPinnedOther:
		return i18n.Tf("cmd.mcp.enforced.pinned_other", n.Service, n.URL, n.Pinned, n.Service)
	}
	return i18n.Tf("cmd.mcp.enforced.not_pinned", n.Service, n.URL, n.Service)
}

// enforcedURLNotices lists the URLs the team imposes that the member did
// not approve.
func enforcedURLNotices(cfg *config.Config, shared map[string]teamstate.SharedMCPConfig) []enforcedURLNotice {
	var out []enforcedURLNotice
	for _, name := range mcpPinServices {
		s, ok := shared[name]
		local := cfg.MCPServer(name)
		if !ok || local == nil {
			continue
		}
		if eff := mcpresolve.Resolve(&s, *local); eff.EnforcedURLPending != "" {
			out = append(out, enforcedURLNotice{Service: name, URL: eff.EnforcedURLPending, Pinned: local.TrustedURL, Reason: eff.EnforcedURLReason})
		}
	}
	return out
}

// tuiEnforcedURLs is the data of the warning and of the T action of the MCP
// view of the TUI, for the team of the active project.
func tuiEnforcedURLs(a *app.App) map[string]*views.MCPEnforcedURL {
	project, _ := resolveActiveProject(a)
	out := map[string]*views.MCPEnforcedURL{}
	for _, n := range enforcedURLNotices(a.Config, sharedMCPForTeam(resolvedTeamConfig(a, project))) {
		e := &views.MCPEnforcedURL{Warning: n.Message()}
		if n.Reason != mcpresolve.PendingInvalid {
			e.URL, _ = mcpresolve.NormalizeURL(n.URL)
		}
		out[n.Service] = e
	}
	return out
}

// tuiHostInfo describes where the host of a URL resolves (it may block).
func tuiHostInfo(rawURL string) ([]string, bool) {
	addrs, err := resolveHost(context.Background(), rawURL)
	p := trustProposal{Addrs: addrs, DNSErr: err}
	return hostLines(addrs, err), p.Sensitive()
}

func printEnforcedURLNotices(out io.Writer, notices []enforcedURLNotice) {
	if len(notices) == 0 {
		return
	}
	fmt.Fprintln(out)
	for _, n := range notices {
		fmt.Fprintf(out, "  %s %s\n", theme.WarningStyle.Render(theme.IconWarning), n.Message())
	}
}
