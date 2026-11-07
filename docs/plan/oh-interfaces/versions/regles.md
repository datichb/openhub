# Règles d'exécution (communes à toutes les versions)

À lire au début de chaque session, avec le plan de la version. Ce fichier **réunit toutes les règles encore valables** des plans précédents, pour ne plus avoir à les relire :
- `oh-modernisation-plan.md` (§ 18 « Règles de travail », et « Règles apprises » des sessions 1 à 3) ;
- `simplification-tests.md` (§ R) ;
- `release-v5.0.md` et `release-v5.1.md` (§ R) ;
- `release-v5.3.md` (§ R) ;
- `release-v5.4.md` (§ R).

En cas de contradiction, c'est la règle la plus récente qui est gardée ici.

## 1. Sessions et sous-agents

- **Sous-agents** : outil `Agent` de Claude Code (type `general-purpose`), qui hérite du modèle de la session. Aucune configuration de fournisseur, de région ni de session à créer à la main.
- **Un lot = un worktree** (§ 4), créé par l'orchestrateur **avant** de lancer le sous-agent. La consigne donne le chemin du worktree, et le sous-agent n'écrit que là.
- **Au plus 3 sous-agents en parallèle**, lancés en arrière-plan. L'orchestrateur note leur nom ou identifiant dans le journal de la version.
- **Commandes longues** (suite complète, `-race`) : délai explicite (10 minutes au plus en premier plan) ; au-delà, les lancer en arrière-plan et attendre leur fin avant de rapporter. Jamais de rapport « en cours ».
- **Reprise** : un sous-agent interrompu se reprend avec `SendMessage` (« reprends où tu en étais, vérifie `git status` »).
- **La session principale (orchestrateur)** coordonne, fusionne, vérifie, écrit le journal et pose le tag (**seulement à la demande de l'utilisateur**).
- **Consigne donnée à un sous-agent**, toujours complète :
  - contexte ;
  - constats, avec les fichiers ;
  - solution décidée ;
  - fichiers de son périmètre ;
  - **fichiers des lots parallèles à ne pas toucher** ;
  - ces règles (§ 2 à § 7 et § 11) ;
  - vérifications obligatoires ;
  - format du rapport (§ 8).
- **On ne pose pas de question au sous-agent** : il prend l'option la plus prudente et la signale dans son rapport.
- **Questions à l'utilisateur** avec `AskUserQuestion`, accompagnées de contexte.
- **Décider avec l'utilisateur, un peu plus souvent** (demandé le 06/11) : l'orchestrateur le consulte avant chaque vague (contenu et choix des lots), **à chaque rapport de lot, avant la fusion** (écarts, options prudentes prises, changements du code de production hors du périmètre, restes), et en fin de vague (bilan et suite). Il explique chaque fois le contexte et donne sa recommandation ; c'est l'utilisateur qui tranche.

## 2. Interdits

- Aucun `git push` (ni `--force`, ni vers une autre remote), sauf vers des dépôts de test locaux ; aucun tag ; aucune MR ou PR.
- Aucune commande qui publie (`gh`, `glab`, `oh remote …`, `oh team … push`, `CommitAndPush` lancé à la main, `make release`).
- `goreleaser` uniquement en `check` et en `--snapshot --skip=sign`.
- **Aucune éval** (`make test-eval`, `oh eval`), **aucun e2e** (tag `e2e`), **aucun appel de modèle**, aucune session réelle (`oh run`), sans l'accord explicite de l'utilisateur.
  - Les tags `integration,e2e,container` ne servent qu'au **lint**.
  - `go test -tags integration` (qui lance `opencode serve` sans modèle) seulement de façon ciblée et en local, si une étape l'exige.
- Aucun appel réseau réel : faux serveurs uniquement.
- Ne pas toucher `~/.oh`, `~/.config/opencode` ni la config git globale.
- **Jamais `oh purge`**, même avec `HOME`/`OH_HOME` temporaires : il lance `pkill -f "oh serve"` et `pkill -f "oh mcp"` sur toute la machine. Aucune commande destructrice réelle en dehors des dossiers de test.
- Un sous-agent ne pousse jamais. L'orchestrateur ne pousse que sur demande explicite de l'utilisateur (une demande vaut pour ce push, pas pour les suivants).

## 3. Isolation, trousseau et secrets

- **Les tests sont isolés du vrai hub** (`internal/testenv`). Ne jamais faire pointer `OH_HOME` vers `openhub-test/oh-home`, `oh-home-2` ni `~/.oh`. Pour lancer `oh` à la main, mettre `OH_HOME` sur un dossier temporaire. En fin de lot, vérifier avec un `OH_HOME` témoin.
- **Trousseau macOS** : dans tout test qui lance git, `GIT_CONFIG_NOSYSTEM=1`, `GIT_CONFIG_GLOBAL` temporaire, `GIT_ASKPASS` et `SSH_ASKPASS` vides, `HOME` temporaire, trousseau d'oh factice. Contrôle avant chaque récapitulatif **et** après chaque fusion : `security dump-keychain 2>/dev/null | grep -c '"srvr"<blob>="127.0.0.1"'` doit valoir 0.
- **Secrets** : aucun secret affiché ni écrit dans un fichier suivi ; les recherches de clés se font par comptage uniquement ; `oh secrets get|set|delete|cleanup` sont refusés aux agents.
- **Protection anti-secrets de GitHub** : ne jamais écrire une fausse clé au format réel dans un test ; couper la chaîne en morceaux.

## 4. Worktrees, branches et commits

- **Un lot = un worktree et une branche**, créés depuis le dépôt :
  ```bash
  cd <dépôt openhub>
  git worktree add ../openhub-wt-<version>-<lot> -b <type>/<version>-<lot> main
  ```
  Exemples : `../openhub-wt-56-sessions`, branche `refactor/56-sessions` ; `<type>` vaut `feat`, `fix` ou `refactor`.
- Commits en anglais, préfixés par `fix:`, `feat:`, `refactor:`, `test:`, `docs:` ou `ci:`, avec le suffixe de version (` (v5.6)`) ; un sujet par commit ; chaque commit compile et passe le lint.
- Le sous-agent **ne fusionne pas et ne rebase pas sur `main` de sa propre initiative**. Si l'orchestrateur le lui demande, il fait un `git rebase main` puis relance ses tests.
- Contenu en français avec les accents ; commits en anglais ; docs en paires fr/en.

## 5. Fichiers partagés

- **Locales** (`internal/i18n/locales/{fr,en}.json`) : un préfixe par lot (par exemple `v56s.`), en français et en anglais ; le lot n'ajoute ou ne supprime que des clés sous son préfixe ; supprimer les clés devenues orphelines ; les fusions se font l'une après l'autre.
- **CHANGELOG** : chaque lot écrit son entrée sous `## [Unreleased]`. À la fusion, en cas de conflit, on garde les deux côtés (ordre : Added, Changed, Fixed). Le texte de MIGRATION est donné dans le récapitulatif, et l'orchestrateur l'écrit au moment de la release.
- **Listes gelées d'`archtest`** (`internal/archtest/testdata/*.txt` : budget du branchement, imports métier des vues, `tuiShell`, `WizardStep.Form`, exceptions de `teamstate`) : un lot ne fait **que retirer** ses propres lignes. Le nouveau total du budget est recalculé par **l'orchestrateur** à la fusion.
- **`.github/windows-test-packages.txt`** : un lot ajoute seulement ses paquets ; à la fusion, on fait l'union.
- **Types communs du contrat** (`internal/ui/contract/common.go`, `errors.go`, `events.go`) et **`present`** : un lot ajoute, il ne renomme pas et ne supprime pas ; un changement de forme passe par l'orchestrateur.
- **Plan et journal de la version** : seul l'orchestrateur les écrit.

## 6. Tests

- **Pendant le travail**, le sous-agent ne lance **que les tests de ses paquets** (la machine est partagée).
- **En fin de lot** :
  - les paquets du lot en `-race` ;
  - **toujours** `./internal/archtest/...` et `./internal/i18n/...` ;
  - `make sync-content`, puis `go test -short ./cmd/` en entier, puis les tests `cmd/` du lot sans `-short` ;
  - `GOOS=windows go vet` sur les paquets du lot ;
  - `go vet` et golangci-lint propres sur les paquets du lot.
- **Charge de la machine** : au-delà d'une charge de 50, les tests sont 3 à 5 fois plus lents. Éviter de lancer d'autres suites pendant un lot ; relancer seuls les tests instables connus plutôt que toute la suite.
- **Écran simulé** : il se lit sur la boucle d'événements ; après `SimulationScreen.SetSize`, poster `tcell.NewEventResize` puis `QueueUpdateDraw`. Ne jamais appeler `QueueUpdateDraw` depuis la boucle elle-même : le faire depuis une goroutine.
- **Tests asynchrones** : une panique rattrapée par `safego` est journalisée **après** l'écriture du rapport de crash. Un test qui lit le journal doit l'attendre (`Eventually`).
- **Chemins** : `filepath`, jamais `/` en dur ; `.git/config` se lit avec `gitremote` (échappements Windows).
- **Goldens** :
  - jamais régénérés pour faire passer un test ; toute régénération est listée et justifiée dans le récapitulatif ;
  - en cas de conflit sur les goldens du bundle : reprendre `budget.golden`, `capabilities.golden`, `rules/*.tsv` et les goldens de `main`, puis régénérer avec `-run 'ShippedWorkflow|Capabilit|Budget|RuleLedger|Contract' -update` ;
  - `capability_allow.yaml` s'**unit** (ne jamais prendre un seul côté) ;
  - deux tests ajoutés au même endroit d'un fichier : garder les deux fonctions.
- **Couverture** : aucune baisse par paquet par rapport à la mesure précédente (`-coverprofile`) ; seuil de la CI ≥ 60 %.
- **Lint** : golangci-lint **2.14.0** compilé avec Go 1.26 (`GOBIN=<dossier> GOTOOLCHAIN=go1.26.10 go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0`), avec `GOTOOLCHAIN=go1.26.10`, `--build-tags=integration,e2e,container` et un `GOLANGCI_LINT_CACHE` en dehors du worktree. Une version plus ancienne refuse le module (« built with a lower Go version »).

## 7. Fusion et fin de vague (orchestrateur)

- `git merge --no-ff`, une branche à la fois, dans l'ordre de la vague, puis `git worktree remove` et `git branch -d`.
- Si `main` a bougé : `git rebase main` dans le worktree, relancer les tests du lot, puis fusionner.
- **Conflit sur un fichier d'un autre lot** : s'arrêter et le signaler ; ne pas résoudre au hasard.
- **Après chaque fusion**, en une seule commande enchaînée (`&&`) :
  - `go build ./... && go vet ./... && GOOS=windows go vet ./...` ;
  - archtest et i18n ;
  - `-race` des paquets du lot ;
  - `-short ./cmd/` ;
  - contrôle du trousseau.
- **En fin de vague** :
  - `go test -race -timeout 40m ./...` ;
  - **`go test ./cmd/` sans `-short`** (environ 6 min : deux régressions ont été cachées par `-short` le 23/10) ;
  - simulation du runner (`HOME` vide, `GIT_CONFIG_GLOBAL=/dev/null`, `go test -timeout 40m ./...` sans `-race`) ;
  - lint du module ;
  - `(cd cli && ../.github/scripts/windows-packages.sh check)` : chaque nouveau paquet doit être classé.
- Sur une machine chargée, les tests `gateway` et `beads` qui lancent un vrai `git` ou `bd` peuvent échouer : les relancer seuls avant de conclure.

## 8. Rapport d'un lot (sous-agent)

- Fichiers touchés ; commits.
- Tests lancés et résultats ; durées (avant et après, si la version le demande).
- **Goldens modifiés et pourquoi.**
- Entrée de CHANGELOG proposée ; texte de MIGRATION s'il y en a un.
- Options prudentes prises à la place d'une question ; risques ; reste à faire.
- Pour les lots de refactoring : voir aussi le récapitulatif de [methode-n1.md](methode-n1.md).

## 9. Avant le tag (orchestrateur)

- `go test -count=1 ./...` **sans `-race`**, comme le job de release.
- `windows-packages.sh check`.
- `.github/scripts/release-notes.sh vX.Y.Z`, qui doit produire au moins un lien absolu (`TestReleaseNotes_RealChangelog`).
- `goreleaser check` (aucun avertissement attendu après la 5.5, quand `brews` aura disparu) et un snapshot.
- CI verte **sur le commit exact**. Sous macOS, si « job was not acquired by Runner », lancer `gh run rerun <id> --failed`.
- Puis, **seulement à la demande de l'utilisateur** : tag annoté, `git push github vX.Y.Z`, vérification de la release (latest, 4 archives, checksums, `.sigstore.json`, `cosign verify-blob`, SHA-256, `oh version`, formule ou cask du tap, liens des notes).

## 10. Règles propres au refactoring (à partir de la 5.5)

- **Branchement du TUI gelé** à partir de la fin de la 5.5 : `archtest` mesure les lignes et les fonctions de `cmd/tui_*.go`, et elles ne peuvent que baisser. Une nouvelle fonctionnalité met sa logique dans `services/` ou `appsvc/`, jamais dans `cmd/tui_*`.
- **Aucun changement visible pour l'utilisateur dans les lots de refactoring** (5.6 à 5.8). Une différence voulue est une décision, notée dans le récapitulatif.
- **Goldens d'écran identiques** pendant un lot de refactoring ; s'ils changent, c'est que l'affichage a bougé : on s'arrête et on corrige.

## 11. CI sur trois systèmes (apprises avec 55-ci, 07/11)

- **Windows et macOS ne tournent qu'en CI** (sur Linux, `GOOS=windows go vet` ne vérifie que la compilation). Un lot qui touche des chemins, `.git/config`, des fichiers temporaires ou des processus **pousse sa branche** pour obtenir une CI avant la fusion, avec l'accord de l'utilisateur.
- **`.git/config`** se lit avec `gitremote` et s'écrit avec `git config` ou `git remote set-url`, jamais en comparant ou remplaçant du texte brut : Windows double les antislashs (`C:\\Users\\…`).
- **Scripts de CI** : bash 3.2 sur macOS. Pas de `mapfile` ni `readarray`, pas de tableaux associatifs (`declare -A`), pas de `${var,,}`.
- **Jobs agrégés** : le résultat d'un job à matrice est unique ; un shard qui échoue fait passer au rouge les trois checks `Go CLI (…)`. Les noms de ces checks ne changent pas (protection de branche).
- **Test instable** : on le reproduit d'abord (charge CPU, `-count=300` sur le binaire de test), on corrige la cause (attendre l'effet plutôt qu'un délai), puis on montre le même `-count` vert. Ni `t.Skip` ni quarantaine.
- **Nouveaux tests de `./cmd` ou nouveaux paquets** : `make test-shards-sync`, puis `test-shards.sh check` ; un test ou un paquet hors de tout shard fait échouer la CI.
