# Inventaire de l'état final de oh v5.0.0 (lot V2-C)

> Source de vérité pour V3-A (CHANGELOG `[5.0.0]`) et V3-B (`MIGRATION.md`, `docs/guides/migration-v5.{fr,en}.md`).
> Rédigé le 22/10/2026, **en lecture seule** sur `/Users/benjamin.datiche/workspace/openhub`, base `main` = `7715be6f` (après fusion de V1-A, V1-B, V1-C, V2-B et du correctif D19). V2-A (jokers communs, confirmation unique) n'est **pas** fusionné : la branche `feat/v50-confirm` n'a, au moment de la lecture, qu'un commit (`a9cf7b19`, jokers).
>
> **Méthode.** Chaque fait est lu dans le code ou les docs de `main`, avec `fichier:ligne` (chemins relatifs à la racine du dépôt ; `CHANGELOG:NN` = ligne de `CHANGELOG.md`). Pour les commandes et options, j'ai compilé `oh` (HEAD) dans un dossier temporaire, avec `HOME` et `OH_HOME` vides, et dumpé l'arbre cobra par un petit programme hors dépôt (`RootCmd()`). Pour savoir ce qui concerne un utilisateur de la **4.2.0**, j'ai extrait le tag `v4.2.0` (`git archive`), compilé et dumpé de la même façon, puis comparé (commandes, options, clés de configuration, codes de sortie). Rien n'a été écrit dans le dépôt (`git status` : seul `docs/dev/rapport-modernisation-2026-10.html`, déjà non suivi avant le lot).
>
> **Point d'attention pour V3-A.** Plusieurs « ruptures » du `[Unreleased]` n'existent que **par rapport à un build de développement de la v5** : ni `oh run`, ni les workflows, ni `oh remote`, ni le fan-out, ni `oh session` n'existaient en 4.2.0 (voir § 7, colonne « 4.2.0 »). Le CHANGELOG `[5.0.0]` doit parler à un utilisateur de la 4.2.0, pas à un testeur de builds intermédiaires.

---

## 1. Chiffres

| Quoi | Valeur | Preuve |
|---|---|---|
| Agents | **18** (15 `mode: primary`, 3 `mode: subagent` : `developer`, `auditor-subagent`, `brief-enricher`), **9 familles** | `agents/**/*.md` (18 fichiers : auditor 2, delivery 2, design 1, developer 1, documentation 1, infra 1, planning 6, quality 3, utility 1) ; `agents/developer/developer.md:5`, `agents/auditor/auditor-subagent.md:5`, `agents/utility/brief-enricher.md:6` ; docs : `README.md:160`, `docs/architecture/agents.fr.md:5` |
| Agents en 4.2.0 | 19 (`git ls-tree v4.2.0 agents`) : 5 retirés (`database`, `developer-migrator`, `developer-refactor`, `infra`, `test-generator`), 4 ajoutés (`conductor`, `merger`, `operator`, `release-manager`) | comparaison des deux listes |
| Workflows livrés | **24** fichiers `workflows/*.yaml` (+ 24 gabarits `workflows/prompts/*.md.tmpl`), tous `apiVersion: oh/v1`, tous valides (`oh workflow validate --all`) | `ls workflows/*.yaml` ; `README.md:125`, `README.fr.md:125` (« 24 ») |
| Workflows en 4.2.0 | 0 (aucun `workflows/` dans le tag) | `git ls-tree v4.2.0` |
| Alias de workflows | 6 workflows portent des alias, **8 déclarations, 7 identifiants distincts** : `perf` → `benchmark` (`workflows/benchmark.yaml:11`) ; `dbg` → `debug` (`debug.yaml:11`) ; `feature` avec `tickets` → `implement-spec` (`implement-spec.yaml:12`) ; `feedback`, `retours` → `review-feedback` (`review-feedback.yaml:11`) ; `secu` → `security-fix` (`security-fix.yaml:11`) ; `cadrage`, `feature` → `spec` (`spec.yaml:13`) | grep `^aliases:` ; lecture dans `cli/internal/workflow/next.go:64-87`, schéma `workflow/schema.go:181-182` |
| Skills | **180** skills (fichiers avec `name:`) + **11 annexes** dans `skills/templates/` = 191 fichiers `.md` ; dont **41** skills de stack (`skills/developer/stacks/`) et 139 de domaine. `oh skill check --hub .` : « no problem ». En 4.2.0 : 183 skills + 14 annexes = 197 fichiers (`git ls-tree v4.2.0 skills`) | `find skills -name '*.md'` ; docs **en retard** : `docs/architecture/skills.fr.md:10,140` et `.en.md:10,140` disent 178 + 11 = 189 |
| Cas d'évaluation | 71 dossiers de cas (`evals/<workflow>/<cas>/case.yaml`) ; mesure de référence : 35 cas, Haiku ×3, 105 passages, 18,63 $ | `evals/`, `cli/internal/evalrun/testdata/evals/baseline.json` |
| Schéma de workflow | `oh/v1` | `cli/internal/workflow/schema.go:33` (`APIVersionV1`) |
| `PipelineSchema` (mode distant) | **5** (2 : `OH_TOOL_VERSION` ; 3 : secrets limités au job, contexte d'image ; 4 : image épinglée par empreinte, paire de clés du journal ; 5 : `reports.json`) | `cli/internal/remote/contract.go:15` (et non `services/remote`) |
| Schéma de la base | **49** (dernière migration `version: 49` : colonne `wiki_proposals.reason`) ; la **47 n'existe pas** (réservée par un autre lot) ; en 4.2.0 : **27** → 21 migrations (28 à 46, 48, 49) | `cli/internal/storage/sqlite/store.go:717-721` (49), `:707-713` (48, « v47 is taken »), `:154-156` (`sequentialMigrations = 31`) ; tag : `git show v4.2.0:cli/internal/storage/sqlite/store.go` |
| Sauvegarde avant migration | `~/.oh/backups/oh.db.v<version>-<date>.bak`, 3 gardées ; `OH_SKIP_DB_BACKUP=1` la saute (alors pas de migration) ; base écrite par un oh plus récent refusée (`OH_ALLOW_NEWER_SCHEMA=1` pour passer outre) | `storage/sqlite/migrate.go:55,217,383` ; `store.go:32,48` ; `config/backup.go:13-32` (`hub.toml.bak-<date>`, 3 gardées) |
| Paquets Windows exclus de la CI | **27** exclus, 83 testés | `.github/windows-test-packages-excluded.txt`, `.github/windows-test-packages.txt` |
| Sous-commandes | 53 commandes de premier niveau ; 140 sous-commandes non masquées, tous niveaux (dont 5 dépréciées) (§ 2) | dump cobra |
| Entrées « Unreleased » | lignes 10 à 1189 de `CHANGELOG.md` ; **121 sections `###`** (43 Added, 39 Changed, 24 Fixed, 12 Security, 2 Removed, 1 Documentation) ; 693 puces ; 48 lignes avec ⚠ et 3 avec « Rupture » | `awk` sur `CHANGELOG.md` |

Autres chiffres utiles : `docs/reference/tui.fr.md` = 361 lignes ; `keys.Changes()` = **23** changements de touches (`cli/internal/tui/v2/keys/keys.go:354-380`) ; `keys.All()` = 23 actions de la convention (`keys.go:201-208`).

**Docs à corriger sur les chiffres** (à reporter à V3-A/V3-B ou à un lot docs) : `README.md:13` et `README.fr.md:13` disent « 22 » workflows alors que `README.md:125` dit 24 ; `docs/guides/getting-started.{fr,en}.md:21` : 22 ; `docs/architecture/skills.{fr,en}.md:10,140` : 178/189 (réel : 180/191).

---

## 2. Commandes, options, codes de sortie

### 2.1 Commandes de premier niveau (dump cobra de `main`)

53 commandes : **43 visibles**, **5 masquées**, **5 dépréciées**.

- **Visibles (43)** : `beads board brick budget bundle completion config conventions daemon dashboard doctor eval export fanout gc history import init mcp metrics migrate mr patterns policies project provider purge remote repair run secrets serve session skill status takeover-brief team teams upgrade version wiki workflow worktree`. Alias de commande : `project` = `p` (`oh project`), `takeover-brief` = `tb`, `worktree` = `wt` ; sous-commandes `ls`/`register`/`rm` (`config list`, `mcp list`, `project add|list|remove`, `worktree list|remove`).
- **Masquées (5)** : `deploy` et `sync` (`cli/cmd/deploy_removed.go:23`, sortent en code 2 avec un message de migration, `:27`), `optimize` et `yield` (`cmd/optimize_yield.go:15,65`), `runner` (côté job distant, `cmd/runner.go:48`). Sous-commandes masquées : `daemon run` (`cmd/daemon.go:30`), `eval pass` (`cmd/eval.go:80`).
- **Dépréciées (5, champ `Deprecated` cobra, absentes de `oh --help`, toujours fonctionnelles, avertissement traduit)** :
  - `oh audit` (`cmd/audit_review_debug.go:105`), `oh review` (`:124`), `oh debug` (`:146`), `oh start` (`cmd/start.go:18`) ;
  - `oh service` (`cmd/misc.go:59`) et ses sous-commandes `setup` (`:70`) et `remove` (`:78`) : **dépréciées depuis la 4.x**, avec un texte français en dur (« Utilisez 'oh mcp status' à la place ») qui ne passe pas par i18n ; **non citées par le CHANGELOG** (`CHANGELOG:86`).
  - Sous-commandes dépréciées : `oh review feedback` (`cmd/review_feedback.go:23`), `oh takeover-brief enrich` (`cmd/takeover_brief.go:49`).
- **Options dépréciées de `oh start`** : `--max-sessions`, `--priority`, `--sweep-branch-prefix` (sans effet, `cmd/start.go:67`), `--yes/-y` (`:43,63`).
- **Disparues depuis la 4.2.0** (comparaison des deux arbres) : `oh plugin` (+ `install|list|remove|status`), `oh skill add|list|remove|search|budget`, `oh upgrade opencode`. Elles répondent désormais « unknown command », code 2.
- **Ajoutées depuis la 4.2.0** (premier niveau) : `brick budget bundle daemon eval fanout gc migrate mr remote run session wiki workflow` (+ `runner` masquée). Sous-commandes ou options nouvelles de commandes existantes : `oh mcp trust|untrust`, `oh team init --solo/--id/--member-id/--name/--project`, `oh team promote --remote/--team`, `oh doctor --fix/--offline/--yes`, `oh repair --migrate/--restore`, `oh config list --all/--keys`, `oh config model show --workflow`, `oh provider setup --from-env`, `oh skill check --hub/--json/--max-age/--stale/--strict`, `oh project add --yes`, `oh service remove --project`, `oh purge --include-tool-data`.
- Options globales : `--log-format`, `--no-tui`, `-v/--verbose` (`dump`, doc `docs/reference/cli.fr.md:11-17`).

### 2.2 Options courtes : une lettre, un sens

Source : dump des options + `docs/reference/cli.fr.md:53-` (tableau « Options courtes »).

| Lettre | Sens (commandes qui la portent) | Ancienne forme encore acceptée (cachée, avertissement, « pendant une version ») |
|---|---|---|
| `-a` | `--agent` (`oh run`, `oh start`) | |
| `-A` | `--assignee` (`oh start`) | |
| `-b` | `--base` (`oh worktree cleanup`) | `oh review -b` (= `--branch`) — `audit_review_debug.go:387` |
| `-d` | `--path` (`oh project add`) | `oh metrics -d` (= `--period`) — `cmd/metrics.go:25` |
| `-f` | `--force` (`project remove`, `teams remove`, `worktree remove\|cleanup`, `service remove`) | |
| `-i` | `--input` (`oh run`) | `oh debug -i` (= `--issue`) — `audit_review_debug.go:397` |
| `-l` | `--language` (`project add\|configure`) | `oh start -l` (= `--label`) — `cmd/start.go:61` |
| `-m` | `--message` (texte : `oh run` = raccourci de `-i request=…`, `brick\|workflow publish`, `workflow archive`, `session approve`, `team notify test`) ; `--prompt` pour `oh start` | `oh review -m` (= `--mode`, `audit_review_debug.go:385`), `oh project configure -m` (= `--model`, `cmd/project_configure.go:225`) |
| `-n` | `--name` (`project add`) | |
| `-o` | `--output` (`export`, `session trace`) | |
| `-p` | `--project` (toutes les commandes qui visent un projet, dont `config model …`) | `oh serve -p` (= `--port`) — `cmd/serve.go:39` |
| `-P` | `--provider` (`run`, `bundle build\|show`, `project configure`, `start`) | |
| `-r` | `--resume` (`start`) | |
| `-s` | `--status` (`project list`) | |
| `-t` | `--ticket` (`oh start`) | `oh audit -t` (= `--type`) — `audit_review_debug.go:378` |
| `-w` | `--workflow` (`config model show`) | `oh start -w` (= `--worktree`) — `cmd/start.go:60` |
| `-y` | `--yes` (`brick publish`, `doctor`, `gc`, `mcp trust`, `migrate deploy-cleanup`, `project add`, `session delete\|resolve`, `workflow archive\|publish\|restore`) | |
| `-v` | `--verbose` (global) | |

Mécanisme : `cmd/deprecated_options.go:20-72` (`deprecateShorthand`, `deprecateFlag`). Autre ancienne forme : `oh session attach --how` → `--attach` (alias masqué, `cmd/session.go:271` ; la valeur `here` devient `suspend` avec avertissement, `:62-64`).

**Formes supprimées SANS alias** (rupture réelle, non signalée comme telle dans le CHANGELOG) :
- `oh config model … -j` (ancien raccourci de `--project`, remplacé par `-p` ; « unknown shorthand flag: 'j' », code 2) — commit `f494b5da`, `CHANGELOG:1102` ne dit pas « sans alias » ;
- `oh purge --include-opencode` (remplacé par `--include-tool-data`, `cmd/purge.go:36` ; « unknown flag », code 2, vérifié en exécutant `oh`) — `CHANGELOG:1106` dit « remplace » ; `docs/reference/cli-infra.fr.md:234` écrit « Ancien nom : `--include-opencode` » sans dire qu'il est refusé.

### 2.3 Codes de sortie

Source : `cli/cmd/exitcodes.go:12-17` ; doc `docs/reference/cli.fr.md:39-52` / `cli.en.md:39-52`.

| Code | Constante | Sens |
|---|---|---|
| 0 | `ExitOK` | Succès. `oh doctor` : aussi quand il n'y a que des avertissements (⚠) ; moteur de conteneurs, `bd`, passerelles = avertissements sauf `execution.runtime = container` |
| 1 | `ExitFailure` | Échec : `workflow validate` / `skill check` avec erreurs, `run --headless` dont une session échoue ou dépasse `--timeout`, `doctor` quand un contrôle obligatoire échoue, panic |
| 2 | `ExitUsage` | Mauvais usage : commande ou option inconnue (`cmd/root.go:284`), commande supprimée (`oh deploy`, `oh sync`, `deploy_removed.go:27`) |
| 3 | `ExitPending` | `oh run --headless` dont une session attend une décision sans qu'aucune n'ait échoué (sinon 1) |
| code de `bd` | | `oh beads` transmet le code de `bd` |

Vérifié en exécutant les deux binaires : en **4.2.0**, commande inconnue et option inconnue sortaient en **1** ; en v5 : **2**. `oh deploy --help` sortait en 0 en 4.2.0 ; `oh deploy` sort en 2.

**Pas encore dans la table** : le refus de confirmation sans terminal (code 2, V2-A) — voir § 7, R47.

---

## 3. TUI : touches et aide

### 3.1 Touches globales et aide

Source : `cli/internal/tui/v2/shell/shell.go:1360-1445` ; doc `docs/reference/tui.fr.md:34-50`.

| Touche | Action | Code |
|---|---|---|
| `Ctrl+P` | Palette de commandes (omnibar). Aussi `/` quand la vue ne filtre pas | `shell.go:1432` (et `:1412` dans les vues qui laissent passer `?` et `Ctrl+P`) |
| `?` ou `F1` | Aide (touches globales, convention, touches de la vue, légende, commandes) ; `F1` marche partout (formulaires, éditeur) ; `?` et `Ctrl+P` aussi dans les onglets d'une session et la validation par lot | `keys.go:224` (`Help`), `shell.go:1391,1438` |
| `Échap` | Ferme d'abord les toasts, puis revient (vue précédente) ; à la racine d'un mode Projet ou Équipe : retour Hub | `shell.go:1440` |
| `Ctrl+T` | Mode Équipe (choix si plusieurs équipes) / retour Hub | `shell.go:1435` |
| `Ctrl+Q` / `Ctrl+C` | Quitter (si des sessions travaillent : finir l'étape puis veille / arrière-plan / arrêter) | `shell.go:1362` |
| `Entrée` | Action de l'élément sélectionné ; palette seulement si rien ne l'utilise | doc `tui.fr.md:43` |

Ordre de traitement : fenêtres et palette, puis touches réservées, puis la vue, puis le widget ; « taper une lettre n'importe où n'ouvre plus la palette » (**en 4.2.0, si** : `git show v4.2.0:cli/internal/tui/v2/shell/shell.go`, l. 1339-1343).

Petits terminaux : mode compact sous 32 lignes ; sous 60×16, message « Terminal trop petit » et seul `Ctrl+Q` fonctionne ; `[cli] compact = auto | always | never` (`config.go:313`). Badge des sessions : **`● N ◐ M`** (`views/sessions_section.go:63-72`, `internal/glyph/glyph.go:57-60`), `⏸` reste le signe d'une décision de type checkpoint (`glyph.go:40,55`).

### 3.2 Convention des touches (`keys.All()`)

Source : `keys.go:1-20, 201-264` ; doc `tui.fr.md:52-79`.

| Contexte | Touches |
|---|---|
| Partout | `↑↓`/`jk`, `g`/`G` (ou `Début`/`Fin`), `/` filtrer, `r` rafraîchir, `?`/`F1` aide, `Entrée` ouvrir, `Échap` revenir |
| Liste | `n` créer, `e` éditer, `d` supprimer (**toujours confirmé**), `x` archiver (jamais destructif), `R` reprendre/restaurer, `c` comparer (diff) |
| Décision (checkpoint, proposition wiki, élément d'un lot) | `y` valider, `n` refuser (avec consigne, la consigne vaut confirmation), `e` modifier, `s` laisser en attente, `A` tout accepter |
| Lecteur (trace, rapports) | `/` rechercher, `n`/`N` suivant/précédent |

Lettres réservées aux actions propres des vues : jamais `g G / r ? d x R y n j k` (`keys.go:272`). `keys.Own` (`:286`) : `p` relancer le pilote, `B` lot, `C` confirmer, `T` découvrir le tracker / transférer, `L` libérer, `M` nettoyer les fusionnés, `Y` YAML brut…

### 3.3 `keys.Changes()` — les 23 changements (`keys.go:354-380`, doc `tui.fr.md:81-95`)

| Vue | Avant → maintenant | Vue en 4.2.0 ? |
|---|---|---|
| Sessions | `D`→`d` supprimer · `c`→`R` reprendre · `g`→`R` récupérer une session distante · **nouveau** `B` lot | non (la vue « parallèle » est remplacée) |
| Récap d'une session existante | `r`→`R` reprendre · `t`→`T` transférer (`k` remplacer, `d` supprimer et relancer, `p` en parallèle restent : `views/launch_form_existing.go:43-47`) | non (nouveau) |
| Propositions wiki d'équipe | `a`→`y` accepter · `x`→`n` refuser | **oui** (`wiki_view.go` : `a`, `x`) |
| Workflows | `D`→`c` comparer · `x`→`d` supprimer un brouillon (`x` n'archive que les publiés) | non |
| Historique d'un workflow | `r`→`R` restaurer | non |
| Éditeur de workflow | `x`→`d` retirer · `y`→`Y` YAML brut | non |
| Patterns | `a`→`n` créer · `v`→`y` valider | **oui** (`patterns_view.go` : `a`, `v`) |
| Projets | `a`→`n` créer · `n`→`e` renommer | **oui** (`projects_view.go` : `a`, `n`) |
| Board d'équipe | `f`→`/` filtrer · `x`→`L` libérer | **oui** (`teamboard_view.go` : `f`, `x`) |
| Worktrees | `a`→`n` créer · `x`→`M` nettoyer les fusionnés | **oui** |
| Détail d'équipe | `y`→`T` découvrir le tracker | **oui** |
| Autres listes (modèles, policies, secrets, équipes…) | `a`→`n` créer | **oui** |

Autres changements propres au contrat de touches (`CHANGELOG:855`) : `Entrée` et les lettres ne sont plus pris à la liste ; `d` ne ferme plus les toasts (`Échap` les ferme) ; assistants : `Échap` ouvre « Quitter l'assistant ? » ; fiche de lancement : `Ctrl+S` va au récap puis lance, retour `Ctrl+B`/`Alt+←`/`PgUp`.

Un message unique « Nouvelles touches » est montré au premier lancement après la mise à jour, puis accessible depuis l'aide (préférence `tui.keys_notice`, `views/keys_help.go:17`, `cmd/tui_keys_notice.go`).

Vue **Nettoyage** unique (`cleanup`, alias `nettoyage`, `gc`, `oh gc`, `deploy-cleanup`, `migrate`…, `cmd/tui_commands.go:57-59`) à deux sections (`Tab`).

---

## 4. Configuration

Fichier : `~/.oh/hub.toml` (`OH_HOME` le déplace). **Plus aucun `./hub.toml` n'est lu** : `config/config.go:541-544` (`AddConfigPath(HubDir())` seulement ; la ligne `AddConfigPath(".")` du tag 4.2.0 est supprimée, commit `620c7722`).

### 4.1 Clés ajoutées depuis la 4.2.0 (diff des tags `mapstructure`, `config/*.go` + `limits`)

| Section | Clés | Défaut | Code |
|---|---|---|---|
| `[llm]` | `default_provider` (**renommée**, voir 4.2) | — | `config.go:329` |
| `[cli]` | `compact` (`auto\|always\|never`) | `auto` | `config.go:313` |
| `[session]` | `attach`, `iterm_style`, `idle_sleep_minutes` (5), `notify` (`on\|off`) | | `config.go:123-134` |
| `[execution]` | `runtime`, `engine` (`auto\|colima\|podman\|docker`), `colima_profile`, `keep_images` (2), **`tool_version`** (renommée), **`strict_isolation`**, **`env_allow`**, `proxy_first_byte_timeout_minutes` (5) | | `config.go:67-96` |
| `[fanout]` | **`experimental`** (défaut `false`), `parallel` (1 à 3), `budget_usd` (10) | | `config/fanout.go:8-26` |
| `[remote]` | **`experimental`** (défaut `false`), `targets[]` (`name,url,group,runner_project,runner_project_id,token_key,trigger_key,tag,builder,arch,timeout,binaries`), `projects` | | `config/remote.go:14-50` |
| `[limits]` | `max_active_sessions`, `session_budget_usd`, `daily_budget_usd`, `memory_mb`, `models` (désactivées par défaut) | | `limits/limits.go:26-30` |
| `[gc]` | `enabled`, `every_hours` (24), `session_days` (30), `history_days` (180), `usage_days` (400), `bundle_grace_hours` (24), `log_max_mb` (20), `log_keep` (3) | | `config/gc.go:10-27` |
| `[beads]` | `zero_impact` (`ask\|auto\|off`, défaut `ask`) | `ask` | `config/beads.go:10` |
| `[mcp.<service>]` | `url`, **`trusted_url`** (approbation personnelle d'une URL imposée par l'équipe), serveurs `github` et `linear` | | `config.go:349-371` |
| `[[teams]]` | `solo` (espace solo) | | `config.go:245` |

Doc de référence : `docs/reference/config.fr.md:15-135` (structure TOML) et `:136-171` (sections v5).

### 4.2 Clés renommées et héritées

| Ancienne clé | Clé actuelle | État réel | Code |
|---|---|---|---|
| `[opencode] default_provider` | `[llm] default_provider` | Lue en repli si `[llm]` est vide ; `oh config set opencode.default_provider` est **accepté** (alias vers `llm.default_provider`) ; `hub.toml` est réécrit avec la clé neutre au prochain enregistrement | `config/legacy_keys.go:8-26`, `cmd/config.go:47` |
| `[execution] opencode_version` | `[execution] tool_version` | Lue en repli seulement ; `oh config set execution.opencode_version …` est **refusé** (« unknown config key », vérifié) — **contredit `CHANGELOG:1105`** (« `oh config set` les accepte ») | `legacy_keys.go:10,22-24`, `cmd/config_fields_v5.go` (seule `execution.tool_version` y est) |
| Clés du trousseau MCP `<service>-token` et `<service>-token-<id-projet>` | `openhub.mcp.<service>.token[.<id>]` | Lues en repli et copiées sous le nouveau nom au premier accès ; anciennes clés non supprimées ; `StoreMCPToken` n'écrit que le nouveau nom | `config/mcpkeys.go:12-35,59-80,92-110` |
| `[opencode] version`, `channel`, `auto_update`, `install_dir` | **supprimées** | Section `[opencode]` absente de `Config` : ignorée à la lecture et **perdue au prochain enregistrement** (vérifié en réécrivant un `hub.toml` d'essai) | `config.go:26-63` ; 4.2.0 : `config.go:30,208,397-399` |
| `[deploy] disable_native_agents` | **supprimée** (le monde fermé masque toujours les agents natifs) ; `[deploy] instruction_files` reste | idem ci-dessus | `config.go:319-323` ; `docs/reference/config.fr.md:134` |
| `[workflow]` / `[workflow.overrides]` (hub), `[workflow]` du `config.toml` d'équipe | migrées vers des workflows d'équipe/projet (migration v38), originaux archivés dans `~/.oh/migrated/` | | `CHANGELOG:967,1063` ; guide §9 |
| `[parallel]` (équipe) | sans effet en v5 (non modifiable dans la TUI) | | `CHANGELOG:1093` ; `docs/reference/config.fr.md:607` |

### 4.3 Clés et variables de sécurité

| Clé / variable | Rôle | Code / doc |
|---|---|---|
| `execution.env_allow` (liste, noms ou `PREFIXE_*`) | Variables de l'utilisateur transmises au serveur opencode **local** et aux shells des agents, **en plus d'une liste de base** (`PATH, HOME, USER, SHELL, TERM, LANG/LC_*, TZ, TMPDIR, proxys, certificats, XDG_*`, emplacements de chaînes d'outils, `OH_*` utiles). **Ne passent plus** : `SSH_AUTH_SOCK`, `GH_TOKEN`, `GEMINI_API_KEY`, `NPM_TOKEN`… (en 4.2.0, l'environnement entier était transmis : `internal/opencode/opencode.go:161`). Refusés quoi qu'on mette : identifiants gérés par oh (`AWS_*`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GITLAB_TOKEN`, `GITHUB_TOKEN`, `OH_PASSPHRASE`…). Conteneurs et jobs distants n'héritent jamais | `config.go:94` ; `cli/internal/envfilter/envfilter.go:30-60,66-82` ; `cmd/doctor_env.go:16-38` ; `docs/reference/config.fr.md:149,173-205` ; `oh doctor` liste les variables retenues |
| `execution.strict_isolation` (bool) | Masque aussi la config personnelle d'opencode des serveurs locaux ; `Attest` refuse un plugin hors paquet (sinon avertissement, S-11) ; refuse un paquet Linux d'opencode **sans empreinte épinglée** (sinon avertissement) | `config.go:85` ; `docs/reference/config.fr.md:148` |
| `execution.tool_version` | Version d'opencode des images de conteneur ; si elle diffère du client de la machine, lancement en conteneur refusé | `config.go:82` |
| `execution.proxy_first_byte_timeout_minutes` | Délai avant 504 du proxy d'identifiants (5 min par défaut) ; un flux commencé n'a pas de limite | `config.go:89`, `ProxyFirstByteTimeout()` |
| `[llm] default_provider` | `bedrock \| anthropic \| openrouter \| github-copilot` | `config.go:329` |
| `provider.bedrock.auth_mode` | `bearer` (clé API enregistrée obligatoire) / `profile` / `env` (SigV4, ignorent la clé enregistrée) | `config.go:344` ; `CHANGELOG:824` |
| `mcp.<service>.trusted_url` | URL imposée par l'équipe approuvée par l'utilisateur (`oh mcp trust`) ; sans elle, le jeton n'est pas envoyé | `config.go:371` ; `cmd/mcp_trust.go` |
| `remote.experimental`, `fanout.experimental` | Interrupteurs des fonctions expérimentales (défaut `false`) | `cmd/remote_experimental.go:19-39`, `cmd/fanout.go:69`, `cmd/config_fields_v5.go:139-141` |
| `OH_ALLOW_NEWER_SCHEMA=1`, `OH_SKIP_DB_BACKUP=1`, `OH_REQUIRE_SIGNATURE=1`, `OH_DOCTOR_OFFLINE=1`, `OH_WORKFLOW_REPORTS=off`, `NO_COLOR` | variables d'environnement documentées | `storage/sqlite/store.go:48`, `migrate.go:217`, `selfupdate/signature.go:26`, `install.sh:154-161`, `cmd/doctor.go:34-40`, `tui/theme/nocolor.go` |

### 4.4 Réglages du mode distant

`[remote] experimental = false` par défaut (`config/remote.go:14`). Sans lui, refus (« mode distant expérimental, non validé : oh config set remote.experimental true ») de `oh remote setup`, `oh run --runtime remote`, de l'option ☁ de la fiche de lancement, de `oh session fetch` et `oh session resolve` ; le runtime distant n'est plus choisi par défaut (`cmd/remote_experimental.go:45-60`). Une fois activé, avertissement permanent (récap de lancement, envoi, `oh remote status`, `oh doctor`). Cibles : `[[remote.targets]]` (`oh config set remote.targets.<cible>.tag|builder|arch|timeout`, `config_fields_v5.go`). Le côté job (`oh runner`) ne lit pas `hub.toml`.

---

## 5. opencode

| Quoi | Valeur | Preuve |
|---|---|---|
| Version minimale supportée | **2.0.0** (4.2.0 : 1.15.0) | `cli/internal/adapters/opencodev2/compatibility.json:3` (`default`) et `:10` (`oh_versions."5.0"`) ; 4.2.0 : `internal/opencode/compatibility.json` |
| Version maximale supportée | **2.99.99** (4.2.0 : 1.99.99) | `compatibility.json:4,11` |
| Version recommandée | **2.0.25** (= `OPENCODE_VERSION` de la CI) | `compatibility.json:5,12` ; `.github/workflows/ci.yml:143` ; test `TestCIVersionIsRecommended` (`adapters/opencodev2/ci_version_test.go`) |
| Versions à empreinte épinglée (4 cibles Linux : `linux-x64`, `linux-x64-musl`, `linux-arm64`, `linux-arm64-musl`) | **2.0.20** et **2.0.25** | `adapters/opencodev2/pinned_integrity.json:4-9` (2.0.20), `:10-15` (2.0.25) ; procédure et script `.github/scripts/opencode-pin.sh` dans `pins.go:12-52` |
| Comportement hors table | Téléchargement du binaire Linux (images de conteneur) avec **avertissement** ; **refus** avec `execution.strict_isolation = true` ; version table = refus si npm annonce une autre empreinte | `pins.go:1-12` ; `docs/reference/config.fr.md:148` ; `oh doctor` avertit (commit `0c00ac89`) |
| Message de refus d'une version trop ancienne | `opencode 1.18.29 n'est pas pris en charge : oh v5 demande la version 2.0.0 ou plus récente.` + conseil `oh doctor` | `cli/internal/i18n/locales/fr.json:342-347` ; **le guide cite un texte différent** (`migration-v5.fr.md:12`, `.en.md:12`) |
| V1 | plus aucun repli : `requireV2` | `cmd/v5_require.go:30`, `cmd/start_alias.go:30` |

---

## 6. État des fonctions (ce que disent le code et les docs)

| Fonction | État réel | Preuves |
|---|---|---|
| **Mode équipe** | **Bêta** en 5.0 ; correction complète annoncée en 5.1. Avertissement une fois par processus sur stderr (`oh team …`), dans les notes du premier claim d'un lancement, dans `oh doctor` (avertissement, code de sortie inchangé), en en-tête des vues d'équipe. Limites connues : réservation libérée à l'arrêt de la session même avec `cp-2` en attente ; pas d'expiration (reprise manuelle après `takeover.stale_days`, 3 par défaut, `oh team claim transfer`) ; libération perdue hors ligne ou si le démon s'arrête avant le push ; `oh team release <ticket> -p <projet>` ; le doctor signale les orphelins. Colonne REVIEW = ticket `in_progress` avec le label `ready-for-review` ; vote MCP approuvé `in_progress` → `review` → `done` | `docs/guides/team-setup.fr.md:5,672-683` (idem `.en`, `team-testing-guide.*:5,1634-1645`, `docs/reference/cli-team.*:5,330-341`) ; `cmd/team.go:45`, `cmd/team_beta.go:11-32`, `cmd/start_dev_mode.go:229`, `cmd/doctor_claims.go:113-154` ; `internal/beads/status.go:5-13` ; `internal/mcp/team/server.go:727` |
| **Mode distant** | **Expérimental, désactivé par défaut** (`remote.experimental`) ; « jamais validé sur un vrai runner » ; macOS/Linux ; `risk: publish` jamais en distant ; workflows qui l'autorisent : `ticket`, `audit`, `review`, `onboarding` | `docs/guides/remote-runners.fr.md:5-13` ; `SECURITY.md:160,247` ; `oh workflow list` (colonne RUNTIME) |
| **Fan-out** | **Expérimental, désactivé par défaut** (`[fanout] experimental`) ; `implement-spec` et `sweep` avec `-i fanout=true` ; refusé en session distante ; budget global 10 $ ; fonctionne en conteneur (« toujours expérimental ») | `docs/guides/fan-out.fr.md:3,5,118-124` ; `docs/reference/config.fr.md:151` ; `config/fanout.go:8` |
| **Conteneur** | Non étiqueté expérimental : Colima/Podman/Docker, macOS et Linux (Windows : local seulement) ; images par projet, opencode à la même version, aucun secret dans le conteneur, passerelles Beads et MCP ; workflows qui l'autorisent : `implement-spec`, `merge-conflicts`, `quick`, `sweep`, `sweep-task`, `ticket`, `libre` ; tests réels `-tags container` en local | `docs/guides/container.fr.md:13,84` ; `oh workflow list` ; `cli/Makefile` (`test-container`) |
| **Wiki vivant** | Livré, non étiqueté : `docs/wiki/`, serveur MCP `wiki` (`wiki_index/read/propose/pending/apply/check`), `oh wiki check\|pending\|decide\|stats`, workflow `doc-sync`, propositions validées à `cp-wiki`/`cp-sync` ; migrations 45, 48, 49 ; en distant, propositions mises en file jamais appliquées | `CHANGELOG:564-570,492-496,169-174` ; `internal/mcp/wiki/server.go:14,54` ; `cmd/wiki*.go` ; `docs/reference/cli-projects.fr.md:170-223` |
| **Évals** (`oh eval`, `make test-eval`) | Outil développeur, **hors CI et manuel** : vrai modèle, jamais de clé dans les secrets de la CI ; 71 cas, référence de 35 cas (Haiku ×3) ; `--tiered`, `--plan`, `--variant`, `--parallel`, `--budget-total` | `.github/workflows/ci.yml:127` ; `cli/Makefile:126-152` ; `evals/README.fr.md` ; `CHANGELOG:324-329,456-461,572-578` |
| **Serve web** (`oh serve`) | Local seulement (`127.0.0.1`), **jeton aléatoire** par lancement (URL `?token=…`, cookie `HttpOnly`/`SameSite=Strict`), 401 sans jeton, 403 si `Host` ≠ `127.0.0.1/localhost/[::1]` ; `-p` (ancien « port ») dépréciée → `--port` ; en 4.2.0 : aucune garde | `cmd/serve_auth.go:15-61`, `cmd/serve.go:26-39,51` ; `docs/reference/cli-infra.fr.md:315-347` |
| **Windows** | Sessions locales ; démon dans le processus d'oh (pas d'arrière-plan) ; pas de conteneur, pas de distant ; `oh upgrade oh` non pris en charge (« unsupported OS ») ; moins vérifié : 27 paquets exclus de la CI | `CHANGELOG:1010` ; `SECURITY.md:94` ; `.github/windows-test-packages-excluded.txt` ; `cli/internal/selfupdate` (exclu) |

---

## 7. Ruptures dédoublonnées (état final vérifié)

Source : les 51 lignes ⚠ / « Rupture » de `[Unreleased]` (`CHANGELOG:10-1189` ; lignes 69, 70, 79, 85, 98, 108, 112, 113, 125, 126, 133, 142, 146, 178, 179, 187, 198, 203, 206, 212, 223, 229, 244, 249, 250, 255, 259, 260, 264, 293, 294, 299, 301, 362, 372, 452, 580, 597, 612, 622, 725, 780, 814, 822, 853, 870, 875, 878, 1118, 1126 et 310) regroupées en **44 ruptures**, auxquelles s'ajoutent **4 ruptures des lots v5.0 récents** (R30 `./hub.toml`, R32 `policies.toml`, R47 confirmation `--yes`, R48 mode équipe bêta) : **48 au total** (R01 à R48). Colonne « 4.2.0 » : **Oui** = un utilisateur de la 4.2.0 la subit ; **Non** = n'existait pas en 4.2.0 (rupture seulement par rapport à un build de développement) ; **Partiel**.

« Couvert » = présent aujourd'hui dans `MIGRATION.md` (M) et/ou `docs/guides/migration-v5.{fr,en}.md` (G, section). **Aucune** rupture des thèmes configuration (sauf `[opencode]`), sécurité, touches, base de données, distant, codes de sortie, options courtes n'est couverte aujourd'hui.

### A. opencode et installation

| # | Rupture — état final | Action requise | CHANGELOG | Couvert | 4.2.0 |
|---|---|---|---|---|---|
| R01 | **opencode V2 obligatoire** : plage 2.0.0 – 2.99.99, recommandée 2.0.25 (§ 5) ; sans V2, `oh run`/`oh start` refusent, sans repli (`cmd/v5_require.go:30`) | Installer opencode V2 (outil propre à opencode), vérifier `oh doctor` (ligne « opencode V2 ») | 1025, 1029, 911 | G §1 (min seulement ; citation du message inexacte ; ni max ni 2.0.25) ; M : seulement l'en-tête l.3 | Oui |
| R02 | `oh upgrade opencode`, installation gérée `~/.oh/bin`, clés `[opencode] version\|channel\|auto_update\|install_dir` supprimées (`upgrade` n'a plus que `oh`, dump) | Installer/mettre à jour opencode avec son outil ; clés ignorées puis perdues à l'enregistrement | 1029 | G §1 ; M l.135-136, 151-153, 216 : **cite encore** ces commandes | Oui |
| R03 | `oh plugin` (+ `install\|list\|remove\|status`), vue Plugins, plugins globaux V1 dont RTK supprimés ; plugins par workflow (`plugins:`) | Déclarer les plugins dans le workflow | 1029 | G §4 ; M l.126-128 : cite encore `oh plugin` | Oui |
| R04 | Empreinte des paquets Linux d'opencode épinglée (2.0.20 et 2.0.25) ; version hors table = avertissement, refus avec `strict_isolation` | Utiliser une version épinglée, ou accepter l'avertissement | 70 (« 2.0.20 » seulement) | non | Non (conteneur) |
| R05 | `oh upgrade oh` : jamais de version plus ancienne sans demande explicite ; signature Cosign vérifiée si `cosign` est là (`OH_REQUIRE_SIGNATURE=1` pour l'exiger) ; `install.sh` refuse sans outil de somme de contrôle | Aucune (info) | 149, 1126, 1157 | non | Partiel |

### B. Commandes et options

| # | Rupture — état final | Action | CHANGELOG | Couvert | 4.2.0 |
|---|---|---|---|---|---|
| R06 | `oh deploy` et `oh sync` : masquées, ne font qu'afficher le message de migration, **code de sortie 2** ; plus d'écriture dans `.opencode` ; étapes Agents/Déploiement retirées de `init`/`project add\|configure` ; `projects.agents` supprimé (migrations 36-37) ; liens `.opencode` des worktrees | Retirer `oh deploy` des scripts ; lancer `oh migrate deploy-cleanup` (proposé une fois) | 82, 1024, 1030 | G §6, §8 (code 2 non dit) ; M : en-tête l.3 seulement, corps cite `oh deploy` (l.71, 77, 95-98, 258) | Oui |
| R07 | `oh start\|audit\|review\|debug`, `oh review feedback`, `oh takeover-brief enrich` **dépréciées** (cachées, avertissement) ; `oh review --publish` → `oh mr create` ; **`oh service` (+ `setup`, `remove`) aussi dépréciées** (oubli du CHANGELOG). `oh start` → `oh run feature` → `spec` (ou `implement-spec` avec tickets) | Passer à `oh run <workflow>` (table du guide) | 86, 1039, 1044 | G §2 (sans `oh service`) ; M l.3 | Oui |
| R08 | `oh start --parallel` sans `--tickets` refusé ; `--agent` combiné à un mode refusé (→ `oh run libre --agent`) ; `--max-sessions`, `--priority`, `--sweep-branch-prefix` sans effet ; `--yes` déprécié ; `--resume <id>` → `oh session attach <id> --attach suspend` | Adapter les scripts | 1032, 1039 | G §2 | Oui |
| R09 | `oh skill add\|list\|remove\|search` et `oh skill budget` supprimés (`oh skill check` reste ; budget : `oh bundle show <wf> --budget`) ; `~/.oh/skills` plus lu (`oh doctor` signale) | Copier les skills voulues dans le catalogue d'équipe | 1033 | G §10 ; M : non | Oui |
| R10 | **Codes de sortie** : 0/1/2/3 (§ 2.3) ; commande ou option inconnue **2** (était 1 en 4.2.0) ; `oh deploy\|sync` 2 ; `oh run --headless` 3 ; `oh doctor` : 1 seulement sur échec obligatoire, avertissements = 0 | Revoir les scripts qui testent `$? == 1` | 79-84, 1087 | **non** (ni G ni M) ; `docs/reference/cli.fr.md:39-52` | Oui |
| R11 | **Options courtes : une lettre, un sens** (§ 2.2) ; anciennes formes acceptées une version avec avertissement : `oh serve -p`, `project configure -m`, `metrics -d`, `review -m\|-b`, `debug -i`, `audit -t`, `start -w\|-l`, `session attach --how` | Utiliser les nouvelles formes | 85, 1102 | **non** (`docs/reference/cli.fr.md:53-` seulement) | Oui |
| R12 | Formes **supprimées sans alias** : `oh config model … -j` (→ `-p`) et `oh purge --include-opencode` (→ `--include-tool-data`) : code 2 | Remplacer dans les scripts | 1102, 1106 | non | Oui |
| R13 | `oh status --json` : `tool` et `tool_version` à la place de `opencode_version` (`cmd/status.go:64-65` ; 4.2.0 : `:63`) | Adapter les scripts | 1107 | non | Oui |
| R14 | Projet implicite : hors du dossier d'un projet, oh ne prend plus en silence le seul projet enregistré ; une commande qui écrit demande confirmation et **refuse sans terminal** (« lancez depuis le dossier du projet ou utilisez -p ») ; une commande qui lit annonce le projet (`cmd/start_helpers.go:50-122` ; message `cmd.run.project.outside` ; erreur simple = code 1) | Se placer dans le projet ou passer `-p` | 899 | non | Oui (4.2.0 : projet unique pris sans question) |

### C. Workflows, agents et couches

| # | Rupture — état final | Action | CHANGELOG | Couvert | 4.2.0 |
|---|---|---|---|---|---|
| R15 | **`feature` et `cadrage` sont des alias** (plus de `feature.yaml` ni `cadrage.yaml`) : `feature`/`cadrage` → `spec`, `feature` avec `--tickets` → `implement-spec` (`-m` y est refusé : pas d'entrée `request` ; consigne par `-i instructions=…`) ; checkpoints `cp-plan` (remplace `cp-0`), `cp-final` (remplace `cp-feature`) ; couche qui étend `hub:feature` ou `hub:cadrage` refusée (`extends_alias`, `workflow/resolution.go:200`) ; `oh doctor` liste « Workflows retirés » | Réécrire les couches avec `extends: hub:spec` ou `hub:implement-spec` | 299, 301, 362, 203 | G §2 (l.26 seulement) ; M : non (mais M parle d'agents) | Non (workflows nouveaux), mais `oh start` / `oh run feature` : Oui |
| R16 | Alias `secu` (`security-fix`), `perf` (`benchmark`), `dbg` (`debug`), `feedback`/`retours` (`review-feedback`) **lus dans les YAML** ; termes en dur `dev`, `start`, `fast`, `rev`, `orchestrator` **plus reconnus** dans la palette et `oh run` | Utiliser les identifiants ou alias ci-contre | 179, 203, 302 | M (« secu/perf » : 2 occurrences non pertinentes) ; G : non | Non |
| R17 | `spec` : la demande (`request`) est **facultative** ; sans demande, la session la demande (`question`) ; `oh run feature`/`oh start`/`oh run spec` sans demande ne sont plus refusés ; entrée `source` ajoutée | Aucune | 294 (vs 299) | non | Non |
| R18 | **Agents** : 5 retirés (`developer-refactor`, `developer-migrator`, `test-generator`, `infra`, `database`), 4 ajoutés (`conductor`, `merger`, `operator`, `release-manager`) ; un workflow d'équipe qui nomme un agent retiré : `agent_removed` (`workflow/removed_agents.go:21`) avec le remplaçant | Retirer ces membres | 725-729 | **M** (§ « Agents supprimés », l.5-17 ✓) ; G : non | Oui |
| R19 | `libre` : plus d'auto-délégation déduite de la permission `task` ; profondeur 3 → 2 ; modes combinés du reviewer par `oh run review` ; agent d'entrée par défaut `orchestrator` ; **sans verrou de commit** (voulu) | Aucune | 229, 160 | G §3 (partiel) | Non |
| R20 | **Couches : « ne peuvent que durcir »** — `skills.deny`, préconditions des workflows `risk: publish`, `mcp`/`plugins`, `after:`, `calls`, `circuit_breaker`, `writes`, `code_mode`/`modes.allowed`, `checkpoints.<id>.approver` (couche d'équipe seulement) ; l'assouplissement donne `loosening` | Revoir les couches existantes (`oh workflow validate`) | 113, 259, 264, 379, 884, 237 | non | Non |
| R21 | **Anciennes surcharges du workflow** (`[workflow]` de `hub.toml`/`config.toml`/projet) migrées automatiquement (migration v38) vers des workflows `implement-spec` du team-state ; original archivé | Vérifier `~/.oh/migrated/` | 967, 1063 | G §9 | Oui |
| R22 | **Checkpoints contrôlés par oh** : `verify:` lancé par oh avant déverrouillage, preuves, limites de tours de correction, verrous `unlocks: [commit, push, close, create, update, apply, tag, mr]` appliqués à tous les agents, fenêtres bornées ; `expect: fail` exige build/typecheck/lint verts ; budget de pauses | Aucune (comportement) | 622-631, 452, 1117, 1150 | non | Non |
| R23 | **Sous-agents fermés par défaut** : le plugin oh relaie les opérations verrouillables ; **démon arrêté → ces opérations refusées** dans toute session ; sans plugin oh actif, un workflow qui délègue à des sous-agents **ne démarre plus** | Garder le démon (`oh daemon status`) | 255 | non | Non |
| R24 | **Passerelle Beads** : `risk: read` = vraiment lecture seule (`bd comments add`, `bd ready --claim` refusés) ; `--actor="<personne> via oh/<agent>"` imposé ; provenance ; `bd --readonly` sur chaque lecture ; `beads.allow` appliqué aussi en local par un faux `bd` en tête du `PATH` | Aucune ; un chemin absolu vers `bd` est refusé aux agents | 780-785, 1163 | non | Non (local) / Partiel |
| R25 | **Conteneur** : vérification d'un checkpoint (`verify:`) lancée dans un conteneur de l'image du projet (jamais sur la machine à la place) ; `implement-spec`, `sweep`, `sweep-task`, `merge-conflicts`, `quick` s'y lancent si le runtime par défaut est `container` | Aucune | 249, 250 | non | Non |
| R26 | **Briques d'équipe** (`oh brick`, ADR-054) : avec `catalog.lock`, une brique modifiée à la main ou commitée sans oh est ignorée ; liens symboliques ignorés ; catalogue sans lock = « jamais scellé », `oh brick publish --adopt` | Publier avec `oh brick publish` | 244 | non | Non |
| R27 | **Budgets** : décision `$` levée dès que le dépassement est vu (402 puis reprise) ; chaque enfant de fan-out reçoit une **part** du budget restant ; réservation des requêtes concurrentes (429 + `Retry-After`) ; `limits.models` vérifiée au lancement | Aucune (info) | 108, 133-138 | non | Non |
| R28 | **Identifiants Bedrock** : `auth_mode` respecté (`bearer` n'utilise que la clé API enregistrée ; `profile`/`env` signent en SigV4 et **ignorent toute clé enregistrée**) ; clé refusée par AWS → alerte ✗ | Vérifier `auth_mode` ; `oh provider setup bedrock --from-env` | 822-827 | non | **Oui** (`auth_mode` existait en 4.2.0) |

### D. Configuration

| # | Rupture — état final | Action | CHANGELOG | Couvert | 4.2.0 |
|---|---|---|---|---|---|
| R29 | Clés renommées : `[llm] default_provider`, `[execution] tool_version` (anciennes lues en repli ; **seule** `opencode.default_provider` est acceptée par `oh config set`, § 4.2) ; `oh purge --include-tool-data` ; `OH_TOOL_VERSION` côté runner | Renommer à la main si besoin | 1104-1108 | M : cite `[llm] default_provider` (l.187) mais aussi `[opencode] default_provider` (l.151-153) ; G §1 : « `[llm] default_provider` reste » | Oui |
| R30 | **`./hub.toml` n'est plus lu** (4.2.0 le lisait quand `~/.oh/hub.toml` manquait) ; test dans `config_test.go` | Placer la configuration dans `~/.oh/hub.toml` (ou `OH_HOME`) | (aucune entrée — lot V1-C) | non | Oui |
| R31 | Clés et sections supprimées : `[opencode]`, `[deploy] disable_native_agents`, `[workflow.overrides]` ; ignorées puis perdues à l'enregistrement | Rien à faire | 1029-1030 | G §1, §6 | Oui |
| R32 | **`policies.toml` est réécrit en entier** (encodeur TOML, un commit par modification) par le service `internal/services/policy` : **les commentaires et la mise en forme d'un fichier écrit à la main sont perdus** (CLI `oh policies add` et vue Policies de la TUI, plus d'écriture sur la boucle de la TUI). Reste hors service : `cmd/team_steps.go:326` (écrit via `SavePolicies` + `CommitAndPush`) | Ne pas compter sur les commentaires du fichier | (aucune entrée — lot V1-C ; la note est dans le plan, § V1-C) | non | Oui |
| R33 | Clés du trousseau MCP : écriture sous `openhub.mcp.<service>.token[.<projet>]`, anciennes clés lues puis copiées (jamais supprimées) ; `oh doctor` signale une clé vide | Aucune | 114 | non | Partiel (la convention existait en 4.2.0) |

### E. Sécurité visible

| # | Rupture — état final | Action | CHANGELOG | Couvert | 4.2.0 |
|---|---|---|---|---|---|
| R34 | **Environnement du serveur opencode filtré par liste blanche** : `GEMINI_API_KEY`, `GH_TOKEN`, **`SSH_AUTH_SOCK`**, `NPM_TOKEN`… ne passent plus ; `oh doctor` liste les variables retenues et avertit si un nom de secret est ajouté | `oh config set execution.env_allow "SSH_AUTH_SOCK, MYCO_*"` pour git par ssh, `gh`, etc. | 98-102, 65 | **non** (ni M ni G) | Oui (4.2.0 passait tout l'environnement) |
| R35 | Le proxy d'identifiants ne relaie plus en-têtes ni query de l'agent tels quels (liste blanche par fournisseur) ; un en-tête personnalisé ajouté aux options du fournisseur d'opencode est retiré | Aucune (cas rare) | 69 | non | Marginal |
| R36 | **Garde global** : les agents ne peuvent plus lancer `oh mcp trust\|untrust\|setup\|enable\|disable\|reset\|serve`, `oh config set\|unset\|language\|websearch`, `oh config model default\|family\|agent\|unset`, `oh service setup\|remove`, `oh secrets set\|delete\|get\|cleanup`, `oh budget set\|unset\|raise` (lectures permises) | Lancer ces commandes soi-même | 94, 112 | non | Non |
| R37 | **`oh mcp trust <service>`** : une URL MCP imposée par l'équipe n'est utilisée qu'une fois approuvée (épinglée dans `mcp.<service>.trusted_url` du `hub.toml` personnel) ; https obligatoire hors loopback ; sans approbation, jeton non envoyé + avertissement (`oh doctor`, `oh mcp status`, `oh team config`, vue MCP touche `T`) | Chaque membre lance `oh mcp trust <service>` | 126-128 | **non** | Oui (`url_enforced` existait) |
| R38 | Passerelle MCP fermée par défaut (`tools/call` seulement si `_meta` nomme une session connue du groupe ; 403/413) | Aucune | 125 | non | Non (conteneur) |
| R39 | **`oh serve`** : jeton aléatoire obligatoire (URL `?token=…` puis cookie), 403 si `Host` ≠ loopback | Ouvrir l'URL affichée ; `curl …?token=…` | 146 | non | Oui |
| R40 | Balises de données des prompts (`<oh:data-…>` avec empreinte), branches vérifiées (`git check-ref-format`) | Aucune | 142-145 | non | Non |

### F. Touches de la TUI

| # | Rupture — état final | Action | CHANGELOG | Couvert | 4.2.0 |
|---|---|---|---|---|---|
| R41 | **Contrat des touches** : taper une lettre n'ouvre plus la palette (`Ctrl+P` ou `/`) ; `Entrée` et les lettres ne sont plus pris à la liste ; `d` ne ferme plus les toasts (`Échap`) ; assistants : `Échap` → « Quitter l'assistant ? » ; fiche de lancement : `Ctrl+S` | Apprendre `Ctrl+P` | 855-860 | non | Oui |
| R42 | **Convention de touches** (§ 3.3) : `n` crée partout, `d` supprime (confirmé), `x` archive, `R` reprend/restaure, `c` compare, `y/n/e/s/A` décident | Message « Nouvelles touches » au premier lancement | 187, 198, 206, 212, 222-224, 853 | non | Oui (projets, patterns, board d'équipe, worktrees, détail d'équipe, wiki d'équipe, listes) |
| R43 | Une seule vue **Nettoyage** (oh gc + anciens déploiements) ; palette : `cleanup`, `gc`, `nettoyage`, `deploy-cleanup` | Aucune | 178, 204 | G §8 | Non |

### G. Mode distant

| # | Rupture — état final | Action | CHANGELOG | Couvert | 4.2.0 |
|---|---|---|---|---|---|
| R44 | **Mode distant expérimental, désactivé par défaut** : `oh config set remote.experimental true` pour l'essayer | Activer pour tester, avec des projets et jetons de test | 870 | non | Non (`oh remote` absent en 4.2.0) |
| R45 | **`oh remote setup` à relancer** : `PipelineSchema` 5 (reports.json), enveloppe de journal 3, paire de clés du journal, variables limitées à `oh-run/*` ; sinon envoi refusé (« pipeline d'une autre version ») | Relancer `oh remote setup` après chaque mise à jour **pour les testeurs de builds intermédiaires** | 612, 618, 875, 878, 1108 | non | **Non** : rien à relancer pour un utilisateur de la 4.2.0 |

### H. Base de données

| # | Rupture — état final | Action | CHANGELOG | Couvert | 4.2.0 |
|---|---|---|---|---|---|
| R46 | Base migrée de la version **27 à 49** au premier lancement (verrou entre processus, copie `~/.oh/backups/oh.db.v27-<date>.bak` avant) ; une base plus récente est refusée (`OH_ALLOW_NEWER_SCHEMA=1`) ; `~/.oh` en 0700, `oh.db` en 0600 ; mots de passe des serveurs hors de `oh.db` (`oh export` n'en emporte plus) ; **pas de retour arrière simple vers la 4.2.0** : restaurer la copie | Garder la copie ; `oh repair --restore <fichier>` | 897, 1048, 1152 | non | Oui |

### I. Confirmation et mode équipe (lots v5.0 en cours)

| # | Rupture — état final | Action | Source | Couvert | 4.2.0 |
|---|---|---|---|---|---|
| R47 | **Confirmation sans terminal : refus sauf `--yes`, code de sortie 2, partout** — **à venir V2-A**. État de `main` : `confirm()` (`cmd/workflow_teamcli.go:152-157`) répond **oui sans terminal** pour `workflow publish\|archive\|restore` et `brick publish` ; `oh doctor --fix`, `oh gc`, `oh migrate deploy-cleanup`, `oh session delete`, `oh repair` refusent déjà sans `--yes` mais **en code 1** (`errors.New`, `cmd/gc.go:219`, `session_delete.go:49`, `migrate_cleanup.go:144`, `doctor_db.go:121`…) ; 12 `huh.NewConfirm` restants (`v5_existing.go`, `project_remove.go`, `migrate_cleanup.go`, `run.go`, `misc.go`, `purge.go` ×2, `worktree.go`, `teams.go`, `session_resolve.go`) échouent sans terminal (« open /dev/tty », code 1, `CHANGELOG:1134`). V2-A (branche `feat/v50-confirm`, `confirmAction`, message `cmd.confirm.needs_yes`) unifiera en refus + code 2 et ajoutera `--yes` aux commandes qui n'en ont pas | Ajouter `--yes` aux scripts et à la CI | Plan § D4, V2-A | non (à écrire après fusion de V2-A) | Oui |
| R48 | **Mode équipe en bêta** (§ 6) : avertissement stderr une fois par processus, claim libéré à l'arrêt de la session, pas d'expiration, libération qui peut se perdre ; colonne REVIEW = `in_progress` + label `ready-for-review` | Lire les limites connues | `docs/guides/team-setup.fr.md:672-683` | non | Oui |

**Total : 48 ruptures dédoublonnées** (R01 à R48), classées par la colonne « 4.2.0 » :
- **27 « Oui »** — un utilisateur de la 4.2.0 les subit : R01-R03, R06-R14, R18, R21, R28-R32, R34, R37, R39, R41, R42, R46-R48 ;
- **16 « Non »** — n'existaient pas en 4.2.0, donc ruptures seulement par rapport à un build de développement de la v5 : R04, R16, R17, R19, R20, R22, R23, R25-R27, R36, R38, R40, R43-R45 ;
- **5 « Partiel »** : R05, R15 (nouveauté, mais `oh start`/`oh run feature` existaient), R24, R33, R35.

Les 44 autres sont issues des 51 lignes ⚠/Rupture ; R30, R32, R47 et R48 viennent des lots v5.0 récents et n'ont pas (encore) d'entrée dans `[Unreleased]`.

---

## 8. Contradictions de `[Unreleased]` (vérifiées contre `main`)

Colonne « Entrée dépassée » : ligne(s) du CHANGELOG qui disent l'ancien état. « Entrée à jour » : ligne qui dit le nouvel état. « État final » : preuve dans le code ou les docs.

| # | Sujet | Entrée dépassée | Entrée à jour | État final réel (preuve) |
|---|---|---|---|---|
| C01 | Alias `secu`/`perf`/`dbg`/`feedback`/`retours` « retirés » puis rétablis | 203 (« ne sont plus reconnus »), 950 (« anciens noms en alias ») | 179 | Rétablis dans les YAML : `workflows/benchmark.yaml:11`, `debug.yaml:11`, `review-feedback.yaml:11`, `security-fix.yaml:11` ; `dev`, `start`, `fast`, `rev` absents (`cmd/tui_commands.go`, `docs/reference/tui.fr.md:128`) |
| C02 | Aide `F1` → `?` et `Ctrl+P` | 198 (« l'aide est sur `F1` »), 847 (« Aide `F1` accessible partout ») | 180 | `?` ou `F1`, plus `Ctrl+P` dans les onglets et le lot (`keys.go:224`, `shell.go:1391-1438`) |
| C03 | Touches Sessions / récap `r t k d p D` | 818 (`r` reprendre · `t` ailleurs · `k` · `d` · `p`, et `D` supprime) | 198, 206, 223 | Récap : `R`, `T`, `k`, `d`, `p` (`views/launch_form_existing.go:43-47`) ; Sessions : `d` supprimer, `R` reprendre (`tui.fr.md:81-83,172-195`) |
| C04 | Badge `● N ⏸ M` | 983 | 87 | `● N ◐ M` (`views/sessions_section.go:63-72`, `glyph.go:57-60`) |
| C05 | `--how` → `--attach` | 1039 et 1101 (`oh session attach --how here`) | 85 | `--attach` (`cmd/session.go:271`), `--how` alias masqué, `here` → `suspend` (`:62-64`) ; doc `cli-sessions.fr.md:418` (« `--attach suspend` ») |
| C06 | Repli sur l'ancien lancement | 1044 (« sans opencode V2 ou sans le workflow, l'ancien lancement reste ») | 1029, 1039 (« sans repli ») | Supprimé : `requireV2` (`cmd/v5_require.go:30`), appelé par `startAlias` (`cmd/start_alias.go:30`) ; `docs/reference/cli-sessions.fr.md:418` (« l'ancien lancement n'existe plus ») |
| C07 | `oh start` → `feature` | 1044, 1052 (« `oh start` lance le workflow `feature` ») | 299 | `startAlias` envoie à l'identifiant `feature` (`start_alias.go:118`), qui est un alias de `spec` (ou `implement-spec` avec tickets) |
| C08 | `feature` et `cadrage` présentés comme workflows | 936 (liste des workflows livrés), 1047, 1117 (« `cp-2` de `ticket`, `feature`… »), 885, 888 (N9 « `cp-tickets` dans `feature` ») | 299, 362 | `workflows/feature.yaml` et `cadrage.yaml` n'existent pas (24 fichiers) ; `cp-tickets` est dans `spec.yaml` ; `docs/reference/workflows.fr.md:19,112-117,179` |
| C09 | `request` de `spec` obligatoire | 299 (« La demande est obligatoire sans tickets ») | 294 | Facultative : `workflows/spec.yaml:18-26` (pas de `required`), commentaire `:15-16` |
| C10 | `-m` de `feature` | 279 (« pas de `-m`, docs corrigées ») | 228, 299 (`oh start -m …`) | `oh run -m` = `-i request=…` si le workflow a l'entrée `request` (`cmd/run.go:37,142-148`) ; refusé pour `implement-spec` (`docs/reference/workflows.fr.md:116-117`) |
| C11a | Nombre d'agents | 654 (« 15 sur 16 »), 702 (16), 729 (« 16 au lieu de 20 ») | 279 (« 18 agents en 9 familles ») | **18** (§ 1) ; 4.2.0 : 19 |
| C11b | Nombre de workflows | 275 (24), 279 (22), `README.md:13` (22), `getting-started.*:21` (22) | `README.md:125` (24) | **24** |
| C11c | Nombre de skills | 586 (« 169 skills »), 696 (« 184 descriptions ») | — | **180** + 11 annexes (§ 1) ; `docs/architecture/skills.*` : 178/189 (retard) |
| C12a | « Non fait » : S-13, S-31, S-32 « toujours ouverts » | 275 | 123-129 | Faits : passerelle MCP fermée, URL d'équipe approuvée, chemins d'API échappés |
| C12b | « Non fait » : validation par lot dans la TUI, migration de `sweep` et de `feature` | 340 | 317, 283-284, 299 | Faits (touche `B`, `sweep -i fanout=true`, `feature` alias) |
| C12c | « Non fait » : `feature` comme enchaînement et retrait d'`orchestrator` | 367 | 299, 302 | Fait |
| C12d | « Reste à brancher » : `audit → security-fix` | 351 | 291 | Branché (`next:`) |
| C12e | « Viendra avec R1-b » : contenu du mode TDD | 454 | 423 | Fait (le skill `tdd` a été retiré faute de gain ; consignes `when method=tdd` livrées) |
| C12f | Agents ne voient pas les propositions refusées | 174 | 75 | `wiki_pending` liste les refus des 30 derniers jours avec motif (`internal/mcp/wiki/server.go:14,54`) ; reste vrai : `decisions-rejetees.md` non alimentée par les motifs |
| C12g | « `cadrage` conservé tel quel (alias prévu avec R5) » | 405 | 362 | `cadrage` = alias de `spec` |
| C12h | Étape 2 du valideur (blocage réel en distant) « à venir » | 239 | — | Toujours à venir (non contradictoire ; à garder) |
| C13 | `orchestrator-dev-parallel` créée puis retirée | 547 | 160 | Absente de `skills/` (`find`) |
| C14 | Commits de l'onboarder | 372 (« `ONBOARDING.md`, `CONVENTIONS.md` et le push restent à l'utilisateur »), 891 (« ne commite pas ») | 259 | `onboarding.yaml:30` (`writes: docs/wiki/**, ONBOARDING.md, CONVENTIONS.md`), `cp-wiki` `unlocks: [commit]` (`:32-40`) : il commite wiki + ces deux fichiers ; le push reste à l'utilisateur |
| C15 | `cp-0`/`cp-feature`/`cp-1/2/3` | 885 (« y compris invoqué par l'orchestrateur de `feature` »), 1117 | 299 | `ticket.yaml:44-82` (cp-1, cp-seams, cp-red, cp-2, cp-3) ; `implement-spec.yaml:83-148` (cp-plan, cp-1, cp-2, cp-3, cp-merge, cp-final) ; plus de `cp-0` ni `cp-feature` |
| C16 | `coordination-only` supprimée | 892 (« exemples de délégation génériques (`coordination-only`) ») | 586 | Aucune skill de ce nom (`skills/`) |
| C17 | Vue Nettoyage | 204 (« Nouvelle vue Nettoyage (oh gc) ») | 178 | Une seule vue à deux sections (`cmd/tui_commands.go:57-59`, `views/gc_view.go`, `cleanup_deploy.go`) |
| C18 | Session libre « V1 » | 948 (« Session libre sans workflow (opencode V1) ») | 1022, 1100 | `libre` est un workflow (`workflows/libre.yaml`) |
| C19 | Schéma du pipeline 2 → 5 | 1108 (« 2 »), 875 (« 3 »), 878 (« 4 ») | 618, 495 (« 5 », « inchangé (5) ») | **5** (`cli/internal/remote/contract.go:15`) |
| C20 | Épinglage 2.0.20 vs CI 2.0.25 | 70 (« pour la 2.0.20 ») | 60 (CI 2.0.25) | Épinglées : 2.0.20 **et** 2.0.25 ; recommandée 2.0.25 (§ 5) |
| C21 | Section « Documentation » de la 4.x (l.1167-1188) | 1181 (`--parallel`, `--sweep`, `oh skill budget`, `review feedback`…), 1188 (SVG `tui-navigation-modes`) | 1032-1033, 689 | `oh skill budget` supprimée ; SVG supprimés ; `--parallel`/`--sweep` dépréciés ; contenu de la 4.x (ADR-037, gates CI) à archiver |
| C22 | Windows | 911 (« pas encore pris en charge sous Windows ») | 1010 | Sessions locales sous Windows ; `container.fr.md:13`, `SECURITY.md:94` |
| C23 | Touches : worktrees `x`, board d'équipe `x`, wiki `x` | 856 | 187, 212, 223 | `M`, `L`, `n` (§ 3.3) |
| C24 | Touches du catalogue de workflows | 972 (`D` diff et impact, `x` archiver ou abandonner un brouillon), 973 (`y` YAML brut) | 187, 223 | `c` comparer, `d` supprimer un brouillon, `x` archive les publiés, `Y` YAML brut (`tui.fr.md:81-95, 311-325`) |
| C25 | `oh skill budget` alias de `oh bundle show` | 944 | 1033 | Supprimé (dump) |
| C26 | « `oh config set` accepte les anciennes clés » | 1105 | — | Seule `opencode.default_provider` ; `execution.opencode_version` refusée (§ 4.2) |
| C27 | Palette : `start`, `feature`, `orchestrator`, `cadrage` → `run spec` | 302 | 203, 179 | `start` et `orchestrator` ne sont plus reconnus ; `feature`/`cadrage` par `aliases:` |
| C28 | `--agent` de `oh start` « garde le lancement par agent (avec confirmation si `--recap`) » | 1044 (fin) | 1022, 1032 | `oh start --agent` → `oh run libre --agent` (`start_alias.go:83-84`) ; combiné à un mode, refusé |
| C29 | Alias dépréciés listés | 86 | — | Oubli : `oh service` (+ `setup`, `remove`) dépréciés (`cmd/misc.go:59-78`) ; `oh deploy`/`sync` sont masquées (pas dépréciées) |
| C30 | `--include-opencode` « ancien nom » | `docs/reference/cli-infra.fr.md:234` | 1106 | Refusé (« unknown flag », code 2) |
| C31 | Message d'opencode trop ancien | `docs/guides/migration-v5.fr.md:12`, `.en.md:12` | — | Texte réel : `i18n/locales/fr.json:342` |
| C32 | Identifiant d'audit « S-28 » employé pour trois sujets | 69 (en-têtes du proxy), 98 (liste blanche d'environnement), 103 (jeton révoqué) | — | À nettoyer en écrivant `[5.0.0]` (un seul libellé par sujet ; renvoyer à `docs/dev/audit-v5.md`) |
| C33 | Doublons de catégories | 121 sections `###` dont plusieurs « Added »/« Changed »/« Fixed » séparées | — | `[5.0.0]` : une section par catégorie (plan V3-A) |

**33 lignes** (dont C12 en 8 sous-cas) ; les ~20 de l'annexe A.1 sont toutes confirmées (C01-C21), + **C22 à C33 trouvées en plus**.

---

## 9. Ruptures et commandes absentes ou fausses dans `MIGRATION.md` et `migration-v5.*`

### 9.1 `MIGRATION.md` (338 lignes)

Structure actuelle : l.1-3 en-tête v5 ; l.5-17 « Agents supprimés » (**correct**, 5 agents) ; l.21-338 guide **oc → oh** (bash → Go, historique, 2.0.0) laissé comme s'il était actuel.

**Faux ou périmé** (à marquer « historique » ou à retirer) :

| Ligne(s) | Texte | Réalité |
|---|---|---|
| 31-39 | « 5.5 MB », « 213 tests, 24 packages », « 481 clés i18n » | chiffres de la v2.0.0 |
| 67, 71 | `oh init` « Télécharge opencode si absent », « Déploie agents/skills/config dans le projet » | `oh init` ne télécharge ni ne déploie (`docs/reference/cli-infra.fr.md:7-24`) |
| 77, 95-98, 258-259 | `oh deploy -p`, `oh deploy --check\|--diff`, `oh sync --all\|--dry-run` ; « les agents déployés restent en place » | supprimées (code 2) ; `oh migrate deploy-cleanup` |
| 88-94, 134, 193 | `oh start -p`, `--dev`, `--onboard`, `--parallel`, `--resume`, `--worktree`, `--provider` | alias dépréciés (G §2) |
| 112-114 | `oh audit --type`, `oh review`, `oh debug --issue` | dépréciées → `oh run audit\|review\|debug` ; `-t`, `-m`, `-b`, `-i` : formes dépréciées |
| 115 | `oc conventions` « Supprimé » | `oh conventions check` existe |
| 120-121 | `oh optimize`, `oh yield` | masquées |
| 126-128 | `oh plugin install\|remove\|status` | supprimée |
| 129-131, 176, 270 | `oh service setup\|status\|remove` | dépréciée → `oh mcp setup\|status\|disable` |
| 135-136, 216 | `oh upgrade opencode`, « `oc update` → `oh upgrade opencode` » | supprimée |
| 151-153 | `[opencode] version = ""`, `default_provider` dans `hub.toml` | supprimé (ignoré) ; `[llm] default_provider` |
| 233-234 | « `--prompt` utilise `-m`, `--path` utilise `-d` » | `-m` = `--message` ; `--prompt` seulement pour `oh start` |
| 275-278 | « `oh doctor` vérifie la compatibilité… warning au `oh start` avec la commande de mise à jour » | refus net si opencode < 2.0.0 ; plus de commande de mise à jour d'oh pour opencode |

**Absent** (rien dans `MIGRATION.md` ; tout le passage v4 → v5 de V3-B) : opencode V2 (min/max/recommandée) ; `oh deploy`/`sync` supprimées ; `oh migrate deploy-cleanup` (cité l.3 seulement) ; `execution.env_allow` et la liste blanche ; `oh mcp trust` ; codes de sortie ; options courtes ; touches de la TUI et `Ctrl+P` ; alias `feature`/`cadrage`/`secu`/`perf`/`dbg`/`feedback`/`retours` ; `[llm] default_provider`, `[execution] tool_version` ; `./hub.toml` ; `policies.toml` ; `--yes` obligatoire sans terminal (V2-A) ; `oh status --json` ; `-j` et `--include-opencode` ; mode distant expérimental et `oh remote setup` ; mode équipe bêta ; migration de la base (27 → 49) et sauvegarde ; skills communautaires (`oh skill add…`) ; `oh plugin` ; Windows.

### 9.2 `docs/guides/migration-v5.{fr,en}.md` (101 lignes chacune, parité structurelle OK : 10 sections identiques)

**Couvert** : §1 opencode V2 (min 2.0.0), §2 alias de lancement, §3 `libre`, §4 plugins, §5 métriques, §6 plus de déploiement, §7 catalogue des briques, §8 `deploy-cleanup`, §9 migration v38, §10 skills communautaires.

**Faux ou imprécis** :
- §1 (`.fr.md:12`, `.en.md:12`) : le message cité n'est pas le message réel (`i18n/locales/fr.json:342` : « … n'est pas pris en charge : oh v5 demande la version 2.0.0 ou plus récente. »).
- §1 : ne dit ni la version maximale (2.99.99) ni la version recommandée (2.0.25) ni l'empreinte épinglée.
- §2 (`.fr.md:26`) : `oh start` → `oh run feature` sans dire que `feature` est un alias de `spec` ; ne cite pas `oh service`.
- §6 (`.fr.md:63`) : « Pendant v5.x, les deux commandes affichent un message de migration » — sans le code 2 ni « masquées de l'aide ».
- §9 (`.fr.md:93`) : « workflows `implement-spec` (`feature` jusqu'à ce qu'il devienne un alias) » : tournure obsolète (les couches migrées visent `implement-spec`, `cp-0` → `cp-plan`).

**Absent** : mêmes sujets que pour `MIGRATION.md` (liste ci-dessus), en plus de : alias `feature`/`cadrage` expliqués ; `[opencode] …` : cité (§1) mais pas `[deploy] disable_native_agents` hors §6 ; absence de table « avant/après » des touches.

### 9.3 Autres fichiers à corriger (hors périmètre strict, à signaler à V3-B)

- `.github/pull_request_template.md:20` : « `oh deploy && oh deploy --check` passes » (plan V3-B).
- `docs/reference/cli-infra.fr.md:234` / `.en.md:234` : « Ancien nom : `--include-opencode` » (refusé).
- `docs/guides/getting-started.*:21`, `README.*:13` (22 workflows) ; `docs/architecture/skills.*:10,140`.
- `cli/cmd/misc.go:59-78` : messages de dépréciation de `oh service` en français en dur (pas i18n).
- Aucune doc ne mentionne `execution.env_allow` dans un guide d'usage git/ssh (seule la référence config, `config.fr.md:173-205`).

---

## 10. Propositions pour V3-A

### 10.1 Plan « En bref » (12 puces, ordre du plan)

1. **Runtime v5 et opencode V2** : chaque session part d'un paquet compilé hors du projet, sur un serveur `opencode serve` par groupe, « monde fermé » vérifié ; opencode V2 obligatoire (2.0.0 – 2.99.99, recommandée 2.0.25) ; démon `ohd`, proxy d'identifiants, `oh session` ; plus rien n'est déployé dans les projets (`oh deploy`/`sync` supprimées, `oh migrate deploy-cleanup`).
2. **Workflows déclaratifs `oh/v1`** : 24 workflows livrés (`oh run`), 18 agents en 9 familles, 180 skills, couches hub/équipe/projet qui ne font que durcir ; `oh start|audit|review|debug` restent des alias dépréciés ; alias `feature`, `cadrage`, `secu`, `perf`, `dbg`, `feedback`, `retours`.
3. **Workflows d'équipe et briques** : team-state (`workflows/`, `catalog/`), brouillons, publication, historique, impact, `workflows.lock`/`catalog.lock`, espace solo, gouvernance ; `oh workflow`, `oh brick`.
4. **Checkpoints contrôlés par oh** : vérification lancée par oh, preuves, verrous d'opérations (`commit`, `close`, `create`, `apply`, `tag`, `mr`…), décisions humaines tracées (valideur), validation par lot, rapports stockés (`oh session report|trace|approve`).
5. **Conteneur** : Colima/Podman/Docker, image par projet, aucun secret dans le conteneur, passerelles Beads et MCP, vérifications dans le conteneur.
6. **Mode distant (expérimental)** : GitLab CI, `oh remote setup`, désactivé par défaut (`remote.experimental`), jamais validé sur un vrai runner.
7. **Fan-out (expérimental)** : une session enfant par ticket ou sous-tâche, merge par oh, `cp-merge`, désactivé par défaut (`fanout.experimental`).
8. **Wiki vivant** : `docs/wiki/`, serveur MCP `wiki`, propositions validées au checkpoint, `oh wiki check|pending|decide|stats`, `doc-sync`.
9. **Sécurité** : environnement filtré (`execution.env_allow`), garde-fous globaux, `oh mcp trust`, passerelle MCP fermée, intégrité des paquets, empreintes d'opencode, `oh serve` protégé, signature des releases ; « garde-fous, pas barrières » (SECURITY).
10. **TUI refondue** : Sessions (à traiter, fan-out, onglets), fiche de lancement, éditeur de workflow, vue Nettoyage, convention de touches unique (`?`, `Ctrl+P`, `F1`), mode compact, `NO_COLOR`.
11. **CLI plus cohérente** : codes de sortie 0/1/2/3, options courtes à sens unique, aide construite depuis l'arbre des commandes, confirmation `--yes` (à écrire après V2-A), i18n fr/en.
12. **Mode équipe en bêta** : publié avec avertissement et limites connues (correction complète en 5.1) ; **évals** (`oh eval`) pour l'équipe de développement.

### 10.2 Regroupement des ruptures par thème (titres du plan V3-A) — avec le renvoi à l'inventaire

| Thème de `[5.0.0]` | Ruptures à y mettre (dédoublonnées) |
|---|---|
| opencode et installation | R01, R02, R03, R04 (une ligne), R05 (info) |
| commandes et options | R06, R07, R08, R09, R10 (codes de sortie), R11, R12 (`-j`, `--include-opencode`), R13 (`status --json`), R14 |
| workflows et couches | R15-R21 (alias, agents, `libre`, durcissement, migration v38) ; R22-R27 en une puce « comportement des sessions » (checkpoints, sous-agents, Beads, conteneur, briques, budgets) ; R28 (Bedrock) |
| configuration | R29, R30 (`./hub.toml`), R31, R32 (`policies.toml`), R33 (info) |
| sécurité visible | R34 (**la plus importante pour un utilisateur de 4.2.0** : `SSH_AUTH_SOCK`, `GH_TOKEN`), R35-R40 |
| touches TUI | R41, R42 (renvoyer au message « Nouvelles touches » et à `docs/reference/tui.*`), R43 |
| distant | R44, R45 (en précisant « rien à faire pour une 4.2.0 ») |
| base de données | R46 |
| confirmation `--yes` | R47 (à écrire une fois V2-A fusionné) |
| mode équipe | R48 |

Règles d'écriture à retenir :
- Une puce = **l'action** (commande à lancer, clé à changer) + renvoi vers `MIGRATION.md` (ancres à créer en V3-B).
- Ne **pas** annoncer comme ruptures les changements internes entre builds de développement (schémas 2→4 du pipeline, `--how` introduit puis renommé…) : dire l'état final seulement.
- `release-notes.sh` (`.github/scripts/release-notes.sh`, test `cli/internal/docstest/release_notes_test.go`) lit le corps de `## [5.0.0]` jusqu'au `## [` suivant, **sans le titre**, sans ligne `---` de fin, et **échoue si la section est vide**. Un tag `v5.0.0-rc.N` lit `## [5.0.0-rc.N]` puis, à défaut, `## [5.0.0]`. Il faut donc que `## [5.0.0]` existe (non vide) avant tout tag, et que `## [Unreleased]` reste vide au-dessus.
- Liens relatifs du détail déplacé dans `docs/dev/changelog-v5-lots.md` : `TestDocLinks` (liens + ancres de tous les `.md`).
- Ne pas oublier : corriger les chiffres (24 workflows, 18 agents, 180 skills, schéma 49) ; la section « Documentation » héritée de la 4.x (l.1167-1188) part dans l'archive.

---

## 11. Points incertains (à trancher avant d'écrire)

1. **R47 (confirmation, V2-A)** : l'état « avant » est vérifié sur `main` ; l'état « après » vient d'une branche non fusionnée (message `cmd.confirm.needs_yes`, `--yes` ajouté à certaines commandes, liste exacte inconnue). À réécrire après fusion.
2. **R45 / schémas du pipeline** : une 4.2.0 n'a pas `oh remote` (vérifié dans les deux arbres). La consigne du plan V3-B « `oh remote setup` à relancer » ne vaut que pour les testeurs de builds intermédiaires.
3. **Nombre de ruptures « qui concernent un utilisateur de la 4.2.0 »** : classement fait sur la présence de la fonction dans le tag `v4.2.0` ; certains cas sont « marginaux » (R35, R05) ; je n'ai pas testé le comportement de la 4.2.0 sur toutes les lignes (seulement les codes de sortie, les options et les arbres de commandes).
4. **Retour arrière vers la 4.2.0** (R46) : je n'ai pas testé l'ouverture d'une base v49 par un binaire 4.2.0 (la copie `…v27-….bak` est la voie sûre).
5. **Skills** : 180 = fichiers avec `name:` ; `oh skill check` ne donne pas de décompte ; le décompte de `docs/architecture/skills.*` (178) peut venir d'une autre règle (à regarder avant de corriger la doc).
6. **Mode équipe** : le vote MCP « approuvé → `review` puis `done` » est lu dans `internal/mcp/team/server.go:727` ; la transition complète (jusqu'à `done` + label `agent-reviewed`) n'est pas revérifiée ici (tests du lot V1-B).
7. La date du `## [5.0.0] — <date>` reste à fixer par l'utilisateur au moment du tag.
