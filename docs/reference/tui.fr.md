> [Read in English](tui.en.md)

# Référence TUI — OpenHub

> Documentation complète de l'interface terminal interactive (TUI) d'OpenHub.

## Lancement

```bash
oh          # Lance le TUI (terminal interactif détecté)
oh --no-tui # Force le mode CLI classique
```

Le TUI se lance automatiquement quand `oh` est exécuté sans sous-commande dans un terminal interactif. Variables qui désactivent le TUI : `CI=true`, `TERM=dumb`, `OH_RICH_TUI=0`.

Si aucun provider n'est configuré (premier lancement), un wizard inline de configuration initiale se lance automatiquement dans le shell. Voir [InlineWizardView](tui-inline-wizard.fr.md) pour les détails du composant.

## Principes de design

Le TUI suit un design **omnibar-first** inspiré des lanceurs fuzzy (fzf, Telescope) et de l'approche minimaliste d'opencode :

- **Point d'interaction unique** : l'omnibar gère toutes les commandes
- **Contenu d'abord** : espace maximum dédié au contenu
- **Contextuel** : l'interface s'adapte à la vue active
- **Raccourcis minimaux** : quelques raccourcis globaux, le reste dans l'omnibar ou la ligne d'aide de la vue

## Layout

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                   │
│                    ZONE DE CONTENU                                │
│              (s'adapte à la vue active)                           │
│                                                                   │
├───────────────────────────────────────────────────────────────────┤
│  > _  Ctrl+P commande · Esc retour · Ctrl+Q quitter             │ Omnibar (1 row)
└───────────────────────────────────────────────────────────────────┘
```

## Raccourcis globaux

| Touche | Action |
|--------|--------|
| `Ctrl+P` | Activer l'omnibar |
| `Esc` | Retour (vue précédente) ; à la racine d'un mode Projet ou Équipe : retour au mode Hub |
| `Ctrl+T` | Mode Équipe (sélecteur si plusieurs équipes) ; depuis le mode Équipe : retour au Hub |
| `Ctrl+Q` / `Ctrl+C` | Quitter la TUI (si des sessions travaillent : finir l'étape puis veille, arrière-plan, ou arrêter maintenant) |
| `?` / `F1` | Aide et raccourcis |
| `j` / `k`, `g` / `G` | Bas / haut, premier / dernier élément (dans toute liste) |
| `d` | Fermer le plus ancien toast affiché |
| `Enter` | Active l'omnibar si la vue ne l'utilise pas |
| Toute autre lettre | Active l'omnibar avec ce caractère (si la vue ne l'utilise pas) |

Le reste passe par l'omnibar ou par les touches de la vue (affichées dans la ligne d'aide).

La barre de mode affiche le badge des sessions **`● N ⏸ M`** (N sessions qui travaillent, M décisions en attente) ; il passe en alerte quand une décision attend.

## L'Omnibar

L'omnibar est toujours visible en bas de l'écran. Il a deux modes :

### Mode passif (par défaut)

Affiche les hints contextuels de la vue active :

```
│  Ctrl+P commande · j/k nav · Enter ouvrir · h/l colonnes        │
```

### Mode actif (saisie)

Accepte la saisie avec suggestions fuzzy au-dessus :

```
│  ● run audit           Audit du projet (sécurité, performance…)   │
│  ○ run review          Review de code d'une branche…               │
│  ○ run debug           Diagnostiquer un bug ou un problème isolé   │
├───────────────────────────────────────────────────────────────────┤
│  > audit_                                                         │
```

### Contrôles de l'omnibar

| Touche | Action |
|--------|--------|
| Saisie | Filtre les commandes (fuzzy, sur le libellé et les alias) |
| `↓` / `Tab` | Descendre dans les suggestions |
| `↑` / `Shift+Tab` | Monter dans les suggestions |
| `Enter` | Exécuter la commande sélectionnée |
| `Esc` | Fermer et retourner au contenu |

## Commandes disponibles

La colonne Commande donne le libellé affiché ; on peut aussi taper un alias. « Projet » : seulement en mode Projet ; « Équipe » : seulement en mode Équipe ; « Projet/Équipe » : dans les deux ; sinon partout.

### Workflows et sessions

| Commande | Alias | Mode | Description |
|----------|-------|------|-------------|
| `run <workflow>` | l'id du workflow ; anciens noms : `dev`, `start.dev` → `ticket` · `start`, `orchestrator` → `feature` · `q`, `fast` → `quick` · `secu`, `security`, `perf`, `archi`, `a11y` → `audit` · `rev`, `cr` → `review` · `dbg`, `debugger`, `diag` → `debug` · `onboard`, `start.onboard` → `onboarding` · `feedback`, `rf`, `retours` → `review-feedback` | — | Fiche de lancement du workflow (une commande par workflow du catalogue) |
| Session libre | `coder`, session, code, free, libre | Projet/Équipe | Fiche de lancement du workflow `libre` (agent d'entrée au choix, par défaut `orchestrator`) |
| Sessions | `sessions`, parallel, par, multi, inbox, à traiter, decisions | — | Vue Sessions |
| Workflows | `workflows`, catalogue, catalog, wf, workflow | — | Catalogue des workflows (brouillons, publication, historique) |
| Briques | `bricks`, briques, agents, skills, catalogue des briques | — | Catalogue des briques (agents et skills : origine, coût, workflows) |
| Review Publish | `review.publish`, publish, mr | Projet/Équipe | Créer la MR de la branche courante (`oh review --publish`, terminal suspendu) ; présent seulement si l'écriture GitLab est activée |

### Fiche de lancement

Générée depuis le YAML du workflow (titre « Lancer · <workflow> »), en trois étapes :

1. **Entrées** : une ligne par entrée (ticket Beads avec sélecteur `Choisir…`, case pour `bool`, liste pour `enum`, zone de texte pour `text`) ; « Ce workflow n'a pas d'entrée » sinon.
2. **Options** : **Mode** ; **Exécution** (`⌂ local`, `▣ conteneur`, `☁ distant` ; un environnement indisponible affiche la raison ; en conteneur : moteur, image en cache ou à construire avec la durée estimée) ; **Emplacement** (`base`, worktrees existants, `+ nouveau worktree`) ; **Ouverture** (auto, iTerm2, Terminal.app, tmux, navigateur, ce terminal, ne pas ouvrir).
3. **Récap** : agents, budget du 1er tour, isolation, sessions et emplacements (« N sessions · 1 serveur · un worktree par session qui écrit »), avertissements.

`Tab` champ suivant, `Ctrl+S` lance depuis n'importe quelle étape, `Ctrl+B` revient, `Esc` ferme. Un second lancement pendant la préparation est ignoré. Avec plusieurs tickets, une case « Une seule session pour tous les tickets » est proposée. Quand une précondition suggère un autre workflow, le récap propose « Lancer <wf> d'abord (puis revenir) ».

### Démarrer, board, catalogue

- **Démarrer** (accueil, projet, équipe) : ★ épinglés (5 max par portée), **Récents** (3), suggestions si rien n'est épinglé ni récent (`ticket`, `feature`, `review`) ; le workflow par défaut du projet est en tête (badge « par défaut ») ; en mode Projet/Équipe, catégories repliées (Développer, Cadrer, Qualité, Connaissance, Autres ; Entrée = choix du workflow). `*` épingle ou désépingle (portée : hub, projet ou équipe selon l'accueil). « Tous les workflows (N) » ouvre le catalogue ; « Session libre » ouvre la fiche de `libre`.
- **Board** : `a` sur un ticket ouvre « Lancer sur <ticket> » avec les workflows qui prennent un ticket Beads ; la fiche s'ouvre à l'étape Options, ticket prérempli.
- **Catalogue** (`workflows`) : workflows par couche (Hub · intégrés, Équipe, Projet ; version, risque, ⌂ ▣ ☁, validité), détail à droite (entrée, chaîne, entrées, exécution). Entrée lance, `*` épingle. Avec un team-state (équipe ou espace solo), le catalogue est **modifiable** :
  - sections Hub (lecture seule : `e` étendre, `n` dupliquer), Équipe, Projet, **Mes brouillons** (`✎`, nombre d'erreurs, badge « nouvelle brique » pour une brique d'équipe utilisée pour la première fois) et **⚠ Intégrité** (fichiers publiés ignorés, briques refusées) ; `✎` sur un workflow publié = vous en avez un brouillon, `⏳` = publication en attente du réseau ;
  - `n` nouveau (Vide, Étendre un workflow (patch), Dupliquer un workflow (copie) ; couche équipe ou projet ; identifiant ; sans team-state, propose de créer un espace solo), `e` éditer (sur un workflow du hub : l'étendre), `v` valider, `t` tester le brouillon (fiche « ✎ brouillon », en local), `p` publier, `D` diff du brouillon avec l'impact, `h` historique, `x` archiver (raison facultative ; sur un brouillon : l'abandonner), `r` recharger ; Entrée sur un brouillon = le tester ;
  - **Publier** : version actuelle → suivante, validation, impact (les élargissements sont marqués ⚠), nouvelles briques, diff du document et du gabarit, gouvernance (« Publication : tout membre ») ; message obligatoire, `Ctrl+S` publie une seule fois ; hors ligne, la publication est mise en attente et rejouée à la synchronisation suivante du team-state (toast « N publication(s) en attente rejouée(s) ») ;
  - **Historique** : versions (auteur, date, message, actuelle) ; Entrée = diff avec la version actuelle, `r` = restaurer (republiée comme nouvelle version).
  - **Éditeur** (`e`, `n`) : cinq sections (`Tab` / `Shift+Tab`) — **Général** (identité, sécurité, exécution : chaque champ montre la valeur résolue, `✎` s'il est écrit dans le brouillon, `← hub:ticket` son origine, 🔒 s'il est verrouillé par le workflow parent ; Entrée modifie, `x` revient à la valeur héritée), **Graphe** (colonne de démarrage, checkpoints dans l'ordre avec leur comportement dans le mode affiché — `m` pour en changer —, agents sous le checkpoint qu'ils attendent, agents indépendants à part ; Entrée modifie, `a` ajoute un agent du catalogue, `c` un checkpoint, `x` retire — un élément hérité est désactivé), **Entrées & prompt** (entrées, `a` ajouter, gabarit, `P` pour l'écrire dans `$EDITOR`, aperçu du prompt), **Ressources** (skills ajoutées/refusées, MCP, Beads, plugins, sorties), **Aperçu paquet** (agents, skills, budget du 1er tour, profondeur, isolation, MCP ; diagnostics : Entrée mène au champ, ou au YAML à la bonne ligne) ;
  - `u` / `U` annuler / rétablir, `y` YAML brut dans `$EDITOR`, `w` (ou `Ctrl+S`) enregistrer le brouillon (refusé tant qu'il reste des erreurs), `Esc` : s'il reste des modifications, « Enregistrer et quitter », « Abandonner les modifications » ou « Continuer l'édition ». Les commentaires et la mise en forme du YAML sont conservés ; seuls les champs modifiés sont écrits.
- **Catalogue des briques** (`bricks`) : sections Agents et Skills (origine hub ou équipe, ~tokens, nombre de workflows qui les utilisent), détail à droite (type, famille, mode, skills, dépendances, chargé par, coût estimé, workflows) ; `/` chercher (identifiant, nom, description), `f` filtrer (tout, agents, skills).
- **Détail d'équipe** : section « Workflows » avec la gouvernance de publication en lecture seule ; pour un espace solo, ligne « Espace » (« solo (local) ») et action « Passer en équipe » (URL d'un dépôt distant vide).

### Vue Sessions

Sections : **À traiter** (décisions en attente), **En cours**, **En veille**, **Terminées, 7 j**, **À récupérer** (sessions distantes terminées). Le détail à droite montre la session (workflow, runtime ⌂ ▣ ☁, emplacement, coût, décisions, sorties, suite proposée « ↪ Enchaîner avec … (e) »).

| Touche | Action |
|--------|--------|
| `Enter` | Ouvrir la décision sélectionnée (fiche checkpoint, question, permission) ; sur une session sans décision : afficher / masquer le flux |
| `y` / `n` | Permission : autoriser une fois / refuser ; checkpoint : valider / corriger d'abord |
| `x` | Classer une erreur, un dépassement de budget ou un coupe-circuit |
| `a` | Attacher (ouvrir opencode sur la session) |
| `A` | Choisir la méthode d'ouverture (automatique, iTerm2, Terminal.app, tmux, navigateur, ici) |
| `w` | Ouvrir dans le navigateur |
| `t` | Afficher / masquer le flux de la session |
| `m` | Envoyer une consigne (prise en compte à la prochaine étape) |
| `i` | Interrompre l'étape en cours |
| `M` | Changer le modèle des prochaines étapes (`fournisseur/modèle`) |
| `s` | Arrêter la session (confirmation) |
| `c` | Reprendre une session en veille |
| `o` | Résultats (description de MR) |
| `e` | Enchaîner avec… (workflows qui prennent une sortie de la session) |
| `g` | Récupérer une session distante (artefacts, rejeu du journal Beads, conflits) |
| `f` | Filtre : projet actif / tous les projets |
| `r` | Rafraîchir |

**Fiche checkpoint** (`⏸ <checkpoint>`) : changements de la session (`+N −M · K fichier(s)`, « Diff complet »), derniers messages, frise ; puis **Décider** : Valider, Corriger d'abord ou Autre consigne, avec un message à l'agent (obligatoire pour les deux derniers choix). Un coupe-circuit s'affiche « Coupe-circuit : N délégations d'affilée sans intervention ».

**Récupération distante** (`g`) : import de la session, puis « Rejouer le journal (N op.) » ; un conflit Beads propose Garder local, Appliquer distant, Fusionner les notes ou Plus tard.

### Projets

| Commande | Alias | Mode | Description |
|----------|-------|------|-------------|
| Board Projet | `board`, kanban, tasks, project board | Projet | Kanban du projet actif |
| Init Board | `board.init`, beads init, init board, init tickets | — | Initialiser Beads dans le projet |
| Projets | `projects`, proj, list | — | Liste des projets |
| Ajouter un projet | `project.add`, project add, add project, nouveau projet | — | Assistant d'ajout de projet |

`deploy` et `sync` n'existent plus (v5 : plus de déploiement dans les projets) ; les restes des anciens déploiements se retirent avec `cleanup`.

### Configuration

| Commande | Alias | Mode | Description |
|----------|-------|------|-------------|
| Settings | `settings`, config, cfg, hub, hub config | — | Réglages du hub |
| Config Projet | `project-config`, config projet, project config | Projet | Configuration du projet (dont Exécution) |
| Models | `models`, mod, model, llm | — | Modèles |
| Provider | `provider`, prov, api | — | Fournisseur LLM |
| MCP | `mcp`, servers | — | Serveurs MCP |
| Secrets & Tokens | `secrets`, tokens, credentials, keychain | — | Secrets du trousseau |
| Équipes | `teams`, team, equipe, equipes | — | Liste des équipes |
| Configuration équipe | `team-detail`, tracker, sync, team config, team detail | Équipe | Détail et configuration de l'équipe |
| Discover Tracker | `team.discover`, tracker discovery, discover, configurer tracker | Équipe | Configurer les colonnes du board depuis le tracker |
| Configurer le hub | `init`, setup, reconfigure, configurer | — | Assistant de configuration (premier lancement) |

**Réglages** (`settings`), dans l'ordre : Général, CLI, Opencode, Sessions (ouverture, style iTerm2, veille), **Restrictions des sessions** (désactivées si vides : sessions actives max, budget par session, budget journalier, plafond mémoire, modèles autorisés), **Exécution** (runtime par défaut, moteur de conteneurs, cache des images, version d'opencode figée, isolation stricte), Workflows (lien vers le catalogue), MCP GitLab / Jira / Figma / Google Slides, Worktree, Tracker (surcharges locales), **Distant (GitLab CI)** (par cible : instance · groupe, projet oh-runner, étiquette des runners, construction de l'image Kaniko ou Docker-in-Docker, architecture, durée maximale du job, jeton GitLab ; « Vérifier ou compléter » lance `oh remote setup`). Touches : `Enter` / `e` modifier, `Espace` basculer, `u` annuler, `r` recharger.

**Config Projet › Exécution** : Dockerfile de dev (vide = détection ; aucun = image oh par défaut Debian + git), build args (`CLÉ=valeur`, séparés par des virgules), volumes de cache, workflow par défaut, runtime par défaut (vide = réglages / workflow).

### Système

| Commande | Alias | Description |
|----------|-------|-------------|
| Statut | `status`, stat, info | État du système |
| Doctor | `doctor`, doc, health, check | Diagnostic de santé |
| Métriques | `metrics`, met, stats, tokens | Statistiques d'usage |
| Notifications | `notifications`, notif, logs, messages, toasts, erreurs | Historique des notifications |
| Nettoyer les anciens déploiements | `cleanup`, deploy-cleanup, nettoyage, migrate | Écran de nettoyage (`oh migrate deploy-cleanup`) |
| Export historique / Import historique | `history.export`, `history.import` (export history, import history…) | Export / import de l'historique des sessions |
| Aide | `help`, ?, aide, shortcuts | Aide et raccourcis |

**Écran de nettoyage** (« Nettoyage des anciens déploiements ») : liste des projets avec des restes de `oh deploy` ; « Voir le diff » (diff de `opencode.json`), « Nettoyer », « Plus tard ». Il est aussi proposé une fois au démarrage quand des restes sont trouvés.

`plugins` et `upgrade` ont été retirés en v5 (plugins déclarés par workflow ; opencode V2 s'installe avec son propre outil).

### Navigation

| Commande | Alias | Description | Disponibilité |
|----------|-------|-------------|---------------|
| Accueil | `home`, accueil, welcome | Retour à l'accueil | Partout |
| Mode Projet | `project.mode`, projet, project, focus | Passer en mode Projet | Modes Hub et Équipe |
| Mode Hub | `hub.mode`, hub, complet, retour | Revenir au mode Hub | Modes Équipe et Projet |
| Quitter | `quit`, exit, q | Quitter la TUI | Partout |

Le mode Équipe s'active avec `Ctrl+T` (ou en choisissant une équipe à l'accueil).

### Équipe

| Commande | Alias | Mode | Description |
|----------|-------|------|-------------|
| Board Équipe | `team.board`, team board, team kanban, equipe board | Équipe | Kanban d'équipe |
| Statut | `team.status`, team stat, status team | Équipe | Statut d'équipe |
| Activité | `team.activity`, activite, feed, activity | Équipe | Activité récente |
| Historique Équipe | `history.team`, team history, team sessions | Équipe | Historique des sessions de l'équipe (vue Activité) |
| Reprises | `team.briefs`, takeover, briefs, reprises | Équipe | Briefs de reprise de contexte |
| Patterns | `team.patterns`, pat, patterns | Équipe | Patterns d'équipe |
| Policies | `team.policies`, pol, rules, policies | Équipe | Politiques d'équipe |
| Wiki | `team.wiki`, wiki, proposals, pending | Équipe | Propositions de wiki |
| Sync Tracker | `team.sync`, sync tracker, synchroniser tracker | Équipe | Synchroniser le tracker |
| Équipe du projet | `team.configure`, team projet, configurer equipe | Équipe | Équipe rattachée au projet |
| Initialiser l'équipe | `team.init`, team init, initialiser | — | Assistant `team init` |
| Rejoindre une équipe | `team.rejoin`, team rejoin, rejoindre, rejoin | — | Rejoindre une équipe existante |
| Worktrees | `worktrees`, wt, git worktree | — | Gestion des git worktrees |

## Raccourcis contextuels par vue

Quand une vue est active, ces touches fonctionnent sans activer l'omnibar. Elles sont rappelées dans la ligne d'aide. `j` / `k` (bas / haut) et `g` / `G` (premier / dernier) marchent partout.

### Vue Board (projet)

| Touche | Action |
|--------|--------|
| `h` / `←` | Colonne précédente |
| `l` / `→` | Colonne suivante |
| `Enter` | Détail du ticket |
| `a` | Actions : lancer un workflow sur le ticket |
| `L` | Lier le ticket au tracker |
| `r` | Rafraîchir |
| `i` | Initialiser Beads (projet sans Beads) |

### Vue Team Board

Colonnes du `[board]` du team-state ; par défaut **6 colonnes** : TODO, IN PROGRESS, REVIEW, VALIDATION, DONE, BLOCKED. Les tickets affichent des étiquettes compactes : `[AI]` (vert) pour `agent-reviewed`, `[!]` (jaune) pour `needs-human-review`.

| Touche | Action |
|--------|--------|
| `h` / `←`, `l` / `→` | Colonne précédente / suivante |
| `[` / `]` | Onglet de projet précédent / suivant |
| `c` | Prendre le ticket (s'assigner) |
| `x` | Libérer le ticket |
| `t` | Transférer à un autre membre |
| `s` | Changer le statut |
| `a` | Actions rapides (dont lancer un workflow) |
| `/` | Rechercher |
| `f` | Filtrer |
| `r` | Rafraîchir (sync tracker si configuré, puis pull git) |

### Vue Projets

| Touche | Action |
|--------|--------|
| `Enter` | Configurer le projet |
| `a` | Ajouter un projet |
| `d` | Supprimer |
| `n` | Renommer |
| `m` | Déplacer (nouveau chemin) |
| `p` | Passer en mode Projet |
| `b` | Initialiser Beads |
| `r` | Rafraîchir |

### Vue Config (Réglages)

| Touche | Action |
|--------|--------|
| `Enter` / `e` | Modifier la valeur |
| `Espace` | Basculer un booléen |
| `u` | Annuler la dernière modification |
| `r` | Recharger |

### Vue Équipes

| Touche | Action |
|--------|--------|
| `a` | Ajouter une équipe |
| `d` | Supprimer |
| `s` | Synchroniser |
| `u` | Annuler |
| `r` | Rafraîchir |

### Vues Statut d'équipe, Activité, Reprises, Worktrees

| Vue | Touches |
|-----|---------|
| Statut d'équipe | `r` rafraîchir |
| Activité | `t` aujourd'hui, `w` semaine, `0` tout, `r` rafraîchir |
| Reprises | `Enter` voir le brief, `e` enrichir (IA, workflow `brief-enrich`), `r` rafraîchir |
| Worktrees | `a` ajouter, `d` supprimer, `o` ouvrir dans un terminal, `p` nettoyer (prune), `x` supprimer les worktrees mergés, `r` rafraîchir |

## Wizards inline

Les flows de configuration multi-step (`team init`, `init`, `project add`) utilisent un composant **InlineWizardView** qui tourne dans le shell TUI sans switch alt-screen. Le wizard est pushé sur le router stack via `shell.PushView(v)` et pop automatiquement à la fin.

Raccourcis : `Ctrl+S` valider, `Ctrl+B` retour, double `Esc` passer.

Voir [Référence InlineWizardView](tui-inline-wizard.fr.md) pour l'API complète et les patterns de création de nouveaux wizards.

### Architecture — PushView

La méthode `PushView(v View)` sur `ShellAccess` permet de pusher une vue éphémère sur le router stack sans pré-enregistrement. Le wizard reçoit automatiquement `ShellAccess` via l'interface `shellAware`. Cette méthode est utilisée par les actions omnibar `team init`, `init`, et `project add`.

## Sessions opencode

Quand une session est lancée (fiche de lancement, `coder`, Démarrer) :
1. oh construit le paquet de session et démarre la session sur le serveur `opencode serve` de son groupe
2. Le client opencode s'ouvre à côté (onglet ou fenêtre iTerm2/Terminal, tmux, navigateur) ; le TUI reste utilisable
3. Fermer opencode n'arrête pas la session : elle se suit et se rouvre depuis la vue **Sessions**
4. La suspension du TUI n'est qu'un dernier recours (`attach = "suspend"`)

Voir [Sessions v5](../guides/sessions-v5.fr.md).

## Synchronisation des vues d'équipe

Toutes les vues d'équipe (board, status, activity, takeover, patterns, policies) tirent automatiquement les dernières données git d'équipe à l'ouverture. Appuyer sur `r` déclenche également un pull avant le rafraîchissement.

- Un toast "Synchronisation..." s'affiche uniquement si le pull dépasse 1 seconde
- En cas d'erreur réseau : un toast d'avertissement est affiché et les données locales sont utilisées (aucun blocage)
