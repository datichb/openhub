> [Read in English](troubleshooting.en.md)

# Guide de dépannage

## Aperçu

Ce guide couvre les problèmes courants avec openhub, comment les diagnostiquer avec `oh doctor`, et comment réparer une installation défaillante.

---

## Diagnostics rapides

### `oh doctor`

Exécutez `oh doctor` pour vérifier l'état de santé de votre installation. Il effectue une vingtaine de vérifications (la vue Doctor de la TUI exécute exactement les mêmes) :

| # | Vérification | Ce qui est vérifié | Correction courante |
|---|--------------|--------------------|--------------------|
| 1 | OS / Architecture | Informations système (toujours OK) | — |
| 2 | Runtime Go | Version de Go (toujours OK) | — |
| 3 | git | Binaire `git` dans le PATH | Installer git |
| 4 | bd (beads) | CLI Beads (optionnel) | `brew install datichb/tap/bd` |
| 5 | Version oh | Dernière version publiée de oh : une version plus récente est un avertissement (⚠, sans échec) ; une préversion ou un build de développement plus récent que la dernière version publiée est « à jour » | `oh upgrade oh` (ou `brew upgrade openhub`) |
| 6 | Configuration | `hub.toml` se charge correctement | `oh init` pour réinitialiser |
| 7 | Identifiants fournisseur | Identifiants trouvés selon la cascade d'un lancement depuis le dossier courant : clé du projet, de l'équipe, du hub, puis profil AWS (Bedrock) ; la clé, sa portée et sa date d'enregistrement sont affichées, une clé API Bedrock est validée auprès d'AWS (sauf `oh doctor --offline` ou `OH_DOCTOR_OFFLINE=1`) ; une `AWS_BEARER_TOKEN_BEDROCK` du shell inutilisée est signalée | `oh provider setup`, `oh provider setup bedrock --from-env`, `oh secrets set` |
| 8 | Base de données | Base SQLite accessible | `oh repair` |
| 9 | Clés API | Jetons des services MCP (Figma, GitLab, etc.) | `oh mcp setup <service>` |
| 10 | Beads zero-impact | Aucun effet de bord de Beads (hooks, gitignore, fichiers d'agents écrits par `bd init` : `AGENTS.md`, `CLAUDE.md`, `.claude/`, `.codex/`, `.cursor/`, `.agents/` ; `.beads/` et `/.beads.gate.lock`, le verrou que bd 1.3 laisse à la racine du projet, dans `.git/info/exclude`). Un `.beads/` suivi par git (partagé avec l'équipe) n'est pas exclu : si le `.gitignore` du dépôt ne couvre pas `/.beads.gate.lock`, un avertissement (⚠) le signale, sans rien écrire | `oh doctor --fix` (voir [Premiers pas](getting-started.fr.md#configuration-initiale--oh-init)) ; `.beads/` partagé : ajouter `/.beads.gate.lock` au `.gitignore` de l'équipe |
| 11 | opencode V2 | opencode installé, version minimale 2.0.0 (V1 refusée) | `brew install anomalyco/tap/opencode` ; voir le [guide de migration v5](migration-v5.fr.md) |
| 12 | Anciens déploiements | Restes de `oh deploy` dans les projets | `oh migrate deploy-cleanup` |
| 13 | Démon oh | État du démon `ohd` (démarré au premier lancement) | `oh daemon status` |
| 14 | Sécurité | Capacité d'émission du démon dans le trousseau, jetons du proxy stockés hachés | — |
| 15 | Restrictions des sessions | Restrictions du hub (désactivées par défaut) | `oh budget show` |
| 16 | git (worktrees relatifs) | Version de git suffisante pour les worktrees | Mettre git à jour |
| 17 | Ouverture des sessions | Méthode d'ouverture des sessions (terminal, iTerm, tmux…) | — |
| 18 | Conteneur | Moteur, partage de fichiers, images des projets | voir [Conteneur](container.fr.md) |
| 19 | Passerelles | `bd` sur la machine (passerelle Beads), passerelles du démon | Installer `bd` ; `oh daemon status` |
| 20 | Serveurs opencode orphelins | Serveurs lancés par oh que plus personne ne possède : test ou découverte des agents natifs interrompus, serveur que ce hub ne fait plus tourner, serveur d'un dossier oh supprimé (les serveurs d'un autre `OH_HOME` sont laissés) | `oh doctor --fix` |

### `oh repair`

Diagnostique et répare la base de données SQLite :

```bash
oh repair                 # Diagnostic (sans migrer la base)
oh repair --check-only    # Vérification sans modification
oh repair --migrate       # Appliquer les migrations en attente, après une sauvegarde
oh repair --restore <fichier.bak> # Remettre une sauvegarde en place (démon arrêté)
oh repair --auto          # Non interactif (pour les scripts)
```

`oh repair` et `oh doctor` démarrent même si `hub.toml` ne se charge pas ou si la base ne s'ouvre pas : ils ne migrent jamais la base d'eux-mêmes.

Si la base de données est corrompue, `oh repair` va :
1. Copier les fichiers de la base (avec `-wal` et `-shm`) dans `<db>.corrupt-<date>`
2. Lister les sauvegardes faites avant les migrations (`~/.oh/backups/`) et les exports (`oh-backup-*.tar.gz`)
3. Proposer, dans l'ordre : restaurer une sauvegarde, restaurer un export, et seulement en dernier recours repartir d'une base vide

**`hub.toml` illisible** : `oh doctor` affiche l'erreur avec sa ligne et sa colonne. `oh` garde une copie de `hub.toml` avant chaque enregistrement (`hub.toml.bak-<date>`, trois gardées) ; `oh doctor --fix` restaure la plus récente valide après confirmation (`--yes` sans terminal), le fichier cassé est gardé en `hub.toml.broken-<date>`.

---

## Erreurs courantes

### Fournisseur et authentification

**Aucun identifiant trouvé**
```
Error: no credentials found for provider "anthropic"
```
Définissez votre clé API :
```bash
oh secrets set ANTHROPIC_API_KEY    # Via le trousseau (recommandé)
export ANTHROPIC_API_KEY="sk-..."   # Via variable d'environnement
```

**Lancement refusé faute d'identifiants**
```
lancement impossible, identifiants du fournisseur introuvables (projet, équipe, hub ; profil AWS pour Bedrock) : …
```
`oh run` (et la fiche de lancement de la TUI, à l'étape Récap) vérifie les identifiants du fournisseur avant de construire le paquet ou de demander quoi que ce soit au démon, selon la même cascade qu'`oh doctor` : clé du projet, de l'équipe, du hub, puis profil AWS pour Bedrock. Configurez-les avec `oh provider setup` (ou `-P` pour un autre fournisseur). Un refus du démon (« le démon oh n'a pas pu préparer les identifiants… ») se diagnostique avec `oh doctor`.

**Clé Bedrock refusée par AWS**
```
Authentication failed: Please make sure your API Key is valid
```
AWS refuse la clé API Bedrock utilisée par la session. Causes habituelles :
- les clés API Bedrock **à court terme** expirent (12 h au plus) ;
- vous avez renouvelé la clé dans votre shell (`AWS_BEARER_TOKEN_BEDROCK`), mais oh ignore cette variable par conception : il n'utilise que la clé enregistrée dans le trousseau.

oh lève aussi une alerte ✗ dans « À traiter » (clé, portée, date d'enregistrement). `oh doctor` dit quelle clé est utilisée et si AWS l'accepte. Pour corriger :
```bash
oh provider setup bedrock --from-env   # enregistrer la clé de votre shell (-p <projet> pour la clé du projet)
oh provider setup bedrock              # ou saisir une nouvelle clé
```
puis relancez la session. Avec `auth_mode = "profile"` ou `"env"`, oh signe en SigV4 et n'utilise aucune clé enregistrée : renouvelez les identifiants AWS (par ex. `aws sso login`).

**Session lente : limite de débit du fournisseur (429)**

L'outil réessaie seul quand le fournisseur répond 429 ; la session semble alors figée. Le proxy d'identifiants du démon le note dans `~/.oh/run/ohd.log` : un avertissement `credproxy: upstream rate limit` au premier 429 d'une série (fournisseur, session, modèle, `retry_after`, identifiant de requête du fournisseur, message), puis des lignes de débogage (`in_a_row=<n>`, absentes du journal par défaut) jusqu'à la première réponse qui n'est pas un 429. Attendez, réduisez le parallélisme (`oh session list`), ou demandez un quota plus élevé au fournisseur.

**Commande lancée hors du dossier d'un projet**
```
vous n'êtes dans le dossier d'aucun projet : lancez la commande depuis le dossier du projet ou utilisez -p <projet>
```
Hors du dossier d'un projet, `oh` ne prend plus en silence le seul projet enregistré. Une commande qui écrit (`oh run` et ses alias, `oh review feedback`, `oh mcp setup|enable|disable|reset`, `oh project rename|move|configure`, `oh budget set`…) demande confirmation dans un terminal et refuse sans terminal ; une commande qui lit (`oh bundle show`, `oh mcp status`…) annonce sur la sortie d'erreur le projet utilisé. Avec plusieurs projets, une liste permet de choisir. Utilisez `-p <projet>` (`oh project list`).

**Jeton invalide (services MCP)**
```
Error: 401 Unauthorized
```
Rafraîchissez ou réinitialisez le jeton :
```bash
oh mcp setup gitlab                 # Relancer l'assistant de configuration
oh secrets set GITLAB_TOKEN         # Définir le jeton directement
```

---

### Configuration

**Modification externe**
```
Error: hub.toml was modified by another process
```
Le fichier de configuration a été modifié en dehors de `oh`. Relancez votre commande — la configuration sera rechargée automatiquement.

**Erreur d'analyse de la configuration**
```
Error: failed to parse hub.toml
```
Vérifiez la syntaxe de `hub.toml`. Causes fréquentes : guillemets non fermés, TOML invalide. Exécutez `oh init` pour régénérer si nécessaire.

---

### État d'équipe

**Dépôt non cloné**
```
Error: team-state repository not cloned
```
Initialisez ou rejoignez l'équipe :
```bash
oh team init       # Créer une nouvelle équipe
oh team rejoin     # Rejoindre une équipe existante
```

**Conflit de synchronisation**
```
Error: push failed after retries (concurrent edits)
```
Un autre membre de l'équipe a poussé des modifications en même temps. Tirez et réessayez :
```bash
cd ~/.oh/team-state && git pull --rebase && git push
```

**Réclamation déjà attribuée**
```
Error: ticket BD-42 is already claimed by alice
```
Le ticket est déjà pris. Utilisez `oh team status` pour voir les réclamations en cours, ou demandez un transfert.

---

### Tracker

**Écriture désactivée**
```
Error: write operations disabled
```
Activez le mode écriture dans la configuration de votre tracker :
```toml
[tracker]
write_enabled = true
```

**Limite de débit atteinte**
```
Error: rate limited (retry after 30s)
```
Attendez la durée indiquée. GitLab et Jira imposent des limites de débit par utilisateur.

---

### Base de données

**Base de données corrompue**
```
Error: database disk image is malformed
```
Lancez l'outil de réparation :
```bash
oh repair
```

Si la réparation échoue, restaurez depuis une sauvegarde :
```bash
oh import oh-backup-2026-10-01.tar.gz
```

**Mise à jour de la base (migrations)**

Au premier lancement après une mise à jour d'`oh`, la base `~/.oh/oh.db` est migrée. Plusieurs processus peuvent l'ouvrir en même temps (TUI, démon, serveurs MCP) : un seul migre, les autres attendent (verrou `oh.db.migrate.lock`, 30 s au plus) puis trouvent la base à jour. Avant de migrer une base existante, `oh` en fait une copie dans `~/.oh/backups/oh.db.v<version>-<date>.bak` (les trois dernières sont gardées). Si la copie échoue, la migration est refusée (`OH_SKIP_DB_BACKUP=1` pour passer outre).

Si une migration échoue, le message donne la sauvegarde à restaurer :
```
La migration v42 de la base de données a échoué : …
Une sauvegarde a été faite avant la migration : ~/.oh/backups/oh.db.v41-….bak
Pour la restaurer : oh repair --restore ~/.oh/backups/oh.db.v41-….bak
```

**Base mise à jour par une version plus récente d'`oh`**
```
La base de données ~/.oh/oh.db a été mise à jour par une version plus récente d'oh (migrations v44 ; …)
```
Une version plus ancienne d'`oh` refuse de travailler sur une base dont elle ne connaît pas le schéma : mettez `oh` à jour. `oh doctor` et `oh repair` restent utilisables. Pour passer outre à vos risques : `OH_ALLOW_NEWER_SCHEMA=1` (aucune migration n'est alors appliquée).

---

### team-state

**team-state occupé**
```
Le team-state ~/.oh/teams/<equipe> est occupé par pid 4242 (oh tui) depuis 10:02:03 : réessayez dans un instant.
```
Toutes les écritures git d'`oh` dans un clone du team-state (CLI, TUI, démon, serveurs MCP) passent par un verrou : `.git/oh.lock` entre processus, et un verrou par chemin dans un même processus. Une commande attend jusqu'à 30 s, le démon et la synchronisation du suivi 2 min ; les synchronisations périodiques de la TUI sautent un tour. Un `.git/index.lock` de plus de 2 minutes (git interrompu) est supprimé avec un avertissement ; un plus récent signale un git lancé à la main dans le clone : attendez qu'il se termine. Les transferts réseau bloqués abandonnent après 30 s sans débit. Un push refusé n'efface plus que les fichiers de la publication en cours (plus de `reset --hard`). Chaque écriture est une transaction (pull, écriture, commit, push, refaite sur l'état distant si un autre membre a poussé entre-temps) qui ne commite **que ses propres fichiers** : un fichier que vous avez modifié à la main dans le clone n'est jamais embarqué ; une synchronisation du suivi (GitLab, Jira) fait un seul commit.

---

### Worktree

**`.opencode/ directory missing` (oh < v5)**
```
Error: .opencode/ directory missing
```
Depuis la v5, le projet n'a plus besoin d'un répertoire `.opencode/` (`oh deploy` supprimé en v5) : le paquet de session est construit au lancement. Mettez `oh` à jour et relancez, ou inspectez le paquet :
```bash
oh run <workflow>
oh bundle show <workflow>
```

**Restes d'anciens déploiements** (signalés par `oh doctor`)
```bash
oh migrate deploy-cleanup --dry-run --diff   # prévisualiser ce qui serait supprimé
oh migrate deploy-cleanup                    # supprimer .opencode/agents, .opencode/skills, clés oh dans opencode.json…
```

**Worktrees orphelins**
```bash
oh worktree cleanup        # Supprimer les worktrees fusionnés (mode sans risque)
oh worktree cleanup -f     # Supprimer aussi les worktrees fusionnés qui ont des modifications non validées
oh worktree list           # Lister tous les worktrees actifs
```

---

### Espace disque

`oh gc --dry-run` montre ce qu'oh peut libérer (paquets de session, données des serveurs, dossiers des sessions arrêtées, worktrees créés par oh, dossiers temporaires, serveurs orphelins, historique ancien) ; `oh gc` le supprime après confirmation. Le démon le fait toutes les 24 h une fois qu'un premier `oh gc` a été lancé (`oh doctor` › « Nettoyage » le rappelle). Voir [`oh gc`](../reference/cli-infra.fr.md#oh-gc).

### Journaux

| Journal | Emplacement |
|---|---|
| Démon `ohd` | `~/.oh/run/ohd.log` |
| Serveur d'un groupe de sessions | `~/.oh/servers/<groupe>/data/oh-server.log` |

Les journaux sont bornés : au-delà de 20 Mo, un journal est décalé en `<fichier>.1` (les copies plus anciennes en `.2`, `.3` ; trois gardées). `ohd.log` l'est au démarrage du démon, le journal d'un serveur à son démarrage ; pendant qu'ils tournent, le démon copie puis vide les journaux trop gros toutes les 10 minutes.

---

## Arbre de décision diagnostique

Si `oh` ne fonctionne pas :

1. Exécutez `oh doctor` — vérifiez les points de santé
2. Si la base de données échoue → `oh repair`
3. Si les identifiants échouent → `oh secrets set` ou `oh mcp setup`
4. Si la configuration échoue → `oh init` pour régénérer
5. Si l'état d'équipe échoue → `oh team init` ou `oh team rejoin`
6. Si toutes les vérifications passent mais le problème persiste → consultez les journaux avec `oh --log-format json <command>`

---

## Obtenir de l'aide

```bash
oh doctor          # Vérification complète de santé
oh repair          # Réparation de la base de données
oh --help          # Vue d'ensemble des commandes (oh help --all : tout le détail)
```

Pour les problèmes non résolus : [GitHub Issues](https://github.com/datichb/openhub/issues)
