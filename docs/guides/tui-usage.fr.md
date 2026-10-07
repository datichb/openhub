> [Read in English](tui-usage.en.md)

# Guide -- Utiliser le TUI OpenHub

> Guide pratique de la TUI d'oh v5 : lancer des workflows, piloter les sessions, prendre les decisions, gerer les workflows et les reglages.

## Demarrage rapide

```bash
oh
```

La TUI est la **tour de controle** : on y lance les sessions, on les suit et on y prend les decisions. Le travail de l'agent se fait dans la fenetre opencode, ouverte a cote (onglet iTerm2 ou Terminal.app, tmux, navigateur). Fermer opencode n'arrete pas la session. Voir [Sessions v5](sessions-v5.fr.md).

Au premier demarrage sans configuration, l'assistant `oh init` se lance dans la TUI (voir [Demarrage rapide](getting-started.fr.md#configuration-initiale--oh-init)).

---

## Navigation

### Raccourcis globaux

| Touche | Action |
|--------|--------|
| `Ctrl+P` | Ouvrir l'omnibar |
| une lettre | Ouvrir l'omnibar avec cette lettre (si la vue ne l'utilise pas) |
| `?` ou `F1` | Aide (raccourcis de la vue active) |
| `Esc` | Retour a la vue precedente ; a la racine d'un mode projet ou equipe, retour au mode Hub |
| `Ctrl+T` | Entrer en mode equipe (choix si plusieurs equipes) ; depuis le mode equipe, revenir au Hub |
| `Ctrl+Q` (ou `Ctrl+C`) | Quitter (voir [Fermer oh](#fermer-oh)) |
| `j` / `k`, `g` / `G` | Bas / haut, debut / fin de liste (dans la vue Sessions, `g` recupere une session distante) |
| `{` / `}` | Section precedente / suivante dans les listes a sections |
| `Tab` | Changer de colonne ou de champ (`h` / `l` sur les accueils a deux colonnes) |
| `d` | Fermer le plus ancien toast |

Les resultats des actions s'affichent en toasts (succes, erreur, info) qui disparaissent seuls. Chaque vue affiche ses raccourcis en bas.

### Badge des sessions

La barre du bas affiche partout `● N ⏸ M` : `N` sessions vivantes (ni en veille ni terminees), `M` decisions en attente. Le badge passe en couleur d'alerte quand `M > 0`, et un toast annonce chaque nouvelle decision. Les notifications systeme viennent du demon oh, meme TUI fermee.

### Modes

| Mode | Contenu de l'accueil |
|------|----------------------|
| **Hub** | Démarrer, Sessions en cours, Système (Configuration = Reglages, Métriques, Doctor, Secrets), Projets, Équipes |
| **Projet** | Démarrer (avec catégories), Sessions du projet, Projet (Board, Métriques, Statut), Configuration (Config Projet, Worktrees, Workflows du projet), Équipe |
| **Équipe** | Démarrer (avec catégories), Sessions de l'équipe, Board (board d'equipe, statut, activite, resumes de reprise), Configuration (patterns, politiques, detail equipe, workflows de l'equipe), Navigation |

Selectionner un projet ou une equipe sur l'accueil Hub ouvre son mode. Lance depuis le dossier d'un projet enregistre, `oh` demarre en mode Projet. L'omnibar ne montre que les commandes du mode actif.

---

## Démarrer

La section **Démarrer** est la meme sur les trois accueils :

| Ligne | Contenu |
|-------|---------|
| `◆` | workflow par defaut du projet (Config projet › Exécution), s'il y en a un |
| `★` | workflows epingles (5 au maximum par portee) |
| `·` Récents / `↺` | les 3 derniers workflows lances (hors epingles), avec leur anciennete |
| `▸ Développer (N)`, `Cadrer`, `Qualité`, `Connaissance`, `Autres` | categories (modes Projet et Equipe) : `Entree` liste leurs workflows |
| `… Tous les workflows (N)` | ouvre le catalogue des workflows |

| Touche | Action |
|--------|--------|
| `Entree` | ouvrir la fiche de lancement du workflow |
| `*` | epingler / desepingler (portee de l'accueil : tout le hub, ce projet ou cette equipe) |

Si aucun workflow n'est disponible, une ligne « Session libre » remplace la liste.

---

## Fiche de lancement

Ouverte depuis « Démarrer », le catalogue, l'omnibar (`run <workflow>`) ou le board (`a` sur un ticket). Elle est generee a partir du workflow et a trois etapes :

| Etape | Contenu |
|-------|---------|
| **Entrées** | les entrees du workflow : ticket (champ + bouton « Choisir… »), texte, liste (`enum`), case (`bool`), branche… Les champs obligatoires sont marques |
| **Options** | Mode (modes autorises), Exécution (`⌂ local`, `▣ conteneur`, `☁ distant`, avec la raison si indisponible), Emplacement (base, worktree existant, « + nouveau worktree »), Ouverture, et « Une seule session pour tous les tickets » si plusieurs tickets |
| **Récap** | agents, skills et budget, MCP, isolation, sessions (« N sessions · 1 serveur · un worktree par session qui écrit »), avertissements (modifications non commitees, decisions deja en attente, preconditions) |

| Touche | Action |
|--------|--------|
| `Tab` | champ suivant |
| `Ctrl+S` | lancer, depuis n'importe quelle etape (sans passer par le recap) |
| `Ctrl+B` | etape precedente |
| `Esc` | fermer la fiche |

Les boutons « Suivant », « Retour » et « Lancer » font la meme chose. Pendant le lancement, la fiche affiche « Lancement en cours… » (un second `Ctrl+S` est ignore) ; en conteneur, la progression de la construction de l'image s'affiche dessous.

### Selecteur de tickets Beads

Le bouton « Choisir… » d'une entree ticket ouvre le selecteur dans la fiche :

| Touche | Action |
|--------|--------|
| `/` | rechercher (id, titre, libelle) ; `Entree` ou `↓` revient a la liste, `Esc` efface |
| `f` | changer de filtre de libelle (par defaut celui du workflow, ex. `ai-delegated`) |
| `e` | changer d'epopee |
| `Espace` | selectionner / deselectionner (si le workflow accepte plusieurs tickets) |
| `Entree` | choisir (ou valider la selection) |
| `Esc` | annuler |

Un apercu du ticket (criteres d'acceptation) s'affiche sous la liste ; un ticket reserve par un autre membre est signale. Plusieurs tickets = une session par ticket, sauf si « Une seule session pour tous les tickets » est cochee.

---

## Vue Sessions

Commande `sessions` (alias `parallel`, `inbox`), ou la section Sessions des accueils (3 lignes au plus, puis « Toutes les sessions »).

| Section | Contenu |
|---------|---------|
| **À traiter** | decisions de toutes les sessions : `⏸` checkpoint, `?` question, `!` permission, `$` budget, `✗` erreur ou coupe-circuit |
| **En cours** | sessions vivantes (au travail, en attente, inactives) |
| **À récupérer** | sessions distantes (GitLab CI) : pipeline en cours, MR prete, journal Beads a rejouer |
| **En veille** | sessions dont le serveur dort ; elles se reprennent |
| **Terminées, 7 j** | sessions arretees ou terminees depuis moins de 7 jours |

Le panneau **Détail** montre la session selectionnee : workflow, mode, execution, dossier, paquet, etat, agent, cout, la **frise** des checkpoints (`✔ cp-1 10:03 → developer (3) → reviewer → ⏸ cp-2 → ○ cp-3`), les decisions ouvertes et, quand une suite est connue, « ↪ Enchaîner avec <workflow> (e) ».

| Touche | Action |
|--------|--------|
| `Entree` | sur une decision : ouvrir sa fiche ; sur une session : afficher / masquer le flux |
| `y` | valider (permission : une fois ; checkpoint : Valider) |
| `n` | refuser une permission ; sur un checkpoint : fiche sur « Corriger d'abord » |
| `x` | classer une alerte (`✗` erreur, `$` budget, `✗` coupe-circuit) |
| `a` | attacher (ouvrir la fenetre opencode ; reprend une session en veille) |
| `A` | ouvrir avec… : automatique, iTerm2, Terminal.app, tmux, navigateur, ici (oh suspendu) |
| `t` | flux en direct |
| `m` | envoyer une consigne (prise en compte a la prochaine etape) |
| `i` | interrompre l'etape en cours |
| `M` | changer le modele des prochaines etapes (`fournisseur/modele`) |
| `s` | arreter la session (confirmation) |
| `c` | reprendre une session en veille |
| `o` | resultats et description de MR |
| `w` | ouvrir dans le navigateur (code a usage unique) |
| `e` | enchainer avec un autre workflow |
| `g` | recuperer une session distante |
| `f` | filtre : projet actif / tous les projets |
| `r` | rafraichir |

### Fiche checkpoint

`Entree` sur un `⏸` ouvre la fiche : titre `⏸ cp-2 · Commit ou correction · ticket · my-app`, resume de l'agent, **Changements** (`+a −d · N fichier(s)`, 8 fichiers listes), **Derniers messages**, **Frise**. Actions :

- **Décider** : formulaire avec trois choix et un « Message à l'agent » :

| Choix | Effet | Message |
|-------|-------|---------|
| **Valider** | le checkpoint passe | facultatif |
| **Corriger d'abord** | l'agent corrige, puis redemande le checkpoint | obligatoire |
| **Autre consigne** | l'agent suit la consigne, puis redemande | obligatoire |

- **Diff complet** : tout le diff de la session (si elle a des changements).
- **Attacher** : ouvrir la fenetre opencode.

Equivalent en CLI : `oh session approve <id> --decision once|fix|other|reject [-m "…"]`.

### Autres fiches

- **Permission** (`!`) : action, ressource, agent ; une fois / toujours / refuser, avec un message. « Toujours » est refuse si le workflow impose l'isolation stricte.
- **Question** (`?`) : formulaire genere depuis les champs de l'agent (choix, « autre reponse » si permise, booleen, choix multiple, texte).
- **Alerte** (`✗`, `$`) : classer ou attacher. Le coupe-circuit (`✗`) suspend les delegations apres N delegations d'affilee ; le classer les debloque.

Si la decision a deja ete prise ailleurs (fenetre opencode, navigateur, CLI), la fiche l'indique : la premiere reponse gagne.

### Flux en direct (`t`)

Un panneau s'ouvre a droite : agent courant, outils appeles et leur resultat, delegations, messages, cout dans le titre. Il suit la session selectionnee (changer de ligne change de flux). `t` le referme. Sans demon oh, le flux est indisponible.

### Enchaîner avec… (`e`)

`e` propose les workflows dont une entree prend une sortie de la session (branche, tickets, chemin), par exemple `review` apres `ticket`. Le choix ouvre la fiche de lancement preremplie, liee a la session precedente. Quand une session de workflow s'arrete, un toast le rappelle : « ✔ … terminée · $… — Sessions › e : enchaîner avec … ».

### Recuperer une session distante (`g`)

Sur une ligne « À récupérer » : compte rendu (artefacts, import de la session, checkpoint differe), puis « Rejouer le journal » Beads. Un ticket modifie entre-temps ouvre une fenetre de conflit : « Garder local », « Appliquer distant », « Fusionner les notes » ou « Plus tard ». Voir [Execution distante](remote-runners.fr.md).

---

## Fermer oh

`Ctrl+Q` (ou la commande `quit`). Si des sessions travaillent encore, la fenetre « Quitter oh — N session(s) travaillent encore » propose, **pour chacune** :

| Choix | Effet |
|-------|-------|
| **Finir l'étape, veille** (defaut) | l'etape en cours se termine, puis la session se met en veille |
| **Arrière-plan** | la session continue sans la TUI |
| **Arrêter maintenant** | la session est arretee |

Les sessions en attente ou inactives sont mises en veille tout de suite. `Esc` annule la fermeture. Au retour, un message resume ce qui s'est passe pendant votre absence (sessions changees, decisions a traiter, cout).

---

## Catalogue des workflows

Commande `workflows` (ou « Tous les workflows », « Workflows du projet », l'entree « Workflow » du mode equipe). Les workflows sont groupes par couche : **Hub · intégrés** (lecture seule), **Équipe**, **Projet**, **Mes brouillons**. Chaque ligne indique la version, le risque, les executions possibles (`⌂ ▣ ☁`), l'etat de validation et la chaine `extends`.

| Touche | Action |
|--------|--------|
| `Entree` | lancer (fiche de lancement) ; sur un brouillon : le tester |
| `n` | nouveau workflow (vide, extension ou copie) |
| `e` | editer ; sur un workflow du hub : l'etendre ; sur un workflow publie : creer un brouillon |
| `v` | valider (diagnostics) |
| `t` | tester le brouillon (lancement avec `--draft`) |
| `p` | publier le brouillon |
| `D` | diff du brouillon avec la version publiee |
| `h` | historique des versions |
| `x` | archiver un workflow publie, abandonner un brouillon |
| `*` | epingler |
| `r` | recharger |

**Publication** (`p`) : validation, impact (droits et controles changes), diff, message obligatoire ; `Ctrl+S` publie. Hors ligne, la publication est mise en attente et rejouee a la synchronisation suivante. **Historique** (`h`) : `Entree` diff avec la version actuelle, `r` restaurer (republiee comme nouvelle version). Voir [Workflows d'equipe](team-workflows.fr.md).

### Editeur

Sections (`Tab` / `Shift+Tab`) : **Général**, **Graphe**, **Entrées & prompt**, **Ressources**, **Aperçu paquet**. Chaque champ indique son origine (heritee ou ecrite dans le brouillon) et s'il est verrouille par le workflow parent.

| Touche | Action |
|--------|--------|
| `Entree` | modifier le champ (Aperçu paquet : aller au champ du diagnostic) |
| `x` | revenir a la valeur heritee ; retirer un element |
| `a` | ajouter un agent (Graphe) ou une entree (Entrées & prompt) |
| `c` | ajouter un checkpoint (Graphe) |
| `m` | changer le mode affiche (Graphe) |
| `P` | editer le gabarit du prompt dans `$EDITOR` |
| `y` | editer le YAML dans `$EDITOR` |
| `u` / `U` | annuler / retablir |
| `w` ou `Ctrl+S` | enregistrer le brouillon |
| `Esc` | fermer (demande quoi faire s'il reste des modifications) |

---

## Catalogue des briques

Commande `bricks` (alias `briques`, `agents`, `skills`). Liste en lecture seule des agents et des skills : origine (hub ou equipe), famille, cout estime en tokens, dependances, workflows qui les utilisent.

| Touche | Action |
|--------|--------|
| `Entree` | ouvrir un workflow qui utilise la brique |
| `/` | chercher (identifiant, nom, description) |
| `f` | agents / skills / tout |
| `Esc` | retour |

---

## Nettoyage des anciens deploiements

Commande `cleanup` (alias `nettoyage`, `deploy-cleanup`). oh v5 ne deploie plus rien dans les projets ; l'ecran liste, par projet, les restes des anciens deploiements (`.opencode/agents`, `.opencode/skills`, cles ecrites par oh dans `opencode.json`…) et ce qui est conserve (vos cles personnelles). Boutons : **Nettoyer**, **Voir le diff**, **Plus tard**. Equivalent CLI : `oh migrate deploy-cleanup`.

---

## Reglages

Commande `settings`. Les sections propres a v5 :

| Section | Champs |
|---------|--------|
| **Sessions** | Ouverture des sessions (auto, iTerm2, Terminal.app, tmux, navigateur, suspension), Style iTerm2 (onglet, volet, fenetre), Veille après (minutes, 5 par defaut) |
| **Restrictions des sessions** (desactivees si vides) | Sessions actives max, Budget par session (USD), Budget journalier (USD), Plafond mémoire (Mo), Modèles autorisés |
| **Exécution** | Runtime par défaut, Moteur de conteneurs (auto = Colima, puis Podman, puis Docker), Cache des images, Version d'opencode figée, Isolation stricte |
| **Distant (GitLab CI)** | par cible : instance, projet `oh-runner`, cle du jeton (lecture seule), etiquette, construction de l'image, architecture, duree maximale ; appliques par le prochain `oh remote setup` |

Les autres sections (Général, CLI, Opencode, Workflows, MCP, Worktree, Tracker) gardent leur role. Dans les vues de configuration : `j`/`k` pour naviguer, `Entree` pour modifier ; les changements sont enregistres.

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
| `run <workflow>` | fiche de lancement d'un workflow du catalogue. Alias : `dev` → `run ticket`, `start` → `run feature`, `onboard` → `run onboarding`, `feedback` → `run review-feedback`, `secu`/`perf`/`archi` → `run audit`, `rev` → `run review`, `dbg` → `run debug`, `fast` → `run quick` |
| `run <workflow> ⟨bd-42⟩` | depuis un board : workflow sur le ticket selectionne |
| `coder` | Session libre (workflow `libre`, agent au choix) — modes Projet et Equipe |
| `sessions` | vue Sessions (alias `parallel`, `inbox`) |
| `workflows` | catalogue des workflows (alias `catalogue`, `wf`) |
| `bricks` | catalogue des briques |
| `cleanup` | nettoyage des anciens deploiements |
| `review.publish` | publier une review (MR + notification), si l'ecriture GitLab est activee |

### Projets et configuration

| Commande | Description |
|----------|-------------|
| `projects` | liste des projets (`a` ajouter, `d` supprimer, `n` renommer, `m` deplacer, `p` mode Projet, `b` initialiser Beads, `r` rafraichir, `Entree` configurer) |
| `project add` | assistant d'ajout de projet |
| `board`, `board init` | board Beads du projet, initialisation |
| `project-config` | configuration du projet (dont Exécution) |
| `worktrees` | worktrees git |
| `settings`, `models`, `provider`, `mcp`, `secrets` | reglages, modeles, fournisseur, serveurs MCP, secrets |
| `teams`, `team-detail` | equipes, detail de l'equipe (`K` cle du fournisseur pour l'equipe ; espace solo : « Passer en équipe ») |
| `init` | relancer l'assistant de configuration |

### Equipe (avec un depot team-state)

| Commande | Description |
|----------|-------------|
| `team board`, `team status`, `team activity` | board, statut, activite de l'equipe |
| `team briefs`, `team patterns`, `team policies`, `team wiki` | resumes de reprise, patterns, politiques, wiki |
| `team sync` | synchroniser les claims avec le tracker |
| `team init`, `team rejoin` | creer, rejoindre une equipe |

### Systeme et navigation

| Commande | Description |
|----------|-------------|
| `status`, `doctor`, `metrics`, `notifications`, `help` | statut, diagnostic (`r` relancer), metriques, notifications, aide |
| `home`, `project mode`, `hub mode` | accueil, mode Projet, mode Hub |
| `quit` | quitter (meme fenetre que `Ctrl+Q`) |

Commandes supprimees en v5 : `deploy`, `sync`, `plugins`, `upgrade`, la vue Workflow (remplacee par le catalogue) et l'ancienne vue parallele (remplacee par Sessions).

---

## Assistants inline

`init`, `project add` et `team init` ouvrent des assistants dans la TUI. L'omnibar et les toasts restent accessibles.

| Touche | Action |
|--------|--------|
| `Ctrl+S` | valider l'etape |
| `Ctrl+B` | etape precedente |
| `Esc` (1 fois) | indication « appuyer encore pour passer » |
| `Esc` (2 fois) | passer l'etape (sauf etape obligatoire) |

Un ecran de resume termine l'assistant : `Entree` ouvre la vue de detail, `Esc` revient a la vue precedente.

---

## Astuces

- **Lancer sans quitter le board** : `a` sur un ticket liste les workflows qui prennent un ticket ; la fiche s'ouvre a l'etape Options, ticket prerempli.
- **Decider vite** : `y` sur une ligne de « À traiter » valide une permission ou un checkpoint sans ouvrir la fiche.
- **Plusieurs projets** : `f` dans la vue Sessions bascule entre le projet actif et tous les projets.
- **Aide contextuelle** : `?` liste les raccourcis de la vue active.
