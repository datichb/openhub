# 15 — Plan de tests manuels de la release v5

> Rédigé le 07/10/2026 sur `main` = `fe9ed932` (toutes les pistes fusionnées, dépôt propre). À dérouler avant REL (tag v5.0.0).
> Légende : **P1** indispensable avant le tag · **P2** important · **P3** cas limite · 🔁 déjà fait en recette par une piste (à refaire par un humain) · ⚠️ **jamais fait en réel**. Cocher `☐` → `☑` (ou `☒` en cas d'échec, avec le n° d'anomalie).
> Rapport d'anomalie : n° du test, commande, résultat obtenu, sortie de `oh doctor`, journaux (`-v`) ; consigner en fin de fichier (§ « Anomalies »).

## 0. Préparation

| ☐ | # | Action |
|---|---|---|
| ☑ | 0.1 | `cd cli && make embed-sync && make build` ; `bin/oh` dans le `PATH` ; `golangci-lint cache clean` (cache périmé des worktrees supprimés) |
| ☑ | 0.2 | Outils : `opencode --version` (2.0.20), `bd --version`, `git --version` (≥ 2.48), Colima et/ou Podman, iTerm2 si possible |
| ☑ | 0.3 | Projet de test : dépôt git jetable + `bd init` + 4 tickets (A, B détaillés `ai-delegated` ; épopée C avec D) |
| ☑ | 0.4 | `OH_HOME` isolé pour les tests destructifs (`export OH_HOME=~/oh-test`) ; `~/.oh` réel **sauvegardé** pour les tests de migration v4 |
| ☑ | 0.5 | Clé LLM de test **au niveau projet** ; `oh budget set session_budget_usd 1` |
| ☑ | 0.6 | Une équipe de test (team-state distant de test) + un espace solo ; un 2ᵉ clone du team-state (publication concurrente) |

## 1. Installation, compatibilité, migration v4 → v5

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☑ | 1.1 | 1 | `oh version`, `oh doctor` | Version opencode, adaptateur, isolation, démon, proxy, sécurité (capacité, socket), git, terminal ; sortie 0 si tout est vert |
| ☑ | 1.2 | 1 | Casser une vérification (VM Colima arrêtée) puis `oh doctor; echo $?` | Ligne ✗, code **1** (les avertissements ne comptent pas) |
| ☑ | 1.3 | 1 | Retirer opencode du `PATH` (ou installer une 1.x), `oh run quick` | Refus clair (`cmd.v1.unsupported.*`), renvoi vers `oh doctor` et `docs/guides/migration-v5` |
| ☑ | 1.4 | 1 ⚠️ | `~/.oh` v4 réel (projets déployés, surcharges de workflow équipe/projet/`hub.toml`), premier lancement v5 | Migrations v28 → v41 sans erreur ; v38 crée les workflows qui étendent `hub:feature` (espace solo si pas d'équipe) ; archives dans `~/.oh/migrated/` |
| ☑ | 1.5 | 1 ⚠️ | `oh migrate deploy-cleanup --dry-run --diff`, puis sans `--dry-run` | Fichiers listés (`.opencode/agents`, `.opencode/skills`, `.deploy-state`, `context-manifest.json`, `.opencode/team.json`) ; dans `opencode.json`, seules les clés d'oh non modifiées sont retirées ; **clés utilisateur conservées** |
| ☑ | 1.6 | 2 | TUI après la mise à jour | Écran de nettoyage proposé une fois ; Doctor ne signale plus de restes après nettoyage |
| ☑ | 1.7 | 1 | Anciennes commandes : `oh start`, `start --dev -t A`, `start --onboard`, `audit -t security`, `review`, `debug -i "…"`, `review feedback <branche>`, `start --parallel --tickets A,B`, `takeover-brief enrich A`, `deploy`, `sync` | Avertissement « alias déprécié » puis le bon `oh run <wf>` ; `deploy`/`sync` = message de migration ; `--parallel` sans `--tickets` refusé |
| ☑ | 1.8 | 2 | `oh start --agent debugger -m "…"` | Session `libre`, entrée `debugger` |
| ☑ | 1.9 | 2 | Anciennes clés de config (fournisseur par défaut, version de l'outil) ; `oh config get/set` sur l'ancien nom | Lues et redirigées vers `[llm] default_provider` / `[execution] tool_version` |
| ☑ | 1.10 | 3 | `oh purge --include-tool-data` (OH_HOME jetable) | Tout supprimé ; `--include-opencode` n'existe plus |

## 2. Catalogue, paquets, skills (CLI)

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☑ | 2.1 | 1 | `oh workflow list` (+ `--json`) | Les 12 workflows livrés (`audit`, `brief-enrich`, `cadrage`, `debug`, `feature`, `libre`, `onboarding`, `quick`, `review`, `review-feedback`, `sweep`, `ticket`) + équipe/projet ; couche, version, brouillons, avertissements d'intégrité |
| ☒ | 2.2 | 1 | `oh workflow show ticket --origin` | Workflow résolu avec l'origine de chaque valeur |
| ☑ | 2.3 | 1 | `oh workflow validate --all` ; fichier avec cycle, agent inconnu, variable de gabarit inconnue, `risk: read` + agent qui écrit, champ inconnu | `--all` vert ; chaque erreur avec ligne/colonne, chemin et indication |
| ☑ | 2.4 | 1 | `oh bundle show ticket --budget` ; `oh bundle build ticket --json` ×2 | Agents, skills (fermeture `requires`), permissions, MCP, budget ; `~/.oh/bundles/<hash>` ; **même hash** les deux fois |
| ☑ | 2.5 | 2 | `oh bundle show audit` | Skills `audit-<domaine>` présentes |
| ☑ | 2.6 | 2 | `oh skill check` ; `oh skill budget --all` | Aucune erreur ; alias fonctionnel |
| ☒ | 2.7 | 2 | Projet Django / FastAPI / Flask / Rails : `oh bundle show ticket` | Skills de stack incluses (django, fastapi, rails) |

## 3. Lancement `oh run` (local)

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☑ | 3.1 | 1 🔁 | `oh run quick -i request="…"` | Paquet, serveur, onglet ouvert, bon agent ; toast de fin (coût, branche) |
| ☑ | 3.2 | 1 🔁 | `oh run ticket --tickets A` | Entrée `orchestrator-dev` ; A passe `in_progress` et vous est réservé |
| ☑ | 3.3 | 1 🔁 | `oh run ticket --tickets A,B --recap` | « 2 sessions · 1 serveur · 2 worktrees », confirmation ; 2 onglets ; **les deux prompts reçus** (pas de redémarrage du serveur) |
| ☒ | 3.4 | 2 | `oh run ticket --tickets C` puis `--one-session` | Une session par ticket de l'épopée, ou une seule |
| ☑ | 3.5 | 1 | `--location base`, `new`, `<worktree existant>` | Bon dossier ; worktree **automatique** si une autre session qui écrit est active au même endroit |
| ☐ | 3.6 | 1 | `--mode manuel/semi-auto/auto` + mode non autorisé | Mode non autorisé refusé ; comportement des checkpoints conforme (§7) |
| ☑ | 3.7 | 1 🔁 | `oh run brief-enrich --headless -i ticket=A --output out.md` | Brief enrichi écrit, session arrêtée ; brief lu automatiquement (`ticket.brief`) |
| ☑ | 3.8 | 1 | `oh run ticket --headless --tickets A` | **Refusé** (cp-2 obligatoire) |
| ☑ | 3.9 | 1 | `oh run review-feedback -i mr=<URL|!iid>` (direct, pas l'alias) | Discussions et branches de la MR lues automatiquement ; MR introuvable = refus avec message |
| ☐ | 3.10 | 2 | Workflow avec précondition (`feature` sans contexte d'onboarding) | `onboarding` suggéré d'abord ; reprise mémorisée et reproposée par « Enchaîner avec… » |
| ☑ | 3.11 | 2 | Même lancement deux fois très vite (ou `Ctrl+S` ×2 dans la fiche) | Un seul lancement |
| ☐ | 3.12 | 2 | `attach = iterm|terminal|tmux|browser|none`, `--attach here` ; styles iTerm `tab|split|window` | Respectés |
| ☑ | 3.13 | 1 | `attach = iterm` **sans iTerm2** | Terminal.app, puis tmux ; jamais le client dans le processus de `oh run` |
| ☐ | 3.14 | 2 | `oh run` sans terminal (script, `attach=none`) | Session lancée, « oh session attach <id> » affiché, pas de blocage |
| ☐ | 3.15 | 2 | `oh run` sans argument, projet avec workflow par défaut | Workflow par défaut lancé |
| ☐ | 3.16 | 3 | Refuser l'autorisation macOS « Automation » | Mode d'ouverture suivant ; signalé par Doctor |

## 4. Monde fermé, isolation, fournisseur

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☑ | 4.1 | 1 🔁 | Dans la session : liste des agents (`@`) et des skills | Seulement les agents du workflow (+ système cachés) ; **ni `build/plan/general/explore`**, ni skills `opencode`/`report` |
| ☑ | 4.2 | 1 | Agent parasite `~/.config/opencode/agents/scout.md`, `strict_isolation = false` | Attestation en échec, message précis (« scout »), session non démarrée |
| ☑ | 4.3 | 1 | Même chose avec `[execution] strict_isolation = true` | Session démarrée (config utilisateur masquée) |
| ☑ | 4.4 | 1 | `env` demandé à l'agent ; `ps eww <pid opencode serve>` | **Aucun secret** (seulement `ohs_…`), ni dans la session ni dans le processus serveur |
| ☐ | 4.5 | 1 | Modèle indisponible / fournisseur non autorisé | Pas de bascule vers les modèles gratuits hébergés |
| ☐ | 4.6 | 2 | Bedrock région UE | Préfixe `eu.` (profil d'inférence) |
| ☐ | 4.7 | 2 | Plugin global de l'utilisateur présent | Avertissement à l'attestation, non bloquant |

## 5. Cycle de vie, démon, veille

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☑ | 5.1 | 1 🔁 | Fermer l'onglet pendant le travail ; `oh session list` | Session vivante (`active` puis `idle`) |
| ☑ | 5.2 | 1 | `oh session attach <id>` (et un préfixe d'id) | Rouverte au bon endroit |
| ☑ | 5.3 | 1 | Session inactive `idle_sleep_minutes` (mettre 1) | `sleeping` ; **pas** de veille si une décision est en attente |
| ☑ | 5.4 | 1 | `oh session resume <id>` / `c` dans Sessions | Serveur redémarré, même paquet ; l'agent reçoit une consigne de reprise (`oh.resume`) |
| ☑ | 5.5 | 1 | `oh session stop <id>` | Arrêtée (Terminées) ; autres sessions du groupe intactes |
| ☑ | 5.6 | 1 🔁 | Quitter la TUI (`Ctrl+Q`, Échap, `quit` de l'omnibar) avec 2 sessions qui travaillent | Choix **par session** (finir l'étape puis veille par défaut / arrière-plan / arrêter) ; inactives en veille tout de suite |
| ☑ | 5.7 | 2 | Rouvrir oh après une absence | Récap « pendant votre absence » (sessions, décisions, coût) |
| ☑ | 5.8 | 2 | `oh daemon status` ; `oh daemon stop` avec sessions (puis `--force`) | Refus sans `--force` ; redémarrage à la demande |
| ☑ | 5.9 | 2 | `kill -9` du démon pendant une session | Redémarre, jetons restaurés, suivi repris |
| ☐ | 5.10 | 3 | Redémarrage de la machine | Sessions reprenables |
| ☐ | 5.11 | 3 | Démon sans serveur > 10 min ; et pendant une longue construction d'image | Arrêt automatique dans le 1er cas ; **pas** d'arrêt dans le 2ᵉ |

## 6. Pilotage depuis oh (sans attacher)

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☑ | 6.1 | 1 | Permission demandée ; `oh session inbox` ; `oh session approve <id> --decision once -m "…"` | Décision `!` visible ; l'agent continue |
| ☑ | 6.2 | 1 | `always` dans un workflow `isolation: strict` | Refus clair |
| ☑ | 6.3 | 1 | Question de l'agent ; `oh session answer <id>` sans champ puis `--field k=v` | Champs attendus listés ; réponse reçue |
| ☑ | 6.4 | 1 | Répondre à la même décision dans opencode **et** dans oh | La première gagne ; l'autre : « résolu dans l'interface » |
| ☑ | 6.5 | 1 | `oh session send <id> "…"` (+ `--queue`, `--synthetic`) | Consigne reçue (immédiate / au tour suivant) |
| ☑ | 6.6 | 2 | `oh session interrupt`, `compact`, `model <id> <fournisseur/modèle>` | Étape interrompue (session ouverte) ; compactage ; nouveau modèle à l'étape suivante |
| ☑ | 6.7 | 2 | `oh session fork <id>` | Nouvelle session avec l'historique ; coût copié **non compté deux fois** (`oh budget show`) |
| ☑ | 6.8 | 1 | `oh session follow <id>` | Agent courant, outils, messages, coût ; sous-agents rattachés |
| ☒ | 6.9 | 2 | `oh session results <id>` | Diff, branche, coût, description de MR (même serveur arrêté) |
| ☐ | 6.10 | 2 | `oh session open <id> --browser` (+ `--print`) | Code d'appairage valable 5 min |
| ☑ | 6.11 | 1 | Notifications, TUI ouverte et fermée | Une par nouvelle décision et fin de tour, regroupées ; clic (terminal-notifier) ramène le terminal ; aucun contenu sensible |

## 7. Checkpoints et état de session

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☑ | 7.1 | 1 🔁 | `oh run ticket --tickets A --mode manuel` | cp-1 ⏸ dans À traiter ; **`developer` refusé avant validation** |
| ☑ | 7.2 | 1 | Valider cp-1 depuis la fiche oh | L'agent continue et appelle `developer` |
| ☑ | 7.3 | 1 | Même chose en `semi-auto` | cp-1 automatique ; cp-2 en pause (obligatoire dans tous les modes) |
| ☑ | 7.4 | 1 | Fiche de cp-2 | Résumé, changements et diff complet, derniers messages, frise ; valider / corriger d'abord (+ message) / autre consigne |
| ☑ | 7.5 | 1 | `oh session approve <id> --decision fix -m "…"`, puis `--decision other` | Message reçu, reprise ; sans `-m` = refus |
| ☒ | 7.6 | 2 | Valider le checkpoint depuis l'interface opencode | Accepté, reflété dans oh |
| ☐ | 7.7 | 2 | > 12 appels de sous-agents d'affilée sans intervention | ✗ coupe-circuit ; appels refusés jusqu'à `oh session dismiss` |
| ☑ | 7.8 | 1 | Après un checkpoint, demander à l'agent d'entrée « checkpoints passés et suivant ? » | Bonne réponse (`oh.checkpoints`) ; un sous-agent ne le sait pas (attendu) |
| ☒ | 7.9 | 2 | Sorties déclarées en fin de session (branche, MR, tickets) | Toast « ↪ Enchaîner avec… », fiche suivante préremplie |

## 8. Restrictions et budget (I6)

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☑ | 8.1 | 1 | `oh budget show` (+ `-p <projet>`) | Restrictions effectives avec origine, dépense du jour |
| ☑ | 8.2 | 1 | `oh budget set session_budget_usd 0.05`, session qui dépasse | Étape finie puis décision `$` ; étape suivante **interrompue** tant qu'elle est ouverte |
| ☑ | 8.3 | 1 | Relever : touche **`$`** dans Sessions (montant vide, puis précis) ; `oh budget raise <id> 1` | Budget relevé, reprise ; l'agent d'entrée connaît le budget restant |
| ☑ | 8.4 | 2 | `daily_budget_usd` épuisé | Nouvelle session refusée (« budget journalier dépensé ») |
| ☑ | 8.5 | 2 | `max_active_sessions 1` + 2 sessions | 2ᵉ en `queued` (`…`), démarre quand la 1ʳᵉ finit |
| ☑ | 8.6 | 2 | `models` restreint à un modèle ; `oh session model` vers un autre | Refusé par le proxy |
| ☐ | 8.7 | 2 | `memory_mb` | File, veille des groupes inactifs, notification ; estimation dans Doctor |
| ☐ | 8.8 | 2 | `[limits.enforced]` d'équipe + valeur projet plus large | Plafond de l'équipe appliqué |
| ☐ | 8.9 | 3 | `limits.models` d'un `extends` plus large que le parent | Erreur d'assouplissement |

## 9. Proxy d'identifiants, secrets, sécurité locale

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☐ | 9.1 | 1 | Clé projet, puis équipe (`K` dans le détail d'équipe), puis hub | La plus spécifique est utilisée |
| ☐ | 9.2 | 1 ⚠️ | Bedrock par **profil AWS** (SigV4) ; `OH_E2E_AWS_PROFILE=… make test-e2e` | Session fonctionnelle ; e2e SigV4 vert |
| ☐ | 9.3 | 2 | `-P anthropic`, `openrouter`, `github-copilot` | Chacun fonctionne via le proxy |
| ☐ | 9.4 | 1 | Sessions ouvertes, `kill` du démon, port occupé avant son redémarrage | Groupes concernés en veille (✗ pour ceux qui travaillaient) ; reprise avec la nouvelle URL (proxy, passerelles, MCP distants) |
| ☑ | 9.5 | 2 | `sqlite3 ~/.oh/oh.db 'select token from proxy_grants'` | Empreintes seulement |
| ☐ | 9.6 | 2 | Appel du socket du démon par un autre utilisateur / sans capacité | Refus (UID, 403) |
| ☑ | 9.7 | 2 | Section sécurité de `oh doctor` | Capacité (trousseau ou ⚠ fichier), droits du socket, jetons hérités ; **aucune fenêtre** du trousseau |

## 10. Beads

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☑ | 10.1 | 1 | Session `review` en **local** : `bd show A`, puis `bd create …` | `show` passe ; `create` **refusé** (`beads.allow`) |
| ☑ | 10.2 | 1 | Contournement : `/opt/homebrew/bin/bd create …`, `sh -c '/usr/local/bin/bd …'` | Refusé (garde-fou) |
| ☑ | 10.3 | 1 | Session `feature` : `bd create/update/close` | Autorisé (listes des workflows livrés) |
| ☑ | 10.4 | 1 | `bd` utilisé par un **sous-agent** | Mêmes règles |
| ☑ | 10.5 | 2 | `oh beads <commande en échec>; echo $?` | Code de `bd` transmis |
| ☐ | 10.6 | 2 | Lancement sur un ticket réservé par un autre membre | Avertissement |

## 11. TUI

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☑ | 11.1 | 1 | « Démarrer » sur les accueils hub, projet, équipe | ★ épinglés (≤ 5), récents (≤ 3, sans doublon), catégories repliées (projet/équipe), « Tous les workflows (N) » ; suggestions si vide |
| ☑ | 11.2 | 1 | `*` épingler (global, puis projet) | Gardé après redémarrage, dans la bonne portée |
| ☑ | 11.3 | 1 | Fiche de lancement de `ticket` : Entrées → Options → Récap | Sélecteur Beads (filtres, épopées, aperçu, multi avec Espace) ; modes autorisés ; exécution (indisponibles grisés + raison) ; emplacement ; ouverture ; récap (agents, skills, budget, MCP, checkpoints, isolation, avertissements) ; `Ctrl+S` / `Ctrl+B` |
| ☑ | 11.4 | 1 | Multi-sélection de 3 tickets | « 3 sessions · 1 serveur · 3 worktrees » |
| ☑ | 11.5 | 1 | Vue Sessions : sections (À traiter, En cours, En veille, Terminées, À récupérer) ; touches `y n x $ a A t m i M s c e o w g f r` | Chaque action fonctionne ; flux `t` suit la sélection |
| ☑ | 11.6 | 1 | Badge `● N ⏸ M` partout ; sections « Sessions » des 3 accueils | Compteurs justes, mis à jour en direct |
| ☑ | 11.7 | 1 | Omnibar : `run <wf>`, alias (`dev`, `start`, `onboard`, `feedback`, `fast`, `usage`, `hub config`), commandes contextuelles sur un ticket | Un seul sens par alias (`q`, `hub`, `tokens`, `team config`, `libre` plus en double) |
| ☑ | 11.8 | 1 | Board : actions rapides sur un ticket | Workflows acceptant un ticket, fiche ouverte à l'étape Options |
| ☑ | 11.9 | 2 | Catalogue des briques (`bricks`) | Agents/skills, origine, coût, workflows qui les utilisent ; `/`, `f` |
| ☑ | 11.10 | 2 | Réglages : Sessions, Exécution, Restrictions, Distant | Écrits dans `hub.toml`, appliqués au lancement suivant |
| ☑ | 11.11 | 1 | `oh config language en` puis `fr` | **Tous** les écrans traduits (Équipes, Statut d'équipe, Activité, Board, Worktrees, Reprises, Modèles…), pas de clé brute ni `%!(EXTRA…)` |
| ☒ | 11.12 | 2 | Vue Doctor | Mêmes vérifications que la CLI |
| ☒ | 11.13 | 3 | Mode projet | Plus de section Déploiement ni de toast d'écart ; en-tête : adaptateur + environnements |

## 12. Workflows d'équipe et de projet (team-state)

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☑ | 12.1 | 1 | `oh workflow new ticket-hotfix --extends hub:ticket --layer team` → `edit` → `validate` → `diff` → `publish -m "…"` → `history` → `restore <id> 1` → `archive` | Chaque étape ; v1 puis v2 ; restauration = nouvelle version ; archive gardée en historique |
| ☐ | 12.2 | 1 | `oh run ticket-hotfix --draft` avant publication | Brouillon lancé en local |
| ☑ | 12.3 | 1 | Brouillon qui assouplit : `risk`, `code_mode` off → on, mode ajouté à `modes.allowed`, `beads.allow` élargi | Erreur `loosening`, valeur du parent gardée |
| ☑ | 12.4 | 1 | Workflow d'équipe de même id qu'un workflow du hub, sans `extends` | `shadow_without_extends` |
| ☑ | 12.5 | 1 | 2ᵉ membre (ou 2ᵉ clone) : synchronisation et lancement | Nouvelle version visible et lançable |
| ☑ | 12.6 | 1 | Fichier publié modifié à la main + synchronisation | **Ignoré** + avertissement (catalogue, `workflow list`, Doctor) |
| ☑ | 12.7 | 1 | Deux publications simultanées (deux clones) | La seconde se refait par-dessus, est revalidée, publiée v+1 |
| ☐ | 12.8 | 2 | Publication hors ligne | File d'attente ; rejouée à la synchronisation de la TUI (ou `publish --retry`) |
| ☑ | 12.9 | 2 | `enforce:` d'équipe + modification par un projet | Refusée |
| ☐ | 12.10 | 2 | Résumé d'impact à la publication | Risque, agents qui écrivent, distant autorisé, nouvelles briques (badge) |
| ☐ | 12.11 | 2 | Brique d'équipe (`catalog/agents`, `catalog/skills`) utilisée par un workflow | Dans le paquet ; collision d'id sans `extends` refusée |
| ☑ | 12.12 | 1 | TUI, catalogue : `n` (vide/étendre/dupliquer), `e`, `v`, `t`, `p`, `D`, `h`, `x` | Chaque action ; brouillons et intégrité visibles |
| ☑ | 12.13 | 1 | Éditeur 5 sections ; `u`/`U`, `y` (YAML), `P` (gabarit dans `$EDITOR`), `w`, Échap avec modifications | Aperçu du paquet en direct, diagnostics cliquables, origines, verrous ; garde « non enregistré » |
| ☑ | 12.14 | 2 | Écrans Publication et Historique | Diff, impact, message ; restauration |
| ☑ | 12.15 | 1 | `oh team init --solo` → workflow de projet → `oh team promote --remote <url>` ; assistant TUI et « Passer en équipe » | Sans perte ; gouvernance affichée dans le détail d'équipe |
| ☒ | 12.16 | 2 | `oh workflow new x --file doc.yaml` : sans gabarit, avec `--prompt-file`, avec un document qui nomme son gabarit | Gabarit de départ créé avec un message / acceptés |

## 13. Modèles

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☑ | 13.1 | 1 | Modèles par agent du workflow, du workflow, du projet, du hub, `[models]` d'équipe ; vérifier `oh bundle show`, `oh config model show` | Ordre : workflow·agent > workflow > projet > hub > **équipe** > frontmatter |
| ☑ | 13.2 | 2 | `oh config model default|family|agent|unset` avec `-p <projet>` | `-p` fonctionne ; `-j` n'existe plus |

## 14. Conteneur (Colima **et** Podman)

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☑ | 14.1 | 1 🔁 | Config projet › Exécution : Dockerfile de dev, build args, volumes, runtime par défaut | Appliqués au lancement **et à la reprise** |
| ☐ | 14.2 | 1 ⚠️ | `oh run ticket --tickets A --runtime container` **dans iTerm2** | Image construite (progression), puis en cache ; commandes shell **dans** le conteneur |
| ☑ | 14.3 | 1 | Dans le conteneur : `env`, `bd show A`, `bd create` | Aucun secret ; `show` via la passerelle ; `create` refusé si non autorisé |
| ☐ | 14.4 | 1 ⚠️ | MCP gitlab et team avec de **vrais jetons** | Fonctionnent via la passerelle, jetons restés dans le trousseau |
| ☑ | 14.5 | 2 | Fiche, option ▣ : VM arrêtée, image en cache, image absente | État, cache ou temps estimé |
| ☑ | 14.6 | 2 | Volume relatif (`node_modules`) absent | Avertissement (plan, `--recap`, fiche) |
| ☐ | 14.7 | 2 | `keep_images`, `tool_version`, moteur dans les réglages | Respectés |
| ☑ | 14.8 | 2 | Veille d'un groupe en conteneur | Conteneur supprimé (teardown) |
| ☑ | 14.9 | 2 | Commit de l'agent dans le conteneur | Votre identité git |
| ☑ | 14.10 | 2 | Doctor conteneur/passerelles | Moteur, virtiofs, keep-id, version de l'outil dans l'image, accès aux passerelles |

## 15. Exécution distante GitLab CI ⚠️ (jamais faite en réel — runners à installer : `docs/guides/remote-runners.*`)

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☐ | 15.1 | 1 | `oh remote setup`, `oh remote status` ; Réglages › Distant | `oh-runner` créé/validé (pipeline schéma 2, déclencheur, variables masquées) |
| ☐ | 15.2 | 1 | `oh run ticket --tickets A --runtime remote` | Ticket réservé, instantané Beads et paquet envoyés, pipeline déclenché ; session ☁ |
| ☐ | 15.3 | 1 | Workflow avec checkpoint `remote: forbid` ou sans `remote` dans `runtime.allowed` | Refus **avant** l'envoi |
| ☐ | 15.4 | 1 | Le job | Aucune `ask` approuvée à l'aveugle ; `defer` s'arrête à « MR prête » ; MR brouillon créée |
| ☐ | 15.5 | 1 | oh fermé pendant le pipeline | Notification « prête à récupérer » (démon) |
| ☐ | 15.6 | 1 | `oh session fetch <id>` / `g` dans la TUI | Artefacts, session importée, reprenable en local |
| ☐ | 15.7 | 1 | Conflit Beads simulé ; `oh session resolve <id> --dry-run`, puis résolution (fenêtre TUI) | Garder local / appliquer distant / fusionner les notes ; rien sans confirmation |
| ☐ | 15.8 | 1 | Variables, artefacts et journaux du job | **Aucun secret de la machine** |

## 16. MCP

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☑ | 16.1 | 2 | `oh mcp list` | Tous les serveurs, descriptions traduites |
| ☑ | 16.2 | 2 | `oh mcp setup gitlab|jira|figma|gslides|<inconnu>` | Service demandé configuré (étape écriture pour gitlab, jira) ; inconnu = erreur |
| ☐ | 16.3 | 1 | Mode écriture pour un seul service | Seule sa variable est posée (`GITLAB_`, `JIRA_`…) |
| ☐ | 16.4 | 1 | Appel d'un outil gitlab en session | Vu `gitlab_get_project` (pas de préfixe en double) ; règles de permission par outil appliquées |
| ☐ | 16.5 | 2 | Skills figma avec une URL Figma dans le ticket | `figma_get_file` / `figma_get_node` appelés |

## 17. CLI, aide, codes de sortie, métriques

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☑ | 17.1 | 1 | `oh --help` en fr puis en en | Sections WORKFLOWS, SESSION, PROJECT, MCP, CONFIGURATION, ANALYTICS, TEAM, INFRASTRUCTURE ; une seule langue ; pas de `claim`/`release` de premier niveau |
| ☑ | 17.2 | 2 | `--help` de 10 commandes au hasard | Aide traduite, options documentées |
| ☑ | 17.3 | 2 | `oh status --json` | Champs `tool`, `tool_version` |
| ☐ | 17.4 | 2 | `oh metrics`, `oh dashboard`, `oh serve` après quelques sessions | Tableau **par agent** rempli (statut, durée, tokens, coût, skills) |
| ☐ | 17.5 | 3 | `oh history export` puis `import` | Fonctionne |

## 18. Plateformes et terminaux

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☐ | 18.1 | 1 ⚠️ | **Linux** : local, conteneur, passerelles, MCP distants | 2ᵉ écoute du proxy joignable depuis le conteneur ; `notify-send` |
| ☐ | 18.2 | 2 ⚠️ | **Windows** (option A) | Proxy dans le processus oh, sessions arrêtées à la fermeture, avertissement Doctor ; pas de conteneur ni de distant |
| ☐ | 18.3 | 2 | tmux < 3.0 | Attachement fonctionnel |
| ☐ | 18.4 | 2 | iTerm2 `tab|split|window` ; Terminal.app | Respectés |

## Environnement préparé (07/10/2026)

Racine : `~/workspace/openhub-test` — binaire `oh` construit depuis `main` `fe9ed932` (version `5.0.0-test`).

| Élément | Chemin / valeur | Charger |
|---|---|---|
| Environnement principal (vous, membre `benjamin`, lead) | `OH_HOME=~/workspace/openhub-test/oh-home` | `source ~/workspace/openhub-test/env.sh` |
| 2ᵉ membre simulé (`testeur2`, dev) — tests 10.6, 12.5, 12.7 | `OH_HOME=~/workspace/openhub-test/oh-home-2`, team-state `team-clone-2` (identité git « Testeur 2 ») | `source ~/workspace/openhub-test/env2.sh` |
| Hub **v4.2.0** déployé (migration 27) — tests 1.4, 1.5 | `v4-home/.oh` (surcharges `[workflow.overrides]` du hub : coupe-circuit 8, cp-1 en pause en semi-auto ; `projects.workflow_config` : mode par défaut `auto`), projet déployé `openhub-projet-v4` (`.opencode/agents` ×10, skills, `.deploy-state`, `context-manifest.json`, `opencode.json` avec une clé utilisateur `share` ajoutée à la main) | `source ~/workspace/openhub-test/env-v4.sh` (`oh4` = binaire v4) ; **remise à zéro** : `~/workspace/openhub-test/reset-v4.sh` |
| Projet de test (équipe) | `openhub-projet-test` (FastAPI : `app/`, `tests/`, `Dockerfile.dev`) — GitHub `datichb/openhub-projet-test` ; ID `openhub-projet-test-eb3f0f23` (env principal), `openhub-projet-test-8fcbab69` (env 2) | |
| Tickets Beads (préfixe `pt`) | **A = `pt-cb8`** export CSV (P1, `ai-delegated`) · **B = `pt-vau`** filtre par statut (P2, `ai-delegated`) · **C = `pt-c3b`** épopée Persistance · **D = `pt-c3b.1`** sauvegarde JSON (enfant de C, `ai-delegated`) | |
| Projet sans équipe (espace solo, 12.15) | `openhub-projet-solo` (local, sans remote), ID `openhub-projet-solo-f165af0f` ; **l'espace solo n'est pas créé** (c'est le test) | |
| Équipe de test | team-state `openhub-team-test` (GitHub `datichb/openhub-team-test`, structure + `members.toml` : `benjamin` lead, `testeur2` dev) ; 2ᵉ clone `team-clone-2` ; projet rattaché à l'équipe dans les deux environnements | |
| Clé Bedrock de test | Trousseau, service `openhub-oh`, **au niveau projet** : `openhub.provider.bedrock.token.<ID projet>` pour les 3 projets ; région `eu-west-1`. Aucune clé de hub ni d'équipe (vérifié : aucune clé réelle de hub dans le trousseau) | |
| Budget | `session_budget_usd = 1` (hub, deux environnements) | |
| Outil d'aide | `bin/oh-test-keyring <clé>` : écrit un secret lu sur l'entrée standard dans le trousseau (`oh secrets set` exige un terminal, voir anomalie A4) — ex. clé d'équipe pour 9.1 : `… | oh-test-keyring openhub.team.openhub-team-test.provider.bedrock.token` | |

Vérifications faites : `oh run quick --headless` → réponse « PRET » via Bedrock (0,087 $) ; `oh team status` (2 membres, dans les deux environnements) ; `oh doctor` (voir anomalies A1, A2) ; conteneur : Colima (virtiofs) et Podman démarrés, passerelles Beads et MCP joignables depuis un conteneur.

**Manquants sur la machine** (tests concernés) : **iTerm2** (3.12, 3.13, 14.2, 18.4) — `brew install --cask iterm2` ; **tmux** (3.12, 18.3) — `brew install tmux` ; **terminal-notifier** (6.11, clic) — `brew install terminal-notifier`.

**Nettoyage après la recette** : révoquer la clé Bedrock de test ; supprimer les entrées du trousseau (`security delete-generic-password -s openhub-oh -a openhub.provider.bedrock.token.<ID>` ×4) ; `oh daemon stop --force` dans chaque environnement ; supprimer `~/workspace/openhub-test`.

## Ordre conseillé
1. **Parcours P1** (≈ ½ journée) : 0 → 1.1–1.7 → 2.1–2.4 → 3.1–3.9 → 4.1–4.4 → 5.1–5.6 → 6.1–6.4 → 7.1–7.5 → 8.2–8.3 → 10.1–10.2 → 11.1–11.8 → 12.1–12.7 → 13.1 → 14.1–14.3 → 17.1.
2. **Jamais testé en réel (⚠️)** : 1.4–1.5, 9.2, 14.2, 14.4, §15, 18.1–18.2.
3. Le reste (P2, P3).

## Avancement
| Date | Section(s) | Testeur | Résultat |
|---|---|---|---|
| 07/10/2026 | 0 (préparation) | agent | ☑ 0.1–0.6 faits (détails : « Environnement préparé ») ; anomalies A1–A5 relevées |
| 08/10/2026 | Lot 3 (TUI) : étapes A–E par l'utilisateur (captures), F–J par l'agent (pseudo-terminal + `pyte`, `bin/oh-tui-drive`) | utilisateur + agent | Faits : 11.1–11.11, 3.13, 5.6, 5.7, 6.11, 7.2, 7.4, 12.12–12.15, 14.1, 14.5, 14.6, 1.6. ☒ 7.6 (A36 : cp-2 posé en `question`), 7.9 (A38), 11.12 (A40), 11.13 (A32). Notes : 11.11 — langue en : écrans traduits (sauf A33, A41) ; 14.6 — avertissement du volume vérifié par `--recap` ; 12.15 — espace solo créé depuis le catalogue (`n`), `project:quick` v1 publié, gouvernance et « Passer en équipe » présents ; 5.6 — choix par session ; 5.7 — « Pendant votre absence : 3 session(s)… » ; 1.6 — écran de nettoyage, `opencode.json` réduit aux clés de l'utilisateur, non reproposé. Anomalies A31–A42 |
| 07/10/2026 | Lot 2 (avec LLM) : étapes 1–2 par l'utilisateur, 3–9 par l'agent (`--attach none`, Haiku) | utilisateur + agent | Faits : 3.1–3.5, 3.7–3.9, 3.11, 1.7, 1.8, 4.1–4.4, 5.1–5.5, 5.8, 5.9, 6.1–6.9, 7.1–7.3, 7.5, 7.8, 8.1–8.6, 10.1–10.5, 14.1, 14.3 (Colima **et** Podman). ☒ 3.4 (A24), 6.9 (A23). Notes : 4.2 — l'agent parasite est **retiré par le plugin** (invisible, la session démarre) au lieu d'un refus d'attestation : monde fermé respecté ; 6.3 — l'agent `developer` de `quick` n'a pas l'outil `question` (fait avec `libre --agent debugger`) ; 6.1 — permission `external_directory /tmp/*` ; 14.3 — image construite depuis `Dockerfile.dev` détecté (Python 3.12.15), cache au 2ᵉ lancement (4 s), conteneur supprimé à l'arrêt, `bd` par la passerelle (`host.docker.internal` / `host.containers.internal`), aucun secret ; `quick` n'autorise pas le conteneur (`libre` utilisé). Anomalies A15–A30. Coût du lot ≈ 8 $ |
| 07/10/2026 | Lot 1 (CLI, sans LLM) : 1.1–1.5, 1.9, 1.10, 2.1–2.7, 9.5, 12.1, 12.3–12.7, 12.9, 12.16, 13.1–13.2, 16.1–16.2, 17.1–17.3 | agent | 27 ☑, 3 ☒ (2.2 → A6, 2.7 → A8, 12.16 → A10) ; anomalies A6–A14. Notes : 1.2 vérifié avec les ✗ de A1/A2 (cas « avertissements seuls = 0 » à refaire après correction) ; 1.4 : migration v38 vérifiée (hub → `~/.oh/migrated`, projet sans équipe → espace solo `solo` + `project:feature` v1 publié) ; 1.10 en `--dry-run` seulement (`--include-tool-data` vise les vrais dossiers globaux de l'outil de l'utilisateur) ; 12.7 : deux publications du même workflow en même temps → v2 (testeur2) puis v3 (benjamin) ; 16.3 non fait (jeton requis) |

## Anomalies
| N° | Test | Commande | Obtenu | Attendu | Gravité | Statut |
|---|---|---|---|---|---|---|
| A1 | 1.1 (préparation) | `oh doctor` avec un binaire `5.0.0-test` | ✗ « Version oh — 5.0.0-test → 4.2.0 disponible (oh upgrade oh) » | 5.0.0(-test) plus récente que 4.2.0 : pas de proposition de mise à jour (comparaison semver avec pré-version à vérifier ; même question pour `5.0.0`) | Majeure (inciterait à revenir en v4) | Corrigée (piste C, C1, fusion `7c05682e` ; détail : `16` § Écarts › Piste C) |
| A2 | 1.1 (préparation) | `oh doctor` avec une clé Bedrock **au niveau projet** seulement | ✗ « Provider credentials — bedrock — credentials non trouvées » alors que `oh run` fonctionne avec cette clé | La vérification suit la cascade (projet du dossier courant → équipe → hub → profil AWS) | Moyenne | Corrigée (piste C, C1, fusion `7c05682e` ; détail : `16` § Écarts › Piste C) |
| A3 | 0.6 (préparation) | `oh teams add` / `oh team init` avec `OH_HOME` | `config.TeamStatePath` clone le team-state sous `~/.oh/team-states/…` (dossier personnel réel), en ignorant `OH_HOME` | Sous `$OH_HOME/team-states/…` (isolation de `OH_HOME`, comme `HubDir`) | Moyenne | Corrigée (piste C, C2, fusion `7c05682e` ; détail : `16` § Écarts › Piste C) |
| A4 | 0.5 (préparation) | `printf … | oh secrets set <clé>` | « lecture du secret: inappropriate ioctl for device » : impossible sans terminal | Lecture sur l'entrée standard quand ce n'est pas un terminal (scripts, CI) | Mineure | Corrigée (piste C, C2, fusion `7c05682e` ; détail : `16` § Écarts › Piste C) |
| A5 | 0.3 (préparation) | `bd init` puis `oh doctor` | ✗ « Beads zero-impact » : `AGENTS.md` généré par bd, `.beads/` absent de `.git/info/exclude` | Constat de configuration du projet (pas d'oh) : corrigé dans le projet de test (fichiers retirés, exclusion ajoutée) ; à documenter (guide de prise en main : `bd init` crée `AGENTS.md`/`CLAUDE.md`) | Info | Corrigé (projet) ; documenté (piste C, C2, fusion `7c05682e` : guide de prise en main) |
| A6 | 2.2, 1.4 | `oh workflow show feature --origin` dans un projet qui a un `project:feature` v1 ; `oh workflow show project:feature` | Affiche `hub:feature` (couche projet ignorée : modes, coupe-circuit du hub) alors que `oh workflow list` montre `feature project v1` ; `project:feature` = « Workflow inconnu ». La couche **équipe** est bien résolue (`review-strict` : chaîne `hub:review → team:review-strict`) | La chaîne complète jusqu'à la couche projet, comme `list` (et `project:<id>` accepté) | Majeure | Corrigée (piste C, C3, fusion `7c05682e` ; détail : `16` § Écarts › Piste C) |
| A7 | 1.4 | `projects.workflow_config` au format inattendu (`{"mode_overrides":…}` sans l'enveloppe `overrides`) puis migration v38 | Colonne vidée **sans message ni archive** (perte silencieuse) | Configuration illisible ou inconnue : gardée (archivée) avec un avertissement, comme pour un JSON invalide. Les données réelles de v4 ont toujours l'enveloppe : risque faible | Mineure | Corrigée (piste C, C3, fusion `7c05682e` ; détail : `16` § Écarts › Piste C) |
| A8 | 2.7 | `oh bundle show ticket` dans le projet FastAPI | Skills `dev-standards-fastapi` et `dev-standards-python` ✓, mais aussi `context-mode-usage` (plugin non chargé sous V2 : consignes pour des outils `ctx_*` absents), `rtk-usage` (plugin RTK retiré en v5 ; vu aussi dans la session `quick` : 15 skills exposées dont celle-ci) et `dev-standards-frontend`, `-frontend-a11y`, `-frontend-data` (projet sans frontend) | Ni skill d'un plugin absent, ni skills de stack hors de la stack détectée | Moyenne | Corrigée (piste C, C3, fusion `7c05682e` ; détail : `16` § Écarts › Piste C) |
| A9 | 12.1 | `oh workflow validate team:ticket-hotfix` sur un brouillon non publié | « Workflow inconnu » (le chemin du fichier brouillon, lui, est validé) | Valider aussi le brouillon du membre (ou le dire dans le message) | Mineure | Corrigée (piste C, C3, fusion `7c05682e` ; détail : `16` § Écarts › Piste C) |
| A10 | 12.16 | `oh workflow new nettoyage --file <doc sans prompt>` | Brouillon enregistré, **aucun gabarit de départ créé ni message** (avec `--prompt-file` : gabarit rangé à côté du brouillon ✓) | Gabarit de départ créé avec un message (Q3-7) — ou, si un workflow sans prompt est voulu, le documenter et vérifier le prompt initial au lancement | Moyenne | Corrigée (piste C, C3, fusion `7c05682e` ; détail : `16` § Écarts › Piste C) |
| A11 | 13.1 | `oh config model show` | Seul le niveau hub est affiché (ni projet, ni équipe, ni workflow) ; en-têtes en anglais (« Model Configuration », « Hub-level ») avec la langue fr | Cascade complète, traduite | Mineure | Corrigée (piste C, C2, fusion `7c05682e` ; détail : `16` § Écarts › Piste C) |
| A12 | 13.2, 17.2 | Option ou commande inconnue (`-j`, `oh "session approve"`) | Erreurs cobra en anglais (« unknown shorthand flag », « unknown command », « session …: not found », « ohd POST /shutdown: 409 sessions are running (use force) ») avec la langue fr | Messages traduits | Mineure | Corrigée (piste C, C2, fusion `7c05682e` ; détail : `16` § Écarts › Piste C) |
| A13 | 16.2 | `oh mcp setup gitlab </dev/null` (sans terminal) | « open /dev/tty: device not configured » | Message clair (« assistant interactif : lancez-le dans un terminal ») et code non nul | Mineure | Corrigée (piste C, C2, fusion `7c05682e` ; détail : `16` § Écarts › Piste C) |
| A14 | 1.1 | `oh doctor --no-tui` dans un tube | Animation (⠋⠙⠹…) écrite quand même dans la sortie | Pas d'animation sans terminal ni avec `--no-tui` | Mineure | Corrigée (piste C, C2, fusion `7c05682e` ; détail : `16` § Écarts › Piste C) |
| A15 | Lot 2 (préparation) | `oh config set session.idle_sleep_minutes 1` (aussi `execution.*`, `limits.*`, `remote.*`) | « unknown config key » : `oh config set` et `unset` ne connaissent aucune des sections v5 (`oh config get` les lit) (`[session]`, `[execution]`, `[limits]` hors `oh budget`, `[remote]`) ; seules la TUI (Réglages) et l'édition de `hub.toml` permettent de les changer | Clés v5 dans `configFieldMap` (et `oh config list`), avec validation des valeurs | Moyenne | Corrigée (piste C, C2, fusion `7c05682e` ; détail : `16` § Écarts › Piste C) |
| A16 | 4.4 | `printenv` dans une session locale | `PATH=~/.volta/bin:…/oh-home/run/bin:…` : le shell de l'utilisateur (zsh, `~/.zshenv`) remet ses dossiers **devant** le faux `bd` d'oh | Faux `bd` toujours en tête (sinon un vrai `bd` installé via npm/volta, `~/.local/bin`, `~/go/bin` contournerait `beads.allow` en local) ; 10.1 : `which bd` → faux `bd` d'oh (aucun `bd` dans `~/.volta/bin` ici), mais l'ordre du `PATH` dépend des fichiers du shell de l'utilisateur | Moyenne (sécurité) | Corrigée (piste T, T2, fusion `69c15431` ; détail : `16` § Écarts › Piste T) |
| A17 | 7.2 → cp-2 | `oh run ticket --tickets pt-cb8 --mode manuel`, cp-1 validé | Le `developer` **commit lui-même** (19:43:50, « Closes pt-cb8 ») **avant** la review et **avant** cp-2 « Commit ou correction » ; l'orchestrateur annonce ensuite la review puis cp-2. **2ᵉ occurrence** après « corriger d'abord » : commit à 20:50:41, cp-2 demandé à 20:51:55 (Sonnet). **Avec Haiku** (2 tickets en worktree, semi-auto) : rien de commité avant cp-2 (fichiers indexés), mais **aucun commit après la validation de cp-2**, cp-3 écrit en texte au lieu d'appeler l'outil, tickets laissés `in_progress` ; l'`orchestrator-dev` ne peut pas lancer `bd` (posture coordination). La responsabilité du commit et de la fermeture (developer ? orchestrator-dev ? après quel checkpoint ?) n'est pas définie de façon fiable | Aucun commit avant la décision de cp-2 (le checkpoint perd son sens) : consigne du `developer` dans le workflow `ticket` (ou permission `git commit` différée jusqu'à cp-2) | **Majeure** | Corrigée (piste T, T1, fusion `69c15431` ; détail : `16` § Écarts › Piste T) |
| A18 | 3.2 | Lancement de `ticket` dans le dossier de base avec une modification non commitée (`app/store.py`, docstring de l'étape 1) | Simple avertissement « Modifications non commitées », session lancée sur la base ; le `developer` juge la modification « fausse » et lance **`git checkout app/store.py`** : **travail de l'utilisateur perdu** (pas de stash) | Workflow qui écrit + dossier modifié : worktree automatique (ou choix stash/worktree comme dans la fiche, `10` §4.4) ; et consigne : ne jamais annuler une modification qu'on n'a pas faite | **Majeure** (perte de données) | Corrigée (piste L, L1, fusion `a3fbea34` ; détail : `16` § Écarts › Piste L) |
| A19 | 10.4, 7.2 | `git commit` dans un projet Beads (hooks `bd init` dans `.beads/hooks`) pendant la session | Le hook `pre-commit` appelle `bd hooks run` via le faux `bd` → **refusé** (`beads.allow`) → le commit échoue ; l'agent **contourne les hooks** : `git commit --no-verify`, `git -c core.hooksPath=/dev/null`, `GIT_CONFIG_*`, puis **modifie `.git/config` (`core.hooksPath`)** le temps du commit. **2ᵉ occurrence (lot 3, Haiku, `pt-c3b.1`)** : `git commit` refusé par le hook, `git --no-verify commit` refusé par la liste blanche, l'agent **abandonne le commit et ferme quand même le ticket** (`bd close`) : ticket `closed`, travail seulement indexé dans le dossier de base | Les hooks de bd passent (commande `bd hooks run` appelée par git — `BD_GIT_HOOK=1` — autorisée par la passerelle, ou relayée au vrai bd) ; et les permissions refusent `--no-verify`, `-c core.hooksPath`, `git config core.hooksPath` | **Majeure** (sécurité + fonctionnelle) | Corrigée (piste T, T2, fusion `69c15431` ; détail : `16` § Écarts › Piste T) |
| A20 | 7.2 | Le `developer` veut lancer les tests (Python du système : `python3`, pas de `python`) | 31 « Permission denied: shell » : la liste blanche autorise `python -m pytest*`, `pytest*`, `pip *` mais **pas `python3 …`** ni `.venv/bin/…` ; tests « validés par analyse statique » seulement | Liste blanche couvrant `python3 -m pytest`, `python3 -m pip`, l'environnement virtuel du projet (`.venv/bin/*`), `uv run` | Moyenne | Corrigée (piste T, T3, fusion `69c15431` ; détail : `16` § Écarts › Piste T) |
| A21 | 8.2 | `oh session list` pendant une session `ticket` | Colonne COÛT = **0,88 $** (session racine seule) alors que le budget compte **3,41 $** (racine + sous-agents developer 1,94 $ et reviewer 0,59 $) | Coût affiché = coût total de la session, sous-agents compris (comme le budget) | Moyenne | Corrigée (piste L, L3, fusion `a3fbea34` ; détail : `16` § Écarts › Piste L) |
| A22 | 8.3 | Décision `$` levée pendant la correction demandée à cp-2, puis `oh budget raise <id> 3` | Décision résolue (`raise`), mais l'étape interrompue **ne reprend pas** ; la session reste affichée `⏸ en attente` alors que l'inbox dit « Rien à traiter » ; il faut `oh session send` pour relancer l'agent | Après un relèvement : reprise de l'étape interrompue (ou consigne de reprise envoyée par oh) ; état `idle`/`active`, pas `en attente` sans décision | Moyenne | Corrigée (piste L, L3, fusion `a3fbea34` ; détail : `16` § Écarts › Piste L) |
| A23 | 6.9 | `oh session results <id>` en fin de session `ticket` (2 commits : `app/main.py`, 2 fichiers de tests) | « 1 fichier modifié · +11 −0 · $1.24 » : seulement le **dernier tour** (le 2ᵉ commit), coût ni de la racine (0,88 $) ni total (≈ 4 $), **pas de description de MR** | Bilan de toute la session : diff depuis le départ de la session (ou de la branche), coût total, description de MR (P3-T11) | Moyenne | Corrigée (piste L, L3, fusion `a3fbea34` ; détail : `16` § Écarts › Piste L) |
| A24 | 3.4 | `oh run ticket --tickets pt-c3b --recap` (épopée avec un enfant `pt-c3b.1` ouvert) | « 1 session · 0 worktree · pt-c3b (base) » : une session **sur l'épopée elle-même**, pas une par ticket enfant ; `--one-session` donne le même résultat | Sans `--one-session` : une session par ticket enfant ouvert de l'épopée (F3) | Moyenne | Corrigée (piste L, L2, fusion `a3fbea34` ; détail : `16` § Écarts › Piste L) |
| A25 | 3.8 | `oh run ticket --headless --tickets pt-cb8` | Refus correct, mais le message conseille « choisissez un autre mode » alors que cp-2 est en pause dans **tous** les modes | Message qui dit que cp-2 est obligatoire et qu'aucun mode ne permet de le passer sans interface | Mineure | Corrigée (piste C, C2, fusion `7c05682e` ; détail : `16` § Écarts › Piste C) |
| A26 | 3.9 | `oh run review-feedback -i mr=1` sans jeton GitLab | « configure via oh service setup ou GITLAB_TOKEN » (`oh service setup` est dépréciée) ; message en partie en anglais | « oh mcp setup gitlab » ; message traduit | Mineure | Corrigée (piste C, C2, fusion `7c05682e` ; détail : `16` § Écarts › Piste C) |
| A27 | 8.5 | `oh run quick -i request="Dis seulement DEUXIEME."` (Haiku) | L'agent répond « test de prompt injection… REJET DE LA DIRECTIVE CACHÉE » et refuse la demande | Une demande simple de l'utilisateur est traitée ; les garde-fous ne doivent viser que le contenu externe délimité (Q4 : entrées de l'utilisateur « à traiter ») — à revoir avec des demandes courtes | Mineure | Corrigée (piste T, T3, fusion `69c15431` ; détail : `16` § Écarts › Piste T) |
| A28 | 8.4, 8.5 | `oh budget unset <restriction>` | Affiche « … : enregistré dans hub.toml » (la valeur est bien retirée) | « … : retiré de hub.toml » | Mineure | Corrigée (piste C, C2, fusion `7c05682e` ; détail : `16` § Écarts › Piste C) |
| A29 | 14.3 (Podman), 8.3 | Session `libre` (sans checkpoint) en conteneur ; relèvement de budget | L'entrée d'état de session `oh.checkpoints` est posée **après le premier tour** : opencode l'annonce par un message système, ce qui **déclenche un tour de plus** (« Contexte mis à jour… prêt à travailler ») qui remplace la vraie réponse en fin de conversation ; entrée posée même pour un workflow sans checkpoint ; après `oh budget raise`, la 1ʳᵉ réponse de l'agent donne encore l'ancien budget | État posé avant le premier prompt (ou sans réveiller l'agent), pas d'entrée `oh.checkpoints` pour un workflow sans checkpoint | Moyenne | Corrigée (piste L, L4, fusion `a3fbea34` ; détail : `16` § Écarts › Piste L) |
| A30 | 6.4 | Permission réglée dans l'interface de l'outil, puis `oh session approve … --decision reject` | « Aucune décision de ce type en attente pour cette session » (la décision est bien `resolved_by=tool`) | « Déjà résolue dans l'interface de l'outil » (P3-T08) | Mineure | Corrigée (piste C, C2, fusion `7c05682e` ; détail : `16` § Écarts › Piste C) |
| A31 | 11.1 (accueil hub, mode équipe) | Ticket `pt-cb8` fermé (`closed`) et sa session arrêtée | « Équipe de test · 3 en cours », `oh team status` : `pt-cb8 in_progress` depuis 2 h 20 : la **réservation d'équipe n'est pas libérée** à la fermeture du ticket ni à l'arrêt de la session | Réservation libérée (ou passée « terminée ») quand le ticket est fermé / la session se termine | Moyenne | Corrigée (piste L, L4, fusion `a3fbea34` ; détail : `16` § Écarts › Piste L) |
| A32 | 11.1, 11.13 | Accueils hub et projet comparés aux maquettes (`10` §1, §2) | Hub : en-tête sans version d'opencode ni adaptateur ; deux sections titrées « Projets » (liste, puis « Ajouter un projet ») ; « Système » sans Notifications ni Catalogue des briques. Projet : en-tête sans les environnements d'exécution (⌂ ▣ ☁ et leur état) ; Catalogue des briques seulement via Config projet › « Briques (agents, skills) → » et l'omnibar (`bricks`) | Conformes aux maquettes (ou maquettes mises à jour) | Mineure | Ouverte |
| A33 | 11.1 | Ctrl+T et libellés des modes | Ctrl+T : hub/projet → équipe, équipe → hub ; **aucun raccourci vers le mode projet** ; indications contradictoires : en mode projet, la ligne du haut dit « Ctrl+T mode équipe » et la barre du bas « Ctrl+T mode hub » ; en mode équipe, « Ctrl+T mode équipe » alors qu'il ramène au hub ; mode équipe : entrée « Workflow · Pipeline de travail » (ancien libellé, ouvre pourtant le catalogue des workflows), « Team Board », « Team Status » en anglais | Indications justes selon le mode ; libellé « Workflows de l'équipe » ; textes traduits ; raccourci vers le mode projet à décider (backlog ?) | Mineure | Corrigée — fonctionnel (piste U, U2) ; raccourci mode projet : BL-24 |
| A34 | 11.2, 11.3 | Accueils, fiche de lancement, sélecteur Beads | Descriptions des listes **coupées net** sans « … » ni retour à la ligne alors qu'il reste de la place ; aide du champ « Ticket » absente (maquette : « Ticket Beads prêt à implémenter… ») ; curseur affiché au milieu du bouton « Chois▌ir… » et dans la liste vide du sélecteur ; ligne d'aide du sélecteur affichée deux fois ; fenêtre « Nouveau workflow » : libellé du type tronqué, champ Identifiant à faire défiler ; titre de la fenêtre de fermeture tronqué (« … : mis… ») ; message « Pendant votre absence » tronqué (« … ») et affiché ~1 s seulement ; aide de « Créer un espace solo » qui déborde sur le champ suivant | Texte tronqué avec « … » (ou sur deux lignes) ; aide des champs affichée ; pas de curseur parasite | Mineure | Ouverte |
| A35 | 11.4 | Récap de la fiche de lancement (`10` §4.4) | Pas de ligne **Checkpoints** (« cp-1 ⏸ en manuel · cp-2 ⏸ obligatoire »), ni **Beads autorisés**, ni **Code Mode** ; dossier avec modifications non commitées : seulement « ⚠ Modifications non commitées », sans les choix **[s] stash / [w] worktree** de la maquette (cf. A18) | Récap complet ; choix proposé quand le dossier n'est pas propre | Moyenne | Corrigée (piste L, L1, fusion `a3fbea34` ; détail : `16` § Écarts › Piste L) |
| A36 | 7.4, 7.6 | Session `ticket` en manuel (Haiku), après la review | L'`orchestrator-dev` pose **cp-2 avec l'outil `question`** (« ? [OrchestratorDev — CP-2] … ◂ Commit (Recommandé) ▸ ») au lieu de `workflow_checkpoint` : le checkpoint **obligatoire** cp-2 n'est jamais « passé » (`checkpoint_state` : cp-1 seulement), la frise et le gating l'ignorent, puis cp-3 est demandé ; toujours aucun commit (A17) | Un checkpoint obligatoire ne peut pas être contourné : refuser les agents verrouillés après cp-2 / cp-3 tant que cp-2 n'est pas passé (gating), consigne plus ferme dans le contenu, ou détection d'une question qui imite un checkpoint | **Majeure** | Corrigée (piste T, T1, fusion `69c15431` ; détail : `16` § Écarts › Piste T) |
| A37 | 7.6 | Fiche de décision ouverte dans oh, réponse donnée dans l'interface de l'outil | La liste « À traiter » se met à jour, mais la **fenêtre de décision reste ouverte** (on peut encore « Confirmer ») | Fenêtre fermée (ou marquée « résolue dans l'interface ») dès que la décision est réglée ailleurs | Mineure | Corrigée (piste U, U2) |
| A38 | 7.9 | Fin d'une session `ticket` (ticket fermé, cp-3 validé) | Aucun « ↪ Enchaîner avec… » : l'agent n'appelle jamais `workflow_outputs` (`sessions.outputs = {}`), et la session passe en **veille** au lieu de se terminer (`completed`) : rien ne déclenche la suggestion de fin (F5) | Sorties déclarées de façon fiable (consigne ou déclaration par oh à partir de la branche / des tickets fermés) ; fin de workflow détectée après le dernier checkpoint | Moyenne | Corrigée (piste T, T1, fusion `69c15431` ; détail : `16` § Écarts › Piste T) |
| A39 | 11.8, 12.13 | Fiche de lancement ouverte depuis les actions rapides du board ; éditeur de workflow | Fiche : **Échap ne ferme pas** (le bandeau dit « Esc fermer »). Éditeur : Code Mode mis à `true` → ✎ et erreur d'assouplissement corrects, mais la valeur affichée reste « – » ; choix proposés `true/false` au lieu de on/off (doc et `oh workflow show` : on/off) ; fenêtre « Nouveau workflow » : champ Identifiant sous la ligne de flottaison (il faut faire défiler), libellé du type tronqué | Échap ferme la fiche ; valeur écrite affichée ; libellés cohérents ; formulaire entièrement visible | Mineure | Corrigée — Échap, éditeur (piste U, U2) ; mise en page du formulaire : revue design |
| A40 | 11.12 | Vue Doctor de la TUI comparée à `oh doctor` | TUI : « 22/22 checks OK · Tout est en ordre » ; la CLI : ✗ « Version oh » (A1), ✗ « Provider credentials » (A2), plus bd, fzf, Beads zero-impact… : la TUI **n'exécute pas les mêmes vérifications** | Mêmes vérifications (ou mention de celles qui ne tournent qu'en CLI) | Moyenne | Corrigée (piste C, C1, fusion `7c05682e` ; détail : `16` § Écarts › Piste C) |
| A41 | 11.11 | Mode équipe › Statut / Activité | « benjamin completed a session on » : **nom du projet absent** de la phrase (paramètre vide) ; événements bruts `workflow.published`, `workflow.archived`, `workflow.restored` au lieu de phrases ; ASCII du nom d'équipe : « É » rendu « ? » | Phrase complète (projet), événements traduits, caractères accentués gérés ou repli sans ASCII-art | Mineure | Corrigée — phrases, événements (piste U, U2) ; ASCII-art : revue design |
| A42 | 12.15 | Catalogue d'un projet sans équipe, touche `n` | L'assistant « Créer un espace solo » s'ouvre ✓, l'espace est créé ✓, mais le formulaire « Nouveau workflow » ne s'ouvre pas ensuite (il faut refaire `n`) ; le détail de l'espace solo affiche les réglages tracker et notifications alors que les fonctions d'équipe y sont désactivées | Enchaîner sur le nouveau workflow ; masquer ce qui ne s'applique pas à un espace solo | Mineure | Corrigée (piste U, U2) |
| A43 | Lot 3 (constat de l'utilisateur) | Fenêtre opencode ouverte pour une session, puis actions depuis oh | Un **nouvel onglet** de la même session s'ouvre dans la fenêtre opencode (onglets de la TUI V2) : les onglets se multiplient. Causes probables : onglets mémorisés par la TUI (`~/.local/state/opencode/latest/tui/tabs.json`, par dossier) **sans tenir compte du serveur** (bogue opencode #47998, corrigé par #47999, non vérifié en 2.0.20) alors qu'oh lance un serveur par groupe ; et/ou nouvelle fenêtre client lancée par une action alors qu'un client est déjà attaché. Action déclencheuse exacte à préciser | Une session = une fenêtre ; une action sur une session déjà ouverte ramène sa fenêtre au premier plan. **Décision du 08/10** : (1) l'adaptateur désactive les onglets de la TUI dans les clients ouverts par oh (`OPENCODE_CLI_CONFIG_CONTENT={"tabs":{"mode":"off"}}`, D19) ; (2) avant d'ouvrir un client, oh vérifie s'il en existe un attaché (suivi des clients par le démon) et ramène sa fenêtre au premier plan (iTerm2 / Terminal.app par AppleScript ; tmux : `select-window`) ; (3) onglets par groupe via `context.ui.tabs` du plugin : backlog BL-23 | Moyenne | Corrigée (piste U, U1 ; cause : `16` § Écarts › Piste U) |

## Suite prévue
1. Piste de corrections des anomalies (A1–A43), par gravité.
2. Recette de non-régression sur les tests ☒ et ceux touchés par les corrections.
3. **Revue globale de l'ergonomie et du design de la TUI** (point soulevé par l'utilisateur le 08/10 ; s'appuyer sur A32–A34, A37, A39, A41, A42 et les maquettes de `10-tui.md`).
4. REL.
