# Entrées CHANGELOG proposées par les lots de la v5.0 (22/10/2026)

> Rassemblées par l'orchestrateur à partir des récapitulatifs des sous-sessions. Source pour V3-A (CHANGELOG `[5.0.0]`) et V3-B (MIGRATION, guides de migration, SECURITY). À vérifier contre le code et `inventaire.md` (même dossier) : l'inventaire a été écrit **avant** V2-A, V2-D et les lots X ; les entrées ci-dessous décrivent l'état final de ces lots.

## Modernisation des tests (déjà dans `[Unreleased]`, `27f3e6b9`)
Déjà rédigée en tête de `[Unreleased]` (sections « Fixed — Bugs trouvés en simplifiant la suite de tests » et « Changed — Suite de tests plus courte et plus rapide »). À intégrer à la synthèse.

## V1-A — S-38, empreinte d'opencode 2.0.25
- Sécurité (S-38) : l'empreinte des paquets Linux d'opencode **2.0.25** (la version de la CI) est épinglée dans oh, avec la 2.0.20 ; `.github/scripts/opencode-pin.sh <version>` (lecture seule) vérifie les quatre paquets contre le registre npm et affiche le fragment à ajouter à `pinned_integrity.json`. `compatibility.json` porte une version recommandée (`opencode_recommended` : 2.0.25) ; un test vérifie qu'elle est épinglée et égale à celle de la CI.
- `oh doctor` avertit quand la version locale d'opencode n'est pas épinglée (les images de conteneur la téléchargeraient sans empreinte vérifiée ; `execution.strict_isolation` la refuse).

## V1-B — Mode équipe en bêta
- ⚠ Le mode équipe est publié en **bêta** : avertissement unique par processus sur stderr (commandes `oh team …`, premier claim pris au lancement), avertissement dans `oh doctor` (sans effet sur le code de sortie), ligne « Bêta » en tête du board d'équipe et de la configuration d'équipe. Limites connues dans `docs/guides/team-setup.fr.md#limites-connues` : claim libéré à l'arrêt même avec `cp-2` en attente, pas d'expiration (reprise manuelle après `takeover.stale_days`), libération perdue hors ligne ou si le démon s'arrête (`oh team release`), orphelins signalés par `oh doctor`. Correction complète prévue en 5.1.
- Fixed : la colonne REVIEW de `oh board` et du Board TUI n'était jamais remplie ; un ticket Beads `in_progress` portant le label `ready-for-review` y est affiché (`beads.BoardColumn`).
- Fixed : vote de review MCP (`team_review_verdict`) : une approbation depuis `in_progress` passe par `review` puis `done` ; un échec de transition est remonté à l'agent (le vote n'est alors ni enregistré ni labellisé).

## V1-C — Politiques, configuration, publication
- Fixed : vue Politiques : ajout, édition, activation, suppression ne bloquent plus l'interface (pull/push hors de la boucle TUI, erreurs en toast). `policies.toml` écrit par l'encodeur TOML sous le verrou du team-state ; plus de fichier invalide avec des guillemets ou caractères spéciaux. `oh policies add` utilise le même service (`internal/services/policy`).
- ⚠ `policies.toml` est réécrit en entier à chaque modification (CLI ou TUI) : commentaires et mise en forme à la main perdus.
- Security : `./hub.toml` n'est plus lu (sans `~/.oh/hub.toml`, oh lisait un `hub.toml` du répertoire courant : un dépôt cloné pouvait injecter une configuration). Seul le dossier du hub (`~/.oh` ou `OH_HOME`).
- Publication : un tag `v5.0.0-rc.N` est une pré-version GitHub (jamais « latest ») et ne met pas à jour Homebrew ; notes de version tirées de la section correspondante de `CHANGELOG.md` (`.github/scripts/release-notes.sh`), échec si absente.

## V2-A — Confirmation unique, jokers
- ⚠ Rupture — confirmation sans terminal : une action qui demande confirmation est **refusée** quand oh n'a pas de terminal (script, CI, entrée redirigée), sauf `--yes` (`-y`) ; message « confirmation requise : relancez avec --yes », code de sortie **2**. Avant : `oh workflow publish|archive|restore` et `oh brick publish` répondaient « oui » seuls ; plusieurs commandes refusaient en silence. Commandes : `workflow publish|archive|restore`, `brick publish`, `purge`, `project remove`, `teams remove`, `service remove`, `worktree cleanup`, `session delete|resolve`, `migrate deploy-cleanup`, `run --recap` (et `start --recap`). `--yes` ajouté à `run`, `purge`, `project remove`, `teams remove`, `service remove`, `worktree cleanup`. Avec `purge`, `--yes` ne propose pas la sauvegarde.
- Boutons des confirmations traduits (Oui/Non).
- Fixed — jokers des permissions (F-12) : oh évaluait les règles par octet (`?` ne correspondait pas à un caractère accentué) alors que l'outil les lit par caractère. Paquet commun `internal/wildcard` (limites, proxy, règles de permission) ; la détection des commandes de vérification dans les traces accepte `?`.

## V2-B — Chargements asynchrones
- Fixed : TUI — Historique, Métriques, Doctor et Nettoyage n'appliquent plus une réponse obsolète (deux `r` rapides, retour sur la vue, plan recalculé pendant une suppression). Métriques ne lit plus son état depuis une goroutine. MCP et Réglages ne gèlent plus l'interface au montage (trousseau et config d'équipe lus hors de la boucle).

## V2-D + X1 — Garde des agents
- Security : le garde global du shell des agents refuse `oh workflow publish|archive|restore`, `oh brick publish`, `oh purge`, `oh project remove` (alias `rm`, `p`), `oh teams remove|archive|restore`, `--yes` compris. Lectures (`list`, `show`, `diff`) permises.

## X1 — Sécurité et petits bugs
- Security (S-20) : une couche de **projet** ne peut plus ajouter d'agent ni de `skills.extra` à un workflow hérité (erreurs `patch_agent_refused`, `patch_skills_extra_refused`) ; la couche d'équipe garde l'avertissement. ⚠ Rupture : déclarer l'agent ou le skill dans le workflow d'équipe, puis l'étendre.
- Security (S-11) : les plugins de la configuration personnelle de l'outil sont signalés au lancement (message explicite) et par `oh doctor` ; `execution.strict_isolation` les masque. Défaut inchangé.
- Fixed (F-07) : `review-feedback` sur un projet GitHub : les entrées `mr.*` annoncent qu'elles ne lisent que les merge requests GitLab.
- Fixed : vue MCP : course de données sur la configuration lors de la lecture des URL imposées par l'équipe.

## X2 — Garde-fous
- Changed : chaque migration SQLite (v24 à v34 comprises) a un retour arrière ou est irréversible ; test d'aller-retour. Revenir en arrière = restaurer la sauvegarde (`oh repair --restore`).
- Tests : règles de couches avec liste figée des écarts ; exemption des tests de contrat calculée sur la contrainte de build ; nouveaux tests MCP Figma et Google Slides, format de fil de `bd`, registre d'adaptateurs, résultats de session, `oh purge`, `oh remote setup`.

## X3 — Aide
- Changed : `oh --help` : une ligne par commande de premier niveau (≈ 70 lignes au lieu de 573), section « Avancé » (`bundle`, `fanout`, `migrate`, `takeover-brief`, `completion`). `oh help --all` (ou `oh --help --all`) : affichage complet d'avant ; `oh <commande> --help` inchangé. Un script qui analysait `oh --help` passe à `oh help --all`. `oh --all` seul affiche l'aide complète (au lieu d'ouvrir la TUI).
- Changed : vue Sessions : la ligne d'aide garde les 8 touches essentielles et `? aide` ; `?` liste toutes les touches.
- `oh teams` (équipes de cette machine) et `oh team` (équipe active) distinguées dans l'aide et la doc.

## X4 — Configuration
- ⚠ Rupture — `hub.toml` lu strictement : chaque valeur doit avoir le type de son réglage. Refusé désormais, avec fichier, ligne, colonne et clé : `"true"`, `"1"` ou `1` pour un booléen ; nombre entre guillemets ; décimal pour un entier ; nombre ou booléen pour un texte ; texte pour une liste (`"A,B"`). Un `hub.toml` écrit par oh reste valide. `oh doctor` affiche le même message ; `oh doctor --fix` restaure la dernière sauvegarde qui se charge.
- ⚠ Les clés des tables `remote.projects` et `binaries` gardent leur casse (elles étaient passées en minuscules).
- Une clé inconnue est ignorée avec un avertissement (perdue à la prochaine écriture) ; `oh doctor` la liste. `oh workflow migrate` doit tourner avant toute écriture s'il reste une section `[workflow]`.
- Anciennes clés (`opencode.default_provider`, `execution.opencode_version`, noms de trousseau `<service>-token`) signalées une fois par commande ; plus lues en v6.
- `hub.toml` écrit de façon atomique et durable. Retrait de viper. Page « Où se règle quoi ».
- Aucune variable d'environnement ne règle `hub.toml` (inchangé).

## X5 — Écritures atomiques
- Security/robustesse : écritures atomiques durables (temporaire unique, fsync, rename ; `internal/fsutil`) : secrets, index du trousseau, mots de passe des serveurs, droits de la passerelle, environnement du shell, état de synchronisation, rapports, wiki, transcript, config d'équipe, import d'une sauvegarde, shim `bd`, historique des notifications, config opencode du projet (`deploycleanup`). Plus de `.tmp` à nom fixe.
- Security : les serveurs MCP Figma et Google Slides refusent un `file_key`, `presentation_id` ou `slide_id` qui n'a pas la forme d'un identifiant (`..`, `/`, `?`, `#`…).
- Security : un identifiant de session contenant `../` ou un séparateur est refusé par les résultats de session.

## X6 — Lancement, coûts, vocabulaire
- Added : `oh run --dry-run` : prépare (workflow, paquet de session, plan) et affiche récapitulatif, pauses et avertissements, sans démarrer de session, créer de worktree, réserver de ticket ni poser de question ; refusé en distant. Seule écriture : le paquet de session (nettoyé par `oh gc`).
- Added : `oh budget report [--by project|workflow] [--since 30d] [--json]` ; sessions supprimées sous « inconnu ».
- Added : `free` alias du workflow `libre`.
- Changed : `manual` accepté partout où l'on saisit un mode, lu comme `manuel`.
- Changed : commandes TUI `coder` / `coder.agent` renommées `libre` / `libre.agent` (anciens noms en alias).
- Changed : vocabulaire : « mode » ne désigne plus que le mode de workflow (type d'agent, vue Hub / Projet / Équipe, type d'installation de `oh init`).
- « Annuler une session » = `oh session delete` (documenté).

## X7 — Service équipe
- ⚠ Jeton GitLab : un seul ordre de recherche (sync du tracker, `oh mr create` / `oh review --publish`, `oh review feedback` / `review-feedback`, vérification d'identité de `oh team init|rejoin`, `oh mcp enable --project`) : `GITLAB_TOKEN`, `tracker_token_key`, jeton du projet, jeton de l'équipe, jeton MCP du hub (clé de `[mcp.gitlab] token`, puis `openhub.mcp.gitlab.token`, puis `gitlab-token`), ancienne clé `openhub.tracker.gitlab.token`. Pour `oh mr create`, le jeton de l'équipe du projet est trouvé et passe avant le jeton MCP du hub ; le tracker trouve les jetons d'équipe et MCP sans `tracker_token_key`.
- ⚠ La TUI ne synchronise plus un tracker désactivé (`[tracker] enabled = false`).
- `oh mr create` lit le projet GitLab et le relecteur dans l'équipe du projet (repli sur l'équipe du hub).
- Nouveau service équipe (`internal/services/team`) ; l'étape « politiques » des assistants d'équipe écrit en un seul commit via le service des politiques (message `policies: init|update`).

## X8 — `oh init`
- Changed : `oh init` en 3 écrans en solo (8 avant) : accueil (langue et type d'installation), fournisseur, premier projet (facultatif, prérempli avec le dossier courant). Type par défaut « solo » (ou « complet » si une équipe est configurée) ; équipe : 4 écrans, complet : 5. ⚠ Les intégrations MCP ne sont plus demandées par l'assistant : le récapitulatif suggère `oh mcp setup`. Description du type d'installation visible même sur un petit terminal.

## X9 — `--yes`
- Deprecated : `--force` / `-f` ne sert plus à sauter une confirmation : `--yes` / `-y`. Sur `oh purge`, `oh project remove`, `oh teams remove`, `oh service remove`, `--force` et `-f` restent acceptés une version (même effet, avertissement, absents de l'aide), retirés en v6. `--force` subsiste là où il outrepasse une sécurité : `oh daemon stop`, `oh remote setup`, `oh worktree remove`, `oh worktree cleanup` (ce dernier supprime aussi les worktrees modifiés).

## Points pour SECURITY / MIGRATION (V3-B)
- Limites assumées : S-11 (isolation stricte non par défaut ; plugins seulement signalés), S-12 (`make`/`npm run`), S-13 (jeton MCP par groupe), S-15 (bash interactif), S-20 (couche d'équipe : avertissement seulement), S-39 (Cosign facultatif). Mode équipe bêta, distant expérimental, fan-out expérimental, Windows moins vérifié (27 paquets exclus de la CI).
- Empreinte opencode 2.0.25 épinglée (2.0.20 aussi).
- Ruptures 4.2.0 → 5.0 les plus importantes non couvertes aujourd'hui (voir `inventaire.md` § 7 et § 9) : `execution.env_allow` (`SSH_AUTH_SOCK`, `GH_TOKEN` ne passent plus aux agents), codes de sortie, options courtes, touches TUI, `oh mcp trust`, `./hub.toml`, base de données (sauvegarde automatique, retour = restauration), `oh config model -j` et `oh purge --include-opencode` supprimés sans alias, `oh service` déprécié, plus toutes les ruptures des lots ci-dessus.
