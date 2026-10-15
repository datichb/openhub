> [Read in English](tui-usage.en.md)

# Guide -- Utiliser le TUI OpenHub

> Guide pratique de la TUI d'oh v5 : lancer des workflows, piloter les sessions, prendre les décisions, gérer les workflows et les réglages.

## Démarrage rapide

```bash
oh
```

La TUI est la **tour de contrôle** : on y lance les sessions, on les suit et on y prend les décisions. Le travail de l'agent se fait dans la fenêtre opencode, ouverte à côté (onglet iTerm2 ou Terminal.app, tmux, navigateur). Fermer opencode n'arrête pas la session. Voir [Sessions v5](sessions-v5.fr.md).

Au premier démarrage sans configuration, l'assistant `oh init` se lance dans la TUI (voir [Démarrage rapide](getting-started.fr.md#configuration-initiale--oh-init)).

---

## Navigation

### Raccourcis globaux

| Touche | Action |
|--------|--------|
| `Ctrl+P` ou `/` | Ouvrir l'omnibar (`/` : si la vue ne l'utilise pas pour chercher) |
| `?` ou `F1` | Aide (raccourcis globaux, fiche et assistants, touches de la vue active) |
| `Esc` | Fermer les toasts affichés ; sinon retour à la vue précédente ; à la racine d'un mode projet ou équipe, retour au mode Hub. Dans la fiche de lancement et les assistants, `Esc` garde son sens (fermer la fiche, « Quitter l'assistant ? ») et ne ferme pas les toasts |
| `Ctrl+T` | Entrer en mode équipe (choix si plusieurs équipes) ; depuis le mode équipe, revenir au Hub |
| `Ctrl+Q` (ou `Ctrl+C`) | Quitter (voir [Fermer oh](#fermer-oh)) |
| `j` / `k`, `g` / `G` | Bas / haut, début / fin de liste (dans la vue Sessions, `g` récupère une session distante) |
| `{` / `}` | Section précédente / suivante dans les listes à sections |
| `Tab` | Changer de colonne ou de champ (`h` / `l` sur les accueils à deux colonnes) |
| `Entree` | action de la liste, du bouton ou du champ sélectionné (l'omnibar s'ouvre seulement si rien ne l'utilise) |

Les résultats des actions s'affichent en toasts (succès, erreur, info) qui disparaissent seuls (`Esc` les ferme, sauf dans la fiche de lancement et les assistants). Chaque vue affiche ses raccourcis en bas.

**Changement en v5.0** : taper une lettre « n'importe où » n'ouvre plus l'omnibar (utilisez `Ctrl+P` ou `/`) ; `Entree` et les lettres vont toujours à la liste ou au champ sélectionné ; `d` ne ferme plus les toasts (`Esc`). Les touches `Esc`, `Ctrl+P`, `Ctrl+T`, `?`, `F1` et `Ctrl+Q` ont le même sens dans toutes les vues. Toute action destructrice (supprimer un worktree, nettoyer, libérer un ticket, rejeter une proposition du wiki…) demande une confirmation.

**Boutons** (fenêtres, formulaires, assistants, récap) : un seul style partout. L'action principale est écrite en gras dans la couleur du mode ; le bouton sélectionné est sur fond clair, encadré `▸ libellé ◂`. À l'ouverture, l'action principale est sélectionnée (« Annuler » pour une confirmation dangereuse, comme une suppression). `Tab` / `Shift+Tab` (ou `←` / `→`) passent d'un bouton à l'autre, tous compris ; `↓` / `↑` font défiler le texte d'une fenêtre trop longue ; `Entree` valide le bouton sélectionné.

### Petits terminaux

La mise en page suit la taille du terminal :

| Taille | Mise en page |
|--------|--------------|
| 32 lignes et plus | complète : bandeau en grand, pied de page, barre du bas sur 8 lignes |
| 16 à 31 lignes | **compacte** : bandeau sur une ligne, pas de pied de page ni d'espaces, barre du bas sur 3 lignes (≈ 19 lignes de liste en 80×24) |
| moins de 60 colonnes ou 16 lignes | message « Terminal trop petit (L×H) — 60×16 minimum » ; seul `Ctrl+Q` fonctionne |

Le passage d'une mise en page à l'autre garde l'élément sélectionné. Réglage `[cli] compact = auto | always | never` (Réglages › CLI › Mode compact, ou `oh config set cli.compact always`) : `auto` (défaut) suit la hauteur, `always` force la mise en page compacte, `never` la mise en page complète (le message « trop petit » reste).

### Badge des sessions

La barre du bas affiche partout `● N ⏸ M` : `N` sessions vivantes (ni en veille ni terminées), `M` décisions en attente. Le badge passe en couleur d'alerte quand `M > 0`, et un toast annonce chaque nouvelle décision. Les notifications système viennent du démon oh, même TUI fermée.

### Modes

| Mode | Contenu de l'accueil |
|------|----------------------|
| **Hub** | Démarrer, Sessions en cours, Système (Configuration = Réglages, Métriques, Doctor, Secrets), Projets (projets connus, puis « Ajouter un projet »), Équipes |
| **Projet** | Démarrer (avec catégories), Sessions du projet, Projet (Board, Métriques, Statut), Configuration (Config Projet, Worktrees, Workflows du projet), Équipe |
| **Équipe** | Démarrer (avec catégories), Sessions de l'équipe, Board (board d'équipe, statut, activité, résumés de reprise), Configuration (patterns, politiques, détail équipe, workflows de l'équipe), Navigation |

Sélectionner un projet ou une équipe sur l'accueil Hub ouvre son mode. Lancée depuis le dossier d'un projet enregistré, `oh` démarre en mode Projet. L'omnibar ne montre que les commandes du mode actif. Sur les accueils, les textes trop longs sont coupés par « … » et les deux colonnes s'élargissent avec le terminal (jusqu'à 180 colonnes).

---

## Démarrer

La section **Démarrer** est la même sur les trois accueils :

| Ligne | Contenu |
|-------|---------|
| `◆` | workflow par défaut du projet (Config projet › Exécution), s'il y en a un |
| `★` | workflows épinglés (5 au maximum par portée) |
| `·` Récents / `↺` | les 3 derniers workflows lancés (hors épinglés), avec leur ancienneté |
| `▸ Développer (N)`, `Cadrer`, `Qualité`, `Connaissance`, `Autres` | catégories (modes Projet et Équipe) : `Entree` liste leurs workflows |
| `… Tous les workflows (N)` | ouvre le catalogue des workflows |

| Touche | Action |
|--------|--------|
| `Entree` | ouvrir la fiche de lancement du workflow |
| `*` | épingler / desepingler (portée de l'accueil : tout le hub, ce projet ou cette équipe) |

Si aucun workflow n'est disponible, une ligne « Session libre » remplace la liste.

---

## Fiche de lancement

Ouverte depuis « Démarrer », le catalogue, l'omnibar (`run <workflow>`) ou le board (`a` sur un ticket). Elle est générée à partir du workflow et a trois étapes :

| Étape | Contenu |
|-------|---------|
| **Entrées** | les entrées du workflow : ticket (champ + bouton « Choisir… »), texte, liste (`enum`), case (`bool`), branche… Les champs obligatoires sont marqués |
| **Options** | Mode (modes autorisés), Exécution (`⌂ local`, `▣ conteneur`, `☁ distant`, avec la raison si indisponible), Emplacement (base, worktree existant, « + nouveau worktree »), Ouverture, et « Une seule session pour tous les tickets » si plusieurs tickets |
| **Récap** | agents, skills et budget, MCP, Code Mode, commandes Beads autorisées, checkpoints dans le mode choisi (obligatoires signalés), isolation, sessions (« N sessions · 1 serveur · un worktree par session qui écrit »), avertissements (décisions déjà en attente, préconditions). Quand une session qui écrit démarrerait dans un dossier modifié : `[w]` nouveau worktree (défaut, vos modifications restent intactes) · `[s]` mettre de côté (stash) · `[b]` lancer quand même |

| Touche | Action |
|--------|--------|
| `Tab` | champ suivant |
| `Ctrl+S` | aller au récap (depuis n'importe quelle étape) ; sur le récap chargé : lancer |
| `Alt+←`, `PgUp` ou `Ctrl+B` | étape précédente |
| `Alt+→` ou `PgDn` | étape suivante |
| `Esc` | fermer la fiche |

On lance toujours depuis le récap : un premier `Ctrl+S` l'affiche, un second lance une fois le récap calculé. Les boutons « Suivant », « Retour » et « Lancer » font la même chose. Pendant le lancement, la fiche affiche « Lancement en cours… » (un second `Ctrl+S` est ignoré) ; en conteneur, la progression de la construction de l'image s'affiche dessous. Si le fournisseur n'a pas d'identifiants, le récap l'indique avec un bouton « Configurer le fournisseur » : il ouvre la vue Fournisseur, et au retour (`Esc`) le récap vérifie à nouveau. « Lancer » est l'action principale du récap.

**Ticket déjà en cours** : si une session ouverte travaille déjà sur le ticket, le récap la compare au lancement (tableau « Session existante / Nouveau lancement » : état, workflow, exécution, mode, modèle, dossier / branche, travail ; différences marquées ⚠). Une session d'un autre workflow est affichée pour information. Pour une session du même workflow, il faut choisir (un choix lance aussitôt ; « Lancer » / `Ctrl+S` ne suffisent plus) :

| Touche | Bouton | Effet |
|--------|--------|-------|
| `r` | Reprendre | rouvrir la session existante, pas de nouvelle session |
| `t` | Reprendre en local / en conteneur / avec ces réglages | transférer la session et sa conversation vers l'exécution et le modèle choisis ; en cas d'échec (« Transfert impossible »), la session est inchangée et oh propose « Remplacer » |
| `k` | Remplacer (garder le travail) | arrêter la session ; la nouvelle continue dans le même dossier et la même branche, avec un brief de passation |
| `d` | Supprimer et relancer | supprimer la session et son travail (confirmation : ce qui sera supprimé et perdu, puis, si elle travaillait dans le dossier du projet, garder ou supprimer ses modifications), puis relancer |
| `p` | Lancer en parallèle | nouvelle session à côté, dans un autre worktree |

Voir [Sessions v5 › Lancer sur un ticket déjà en cours](sessions-v5.fr.md#lancer-sur-un-ticket-déjà-en-cours).

### Sélecteur de tickets Beads

Le bouton « Choisir… » d'une entrée ticket ouvre le sélecteur dans la fiche :

| Touche | Action |
|--------|--------|
| `/` | rechercher (id, titre, libellé) ; `Entree` ou `↓` revient à la liste, `Esc` efface |
| `f` | changer de filtre de libellé (par défaut celui du workflow, ex. `ai-delegated`) |
| `e` | changer d'épopée |
| `Espace` | sélectionner / désélectionner (si le workflow accepte plusieurs tickets) |
| `Entree` | choisir (ou valider la sélection) |
| `Esc` | annuler |

Un aperçu du ticket (critères d'acceptation) s'affiche sous la liste ; un ticket réservé par un autre membre est signalé. Plusieurs tickets = une session par ticket, sauf si « Une seule session pour tous les tickets » est cochee.

---

## Vue Sessions

Commande `sessions` (alias `parallel`, `inbox`), ou la section Sessions des accueils (3 lignes au plus, puis « Toutes les sessions »).

| Section | Contenu |
|---------|---------|
| **À traiter** | sessions qui ont des décisions en attente, chacune suivie de ses décisions : `⏸` checkpoint, `?` question, `!` permission, `$` budget, `✗` erreur ou coupe-circuit (une session n'apparaît qu'une fois dans la vue, avec son identifiant court) |
| **En cours** | sessions vivantes (au travail, en attente, inactives) |
| **À récupérer** | sessions distantes (GitLab CI) : pipeline en cours, MR prête, journal Beads à rejouer |
| **En veille** | sessions dont le serveur dort ; elles se reprennent |
| **Terminées, 7 j** | sessions arrêtées ou terminées depuis moins de 7 jours |

Le panneau **Détail** montre la session sélectionnée : workflow, mode, exécution, dossier, paquet, état, agent, coût, la **frise** des checkpoints (`✔ cp-1 10:03 → developer (3) → reviewer → ⏸ cp-2 → ○ cp-3`), les décisions ouvertes et, quand une suite est connue, « ↪ Enchaîner avec <workflow> (e) ».

| Touche | Action |
|--------|--------|
| `Entree` | sur une décision : ouvrir sa fiche ; sur une session : afficher / masquer le flux |
| `y` | valider (permission : une fois ; checkpoint : Valider) |
| `n` | refuser une permission ; sur un checkpoint : fiche sur « Corriger d'abord » |
| `x` | classer une alerte (`✗` erreur, `$` budget, `✗` coupe-circuit) |
| `a` | attacher (ouvrir la fenêtre opencode ; reprend une session en veille) |
| `A` | ouvrir avec… : automatique, iTerm2, Terminal.app, tmux, navigateur, ici (oh suspendu) |
| `t` | flux en direct |
| `m` | envoyer une consigne (prise en compte à la prochaine étape) |
| `i` | interrompre l'étape en cours |
| `M` | changer le modèle des prochaines étapes (`fournisseur/modele`) |
| `s` | arrêter la session (confirmation) |
| `B` | valider par lot : les checkpoints en attente du fan-out de la session sélectionnée (sinon tous), chacun à valider, laisser en attente ou refuser avec une consigne ; confirmation, puis le résultat de chacun (comme `oh session approve --batch`) |
| `D` | supprimer définitivement la session : worktree, branche locale (jamais la distante), décisions, fichiers (confirmation, « Annuler » sélectionné) |
| `c` | reprendre une session en veille |
| `o` | résultats et description de MR |
| `w` | ouvrir dans le navigateur (code à usage unique) |
| `e` | enchaîner avec un autre workflow |
| `g` | récupérer une session distante |
| `f` | filtre : projet actif / tous les projets |
| `r` | rafraîchir |

### Fiche checkpoint

`Entree` sur un `⏸` ouvre la fiche : titre `⏸ cp-2 · Commit ou correction · ticket · my-app`, résumé de l'agent, **Changements** (`+a −d · N fichier(s)`, 8 fichiers listes), **Derniers messages**, **Frise**. Actions :

- **Décider** : formulaire avec trois choix et un « Message à l'agent » :

| Choix | Effet | Message |
|-------|-------|---------|
| **Valider** | le checkpoint passe | facultatif |
| **Corriger d'abord** | l'agent corrige, puis redemande le checkpoint | obligatoire |
| **Autre consigne** | l'agent suit la consigne, puis redemande | obligatoire |

- **Diff complet** : tout le diff de la session (si elle a des changements).
- **Attacher** : ouvrir la fenêtre opencode.

Équivalent en CLI : `oh session approve <id> --decision once|fix|other|reject [-m "…"]`.

### Autres fiches

- **Permission** (`!`) : action, ressource, agent ; une fois / toujours / refuser, avec un message. « Toujours » est refusé si le workflow impose l'isolation stricte.
- **Question** (`?`) : formulaire généré depuis les champs de l'agent (choix, « autre réponse » si permise, booléen, choix multiple, texte).
- **Alerte** (`✗`, `$`) : classer ou attacher. Le coupe-circuit (`✗`) suspend les délégations après N délégations d'affilée ; le classer les débloque.

Si la décision a déjà été prise ailleurs (fenêtre opencode, navigateur, CLI), la fiche l'indique : la première réponse gagne.

### Flux en direct (`t`)

Un panneau s'ouvre à droite : agent courant, outils appelés et leur résultat, délégations, messages, coût dans le titre. Il suit la session sélectionnée (changer de ligne change de flux). `t` le referme. Sans démon oh, le flux est indisponible.

### Enchaîner avec… (`e`)

`e` propose les workflows dont une entrée prend une sortie de la session (branche, tickets, chemin), par exemple `review` après `ticket`. Le choix ouvre la fiche de lancement préremplie, liée à la session précédente. Quand une session de workflow s'arrête, un toast le rappelle : « ✔ … terminée · $… — Sessions › e : enchaîner avec … ».

### Récupérer une session distante (`g`)

Sur une ligne « À récupérer » : compte rendu (artefacts, import de la session, checkpoint différé), puis « Rejouer le journal » Beads. Un ticket modifié entre-temps ouvre une fenêtre de conflit : « Garder local », « Appliquer distant », « Fusionner les notes » ou « Plus tard ». Voir [Exécution distante](remote-runners.fr.md).

---

## Fermer oh

`Ctrl+Q` (ou la commande `quit`). Si des sessions travaillent encore, la fenêtre « Quitter oh — N session(s) travaillent encore » propose, **pour chacune** :

| Choix | Effet |
|-------|-------|
| **Finir l'étape, veille** (défaut) | l'étape en cours se termine, puis la session se met en veille |
| **Arrière-plan** | la session continue sans la TUI |
| **Arrêter maintenant** | la session est arrêtée |

Les sessions en attente ou inactives sont mises en veille tout de suite. `Esc` annule la fermeture. Au retour, un message résume ce qui s'est passé pendant votre absence (sessions changées, décisions à traiter, coût).

---

## Catalogue des workflows

Commande `workflows` (ou « Tous les workflows », « Workflows du projet », l'entrée « Workflow » du mode équipe). Les workflows sont groupés par couche : **Hub · intégrés** (lecture seule), **Équipe**, **Projet**, **Mes brouillons**. Chaque ligne indique la version, le risque, les exécutions possibles (`⌂ ▣ ☁`), l'état de validation et la chaîne `extends`.

| Touche | Action |
|--------|--------|
| `Entree` | lancer (fiche de lancement) ; sur un brouillon : le tester |
| `n` | nouveau workflow (vide, extension ou copie) |
| `e` | éditer ; sur un workflow du hub : l'étendre ; sur un workflow publié : créer un brouillon |
| `v` | valider (diagnostics) |
| `t` | tester le brouillon (lancement avec `--draft`) |
| `p` | publier le brouillon |
| `D` | diff du brouillon avec la version publiée |
| `h` | historique des versions |
| `x` | archiver un workflow publié, abandonner un brouillon |
| `*` | épingler |
| `r` | recharger |

**Publication** (`p`) : validation, impact (droits et contrôles changés), diff, message obligatoire ; `Ctrl+S` publié. Hors ligne, la publication est mise en attente et rejouée à la synchronisation suivante. **Historique** (`h`) : `Entree` diff avec la version actuelle, `r` restaurer (republiée comme nouvelle version). Voir [Workflows d'équipe](team-workflows.fr.md).

### Éditeur

Sections (`Tab` / `Shift+Tab`) : **Général**, **Graphe**, **Entrées & prompt**, **Ressources**, **Aperçu paquet**. Chaque champ indique son origine (héritée ou écrite dans le brouillon) et s'il est verrouillé par le workflow parent.

| Touche | Action |
|--------|--------|
| `Entree` | modifier le champ (Aperçu paquet : aller au champ du diagnostic) |
| `x` | revenir à la valeur héritée ; retirer un élément |
| `a` | ajouter un agent (Graphe) ou une entrée (Entrées & prompt) |
| `c` | ajouter un checkpoint (Graphe) |
| `m` | changer le mode affiché (Graphe) |
| `P` | éditer le gabarit du prompt dans `$EDITOR` |
| `y` | éditer le YAML dans `$EDITOR` |
| `u` / `U` | annuler / rétablir |
| `w` ou `Ctrl+S` | enregistrer le brouillon |
| `Esc` | fermer (demande quoi faire s'il reste des modifications) |

---

## Catalogue des briques

Commande `bricks` (alias `briques`, `agents`, `skills`). Liste en lecture seule des agents et des skills : origine (hub ou équipe), famille, coût estimé en tokens, dépendances, workflows qui les utilisent.

| Touche | Action |
|--------|--------|
| `Entree` | ouvrir un workflow qui utilise la brique |
| `/` | chercher (identifiant, nom, description) |
| `f` | agents / skills / tout |
| `Esc` | retour |

---

## Nettoyage des anciens déploiements

Commande `cleanup` (alias `nettoyage`, `deploy-cleanup`). oh v5 ne déploie plus rien dans les projets ; l'écran liste, par projet, les restes des anciens déploiements (`.opencode/agents`, `.opencode/skills`, clés écrites par oh dans `opencode.json`…) et ce qui est conservé (vos clés personnelles). Boutons : **Nettoyer**, **Voir le diff**, **Plus tard**. Équivalent CLI : `oh migrate deploy-cleanup`.

---

## Réglages

Commande `settings`. Les sections propres à v5 :

| Section | Champs |
|---------|--------|
| **Sessions** | Ouverture des sessions (auto, iTerm2, Terminal.app, tmux, navigateur, suspension), Style iTerm2 (onglet, volet, fenêtre), Veille après (minutes, 5 par défaut) |
| **Restrictions des sessions** (désactivées si vides) | Sessions actives max, Budget par session (USD), Budget journalier (USD), Plafond mémoire (Mo), Modèles autorisés |
| **Exécution** | Runtime par défaut, Moteur de conteneurs (auto = Colima, puis Podman, puis Docker), Cache des images, Version d'opencode figée, Isolation stricte |
| **Distant (GitLab CI)** | par cible : instance, projet `oh-runner`, clé du jeton (lecture seule), étiquette, construction de l'image, architecture, durée maximale ; appliqués par le prochain `oh remote setup` |

Les autres sections (Général, CLI, Opencode, Workflows, MCP, Worktree, Tracker) gardent leur rôle. Dans les vues de configuration : `j`/`k` pour naviguer, `Entree` pour modifier ; les changements sont enregistrés.

**Config projet** (`project-config`) a sa section **Exécution** : Dockerfile de dev, Build args, Volumes de cache, Workflow par défaut, Runtime par défaut. Voir [Sessions en conteneur](container.fr.md).

---

## Omnibar

Fuzzy matching sur les noms et les alias :

```
> tick        → run ticket (fiche de lancement)
> secu        → run audit
> brick       → catalogue des briques
> inbox       → vue Sessions
```

### Sessions et workflows

| Commande | Description |
|----------|-------------|
| `run <workflow>` | fiche de lancement d'un workflow du catalogue. Alias : `dev` → `run ticket`, `start`, `feature` → `run spec`, `onboard` → `run onboarding`, `feedback` → `run review-feedback`, `secu`/`perf`/`archi` → `run audit`, `rev` → `run review`, `dbg` → `run debug`, `fast` → `run quick` |
| `run <workflow> ⟨bd-42⟩` | depuis un board : workflow sur le ticket sélectionné |
| `coder` | Session libre (workflow `libre`, agent au choix) — modes Projet et Équipe |
| `sessions` | vue Sessions (alias `parallel`, `inbox`) |
| `workflows` | catalogue des workflows (alias `catalogue`, `wf`) |
| `bricks` | catalogue des briques |
| `cleanup` | nettoyage des anciens déploiements |
| `review.publish` | publier une review (MR + notification), si l'écriture GitLab est activée |

### Projets et configuration

| Commande | Description |
|----------|-------------|
| `worktrees` | worktrees git du projet (`a` ajouter, `d` supprimer, `o` ouvrir une session, `p` nettoyer les références, `x` supprimer les worktrees fusionnés, jamais ceux qu'utilise une session ; `d`, `p` et `x` demandent confirmation) |
| `projects` | liste des projets (`a` ajouter, `d` supprimer, `n` renommer, `m` déplacer, `p` mode Projet, `b` initialiser Beads, `r` rafraîchir, `Entree` configurer) |
| `project add` | assistant d'ajout de projet |
| `board`, `board init` | board Beads du projet, initialisation |
| `project-config` | configuration du projet (dont Exécution) |
| `settings`, `models`, `provider`, `mcp`, `secrets` | réglages, modèles, fournisseur (région Bedrock vérifiée dès la saisie, message sous le champ), serveurs MCP, secrets |
| `teams`, `team-detail` | équipes, détail de l'équipe (`K` clé du fournisseur pour l'équipe ; espace solo : « Passer en équipe ») |
| `init` | relancer l'assistant de configuration |

### Équipe (avec un dépôt team-state)

| Commande | Description |
|----------|-------------|
| `team board`, `team status`, `team activity` | board, statut, activité de l'équipe |
| `team briefs`, `team patterns`, `team policies`, `team wiki` | résumés de reprise, patterns, politiques, wiki |
| `team sync` | synchroniser les claims avec le tracker |
| `team init`, `team rejoin` | créer, rejoindre une équipe |

### Système et navigation

| Commande | Description |
|----------|-------------|
| `status`, `doctor`, `metrics`, `notifications`, `help` | statut, diagnostic (`r` relancer), métriques, notifications, aide |
| `home`, `project mode`, `hub mode` | accueil, mode Projet, mode Hub |
| `quit` | quitter (même fenêtre que `Ctrl+Q`) |

Commandes supprimées en v5 : `deploy`, `sync`, `plugins`, `upgrade`, la vue Workflow (remplacée par le catalogue) et l'ancienne vue parallèle (remplacée par Sessions).

---

## Assistants inline

`init`, `project add` et `team init` ouvrent des assistants dans la TUI. L'omnibar et les toasts restent accessibles.

| Touche | Action |
|--------|--------|
| `Ctrl+S` | valider l'étape |
| `Alt+←`, `PgUp` ou `Ctrl+B` | étape précédente |
| `Esc` | « Quitter l'assistant ? » : **Quitter**, **Passer l'étape** (si elle est facultative) ou **Continuer** (choix par défaut ; `Esc` aussi) |

Un écran de résumé termine l'assistant : `Entree` ouvre la vue de détail, `Esc` revient à la vue précédente.

---

## Astuces

- **Lancer sans quitter le board** : `a` sur un ticket liste les workflows qui prennent un ticket ; la fiche s'ouvre à l'étape Options, ticket prérempli.
- **Décider vite** : `y` sur une ligne de « À traiter » valide une permission ou un checkpoint sans ouvrir la fiche.
- **Plusieurs projets** : `f` dans la vue Sessions bascule entre le projet actif et tous les projets.
- **Aide contextuelle** : `?` liste les raccourcis de la vue active.
