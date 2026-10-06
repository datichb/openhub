> [Read in English](troubleshooting.en.md)

# Guide de depannage

## Apercu

Ce guide couvre les problemes courants avec openhub, comment les diagnostiquer avec `oh doctor`, et comment reparer une installation defaillante.

---

## Diagnostics rapides

### `oh doctor`

Executez `oh doctor` pour verifier l'etat de sante de votre installation. Il effectue 13 verifications :

| # | Verification | Ce qui est verifie | Correction courante |
|---|--------------|--------------------|--------------------|
| 1 | OS / Architecture | Informations systeme (toujours OK) | — |
| 2 | Runtime Go | Version de Go (toujours OK) | — |
| 3 | git | Binaire `git` dans le PATH | Installer git |
| 4 | opencode | Runtime OpenCode installe | `brew install anomalyco/tap/opencode` |
| 5 | bd (beads) | CLI Beads (optionnel) | `brew install datichb/tap/bd` |
| 6 | fzf | Recherche floue (optionnel) | `brew install fzf` |
| 7 | opencode V2 | version minimale d'opencode V2 (V1 refusé) | voir le [guide de migration v5](migration-v5.fr.md) |
| 8 | Version | Derniere version de oh disponible | `oh upgrade` |
| 9 | Configuration | `hub.toml` se charge correctement | `oh init` pour reinitialiser |
| 10 | Identifiants fournisseur | Cle API du fournisseur LLM presente | `oh secrets set` ou definir la variable d'env |
| 11 | Base de donnees | Base SQLite accessible | `oh repair` |
| 12 | Cles API | Tokens des services MCP (Figma, GitLab, etc.) | `oh mcp setup <service>` |
| 13 | Beads zero-impact | Aucun effet de bord de Beads | `oh repair` |

### `oh repair`

Diagnostique et repare la base de donnees SQLite :

```bash
oh repair                 # Diagnostic et reparation interactifs
oh repair --check-only    # Verification sans modification
oh repair --auto          # Non-interactif (pour les scripts)
```

Si la base de donnees est corrompue, `oh repair` va :
1. Sauvegarder le fichier corrompu dans `<db>.corrupt-backup`
2. Rechercher les sauvegardes d'export existantes (`oh-backup-*.tar.gz`)
3. Proposer des options de recuperation : restaurer depuis une sauvegarde, reinitialiser, ou re-enregistrement manuel

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

---

### Worktree

**Projet non deploye**
```
Error: .opencode/ directory missing
```
Deployez la configuration du projet :
```bash
oh deploy
```

**Worktrees orphelins**
```bash
oh worktree cleanup        # Supprimer les worktrees fusionnes (mode sans risque)
oh worktree cleanup -f     # Forcer la suppression de tous les worktrees fusionnes
oh worktree list           # Lister tous les worktrees actifs
```

---

## Arbre de decision diagnostique

Si `oh` ne fonctionne pas :

1. Executez `oh doctor` — verifiez les 13 points de sante
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
