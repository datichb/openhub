> [Read in English](tui.en.md)

# Référence TUI — OpenHub

> Référence de l'interface terminal (TUI) d'oh : touches, commandes de la palette, vues et leurs actions. Pour les parcours pas à pas, voir le [guide de la TUI](../guides/tui-usage.fr.md).

## Lancement

```bash
oh          # Lance la TUI (terminal interactif détecté)
oh --no-tui # Force le mode CLI classique
```

La TUI se lance quand `oh` est exécuté sans sous-commande dans un terminal interactif. Variables qui la désactivent : `CI=true`, `TERM=dumb`, `OH_RICH_TUI=0`.

Si aucun fournisseur n'est configuré (premier lancement), l'assistant de configuration initiale s'ouvre dans la TUI. Voir [InlineWizardView](tui-inline-wizard.fr.md).

## Layout

```
┌──────────────────────────────────────────────────────────────────┐
│                       ZONE DE CONTENU                            │
│                 (s'adapte à la vue active)                       │
├──────────────────────────────────────────────────────────────────┤
│  ligne d'aide de la vue (touches) ou palette de commandes        │ Omnibar
│  🏠 Hub · ● 2 ◐ 1                                                │ Barre de mode
└──────────────────────────────────────────────────────────────────┘
```

La barre de mode affiche le badge des sessions **`● N ◐ M`** (N sessions qui travaillent, M décisions en attente) ; il passe en alerte quand une décision attend. La légende des symboles (états, décisions) et `NO_COLOR` (sans couleur) sont dans la [référence CLI](cli.fr.md#symboles-et-couleurs).

**Petits terminaux** : sous 32 lignes, mise en page compacte (bandeau sur une ligne, omnibar et barre de mode sur 3 lignes) ; sous 60×16, message « Terminal trop petit » et seul `Ctrl+Q` fonctionne. Réglage `[cli] compact = auto | always | never`. Le changement de mise en page garde l'élément sélectionné.

## Touches globales

| Touche | Action |
|--------|--------|
| `Ctrl+P` | Palette de commandes |
| `/` | Filtrer la liste de la vue ; si la vue ne filtre pas : palette |
| `?` / `F1` | Aide : touches globales, convention, touches de la vue active, légende, commandes |
| `Échap` | Ferme d'abord les toasts ; sinon revient (vue précédente) ; à la racine d'une vue Projet ou Équipe : retour à la vue Hub |
| `Ctrl+T` | Vue Équipe (choix si plusieurs équipes) ; depuis la vue Équipe : retour à la vue Hub |
| `Ctrl+Q` / `Ctrl+C` | Quitter (si des sessions travaillent : finir l'étape puis veille, arrière-plan ou arrêter) |
| `Entrée` | Action de la liste, du bouton ou du champ sélectionné ; palette seulement si rien ne l'utilise |

**Ordre des touches** : les fenêtres (formulaires, confirmations) et la palette reçoivent toutes les touches ; puis viennent les touches globales ; puis les touches de la vue ; puis la liste ou le champ sélectionné. Taper une lettre « n'importe où » n'ouvre pas la palette.

**Vues qui gardent les touches** : la fiche de lancement, les assistants, l'éditeur de workflow et la publication gardent `Échap` (fermer, « Quitter l'assistant ? ») et ne ferment pas les toasts ; `F1` y ouvre l'aide. Les onglets du détail d'une session et la validation par lot gardent aussi `Échap` (retour au détail, annuler le lot), mais `?` et `Ctrl+P` y restent disponibles, et la ligne d'aide montre les touches de l'onglet ou du lot.

**Boutons** : un seul style dans les fenêtres, formulaires, assistants et le récap. L'action principale est en gras ; le bouton sélectionné est encadré `▸ libellé ◂`. À l'ouverture, l'action principale est sélectionnée (« Annuler » pour une confirmation dangereuse) ; `Tab` / `Shift+Tab` (et `←` / `→`) parcourent les boutons ; `↓` / `↑` font défiler le texte d'une fenêtre.

## Convention des touches

Une touche a le même sens dans toutes les vues. Une vue peut donner une lettre libre à une action à elle (`a` attacher, `p` relancer le pilote, `B` lot…), mais ne donne jamais un autre sens à une touche de la convention, ni une autre touche à une action de la convention. L'aide (`?`) affiche ce tableau et propose de revoir le message « Nouvelles touches », montré une fois au premier lancement après la mise à jour.

| Contexte | Touche | Action |
|----------|--------|--------|
| Partout | `↑` `↓` / `j` `k` | Monter / descendre |
| Partout | `g` / `G` (`Début` / `Fin`) | Premier / dernier élément |
| Partout | `/` | Filtrer la liste ; sans filtre dans la vue : palette |
| Partout | `r` | Rafraîchir |
| Partout | `?` / `F1` | Aide et touches |
| Partout | `Entrée` | Ouvrir l'élément sélectionné |
| Partout | `Échap` | Revenir (ferme d'abord les toasts) |
| Liste | `n` | Créer |
| Liste | `e` | Éditer |
| Liste | `d` | Supprimer, **toujours confirmé** |
| Liste | `x` | Archiver ou classer, **jamais destructif** |
| Liste | `R` | Reprendre ou restaurer (session en veille, session distante, version d'un workflow) |
| Liste | `c` | Comparer (diff) |
| Décision | `y` | Valider ou accepter (checkpoint, proposition wiki, élément d'un lot) |
| Décision | `n` | Refuser, avec une consigne ; la consigne validée vaut confirmation |
| Décision | `e` | Modifier avant de valider |
| Décision | `s` | Laisser en attente |
| Décision | `A` | Tout accepter |
| Lecteur | `/` | Rechercher dans le texte (trace, rapports) |
| Lecteur | `n` / `N` | Résultat suivant / précédent |

`e`, `s`, `A` et `c` portent une action propre à la vue quand elle n'a pas l'action de la convention (`e` enchaîner dans Sessions, `s` arrêter une session, `A` ouvrir avec…).

### Touches changées

| Vue | Avant → maintenant |
|-----|--------------------|
| Sessions | `D` → `d` supprimer · `c` → `R` reprendre · `g` → `R` récupérer une session distante · nouveau : `B` valider en lot |
| Récap d'une session existante (fiche de lancement) | `r` → `R` reprendre · `t` → `T` transférer |
| Propositions wiki | `a` → `y` accepter · `x` → `n` refuser |
| Workflows | `D` → `c` comparer · `x` → `d` supprimer un brouillon (`x` n'archive que les workflows publiés) |
| Historique d'un workflow | `r` → `R` restaurer (`r` rafraîchit) |
| Éditeur de workflow | `x` → `d` retirer ou revenir à la valeur héritée · `y` → `Y` YAML brut |
| Patterns | `a` → `n` créer · `v` → `y` valider |
| Projets | `a` → `n` créer · `n` → `e` renommer |
| Board d'équipe | `f` → `/` filtrer · `x` → `L` libérer |
| Worktrees | `a` → `n` créer · `x` → `M` nettoyer les fusionnés |
| Détail d'équipe | `y` → `T` découvrir le tracker |
| Autres listes (modèles, policies, secrets, équipes…) | `a` → `n` créer |

## L'omnibar

En bas de l'écran. Au repos, elle affiche la ligne d'aide de la vue (ou de l'onglet) ; `Ctrl+P` (ou `/` hors filtre) l'ouvre en saisie, avec les suggestions au-dessus :

```
│  ● run audit           Audit du projet (sécurité, performance…)   │
│  ○ run review          Review de code d'une branche…               │
├───────────────────────────────────────────────────────────────────┤
│  > aud_                                                           │
```

| Touche | Action |
|--------|--------|
| Saisie | Filtre les commandes (fuzzy, sur le libellé et les alias) |
| `↓` / `Tab`, `↑` / `Shift+Tab` | Parcourir les suggestions |
| `Entrée` | Exécuter la commande sélectionnée |
| `Échap` | Fermer ; le focus revient où il était (onglet compris) |

## Commandes de la palette

La colonne Commande donne l'identifiant ; on peut taper le libellé ou un alias. Chaque terme n'a qu'un sens. « Projet » : seulement dans la vue Projet ; « Équipe » : seulement dans la vue Équipe ; sinon partout. Le mot « mode » est réservé au mode de workflow : les trois écrans d'accueil se disent « vue Hub / Projet / Équipe ».

### Workflows et sessions

| Commande | Alias | Vue | Description |
|----------|-------|------|-------------|
| `run.<workflow>` | l'id du workflow et ses `aliases:` : `cadrage`, `feature` (→ `spec`), `secu` (→ `security-fix`), `perf` (→ `benchmark`), `dbg` (→ `debug`), `feedback`, `retours` (→ `review-feedback`) | — | Fiche de lancement du workflow (une commande par workflow du catalogue) |
| `run.feature.implement-spec` | « run feature → implement-spec (avec tickets) » | — | Fiche d'`implement-spec` sous l'alias `feature` avec des tickets |
| `libre` | coder, session, code, free | Projet/Équipe | Fiche de `libre` (agent par défaut `orchestrator`) |
| `libre.agent` | coder.agent, libre agent, free agent, session agent | Projet/Équipe | « Session libre avec un agent… » : choisir l'agent d'entrée, puis la fiche |
| `sessions` | parallel, par, multi, inbox, à traiter, decisions | — | Vue Sessions |
| `workflows` | catalogue, catalog, wf, workflow | — | Catalogue des workflows |
| `bricks` | briques, agents, skills, catalogue des briques | — | Briques (agents et skills) |
| `review.publish` | publish, mr | Projet/Équipe | Créer la MR de la branche courante ; seulement si l'écriture GitLab est activée |

Les alias viennent des YAML des workflows (`aliases:`, voir la [référence des workflows](workflows.fr.md)) : ils valent aussi pour `oh run`. Les anciens termes en dur (`dev`, `start`, `fast`, `rev`…) ne sont plus reconnus.

### Projets et configuration

| Commande | Alias | Vue | Description |
|----------|-------|------|-------------|
| `board` | kanban, tasks, project board | Projet | Board Beads du projet |
| `project-wiki` | wiki, wiki projet, project wiki, doc metrics | Projet | Wiki du projet : file des propositions et métriques |
| `board.init` | beads init, init board, init tickets | — | Initialiser Beads |
| `projects`, `project.add` | proj, list · project add, nouveau projet | — | Projets, assistant d'ajout |
| `settings`, `project-config` | config, cfg · config projet | — / Projet | Réglages du hub, configuration du projet (dont Exécution) |
| `models`, `provider`, `mcp`, `secrets` | mod, llm · prov, api · servers · tokens, credentials | — | Modèles, fournisseur, serveurs MCP, secrets |
| `teams`, `team-detail`, `team.discover` | team, équipes · tracker · discover | — / Équipe | Équipes, détail de l'équipe, découverte du tracker |
| `init` | setup, reconfigure | — | Assistant de configuration |

### Système et navigation

| Commande | Alias | Description |
|----------|-------|-------------|
| `status`, `doctor`, `metrics`, `notifications` | stat · doc, health · stats, usage · notif, logs | Statut, diagnostic, métriques, notifications |
| `cleanup` | nettoyage, gc, oh gc, deploy-cleanup, migrate, garbage, espace disque, disk | Vue **Nettoyage** (anciens déploiements et données d'oh) |
| `history.export`, `history.import` | export history, import history | Export / import de l'historique des sessions |
| `help` | ?, aide, shortcuts | Aide |
| `home`, `project.mode`, `hub.mode`, `quit` | accueil · projet, focus · hub, retour · exit, q | Accueil, vue Projet, vue Hub, quitter |

### Équipe

| Commande | Alias | Description |
|----------|-------|-------------|
| `team.board`, `team.status`, `team.activity`, `history.team` | team board, team stat, activity, team history | Board, statut, activité, historique de l'équipe |
| `team.briefs`, `team.patterns`, `team.policies` | takeover, reprises · pat · pol, rules | Reprises, patterns, policies |
| `team.wiki` | wiki, proposals, pending, team-state | Propositions d'équipe (team-state) |
| `team.sync`, `team.configure`, `team.init`, `team.rejoin` | sync tracker · team projet · team init · rejoindre | Synchroniser le tracker, équipe du projet, créer ou rejoindre une équipe |
| `worktrees` | wt, git worktree | Worktrees git |

## Vues et touches

Chaque vue affiche ses touches dans la ligne d'aide ; `?` les reprend. Les touches de la convention (`j`/`k`, `g`/`G`, `r`, `Échap`…) ne sont pas répétées quand elles ont leur sens habituel.

### Sessions

Sections : **À traiter** (sessions avec des décisions en attente, chacune suivie de ses décisions : `⏸` checkpoint, `?` question, `!` permission, `$` budget, `✗` erreur, `⊘` coupe-circuit), **En cours**, **À récupérer** (sessions distantes), **En veille**, **Terminées, 7 j**. Les enfants d'un fan-out (`↳`) sont rangés en arbre sous leur parente (`⑂`) ; le groupe passe dans « À traiter » dès qu'un enfant attend.

```
┌ Sessions ─────────────────────────────┐┌ implement-spec · ses_8f2a ────────────┐
│ ● ⑂ implement-spec  spec auth   $1.84 ││ [Détail]  Trace  Rapports  Fan-out    │
│   ├ ✔ ↳ ticket  oh-12  merged   $0.41 ││ ⏸ cp-merge : 2/3 enfants mergés       │
│   └ ◐ ↳ ticket  oh-14  cp-2     $0.22 ││ Sorties relevées par oh               │
│ ✔   spec        auth OAuth      $0.93 ││ ↪ Suites proposées (next:)            │
└───────────────────────────────────────┘└───────────────────────────────────────┘
```

| Touche | Action |
|--------|--------|
| `Entrée` | Sur une décision : ouvrir sa fiche ; sur une session : afficher / masquer le flux |
| `y` / `n` | Permission : autoriser une fois / refuser ; checkpoint : valider (fiche si la vérification échoue) / formulaire « Corriger d'abord » (valideur attendu affiché) |
| `x` | Classer une erreur, un dépassement de budget ou un coupe-circuit |
| `$` | Relever le budget (décision `$`) |
| `a` / `A` | Attacher (ouvrir opencode) / ouvrir avec… (automatique, iTerm2, Terminal.app, tmux, navigateur, ici) |
| `t` | Afficher / masquer le flux en direct |
| `m` / `i` / `M` | Envoyer une consigne / interrompre l'étape / changer le modèle des prochaines étapes |
| `s` | Arrêter la session (confirmation) |
| `e` | Enchaîner avec… (suites `next:` en tête) |
| `B` | Valider en lot (voir plus bas) |
| `d` | Supprimer la session (confirmation : ce qui est supprimé et perdu) |
| `R` | Reprendre une session en veille ; récupérer une session distante prête |
| `o` / `w` | Résultats (description de MR) / ouvrir dans le navigateur |
| `f` | Projet actif / tous les projets |
| `Tab` / `Shift+Tab` | Onglet suivant / précédent du détail |

**Onglets du détail** (`Tab`). Hors Détail, les touches vont à l'onglet ; `Échap` revient au Détail et à la liste ; `?` et `Ctrl+P` restent disponibles.

| Onglet | Contenu | Touches |
|--------|---------|---------|
| Détail | Session (workflow, runtime ⌂ ▣ ☁, emplacement, coût, décisions), **sorties relevées par oh**, **suites proposées (`next:`)** | celles de la liste |
| Trace | Texte de `oh session trace` | `/` rechercher, `n` / `N`, `g` / `G`, `r` |
| Rapports | Rapports de la session, lecteur à droite ; sur la session parente d'un fan-out, ceux des enfants s'y ajoutent, triés par date, avec l'enfant qui les a déposés (colonne après `rp-N`) | `Entrée` lire (puis `Échap` revient à la liste), `/`, `n` / `N`, `r` |
| Fan-out | État du fan-out : parallèle, pilote, budget, enfants (ticket, session, état, branche, coût, fin), tours de correction, dernier refus de `cp-merge`, branche d'intégration | `Entrée` ouvrir l'enfant, `p` relancer le pilote, `d` nettoyer, `B` valider en lot, `r` |

`p` refuse de relancer un pilote qui tourne (« tourne déjà ») ; une relance acceptée écrit l'événement « started again by the user » dans les événements du fan-out (le démon écrit « by the daemon ») ; le journal du pilote est `~/.oh/sessions/<id>/fanout.log`. `d` (après `cp-merge` ou une fois la session arrêtée) montre ce qui sera supprimé et gardé, propose `--discard` (branches non mergées) à cocher, puis confirme ; c'est `oh fanout clean`.

**Fiche de checkpoint** (`Entrée` sur `⏸`) : valideur attendu (`approver:`, « ⚠ hors valideur » si votre identité git n'en est pas), **vérification lancée par oh** (commandes et état), preuves de l'agent, changements (`+N −M · K fichier(s)`), derniers messages, frise, rapports. Boutons : **Décider**, **Diff complet**, **Lire rp-N** (un par rapport), **Propositions du wiki (N) : décider…** (s'il y en a), **Attacher**.

| Formulaire « Décider » | Effet | Message |
|------------------------|-------|---------|
| Valider | Le checkpoint passe (hors valideur : confirmation, tracé) | facultatif |
| Corriger d'abord | L'agent corrige puis redemande | obligatoire |
| Autre consigne | L'agent suit la consigne puis redemande | obligatoire |
| Passer outre la vérification | Seulement si la vérification a échoué et que le checkpoint le permet | motif obligatoire, tracé |

Vérification échouée : la fiche s'ouvre sur **Corriger (consigne)** et **Passer outre…** (motif, confirmation, tracé dans `oh session trace`) ; jamais de « Passer outre » sur `cp-merge` du fan-out.

**Propositions du wiki d'un checkpoint** (bouton de la fiche) : liste sans limite (avant / après, source, auteur, tag), `y` accepter, `n` refuser (motif facultatif, confirmé ; `Entrée` sans rien saisir refuse sans motif), `e` modifier (une ligne, ou `$EDITOR`), `s` laisser, `A` tout accepter, `Entrée` décider le checkpoint, `Échap` revenir à la fiche. Le motif saisi est conservé avec la proposition et s'affiche sous elle une fois refusée (« Motif du refus : … »). Les acceptées sont appliquées au passage du checkpoint, avant le commit.

**Validation par lot** (`B`) : remplace la liste ; portée = le fan-out de la session sélectionnée (sinon toutes les sessions), filtre = le checkpoint de la décision sélectionnée.

| Touche | Action |
|--------|--------|
| `y` / `n` / `s` | Valider / refuser (consigne) / laisser l'élément |
| `A` | Tout valider |
| `/` | Filtrer par checkpoint (comme `--checkpoint`) |
| `Entrée` | Ouvrir la fiche (seul moyen pour une vérification échouée : elle ne se valide pas en lot) |
| `C` | Confirmer : récapitulatif, puis réponse de chaque élément du filtre |
| `Échap` | Annuler le lot |

Seuls les éléments du filtre sont répondus.

**Récupération distante** (`R` sur une ligne « À récupérer ») : import de la session, puis « Rejouer le journal (N op.) » ; un conflit Beads propose Garder local, Appliquer distant, Fusionner les notes ou Plus tard.

### Fiche de lancement

Générée depuis le YAML du workflow (« Lancer · <workflow> », ou « Lancer · feature → implement-spec (avec tickets) » sous un alias), en trois étapes :

1. **Entrées** : une ligne par entrée (ticket avec « Choisir… », case pour `bool`, liste pour `enum`, texte) ; l'aide de l'entrée sous son champ ; « Entier attendu » bloque, « Chemin introuvable » et « Branche introuvable : … » avertissent. Une entrée `from:` affiche « ⚙ calculé par oh : … → ses_… » (elle n'est pas préremplie par son défaut). Pour `libre` : **Agent d'entrée** (agents « selectable »).
2. **Options** : mode, exécution (`⌂ local`, `▣ conteneur`, `☁ distant`, avec la raison si indisponible), emplacement (base, worktrees, « + nouveau worktree »), ouverture.
3. **Récap** : agents, budget du 1er tour, isolation, sessions et emplacements, avertissements ; « Lancer ».

| Touche | Action |
|--------|--------|
| `Tab` | Champ suivant |
| `Ctrl+S` | Aller au récap ; sur le récap chargé : lancer |
| `Alt+←` / `PgUp` / `Ctrl+B`, `Alt+→` / `PgDn` | Étape précédente, suivante |
| `Échap` | Fermer |

**Session existante** (une session du même workflow travaille déjà sur le ticket) : le récap compare les deux ; un choix lance aussitôt : `R` reprendre, `T` transférer (local ↔ conteneur, modèle), `K` remplacer en gardant le travail, `d` supprimer et relancer (confirmé), `p` en parallèle.

**Sélecteur de tickets** (« Choisir… ») : `/` rechercher, `f` filtre de libellé, `e` épopée, `Espace` cocher (plusieurs tickets), `Entrée` choisir, `Échap` annuler.

### Wiki du projet

Commande `project-wiki` (vue Projet). En tête, les métriques résumées (pages, couverture des domaines, part de CONFIRMÉ / INCERTAIN, dernière synchronisation) ; dessous, la file des propositions hors checkpoint : **En attente**, **Acceptées, à appliquer au prochain commit**, **Périmées** (ligne remplacée disparue : rien n'est appliqué).

| Touche | Action |
|--------|--------|
| `y` / `n` / `e` / `s` / `A` | Accepter / refuser (motif facultatif, confirmé) / modifier / laisser / tout accepter, comme `oh wiki decide` |
| `/` | Filtrer (page, contenu, agent, workflow) |
| `m` | Toutes les métriques |
| `r` | Rafraîchir |

### Propositions d'équipe (team-state)

Commande `team.wiki` (vue Équipe) : propositions de wiki du team-state, puis pages. `{` / `}` sections, `Entrée` voir (boutons Accepter, Refuser confirmé, Fermer), `y` accepter, `n` refuser (confirmé), `r` rafraîchir.

### Briques

Commande `bricks`, lecture seule. Colonnes : id, origine (`hub`, `équipe`, `équipe ⟳ hub` quand une brique d'équipe remplace celle du hub, `(non scellée)` sans `catalog.lock`), version, état (`✓`, `!` problème, `✗`). Le détail montre le remplacement, la publication (auteur, date), les agents et workflows qui l'utilisent, le coût, les problèmes et les commandes d'écriture (`oh brick new`, `edit`, `publish`).

`/` chercher, `f` agents / skills / tout, `Entrée` ouvrir un workflow qui l'utilise, `r` rafraîchir.

### Nettoyage

Commande `cleanup` (alias `nettoyage`, `gc`), et Doctor › `f` sur les contrôles de nettoyage. Une vue, deux sections (`Tab`) :

```
 [Anciens déploiements]  Données d'oh
╔ Nettoyage des anciens déploiements ═══════════════════════╗
║ demo  /work/demo                                          ║
║   − .opencode/agents/ (12 fichiers)                       ║
╚═══════════════════════════════════════════════════════════╝
```

| Section | Contenu | Touches |
|---------|---------|---------|
| Anciens déploiements | Ce que les anciens déploiements ont laissé dans les projets et ce qui est gardé (vos clés personnelles), comme `oh migrate deploy-cleanup` | `c` voir le diff de la configuration, `d` nettoyer (confirmé), `r` chercher à nouveau |
| Données d'oh | Ce que `oh gc` peut supprimer, par catégorie et par taille (paquets, données de serveurs, sessions arrêtées, worktrees, fan-outs, dossiers temporaires, serveurs orphelins, historique) ; l'aperçu ne supprime rien | `d` supprimer l'élément, `A` tout nettoyer (confirmés), `r` actualiser l'aperçu ; le résultat s'affiche au-dessus |

La palette ouvre « Données d'oh » ; Doctor ouvre la section de son contrôle. L'écran « Nettoyage des anciens déploiements » est aussi proposé une fois au démarrage quand des restes sont trouvés. Avec un nettoyage du démon en cours, « Données d'oh » répond « occupé ».

### Doctor

Commande `doctor`. `f` corriger le contrôle sélectionné (comme `oh doctor --fix`, avec un aperçu et une confirmation : configuration restaurée depuis une sauvegarde, Beads, réservations rattachées au projet recréé, serveurs orphelins ; ouvre la section du Nettoyage sur ses contrôles), `o` hors ligne (aucun appel réseau, comme `--offline`), `r` revérifier.

### Workflows, éditeur, historique

**Catalogue** (`workflows`) : sections Hub (lecture seule), Équipe, Projet, **Mes brouillons**, **⚠ Intégrité**.

| Touche | Action |
|--------|--------|
| `Entrée` | Lancer ; sur un brouillon : le tester |
| `n` / `e` | Nouveau (vide, extension, copie) / éditer (sur le hub : étendre) |
| `v` / `t` / `p` | Vérifier / tester le brouillon / publier |
| `c` | Comparer : diff du brouillon avec l'impact |
| `h` | Historique |
| `x` | Archiver un workflow publié (raison) |
| `d` | Supprimer un brouillon (confirmé) |
| `*` | Épingler |

**Publication** : version, validation, impact (élargissements ⚠), diff, message obligatoire ; `Ctrl+S` publie ; hors ligne, mise en attente et rejouée à la synchronisation suivante.

**Historique** : `Entrée` ou `c` diff avec la version actuelle, `R` restaurer (confirmé, republiée comme nouvelle version), `r` rafraîchir.

**Éditeur** : sections Général, Graphe, Entrées & prompt, Ressources, Aperçu paquet (`Tab` / `Shift+Tab`). `Entrée` modifier, `d` (ou `Suppr`) retirer un élément ou revenir à la valeur héritée (confirmé), `a` agent ou entrée, `c` checkpoint, `m` mode affiché (Graphe), `P` gabarit dans `$EDITOR`, `Y` YAML brut dans `$EDITOR`, `u` / `U` annuler / rétablir, `w` ou `Ctrl+S` enregistrer, `Échap` (enregistrer et quitter, abandonner ou continuer).

### Boards

| Vue | Touches |
|-----|---------|
| Board du projet | `h` / `l` colonnes, `Entrée` détail, `a` actions (lancer un workflow sur le ticket), `L` lier au tracker, `i` initialiser Beads |
| Board d'équipe | `h` / `l` colonnes, `[` / `]` projets, `Entrée` détail, `a` actions (workflows à ticket ; sur une issue GitLab, aussi les workflows à demande libre avec `#<iid>`), `/` filtrer (menu : texte, assigné, libellé), `c` prendre, `L` libérer (confirmé), `t` transférer, `s` statut |

### Autres vues

| Vue | Touches |
|-----|---------|
| Projets | `Entrée` configurer, `n` ajouter, `e` renommer, `m` déplacer, `d` supprimer (confirmé), `p` vue Projet, `b` initialiser Beads |
| Worktrees | `Entrée` / `o` ouvrir dans un terminal, `n` créer, `d` supprimer, `p` nettoyer les références (prune), `M` supprimer les worktrees fusionnés (jamais ceux d'une session non arrêtée) ; confirmés |
| Réglages, Config projet | `{` / `}` sections, `Espace` basculer, `Entrée` modifier, `u` annuler, `r` recharger (confirmé s'il reste des modifications) |
| Modèles (hub, projet, équipe) | `Entrée` modifier, `n` créer, `d` supprimer (confirmé), `u` annuler, `w` enregistrer (équipe) |
| Fournisseur | `Espace` par défaut, `Entrée` / `e` modifier, `s` configurer, `d` effacer la configuration et la clé du trousseau (confirmé) |
| MCP (hub, projet, équipe) | `Espace` activer, `Entrée` modifier, `t` jeton ou test, `w` enregistrer, `u` annuler, `T` approuver l'URL imposée par l'équipe (vue du hub ; confirmé, voir [`oh mcp trust`](cli-mcp.fr.md#oh-mcp-trust)) |
| Secrets | `Entrée` / `e` modifier, `n` créer, `d` supprimer (confirmé) |
| Équipes | `Entrée` ouvrir, `n` créer, `d` supprimer (confirmé), `s` synchroniser, `u` annuler |
| Détail d'équipe | `Espace` basculer, `Entrée` modifier, `w` enregistrer, `s` synchroniser, `t` tester, `u` annuler, `T` découvrir le tracker, `K` clé du fournisseur de l'équipe |
| Patterns | `Entrée` voir, `n` créer, `y` valider, `d` supprimer (confirmé) |
| Policies | `Entrée` détail, `n` créer, `t` modèle, `e` éditer, `d` supprimer (confirmé), `c` vérifier |
| Statut, Activité, Métriques | `c` vérifier les conventions · `t` / `w` / `0` aujourd'hui, semaine, tout · `7` / `3` / `0` période, `Tab` usage par agent |
| Reprises | `Entrée` voir le brief, `e` enrichir (workflow `brief-enrich`) |

## Assistants

`init`, `project add` et `team init` ouvrent des assistants dans la TUI (`shell.PushView`). `Ctrl+S` valider l'étape, `Ctrl+B` / `Alt+←` / `PgUp` retour, `Échap` → « Quitter l'assistant ? » (**Quitter**, **Passer l'étape** si elle est facultative, **Continuer**). Voir [InlineWizardView](tui-inline-wizard.fr.md).

## Sessions opencode

Quand une session est lancée, oh construit son paquet et la démarre sur le serveur `opencode serve` de son groupe ; le client opencode s'ouvre à côté (onglet ou fenêtre, tmux, navigateur) et la TUI reste utilisable. Fermer opencode n'arrête pas la session : elle se suit et se rouvre depuis **Sessions**. Voir [Sessions v5](../guides/sessions-v5.fr.md).

## Synchronisation des vues d'équipe

Les vues d'équipe (board, statut, activité, reprises, patterns, policies) tirent les données git de l'équipe à l'ouverture et sur `r`. Un toast « Synchronisation… » s'affiche si le pull dépasse une seconde ; en cas d'erreur réseau, un avertissement et les données locales.
