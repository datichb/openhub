# 12 — Backlog et ADR à rédiger

## Backlog (hors phases 0–5)

| ID | Élément | Notes |
|---|---|---|
| BL-1 | Éditeur de prompt **intégré** à la TUI | `tview.TextArea`, coloration et autocomplétion des variables `{{…}}`, aperçu en direct, validation ; `$EDITOR` en secours |
| BL-2 | Gouvernance avancée de la publication | lead seul, N approbations ; le champ `governance.publish` existe dès la phase 2 |
| BL-3 | `oh serve` (dashboard web oh) | Exposer les services : décisions, suivi, propositions |
| BL-4 | Filtrage du réseau sortant des conteneurs | Liste blanche (LLM via proxy, forge git, registres) ; le proxy d'identifiants est déjà en phase 0 |
| BL-5 | Protection côté serveur de `team-state` | Branche protégée ou commits signés (`git verify-commit`) |
| BL-6 | Adaptateurs pour d'autres outils | Claude Code, outil maison ; `Capabilities()` partielles gérées par l'UI |
| BL-7 | Workflows multi-dépôts | Une session sur plusieurs dépôts (V2 : locations multiples, `references`) |
| BL-8 | IDE via ACP | `opencode acp` (Zed, JetBrains) |
| BL-9 | Suivi en direct d'une session distante | Tunnel vers le serveur du runner |
| BL-10 | Espace tmux géré par oh (M3) | Tableau de bord fenêtre 0, une fenêtre par session |
| BL-11 | Actions depuis la notification système | Boutons (valider/refuser) |
| BL-12 | Pipelines de workflows | Enchaînements déclaratifs automatiques (au-delà de « Enchaîner avec… ») |
| BL-13 | Prise en charge WSL | Conteneur/distant sous Windows |
| BL-14 | Mesure réelle du prompt système (O19) | `oh bundle show --measure` : serveur + plugin de sonde (`02` R5) + prompt minimal (coût LLM, confirmation) ; aujourd'hui estimation ≈ 4 caractères/token (piste F, décision du 06/10) |
| BL-15 | Invitation des membres d'une équipe | `oh team promote` (phase 2) affiche seulement l'URL et `oh team init` ; inviter depuis oh (forge : membres du projet GitLab, `members.toml` prérempli) |
| BL-16 | Impact : permissions propres des agents | Le résumé d'impact de la publication (phase 2) ne compare pas les permissions d'un agent entre deux versions (catalogue de briques versionné nécessaire) |
| BL-17 ✅ (piste R, lot R6) | Suivi distant par le démon | Crochet périodique du démon (piste R) appelant `remotesvc.Service.Track` : notification « prête à récupérer » oh fermé, entretien du bail Beads (`bd heartbeat`) pendant le pipeline (phase 5, `09` § Lot 5.C) |
| BL-18 | MCP à jeton dans les jobs distants | Serveurs MCP oh qui lisent un jeton dans le trousseau de la machine (gitlab, jira, figma…) : variables CI dédiées ou passerelle vers la machine (phase 5) |
| BL-19 | Build args de l'image distante | Transmettre les build args de la config Exécution (4.C) à la construction de l'image du job, sans secret dans la page du pipeline (phase 5) |
| BL-20 ✅ (Q-bis, Q3-5 : touche `$` de la vue Sessions) | Relever un budget depuis la TUI | Touche « relever » (montant) et « arrêter » sur une décision `$` des restrictions dans la vue Sessions ; aujourd'hui `x` classe (une étape de plus) et `oh budget raise` relève (piste R, lot R4) |
| BL-21 | Budgets I6 dans les jobs distants | `oh runner` démarre son démon sans registre d'usage ni capacité : brancher `Options.Usage` et transmettre les restrictions résolues dans l'enveloppe (phase 5, piste R) |
| BL-22 | Notificateur embarqué (macOS) | Mini-application `oh-notifier.app` (Swift, `UserNotifications`) écrite par oh dans `~/.oh/run/`, signée et notarisée (compte Apple Developer) : notifications au nom d'oh, clic qui ramène oh, **boutons d'action** (prérequis de BL-11) ; en attendant, `terminal-notifier` en dépendance recommandée de la formule Homebrew (avec `opencode` en dépendance obligatoire, absente aujourd'hui) — décision du 07/10 |
| BL-23 | Onglets opencode par groupe de serveur | Une fenêtre opencode par groupe, un onglet par session : « attacher » ouvre/active l'onglet dans la fenêtre existante via le plugin (`context.ui.tabs.open/focus`), exposé comme capacité de l'adaptateur (D19), avec repli « une fenêtre par session » ; après la correction opencode #47999 (onglets mémorisés par serveur) — A43, décision du 08/10 |
| BL-25 | Colonne « review » de `oh board` par label | `oh board` (`cmd/board.go`, `normalizeStatus`) range les tickets par statut Beads ; bd 1.3 n'a pas de statut `review` (T5 : label `ready-for-review` sur un ticket `in_progress`). Ranger dans la colonne « review » un ticket qui porte ce label (le board d'équipe de la TUI, fondé sur les réservations, n'est pas concerné) — décision du 08/10 |
| BL-24 | Raccourci vers le mode projet | Ctrl+T ne bascule qu'entre hub et équipe ; un raccourci (ou une bascule à trois états) vers le mode projet, avec sélection du projet si plusieurs — A33 |

## ADR à rédiger (dans un second temps, format du dépôt `docs/architecture/adr/NNN-*.{fr,en}.md`, à partir de 038)

> ✅ **Rédigés** par la piste ADR de la vague 4 (038 → 048, fr et en), fusion dans `main` `f4d9dc8d`. Détails et points à trancher : `14-finalisation.md` § Écarts › Piste ADR.

| # | Titre proposé | Contenu | État |
|---|---|---|---|
| 038 | Modèle neutre `SessionSpec` et adaptateurs d'outil | D2, D3, O3, F1–F7, remplace partiellement ADR-036 | ✅ `038-sessionspec-tool-adapters` |
| 039 | Workflows déclaratifs `oh/v1` | Schéma, couches, validation, conductor (D5, D6, O1, O7, O9, O11) | ✅ `039-declarative-workflows-oh-v1` |
| 040 | Workflows dans team-state et gouvernance dans le hub | D7, O12, O14, solo | ✅ `040-workflows-team-state-governance` |
| 041 | Monde fermé et vérification d'isolation | D13, F5, F21, plugin | ✅ `041-closed-world-isolation` |
| 042 | Checkpoints à trois niveaux et décisions headless | D8, S3–S6, F17, F26 | ✅ `042-checkpoints-headless-decisions` |
| 043 | Paquet de session et suppression du déploiement par projet | D4, D14, migrate deploy-cleanup | ✅ `043-session-bundle-deploy-removal` |
| 044 | Proxy d'identifiants LLM | O4, F14–F16 (+ restrictions I6) | ✅ `044-credential-proxy-session-limits` |
| 045 | Environnements d'exécution (local, conteneur, distant) | D11, D12, phases 4–5 | ✅ `045-execution-environments` |
| 046 | Beads sur la machine et passerelles | D9, faux bd, instantané/journal | ✅ `046-beads-gateways` |
| 047 | Interaction avec les sessions et multi-session | I1–I7, groupes de serveurs, veille | ✅ `047-session-interaction-daemon` |
| 048 | Abandon d'opencode V1 (ajouté en vague 4) | D3 révisé (06/10) | ✅ `048-opencode-v1-abandonment` |
| — | Mettre à jour/archiver ADR-010 (hybrid skills, encore bash), ADR-012 (worktree), ADR-036 | B15 | ◐ 010 : « Évolué par 043 » ; **012 et 036 à trancher** (`14` § Écarts › Piste ADR) |
