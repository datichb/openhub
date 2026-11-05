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

**Accueils.** Les trois accueils dessinent leur nom dans la bannière tel qu'il a été saisi (une équipe « Core » n'est pas « CORE » ; le Hub dessine le logo « OPENHUB »).

- **Hub** : sous la bannière, la ligne de l'outil `opencode v2.2.5 · adaptateur opencode-v2 ✔` (l'outil et l'adaptateur qui lancent les sessions). La section Système liste Configuration, Métriques, Doctor, Secrets, **Notifications** et le **Catalogue des briques**.
  Les sections se suivent dans cet ordre, en une colonne comme en deux (la colonne de droite porte Projets et Équipes) : **Démarrer**, **Sessions**, **Projets**, **Équipes**, **Système**. La section **Équipes** est toujours là, comme Projets : une ligne `👥 <équipe>  ★ par défaut · 3 membres · 2 en cours` par équipe activée (`★` = équipe par défaut ; « pas encore synchronisée » tant que son team-state n'est pas cloné), puis **➕ Nouvelle équipe**. Sans équipe, cette ligne explique à quoi sert une équipe. **Nouvelle équipe** ouvre une fenêtre : *Créer une équipe* (un nouveau dépôt team-state), *Rejoindre une équipe* (dépôt existant : nouveau membre ou réinstallation), *Espace solo* (vos workflows en local, sans équipe). Quand vous avez des espaces solo, une dernière ligne discrète, **🧑 2 espaces solo ›** (« 1 espace solo › » au singulier), ferme la section après **Nouvelle équipe** : `Entrée` ouvre la vue Équipes, qui les liste avec les équipes. Les espaces solo archivés ne sont pas comptés ; sans espace solo, la ligne n'apparaît pas.
- **Projet** : l'en-tête donne le chemin et la branche, la ligne de l'outil et les environnements d'exécution avec leur état, `exécution : ⌂ local ✔  ▣ conteneur · colima ✔  ☁ distant ✗` (lus à l'ouverture de l'accueil, hors de la boucle d'événements).
  La section **Équipe** est toujours présente : la ligne `👥 Équipe : Core › changer…` (ou `Équipe : aucune`, `Équipe : espace solo mine`) ouvre le choix de l'équipe du projet ; elle est suivie du statut, du board et de l'activité de l'équipe quand le projet est dans une équipe.
- **Équipe** : en plus des *Sessions de l'équipe*, la section **Sessions des autres membres** liste, d'après les claims du team-state, qui travaille sur quel ticket (membre, ticket, session, depuis quand ; `●` le claim est vivant, `○` son bail est dépassé). Elle est en **lecture seule** : `Entrée` ouvre une fenêtre de détail avec un seul bouton *Fermer* ; une session tourne sur la machine de son membre, oh n'agit pas dessus.
  La colonne de droite commence par la section **Équipe** : *Membres et ma fiche*, *Config Équipe*, *MCP de l'équipe*, *Modèles de l'équipe*, *Politiques*, *Patterns*, *Wiki de l'équipe*. *Membres et ma fiche* ouvre le détail de l'équipe sur la section **Ma fiche de membre**, *Config Équipe* sur la première section de la configuration partagée (le tracker). La section **Navigation** propose *Changer d'équipe…*, *Nouvelle équipe*, *Projets*, *Métriques* et *Vue Hub*. Sans équipe active, la vue liste ce qu'on peut faire (nouvelle équipe, choisir une équipe, vue Hub) au lieu d'un simple texte.

## Touches globales

| Touche | Action |
|--------|--------|
| `Ctrl+P` | Palette de commandes |
| `/` | Filtrer la liste de la vue ; si la vue ne filtre pas : palette |
| `?` / `F1` | Aide : touches globales, convention, touches de la vue active, légende, commandes |
| `Échap` | Ferme d'abord les toasts ; sinon revient (vue précédente) ; à la racine d'une vue Projet ou Équipe : retour à la vue Hub |
| `Ctrl+T` | Vue suivante du cycle **Hub → Projet → Équipe → Hub**, en sautant ce qui n'est pas configuré : depuis le Hub, le projet quitté (le seul, ou un choix parmi plusieurs), sinon l'équipe ; depuis le Projet, l'équipe (choix si plusieurs, l'équipe par défaut en premier ; ni espace solo ni équipe archivée), sinon le Hub ; depuis l'Équipe, le Hub. Sans aucune équipe configurée, `Ctrl+T` ouvre la fenêtre « Créer ou rejoindre une équipe ? » au lieu d'un simple message |
| `Ctrl+Q` / `Ctrl+C` | Quitter (si des sessions travaillent : finir l'étape puis veille, arrière-plan ou arrêter) |
| `Entrée` | Action de la liste, du bouton ou du champ sélectionné ; palette seulement si rien ne l'utilise |

**Ordre des touches** : les fenêtres (formulaires, confirmations) et la palette reçoivent toutes les touches ; puis viennent les touches globales ; puis les touches de la vue ; puis la liste ou le champ sélectionné. Taper une lettre « n'importe où » n'ouvre pas la palette. Si une vue qui finit de se charger en arrière-plan prend le focus alors que la palette ou une fenêtre est ouverte, celle-ci le reprend à la touche suivante : `Échap` la ferme toujours. Une vue qui n'est pas la racine affiche `Échap revenir` dans sa ligne d'aide (ajouté par la TUI quand la vue ne le dit pas déjà).

**Fenêtres** : une fenêtre ouverte alors qu'une autre est affichée s'empile dessus, jusqu'à trois niveaux (le fond est de plus en plus sombre). `Échap` ferme celle du dessus, avec ou sans bouton, et rend la main à celle du dessous ; fermer une fenêtre ferme aussi celles qui sont au-dessus. Le focus revient à l'endroit d'où la fenêtre a été ouverte : le bouton de la fenêtre du dessous, ou le composant de la vue qui l'avait. Une question posée depuis une carte (« Décider », « Refuser », « Passer outre »…) s'ouvre par-dessus la carte : « Annuler » ou `Échap` redonne la carte telle qu'elle était, répondre ferme la question et la carte avec elle. Un champ obligatoire laissé vide (consigne, motif) est refusé sans fermer la fenêtre : la saisie reste, avec un avertissement. `Tab` et `Maj+Tab` ne ferment pas une saisie. **Largeur** : une liste de choix (menu `a` d'une carte, statut, workflow…) est aussi large que sa ligne la plus longue, de 40 colonnes à l'écran moins 4 ; son titre compte aussi. Rien n'est coupé, sauf sur un écran trop étroit, où la ligne ou le titre finit par « … ». Les saisies et les formulaires prennent 70 % de l'écran, entre 72 et 120 colonnes (jamais plus que l'écran moins 4), et leurs champs s'étirent avec eux ; les aides sous les champs passent à la ligne selon la largeur.

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
| Partout | `Ctrl+S` | Enregistrer : les pages d'équipe et l'éditeur de workflow, qui n'enregistrent pas seuls (dans une fiche de lancement ou un assistant : étape suivante ou lancer) |
| Liste | `n` | Créer |
| Liste | `e` | Éditer |
| Liste | `d` | Supprimer, **toujours confirmé** |
| Liste | `x` | Archiver ou classer, **jamais destructif** |
| Liste | `R` | Reprendre ou restaurer (session en veille, session distante, version d'un workflow) |
| Liste | `c` | Comparer (diff) |
| Liste | `Espace` | Cocher / décocher, basculer un réglage, choisir l'option sous le curseur |
| Décision | `y` | Valider ou accepter (checkpoint, proposition wiki, élément d'un lot) |
| Décision | `n` | Refuser, avec une consigne ; la consigne validée vaut confirmation |
| Décision | `e` | Modifier avant de valider |
| Décision | `s` | Laisser en attente |
| Décision | `A` | Tout accepter |
| Lecteur | `/` | Rechercher dans le texte (trace, rapports) |
| Lecteur | `n` / `N` | Résultat suivant / précédent |

`e`, `s`, `A` et `c` portent une action propre à la vue quand elle n'a pas l'action de la convention (`e` enchaîner dans Sessions, `s` arrêter une session, `A` ouvrir avec…).

`w` n'a qu'un sens : ouvrir dans le navigateur (Sessions). Enregistrer, c'est `Ctrl+S`.

**Enregistrer** : les pages Réglages et Projet (config, modèles, MCP) enregistrent seules après **un seul délai** (500 ms, le même partout) et le disent dans le titre de la page : `✎ enregistrement…` puis `✔ enregistré` (`✗ non enregistré` si l'enregistrement a échoué). Les pages d'équipe (détail d'équipe, modèles d'équipe, MCP d'équipe) publient dans le team-state, partagé avec l'équipe : le partagé n'est pas enregistré seul, le titre dit `✎ non publié — Ctrl+S` tant qu'un changement attend, `✔ enregistré` une fois que `Ctrl+S` l'a publié (le personnel de `hub.toml` s'enregistre seul : voir le détail d'équipe).

### Touches changées

| Vue | Avant → maintenant |
|-----|--------------------|
| Sessions | `D` → `d` supprimer · `c` → `R` reprendre · `g` → `R` récupérer une session distante · nouveau : `B` valider en lot |
| Récap d'une session existante (fiche de lancement) | `r` → `R` reprendre · `t` → `T` transférer · `k` → `K` remplacer en gardant le travail |
| Propositions wiki | `a` → `y` accepter · `x` → `n` refuser |
| Workflows | `D` → `c` comparer · `x` → `d` supprimer un brouillon (`x` n'archive que les workflows publiés) |
| Historique d'un workflow | `r` → `R` restaurer (`r` rafraîchit) |
| Éditeur de workflow | `x` → `d` retirer ou revenir à la valeur héritée · `y` → `Y` YAML brut |
| Patterns | `a` → `n` créer · `v` → `y` valider |
| Projets | `a` → `n` créer · `n` → `e` renommer |
| Board d'équipe | `f` → `/` filtrer · `x` → `L` libérer |
| Worktrees | `a` → `n` créer · `x` → `M` nettoyer les fusionnés |
| Détail d'équipe, modèles d'équipe, MCP d'équipe | `y` → `T` découvrir le tracker (détail) · `w` → `Ctrl+S` enregistrer |
| Éditeur de workflow | aussi `w` → `Ctrl+S` enregistrer |
| Activité d'équipe | `w` → `s` semaine (`t` aujourd'hui, `0` tout) |
| MCP | `w` → `m` mode écriture d'un service |
| Fiche de lancement (dossier modifié) | `w` → `t` worktree (`s` stash, `b` autoriser) |
| Changer de vue | `Ctrl+T` : Hub → Projet → Équipe → Hub (il n'allait qu'entre le Hub et l'Équipe) |
| Autres listes (modèles, policies, secrets, équipes…) | `a` → `n` créer |
| Formulaires (choix multiple) | `x` → `Espace` cocher (`x` ne coche plus : c'est l'archivage des listes d'objets) |
| Éditeur de colonnes du board | `a` → `n` ajouter · `r` → `o` changer le rôle · nouveau : `e` renommer, `d` demande confirmation (voir [l'éditeur](#éditeur-de-colonnes-du-board)) |

## Légende des symboles

Un symbole veut dire **une seule chose**, sur tous les écrans de la TUI et dans les commandes. L'aide (`?`) en montre la légende, sous la convention des touches ; la source est la table des sens du thème (`theme.Meanings`, voir [Aurum](../design/aurum.md#table-des-sens-themeiconsgo-glyphsymbolsgo)).

| Symbole | Sens |
|---------|------|
| `✔` | réussi, valide (aussi une étape terminée) |
| `✗` | échoué, invalide : une erreur ponctuelle, une saisie refusée |
| `○` | désactivé, au repos (aussi une étape à venir, un choix non retenu) |
| `–` | absent, non renseigné (aussi une étape sautée) |
| `⚠` | marche, mais demande de l'attention |
| `●` | en cours d'exécution |
| `◆` | élément courant : vue, étape, équipe |
| `✎` | modifié, ni enregistré ni publié |
| `★` | choix par défaut, élément épinglé |
| `▸` | ligne sous le curseur, début d'une action |
| `◉` | choisi dans un choix unique |
| `☑` / `☐` | coché / non coché dans un choix multiple |

Les états d'une session ont leurs propres symboles (`⊗` une session arrêtée sur un échec, `◌` une session en veille, voir la [référence de la CLI](cli.fr.md)) : `✗` est une erreur ponctuelle, `⊗` la façon dont une exécution a fini.

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

Les commandes d'un projet (`board`, `board.init`, `project-wiki`, `project-config`) sont toujours enregistrées et ne s'affichent que dans la vue Projet : un premier projet ajouté dans la TUI les apporte tout de suite, sans redémarrer.

| Commande | Alias | Vue | Description |
|----------|-------|------|-------------|
| `board` | kanban, tasks, project board | Projet | Board Beads du projet |
| `project-wiki` | wiki, wiki projet, project wiki, doc metrics | Projet | Wiki du projet : file des propositions et métriques |
| `board.init` | beads init, init board, init tickets | — | Initialiser Beads |
| `projects`, `project.add` | proj, list · project add, nouveau projet | — | Projets, assistant d'ajout |
| `settings`, `project-config` | config, cfg · config projet | — / Projet | Réglages du hub, configuration du projet (dont Exécution) |
| `models`, `provider`, `mcp`, `secrets` | mod, llm · prov, api · servers · tokens, credentials | — | Modèles, fournisseur, serveurs MCP, secrets |
| `teams` | team, equipe, equipes | — | Équipes (catégorie Équipe) |
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

Toutes les commandes d'équipe sont dans la catégorie **Équipe**. Elles sont toujours enregistrées : une équipe créée ou rejointe dans la TUI apporte ses commandes tout de suite, sans redémarrer.

| Commande | Alias | Vue | Description |
|----------|-------|-----|-------------|
| `team.new` | new team, nouvelle équipe, add team | — | **Nouvelle équipe…** : fenêtre *Créer* / *Rejoindre* / *Espace solo* |
| `team.init` | team init, create team, créer, initialiser | — | Créer une équipe (un nouveau dépôt team-state, même si on en a déjà une) |
| `team.rejoin` | team rejoin, rejoindre, join team | — | Rejoindre une équipe : dépôt existant, nouveau membre ou réinstallation |
| `team.configure` | attach project, rattacher, team projet | Projet | **Rattacher le projet à une équipe** : toutes les équipes, un espace solo ou aucune |
| `team.board`, `team.status`, `team.activity` | team board, team stat, activity, team history | Équipe | Board, statut de l'équipe, activité de l'équipe (l'ancienne commande « Historique Équipe » était l'activité sous un autre nom) |
| `team.briefs`, `team.patterns`, `team.policies` | takeover, reprises · pat · pol, rules | Équipe | Reprises, patterns, policies |
| `team.wiki` | wiki, proposals, pending, team-state | Équipe | Propositions d'équipe (team-state) |
| `team.sync`, `team.reconfigure`, `team.switch` | sync tracker · reconfigure team · changer equipe | Équipe | Synchroniser le tracker, reconfigurer l'équipe ouverte, changer d'équipe |
| `team-detail`, `team.discover` | tracker, profile · discover | Équipe | Configuration de l'équipe, découvrir le tracker |
| `worktrees` | wt, git worktree | — | Worktrees git |

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

**Fiche de checkpoint** (`Entrée` sur `⏸`) : valideur attendu (`approver:`, « ⚠ hors valideur » si votre identité git n'en est pas), **vérification lancée par oh** (commandes et état), preuves de l'agent, changements (`+N −M · K fichier(s)`), derniers messages, frise, rapports. Boutons : **Décider**, **Diff complet**, **Lire rp-N** (un par rapport), **Propositions du wiki (N) : décider…** (s'il y en a), **Attacher**. « Diff complet » et « Lire rp-N » ouvrent le lecteur par-dessus la carte : `Échap` ou un clic dehors le ferme et redonne la carte, sur le bouton d'où l'on venait ; si la décision est tranchée ailleurs pendant la lecture, la carte et le lecteur se ferment ensemble.

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

Commande `team.wiki` (vue Équipe) : propositions de wiki du team-state, puis pages. `{` / `}` sections, `Entrée` voir (boutons Accepter, Refuser, Fermer), `y` accepter, `n` refuser, `r` rafraîchir. **Refuser demande un motif** (obligatoire : un motif vide ne refuse rien) ; il est gardé dans l'historique du team-state et dans l'événement `wiki.rejected`, affiché dans la vue Activité et dans la notification.

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

**Éditeur** : sections Général, Graphe, Entrées & prompt, Ressources, Aperçu paquet (`Tab` / `Shift+Tab`). `Entrée` modifier, `d` (ou `Suppr`) retirer un élément ou revenir à la valeur héritée (confirmé), `a` agent ou entrée, `c` checkpoint, `m` mode affiché (Graphe), `P` gabarit dans `$EDITOR`, `Y` YAML brut dans `$EDITOR`, `u` / `U` annuler / rétablir, `Ctrl+S` enregistrer, `Échap` (enregistrer et quitter, abandonner ou continuer).

### Boards

`oh board`, `oh team board` et `oh dashboard` (déprécié) ouvrent la TUI directement sur les vues **Board**, **Board d'équipe** et **Métriques** ci-dessous, au-dessus de l'accueil du mode (projet, équipe, hub) : `Esc` y revient, `Ctrl+Q` quitte. Sans terminal, elles sortent avec le code 2. Il n'y a plus de board plein écran séparé : ce que la commande ouvre est la vue du shell, avec ses touches.

| Vue | Touches |
|-----|---------|
| Board du projet | `h` / `l` colonnes, `Entrée` détail, `a` actions (lancer un workflow sur le ticket), `L` lier au tracker, `i` initialiser Beads |
| Board d'équipe | `h` / `l` colonnes, `[` / `]` projets (les onglets portent le nom du projet, triés par nom ; un ID qu'aucun projet local ne connaît s'affiche « (projet inconnu) »), `Entrée` détail, `a` actions (le menu suit la colonne de la carte : voir [le menu `a` d'une carte](#le-menu-a-dune-carte)), `/` filtrer (menu : texte, **Mes tickets**, **Non assignés** — la réserve, à prendre avec `c` —, assigné, libellé, effacer), `c` prendre (refusé à `max_in_progress_per_member`), `L` libérer (confirmé), `t` transférer, `s` statut (le menu ne propose que les colonnes que les transitions du board autorisent ; la colonne TODO du board par défaut est le statut `planned`), `r` rafraîchir (synchronise le tracker et dit ce qu'elle a fait dans un toast, puis tire le team-state ; une équipe sans tracker ne fait que tirer), `C` éditer les colonnes du board (l'éditeur ci-dessous). Le board tire le team-state de lui-même chaque minute (sans toast), et synchronise le tracker à son ouverture quand l'équipe a activé `auto_sync`. `c`, `L`, `t` et `s` agissent sur le projet de la carte, pas sur le projet actif, et répondent de nouveau après un `Échap` dans la liste de `t` ou de `s` |

#### Le menu `a` d'une carte

Sur le board d'équipe, `a` propose les actions de la colonne où est la carte. La colonne est reconnue à son **rôle** (`initial`, `active`, `review`…), pas à son nom.

| Colonne (rôle) | Actions de `a` |
|---|---|
| todo (`initial`) | **Cadrer et écrire la spec** (`spec`, avec l'issue `#<iid>` ou l'id Beads comme demande) · **Implémenter le ticket** (`ticket`) |
| première colonne `active` (en cours) | **Implémenter le ticket** · **Donner une consigne à la session en cours**, quand le claim en nomme une qui n'est pas finie |
| `review` | **Review de code** (`review`, sur la branche du claim) · **Traiter les retours de review** (`review-feedback`, avec la MR du claim : seulement si le claim a un `mr_url`) |
| les autres (validation, bloquée, terminée…) | Détail · statut · transfert |

« Implémenter le ticket » dépend de la source de la carte :

- **ticket Beads** : `ticket` sur son id ;
- **issue GitLab** : si Beads est initialisé dans le projet, l'issue est **importée en ticket Beads** (liée par `external_ref = gitlab-<iid>`) puis `ticket` est lancé sur cet id. L'import a lieu **quand le lancement est confirmé** dans le formulaire (le récapitulatif annonce « sera importée en ticket Beads au lancement ») : annuler ne laisse rien, un import qui échoue arrête le lancement, et un second lancement reprend le ticket déjà lié, pas un second. Sans Beads dans le projet, `quick` prend l'issue comme demande. Un workflow à id Beads ne reçoit jamais `#<iid>` ;
- **ticket d'un autre tracker** (clé Jira…) : `quick`, avec la clé comme demande.

Le lancement de « Implémenter le ticket » met le claim de la carte **en cours** une fois la session démarrée (le claim est pris s'il est dans la réserve ou n'existe pas) et respecte `max_in_progress_per_member` : au-delà, le lancement est refusé avant de rien créer. Les autres actions ne touchent pas au claim. À la fin de la session, le claim de la carte est réglé comme ceux des tickets de la session (ticket fermé : terminé ; en revue : gardé ; checkpoint en attente ou reporté avec du travail : gardé et marqué `session-ended` ; sinon : libéré) ; pour une issue importée, « fermé » et « en revue » se lisent sur le ticket Beads lié.

Une action indisponible reste dans la liste, en gris, avec la raison (« `spec` n'est pas proposé dans ce projet », « le claim n'a pas d'URL de merge request », « aucune session en cours sur ce ticket ») ; la choisir affiche la raison. Dans la barre de commandes, `run <workflow>` suit le même menu. Un projet qui ne propose pas le workflow, ou une colonne sans rôle (board construit à la main), garde le menu des autres boards.

Le formulaire de lancement d'un workflow à id Beads sur lequel on a saisi `#<iid>` dit pourquoi il ne le lit pas et quoi lancer à la place, au lieu de « Impossible de charger ».

#### Réservations sur le board d'équipe

- **Prendre (`c`)** : sur un ticket que quelqu'un détient, le board dit « déjà pris par X » au lieu d'annoncer une prise qui n'a pas eu lieu. Sur un ticket à soi : « déjà à vous ».
- **Badges sur la carte** : `bail 05/11 14:30` (fin du bail du claim), `⚠ expiré` une fois le bail dépassé, `session terminée` quand la session a pris fin en laissant le claim (label `session-ended`). Le détail (`Entrée`) donne la date.
- **Reprise d'un claim expiré** : `c` sur une carte expirée demande « Reprendre <ticket> ? » (la question nomme le propriétaire) ; si oui, le claim passe à vous avec un brief de reprise et un événement `claim.transferred` signé de vous. Un claim dont le bail a été renouvelé entre-temps n'est pas repris.
- **Libérer (`x`) et transférer (`t`)** le claim d'un autre membre : la confirmation le nomme (« Libérer la réservation de Alice ? »). Un claim qui a changé de main entre l'affichage et l'action n'est pas libéré : le board dit qui le détient.
- **Réassigner les réservations d'un membre** : commande de palette « Réassigner les réservations d'un membre… » (membres qui détiennent des réservations ouvertes avec leur nombre, puis un membre qui reçoit ou « Les libérer », puis confirmation). Tout est un seul commit, un événement par réservation, signé de vous. Même chose en CLI : `oh team claim transfer --from`.

### Éditeur de colonnes du board

Accueil de l'équipe → **Colonnes du board**, ou `C` sur le board d'équipe (il faut une équipe active et son team-state cloné ; au retour de l'éditeur, le board relit ses tickets avec les nouvelles colonnes). L'éditeur règle les colonnes du board d'équipe (section `[board]` de la configuration d'équipe, voir [`config.fr.md`](config.fr.md#section-board)) et les règles qui rangent un label ou un statut du tracker dans une colonne. C'est une étape d'assistant : `Échap` demande « Quitter l'assistant ? », et rien n'est écrit tant que `Ctrl+S` n'a pas été pressé.

Liste des colonnes :

| Touche | Action |
|--------|--------|
| `↑` `↓` / `j` `k` | choisir une colonne |
| `n` | ajouter une colonne après celle du curseur (le nom est demandé ; l'identifiant en est tiré, le rôle est proposé d'après le nom) |
| `e` / `Entrée` | renommer la colonne (son identifiant ne change pas) |
| `d` | supprimer la colonne, **après confirmation** (le focus est sur « Annuler ») ; les labels et statuts qui y mènent partent avec elle, la confirmation en donne le nombre. Deux colonnes au moins restent : en dessous, un message refuse sans rien demander |
| `o` | changer le rôle : `initial` → `active` → `review` → `terminal` → `blocked` |
| `K` / `J` | monter / descendre la colonne |
| `m` | ouvrir les rattachements de la colonne (quand l'équipe a un tracker) |
| `Ctrl+S` | enregistrer : au moins deux colonnes dont une `terminal`, sinon un message dit ce qui manque |

Rattachements d'une colonne (`m`) : la liste des labels, puis des statuts (Jira), qui rangent un ticket dans la colonne.

| Touche | Action |
|--------|--------|
| `↑` `↓` / `j` `k` | choisir un label ou un statut (les titres sont sautés) |
| `n` | ajouter un label (le nom est demandé) |
| `e` / `Entrée` | renommer le label ou le statut |
| `d` | le retirer de la colonne, **après confirmation** |
| `v` | le déplacer vers une autre colonne : la liste des autres colonnes est proposée, le label ou le statut quitte alors cette colonne (il apparaît dans les rattachements de la cible) |
| `q` | revenir à la liste des colonnes |

L'assistant de découverte du tracker (« Découvrir le tracker ») utilise le même éditeur pour ses colonnes, sans la touche `m`. Avant la 5.1 : `a` ajoutait et `r` changeait le rôle ; c'est maintenant `n` et `o` (`r` rafraîchit partout ailleurs), et `d` demande confirmation (voir [Touches changées](#touches-changées)).

### Autres vues

| Vue | Touches |
|-----|---------|
| Projets | `Entrée` configurer, `n` ajouter, `e` renommer, `m` déplacer, `d` supprimer (confirmé), `p` vue Projet, `b` initialiser Beads |
| Worktrees | `Entrée` / `o` ouvrir dans un terminal, `n` créer, `d` supprimer, `p` nettoyer les références (prune), `M` supprimer les worktrees fusionnés (jamais ceux d'une session non arrêtée) ; confirmés |
| Réglages, Config projet | `{` / `}` sections, `Espace` basculer, `Entrée` modifier, `u` annuler, `r` recharger (confirmé s'il reste des modifications) |
| Modèles (hub, projet, équipe) | `Entrée` modifier, `n` créer, `d` supprimer (confirmé), `u` annuler, `Ctrl+S` enregistrer (équipe) |
| Fournisseur | `Espace` par défaut, `Entrée` / `e` modifier, `s` configurer, `d` effacer la configuration et la clé du trousseau (confirmé) |
| MCP (hub, projet, équipe) | `Espace` activer, `Entrée` modifier, `t` jeton ou test, `m` mode écriture (vue du hub), `Ctrl+S` enregistrer (équipe), `u` annuler, `T` approuver l'URL imposée par l'équipe (vue du hub ; confirmé, voir [`oh mcp trust`](cli-mcp.fr.md#oh-mcp-trust)) |
| Secrets | `Entrée` / `e` modifier, `n` créer, `d` supprimer (confirmé) |
| Équipes | `Entrée` détail de l'équipe, `p` vue Équipe (ni espace solo ni équipe archivée), `n` nouvelle équipe (la fenêtre *Créer* / *Rejoindre* / *Espace solo* ; la liste montre l'équipe au retour), `*` définir par défaut (`★`), `e` renommer, `x` archiver, `R` restaurer, `d` retirer : *Quitter l'équipe* (libère vos réservations, supprime le clone et les clés, puis retire l'équipe ; la fenêtre de confirmation liste tout), *Quitter et retirer ma fiche*, ou *Retirer de hub.toml seulement* (détache les projets rattachés ; le clone et les réservations restent), `s` synchroniser, `u` annuler. La liste se termine par la ligne **➕ Nouvelle équipe**, qui est toute la liste sans équipe |
| Détail d'équipe | l'équipe ouverte dans la liste (sinon l'équipe par défaut ; « Aucune équipe à configurer » et `Ctrl+S` sans effet quand il n'y en a pas) : `Espace` basculer, `Entrée` modifier, `Ctrl+S` enregistrer, `s` synchroniser, `t` tester, `u` annuler, `T` découvrir le tracker, `K` clé du fournisseur de l'équipe |
| Patterns | `Entrée` voir, `n` créer, `y` valider, `d` supprimer (confirmé) |
| Policies | `Entrée` détail, `n` créer, `t` modèle, `e` éditer, `d` supprimer (confirmé), `c` vérifier |
| Statut, Activité, Métriques | `c` vérifier les conventions · `t` / `s` / `0` aujourd'hui, semaine, tout (Activité) · `7` / `3` / `0` période, `Tab` usage par agent |
| Reprises | `Entrée` voir le brief, `e` enrichir (workflow `brief-enrich`) |

**Détail d'équipe.** Trois sortes de réglages, dites dans le titre de chaque section :

| Titre de section | Où ça s'écrit | Quand |
|---|---|---|
| « … partagé avec l'équipe (Ctrl+S pour publier) » : Tracker, Réserve et auto-plan, Board, Claims, Notifications, Restrictions, Gouvernance | `config.toml` du team-state | **brouillon** publié d'un bloc par `Ctrl+S` (un commit, un événement `config_changed`) |
| « … enregistré automatiquement (hub.toml) » : Mes surcharges pour cette équipe | la section `[teams.tracker]` de l'équipe | **tout seul** après chaque modification |
| « … enregistré aussitôt » : Cette équipe sur ce poste (nom, identifiant de membre, défaut, GitLab) et Ma fiche (profil, rôle, mode) | `hub.toml` ou `members.toml` | **à la validation** du champ |

Chaque réglage partagé indique s'il vient de l'`équipe` ou du `défaut` ; une réponse **vide** à un champ numérique ou texte remet le défaut. Les réglages `*_enforced`, les plafonds `Restrictions › Imposé`, le rôle et la gouvernance sont **réservés aux leads** (cadenas et message sinon). Le webhook de notification se saisit dans une fenêtre qui masque la saisie et s'affiche masqué. `Échap` avec un brouillon non publié demande : **publier et quitter**, **abandonner**, **rester** ; un lien vers MCP, Modèles ou Workflows garde le brouillon et le remet au retour (abandonné, avec le nom des clés, si un collègue a changé le même réglage entre-temps). « Reconfigurer l'équipe… » rouvre l'assistant ; « Colonnes et mappings… » ouvre l'éditeur du board (après publication du brouillon). Sans équipe, `n` ouvre la création d'une équipe.

## Assistants

`init`, `project add` et `team init` ouvrent des assistants dans la TUI (`shell.PushView`). `Ctrl+S` valider l'étape, `Ctrl+B` / `Alt+←` / `PgUp` retour, `Échap` → « Quitter l'assistant ? » (**Quitter**, **Passer l'étape** si elle est facultative, **Continuer**). Voir [InlineWizardView](tui-inline-wizard.fr.md).

## Sessions opencode

Quand une session est lancée, oh construit son paquet et la démarre sur le serveur `opencode serve` de son groupe ; le client opencode s'ouvre à côté (onglet ou fenêtre, tmux, navigateur) et la TUI reste utilisable. Fermer opencode n'arrête pas la session : elle se suit et se rouvre depuis **Sessions**. Voir [Sessions v5](../guides/sessions-v5.fr.md).

## Synchronisation des vues d'équipe

Les vues d'équipe (board, statut, activité, reprises, patterns, policies) tirent les données git de l'équipe à l'ouverture et sur `r`. Un toast « Synchronisation… » s'affiche si le pull dépasse une seconde ; en cas d'erreur réseau, un avertissement et les données locales.
