# 02 — Constats sur opencode v2.0.20 (tests réels) et recettes reproductibles

Tous les tests ont été faits le 05/10/2026 sur macOS, opencode **v2.0.20** (Homebrew), git 2.54, Colima 0.10.1, Podman présent, modèle Bedrock Haiku 4.5 EU. Dossier de travail : `$TMPDIR/opencode/expN` (hors dépôt).

## F — Constats

| # | Constat | Statut | Conséquence |
|---|---|---|---|
| F1 | `opencode --agent X` (TUI) n'existe pas en V2 : affiche l'aide et quitte. `--agent` n'existe que pour `run`. | ❌ | **Tous les lancements oh (CLI + TUI) cassés sous V2.** |
| F2 | Sans `--standalone`/`serve`, les clients parlent au **service partagé** (`opencode service status` → `http://127.0.0.1:49374`) qui **ignore** les variables d'environnement du client. | ❌ | Toujours lancer un serveur dédié. |
| F3 | Avec `serve` dédié ou `--standalone` : `OPENCODE_CONFIG_DIR`, `OPENCODE_CONFIG_CONTENT`, `OPENCODE_CONFIG`, `XDG_CONFIG_HOME` **fonctionnent**. | ✅ | Paquet par session sans écrire dans le projet. |
| F4 | `OPENCODE_DISABLE_PROJECT_CONFIG=1` (et `OPENCODE_CONFIG_PROJECT_DISABLE=1`) exclut `.opencode/` et `opencode.json` du projet. | ✅ | Isolation du projet. |
| F5 | Config en ligne (`agents` + `disabled` sur build/plan/general/explore + `skills:[dir]`) → seuls nos agents visibles. **Restent 2 skills intégrées : `opencode`, `report`** → à refuser (`skill` deny). `~/.config/opencode` toujours chargé (contournable par `XDG_CONFIG_HOME`). | ✅ | Monde fermé atteignable. |
| F6 | Config **V1** (`agent`, `permission{bash,task,skill,edit}`, `disable`, `prompt`) lue par V2 et traduite correctement (`bash→shell`, `task→subagent`, `prompt→system`). | ✅ | Compat V1 peu coûteuse. |
| F7 | `opencode serve` génère un **mot de passe** (Basic, utilisateur `opencode`) ou utilise `OPENCODE_SERVER_PASSWORD`. Agents/skills chargés **en asynchrone** (prêt ≈ 1,4 s). | ✅ | Attendre la disponibilité (polling `agent.list`). |
| F8 | `instructions` : « accepté mais non chargé » (doc). | 📄 | Ne jamais écrire `AGENTS.md` ; injecter autrement (O2/S8). |
| F9 | **Profondeur de délégation par défaut = 1** ; la clé V1 `subagent_depth` (écrite à la racine par le hub) est ignorée ; **`experimental.subagent_depth: 3`** → chaîne L0→L1→L2→L3 OK. | ✅ | `feature` cassé sous V2 tant que non corrigé. |
| F10 | Prompt système envoyé (capturé par plugin `session.hook("context")`) = 4 blocs : **[1] prompt de base opencode (~1,8k car.)**, [2] infos modèle, **[3] « Code Mode » (~10,5k car.)**, [4] conseil worktree. Un agent avec `system` **remplace uniquement le bloc [1]**. | ✅ | O2 (plugin) + CM (code_mode). |
| F11 | Interface interactive `--standalone` + config par env + `default_agent` → s'ouvre directement sur notre agent (Build/Plan absents). | ✅ | Secours. |
| F12 | `POST /api/session {id, agent, title, location, permissions}` puis `opencode --server URL -s <id>` → TUI attachée sur la session et le bon agent ; `opencode run --server URL -s <id>` continue la session. | ✅ | Lancement O3. |
| F13 | Isolation des données par `XDG_DATA_HOME` (crée `opencode.db`, logs, snapshot…) ; `OPENCODE_DATA_HOME` n'existe plus. | ✅ | Par groupe de serveur. |
| F14 | Dossier de données isolé ⇒ identifiants stockés par opencode invisibles (« AWS default credential chain failed »). `AWS_BEARER_TOKEN_BEDROCK` en env ✅. `opencode auth import` **sans** `--standalone` va au service partagé (piège) ; **avec** `--standalone` ✅. | ✅ | O4. |
| F15 | Bedrock : **région obligatoire** dans la config fournisseur (`provider.amazon-bedrock.options.region = eu-west-1`) sinon identifiant de modèle invalide / refus politique « non-EU ». | ✅ | Région dans `SessionSpec`. |
| F16 | **Proxy d'identifiants** : `baseURL` du fournisseur → proxy local ; opencode n'a qu'un faux jeton ; proxy remplace l'`Authorization` ; streaming OK (48 morceaux) ; `env` dans le shell de l'agent ne montre que le faux jeton ; sans jeton de session → 401. | ✅ | O4. |
| F17 | **Pilotage headless complet** : `permission.asked` visible via `GET /api/session/{id}/permission` et SSE ; réponse `POST …/permission/{rid}/reply {"decision":"once","message":"…"}` → l'agent continue. Outil `question` → **formulaire typé** (`GET …/form`), réponse `POST …/form/{fid}/reply {"answer":{"q0":"Blue"}}`. `wait`, `diff`, coût/tokens par session OK. | ✅ | S3, S4. |
| F18 | Événements SSE observés : `session.created`, `session.execution.started/succeeded`, `session.step.*`, `session.text.*`, `session.tool.input.started/ended`, `session.tool.called/progress/success`, `session.usage.updated`, `session.inbox.enqueued/delivered`, `session.instructions.updated`, `session.renamed`, `permission.asked/replied`, `form.created/replied`, `shell.created/exited`, `vcs.branch.updated`, `worktree.resolved`, `provider/model/integration/skill/websearch.updated`. **Pas de rejeu** des événements manqués. | ✅ | S2 + resynchronisation. |
| F19 | **Un serveur, plusieurs dossiers** (`location`) : 2 sessions dans 2 dépôts différents sur le même serveur. | ✅ | Groupe de serveur (I5). |
| F20 | **Deux clients sur une même session** : TUI attachée + oh via API → la TUI affiche le prompt envoyé par l'API, affiche la demande de permission (Allow/Reject), et **continue quand oh répond par l'API** (première réponse gagne). | ✅ | I1, I3. |
| F21 | Permissions **par session** sur un serveur partagé : **masquent les skills** ✅ ; **ne masquent pas les sous-agents** ❌ (appel bloqué mais agent listé). Seules les règles **au niveau de l'agent** masquent un sous-agent. | ⚠ | Serveur global multi-workflows rejeté (T-c) ; groupe = même version de workflow. |
| F22 | Coût mémoire d'un serveur : **~340 Mo** au repos, **~440 Mo** après une session ; prêt en **1,4 s**. | ✅ | I5, I6, I7. |
| F23 | `pair` : `POST /api/pair` → code à usage unique (5 min) ; interface web servie par le même serveur. | ✅ | S11. |
| F24 | Plugin local : chemin vers **dossier** avec `package.json` obligatoire (fichier refusé : « configured plugin path must be a directory ») ; objet exporté simple `{ id, setup(ctx) }` suffit, sans npm. | ✅ | O6. |
| F25 | En CI, `--auto` approuve automatiquement **tous** les `ask` (doc). | 📄 | Répondeur de politique (S5). |
| F26 | `POST /api/session` accepte `permissions` (règles de session) ; plugin `ctx.permission.rules({sessionID, permissions})` les remplace en cours de route. | ✅/📄 | Gating dynamique des checkpoints. |
| F28 | `GET /api/skill` liste **toutes** les skills découvertes (y compris `opencode`, `report`) **sans tenir compte des permissions** ; les listes vides observées plus tôt étaient dues au chargement asynchrone (skills chargées après les agents). La visibilité pour le modèle est garantie par les règles `skill` (global + agent). | ✅ | Attest : skill hors paquet acceptée seulement si toutes les règles rendues la refusent ; attendre que les skills du paquet soient listées. |
| F29 | Config V2 des fournisseurs : `providers.<id>.settings.{region, baseURL, profile}` ; Bedrock lit `AWS_BEARER_TOKEN_BEDROCK` (clé API) ou la chaîne AWS (SigV4). MCP V2 : `mcp.servers.<name>` avec `codemode: false` pour exposer les outils sans Code Mode. | 📄 | Rendu V2 (E3). |
| F27 | Doc plugins V2 : `permission.hook("evaluate")` (modifie allow/ask/deny + message), `agent.transform` (retirer/ajouter des agents), `tool.hook("execute.before/after")`, `session.hook("context"|"prompt"|…)`, `skill.transform`, `mcp.transform`, `ctx.storage`, RPC plugin (`POST /api/rpc/{rpcID}/{method}`). Export dual V1/V2 possible. | 📄 | O2, O6, D8, D13. |
| F30 | Avec un environnement de session posé (`PUT …/environment`), le **shell de la session n'hérite pas des variables du processus serveur** : il ne voit que ces variables plus une base minimale (8 variables observées en conteneur). | ✅ (piste H) | Toute variable utile au shell de l'agent (`OH_GATEWAY_URL`) passe par l'environnement de session. |
| F31 | L'environnement de session **n'est pas transmis aux sous-sessions** créées par l'outil `task` (sous-agents). | ✅ (e2e `TestE2ESubagentSessionEnvironment`) | Le démon applique l'environnement de la racine à chaque sous-session dès qu'il la voit (`adoptChild`). |
| F32 | MCP `type: remote` : transport « streamable HTTP » (POST JSON-RPC, `Accept: application/json, text/event-stream`, réponses JSON acceptées, GET de flux toléré en 405). `{env:VAR}` est développé dans les `headers`. Le `_meta` de `tools/call` porte la session appelante, comme en local. | ✅ (piste H, e2e `TestE2ERemoteMCPReceivesSessionMeta`) | Passerelle MCP : MCP oh distants sans jeton écrit dans la configuration. |

## Recettes reproductibles

### R1 — Serveur dédié avec paquet en env (monde fermé)
```bash
export OPENCODE_SERVER_PASSWORD=<aléatoire>
export XDG_DATA_HOME=<dossier données du groupe>
export OPENCODE_DISABLE_PROJECT_CONFIG=1
export OPENCODE_CONFIG_CONTENT='{
  "provider":{"amazon-bedrock":{"options":{"region":"eu-west-1","baseURL":"http://127.0.0.1:<proxy>"}}},
  "model":"amazon-bedrock/eu.anthropic.claude-haiku-4-5-20251001-v1:0",
  "skills":["<bundle>/skills"],
  "plugins":["<bundle>/plugin"],
  "experimental":{"subagent_depth":3},
  "permissions":[{"action":"skill","resource":"*","effect":"deny"},{"action":"skill","resource":"<id>","effect":"allow"}],
  "agents":{"build":{"disabled":true},"plan":{"disabled":true},"general":{"disabled":true},"explore":{"disabled":true}, "...":{}}
}'
export AWS_BEARER_TOKEN_BEDROCK=<jeton de session du proxy>
opencode serve --hostname 127.0.0.1 --port <port>
# prêt quand GET /api/agent?location[directory]=<dir> renvoie nos agents
```

### R2 — Vérification d'isolation (Attest)
```bash
curl -s -u opencode:$PW "http://127.0.0.1:$PORT/api/agent?location%5Bdirectory%5D=$DIR"   # ⊆ paquet ∪ {compaction,title,summary}
curl -s -u opencode:$PW "http://127.0.0.1:$PORT/api/skill?location%5Bdirectory%5D=$DIR"   # ⊆ paquet
curl -s -u opencode:$PW "http://127.0.0.1:$PORT/api/mcp?location%5Bdirectory%5D=$DIR"     # ⊆ paquet
```
Attention : listes vides pendant ~1–3 s après démarrage (chargement asynchrone) → réessayer.
Agents système à tolérer : `compaction`, `title`, `summary` (champ `hidden: true`).

### R3 — Créer, lancer, attacher
```bash
curl -u opencode:$PW -X POST $U/api/session -H 'content-type: application/json' \
  -d '{"id":"ses_<oh-id>","agent":"<entry>","title":"<wf> · <input>","location":{"directory":"<worktree>"},"permissions":[...]}'
curl -u opencode:$PW -X POST $U/api/session/ses_<oh-id>/prompt -H 'content-type: application/json' -d '{"text":"<prompt rendu>"}'
OPENCODE_SERVER_PASSWORD=$PW opencode --server $U -s ses_<oh-id>     # interface interactive
```
Contrainte : identifiant de session `^ses` (on a utilisé `ses_` + 26 caractères).

### R4 — Décisions headless
```bash
curl -u opencode:$PW $U/api/session/$S/permission                    # demandes en attente
curl -u opencode:$PW -X POST $U/api/session/$S/permission/$RID/reply -d '{"decision":"once","message":"Validé par oh (cp-2)"}'
# decision ∈ once | always | reject   (Permission.Reply)
curl -u opencode:$PW $U/api/session/$S/form                          # formulaires (questions)
curl -u opencode:$PW -X POST $U/api/session/$S/form/$FID/reply -d '{"answer":{"q0":"Blue"}}'
# Form.Answer = map clé → (string | number | boolean | string[])
curl -u opencode:$PW -X POST $U/api/experimental/session/$S/wait -d '{}'   # attendre l'inactivité
curl -u opencode:$PW $U/api/session/$S/diff
curl -u opencode:$PW $U/api/session/$S                                    # cost, tokens
curl -N -u opencode:$PW $U/api/event                                      # flux SSE
```

### R5 — Plugin de sonde (mesure du prompt système)
Dossier `plugin/` avec `package.json` `{"name":"probe","type":"module","exports":{".":"./index.ts"}}` et :
```ts
import { appendFileSync } from "fs"
export default { id: "probe", async setup(ctx: any) {
  await ctx.session.hook("context", (event: any) => {
    const sys = (event.system ?? []).map((s: any) => typeof s === "string" ? s : (s.text ?? JSON.stringify(s)))
    appendFileSync(process.env.PROBE_LOG!, JSON.stringify({ agent: event.agent?.id ?? event.agent, lens: sys.map((x: string) => x.length), tools: Object.keys(event.tools ?? {}) }) + "\n")
  })
}}
```

### R6 — Proxy d'identifiants (prototype validé)
- Config fournisseur : `baseURL: http://127.0.0.1:<proxy>` ; opencode reçoit `AWS_BEARER_TOKEN_BEDROCK=<jeton-session>`.
- Proxy : vérifie `Authorization: Bearer <jeton-session>`, remplace par la vraie clé, relaie vers `bedrock-runtime.<region>.amazonaws.com`, **streaming chunké sans tampon** (`/model/<id>/converse-stream`).
- Implémentation cible Go : `httputil.ReverseProxy` avec `FlushInterval: -1`, `Rewrite` qui remplace l'en-tête ; SigV4 via `aws-sdk-go-v2/aws/signer/v4` quand la source est un profil AWS.

### R7 — Profondeur de délégation
`"experimental": {"subagent_depth": N}` où N = longueur du plus long chemin d'appels `subagent` dans le graphe du workflow.

## API V2 — opérations utiles (extrait OpenAPI `GET /openapi.json`)

| Domaine | Opérations |
|---|---|
| server | `GET /api/info`, `POST /api/pair`, `GET /auth/connect/{code}` |
| agent / skill / mcp / model / provider | `GET /api/agent`, `GET /api/skill`, `GET /api/mcp`, `PUT/DELETE /api/experimental/mcp/{server}`, `GET /api/model`, `GET /api/provider` |
| session | `GET/POST /api/session`, `GET/PATCH/DELETE /api/session/{id}`, `POST …/prompt`, `…/synthetic`, `…/agent` (switch), `…/model` (switch), `…/interrupt`, `…/compact`, `…/fork`, `…/move`, `…/background`, `PUT …/environment`, `GET …/diff`, `GET …/message`, `GET …/context`, `GET …/inbox` |
| expérimental session | `POST /api/experimental/session/{id}/wait`, `GET /api/experimental/session/stats`, `GET …/{id}/export`, `POST /api/experimental/session/import`, `PUT/DELETE …/{id}/instructions/entries/{key}`, `GET …/{id}/log`, `POST …/{id}/skill` |
| permission | `GET /api/permission/request`, `GET /api/session/{id}/permission[/{rid}]`, `POST …/{rid}/reply`, `GET/DELETE /api/permission/saved` |
| form | `GET /api/form`, `GET/POST /api/session/{id}/form`, `POST …/{fid}/reply`, `DELETE …/{fid}` |
| vcs / worktree / fs | `GET /api/vcs`, `…/status`, `…/diff`, `…/branch` ; `GET/POST/DELETE /api/worktree` ; `GET /api/fs/read/*`, `…/list`, `…/find` |
| event | `GET /api/event` (SSE) |
| rpc | `POST /api/rpc/{rpcID}/{method}` (RPC des plugins) |
| pty | `/api/pty*`, `/api/experimental/persistent-pty*` |

Schémas utiles : `POST /api/session` body = `{id?, title?, agent?, model?: Model.Ref, location?: {directory}, metadata?, permissions?: Permission.Ruleset}` ; `prompt` body = `{text, files?, agents?, skills?, metadata?, delivery?: steer|queue, resume?}` ; `environment` body = `{variables: {K: V}}` ; `synthetic` body = `{text, description?, metadata?, delivery?, resume?}`.

## Points encore non vérifiés (à couvrir par tests de contrat)

- Comportement exact de `default_agent` vs agent de session lors d'une reprise.
- `session.hook("context")` : ajout d'un bloc système **et** absence de `system` sur l'agent → vérifier que le corps de l'agent est bien présent à chaque appel (y compris sous-agents).
- `permission.hook("evaluate")` déclenché pour les outils MCP (`<server>_<tool>`).
- Masquage des sous-agents via `agent.transform` dans le plugin.
- Code Mode désactivé via permission `execute: deny` → outils MCP exposés directement ?
- Compatibilité du plugin `rtk` (V1, `@opencode-ai/plugin`) et `context-mode` sous V2.
- Proxy : autres fournisseurs (Anthropic `x-api-key`, OpenRouter) ; Bedrock via SigV4.
- Mémoire réelle d'un serveur avec 5 sessions actives.

## Ajout piste E (06/10/2026) — plugins npm
- `plugins` accepte une spécification npm (`"pkg@tag"`) ou `{ "package": "<spec>", "options": {…} }` ; installation automatique dans `~/.cache/opencode/npm/<spec>/` au démarrage ; log `loading plugin id=<spec>`.
- Un plugin au format V1 (`server()`) échoue (`Plugin must export a default definition with an id and an effect or setup function`) sans bloquer le serveur, et n'apparaît pas dans `GET /api/plugin`. Cas de `context-mode@latest`.
- `tool/task` (source) : pas de contrôle du mode de l'agent cible à l'exécution (un agent `primary` nommé explicitement est lancé) ; profondeur bornée par `subagent_depth`.

## Étude S8 (P1-T29) — entrées d'instructions par session (07/10/2026, opencode 2.0.20, Haiku EU)

Essai e2e réel (test temporaire sur le serveur de contrat : bundle `lead` + sous-agent `helper`, plugin oh actif), non conservé dans le dépôt ; **aucune mise en œuvre** (décision à prendre).

**API** (`GET /openapi.json`) : `GET …/session/{id}/instructions/entries` (liste `{key, value}`), `PUT …/entries/{key}` corps `{"value": <JSON quelconque>}` → 204, `DELETE …/entries/{key}` → 204. Clé `^[a-z0-9][a-z0-9._-]*$` (400 sinon) ; valeur ≤ **256 Kio** (413 `InstructionEntryValueTooLargeError` au-delà ; 128 Kio accepté). « Changes announce as updates at the next step boundary ».

**Constats**
- **Entrée posée avant le 1er tour** : prise en compte (consigne « termine par S8END » suivie). Elle n'apparaît **ni dans l'historique** (`/message`) **ni comme bloc séparé** du prompt système vu par `session.hook("context")` (5 blocs inchangés, dont le corps d'agent injecté par le plugin en [1]) : opencode l'ajoute après les hooks.
- **Modification ou suppression en cours de session** : annoncée au tour suivant par un **message `system` synthétique persistant dans l'historique** (`metadata.notice = "instructions"`, `instructionSources: ["api/<clé>"]`) : « The context under "<clé>" changed and supersedes the previous value: <context key="<clé>">…</context> » (objet JSON joliment indenté), puis « … no longer applies. Disregard it. ». La consigne remplacée est bien suivie (S8OBJ) ; après suppression, elle cesse (pas de S8OBJ au tour 4).
- **Sous-agents** : l'entrée **n'est pas transmise** aux sessions enfants (`helper` : aucune annonce, consigne non connue ; le texte de l'enfant ne la suit pas — la fin « S8OBJ » vue dans la réponse est celle du parent). Même limite que l'environnement de session (F-constats 4.C).
- **Coût** : chaque changement ajoute un message à l'historique (cache de prompt invalidé à partir de là), alors que le corps d'agent du plugin reste dans le préfixe système stable.

**Comparaison avec O2 (plugin `session.hook("context")`)**
| | Plugin O2 (actuel) | Entrées S8 |
|---|---|---|
| Portée | par **agent** (corps de chaque agent, sous-agents compris) | par **session** racine seulement |
| Mise à jour en cours de session | non (fichier du paquet) | oui, annoncée au tour suivant |
| Trace dans l'historique | aucune | message système à chaque changement |
| Stabilité du cache | préfixe système stable | invalidée à chaque changement |
| Dépendance | plugin oh chargé (`waitPluginActive`) | API **expérimentale** (peut changer) |

**Recommandation** : garder O2 pour les corps d'agent et la carte du workflow (portée agent + sous-agents, cache stable). S8 conviendrait à un **état de session qui évolue** et que seul l'agent d'entrée doit connaître (ex. checkpoints passés, budget restant, consigne de reprise), en remplacement des consignes `steer` envoyées aujourd'hui par `SessionService.Send` — à décider (BL), avec repli si l'API expérimentale disparaît (`Capabilities`).

**Décision (07/10/2026)** : **option 2** — S8 utilisé pour l'**état de session évolutif** connu de l'agent d'entrée (checkpoints passés et courant, budget restant, consigne de reprise), **derrière une capacité neutre de l'adaptateur** (D19), avec repli sur `SessionService.Send` (`steer`/`synthetic`) si la capacité est absente ou si l'API expérimentale répond en erreur. Corps d'agent et carte du workflow : inchangés (plugin, O2). Mise en œuvre : `14-finalisation.md` § QB8 (fait le 07/10/2026 : `adapters.SessionContextSetter`, `internal/sessionctx`, ADR-050 ; la liste `GET …/entries` renvoie `{"data": [{key, value}]}`).

## Sondes U1–U5 (lot 0 des points critiques, 09/10/2026, opencode 2.0.20, Haiku EU)

Source lue (`core/src/shell/parse.ts`, `permission.ts`, `util/wildcard.ts`, `shell.ts`) + e2e `TestE2EShellMatcherSemantics` (fichiers marqueurs). Détail et conséquences : `17-ecarts-S.md` § Lot 0.
- **U1** : le préfixe `X=y` fait partie du texte comparé (texte brut du nœud `command` de tree-sitter-bash ; une redirection aussi).
- **U2** : guillemets et espaces **non normalisés** (`git pu''sh`, `git  push` échappent à `git push*`).
- **U3** : le shell voit `OPENCODE_SERVER_PASSWORD` et `OPENCODE_CONFIG_CONTENT` **quand la session n'a pas d'environnement** (`sessionEnvironment ?? process.env`) : sous-session non encore adoptée, session ouverte dans l'interface, reprise avant réapplication. Avec un environnement de session : invisibles.
- **U4** : `external_directory` ne porte que sur le dossier de travail et les arguments de `cd`/`pushd` ; redirections et arguments écrivent hors du projet sans demande.
- **U5** : une ligne est refusée si une commande l'est ; règles = agent puis session, la dernière qui correspond gagne (un `allow` de session ouvre un refus de l'agent).
- En plus : `export`/`declare`/`local`/`readonly`/`typeset`/`unset` ne sont pas vérifiés ; `sh -c '…'` n'est pas analysé ; un motif finissant par ` *` correspond aussi à la commande nue ; `permission.replied` = `{sessionID, requestID, reply: once|always|reject}`, un `reject` rejette toutes les demandes en attente de la session.
