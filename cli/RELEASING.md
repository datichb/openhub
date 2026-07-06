# Releasing oh

Process de release du binaire `oh` (OpenHub CLI).

## Prérequis

- Go 1.26+
- [GoReleaser](https://goreleaser.com/) v2.10+
- Un `GITHUB_TOKEN` avec les droits `repo` (pour push la formula Homebrew)
- Le repo `datichb/homebrew-openhub` doit exister (public, avec un dossier `Formula/`)

## Process de release

### 1. Préparer

```bash
# S'assurer qu'on est sur main, à jour
git checkout main && git pull

# Vérifier que tout passe
cd cli
make test
make lint
make build
```

### 2. Tagger

```bash
# Convention: vX.Y.Z (SemVer)
git tag -a v2.0.0 -m "oh v2.0.0 — première release Go CLI"
git push origin v2.0.0
```

### 3. Releaser

```bash
cd cli

# Dry-run (ne publie rien)
goreleaser release --snapshot --clean

# Release réelle
GITHUB_TOKEN=ghp_xxx goreleaser release --clean
```

GoReleaser va :
1. Compiler 4 binaires (darwin/amd64, darwin/arm64, linux/amd64, linux/arm64)
2. Créer les archives `.tar.gz` + checksums
3. Publier une release GitHub avec les assets
4. Pusher la formula dans `datichb/homebrew-openhub/Formula/oh.rb`

### 4. Vérifier

```bash
# Vérifier la release GitHub
open https://github.com/datichb/openhub/releases/latest

# Tester l'installation Homebrew
brew update
brew install datichb/openhub/oh
oh version

# Tester le script curl
curl -sSfL https://raw.githubusercontent.com/datichb/openhub/main/install.sh | sh
```

## Artefacts produits

| Fichier | Description |
|---------|-------------|
| `oh_darwin_amd64.tar.gz` | macOS Intel |
| `oh_darwin_arm64.tar.gz` | macOS Apple Silicon |
| `oh_linux_amd64.tar.gz` | Linux x86_64 |
| `oh_linux_arm64.tar.gz` | Linux ARM64 |
| `checksums.txt` | SHA256 de chaque archive |
| `Formula/oh.rb` | Homebrew formula (poussée dans le tap) |

## Configuration GoReleaser

Le fichier `.goreleaser.yml` est à la racine de `cli/`. Points clés :
- `CGO_ENABLED=0` — binaire statique, pas de dépendance glibc
- ldflags injectent Version, Commit, BuildDate
- Format unique `tar.gz` (simple, universel)
- La formula Homebrew est auto-générée

## Hotfix release

```bash
git checkout -b hotfix/v2.0.1
# fix...
git commit -m "fix: ..."
git checkout main && git merge hotfix/v2.0.1
git tag -a v2.0.1 -m "fix: ..."
git push origin main v2.0.1
cd cli && GITHUB_TOKEN=ghp_xxx goreleaser release --clean
```

## Notes

- Le `GITHUB_TOKEN` peut être configuré dans les secrets du repo pour la CI
- Pour une release depuis la CI, ajouter un workflow triggered par les tags `v*`
- La taille du binaire est ~5.5 MB (stripped, sans CGO)
- Le binaire est cross-compilable sans outils supplémentaires grâce à `modernc.org/sqlite`
