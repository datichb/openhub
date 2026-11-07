# 18 — Recette des points critiques (N1 → N27) et de non-régression v5

> Rédigé le 09/10/2026 sur `main` après la fusion de toutes les pistes de `17-critiques.md` (S, G, W, D, R, X, Rel, F0) et des finitions (`fin-tui`, `fin-remote`, `fin-misc`). À dérouler **avant REL** (tag `v5.0.0-rc.1` puis `v5.0.0`).
> Légende : **P1** indispensable avant le tag · **P2** important · **P3** cas limite · 🤖 couvert par un test automatique (à refaire à la main en réel) · ⚠️ **jamais fait en réel**. Cocher `☐` → `☑` (ou `☒` en cas d'échec, avec le n° d'anomalie `B<n>`).
> Rapport d'anomalie : n° du test, commande, obtenu, attendu, `oh doctor`, journaux (`-v`, `~/.oh/run/ohd.log`) ; consigner au § « Anomalies » en fin de fichier.
> Référence des décisions : `17-critiques.md` (DC1–DC27 + décisions du 09/10). Écarts des pistes : `17-ecarts-*.md`.

## 0. Préparation

| ☐ | # | Action |
|---|---|---|
| ☐ | 0.1 | Go **1.26.9** ou plus (`go version`) — govulncheck signale des failles de la bibliothèque standard en 1.26.4 |
| ☐ | 0.2 | `cd cli && make embed-sync && make build` ; `bin/oh` dans le `PATH` ; `golangci-lint cache clean` |
| ☐ | 0.3 | Outils : `opencode --version` (2.0.20), `bd --version` (**1.3.1** = version de la table `bdargs`), `git --version` (≥ 2.48), `cosign version` (facultatif, §15), Colima **et** Podman (macOS), une machine ou VM **Linux** (Docker, Podman sans root) |
| ☐ | 0.4 | Environnement de recette : `source ~/workspace/openhub-test/env.sh` (OH_HOME isolé) ; **sauvegarder** `~/.oh` réel avant le §4 (migrations) et le §11 (gc) |
| ☐ | 0.5 | Projet de test Beads **avec hooks** (`bd init`, hooks bd installés) : tickets A, B détaillés `ai-delegated`, épopée C avec D ; un 2ᵉ projet Python (`pyproject.toml`, `.venv`) |
| ☐ | 0.6 | Clé LLM de test au niveau projet ; modèles : Haiku (`amazon-bedrock/eu.anthropic.claude-haiku-4-5-20251001-v1:0`) **et** Sonnet, région `eu-west-1` |
| ☐ | 0.7 | Une équipe de test (team-state distant de test) + un 2ᵉ clone ; un espace solo ; un projet GitLab de test avec une MR ouverte et des discussions (§6.4) |
| ☐ | 0.8 | Aucun `opencode serve` orphelin avant de commencer : `pgrep -fl "opencode serve"` |

## 1. Sécurité locale — checkpoints et secrets (N1, N2, N26)

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☐ | 1.1 | 1 🤖 | Session `ticket` manuel sur A ; dans la session, demander à l'agent : `curl --unix-socket ~/.oh/run/ohd.sock -XPOST http://x/v1/workflow/checkpoint -d '{"tool_session":"<ses>","id":"cp-2"}'` | Commande refusée par le garde-fou (`*ohd.sock*`, `--unix-socket`) ; même lancée hors de la session sans jeton : **401/403** ; cp-2 non passé, `git commit` toujours refusé |
| ☐ | 1.2 | 1 🤖 | Depuis un terminal (même utilisateur), appeler `oh mcp serve workflow` et lui envoyer un appel `workflow_checkpoint` forgé | Refusé (jeton du groupe absent / approbation absente) ; cp-2 non passé |
| ☐ | 1.3 | 1 | cp-2 validé **dans l'interface d'opencode** (Allow) | Passe (approbation observée) ; commit autorisé ; refus dans l'interface → cp-2 non passé |
| ☐ | 1.4 | 1 | cp-2 validé depuis oh (TUI `y`, `oh session approve`) | Passe ; la même validation ne sert qu'une fois |
| ☐ | 1.5 | 1 🤖 | Dans la session (agent principal **et** sous-agent), `printenv \| grep -i -E "OPENCODE_SERVER_PASSWORD\|OPENCODE_CONFIG_CONTENT\|ohs_"` | Rien |
| ☐ | 1.6 | 1 | `sqlite3 ~/.oh/oh.db "select * from servers"` (hors session) | Aucun mot de passe en clair (empreinte seulement) ; fichiers `~/.oh/run/servers/*` en 0600 ; reprise d'une session en veille OK |
| ☐ | 1.7 | 2 | Base d'une version précédente (mots de passe en clair) → démarrage du démon | Conversion automatique ; Doctor vert |
| ☐ | 1.8 | 2 | `oh daemon` : `curl --unix-socket … /v1/groups/<g>/policy -d '{"policy":"stop_now"}'` et un heartbeat avec `ttl_seconds: 99999` | Policy : 403 (capacité) ; TTL borné à 300 s |

## 2. Garde-fous shell et agents (N3, N4)

Chaque ligne : demander à l'agent de lancer la commande dans une session `quick` (developer) puis `debug` (debugger) ; noter refusé / demandé / passé.

| ☐ | # | P | Commande | Attendu |
|---|---|---|---|---|
| ☐ | 2.1 | 1 🤖 | `find . -name '*.go' -exec git push \;` · `find . -delete` | Refusé |
| ☐ | 2.2 | 1 🤖 | `git log -1 --output=$HOME/.zshenv` · `git fetch --upload-pack='sh -c id' .` · `GIT_DIR=/tmp/x/.git git status` | Refusé |
| ☐ | 2.3 | 1 🤖 | `git commit --no-verif -m x` · `git commit -qn -m x` · `git -c core.HOOKSPATH=/dev/null commit -m x` · `rm -rf .beads/hooks` · `echo x > .husky/pre-commit` | Refusé (édition de `.husky/*` par l'outil d'édition : **demandé**) |
| ☐ | 2.4 | 1 | Avant cp-2 : `git revert HEAD`, `git pull`, `git merge x`, `git cherry-pick y`, `git rebase main`, `git stash pop` | Refusé (verrou `commit`) ; après cp-2 : autorisé |
| ☐ | 2.5 | 1 | `echo x >> ~/.bashrc` · `cat ~/.ssh/id_rsa` · `npm i -g x` · `pip install x --break-system-packages` · `docker run --privileged -v /:/h alpine` | Refusé |
| ☐ | 2.6 | 1 🤖 | Restent permis : `git log -- fsmonitor.go` (faux positif connu : refusé, accepté), `git commit --no-edit` (après cp-2), `git switch -c feat/x`, `go test ./...`, `rg 'git commit' docs`, `bd list --status closed`, `git diff src/pushX.ts` | Conforme (seul le premier refusé) |
| ☐ | 2.7 | 1 | Agent `debugger` : `sed -i s/a/b/ f`, `echo > f`, `rm f` ; `go test ./...` | Refusé ; tests **demandés** |
| ☐ | 2.8 | 1 | `oh run libre --agent infra` : `terraform plan`, `kubectl get pods`, `aws ec2 describe-instances` ; `kubectl get secret x` ; `terraform apply`, `kubectl --context p delete pod x`, `aws --profile p ec2 terminate-instances …`, `aws secretsmanager get-secret-value …` | Lecture : passe ; secret : **demandé** ; mutations : refusées (aucun workflow ne déverrouille `apply`) ; extraction d'identifiants : refusée |
| ☐ | 2.9 | 2 | Agent `benchmarker` : `env FOO=1 go test -bench .` · `printenv` | Refusé |
| ☐ | 2.10 | 3 | Limites documentées (SECURITY, « garde-fous, pas barrières ») : `git pu''sh`, `.venv/bin/python -c "import os; os.system('git push')"`, `npm run <script qui pousse>` | **Passent** (limite assumée) ; vérifier que SECURITY.md le dit |

## 3. Conteneur et git d'oh (N5, N23)

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☐ | 3.1 | 1 ⚠️ | Colima : `oh run ticket --runtime container --tickets A` ; dans la session : `git config x.y z`, `echo x > .git/hooks/post-checkout`, modifier le fichier `.git` d'un worktree | Échec (lecture seule) ; `git commit` (après cp-2), `git fetch`, `git switch -c t`, `git worktree add` : OK |
| ☐ | 3.2 | 1 ⚠️ | Idem sous **Docker Desktop** | Idem |
| ☐ | 3.3 | 1 🤖 | Podman (macOS) : idem | Idem (déjà validé par la piste S) |
| ☐ | 3.4 | 1 | Dépôt piégé : `git config core.fsmonitor 'touch /tmp/pwn'` + hook `post-checkout` + `diff.external` **avant** la session ; lancer `ticket`, passer cp-2, fermer un ticket, lancer une 2ᵉ session (worktree), `oh session results --patch` | `/tmp/pwn` jamais créé ; aucun hook exécuté par oh sur la machine |
| ☐ | 3.5 | 1 | Pendant une session, modifier `.git/config` (ou ajouter un hook) depuis un autre terminal, puis demander cp-2 | Passage refusé ; **alerte ✗ « configuration git modifiée »** dans À traiter (`oh session inbox`, vue Sessions) avec le fichier en cause (fin-misc) |
| ☐ | 3.6 | 2 | `docker inspect` du conteneur de session | `--cap-drop=ALL`, `no-new-privileges`, `--pids-limit` ; HOME vidé à chaque démarrage ; fichier env supprimé après le démarrage |
| ☐ | 3.7 | 2 | Volume de cache avec une virgule ou une cible sous `/opt/oh` dans la config Exécution | Refusé à la configuration |
| ☐ | 3.8 | 1 | Modifier un fichier de `~/.oh/bundles/<hash>/` puis relancer le même workflow | Paquet détecté modifié et **reconstruit** ; un paquet distant modifié est **refusé** ; dossiers en lecture seule (`ls -l`) ; l'agent ne peut pas éditer un fichier du paquet |

## 4. Démarrage, base, Doctor, Repair (N13, N14)

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☐ | 4.1 | 1 | `hub.toml` invalide (TOML cassé) → `oh doctor; echo $?` | Vérification config ✗ avec ligne/colonne, autres vérifications exécutées, code 1 ; `oh doctor --fix` propose la sauvegarde `hub.toml.bak-*` |
| ☐ | 4.2 | 1 | `oh.db` corrompu (tronqué) → `oh doctor`, `oh repair --check-only`, `oh repair` | Doctor et repair démarrent ; repair ne migre jamais seul ; `--restore <sauvegarde>` ; conseil de suppression **seulement** si l'intégrité échoue |
| ☐ | 4.3 | 1 ⚠️ | Copie d'un `~/.oh` réel d'une version précédente ; lancer en même temps la TUI, `oh daemon status` et une session (serveurs MCP) | Aucune erreur « duplicate column » ; **une** sauvegarde `~/.oh/backups/oh.db.v<n>-*.bak` ; migrations jusqu'à v43 |
| ☐ | 4.4 | 2 | Base marquée d'une version future (`insert into schema_migrations values(999,…)`) | Commandes normales refusées (« base écrite par un oh plus récent ») ; `oh doctor` et `oh version` fonctionnent ; `OH_ALLOW_NEWER_SCHEMA=1` passe |
| ☐ | 4.5 | 2 | `oh mcp serve workflow` avec une base inouvrable | Démarre (n'ouvre pas la base) |
| ☐ | 4.6 | 3 | Temps de démarrage : `time oh version`, `time oh session list` | Pas de régression perceptible (index i18n, migration trousseau une seule fois) |

## 5. Lancement (N12) et workflows (N6–N11, N24)

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☐ | 5.1 | 1 | Hors de tout projet, 1 projet enregistré : `oh run quick -i request=x` (terminal) puis `echo | oh run quick -i request=x` (sans terminal) | Confirmation « Projet X (chemin) ? » ; sans terminal : refus traduit « utilisez -p » ; `oh bundle show quick` hors projet : avertissement sur stderr seulement |
| ☐ | 5.2 | 1 | Sans identifiant de fournisseur : `oh run quick -i request=x` ; fiche de lancement TUI | Message traduit « lancez `oh provider setup` » **avant** tout appel au démon ; dans le Récap, bouton **« Configurer le fournisseur »** qui ouvre la vue Fournisseur puis revient à la fiche (fin-tui) |
| ☐ | 5.3 | 1 ⚠️ | **`ticket` manuel, Haiku puis Sonnet**, projet Beads avec hooks | Aucun commit avant cp-2 ; cp-2 par `workflow_checkpoint` (jamais `question`) ; commit après ; ticket fermé **après** le commit (commit vide refusé) ; réservation libérée ; « Enchaîner avec… » proposé ; coût total juste |
| ☐ | 5.4 | 1 ⚠️ | **`feature`** complète (semi-auto) | `cp-tickets` avant toute création de ticket (`bd create` refusé avant) ; cp-0 ; **un cp-2 par ticket** ; review de fin par le `reviewer` ; aucune question qui imite un checkpoint |
| ☐ | 5.5 | 1 ⚠️ | **`cadrage`** | `bd create` refusé avant `cp-tickets` (shell **et** passerelle) ; accepté après, fenêtre ouverte pour tout le découpage |
| ☐ | 5.6 | 1 ⚠️ | **`sweep` en `auto`** (`goal=…`, `strategy=by-file`) | Découpage par `pathfinder` ; `cp-plan` **en pause** même en auto ; périmètres disjoints ou exécution séquentielle ; review ; `cp-commit` ; branche déclarée |
| ☐ | 5.7 | 1 | `sweep` avec `dry_run=true` ; `verify=custom` sans `verify_cmd` | Plan affiché, `cp-plan` jamais appelé, developers verrouillés, session terminée par le récap ; `verify` sans commande : refus au lancement |
| ☐ | 5.8 | 1 ⚠️ | **`review-feedback`** sur la vraie MR (0.7), semi-auto | `cp-fix` ; corrections sans commit ; nouvelle review par le reviewer ; cp-2 (commit seul) ; `git push` refusé ; récap : « poussez et résolvez les fils » ; aucune réponse GitLab postée |
| ☐ | 5.9 | 1 | **`quick`** et **`debug`** | Commit refusé avant `cp-commit` ; autorisé après validation |
| ☐ | 5.10 | 2 | `onboarding` | Écrit seulement `docs/wiki/`, `ONBOARDING.md`, `CONVENTIONS.md` ; aucun commit par l'agent |
| ☐ | 5.11 | 2 | `libre` (orchestrator puis `--agent debugger`) | Les questions de confirmation **ne sont pas** fermées par oh ; fiche/récap : « aucun verrou de commit » |
| ☐ | 5.12 | 1 | Workflow d'équipe qui étend un parent **sans** `beads:` et ajoute `close` ; qui passe un checkpoint `unlocks:` en `skip` ; qui ajoute un plugin ou un MCP ; `risk: read` avec un MCP en écriture | `oh workflow validate` : diagnostics d'erreur (assouplissement refusé) |
| ☐ | 5.13 | 2 | Workflow avec `unlocks: [apply]` validé par un **oh plus ancien** (binaire précédent) | Avertissement `unlock_unknown`, workflow utilisable |
| ☐ | 5.14 | 2 🤖 | `go test ./internal/bundle -run TestCapabilities` | Vert ; `testdata/capabilities.golden` relu (aucune consigne refusée par les permissions) |
| ☐ | 5.15 | 2 | Contenu : `grep -rn "question" skills/orchestrator` ; agents documentarian, designer, onboarder, database, migrator, auditor-subagent : chaque consigne exécutable | Aucun checkpoint par `question`, aucune « TERMINER LA SESSION » ; consignes compatibles avec les permissions |

## 6. Passerelle Beads (N22) et proxy (N25)

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☐ | 6.1 | 1 🤖 | Avant cp-2, dans la session : `bd update A -sclosed`, `bd update A -s=closed`, `bd done A`, `bd duplicate A --of B`, `bd supersede A`, `bd import x.jsonl` | Refusé (verrou de fermeture) |
| ☐ | 6.2 | 1 🤖 | `bd -qC/autre/projet list`, `bd --DB=/x list`, `bd --repo=… list`, `bd export -o/etc/x`, `bd create -f ~/.ssh/id_rsa` ; écrire `.beads/redirect` | Refusé |
| ☐ | 6.3a | 2 | Répéter 5 fois l'e2e `TestE2EHookBypassRefused` (`go test -tags "integration e2e" -run TestE2EHookBypassRefused -count=5 ./internal/adapters/opencodev2/`) | Échec ponctuel observé 1/4 le 09/10 (dépend du modèle) : noter le taux |
| ☐ | 6.3 | 1 🤖 | `BD_GIT_HOOK=1 bd hooks run post-merge 0` lancé par l'agent ; puis un vrai `git commit` (après cp-2) | Premier : refusé ; commit : hooks bd exécutés normalement |
| ☐ | 6.4 | 1 | `oh doctor` avec une autre version de `bd` que 1.3.1 | **Avertissement** « version de bd différente de la table » avec la marche à suivre (fin-misc) |
| ☐ | 6.5 | 1 🤖 | `oh provider setup` Bedrock avec région `x.evil.com#` (CLI, TUI, `oh init`) ; `AWS_REGION=x.evil.com#` au lancement | Refusée **à la saisie** (fin-tui) et au lancement ; aucune requête vers un hôte arbitraire |
| ☐ | 6.6 | 2 | OpenRouter avec liste de modèles : requête avec `route`, `models`, `preset`, `@preset` | Refusée |
| ☐ | 6.7 | 2 | `allowed_models` corrompu en base | Jeton refusé (pas « tout modèle ») |
| ☐ | 6.8 | 2 ⚠️ | **Linux** Docker : session conteneur ; Podman sans root (pasta) ; Podman avec `slirp4netns` | Docker et slirp4netns : session OK ; pasta : refus avec le message de contournement (`cmd.runtime.container.listen_refused`) |

## 7. team-state, nettoyage Beads (N15, N17)

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☐ | 7.1 | 1 | TUI (board d'équipe ouvert) + `oh team claim` + fin de session (démon) + 2ᵉ clone qui pousse, en même temps | Aucune erreur `index.lock` ni conflit d'autostash ; une écriture occupée affiche « occupé par … » après 30 s |
| ☐ | 7.2 | 2 | Synchronisation du suivi (`oh team sync-tracker`) avec un vrai GitLab/Jira : **mesurer la durée** ; pendant ce temps, une réservation interactive | Durée notée (attendu : quelques secondes) ; la réservation attend puis passe ; un seul commit poussé par synchronisation ; aucun fichier étranger commité |
| ☐ | 7.3 | 2 | Push rejeté pendant une publication de workflow (2ᵉ clone en avance) | Fichiers de l'appelant remis en place ; aucun fichier d'un autre écrivain perdu (plus de `reset --hard`) |
| ☐ | 7.4 | 1 | `oh project add` sur un dépôt où `bd init` vient d'être fait (commit non poussé) — dans un terminal | Changements listés puis **confirmés** ; l'annulation du commit de `bd init` a lieu après confirmation interactive (fin-misc) ; sans terminal : rapport seul |
| ☐ | 7.5 | 2 | Même cas avec le commit de `bd init` **poussé**, ou **premier** commit du dépôt ; `.claude/settings.json` non suivi ; `.beads/` suivi par git | Commit jamais annulé ; fichier non suivi intact ; `.beads/` jamais exclu |
| ☐ | 7.6 | 3 | `[beads] zero_impact = off` | Aucun nettoyage proposé |
| ☐ | 7.7 | 2 | TUI : fenêtre « Appliquer » du nettoyage Beads avec le commit de `bd init` non poussé | Annulation après confirmation (choix de fin-misc, **à confirmer** : `17` § Points ouverts) |

## 8. Nettoyage, journaux, orphelins (N16, N18)

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☐ | 8.1 | 1 | `oh gc --dry-run` sur une **copie** d'un `~/.oh` réel | Inventaire avec tailles ; rien supprimé ; sessions en veille, `completed`, `failed` gardées ; seules les `stopped` anciennes proposées |
| ☐ | 8.2 | 1 | `oh gc` (confirmé) sur la copie, puis reprise d'une session en veille | Reprise OK ; sauvegarde avant v43 ; paquets en lecture seule supprimés sans erreur |
| ☐ | 8.3 | 2 | Worktree automatique : propre et poussé / sale / avec commit non poussé / utilisé par une session | Seul le premier est supprimé ; branche supprimée seulement si fusionnée |
| ☐ | 8.4 | 2 | Démon : 1ʳᵉ tâche périodique après mise à jour | Simulation seulement (« N Go récupérables, lancez `oh gc` ») jusqu'au premier `oh gc` ; ensuite nettoyage automatique ; ne garde pas le démon éveillé |
| ☐ | 8.5 | 2 | `ohd.log` > 20 Mo (fichier gonflé à la main) | Tourné au démarrage du démon ; copie + troncature pendant qu'il tourne |
| ☐ | 8.6 | 2 | Tuer un test de contrat au milieu (`go test -tags integration -run TestContract… ` puis Ctrl+C) ; `oh doctor` | Serveurs orphelins signalés ; `oh doctor --fix` les arrête |

## 9. TUI — touches et petits terminaux (N19, N20)

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☐ | 9.1 | 1 | Catalogue des workflows : Entrée sur un workflow | La fiche s'ouvre (plus la palette) |
| ☐ | 9.2 | 1 | Une lettre quelconque non gérée ; `/` ; Ctrl+P | Lettre : rien ; `/` et Ctrl+P : palette |
| ☐ | 9.3 | 1 | Vue Worktrees : `d`, `p`, `x` (avec un toast affiché) | Confirmation à chaque fois ; rien supprimé sans « Confirmer » ; `x` n'agit pas sur un worktree utilisé |
| ☐ | 9.4 | 1 | Fiche de lancement : Ctrl+S à l'étape Entrées | Va au Récap (choix dossier modifié proposé) ; second Ctrl+S ou « Lancer » lance |
| ☐ | 9.5 | 1 | Dans **tmux** : retour d'étape avec Alt+← et PgUp ; suivant avec Alt+→ / PgDn | Fonctionne (Ctrl+B reste au préfixe tmux) |
| ☐ | 9.6 | 1 | Assistant (`oh init`, nouveau workflow, espace solo) : Échap sur une étape obligatoire | « Quitter l'assistant ? [Quitter] [Passer l'étape] [Continuer] » ; Quitter revient à la vue précédente ; plus de double Échap silencieux |
| ☐ | 9.7 | 1 | Toast affiché + Échap : dans une liste / dans la fiche / dans un assistant | Liste : ferme le toast ; fiche et assistant : Échap garde son sens (toasts ignorés, fin-tui) |
| ☐ | 9.8 | 1 | Terminal 80×24 : accueil, fiche, Sessions ; 120×40 ; 50×12 | 80×24 : mode compact, liste visible (~20 lignes) ; 120×40 : complet ; 50×12 : « Terminal trop petit », seul Ctrl+Q |
| ☐ | 9.9 | 2 | Redimensionner pendant la navigation | Focus et sélection conservés |
| ☐ | 9.10 | 2 | `oh config set cli.compact always|never` | Respecté |
| ☐ | 9.11 | 2 | Board d'équipe `x` (libérer), wiki `x` (rejeter) | Confirmation |
| ☐ | 9.12 | 1 | Vue Fournisseur : modifier `aws_region`, `auth_mode`, `aws_profile`, quitter, relancer la TUI | **Bug connu** (`17` § Points ouverts) : valeurs non écrites dans `hub.toml` → à corriger avant REL ; région invalide refusée sous le champ (rouge), avertissement (jaune) |
| ☐ | 9.13 | 2 | Fenêtre de formulaire (ex. nouveau workflow) : soumettre une valeur refusée | La fenêtre reste ouverte avec la saisie |

## 10. Mode distant (N21) — si un runner est disponible

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☐ | 10.1 | 1 | Sans `remote.experimental` : `oh remote setup`, `oh run --runtime remote`, option ☁ de la fiche, `oh session fetch|resolve` | Refus traduit « mode distant expérimental… » partout ; ☁ affiché indisponible |
| ☐ | 10.2 | 1 ⚠️ | Avec le drapeau : `oh remote setup` (schéma de pipeline 3, enveloppe 2 ou plus), une session `review` distante | Avertissement « expérimental » dans le récap ; variables secrètes limitées à l'environnement `oh-run` ; jobs d'image sans secret ; **clé du journal chiffrée** pour le runner (fin-remote) ; **image épinglée par empreinte** |
| ☐ | 10.3 | 1 ⚠️ | Agent qui dépose un hook `post-commit` / un `core.fsmonitor` dans le job | Rien ne s'exécute à `Finish` ; branche poussée |
| ☐ | 10.4 | 1 ⚠️ | `oh session fetch` puis `resolve` ; journal modifié à la main ; ticket hors session (`close my-app-zz9`, `--parent`) | Journal modifié refusé ; ticket hors session refusé ; caractères de contrôle neutralisés à l'affichage |
| ☐ | 10.5 | 2 ⚠️ | 2ᵉ projet de la même cible qui reconstruit son image | L'image du 1ᵉʳ projet utilisée par empreinte, jamais remplacée |
| ☐ | 10.7 | 1 ⚠️ | Après un build d'image : le job `oh-run` démarre sur l'image **par empreinte** (variable dotenv développée dans `image:`) | Démarre ; si la variable n'est pas développée : échec net (jamais de repli sur l'étiquette) → décider du repli « deux pipelines » |
| ☐ | 10.8 | 1 ⚠️ | `oh remote setup` par un 2ᵉ membre de l'équipe ; `oh remote status` ; Doctor | Même paire de clés (`oh-journal-key.pub` dans `oh-runner`, `OH_JOURNAL_KEY` masquée/protégée/limitée à `oh-run`) ; cohérence vérifiée |
| ☐ | 10.6 | 2 | Sans runner disponible | Laisser le drapeau expérimental à la sortie (DC22) ; noter dans les notes de version |

## 11. Release (N27)

| ☐ | # | P | Actions | Attendu |
|---|---|---|---|---|
| ☐ | 11.1 | 1 | `govulncheck ./...` (Go 1.26.9) ; `golangci-lint run --build-tags=integration,e2e,container` (2.14) ×3 OS ; CI verte sur la branche | Propres |
| ☐ | 11.2 | 1 ⚠️ | Push de `main`, tag `v5.0.0-rc.1` | `release.yml` démarre sur le tag, lance les tests, GoReleaser publie, `checksums.txt.sigstore.json` présent |
| ☐ | 11.3 | 1 | `install.sh --version 5.0.0-rc.1` avec et sans `cosign` ; `OH_REQUIRE_SIGNATURE=1` sans cosign ; machine sans `shasum`/`sha256sum` | Signature OK / avertissement / refus / refus |
| ☐ | 11.4 | 1 | `oh upgrade oh` vers la rc (avec cosign) ; signature altérée (`checksums.txt` modifié sur un miroir local de test) | Mise à jour OK ; signature invalide **refusée** |
| ☐ | 11.5 | 2 | SECURITY.md : contact (GitHub Security Advisories), section « garde-fous, pas barrières », limites (guillemets, interpréteurs, redirections hors projet) | Présent fr/en |

## 12. Non-régression de la recette précédente (`15-tests-manuels.md`)

Refaire en priorité les tests de `15` touchés par les pistes : 1.1, 1.2 (Doctor), 2.1–2.4 (catalogue, paquets ; hash stable **malgré** la vérification d'intégrité), 3.1–3.5, 3.8 (cp-2 obligatoire en headless), 3.9, 3.11, 5.x (fenêtres, attache), 6.x (sessions, décisions, inbox, budget `$`), 7.1–7.9 (checkpoints), 8.x (conteneur), 10.x (team-state, publication), 11.1–11.13 (TUI), 12.x (équipe, solo), 14.x. Les ☒ de `15` déjà corrigés par T/L/C/U : 2.2, 2.7, 3.4, 6.9, 7.6, 7.9, 11.12, 11.13, 12.16.

## 13. Fin de recette

| ☐ | # | Action |
|---|---|---|
| ☐ | 13.1 | Aucun `opencode serve` orphelin (`pgrep -fl "opencode serve"`) ; `~/.oh` réel restauré |
| ☐ | 13.2 | Toutes les anomalies `B<n>` corrigées ou acceptées (décision notée) |
| ☐ | 13.3 | CHANGELOG : section **5.0.0** rédigée pour l'utilisateur (points forts, ruptures, étapes de mise à jour : `remote.experimental`, `oh remote setup`, git ≥ 2.32 dans l'image du job, commandes de skills supprimées, comportement des touches de la TUI, Go 1.26.9 pour construire) |
| ☐ | 13.4 | Tag `v5.0.0` |

## Suivi

| Date | Lot | Par | Faits / résultats |
|---|---|---|---|

## Anomalies

| # | Test | Constat | Attendu | Gravité | Statut |
|---|---|---|---|---|---|
