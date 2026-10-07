> [Read in English](047-session-interaction-daemon.en.md)

# ADR-047 — Interaction avec les sessions, multi-session et démon `ohd`

## Statut

Accepté — **Évolué par [ADR-050](050-session-context-capability.fr.md)**

## Date

2026-10-05

## Contexte

Jusqu'à la v4, `oh` lançait opencode dans le terminal : la session était un processus. Fermer la fenêtre la terminait, et oh ne savait plus rien d'elle.

Le mode parallèle reposait sur un coordinateur à part, qui interrogeait l'API toutes les 5 secondes ([ADR-036](./036-platform-abstraction-layer.fr.md)), avec un moniteur dédié. Les worktrees ([ADR-012](./012-git-worktree.fr.md)) étaient créés par l'orchestrateur ou par `--parallel`. Rien ne permettait de suivre plusieurs sessions, d'y répondre sans les ouvrir, ni de libérer la mémoire d'un serveur oublié.

opencode V2 change la donne :

- **serveur durable** auquel on s'attache ;
- **plusieurs clients** sur une session, la première réponse gagnant (F20) ;
- **plusieurs dossiers par serveur** (F19) ;
- **flux d'événements** sans rejeu (F18).

Un serveur occupe environ 340 à 440 Mo et démarre en 1,4 s (F22).

Décisions I1 à I7 (interaction, multi-session, cycle de vie) et S2 (suivi temps réel). Le démon a été décidé le 05/10/2026 pendant la phase 0.

## Décision

### 1. oh est la tour de contrôle, opencode la cabine (I1)

Fermer la fenêtre opencode **ne coupe pas** la session : seul « Arrêter » l'interrompt. Les décisions structurées (checkpoints, questions, permissions) se prennent depuis n'importe quel client ([ADR-042](./042-checkpoints-headless-decisions.fr.md)) ; la conversation libre se fait dans opencode.

### 2. Groupes de serveur (I5)

- Un `opencode serve` par **groupe**. La clé du groupe a la forme `<projet>-<paquet12>-<config10>-<runtime>`, avec un suffixe `-sN` pour un groupe voisin en conteneur.
- Elle contient :
  - le hash du paquet, donc la version du workflow ;
  - une empreinte de la configuration : projet, fournisseur, région, source et empreinte du secret, liste de modèles si elle est active, isolation stricte si elle est active ;
  - le runtime.
- Les sessions d'un groupe ont chacune leur dossier (base ou worktree), leur environnement de session et leurs règles.
- Registre des serveurs : table `servers` (v28) et `~/.oh/servers/<groupe>/`. À la reprise, une session reste dans son groupe d'origine, là où sont ses données.

### 3. Démon `ohd`

**Fonctionnement.**

- Un démon par utilisateur, sur le socket Unix `~/.oh/run/ohd.sock` (0600, UID du pair vérifié).
- Il est démarré à la demande, avec un verrou de lancement. Il s'arrête après 10 minutes sans serveur vivant, sauf pendant une construction d'image ou un suivi distant.
- Il n'est remplacé par une nouvelle version que si aucune session n'est active.

**Ce qu'il héberge.**

- le proxy d'identifiants ([ADR-044](./044-credential-proxy-session-limits.fr.md)) ;
- les passerelles ([ADR-046](./046-beads-gateways.fr.md)) ;
- les crochets du plugin et le CheckpointService ;
- les notifications système ;
- la tâche périodique (suivi des pipelines distants, bail Beads).

**Suivi des sessions.**

- Un abonnement au flux d'événements par serveur prêt. À chaque (re)connexion, une resynchronisation lit les sessions actives, les permissions et les formulaires en attente.
- États dérivés : `waiting` (décision en attente) > `active` (boucle en cours) > `idle`. Coût et tokens sont lus en fin de tour.
- Les sous-sessions sont rattachées à leur session racine : leur activité et leurs décisions remontent à elle.
- Flux en direct `GET /v1/stream` (NDJSON, précédé des 100 dernières entrées) pour la TUI et `oh session follow`.

**Sécurité.** Un PID n'est jamais signalé sans qu'un appel authentifié au serveur ait réussi : un PID réutilisé par une autre application n'est pas touché.

### 4. Ouverture d'une session (I2)

- En mode `auto`, oh essaie dans l'ordre :
  1. le terminal courant (iTerm2 : onglet, scindé ou fenêtre ; Terminal.app : nouvelle fenêtre) ;
  2. l'autre terminal ;
  3. tmux, si `$TMUX` ;
  4. le navigateur (`pair`) ;
  5. en dernier recours, la suspension de la TUI.
- Réglages `[session] attach` et `iterm_style`.
- L'onglet lance `oh session attach <id>`, sans aucun secret en ligne de commande. Le client attaché envoie des battements au démon.

### 5. Cycle de vie et veille (I7)

- États : `preparing`, `queued`, `active`, `waiting`, `idle`, `sleeping`, `completed`, `failed`, `stopped`.
- **Veille**, décidée par le démon pour chaque groupe :
  - jamais si un client est attaché ou si une boucle tourne ;
  - une décision en attente garde le serveur éveillé tant qu'une TUI est ouverte ;
  - sinon, au bout de `[session] idle_sleep_minutes` (5 par défaut) : serveur arrêté, jetons révoqués, conteneur supprimé, sessions `sleeping`.
- Une session en veille se reprend (même dossier de données, paquet relu par son hash) ; `oh session attach` la reprend automatiquement.
- **En quittant oh** :
  - les sessions inactives ou en attente sont mises en veille ;
  - pour chaque session qui travaille, l'utilisateur choisit : « finir l'étape puis mettre en veille » (par défaut), « continuer en arrière-plan » ou « arrêter maintenant ».
- Au démarrage suivant, un récapitulatif « pendant votre absence » liste les sessions, le coût et les décisions restées en attente.
- Les résultats (diff, coût) sont enregistrés dans `~/.oh/sessions/<id>/results/` avant la veille ou l'arrêt. Un événement d'équipe `session.complete` est émis à la fin d'une session.

### 6. Multi-session

- `oh run <wf> --tickets a,b` (ou la multi-sélection de la fiche) ouvre N sessions dans un seul serveur.
- **Worktree automatique (O10)** : une session qui écrit reçoit un worktree si une autre session qui écrit occupe déjà le dossier (une session en veille ne le retient pas). Plusieurs tickets qui écrivent reçoivent un worktree chacun.
- Un verrou par projet et par dossier empêche un double lancement.
- Cela remplace le coordinateur et le moniteur de `--parallel`.

### 7. Surfaces

- **TUI** :
  - la vue **Sessions** remplace la vue parallèle, avec les sections À traiter, En cours, En veille, Terminées et À récupérer ;
  - détail de session, flux en direct, consigne, interruption, changement de modèle, arrêt, reprise, navigateur, « Enchaîner avec… » ;
  - badge `● N ⏸ M` et sections Sessions sur les accueils du hub, du projet et de l'équipe.
- **CLI** : `oh session list|inbox|attach|follow|approve|answer|dismiss|send|interrupt|compact|model|fork|results|stop|resume|open|fetch|resolve`. Un préfixe d'identifiant unique suffit.

### 8. Windows

Le démon tourne dans le processus d'oh. Les sessions sont mises en veille quand oh quitte : « continuer en arrière-plan » n'est pas proposé.

## Conséquences

### Positives

- Une session survit à la fermeture de sa fenêtre et au départ d'oh, et elle se reprend même après un redémarrage de la machine.
- Plusieurs sessions se suivent et se pilotent d'un seul endroit, avec notifications, sans sondage.
- La mémoire et le coût sont contenus : un serveur inutilisé s'arrête tout seul.
- Le mode parallèle n'est plus un chemin à part : c'est N sessions dans un groupe.

### Négatives / Compromis

- Chaque groupe coûte environ 340 à 440 Mo, et un workflow différent implique un autre serveur.
- Le démon est un point central : s'il est arrêté, rien n'est suivi. Les commandes `oh session` le relancent quand un serveur vit encore.
- Pas de rejeu des événements : ce qui est manqué pendant une coupure n'est récupéré que par la resynchronisation.
- Une session de workflow ne passe « terminée » que lorsqu'on l'arrête : une boucle finie reste `idle`, puis `sleeping`.
- Ouverture :
  - l'autorisation macOS « Automation » ne se détecte pas à l'avance ;
  - les onglets de Terminal.app exigeraient l'autorisation Accessibilité, d'où une fenêtre par session ;
  - iTerm2 et tmux n'ont pas été essayés en réel ;
  - avec `attach = "iterm"` sans iTerm2, le client est lancé dans le processus de `oh run` (anomalie Q3-3).
- Les sessions des autres membres de l'équipe (claims) ne sont pas encore affichées.

## Alternatives considérées

| Alternative | Rejetée car |
|---|---|
| Lancer opencode dans le terminal courant (comme en v4) | Une fenêtre = une session ; rien à suivre ni à reprendre, aucune décision depuis oh. |
| Un serveur global pour tous les workflows | Les permissions de session ne masquent pas les sous-agents (F21) : le monde fermé serait perdu. |
| Un serveur par session | Mémoire et démarrage multipliés, sans profiter des dossiers multiples d'un serveur (F19). |
| Sonder l'API à intervalle régulier | 180 requêtes par minute pour 5 sessions dans l'ancien coordinateur ; le flux d'événements avec resynchronisation suffit. |
| Superviser depuis la TUI ou la CLI | Plus rien n'est suivi, ni mis en veille ni notifié quand oh est fermé. |
| Tout arrêter, ou tout laisser tourner, en quittant oh | Perte du travail en cours d'un côté, coût et mémoire sans fin de l'autre ; d'où « finir l'étape puis mettre en veille » par défaut. |
