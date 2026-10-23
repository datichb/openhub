package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// MCPService represents the hub-level state of an MCP service.
type MCPService struct {
	Name         string
	Enabled      bool
	HasToken     bool
	WriteEnabled bool
	// Enforced is the URL the team imposes when it is not used (audit v5,
	// S-31); nil when there is none.
	Enforced *MCPEnforcedURL
}

// MCPEnforcedURL is a URL the team imposes on a service that the member has
// not approved: the service token is not sent to it until it is pinned.
type MCPEnforcedURL struct {
	// URL is the normalized URL that pinning stores; empty when it is refused
	// (not https…) and cannot be pinned.
	URL string
	// Warning is the localized warning, with the command that fixes it.
	Warning string
}

// MCPViewConfig holds configuration for the MCP view.
type MCPViewConfig struct {
	// GetConfig returns the live hub config pointer.
	GetConfig func() *config.Config
	// SaveConfig persists the modified config to hub.toml (unified mutation path).
	SaveConfig func(c *config.Config) error
	// EnforcedURLs prepares, by service, the URL the team imposes when it is
	// not used (empty: none, or no team). Called on the event loop, which owns
	// the config it reads; the function it returns runs off the loop (it may
	// read the disk) and must not touch what the loop writes. Optional.
	EnforcedURLs func() func() map[string]*MCPEnforcedURL
	// HostInfo describes the addresses a URL resolves to, and whether one is
	// private, loopback or link-local. It may block: the view calls it off
	// the event loop. Optional.
	HostInfo func(url string) (lines []string, sensitive bool)
}

// trustKey pins the URL the team imposes on the selected service (the same
// as `oh mcp trust`, after a confirmation).
var trustKey = keys.Own("mcp.trust", 'T', "tui.mcp.trust_key")

// MCPView displays MCP server management with a SectionedList panel
// for hub-level configuration (enable/disable, token, write).
//
// Per-project MCP overrides are managed exclusively in ProjectConfigView
// (single mutation path — ADR-031).
type MCPView struct {
	app    *tview.Application
	appCtx *app.App
	cfg    MCPViewConfig
	shell  ShellAccess

	// Layout
	list *widgets.SectionedList
	page *widgets.ListDetailPage

	// Data
	services []MCPService

	commands []ContextCommand
	// loads numbers the reads of the services, stores the writes of a token:
	// a result is applied only while it is the latest of its slot and the
	// view is still mounted. hosts numbers the look-ups of a host.
	loads  loadSlot
	stores loadSlot
	hosts  loadSlot
}

var _ View = (*MCPView)(nil)
var _ CommandProvider = (*MCPView)(nil)

// NewMCPView creates a new MCP view.
func NewMCPView(a *app.App, cfg MCPViewConfig) *MCPView {
	return &MCPView{appCtx: a, cfg: cfg}
}

// SetShell provides the shell reference for interactions.
func (v *MCPView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *MCPView) ID() string { return "mcp" }

// Title returns the display title.
func (v *MCPView) Title() string { return i18n.T("tui.mcp.title") }

// StatusHints returns keybinding hints.
func (v *MCPView) StatusHints() string {
	return keys.Hints(keys.Navigate.Hint(), "{/} "+i18n.T("tui.hints.sections"),
		keys.SpaceName()+" "+i18n.T("tui.hints.toggle"),
		"t "+i18n.T("tui.hints.token"), "w "+i18n.T("tui.hints.write"), trustKey.Hint(), keys.Refresh.Hint())
}

// Mount builds the MCP management interface.
func (v *MCPView) Mount(content *tview.Flex, tvApp *tview.Application) {
	v.app = tvApp

	// Show loading placeholder immediately
	loading := stateView(stateLoading, i18n.T("tui.mcp.loading"))
	content.AddItem(loading, 0, 1, true)

	// Read the services off the event loop (keychain, team config), then
	// build the widgets on it.
	v.reloadServices(func() {
		v.list = widgets.NewSectionedList()
		v.list.SetApp(tvApp)
		v.page = newListPage(v.Title(), v.list)
		v.list.SetItemChangedFunc(func(_ int, it widgets.SectionItem) {
			if i, ok := it.Reference.(int); ok {
				v.showDetail(i)
			}
		})

		v.populateList()
		v.buildCommands()

		v.page.Mount(content)
		if tvApp != nil {
			tvApp.SetFocus(v.list)
		}
	}, func(err error) { setState(loading, stateError, err.Error()) })
}

// Unmount cleans up resources.
func (v *MCPView) Unmount() {
	v.loads.Invalidate() // drop the loads in flight
	v.stores.Invalidate()
	v.hosts.Invalidate()
	v.app = nil
	v.list = nil
	v.page = nil
	v.commands = nil
}

// HandleKey processes key events.
func (v *MCPView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if keys.Refresh.Matches(event) {
		if v.list == nil {
			return nil
		}
		v.reloadServices(func() {
			if v.list == nil {
				return
			}
			v.populateList()
			if v.shell != nil {
				v.shell.ShowToastMsg(i18n.T("tui.mcp.refreshed"), true)
			}
		}, v.failed)
		return nil
	}
	if trustKey.Matches(event) {
		v.trustCurrent()
		return nil
	}
	if event.Key() != tcell.KeyRune {
		return event
	}
	switch event.Rune() {
	case ' ':
		v.toggleHubCurrent()
		return nil
	case 't':
		v.promptTokenCurrent()
		return nil
	case 'w':
		v.toggleWriteCurrent()
		return nil
	}
	return event
}

// ContextCommands returns contextual commands for the omnibar.
func (v *MCPView) ContextCommands() []ContextCommand {
	return v.commands
}

// ─────────────────────────────────────────────────────────────────────────────
// List management
// ─────────────────────────────────────────────────────────────────────────────

func (v *MCPView) populateList() {
	if v.list == nil {
		return
	}

	items := make([]widgets.SectionItem, 0, len(v.services)+2)
	items = append(items, widgets.SectionItem{
		IsHeader: true,
		MainText: i18n.T("tui.mcp.section_hub"),
	})

	for i, svc := range v.services {
		items = append(items, widgets.SectionItem{
			MainText:      v.hubItemText(svc),
			SecondaryText: v.hubItemSubtext(svc),
			Reference:     i,
		})
	}

	v.list.SetItems(items)
	v.showDetail(v.currentServiceIndex())
}

// showDetail fills the detail pane with the service at index i.
func (v *MCPView) showDetail(i int) {
	if v.page == nil {
		return
	}
	if i < 0 || i >= len(v.services) {
		v.page.SetDetail(emptyDetail())
		return
	}
	svc := v.services[i]
	onOff := func(b bool) string {
		if b {
			return theme.ColorTag(theme.SuccessHex) + theme.SymSuccess + " " + i18n.T("tui.config.enabled") + theme.TagReset
		}
		return theme.ColorTag(theme.TextMutedHex) + "✗ " + i18n.T("tui.config.disabled") + theme.TagReset
	}
	var d detailText
	d.title(svc.Name)
	if svc.Name == "team" {
		d.para(i18n.T("tui.detail.mcp.team_desc"))
	} else {
		d.para(i18n.Tf("tui.detail.mcp.hub_desc", svc.Name))
	}
	d.field(i18n.T("tui.detail.mcp.enabled"), onOff(svc.Enabled))
	switch {
	case svc.Name == "team":
		d.field(i18n.T("tui.detail.mcp.token"), i18n.T("tui.mcp.no_token"))
	case svc.HasToken:
		d.field(i18n.T("tui.detail.mcp.token"), theme.ColorTag(theme.SuccessHex)+theme.SymSuccess+" "+i18n.T("tui.config.configured")+theme.TagReset)
	default:
		d.field(i18n.T("tui.detail.mcp.token"), theme.ColorTag(theme.ErrorHex)+"✗ "+i18n.T("tui.mcp.token_missing")+theme.TagReset)
	}
	if svc.Name == "gitlab" {
		d.field(i18n.T("tui.detail.mcp.write"), onOff(svc.WriteEnabled))
	}
	if e := svc.Enforced; e != nil {
		d.field(i18n.T("tui.detail.mcp.team_url"), theme.ColorTag(theme.WarningHex)+"⚠ "+e.Warning+theme.TagReset)
	}
	pairs := []string{keys.SpaceName(), i18n.T("tui.hints.toggle")}
	if svc.Name != "team" {
		pairs = append(pairs, "t", i18n.T("tui.hints.token"))
	}
	if svc.Name == "gitlab" {
		pairs = append(pairs, "w", i18n.T("tui.hints.write"))
	}
	if svc.Enforced != nil {
		pairs = append(pairs, keys.Pairs(trustKey)...)
	}
	d.keys(append(pairs, keys.Pairs(keys.Refresh)...)...)
	v.page.SetDetail(d.String())
}

func (v *MCPView) hubItemText(svc MCPService) string {
	var statusIcon, statusColor string
	if svc.Enabled {
		statusIcon = theme.SymSuccess
		statusColor = theme.SuccessHex
	} else {
		statusIcon = "✗"
		statusColor = theme.TextMutedHex
	}
	return fmt.Sprintf("%s%s%s  %s%-12s%s",
		theme.ColorTag(statusColor), statusIcon, theme.TagColor,
		theme.ColorTag(theme.TextPrimaryHex), svc.Name, theme.TagColor)
}

func (v *MCPView) hubItemSubtext(svc MCPService) string {
	var parts []string

	// Token indicator
	switch {
	case svc.Name == "team":
		parts = append(parts, fmt.Sprintf("%s(%s)%s", theme.ColorTag(theme.TextMutedHex), i18n.T("tui.mcp.no_token"), theme.TagColor))
	case svc.HasToken:
		parts = append(parts, fmt.Sprintf("%s%s %s%s", theme.ColorTag(theme.SuccessHex), i18n.T("tui.hints.token"), theme.SymSuccess, theme.TagColor))
	default:
		parts = append(parts, fmt.Sprintf("%s%s !%s", theme.ColorTag(theme.ErrorHex), i18n.T("tui.mcp.token_missing"), theme.TagColor))
	}

	// Write mode (gitlab only)
	if svc.Name == "gitlab" {
		if svc.WriteEnabled {
			parts = append(parts, fmt.Sprintf("%s%s %s%s", theme.ColorTag(theme.SuccessHex), i18n.T("tui.hints.write"), theme.SymSuccess, theme.TagColor))
		} else {
			parts = append(parts, fmt.Sprintf("%s%s ✗%s", theme.ColorTag(theme.TextMutedHex), i18n.T("tui.hints.write"), theme.TagColor))
		}
	}

	if svc.Enforced != nil {
		parts = append(parts, fmt.Sprintf("%s⚠ URL%s", theme.ColorTag(theme.WarningHex), theme.TagColor))
	}

	return strings.Join(parts, "  ")
}

// ─────────────────────────────────────────────────────────────────────────────
// Hub-level actions — unified mutation via config.Save()
// ─────────────────────────────────────────────────────────────────────────────

func (v *MCPView) currentServiceIndex() int {
	if v.list == nil {
		return -1
	}
	_, item, ok := v.list.CurrentItem()
	if !ok {
		return -1
	}
	idx, ok := item.Reference.(int)
	if !ok || idx < 0 || idx >= len(v.services) {
		return -1
	}
	return idx
}

func (v *MCPView) toggleHubCurrent() {
	idx := v.currentServiceIndex()
	if idx < 0 {
		return
	}
	svc := &v.services[idx]
	svc.Enabled = !svc.Enabled

	// Mutate live config and save
	cfg := v.cfg.GetConfig()
	v.setMCPEnabled(cfg, svc.Name, svc.Enabled)
	if err := v.cfg.SaveConfig(cfg); err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.mcp.save_error")+": "+err.Error(), false)
		}
		return
	}

	v.populateList()
	v.buildCommands()

	if v.shell != nil {
		if svc.Enabled {
			v.shell.ShowToastMsg(svc.Name+" "+i18n.T("tui.mcp.enabled"), true)
		} else {
			v.shell.ShowToastMsg(svc.Name+" "+i18n.T("tui.mcp.disabled"), true)
		}
	}
}

func (v *MCPView) promptTokenCurrent() {
	idx := v.currentServiceIndex()
	if idx < 0 {
		return
	}
	svc := v.services[idx]
	if svc.Name == "team" {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.mcp.team_no_token"), false)
		}
		return
	}
	if v.shell != nil {
		v.shell.ShowPasswordModal(i18n.Tf("tui.mcp.token_title", svc.Name), func(token string) {
			if token != "" && v.appCtx != nil && v.appCtx.Secrets != nil {
				v.storeToken(svc.Name, token)
			}
		})
	}
}

// storeToken writes the token in the keychain, which may block (a locked
// keychain waits for the user), off the event loop; the list shows it once
// it is stored.
func (v *MCPView) storeToken(name, token string) {
	secrets := v.appCtx.Secrets
	load(v.app, context.Background(), &v.stores, func(ctx context.Context) (struct{}, error) {
		_, err := config.StoreMCPToken(ctx, secrets, name, "", token)
		return struct{}{}, err
	}, func(_ struct{}, err error) {
		if v.shell == nil {
			return
		}
		if err != nil {
			v.shell.ShowToastMsg(errorText(err), false)
			return
		}
		for i := range v.services {
			if v.services[i].Name == name {
				v.services[i].HasToken = true
			}
		}
		v.populateList()
		v.buildCommands()
		v.shell.ShowToastMsg(i18n.Tf("tui.mcp.token_saved", name), true)
	})
}

func (v *MCPView) toggleWriteCurrent() {
	idx := v.currentServiceIndex()
	if idx < 0 {
		return
	}
	svc := &v.services[idx]
	if svc.Name != "gitlab" {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.mcp.write_gitlab_only"), false)
		}
		return
	}
	svc.WriteEnabled = !svc.WriteEnabled

	cfg := v.cfg.GetConfig()
	cfg.MCP.Gitlab.WriteEnabled = svc.WriteEnabled
	if err := v.cfg.SaveConfig(cfg); err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.mcp.save_error")+": "+err.Error(), false)
		}
		return
	}

	v.populateList()
	v.buildCommands()

	if v.shell != nil {
		if svc.WriteEnabled {
			v.shell.ShowToastMsg(i18n.T("tui.mcp.write_enabled"), true)
		} else {
			v.shell.ShowToastMsg(i18n.T("tui.mcp.write_disabled"), true)
		}
	}
}

// trustCurrent asks to pin the URL the team imposes on the selected service.
// The addresses of its host are looked up off the event loop, then shown in
// the confirmation (a private address is flagged).
func (v *MCPView) trustCurrent() {
	idx := v.currentServiceIndex()
	if idx < 0 || v.shell == nil || v.app == nil {
		return
	}
	svc := v.services[idx]
	switch {
	case svc.Enforced == nil:
		v.shell.ShowToastMsg(i18n.Tf("tui.mcp.trust_none", svc.Name), false)
		return
	case svc.Enforced.URL == "":
		v.shell.ShowToastMsg(svc.Enforced.Warning, false)
		return
	}
	url := svc.Enforced.URL
	type host struct {
		lines     []string
		sensitive bool
	}
	load(v.app, context.Background(), &v.hosts, func(context.Context) (host, error) {
		var h host
		if v.cfg.HostInfo != nil {
			h.lines, h.sensitive = v.cfg.HostInfo(url)
		}
		return h, nil
	}, func(h host, err error) {
		if err != nil {
			v.failed(err)
			return
		}
		if v.shell != nil {
			v.confirmTrust(svc.Name, url, h.lines, h.sensitive)
		}
	})
}

func (v *MCPView) confirmTrust(name, url string, hostLines []string, sensitive bool) {
	detail := url + "\n\n" + i18n.T("tui.mcp.trust_resolves") + "\n  " + strings.Join(hostLines, "\n  ")
	if sensitive {
		detail += "\n\n⚠ " + i18n.Tf("tui.mcp.trust_private", name)
	}
	detail += "\n\n" + i18n.Tf("tui.mcp.trust_detail", name)
	v.shell.Confirm(i18n.Tf("tui.mcp.trust_title", name), detail, sensitive, func() { v.pinURL(name, url) })
}

// pinURL stores the approved URL in the personal config.
func (v *MCPView) pinURL(name, url string) {
	cfg := v.cfg.GetConfig()
	s := cfg.MCPServer(name)
	if s == nil {
		return
	}
	s.TrustedURL = url
	if err := v.cfg.SaveConfig(cfg); err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.mcp.save_error")+": "+err.Error(), false)
		}
		return
	}
	if v.list == nil {
		return // left meanwhile: the next mount reads the services
	}
	// The services are read again (keychain, team config) off the event loop.
	v.reloadServices(func() {
		v.populateList()
		v.buildCommands()
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.Tf("tui.mcp.trust_done", name, url), true)
		}
	}, v.failed)
}

// failed tells that the services could not be read.
func (v *MCPView) failed(err error) {
	if v.shell != nil {
		v.shell.ShowToastMsg(errorText(err), false)
	}
}

func (v *MCPView) toggleService(name string, enable bool) {
	for i := range v.services {
		if v.services[i].Name != name {
			continue
		}
		v.services[i].Enabled = enable
		cfg := v.cfg.GetConfig()
		v.setMCPEnabled(cfg, name, enable)
		if err := v.cfg.SaveConfig(cfg); err != nil {
			if v.shell != nil {
				v.shell.ShowToastMsg(errorText(err), false)
			}
		}
		v.populateList()
		v.buildCommands()
		return
	}
}

// setMCPEnabled sets the enabled field on the corresponding MCPServerConfig.
func (v *MCPView) setMCPEnabled(cfg *config.Config, name string, enabled bool) {
	switch name {
	case "figma":
		cfg.MCP.Figma.Enabled = enabled
	case "gitlab":
		cfg.MCP.Gitlab.Enabled = enabled
	case "jira":
		cfg.MCP.Jira.Enabled = enabled
	case "gslides":
		cfg.MCP.Gslides.Enabled = enabled
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Data loading
// ─────────────────────────────────────────────────────────────────────────────

// mcpSnapshot is what loadServices reads of the live config; it is taken on
// the event loop, which is the one that writes the config.
type mcpSnapshot struct {
	figma, gitlab, gslides, jira, gitlabWrite bool
	// enforced reads the URLs imposed by the team (nil: none), on a snapshot.
	enforced func() map[string]*MCPEnforcedURL
}

func (v *MCPView) snapshotServices() mcpSnapshot {
	var snap mcpSnapshot
	if v.cfg.GetConfig != nil {
		cfg := v.cfg.GetConfig()
		snap = mcpSnapshot{
			figma: cfg.MCP.Figma.Enabled, gitlab: cfg.MCP.Gitlab.Enabled, gslides: cfg.MCP.Gslides.Enabled,
			jira: cfg.MCP.Jira.Enabled, gitlabWrite: cfg.MCP.Gitlab.WriteEnabled,
		}
	}
	if v.cfg.EnforcedURLs != nil {
		snap.enforced = v.cfg.EnforcedURLs()
	}
	return snap
}

// readServices lists the services: it reads the keychain and the team
// configuration, so it runs off the event loop.
func (v *MCPView) readServices(snap mcpSnapshot) []MCPService {
	services := []MCPService{
		{Name: "figma", Enabled: snap.figma, HasToken: v.checkToken("figma"), WriteEnabled: false},
		{Name: "gitlab", Enabled: snap.gitlab, HasToken: v.checkToken("gitlab"), WriteEnabled: snap.gitlabWrite},
		{Name: "gslides", Enabled: snap.gslides, HasToken: v.checkToken("gslides"), WriteEnabled: false},
		{Name: "jira", Enabled: snap.jira, HasToken: v.checkToken("jira"), WriteEnabled: false},
		{Name: "team", Enabled: true, HasToken: true, WriteEnabled: false},
	}
	if snap.enforced != nil {
		enforced := snap.enforced()
		for i := range services {
			services[i].Enforced = enforced[services[i].Name]
		}
	}
	return services
}

// loadServices reads the services in place, on the calling goroutine (tests
// only: the view reads them with reloadServices).
func (v *MCPView) loadServices() { v.services = v.readServices(v.snapshotServices()) }

// reloadServices reads the services off the event loop, then calls then on
// it (fail, with the error, when the read failed). The latest request wins;
// Unmount drops the one in flight.
func (v *MCPView) reloadServices(then func(), fail func(error)) {
	snap := v.snapshotServices()
	load(v.app, context.Background(), &v.loads, func(context.Context) ([]MCPService, error) {
		return v.readServices(snap), nil
	}, func(services []MCPService, err error) {
		if err != nil {
			if fail != nil {
				fail(err)
			}
			return
		}
		v.services = services
		then()
	})
}

func (v *MCPView) checkToken(serviceName string) bool {
	if serviceName == "team" {
		return true
	}
	if v.appCtx == nil || v.appCtx.Secrets == nil {
		return false
	}
	val, err := config.ReadMCPToken(context.Background(), v.appCtx.Secrets, serviceName, "")
	return err == nil && val != ""
}

// ─────────────────────────────────────────────────────────────────────────────
// Contextual commands
// ─────────────────────────────────────────────────────────────────────────────

func (v *MCPView) buildCommands() {
	var cmds []ContextCommand

	for _, svc := range v.services {
		svc := svc
		var desc string
		if svc.Enabled {
			desc = theme.SymSuccess + " → " + i18n.T("tui.mcp.cmd_disable")
		} else {
			desc = "✗ → " + i18n.T("tui.mcp.cmd_enable")
		}
		cmds = append(cmds, ContextCommand{
			ID:          "toggle." + svc.Name,
			Label:       svc.Name,
			Aliases:     []string{"toggle " + svc.Name},
			Description: desc,
			Category:    v.Title(),
			Action:      func() { v.toggleService(svc.Name, !svc.Enabled) },
		})
		if svc.Name != "team" {
			cmds = append(cmds, ContextCommand{
				ID:          "token." + svc.Name,
				Label:       "token " + svc.Name,
				Aliases:     []string{svc.Name + " token"},
				Description: i18n.T("tui.mcp.cmd_token"),
				Category:    v.Title(),
				Action: func() {
					for i, s := range v.services {
						if s.Name == svc.Name {
							v.list.SelectIndex(i + 1) // +1 for header
							v.promptTokenCurrent()
							return
						}
					}
				},
			})
		}
	}

	for _, svc := range v.services {
		if svc.Enforced == nil {
			continue
		}
		name := svc.Name
		cmds = append(cmds, ContextCommand{
			ID:          "trust." + name,
			Label:       "trust " + name,
			Aliases:     []string{name + " trust"},
			Description: i18n.T("tui.mcp.cmd_trust"),
			Category:    v.Title(),
			Action: func() {
				for i, s := range v.services {
					if s.Name == name {
						v.list.SelectIndex(i + 1) // +1 for header
						v.trustCurrent()
						return
					}
				}
			},
		})
	}

	cmds = append(cmds, ContextCommand{
		ID:          "write.gitlab",
		Label:       "gitlab " + i18n.T("tui.hints.write"),
		Aliases:     []string{"write gitlab"},
		Description: i18n.T("tui.mcp.cmd_write_toggle"),
		Category:    v.Title(),
		Action: func() {
			for i, s := range v.services {
				if s.Name == "gitlab" {
					v.list.SelectIndex(i + 1) // +1 for header
					v.toggleWriteCurrent()
					return
				}
			}
		},
	})

	v.commands = cmds
}
