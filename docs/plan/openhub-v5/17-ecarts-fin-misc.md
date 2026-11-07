# Écarts — finitions diverses (`feat/v5-fin-misc`, worktree `../openhub-fin-misc`, base `23e64493`)

Vérifié à chaque point : `go build ./...`, `go vet -tags "integration e2e container" ./...`, `go test ./...`, `-race -tags integration` sur les paquets touchés, golangci-lint 2.14 à 0 (darwin, linux, windows). Non fusionné, non poussé.

## 1. Doctor : version de bd ≠ table de l'analyseur (`0e098d39`)

- `bdargs.VersionNumber(out)` (« bd version 1.3.1 (Homebrew) » → `1.3.1`) et `bdargs.SameVersion(out)` : on compare le **numéro** seulement, la mention de la construction (Homebrew, source…) ne compte pas.
- Contrôle « Passerelle Beads : version de bd » dans `cmd/doctor_gateway.go` (`bdVersionCheck`, `bd version`, 10 s), juste après « bd sur la machine » : enregistré par `registerDoctorCheck`, donc présent dans `oh doctor` et dans la vue Doctor de la TUI. Avertissement (pas un échec) ; version illisible → avertissement aussi.
- **Écart** sur la commande : `go run ./internal/gateway/bdargs/gen/main.go` écrit la table sur la sortie standard ; le message donne la commande complète `go run ./internal/gateway/bdargs/gen/main.go > internal/gateway/bdargs/spec_bd.go` (depuis `cli/`) puis « recompilez oh ». Vérifié : avec bd 1.3.1, la sortie est identique à `spec_bd.go`.
- Doc : `docs/reference/beads-model.{fr,en}.md` (formes refusées) ; CHANGELOG (puce N22 de Security).
- Tests : `bdargs.TestVersionNumber`, `cmd.TestBdVersionCheck`.

## 2. Alerte « configuration git modifiée » (`04fbf7f5`)

- `gitutil` : l'enregistrement de confiance garde aussi l'empreinte **de chaque fichier** (`files`, champ facultatif : un enregistrement d'une version précédente reste lisible) ; `Check` renvoie un `*gitutil.ConfigChangedError{Common, Fingerprint, Files}` (`errors.Is(…, ErrConfigChanged)` inchangé) qui liste les fichiers modifiés, apparus (hook ajouté) ou disparus (hook supprimé). Enregistrement ancien sans liste → message générique avec le dossier git.
- Démon : `handleWorkflowCheckpoint` appelle `raiseGitChanged` sur `ErrConfigChanged` : décision `error` marquée `oh_alert` (comme l'alerte du shell, A16 : non effacée par la boucle de l'agent, l'utilisateur la ferme), titre et message traduits (checkpoint, fichiers, vérifier — `git config --list --show-origin`, hooks —, restaurer à l'identique, ou relancer la session). `refreshAlerts` → état « en attente », flux des sessions (`oh session inbox`, vue Sessions) et notification (`notes.requestScan`). Affichage générique des décisions `error` : rien à changer dans la CLI ni la TUI.
- **Choix** : une alerte **par état du dépôt** (`ToolRef` = `git_changed:<12 premiers caractères de l'empreinte>`) : le même changement refusé de nouveau, ou fermé par l'utilisateur, n'est pas relevé deux fois ; un nouveau changement l'est.
- **Écart restant** : `GuardClose` (zone G, S-09) ne propage toujours pas `ErrConfigChanged` → pas d'alerte sur ce chemin. Le message renvoyé à l'agent (`cmd.daemon.workflow.git_changed`) est inchangé.
- Doc : SECURITY fr/en (« git d'oh sur la machine »), CHANGELOG (puce N5).
- Tests : `gitutil.TestCheckDetectsTamperedConfig` (fichier nommé : config, hook ajouté, hook supprimé), `daemon.TestGitChangedAlert` (vrai dépôt, vrai démon : 409, alerte ✗ nommant `.git/config`, pas de doublon, pas relevée après fermeture, nouvelle alerte pour un hook ajouté).

## 3. Nettoyage Beads : confirmation suffisante pour annuler le commit de `bd init` (`07b6b448`)

- `beadsCleanPlan(a, path, yes, confirmed)` : `UndoCommit = yes || (confirmed && mode ask)`. CLI (`printBeadsClean` : enregistrement d'un projet, assistant d'init sans TUI) : `confirmed = stdinIsTerminal()` ; le plan liste l'annulation, la confirmation l'applique. Sans terminal : plan sans annulation (commit « gardé ») et rien n'est appliqué. Mode `auto` : fichiers nettoyés sans question, commit **gardé** (personne n'a confirmé).
- **TUI** (`offerBeadsClean` : ajout de projet, init de Beads) : la fenêtre Appliquer / Plus tard est une confirmation dans un terminal → l'annulation y est listée et appliquée avec « Appliquer ». **Écart à confirmer** par rapport au lot 5 de R (« jamais d'annulation depuis la TUI ») : lecture de la décision de l'utilisateur (« une fois présentée et confirmée »). Texte de la fenêtre (`tui_hint`) mis à jour.
- `oh doctor --fix` : plans calculés avec l'annulation quand `--yes` ou terminal ; confirmation par `cliConfirm` (remplaçable dans les tests, au lieu de `launcher.NewCLIUI`) ; sans terminal ni `--yes` : refus inchangé (`needs_yes`), rien n'est fait.
- `beads` : nouveau genre `undo_commit` pour le **plan** (`DryRun`) : « annulé s'il n'est pas poussé (dépôts distants récupérés juste avant) » — le plan ne fait pas de `git fetch` (lent, 20 s par distant) ; le `fetch` reste fait juste avant l'annulation, et un commit trouvé poussé est alors gardé (`skipped`). `uncommit` reste le genre du résultat. Inchangé : premier commit jamais annulé, distant injoignable → gardé.
- Textes : `commit_kept`, `commit_hint`, `tui_hint`, nouveau `kind.undo_commit` (fr/en). Doc : guide Premiers pas fr/en (encadré zéro impact), `reference/config` (`[beads] zero_impact`), `reference/cli-projects` (`--yes`), `reference/cli-infra` (`--fix`), CHANGELOG (puce N17).
- Tests : `beads.TestUndoCommitOnlyWhenAsked` (ex-`TestUndoCommitNeedsExplicitYes` : plan → `undo_commit` sans rien changer, puis `uncommit`) ; `cmd.TestPrintBeadsCleanCommit` (terminal confirmé → annulé et listé avant ; refusé → gardé ; sans terminal → gardé + indication ; `--yes` → annulé ; `auto` → gardé ; poussé → jamais, même avec une ref distante périmée et une confirmation) ; `cmd.TestDoctorFixUndoesCommitOnceConfirmed` (sans terminal → refus ; refusé → gardé ; confirmé → listé puis annulé).
