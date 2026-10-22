# Checklist de release d'oh

> Pour les mainteneurs. À suivre dans l'ordre, de `main` à la vérification de la release publiée. Chaque ligne a été relue contre `.github/workflows/release.yml`, `.github/workflows/ci.yml`, `cli/.goreleaser.yml`, `install.sh` et `cli/internal/selfupdate`. Politique de documentation : [ADR-037](../architecture/adr/037-documentation-policy.fr.md). Étapes de développement ordinaires : [contributing](../guides/contributing.fr.md).

Trois règles gouvernent tout le reste :

1. **Seul le workflow `release.yml` publie**, déclenché par le push d'un tag `v*` ; ce tag est posé et poussé **par un humain**. Ne jamais lancer `goreleaser release` (sans `--snapshot`) à la main : la signature Cosign est liée à l'identité de ce workflow (voir [étape 5](#5-pousser-le-tag)).
2. **`release.yml` n'attend pas la CI.** Le push d'un tag déclenche aussi `ci.yml` (`tags: ["v*"]`), mais les deux workflows tournent en parallèle : une release peut être publiée alors que la CI du même commit est rouge. La CI doit donc être verte **avant** le tag.
3. **Un tag publié ne se réécrit pas.** Avant le tag, tout se rattrape par un commit ; après, par une version suivante (voir [En cas de problème](#en-cas-de-problème)).

---

## 1. Préalables (sur `main`)

- [ ] `git switch main && git pull --ff-only && git fetch --tags`, arbre propre (`git status`).
- [ ] **CHANGELOG.** `CHANGELOG.md` a une section `## [X.Y.Z] — AAAA-MM-JJ` (ADR-037 : le CHANGELOG est à jour avant tout tag `v*`), la date du jour du tag, et `## [Unreleased]` est vide. Vérifier ce que verra la release :

  ```bash
  .github/scripts/release-notes.sh vX.Y.Z | less
  ```

  Le script imprime le corps de la section ; il échoue (code 1) si elle est absente ou vide, et `release.yml` l'appelle avant GoReleaser : sans section, aucune release. Pour un tag `vX.Y.Z-rc.N`, il lit d'abord `## [X.Y.Z-rc.N]`, sinon `## [X.Y.Z]`.
- [ ] **MIGRATION.** Pour une version majeure ou une rupture : `MIGRATION.md` et `docs/guides/migration-v5.{fr,en}.md` (ou le guide de la version) décrivent chaque rupture annoncée au CHANGELOG, avec l'action à faire.
- [ ] **Version d'opencode recommandée épinglée et égale à celle de la CI.** Trois valeurs doivent être identiques :

  | Où | Clé |
  |----|-----|
  | `cli/internal/adapters/opencodev2/compatibility.json` | `opencode_recommended` (dans `default` **et** dans `oh_versions."X.Y"`) |
  | `.github/workflows/ci.yml` | `OPENCODE_VERSION` (et les deux `OPENCODE_SHA256_DARWIN_*`) |
  | `cli/internal/adapters/opencodev2/pinned_integrity.json` | l'empreinte sha512 des quatre paquets Linux de cette version |

  Pour épingler une version (lecture seule, ne modifie rien ; à lancer deux fois, de préférence depuis deux réseaux, et à comparer) :

  ```bash
  .github/scripts/opencode-pin.sh --darwin 2.0.25
  ```

  Il lit `dist.integrity` et `dist.tarball` de `@opencode/cli-<cible>@<version>` sur le registre npm pour `linux-x64`, `linux-x64-musl`, `linux-arm64` et `linux-arm64-musl`, télécharge chaque `.tgz`, recalcule son sha512 et échoue en cas d'écart ; il affiche le fragment JSON à coller dans `pinned_integrity.json` (jamais écrit par un test) et, avec `--darwin`, les SHA-256 des paquets macOS pour `ci.yml`. Procédure complète : commentaire de `cli/internal/adapters/opencodev2/pins.go`. Trois tests gardent l'accord (`cd cli && go test ./internal/adapters/opencodev2/ -run 'TestPinnedIntegrityTable|TestRecommendedVersionIsPinned|TestCIVersionIsRecommended'`) : version recommandée dans la plage supportée et épinglée pour les quatre cibles, et égale à `OPENCODE_VERSION` de `ci.yml`.
- [ ] **Tests.** Depuis `cli/` :

  ```bash
  make embed-sync      # contenu du hub embarqué + bd (obligatoire avant le premier build)
  go vet ./...
  make lint            # golangci-lint sur linux, darwin et windows
  make test            # go test ./... -race, timeout 40 min
  ```

  `make test` ne lance ni l'intégration, ni l'e2e, ni les conteneurs, ni les évals (voir [étape 3](#3-validations-manuelles)). Compter une dizaine de minutes sur une machine peu chargée (515 s le 22/10/2026) ; sous forte charge, deux tests à délai fixe peuvent échouer (`gitutil TestFingerprintDoesNotBlockOnFIFO`, `fanout TestDriverMergerKeepsAPartForThePendingChildren`) : les relancer seuls avant de conclure.
- [ ] **GoReleaser en local.** Depuis `cli/` :

  ```bash
  goreleaser check
  goreleaser release --snapshot --clean --skip=sign
  ```

  - `goreleaser check` **échoue aujourd'hui, et c'est attendu** : il signale que `brews` est déprécié (au profit de `homebrew_casks`). **Décision du 22/10/2026 : `brews` est gardé pour la 5.0** (la formule `openhub` du tap continue d'être produite) ; la migration vers `homebrew_casks` est à planifier après la 5.0, en étudiant l'effet sur `brew install datichb/tap/openhub` et sur Linux. Tout autre message de `check` bloque la release.
  - Le snapshot ne publie rien. `--skip=sign` est nécessaire en local : la signature Cosign sans clé utilise l'identité OIDC de GitHub Actions, absente d'un poste. Il lance les hooks `before` (`go mod tidy`, `make sync-content`, `make embed-bd`), construit `darwin` et `linux` en `amd64` et `arm64` (pas de Windows) et écrit dans `cli/dist/` (ignoré par git) : quatre `openhub_<os>_<arch>.tar.gz`, `checksums.txt` et la formule `dist/homebrew/Formula/openhub.rb`. Compter une trentaine de secondes. Ensuite, `git status` doit être propre (sinon `go mod tidy` a trouvé un écart à committer), et le binaire de la machine (`cli/dist/openhub_<os>_<arch>*/oh`) répond à `oh version` (avec un `OH_HOME` temporaire). La version affichée est celle d'un snapshot (`4.2.0-SNAPSHOT-<commit>`) : c'est normal.

## 2. Push et CI verte

- [ ] Pousser `main` (ou la branche de release fusionnée). Le workflow `CI — Go CLI` (`ci.yml`) tourne sur `push` vers `main` et `develop`.
- [ ] Les trois systèmes du job `Go CLI` sont verts : `ubuntu-latest`, `macos-latest`, `windows-latest` (vet, golangci-lint v2.14, govulncheck, tests `-race` avec couverture ≥ 60 % hors Windows, build). Sous Windows, seuls les paquets de `.github/windows-test-packages.txt` sont testés ; 27 paquets sont exclus (`.github/windows-test-packages-excluded.txt`, avec leur cause) : l'annoncer comme « Windows moins vérifié », ne pas le présenter comme « multiplateforme ».
- [ ] Le job `Go CLI integration (opencode V2, no LLM)` est vert (macOS, avec l'opencode épinglé de `ci.yml`, sans modèle).
- [ ] Le commit à taguer est **le commit vert** : `git rev-parse HEAD` égale le commit du run.
- [ ] Le secret de dépôt `HOMEBREW_TAP_TOKEN` existe (Settings → Secrets) : GoReleaser s'en sert pour pousser la formule dans `datichb/homebrew-tap`. `GITHUB_TOKEN` est fourni par GitHub ; la signature utilise `id-token: write`.

## 3. Validations manuelles

La CI n'appelle aucun modèle et ne lance ni les évals ni l'e2e (décision documentée en tête du job d'intégration de `ci.yml`). Elles se font à la main, sur le commit à taguer, avec de vrais agents (coût Bedrock) :

- [ ] **Évals de comportement** : `cd cli && make test-eval` (variables `OH_EVAL_*` documentées dans `cli/Makefile` ; `OH_EVAL_PLAN=1` estime le coût sans rien lancer). Comparer à `internal/evalrun/testdata/evals/baseline.json` ; ne pas réenregistrer la référence (`OH_EVAL_RECORD`) sans décision.
- [ ] **E2E** : `cd cli && make test-e2e` (tests `TestE2E`, vrai modèle). Intégration contre un vrai `opencode serve` : `make test-integration` (sans modèle). Conteneurs, si Colima, Podman ou Docker tourne : `make test-container`.
- [ ] **Mode équipe (bêta)** : suivre [team-testing-guide](../guides/team-testing-guide.fr.md) sur un team-state de test. Vérifier l'avertissement bêta (une fois par processus), la colonne REVIEW de `oh board`, `oh team release`, et que les [limites connues](../guides/team-setup.fr.md#limites-connues) correspondent toujours à ce que l'on observe.
- [ ] **Plugins (S-11)** : avec un plugin dans la configuration personnelle de l'outil (`opencode.json`, clé `plugin`, ou dossier `plugin/` de `$XDG_CONFIG_HOME/opencode`), `oh doctor` affiche l'avertissement, le lancement d'une session local le signale, et `execution.strict_isolation = true` les masque. Le format de configuration d'opencode est lu par `adapters/opencodev2/user_plugins.go` : à revérifier à chaque nouvelle version d'opencode recommandée.
- [ ] **`oh init`** : dans un `OH_HOME` temporaire (jamais le vrai `~/.oh`), le parcours solo tient en 3 écrans, l'équipe en 4, le complet en 5 ; le `hub.toml` produit est relu sans avertissement (`oh doctor`).
- [ ] Binaire du snapshot (étape 1), avec un `OH_HOME` temporaire : `oh version`, `oh doctor`, `oh --help` (une page).

## 4. Dernier contrôle avant le tag

- [ ] Rien n'a été poussé sur `main` depuis le commit vert.
- [ ] La date de la section du CHANGELOG est celle du jour du tag (la section `## [X.Y.Z]` ne se corrige plus après le tag : les notes de la release en sont une copie).
- [ ] Les liens de la section : les notes sont affichées sur la page de la release GitHub, où un lien relatif (`MIGRATION.md`, `docs/…`) ne se résout pas. Vérifier dans la sortie de `release-notes.sh` (étape 1) que les liens sont absolus ou réécrits ; sinon, le corriger avant le tag.
- [ ] Choix du tag : `vX.Y.Z` pour une release ; `vX.Y.Z-rc.N` pour un essai (voir ci-dessous). Un tag `rc` ne coûte rien : il passe par tout le workflow, sans toucher Homebrew ni `latest`.

## 5. Pousser le tag

**Par un humain, depuis un poste qui a les droits d'écriture.** Jamais depuis un agent, jamais depuis un script.

```bash
git tag -a vX.Y.Z -m "Release vX.Y.Z"
git push origin vX.Y.Z
```

Ce que déclenche le tag (`release.yml`, job `Release oh`, `ubuntu-latest`) : `go mod download`, `make embed-sync`, `go test ./...` (sans `-race`), installation de Cosign, extraction des notes (`release-notes.sh "$TAG"` vers `$RUNNER_TEMP/notes.md`), puis `goreleaser release --clean --release-notes=…` dans `cli/`.

Pourquoi jamais de publication manuelle : GoReleaser signe `checksums.txt` avec Cosign sans clé (`checksums.txt.sigstore.json`). Le certificat porte l'identité du workflow, et `install.sh` comme `oh upgrade oh` n'acceptent que :

```text
^https://github\.com/datichb/openhub/\.github/workflows/release\.yml@refs/tags/v.*$
émetteur : https://token.actions.githubusercontent.com
```

Une release construite ailleurs n'a pas cette identité : sans signature, `install.sh` et `oh upgrade oh` n'installent qu'avec la somme de contrôle (et refusent avec `OH_REQUIRE_SIGNATURE=1`) ; avec une signature d'une autre origine, ils refusent.

### Tag `rc`

`vX.Y.Z-rc.N` suit le même chemin, avec trois différences (`cli/.goreleaser.yml`) :

- la release GitHub est une **pré-version** (`release.prerelease: auto`) : elle n'est jamais « latest », donc ni `install.sh` sans `--version` ni `oh upgrade oh` sans version ne la voient (ils lisent `releases/latest`) ;
- **Homebrew n'est pas mis à jour** (`brews[].skip_upload: auto`) ;
- les notes viennent de `## [X.Y.Z-rc.N]` si elle existe, sinon de `## [X.Y.Z]`.

Pour l'essayer : `install.sh --version X.Y.Z-rc.N` ou `oh upgrade oh X.Y.Z-rc.N`. La release finale reprend la même chaîne, sur un nouveau tag : un `rc` ne se « promeut » pas.

## 6. Vérifications après publication

Attendre la fin du workflow `Release — GoReleaser` (onglet Actions), puis :

- [ ] **Release GitHub.** Une pré-version pour un `rc`, une release « latest » sinon. Assets : `openhub_darwin_amd64.tar.gz`, `openhub_darwin_arm64.tar.gz`, `openhub_linux_amd64.tar.gz`, `openhub_linux_arm64.tar.gz`, `checksums.txt`, `checksums.txt.sigstore.json`. Les notes sont la section du CHANGELOG, avec des liens qui s'ouvrent.
- [ ] **Signature.** Télécharger `checksums.txt` et `checksums.txt.sigstore.json` de la release, puis :

  ```bash
  cosign verify-blob --bundle checksums.txt.sigstore.json \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com \
    --certificate-identity-regexp '^https://github\.com/datichb/openhub/\.github/workflows/release\.yml@refs/tags/v.*$' \
    checksums.txt
  ```

  Puis `shasum -a 256 -c checksums.txt --ignore-missing` à côté d'une archive.
- [ ] **Homebrew** (release finale seulement) : un commit de GoReleaser apparaît dans `datichb/homebrew-tap`, `Formula/openhub.rb`, à la nouvelle version, avec les URL et les SHA-256 de la release. Sur une machine : `brew update && brew upgrade openhub` (ou `brew install datichb/tap/openhub`), puis `oh version`.
- [ ] **`install.sh`** (release finale) : dans un dossier jetable, avec `cosign` installé pour que la signature soit contrôlée :

  ```bash
  curl -sSfL https://raw.githubusercontent.com/datichb/openhub/main/install.sh | sh -s -- --dir "$(mktemp -d)"
  ```

  Attendus : `Signature OK`, `Checksum OK`, puis `oh version` à la nouvelle version. Avec `OH_REQUIRE_SIGNATURE=1`, l'absence de signature ou de `cosign` fait échouer l'installation. Le script lit `releases/latest` et télécharge `openhub_<os>_<arch>.tar.gz` ; il ne prend en charge que macOS et Linux.
- [ ] **`oh upgrade oh`** depuis la version précédente (une installation hors Homebrew : `oh upgrade oh --check`, puis `oh upgrade oh`). Il télécharge l'archive, contrôle la somme puis la signature (avec `cosign` installé), remplace le binaire ; il refuse une version qui n'est pas plus récente que la version en cours, sauf demande explicite. Les utilisateurs Homebrew passent par `brew upgrade openhub`. Windows n'est pas pris en charge.
- [ ] `oh version` donne la version taguée (le commit et la date viennent des `ldflags` de GoReleaser).
- [ ] Le CHANGELOG et le tag concordent : `git describe --tags` sur `main` donne `vX.Y.Z`, et `## [Unreleased]` est vide.

## En cas de problème

| Situation | Que faire |
|-----------|-----------|
| Le workflow échoue **avant** GoReleaser (tests, notes absentes) | Rien n'est publié. Corriger par un commit, attendre la CI verte, puis supprimer le tag mal placé (`git tag -d vX.Y.Z && git push origin :refs/tags/vX.Y.Z`) et le poser à nouveau sur le bon commit. |
| `release-notes.sh` échoue (« no non-empty section ») | La section `## [X.Y.Z]` manque, est vide ou est mal nommée : la corriger dans le CHANGELOG (voir étape 1), même procédure que ci-dessus. |
| Le workflow échoue **pendant** GoReleaser | Lire le journal avant tout : la release GitHub, la signature et la formule Homebrew se font à la suite et peuvent être partielles. Ne pas relancer à l'aveugle. Repartir du même tag n'est raisonnable que si rien n'a été publié (ni release GitHub, ni formule) ; sinon, version suivante. |
| La formule Homebrew n'est pas poussée | Vérifier `HOMEBREW_TAP_TOKEN` (échu, droits) ; corriger la formule du tap à la main ou publier la version suivante. |
| La signature ne se vérifie pas | Ne pas annoncer la release. `install.sh` et `oh upgrade oh` refusent une signature invalide, ils ne se rabattent pas sur la somme de contrôle. Vérifier que le tag est bien du type `v*` et que le workflow a `id-token: write`. |
| Une release publiée est mauvaise (binaire, régression) | **Ne pas supprimer ni déplacer le tag** (des utilisateurs ont déjà la somme et la formule). Publier un correctif `vX.Y.Z+1` avec sa section de CHANGELOG ; si la version est dangereuse, la repasser en pré-version dans l'interface GitHub (elle n'est plus « latest », donc plus servie par `install.sh` ni `oh upgrade oh`) et rétablir la formule précédente dans le tap. |
| Notes de release mal rendues | Les modifier dans l'interface GitHub ; corriger ensuite le CHANGELOG ou `release-notes.sh` pour la suivante. |
| `goreleaser check` signale autre chose que `brews` | Corriger `cli/.goreleaser.yml` avant tout tag. Si GoReleaser supprime un jour `brews` dans une version 2.x (le workflow demande `~> v2`, soit la dernière), c'est le moment de migrer vers `homebrew_casks`. |
