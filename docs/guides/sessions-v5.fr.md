> [Read in English](sessions-v5.en.md)

# Sessions sur le runtime v5 (opencode V2)

Toutes les sessions passent par le runtime v5 : `oh run <workflow>` (y compris `--tickets` et `--headless`), les lancements depuis la TUI et les alias dépréciés (`oh start`…). Il demande opencode V2 (≥ 2.0.0) : opencode V1 n'est plus pris en charge (voir le [guide de migration v5](migration-v5.fr.md)).

## Fonctionnement

| | Runtime v5 |
|---|---|
| Agents et skills | compilés dans un paquet de session hors du projet (`~/.oh/bundles/<hash>`), à partir du workflow ; rien n'est déployé dans le projet |
| Agents et skills visibles | seulement ceux du paquet (« monde fermé », vérifié à chaque démarrage ; en cas d'échec, la session ne démarre pas) |
| Clé LLM | jamais transmise à opencode : c'est le proxy du démon oh qui la détient |
| Serveur | un `opencode serve` par groupe (version du paquet, projet, runtime), partagé par les sessions du groupe |
| Fenêtre de session | nouvel onglet ou nouvelle fenêtre de terminal ; oh reste utilisable |
| Fermeture de la fenêtre | la session continue ; on la rouvre avec `oh session attach` |

## Ouverture d'une session

Le client de la session s'ouvre avec la première méthode qui fonctionne, dans cet ordre :

1. iTerm2 (onglet, panneau ou fenêtre : `iterm_style`)
2. Terminal.app
3. tmux (nouvelle fenêtre quand oh tourne dans tmux)
4. navigateur (interface web d'opencode)

La suspension de oh n'est qu'un dernier recours (`attach = "suspend"`).

## Configuration

`~/.oh/hub.toml` :

```toml
[session]
attach = "auto"            # auto | iterm | terminal | tmux | browser | suspend
iterm_style = "tab"        # tab | split | window
idle_sleep_minutes = 5     # un serveur inactif se met en veille après N minutes
```

`OH_SESSION_ATTACH` remplace `attach` le temps d'une commande.

## Veille et reprise

- Un serveur se met en veille quand **aucune** session ne travaille, qu'aucun client n'est attaché et que rien ne s'est passé depuis `idle_sleep_minutes`. Une décision qui vous attend (permission, question) garde le serveur éveillé tant que oh est ouvert.
- Une session en veille reprend quand vous la rouvrez : `oh session attach <id>` redémarre son serveur, puis ouvre le client.
- Si vous quittez la TUI (Ctrl+Q ou commande `quit`) pendant que des sessions travaillent, oh demande quoi faire **pour chacune** : « finir l'étape, veille » (défaut), « arrière-plan » ou « arrêter maintenant ». Échap annule. Les sessions en attente ou inactives sont mises en veille tout de suite.
- Au retour, un message résume ce qui s'est passé pendant votre absence (sessions changées, décisions arrivées, coût).

## Commandes

| Commande | Rôle |
|---|---|
| `oh session list [--all] [--json]` | sessions, leur état (active, en attente, inactive, en veille, arrêtée) et leurs décisions en attente |
| `oh session inbox [--json]` | décisions en attente de toutes les sessions : `⏸` checkpoint, `?` question, `!` permission, `$` budget, `✗` erreur |
| `oh session approve <id> [--decision once\|always\|reject] [-m "…"]` | répondre à une permission sans ouvrir la session (`always` refusé en isolation stricte) ; pour un checkpoint : `once`, `fix`, `other` (avec `-m`) ou `reject` |
| `oh session answer <id> --field clé=valeur…` | répondre à une question de l'agent ; sans `--field`, affiche les champs attendus |
| `oh session dismiss <id>` | classer une alerte (erreur, budget) |
| `oh session send <id> "…" [--queue] [--synthetic]` | envoyer une consigne courte (prise en compte à la prochaine étape, ou après l'étape avec `--queue`) |
| `oh session follow <id>` | suivre une session en direct, en lecture seule (Ctrl+C pour quitter) |
| `oh session interrupt\|compact <id>` | interrompre l'étape en cours, compacter l'historique |
| `oh session model <id> <fournisseur/modèle>` | changer le modèle des prochaines étapes |
| `oh session fork <id>` | créer une variante (copie de l'historique, même serveur) |
| `oh session results <id> [--mr] [--patch] [--json]` | fichiers modifiés, branche, coût ; description de MR ; diff |
| `oh session attach <id> [--how auto\|iterm\|terminal\|tmux\|browser\|suspend]` | ouvrir (ou reprendre) une session |
| `oh session open <id> --browser [--print]` | ouvrir une session dans le navigateur (code à usage unique, 5 min) |
| `oh session fetch <id> [--no-import]` | récupérer une session distante terminée (artefacts, import pour la reprendre) — [exécution distante](remote-runners.fr.md) |
| `oh session resolve <id> [--dry-run] [--yes]` | rejouer son journal Beads (conflits, confirmation) |
| `oh session resume <id>` | reprendre une session en veille sans ouvrir d'interface |
| `oh session stop <id>` | arrêter une session, et son serveur si aucune autre session ne l'utilise |
| `oh daemon status` | état du démon (serveurs, jetons de proxy) |
| `oh daemon stop [--force]` | arrêter le démon (refusé si des sessions tournent, sauf `--force`) |
| `oh doctor` | vérifications v5 : runtime, démon, git, terminal |

Dans ces commandes, `<id>` peut être un début d'identifiant (`oh session follow dRcJ`) ; `approve`, `answer` et `dismiss` acceptent aussi l'identifiant d'une décision (affiché par `inbox`) quand une session en a plusieurs. **Première réponse gagne** : si la décision a déjà été prise dans l'interface opencode ou le navigateur, oh le dit et n'envoie rien.

## Vue Sessions (TUI)

Ouvrez-la depuis l'omnibar (`sessions`) ou les sections « Sessions » des pages d'accueil, de projet et d'équipe. La barre du bas affiche partout `● N ⏸ M` (sessions vivantes, décisions en attente).

- **À traiter** : décisions de toutes les sessions. `Entrée` ouvre la fiche (permission : une fois / toujours / refuser + message ; question : formulaire généré ; alerte : classer ou attacher), `y`/`n` valident ou refusent une permission, `x` classe une alerte.
- **En cours, En veille, Terminées (7 j), À récupérer** (sessions distantes terminées) ; détail de la session sélectionnée en bas.
- `t` (ou `Entrée` sur une session) : flux en direct à droite (agent, outils, messages, coût). `a` attacher, `A` choisir comment ouvrir (iTerm2, Terminal.app, tmux, navigateur, ici), `m` consigne, `i` interrompre, `M` modèle, `s` arrêter, `c` reprendre, `o` résultats et description de MR, `w` navigateur, `e` enchaîner un autre workflow, `g` récupérer une session distante, `f` projet actif / tous les projets, `r` rafraîchir.

Avec tmux, la session s'ouvre dans une nouvelle fenêtre ; avec `[session] iterm_style = "split"`, dans un volet à côté.

## Checkpoints

Une session lancée depuis un workflow passe ses checkpoints par l'outil `workflow_checkpoint` (serveur MCP `workflow` d'oh, ajouté à chaque paquet). C'est oh qui applique le mode :

- checkpoint **automatique** dans le mode : il passe sans demande ;
- checkpoint **en pause** : il apparaît dans « À traiter » (⏸) et l'agent attend. `Entrée` ouvre la fiche : résumé de l'agent, changements (`Diff complet`), derniers messages, frise ; puis **Décider** : *Valider*, *Corriger d'abord* ou *Autre consigne*, avec un message à l'agent (obligatoire pour les deux derniers). `y` valide directement, `n` ouvre la fiche sur « Corriger d'abord ». La validation reste possible dans l'interface opencode : la première réponse gagne ;
- un agent verrouillé par `after:` dans le workflow est refusé tant que son checkpoint n'est pas passé ;
- **coupe-circuit** (`circuit_breaker`) : après N délégations d'affilée sans intervention, les délégations sont suspendues et une alerte ✗ apparaît ; `x` (ou `oh session dismiss`) la classe et les débloque.

Le détail de la session affiche la frise : `✔ cp-1 10:03 → developer (3) → ⏸ cp-2 → ○ cp-3`.

En ligne de commande : `oh session approve <id>` valide (`--decision once`), `--decision fix -m "…"` ou `--decision other -m "…"` refuse avec une consigne, `--decision reject` refuse sans consigne.

## Notifications

Le démon oh affiche une notification système quand une décision vous attend et quand une session finit son étape sans que personne n'y soit attaché. Les notifications proches sont regroupées et ne contiennent jamais le contenu de la session. Avec [`terminal-notifier`](https://github.com/julienXX/terminal-notifier) installé, un clic ramène le terminal d'oh ; sinon oh utilise `osascript` (ou `notify-send` sous Linux). Pour les couper : `[session] notify = "off"` dans `hub.toml` (pris en compte au prochain démarrage du démon).

## Clés LLM

La clé est cherchée dans cet ordre : projet, équipe, hub, puis (Bedrock uniquement) le profil AWS avec signature SigV4. Une clé d'équipe se saisit dans **Détail équipe**, touche `K`. Elle est rangée dans le trousseau sous `openhub.team.<équipe>.provider.<fournisseur>.token`.

La région Bedrock vient de la config oh, puis de `AWS_REGION` / `AWS_DEFAULT_REGION`, puis du profil AWS. Si aucune n'est définie, oh utilise `us-east-1` et l'indique dans ses logs.

Changer le fournisseur, la région ou la clé d'un projet démarre un nouveau serveur au lancement suivant. Les sessions en cours gardent leurs réglages jusqu'à la mise en veille de leur serveur.

## Restrictions

Désactivées par défaut. Elles limitent les sessions de la machine :

| Restriction | Effet |
|---|---|
| `max_active_sessions` | nombre de sessions dont l'agent travaille en même temps ; au-delà, une nouvelle session attend dans la file (état `en file`, sessions interactives d'abord) et démarre dès qu'une place se libère |
| `session_budget_usd` | budget d'une session, sous-agents compris |
| `daily_budget_usd` | budget de toutes les sessions d'une journée (du projet s'il est réglé pour un projet) ; une fois dépensé, pas de nouvelle session |
| `memory_mb` | mémoire des serveurs de sessions (estimation dans `oh doctor`) ; au-delà, les nouvelles sessions attendent, les groupes inactifs sont mis en veille et oh avertit une fois |
| `models` | modèles autorisés (motifs sur l'identifiant envoyé au fournisseur, ex. `eu.anthropic.*`) ; le proxy refuse les autres |

Elles se règlent en cascade : `hub.toml` `[limits]` (`oh budget set …`, ou **Réglages → Restrictions des sessions**), le `config.toml` de l'équipe (`[limits.recommended]`, `[limits.enforced]`), le projet (`oh budget set … --project <p>`), puis le workflow (`limits:`). La valeur la plus précise l'emporte ; une valeur imposée par l'équipe est un plafond que personne ne peut relâcher. `oh budget show [-p <projet>]` affiche les valeurs effectives, leur origine et les dépenses du jour.

Les budgets sont des **plafonds souples**, vérifiés sur le coût indiqué par l'outil : l'étape qui dépasse un budget se termine, puis une décision `$` apparaît dans « À traiter ». Tant qu'elle est ouverte, toute nouvelle étape de la session est interrompue. Réponses : `oh budget raise <session> [montant]` (par défaut : le budget configuré une fois de plus), `oh session stop <session>`, ou classer (une étape de plus, la décision revient après). Les dépenses sont gardées dans `oh.db` (registre d'usage) : elles survivent aux redémarrages du démon et aux cycles veille/reprise.

## Variables d'environnement

| Variable | Effet |
|---|---|
| `OH_HOME` | déplace `~/.oh` (environnements de test) |
| `OH_SESSION_ATTACH` | remplace `[session] attach` |

## Limites

- **Windows** (sessions locales seulement) : pas de démon en arrière-plan ; le proxy d'identifiants et le suivi des sessions tournent dans le processus oh (la TUI, ou la commande qui a ouvert la session dans le terminal courant). Les sessions ne tournent que tant que cet oh est ouvert : en le quittant, il attend la fin des étapes choisies, puis met les sessions en veille (reprise avec `oh session attach`). `oh doctor` le rappelle. Pour des sessions qui survivent à oh, utilisez WSL.
- **Port du proxy** : le démon garde le port de son proxy d'un redémarrage à l'autre. Si un autre programme l'a pris entre-temps, le démon en choisit un autre et met en veille les serveurs qui utilisent encore l'ancien (ils ne joignent plus le fournisseur) ; les sessions qui travaillaient affichent une erreur dans « À traiter ». Reprenez-les (`oh session resume <id>` ou attachement) : leur serveur redémarre avec le nouveau port.
- **Sécurité en local** : l'agent tourne sous votre utilisateur. Il peut atteindre le socket du démon et `oh.db` (empreintes de jetons seulement), mais jamais la clé LLM ; l'émission de nouveaux jetons est réservée à la CLI oh (capacité dans le trousseau, voir `oh doctor`). Voir [SECURITY.fr.md](../../SECURITY.fr.md).
- **opencode 2.0.20** : le shell d'une session ne reçoit pas l'environnement du serveur (oh y repose `PATH`, `HOME`… en local) ; les sous-agents ne reçoivent pas l'environnement de session (le démon le leur réapplique) ; quand une permission est refusée, l'agent ne voit pas le message du refus (oh lui envoie la consigne à part).
- **Restrictions souples** : le nombre maximum de sessions actives peut être dépassé par deux lancements simultanés ; un budget est contrôlé à la fin d'une étape (l'étape en cours se termine) ; le plafond mémoire ne mesure pas les conteneurs. Une session créée par `oh session fork` peut compter une seconde fois le coût de l'historique copié.
- **Relever un budget** : la vue Sessions ne permet que de classer une décision `$` (`x`) ; pour relever le budget, utilisez `oh budget raise <session|décision> [montant]`.
- **`attach = "iterm"` sans iTerm2 installé** : `oh run` peut ouvrir le client dans le terminal courant au lieu de passer à Terminal.app ; préférez `attach = "auto"` (ou `oh session attach`, qui bascule correctement).
- **Paquets** : `~/.oh/bundles/` n'est pas purgé automatiquement (les paquets non utilisés restent sur le disque).