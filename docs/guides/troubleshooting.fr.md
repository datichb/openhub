> [Read in English](troubleshooting.en.md)

# Guide de depannage

## Apercu

Ce guide couvre les problemes courants avec openhub, comment les diagnostiquer avec `oh doctor`, et comment reparer une installation defaillante.

---

## Diagnostics rapides

### `oh doctor`

Executez `oh doctor` pour verifier l'etat de sante de votre installation. Il effectue une vingtaine de verifications (la vue Doctor de la TUI execute exactement les memes) :

| # | Verification | Ce qui est verifie | Correction courante |
|---|--------------|--------------------|--------------------|
| 1 | OS / Architecture | Informations systeme (toujours OK) | — |
| 2 | Runtime Go | Version de Go (toujours OK) | — |
| 3 | git | Binaire `git` dans le PATH | Installer git |
| 4 | bd (beads) | CLI Beads (optionnel) | `brew install datichb/tap/bd` |
| 5 | Version oh | Derniere version publiee de oh : une version plus recente est un avertissement (⚠, sans echec) ; une pre-version ou un build de developpement plus recent que la derniere version publiee est « a jour » | `oh upgrade oh` (ou `brew upgrade openhub`) |
| 6 | Configuration | `hub.toml` se charge correctement | `oh init` pour reinitialiser |
| 7 | Identifiants fournisseur | Identifiants trouves selon la cascade d'un lancement depuis le dossier courant : cle du projet, de l'equipe, du hub, puis profil AWS (Bedrock) ; la source est affichee | `oh provider setup`, `oh secrets set` ou variable d'env |
| 8 | Base de donnees | Base SQLite accessible | `oh repair` |
| 9 | Cles API | Tokens des services MCP (Figma, GitLab, etc.) | `oh mcp setup <service>` |
| 10 | Beads zero-impact | Aucun effet de bord de Beads (hooks, gitignore, fichiers d'agents écrits par `bd init` : `AGENTS.md`, `CLAUDE.md`, `.claude/`, `.codex/`, `.cursor/`, `.agents/` ; `.beads/` dans `.git/info/exclude`) | `oh doctor --fix` (voir [Premiers pas](getting-started.fr.md#configuration-initiale--oh-init)) |
| 11 | opencode V2 | opencode installe, version minimale 2.0.0 (V1 refuse) | `brew install anomalyco/tap/opencode` ; voir le [guide de migration v5](migration-v5.fr.md) |
| 12 | Anciens deploiements | Restes de `oh deploy` dans les projets | `oh migrate deploy-cleanup` |
| 13 | Demon oh | Etat du demon `ohd` (demarre au premier lancement) | `oh daemon status` |
| 14 | Securite | Capacite d'emission du demon dans le trousseau, jetons du proxy stockes hashes | — |
| 15 | Restrictions des sessions | Restrictions du hub (desactivees par defaut) | `oh budget show` |
| 16 | git (worktrees relatifs) | Version de git suffisante pour les worktrees | Mettre git a jour |
| 17 | Ouverture des sessions | Methode d'ouverture des sessions (terminal, iTerm, tmux…) | — |
| 18 | Conteneur | Moteur, partage de fichiers, images des projets | voir [Conteneur](container.fr.md) |
| 19 | Passerelles | `bd` sur la machine (passerelle Beads), passerelles du demon | Installer `bd` ; `oh daemon status` |
| 20 | Serveurs opencode orphelins | Serveurs lances par oh que plus personne ne possede : test ou decouverte des agents natifs interrompus, serveur que ce hub ne fait plus tourner, serveur d'un dossier oh supprime (les serveurs d'un autre `OH_HOME` sont laisses) | `oh doctor --fix` |

### `oh repair`

Diagnostique et repare la base de donnees SQLite :

```bash
oh repair                 # Diagnostic (sans migrer la base)
oh repair --check-only    # Verification sans modification
oh repair --migrate       # Appliquer les migrations en attente, apres une sauvegarde
oh repair --restore <fichier.bak> # Remettre une sauvegarde en place (demon arrete)
oh repair --auto          # Non-interactif (pour les scripts)
```

`oh repair` et `oh doctor` demarrent meme si `hub.toml` ne se charge pas ou si la base ne s'ouvre pas : ils ne migrent jamais la base d'eux-memes.

Si la base de donnees est corrompue, `oh repair` va :
1. Copier les fichiers de la base (avec `-wal` et `-shm`) dans `<db>.corrupt-<date>`
2. Lister les sauvegardes faites avant les migrations (`~/.oh/backups/`) et les exports (`oh-backup-*.tar.gz`)
3. Proposer, dans l'ordre : restaurer une sauvegarde, restaurer un export, et seulement en dernier recours repartir d'une base vide

**`hub.toml` illisible** : `oh doctor` affiche l'erreur avec sa ligne et sa colonne. `oh` garde une copie de `hub.toml` avant chaque enregistrement (`hub.toml.bak-<date>`, trois gardees) ; `oh doctor --fix` restaure la plus recente valide apres confirmation (`--yes` sans terminal), le fichier casse est garde en `hub.toml.broken-<date>`.

---

## Erreurs courantes

### Fournisseur et authentification

**Aucun identifiant trouve**
```
Error: no credentials found for provider "anthropic"
```
Definissez votre cle API :
```bash
oh secrets set ANTHROPIC_API_KEY    # Via le trousseau (recommande)
export ANTHROPIC_API_KEY="sk-..."   # Via variable d'environnement
```

**Lancement refuse faute d'identifiants**
```
lancement impossible, identifiants du fournisseur introuvables (projet, équipe, hub ; profil AWS pour Bedrock) : …
```
`oh run` (et la fiche de lancement de la TUI, a l'etape Recap) verifie les identifiants du fournisseur avant de construire le paquet ou de demander quoi que ce soit au demon, selon la meme cascade qu'`oh doctor` : cle du projet, de l'equipe, du hub, puis profil AWS pour Bedrock. Configurez-les avec `oh provider setup` (ou `-P` pour un autre fournisseur). Un refus du demon (« le démon oh n'a pas pu préparer les identifiants… ») se diagnostique avec `oh doctor`.

**Commande lancee hors du dossier d'un projet**
```
vous n'êtes dans le dossier d'aucun projet : lancez la commande depuis le dossier du projet ou utilisez -p <projet>
```
Hors du dossier d'un projet, `oh` ne prend plus en silence le seul projet enregistre. Une commande qui ecrit (`oh run` et ses alias, `oh review feedback`, `oh mcp setup|enable|disable|reset`, `oh project rename|move|configure`, `oh budget set`…) demande confirmation dans un terminal et refuse sans terminal ; une commande qui lit (`oh bundle show`, `oh mcp status`…) annonce sur la sortie d'erreur le projet utilise. Avec plusieurs projets, une liste permet de choisir. Utilisez `-p <projet>` (`oh project list`).

**Token invalide (services MCP)**
```
Error: 401 Unauthorized
```
Rafraichissez ou reinitisalisez le token :
```bash
oh mcp setup gitlab                 # Relancer l'assistant de configuration
oh secrets set GITLAB_TOKEN         # Definir le token directement
```

---

### Configuration

**Modification externe**
```
Error: hub.toml was modified by another process
```
Le fichier de configuration a ete modifie en dehors de `oh`. Relancez votre commande — la configuration sera rechargee automatiquement.

**Erreur d'analyse de la configuration**
```
Error: failed to parse hub.toml
```
Verifiez la syntaxe de `hub.toml`. Causes frequentes : guillemets non fermes, TOML invalide. Executez `oh init` pour regenerer si necessaire.

---

### Etat d'equipe

**Depot non clone**
```
Error: team-state repository not cloned
```
Initialisez ou rejoignez l'equipe :
```bash
oh team init       # Creer une nouvelle equipe
oh team rejoin     # Rejoindre une equipe existante
```

**Conflit de synchronisation**
```
Error: push failed after retries (concurrent edits)
```
Un autre membre de l'equipe a pousse des modifications simultanement. Tirez et reessayez :
```bash
cd ~/.oh/team-state && git pull --rebase && git push
```

**Reclamation deja attribuee**
```
Error: ticket BD-42 is already claimed by alice
```
Le ticket est deja pris. Utilisez `oh team status` pour voir les reclamations en cours, ou demandez un transfert.

---

### Tracker

**Ecriture desactivee**
```
Error: write operations disabled
```
Activez le mode ecriture dans la configuration de votre tracker :
```toml
[tracker]
write_enabled = true
```

**Limite de debit atteinte**
```
Error: rate limited (retry after 30s)
```
Attendez la duree indiquee. GitLab et Jira imposent des limites de debit par utilisateur.

---

### Base de donnees

**Base de donnees corrompue**
```
Error: database disk image is malformed
```
Lancez l'outil de reparation :
```bash
oh repair
```

Si la reparation echoue, restaurez depuis une sauvegarde :
```bash
oh import oh-backup-2026-10-01.tar.gz
```

**Mise a jour de la base (migrations)**

Au premier lancement apres une mise a jour d'`oh`, la base `~/.oh/oh.db` est migree. Plusieurs processus peuvent l'ouvrir en meme temps (TUI, demon, serveurs MCP) : un seul migre, les autres attendent (verrou `oh.db.migrate.lock`, 30 s au plus) puis trouvent la base a jour. Avant de migrer une base existante, `oh` en fait une copie dans `~/.oh/backups/oh.db.v<version>-<date>.bak` (les trois dernieres sont gardees). Si la copie echoue, la migration est refusee (`OH_SKIP_DB_BACKUP=1` pour passer outre).

Si une migration echoue, le message donne la sauvegarde a restaurer :
```
La migration v42 de la base de données a échoué : …
Une sauvegarde a été faite avant la migration : ~/.oh/backups/oh.db.v41-….bak
Pour la restaurer : oh repair --restore ~/.oh/backups/oh.db.v41-….bak
```

**Base mise a jour par une version plus recente d'`oh`**
```
La base de données ~/.oh/oh.db a été mise à jour par une version plus récente d'oh (migrations v44 ; …)
```
Une version plus ancienne d'`oh` refuse de travailler sur une base dont elle ne connait pas le schema : mettez `oh` a jour. `oh doctor` et `oh repair` restent utilisables. Pour passer outre a vos risques : `OH_ALLOW_NEWER_SCHEMA=1` (aucune migration n'est alors appliquee).

---

### team-state

**team-state occupe**
```
Le team-state ~/.oh/teams/<equipe> est occupé par pid 4242 (oh tui) depuis 10:02:03 : réessayez dans un instant.
```
Toutes les ecritures git d'`oh` dans un clone du team-state (CLI, TUI, demon, serveurs MCP) passent par un verrou : `.git/oh.lock` entre processus, et un verrou par chemin dans un meme processus. Une commande attend jusqu'a 30 s, le demon et la synchronisation du suivi 2 min ; les synchronisations periodiques de la TUI sautent un tour. Un `.git/index.lock` de plus de 2 minutes (git interrompu) est supprime avec un avertissement ; un plus recent signale un git lance a la main dans le clone : attendez qu'il se termine. Les transferts reseau bloques abandonnent apres 30 s sans debit. Un push refuse n'efface plus que les fichiers de la publication en cours (plus de `reset --hard`). Chaque ecriture est une transaction (pull, ecriture, commit, push, refaite sur l'etat distant si un autre membre a pousse entre-temps) qui ne commite **que ses propres fichiers** : un fichier que vous avez modifie a la main dans le clone n'est jamais embarque ; une synchronisation du suivi (GitLab, Jira) fait un seul commit.

---

### Worktree

**`.opencode/ directory missing` (oh < v5)**
```
Error: .opencode/ directory missing
```
Depuis la v5 le projet n'a plus besoin d'un repertoire `.opencode/` (`oh deploy` supprime en v5) : le paquet de session est construit au lancement. Mettez `oh` a jour et relancez, ou inspectez le paquet :
```bash
oh run <workflow>
oh bundle show <workflow>
```

**Restes d'anciens deploiements** (signales par `oh doctor`)
```bash
oh migrate deploy-cleanup --dry-run --diff   # previsualiser ce qui serait supprime
oh migrate deploy-cleanup                    # supprimer .opencode/agents, .opencode/skills, cles oh dans opencode.json…
```

**Worktrees orphelins**
```bash
oh worktree cleanup        # Supprimer les worktrees fusionnes (mode sans risque)
oh worktree cleanup -f     # Forcer la suppression de tous les worktrees fusionnes
oh worktree list           # Lister tous les worktrees actifs
```

---

### Espace disque

`oh gc --dry-run` montre ce qu'oh peut libérer (paquets de session, données des serveurs, dossiers des sessions arrêtées, worktrees créés par oh, dossiers temporaires, serveurs orphelins, historique ancien) ; `oh gc` le supprime après confirmation. Le démon le fait toutes les 24 h une fois qu'un premier `oh gc` a été lancé (`oh doctor` › « Nettoyage » le rappelle). Voir [`oh gc`](../reference/cli-infra.fr.md#oh-gc).

### Journaux

| Journal | Emplacement |
|---|---|
| Demon `ohd` | `~/.oh/run/ohd.log` |
| Serveur d'un groupe de sessions | `~/.oh/servers/<groupe>/data/oh-server.log` |

Les journaux sont bornes : au-dela de 20 Mo, un journal est decale en `<fichier>.1` (les copies plus anciennes en `.2`, `.3` ; trois gardees). `ohd.log` l'est au demarrage du demon, le journal d'un serveur a son demarrage ; pendant qu'ils tournent, le demon copie puis vide les journaux trop gros toutes les 10 minutes.

---

## Arbre de decision diagnostique

Si `oh` ne fonctionne pas :

1. Executez `oh doctor` — verifiez les points de sante
2. Si la base de donnees echoue → `oh repair`
3. Si les identifiants echouent → `oh secrets set` ou `oh mcp setup`
4. Si la configuration echoue → `oh init` pour regenerer
5. Si l'etat d'equipe echoue → `oh team init` ou `oh team rejoin`
6. Si toutes les verifications passent mais le probleme persiste → consultez les logs avec `oh --log-format json <command>`

---

## Obtenir de l'aide

```bash
oh doctor          # Verification complete de sante
oh repair          # Reparation de la base de donnees
oh --help          # Reference des commandes
```

Pour les problemes non resolus : [GitHub Issues](https://github.com/datichb/openhub/issues)
