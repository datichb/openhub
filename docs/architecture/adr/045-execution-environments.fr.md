> [Read in English](045-execution-environments.en.md)

# ADR-045 — Environnements d'exécution : local, conteneur, distant (GitLab CI)

## Statut

Accepté

## Date

2026-10-05

## Contexte

Jusqu'à la v4, une session tournait toujours sur la machine, sous le compte de l'utilisateur, avec ses outils et sa configuration. Trois besoins sont apparus :

- **un environnement reproductible par projet**, celui du Dockerfile de développement, où le shell de l'agent n'a accès ni au reste de la machine ni au démon ;
- **lancer un workflow autonome sans garder la machine allumée** (ticket, review, audit), sur l'infrastructure de l'équipe ;
- **garder les secrets et Beads sur la machine** (D9, D10).

Décisions D11 (local ou distant au choix à chaque lancement, distant = runners GitLab CI), D12 (conteneurs OCI via Colima ou Podman, une image par projet, paquet monté en lecture seule) et O16 (conteneur et distant sur macOS et Linux ; Windows en local seulement). L'équipe n'avait pas de runners : la phase 5 les met en place.

## Décision

### 1. Interface commune (`internal/runtime`)

- `ohruntime.Runtime` : `Kind`, `Available` (raisons traduites), `Prepare` (image, montages, réseau, puis `Prepared` avec la traduction de chemins machine ↔ environnement), `Command` (commande machine non démarrée : l'adaptateur garde la main sur le démarrage), `HostAddress`, `Teardown`. Interface facultative : `Estimator` (préparation estimée sans rien construire).
- Le runtime fait partie de la clé de groupe.
- Choix à chaque lancement (`oh run --runtime local|container|remote`, option de la fiche), dans cet ordre : option > config Exécution du projet > Réglages > défaut du workflow. Le choix est borné par `runtime.allowed`.

### 2. Local

`opencode serve` tourne sur la machine. Les variables utiles de la machine (`PATH`, `HOME`, outils, jamais de secret) sont reposées dans l'environnement de session, parce qu'opencode 2.0.20 remplace l'environnement du shell.

### 3. Conteneur (`internal/runtime/container`)

**Moteur.** CLI OCI seulement, pas d'API Docker. `auto` essaie Colima, puis Podman, puis Docker (réglage `[execution] engine`).

**Image en deux étages**, étiquetée par le hash de son contenu :

- `oh-base/<projet>` : le Dockerfile de développement du projet (détecté ou configuré, avec ses build args). Sans Dockerfile, une base oh par défaut (`debian:bookworm-slim` + git, ca-certificates, ripgrep).
- `oh-dev/<projet>` : une couche fine qui ajoute opencode Linux à la version de l'adaptateur (paquet npm vérifié par son intégrité sha512) et le faux `bd`.

Deux images sont gardées par projet et par étage. La durée de construction est estimée à l'avance, et la progression s'affiche dans la fiche.

**Montages** à chemins fixes, traduits dans les deux sens :

- paquet sur `/opt/oh/bundle`, en lecture seule ;
- données du groupe sur `/opt/oh/data` ;
- projet et worktrees sur `/work/<nom>` ;
- `HOME` sur un volume propre au projet : la config de l'utilisateur n'est jamais visible.

Le dossier git commun d'un worktree est aussi monté à son chemin machine, et git est configuré avec `safe.directory=*` et `gc.worktreePruneExpire=never`. Un montage hors des dossiers partagés avec la VM est refusé (sinon Colima monterait un dossier vide en silence).

**Utilisateur, environnement, réseau, processus.**

- Utilisateur : `--userns=keep-id:uid=…,gid=…` (Podman sans root), sinon `--user`.
- Environnement : passé par `--env-file` (0600) et minimal, sans aucune variable de la machine. L'identité git de la machine est passée en environnement de session.
- Réseau : port publié sur `127.0.0.1` uniquement. La machine est jointe par `host.docker.internal`, `host.lima.internal` ou `host.containers.internal` ; sous Linux, par une seconde écoute du proxy.
- Processus : `run --rm --init`. `Teardown` supprime le conteneur, aussi à la veille décidée par le démon. Le démon reste actif pendant une longue construction d'image.

**Groupe déjà occupé.** Un groupe occupé qui ne voit pas un nouveau dossier passe à un groupe voisin (`-s1`, `-s2`…).

**Configuration Exécution** du projet (migration v41 : Dockerfile, build args, volumes, workflow et runtime par défaut) et Réglages `[execution]` (runtime, moteur, images gardées, version d'opencode figée, isolation stricte). Doctor vérifie le moteur, virtiofs, keep-id, les dossiers partagés, opencode dans l'image et la joignabilité du proxy et des passerelles depuis un conteneur.

### 4. Distant (GitLab CI)

**Mise en place.** Un projet central **`oh-runner`** par groupe GitLab, aucune modification de la CI des projets. `oh remote setup|status` vérifie l'accès, crée le projet, protège sa branche par défaut, écrit le `.gitlab-ci.yml` généré (schéma versionné), crée le déclencheur et les variables masquées (clé LLM, jeton par projet cible, jeton team-state), et contrôle les runners. Les cibles sont déclarées dans `[[remote.targets]]`.

**Envoi** (`oh run … --runtime remote`) :

1. Validation : `remote` est dans `runtime.allowed`, et aucun checkpoint qui attend l'utilisateur dans le mode n'a `remote: forbid`.
2. Vérifications : branche poussée, Dockerfile commité, binaire oh disponible.
3. Réservation : `bd update --claim` et claim team-state.
4. Instantané Beads.
5. Paquet (`oh-bundle/<hash>`) et enveloppe de session (`oh-session/<sha256>` : manifeste, entrées, prompt, politique des checkpoints, instantané) dans le registre de packages. Les entrées n'apparaissent pas dans la page du pipeline.
6. Déclenchement et référence enregistrée dans `sessions.remote_ref` (migration v39).

**Job** (`oh runner run`) :

- trois jobs : `oh-cli`, `oh-image` (Kaniko par défaut, ou Docker-in-Docker) et `oh-run` ;
- le démon, le proxy et les passerelles d'oh tournent dans le processus du job, et la clé LLM vient d'une variable CI ;
- les variables secrètes sont retirées de l'environnement avant le démarrage, et opencode tourne sous un compte `oh` (uid 10001) ;
- **répondeur de politique**, jamais `--auto` : un checkpoint `auto` est validé ; `defer`, un checkpoint inconnu ou une question arrêtent proprement le travail ; toute autre permission est refusée ;
- la branche est poussée avec une MR en brouillon (`git push -o merge_request.create`) ;
- artefacts : `journal.jsonl`, `summary.json`, `session.export` (session et sous-sessions).

**Retour** :

- suivi du pipeline, aussi par le démon quand oh est fermé ; notification « À récupérer » ;
- `oh session fetch` : artefacts, puis import de la session dans un serveur local, sur le worktree de la branche poussée ; la session est reprenable ;
- `oh session resolve` : rejeu du journal Beads ([ADR-046](./046-beads-gateways.fr.md)) ;
- TUI : option ☁ de la fiche, section « À récupérer », fenêtre de conflit.

### 5. Windows

En local seulement, avec le démon dans le processus d'oh. Conteneur et distant ne sont pas pris en charge.

## Conséquences

### Positives

- Un seul chemin de lancement pour les trois environnements : même paquet, même monde fermé, mêmes checkpoints.
- En conteneur, le shell de l'agent n'a accès ni au socket du démon ni à `oh.db` (vérifié), et aucun secret n'est présent ; l'environnement est celui du projet, reproductible par hash.
- À distance, la machine peut être éteinte, aucune demande n'est approuvée à l'aveugle, et la session se reprend en local.

### Négatives / Compromis

- **Le distant n'a pas été validé en réel** (aucun runner pendant le build) : téléchargement des artefacts GitLab, import d'une vraie session, Kaniko sur un projet privé, base musl.
- Limites du job distant :
  - les MCP qui lisent un jeton dans le trousseau de la machine (gitlab, jira, figma…) n'y sont pas disponibles (BL-18) ;
  - les build args ne sont pas transmis (BL-19) ;
  - pas de budgets I6 (BL-21) ni de suivi en direct (BL-9) ;
  - clé API seulement ;
  - un checkpoint différé n'est pas recréé comme décision après l'import : l'agent le redemande à la reprise.
- Conteneur :
  - validé en réel sous macOS (Colima et Podman) ; Linux en unitaire seulement ;
  - iTerm2, et les MCP gitlab/team avec de vrais jetons, non testés ;
  - un volume relatif crée un dossier vide dans le projet (anomalie Q3-4) ;
  - changer de moteur garde la même clé de groupe ;
  - `bd` interactif est indisponible ;
  - la première construction d'image est longue.

## Alternatives considérées

| Alternative | Rejetée car |
|---|---|
| API Docker (SDK) plutôt que la CLI | Dépendance lourde, et Colima, Podman et Docker se pilotent tous par la même CLI. |
| CLI devcontainer | Dépendance en plus et moins de contrôle sur les montages, le réseau et l'utilisateur. |
| Monter `~/.oh` dans le conteneur | Exposerait le socket du démon, `oh.db` et les jetons à l'agent. |
| Une image par session | Constructions répétées ; l'image par projet, étiquetée par contenu, se réutilise. |
| Modifier la CI de chaque projet | Intrusif ; un projet `oh-runner` central par groupe suffit. |
| Machines distantes par SSH | Infrastructure à maintenir ; l'équipe s'appuie sur GitLab CI. |
| `opencode --auto` dans le job | Approuve toutes les demandes (F25) ; le répondeur applique la politique du workflow. |
| Entrées en variables du pipeline | Visibles dans la page du pipeline ; elles passent par l'enveloppe de session. |
