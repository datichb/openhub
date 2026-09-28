# Releasing oh

Process de release du binaire `oh` (OpenHub CLI).

## Prérequis

- Go 1.26+
- [GoReleaser](https://goreleaser.com/) v2.10+
- Un `GITHUB_TOKEN` avec les droits `repo` (pour push la formula Homebrew)
- Le repo `datichb/homebrew-tap` doit exister (public, avec un dossier `Formula/`)

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

### 3. Release automatique

Le push du tag déclenche automatiquement :
1. Le workflow CI (`ci.yml`) qui teste le code sur 3 OS
2. Si le CI réussit, le workflow Release (`release.yml` via `workflow_run`) qui exécute GoReleaser

GoReleaser va :
1. Compiler 4 binaires (darwin/amd64, darwin/arm64, linux/amd64, linux/arm64)
2. Créer les archives `.tar.gz` + checksums
3. Publier une release GitHub avec les assets
4. Pusher la formula dans `datichb/homebrew-tap/Formula/openhub.rb`

### 4. Vérifier

```bash
# Vérifier la release GitHub
open https://github.com/datichb/openhub/releases/latest

# Tester l'installation Homebrew
brew update
brew install datichb/tap/openhub
oh version

# Tester le script curl
curl -sSfL https://raw.githubusercontent.com/datichb/openhub/main/install.sh | sh
```

## Artefacts produits

| Fichier | Description |
|---------|-------------|
| `openhub_darwin_amd64.tar.gz` | macOS Intel |
| `openhub_darwin_arm64.tar.gz` | macOS Apple Silicon |
| `openhub_linux_amd64.tar.gz` | Linux x86_64 |
| `openhub_linux_arm64.tar.gz` | Linux ARM64 |
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

- Le `GITHUB_TOKEN` et `HOMEBREW_TAP_TOKEN` sont configurés dans les secrets du repo
- La release est automatique via `workflow_run` : le CI doit passer avant que la release se déclenche
- Pour un dry-run local : `cd cli && goreleaser release --snapshot --clean`
- La taille du binaire est ~5.5 MB (stripped, sans CGO)
- Le binaire est cross-compilable sans outils supplémentaires grâce à `modernc.org/sqlite`
- Les GitHub Actions sont pinnées par SHA (commit hash) pour la sécurité de la supply chain
- Les mises à jour des actions sont proposées automatiquement par Dependabot
- Les checksums sont signés avec Cosign (keyless via Sigstore OIDC)

## Vérification manuelle d'une release

```bash
# Télécharger checksums.txt et checksums.txt.sigstore.json depuis la release GitHub

# 1. Vérifier la signature Cosign
cosign verify-blob \
  --certificate-identity-regexp 'github.com/datichb/openhub' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  --bundle checksums.txt.sigstore.json \
  checksums.txt

# 2. Vérifier les checksums des archives
sha256sum --check --ignore-missing checksums.txt
```

## Branch protection (GitHub UI)

Les règles suivantes sont recommandées sur `main` (Settings > Branches > Add rule) :

| Règle | Valeur |
|-------|--------|
| Require status checks to pass | `go-cli (ubuntu-latest)` |
| Require branches to be up-to-date | Oui |
| Block force pushes | Oui |
