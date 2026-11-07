# A1 — `textfmt` : API, choix par défaut et doublons restants (entrée de A4-A7)

> Récapitulatif du lot A1 (23/10/2026), fusionné sur `main`.

## API
`Ago`/`AgoAt`, `Age`/`AgeAt`, `Duration`, `Date`, `DateTime`, `ShortDateTime`, `Clock`, `ClockSec` (`.Local()`, mises en page dans `fmt.*`), `Cost` (2 déc.), `CostPrecise` (4 déc.), `Bytes` (KiB/MiB/GiB ; FR Ko/Mo/Go), `Tokens` (« 1.5K »), `OnOff`, `Clip` (runes, « … » compté), `ClipWidth` (runewidth), `ClipLeft`, `FirstLine` (TrimSpace), `ClipLine` (sans TrimSpace), `ClipBytes`, `ClipBytesLeft`, `Now`/`SetNow` (horloge injectable).

## Décisions du 23/10 (utilisateur)
1. Coût FR « 0.12 $ », EN « $0.12 » (inchangé dans textfmt).
2. `Cost` à 2 décimales **avec plancher** : un coût > 0 et < 0,01 s'affiche « <$0.01 » (FR « <0.01 $ ») → **à coder** (+ clé `fmt.cost_floor` ou équivalent).
3. `ShortDateTime` EN « 01-02 15:04 », FR « 02/01 15:04 » → **à changer** dans `en.json` (`fmt.shortdatetime`).
4. `Tokens` « 1.5K » partout (inchangé).
5. `Bytes` en KiB/MiB/GiB ; FR en **Kio/Mio/Gio** (cohérent avec le choix d'unités exactes) → **à changer** dans `fr.json`.
6. `Ago` unifié tel quel (jours dès 24 h, pas de « hier »).
7. Toujours l'heure locale (`.Local()` partout).
Les ajustements 2, 3 et 5 sont faits dans le lot A2 (premier commit, paquet `textfmt` seulement).

## Choix par défaut (proposés par A1, voir décisions ci-dessus)
1. Coût FR « 0.12 $ » (comme `cmd.budget.today`) ; `metrics`, `session list`, `v5_existing` affichent « $0.12 » aujourd'hui.
2. `Cost` à 2 décimales ; `session list`, `session_control`, `eval` en ont 3 (« $0.00 » sous 0,005 $ : plancher « <$0.01 » ?).
3. `ShortDateTime` « 02/01 15:04 » en EN aussi (ambigu).
4. `Tokens` « 1.5K » ; `views/metrics_view.go` affiche « 2K ».
5. `Bytes` en KiB/MiB (`purge` affichait MB/KB).
6. `Ago` : jours à 24 h (`agoLabel` : 48 h) ; plus de « yesterday » ni « 1 hour ago » ; repli date après 7 j fait par l'appelant.
7. Tout en `.Local()` (sites sans `.Local()` : `v5_existing:95`, `serve_api:330`, `export:308`, `session:197`, `history:97,148`, `takeover_brief:135`, `notifications:86`, `takeover_view:87,175`, `wiki_view:272,333,539`, `teamstatus_view:242`).

Gardés hors `textfmt` : `session/trace.go` `clipText` (note « [N bytes truncated] »), `workflow/render_prompt.go` `truncate` (note FR dans le prompt).

## Doublons restants dans `cmd/` et `internal/tui/**`

**Coupes** → `Clip` : `cmd/tui_launch_inputs.go` clipText, `cmd/session_batch.go` truncate, `cmd/session_trace.go` clipRunes, `views/sessions_view.go` clipText, `views/bricks_view.go` truncateRunes, `views/teams_view.go` truncateString, `views/board.go` truncateTitle, `widgets/graphlayout.go` truncate (n<=0 : `""`) ; `cmd/teams.go` truncateStr (octets + « ... » → runes + « … ») ; en ligne : `views/board_quick_actions.go:67,90`, `views/teamboard_view.go:368`, `views/metrics_view.go:302`, `shell/toast.go:88`, `cmd/tui_team_tracker.go:410`.
`cmd/wiki.go` clipLine → `ClipLine`. `views/launch_form_existing.go` clipLeft → `ClipLeft(s, max(n,8))`. `views/beads_picker.go:529` → `ClipWidth`.
À garder : `cmd/review_feedback.go` clipNote, `shortHash`/`shortSessionID`, `widgets/sectioned_list.go` TruncateTagged.

**Âges, durées** : `cmd/tui_start.go` agoLabel → `AgoAt` ; `views/sessions_view.go` ago → `Ago` (« 3m ago » → « 3 min ago ») ; `views/wiki_proposals.go` shortAge → `AgeAt` ; `views/wiki_view.go` wikiFormatTimeAgo et `views/teamstatus_view.go` formatTimeAgo → `Ago` + `Date` après 7 j ; `cmd/session_control.go` since → `Ago` ; `cmd/team.go` formatDuration → `Duration` ; `cmd/history.go:89` → `Duration`. À garder : `cmd/tui_launch_runtime.go` approxDuration.

**Coûts, tailles, nombres** : `cmd/dashboard_cmd.go` formatTokens, `cmd/metrics.go` formatTokenCount, `views/metrics_view.go` formatTokens → `Tokens` ; `cmd/purge.go` formatBytes, `cmd/session_report.go` et `views/sessions_tabs.go` reportSize → `Bytes` ; coûts `cmd/metrics.go:114,137`, `v5_existing.go:94`, `session.go:196`, `session_control.go:404`, `budget_report.go:122`, `eval.go:1001`, `serve_api.go:264` → `Cost` ; `cmd/session_trace.go:131` → `CostPrecise` ; `cmd/config.go:619-621`, `cmd/misc.go:115`, `cmd/mcp.go:612`, `views/config_field.go`, `mcp_view.go` → `OnOff`. À garder : `cmd/team_config.go` fmtBool.

**Dates** :
- `DateTime` : `cmd/{v5_existing,gc,serve_api,doctor_credentials,team_wiki:199,export:308,workflow_history,brick_publish,history:97,148,session_report:60}.go`, `views/{takeover_view:87,wiki_view:272,333}.go`, `views/history_view.go:205`.
- `Date` : `cmd/team_wiki.go:97,162`, `views/takeover_view.go:175`, `bricks_view.go:371`, `takeover_brief.go:135`.
- `ShortDateTime` : `cmd/{session,claim:212}.go`, `views/history_view.go:162`, `shell/notifications.go:174` (perd les secondes).
- `Clock` : `cmd/team.go:701`, `cmd/session_report.go:89`, `views/{sessions_tabs:319,sessions_view:1037,activity_view:152,sessions_checkpoint_reports:77}.go`.
- `ClockSec` : `cmd/{team_sync_tracker,fanout:284,session_control:399,session_trace:85}.go`, `shell/notifications.go:86`.

**Clés à retirer en migrant** (sinon `TestNoOrphanKeys`) : `tui.pm.age.*`, `tui.wikilist.age.*`, `tui.start.ago_*`, `tui.wiki.time_*`, `tui.teamstatus.time_*`, `cmd.session.ago`, `cmd.session.report.bytes/kib`, `tui.checkpoint.report_bytes/kib`, `websearch_on/off`, `status_enabled/disabled`, `tui.config.enabled/disabled`.

## Hors `cmd/` et `tui/` (non migrés, hors périmètre)
Coupes : `credproxy/authreject.go:73`, `credproxy/relay.go:153`, `evalrun/report.go:354`, `specdoc/specdoc.go:384`, `services/wiki/propose.go:131`, `services/report/report.go:115`, `tracker/tracker.go` TruncateDescription, `beads/beads.go:702`. Tailles : `mcp/workflow/server_reports.go` size. Dates : `services/checkpoint/timeline.go:114`, `mcp/workflow/server_reports.go:130`, `daemon/auth_alert.go:133`, `prettylog/handler.go:99`, `evalrun/tier.go:397`, `teamstate/lock.go:192`. `teamstate/takeover_template.go` écrit « 02 Jan » dans des fichiers (ne pas migrer sans décision).
