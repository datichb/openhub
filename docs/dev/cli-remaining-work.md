# Travail restant — CLI oh v2.0.0

> Backlog structuré pour atteindre une release production-ready.
> Priorisé par blocs : P0 = bloquant pour release, P1 = important, P2 = nice-to-have.
>
> Créé le 1er juillet 2026 — Post-Phase 7.

---

## P0 — Bloquants release

### Auto-download opencode

Le module `internal/opencode/` détecte le binaire mais ne le télécharge pas automatiquement.

- [ ] Implémenter download depuis GitHub releases dans `internal/opencode/download.go`
- [ ] Vérification checksum SHA256 après téléchargement
- [ ] Stockage dans `~/.oh/bin/opencode-<version>`
- [ ] Symlink `~/.oh/bin/opencode` → version courante
- [ ] Intégration dans `oh start` : download si absent avant exec
- [ ] `oh upgrade opencode` : mise à jour vers la dernière version compatible
- [ ] Tests : mock HTTP pour le download, vérification checksum

### Tests et qualité

- [ ] Configurer `golangci-lint` (`.golangci.yml` à la racine de `cli/`)
- [ ] Exécuter et corriger les findings du linter
- [ ] Ajouter `-race` aux tests dans la CI
- [ ] Tests d'intégration `cmd/` : invoquer le binaire en subprocess, vérifier stdout/stderr/exit code
- [ ] Commandes prioritaires à tester en intégration : `oh project add/list/remove`, `oh config set/get`, `oh doctor`
- [ ] Fix : `GetApp()` nil-safety — certaines commandes pourraient être appelées sans init (ex: si DB corrompue)

### Distribution

- [ ] Créer le repo `datichb/homebrew-openhub` sur GitHub
- [ ] Tester `goreleaser release --snapshot --clean` en local
- [ ] Vérifier cross-compilation : `GOOS=linux GOARCH=amd64 go build`
- [ ] Vérifier cross-compilation : `GOOS=linux GOARCH=arm64 go build`
- [ ] Script d'installation curl (remplace `install.sh`) pour les utilisateurs hors Homebrew
- [ ] Documenter le process de release dans ce fichier ou un `RELEASING.md`

---

## P1 — Important

### Session tracking

Actuellement `oh start` ne crée pas de session en DB. Les métriques sont vides.

- [ ] `oh start` : créer un `domain.Session` avec `SessionStore.Create()` avant exec
- [ ] Capturer les métriques de session à la fin (tokens in/out, durée)
  - Option A : opencode écrit un fichier de métriques que `oh` lit après exit
  - Option B : wrapper subprocess au lieu de `exec` (perdre le pattern exec)
- [ ] `oh metrics` : utiliser les vraies données sessions
- [ ] `oh dashboard` : stats réelles au lieu de zéros

### Deploy MCP injection

La Phase 4 bash (injection des serveurs MCP dans `opencode.json`) n'est pas répliquée.

- [ ] Ajouter `deploy.DeployMCP()` phase
- [ ] Générer la section `mcpServers` dans `opencode.json` :
  ```json
  {
    "mcpServers": {
      "figma": { "command": "oh", "args": ["mcp", "serve", "figma"] },
      "gitlab": { "command": "oh", "args": ["mcp", "serve", "gitlab"] }
    }
  }
  ```
- [ ] Respecter la config projet (quels MCP sont activés pour ce projet)
- [ ] Test : vérifier que le JSON généré est valide et merge sans casser l'existant

### Plugin RTK (natif)

Le plugin est listé mais pas implémenté.

- [ ] Réécrire la logique RTK en Go (`internal/plugin/rtk/`)
- [ ] Implémenter l'interface `Plugin` définie dans le plan
- [ ] Hook `OnBeforeCommand` : réécriture des commandes pour utiliser `rtk`
- [ ] Hook `OnSessionEnd` : tracking des tokens économisés
- [ ] Configuration : activation/désactivation par projet dans `hub.toml`

### Matrice de compatibilité

- [ ] Créer `cli/internal/opencode/compatibility.json` (versions oh ↔ opencode)
- [ ] `//go:embed compatibility.json` dans le package opencode
- [ ] `oh doctor` : vérifier que la version opencode installée est dans la matrice
- [ ] Warning au `oh start` si version opencode non testée

---

## P2 — Nice-to-have

### TUI polish

- [ ] `oh board` : message actionnable si `bd` non installé ("Installez bd ou configurez un tracker")
- [ ] `oh dashboard` : fallback vers métriques opencode SQLite si DB sessions vide
- [ ] Migrer les sélections >10 items vers le picker TUI (au lieu de `huh.Select`)
- [ ] Mouse support dans le board (clic sur une carte)
- [ ] Resize handling amélioré (recalcul des colonnes board)

### i18n complet

- [ ] Extraire toutes les chaînes hardcodées françaises des Phases 3-7 vers `T()`
- [ ] Compléter `locales/fr.json` avec toutes les clés
- [ ] Compléter `locales/en.json` (actuellement partiel)
- [ ] Ajouter un test qui vérifie la parité des clés entre locales

### Keychain fallback

- [ ] Implémenter un fallback fichier chiffré AES-256-GCM dans `internal/storage/keychain/`
- [ ] Détection automatique : D-Bus disponible → keychain OS, sinon → fichier
- [ ] Passphrase demandée au premier usage, cachée en mémoire pour la session
- [ ] Tests : mock keyring failure → fallback vers fichier

### Cleanup repo (à faire au moment du merge)

- [ ] Supprimer `scripts/` (30 cmd-*.sh + 26 lib/*.sh + adapters/)
- [ ] Supprimer `oc.sh`, `ocp.sh`
- [ ] Supprimer `tests/` (80+ fichiers .bats)
- [ ] Supprimer `servers/` (3 MCP servers TypeScript)
- [ ] Supprimer `plugins/rtk/` (plugin TypeScript)
- [ ] Supprimer `install.sh`, `uninstall.sh`
- [ ] Mettre à jour `.gitignore` (retirer les patterns bash)
- [ ] Mettre à jour `opencode.json` : MCP pointe vers `oh mcp serve <name>`
- [ ] Supprimer le job ShellCheck dans `.github/workflows/ci.yml`

### Documentation utilisateur

- [ ] `README.md` : réécrire pour documenter `oh` (installation, quick start, commandes)
- [ ] Guide de migration depuis `oc` (breaking changes, equivalences de commandes)
- [ ] `oh help` long : ajouter des exemples d'usage par commande

### Améliorations futures

- [ ] `oh self-update` : mise à jour du binaire oh via Homebrew ou download
- [ ] Telemetry opt-in : métriques d'usage anonymes
- [ ] `oh doctor` amélioré : espace disque, version opencode, réseau
- [ ] Mode offline : cache des opérations quand pas de réseau
- [ ] `oh project archive/unarchive` : gestion du cycle de vie projet

---

## Estimation effort

| Bloc | Effort estimé | Dépendances |
|------|---------------|-------------|
| P0 — Auto-download opencode | 1-2 jours | Aucune |
| P0 — Tests + qualité | 1-2 jours | Aucune |
| P0 — Distribution | 1 jour | Repo Homebrew créé |
| P1 — Session tracking | 1-2 jours | Choix architecture (exec vs subprocess) |
| P1 — Deploy MCP | 0.5 jour | Aucune |
| P1 — Plugin RTK | 1-2 jours | Aucune |
| P1 — Matrice compatibilité | 0.5 jour | Aucune |
| P2 — TUI polish | 1-2 jours | Aucune |
| P2 — i18n | 1 jour | Aucune |
| P2 — Keychain fallback | 1 jour | Aucune |
| P2 — Cleanup repo | 0.5 jour | Merge de la branche |
| P2 — Documentation | 1 jour | Cleanup fait |
| **Total** | **~10-14 jours** | |

---

## Ordre d'exécution recommandé

```
1. P0 Tests + golangci-lint     → fiabilise la base
2. P0 Auto-download opencode    → feature critique
3. P0 Distribution (GoReleaser) → valide le packaging
4. P1 Deploy MCP injection      → rapide, complète le deploy
5. P1 Session tracking          → donne du sens aux métriques
6. P1 Matrice compatibilité     → sécurise les mises à jour
7. P1 Plugin RTK                → feature value pour les utilisateurs
8. P2 i18n + TUI polish         → qualité UX
9. P2 Cleanup repo              → merge time
10. P2 Documentation            → post-merge
```

---

*Document créé le 1er juillet 2026*
