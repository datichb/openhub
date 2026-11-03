---
name: tui-patterns
description: "Utiliser quand la spec porte sur une interface terminal (TUI OpenHub) : contraintes, clavier d'abord, widgets, anti-patterns."
---

# Patterns TUI — référence

Les TUI ne sont pas des interfaces web en monochrome. Elles ont leurs propres contraintes,
patterns et forces. Ce skill couvre les règles spécifiques au terminal.

---

## Contraintes terminales fondamentales

### Grille de caractères

Tout est aligné sur une grille de caractères (colonnes x lignes). Pas de pixels, pas de
sous-pixel, pas de positions fractionnaires. Un caractère = une cellule.
- Largeurs en **colonnes** (typiquement 80-200+ colonnes)
- Hauteurs en **lignes** (typiquement 24-60+ lignes)
- Un widget occupe un nombre entier de colonnes/lignes

### Couleurs

- **True color** (24-bit) : supporté par la plupart des terminaux modernes (iTerm2, kitty, Wezterm, Windows Terminal)
- **256 couleurs** : fallback pour les terminaux plus anciens
- Le design system Aurum utilise Catppuccin Mocha (true color) comme palette
- **Ne jamais se fier uniquement à la couleur** : toujours doubler avec une icône Unicode ou du texte
- Avec `NO_COLOR`, la TUI n'émet ni couleur ni gras : le sens doit tenir dans le symbole et le texte
- Une couleur ou un style ne s'écrit jamais en dur dans une vue (`[#64a0ff]`, `[::b]`, `tcell.ColorRed`) : on passe par `theme.Paint(couleur, texte)`, `theme.StyleTag`, `theme.BgTag`, qui suivent `NO_COLOR`

### Pas de souris fiable

La souris fonctionne dans certains terminaux mais pas tous. Les interactions doivent
fonctionner **uniquement au clavier**. La souris est un bonus, pas une dépendance.

### Navigation clavier-first

Le clavier est le moyen d'interaction principal. Chaque écran doit être entièrement
utilisable au clavier. Les raccourcis doivent être découvrables (affichés dans la
ligne d'aide de la vue ou dans l'aide `?`).

**Une touche passe toujours par le paquet `keys`** (`cli/internal/tui/v2/keys`), jamais par la
lecture d'une lettre (`case 'j'`, `ev.Rune() == ' '`) : une garde de test le vérifie. La
convention est la même dans toutes les vues ; une spec ne donne pas un autre sens à ces touches :

| Contexte | Touches |
|----------|---------|
| Partout | `↑↓`/`jk` déplacer, `g`/`G` premier/dernier, `/` filtrer, `r` rafraîchir, `?` ou `F1` aide, `Entrée` ouvrir, `Échap` revenir, `Ctrl+S` enregistrer (`keys.Save`) |
| Liste | `n` créer, `e` éditer, `d` supprimer (toujours confirmé), `x` archiver/classer (jamais destructif), `R` reprendre/restaurer, `c` comparer, `Espace` cocher/basculer (`keys.Toggle`) |
| Décision | `y` accepter, `n` refuser (avec consigne), `e` modifier, `s` laisser en attente, `A` tout accepter |
| Lecteur | `/` rechercher, `n`/`N` résultat suivant/précédent |
| Shell | `Ctrl+P` palette, `Ctrl+T` change de mode (Hub → Projet → Équipe → Hub), `Ctrl+Q` quitte |

- Les lettres `g G / r ? d x R y n` (et `j k`) ont **un seul sens** : une vue ne les détourne pas, et
  ne met pas une autre lettre sur une action de la convention (`a ajouter` → `n créer`, `w enregistrer` → `Ctrl+S`).
- Une touche qui n'est qu'à la vue (`a` attacher, `p` relancer…) se déclare avec `keys.Own` ; la ligne d'aide
  se construit avec `.Hint()` de l'action (`keys.Hints(keys.Navigate.Hint(), keys.Open.Hint(), …)`), pas avec du texte.
- Une touche qui change de sens entre deux versions s'ajoute à `keys.Changes()` : c'est ce qui alimente
  le message « Nouvelles touches ».
- `Alt+lettre` n'agit pas comme la lettre. Une ligne d'aide reste courte (10 entrées, 120 colonnes) ; au-delà,
  on garde l'essentiel avec `? aide`.

### Taille terminal variable

Le terminal peut être redimensionné à tout moment. Les layouts doivent s'adapter.
Définir des seuils minimum (ex : 80 colonnes, 24 lignes) en dessous desquels un
message "terminal trop petit" s'affiche.

---

## Patterns TUI d'OpenHub

### Omnibar (Command Palette)

Pattern central d'OpenHub. L'omnibar en bas de l'écran sert à la fois de :
- **Barre de hints** (mode passif) : affiche les raccourcis clavier contextuels
- **Command palette** (mode actif, Ctrl+P) : recherche fuzzy dans toutes les commandes
- **Filtre** : `/` ouvre la palette quand la vue n'a pas de filtre à elle ; une vue qui filtre prend `/` elle-même

Taper une lettre n'ouvre **pas** la palette : c'est `Ctrl+P` ou `/`.

**Règles :**
- Position fixe en bas
- Le contenu principal occupe tout le reste de l'écran
- Les suggestions flottent au-dessus de l'omnibar
- Escape ferme l'omnibar et revient au contenu
- Les vues peuvent injecter des commandes contextuelles via `CommandProvider`

### Stack-based Router (navigation entre vues)

Navigation par pile : Push (aller vers), Pop (revenir), Replace (remplacer la vue courante).

**Règles :**
- Escape = Pop (retour)
- Chaque vue a un cycle de vie Mount/Unmount
- Pas de navigation directe entre vues sœurs — toujours via le router
- Le titre de la vue courante est visible (pour l'orientation)

### Overlay system (modals TUI)

Les modals utilisent `tview.Pages` avec une grille 3x3 : fond assombri sur les 8
cellules périphériques, contenu dans la cellule centrale.

**Règles :**
- Deux couches : `inline-overlay` (modal primaire) + `sub-overlay` (sous-modal)
- Jamais plus de 2 niveaux d'overlay
- Un clic sur le fond assombri ferme les overlays simples (pas les formulaires)
- Escape ferme le dernier overlay

### Toast notifications

Notifications temporaires en haut à droite. 4 niveaux : Success, Error, Warning, Info.

**Règles :**
- Auto-dismiss (4 s pour succès et info, 8 s pour un avertissement, 10 s pour une erreur)
- Non bloquantes (ne prennent pas le focus)
- `Échap` ferme celles qui sont affichées avant de revenir en arrière
- Elles s'empilent, 5 au plus à l'écran ; les suivantes ne s'affichent pas mais restent dans l'historique
- Historique persistant (vue Notifications)
- Un message d'erreur dit quoi, pourquoi et que faire : un toast n'est pas le seul endroit où il est lisible

### Inline Forms (formulaires modaux)

Formulaires dans un overlay centré. 5 types de champs : Text, Password, Select,
MultiSelect, Bool.

**Règles :**
- Tab / ↑↓ pour naviguer entre les champs
- Enter pour valider / ouvrir un sous-menu
- Escape pour annuler le formulaire
- Select : flèches gauche/droite pour cycler inline, Enter pour la sous-modal si 5+ options
- MultiSelect : **Espace seul** coche et décoche l'option sous le curseur (`x` ne coche plus : c'est la touche d'archivage des listes)

### Confirmation (`Shell.Confirm`)

Toute action destructive ou irréversible demande une confirmation, par `Shell.Confirm(titre, détail, danger, onYes)`.

**Règles :**
- Boutons « Oui » / « Annuler » traduits ; pour une action dangereuse (`danger = true`), le focus **commence sur « Annuler »** : un Enter tapé par réflexe ne détruit rien
- `Échap` ou « Annuler » ferment sans agir ; le focus revient sur l'élément qui avait demandé
- `d` (supprimer) passe toujours par là, pour une colonne, un mapping, un brouillon ou un fichier ; `x` ne détruit rien
- Refuser une proposition demande un motif (consigne obligatoire pour le wiki d'équipe) : le motif validé vaut confirmation
- Hors TUI (script, CI), une confirmation sans terminal est refusée avec le code de sortie 2 sauf `--yes`

### Assistants (wizards)

Un assistant s'écrit une seule fois (`InlineWizardView`) et tourne dans la TUI ou, pour une commande de la CLI,
seul par `views.RunInlineWizardStandalone` : même moteur, même rendu (`oh init`, `oh project add`, `oh provider setup`, `oh team init`…).

**Règles :**
- Échap demande « Quitter l'assistant ? » (jamais de double Échap silencieux)
- Une étape qui échoue affiche son erreur et attend une nouvelle tentative au lieu de terminer l'assistant
- `Ctrl+S` agit comme le bouton principal de l'étape
- Ce qu'une étape imprime apparaît une fois l'assistant sorti de l'écran ; pas de notification bloquante

### Liste filtrable (`ActionList` avec `Filter`)

Liste d'éléments sur une ligne, détail dessous ou à côté, actions sur l'élément sélectionné et actions globales
(`views.ActionList`). `ActionListConfig.Filter` ajoute un filtre sur `/` ; les actions globales portent alors
sur les éléments visibles. Il n'y a plus de `FilterableList`.

**Règles :**
- La saisie filtre en temps réel ; flèches haut/bas pour naviguer dans la liste filtrée
- Enter pour ouvrir ; le nombre d'éléments (« 2/9 ») est affiché
- Les actions viennent de la convention (`keys.Create`, `keys.Delete`…), jamais d'une lettre libre qui en reprend le sens
- Une liste sans données dit pourquoi (chargement, vide, erreur) avec `SetEmpty(kind, msg)`

### SectionedList (liste par sections)

Liste avec des headers de section non sélectionnables et navigation par section.

**Règles :**
- Les headers de section sont visuellement distincts (gras, couleur différente)
- `{` et `}` pour sauter entre les sections
- j/k ou flèches pour la navigation item par item ; `Début`/`Fin` (`g`/`G`) vont au premier et au dernier champ, jamais sur un titre
- Les headers sont sautés automatiquement lors de la navigation
- Un titre de section reste visible quand la liste est remontée au début

### StepBar (barre de progression wizard)

Indicateur horizontal de progression pour les processus multi-étapes.

**Règles :**
- Étapes terminées en couleur Success avec `✔`
- Étape courante en couleur Accent (focus) avec `◆`
- Étapes futures en couleur Muted avec `○`
- Étape sautée en Muted atténué avec `–`
- Labels courts (1-2 mots par étape)
- Maximum 5-7 étapes visibles

---

## Tokens Aurum — quick reference

### Arrière-plans (3 niveaux de profondeur)

| Token | Hex | Usage |
|-------|-----|-------|
| `BgApp` | #181825 (Mantle) | Fond le plus profond — shell, barre d'état |
| `BgPanel` | #1e1e2e (Base) | Surface principale — contenu, panneaux |
| `BgElement` | #313244 (Surface0) | Éléments surélevés — cards, boutons, headers |

### Texte (3 niveaux de contraste)

| Token | Hex | Usage |
|-------|-----|-------|
| `TextPrimary` | #cdd6f4 | Titres, contenu principal |
| `TextSecondary` | #a6adc8 | Descriptions, texte secondaire |
| `TextMuted` | #7f849c | Placeholders, texte désactivé |

### Couleurs sémantiques

| Token | Hex | Couleur | Usage |
|-------|-----|---------|-------|
| `Accent` | #89b4fa | Blue | Focus structurel, sélection |
| `Action` | #fab387 | Peach | CTA, éléments actifs |
| `Success` | #a6e3a1 | Green | Confirmations, succès |
| `Warning` | #f9e2af | Yellow | Alertes non bloquantes |
| `Error` | #f38ba8 | Red | Erreurs, danger |
| `Info` | #b4befe | Lavender | En cours, informationnel |

### Bordures

| Token | Hex | Usage |
|-------|-----|-------|
| `BorderNormal` | #313244 | Bordure par défaut |
| `BorderFocus` | #89b4fa (= Accent) | Bordure de l'élément focusé |
| `BorderActive` | #fab387 (= Action) | Bordure de l'élément actif |

### Symboles : une table de sens

Un symbole veut dire **une seule chose**, dans la TUI comme dans la CLI. Il vient de `theme.Sym*`
(jamais d'un littéral dans une vue : `✓` est interdit, c'est `✔`) ; l'aide `?` en montre la légende.

| Symbole | Sens | Couleur |
|---------|------|---------|
| `✔` | réussi, valide (aussi une étape terminée) | Success |
| `✗` | échoué, invalide : une erreur ponctuelle, une saisie refusée | Error |
| `○` | désactivé, au repos (aussi une étape à venir) | Muted |
| `–` | absent, non renseigné (aussi une étape sautée) | Muted |
| `⚠` | marche, mais demande de l'attention | Warning |
| `●` | en cours d'exécution, et rien d'autre | texte |
| `◆` | l'élément courant : vue, étape, équipe | texte |
| `✎` | modifié, ni enregistré ni publié | texte |
| `★` | choix par défaut, élément épinglé | texte |
| `▸` | la ligne sous le curseur, le début d'une action ; jamais un état | texte |
| `◉` | choisi dans un choix unique (les autres : `○`) | texte |
| `☑` / `☐` | coché / non coché dans un choix multiple | texte |

- Un réglage ou un service **désactivé** est `○` gris, pas `✗` rouge : désactivé n'est pas une erreur.
- Une session qui s'est arrêtée sur un échec est `⊗`, une session en veille `◌` (états de session, table `glyph`).
- Pour une spec : nommer le symbole par son sens (« succès », « désactivé »), pas par son dessin.

### Formats de valeurs

Dates, durées, coûts, tailles, nombres de jetons et coupures de texte viennent du paquet `textfmt`, dans la langue
de l'interface : heure locale, coût au centime avec plancher « <$0.01 », tailles en Kio/Mio (FR) ou KiB/MiB (EN).
Une colonne se mesure en largeur d'affichage (`textfmt.ClipWidth`), jamais avec `len`. Une spec ne dessine pas
un format à la main.

### États d'une vue sans données

Chargement, vide et erreur s'écrivent avec `stateText` (`stateLoading`, `stateEmpty`, `stateError`) : mêmes mots et
mêmes couleurs dans toutes les vues ; une erreur est en `✗` rouge et dit quoi faire. Une vue lit ses données
**hors de la boucle d'événements** (`load`), n'y fait ni E/S ni attente, et n'applique pas une réponse périmée.

---

## Règles de layout TUI

### Espacement

- **Padding horizontal du contenu :** 2 colonnes (gauche et droite)
- **Padding vertical haut :** 1 ligne
- **Séparateurs de section :** 1 ligne vide
- **Indentation des sous-éléments :** 2 espaces

### Responsive terminal

- Détecter la taille du terminal à chaque resize
- **Seuil minimum :** 80 colonnes x 24 lignes
- En dessous : message "Terminal trop petit" avec les dimensions requises
- Adapter les layouts :
  - Large (120+ col) : sidebar + contenu + panneau info
  - Medium (80-119 col) : contenu + omnibar
  - Small (< 80 col) : contenu seul, simplifié

### Focus management

- Un seul élément focusé à la fois (visible par bordure/couleur `BorderFocus`)
- Tab / Shift+Tab pour cycler entre les zones de focus
- Le focus ne doit jamais être invisible — si aucun élément visuel n'a le focus, c'est un bug
- Restaurer le focus après la fermeture d'un overlay (revenir à l'élément qui l'a ouvert)

---

## Anti-patterns TUI

| Anti-pattern | Problème | Correction |
|--------------|----------|------------|
| **Trop de couleurs** | Interface carnaval, fatigue visuelle | Palette restreinte (Aurum : 6 sémantiques + 3 textes + 3 fonds) |
| **Scroll horizontal** | Le terminal gère mal le scroll horizontal | Revenir à la ligne, tronquer avec ellipsis, adapter au viewport |
| **Menus profonds (3+ niveaux)** | Impossible de naviguer efficacement au clavier | Navigation à plat + recherche dans l'omnibar |
| **Texte tronqué sans indication** | L'utilisateur ne sait pas qu'il manque du contenu | Ellipsis (`...`) + tooltip ou expand |
| **Manque de raccourcis clavier** | L'utilisateur expert ne peut pas accélérer | Raccourcis pour toutes les actions fréquentes, affichés dans l'aide |
| **ASCII art excessif** | Décoratif, consomme de l'espace utile | Unicode sobre pour les bordures et icônes. Pas de logo ASCII |
| **Animations lourdes** | Scintillement, consommation CPU | Animations subtiles (spinner braille, shimmer sobre) |
| **Couleur seule pour le sens** | Inaccessible en monochrome ou pour les daltoniens | Icône + texte + couleur pour chaque état |
| **Modal dans modal dans modal** | Confusion de contexte, perte de l'état | Maximum 2 niveaux d'overlay |
| **Pas de confirmation sur destructif** | Actions irréversibles sans filet | `Shell.Confirm` (focus sur « Annuler ») avant suppression/reset |
| **Lettre libre qui reprend un sens de la convention** | `a` pour ajouter, `w` pour enregistrer : l'utilisateur doit réapprendre chaque vue | `n` créer, `Ctrl+S` enregistrer ; une touche propre via `keys.Own` |
| **Symbole ou couleur en dur** | Deux écrans disent la même chose autrement, rien ne suit `NO_COLOR` | `theme.Sym*`, `theme.Paint` |
| **Format de date, de coût ou de taille écrit à la main** | Incohérences entre commandes et écrans, mauvaise langue | `textfmt` |
| **Lecture ou écriture sur la boucle d'événements** | L'écran se fige pendant un accès disque, git ou réseau | `load` pour lire, `persist` pour écrire |
| **Message de chargement ou d'erreur improvisé** | Mots et couleurs différents d'une vue à l'autre | `stateText` |

---

## Catalogue des vues OpenHub

Référence rapide des vues existantes (`cli/internal/tui/v2/views`) pour vérifier l'existant avant de spécifier :

| Vue | ID | Description |
|-----|----|-------------|
| Home | `home` | Tableau de bord principal |
| Board | `board` | Vue kanban Beads (`oh board` ouvre la TUI sur cette vue) |
| Sessions | `sessions` | Sessions de travail en cours |
| Launch | `launch.<workflow>` | Formulaire de lancement d'un workflow |
| Workflows | `workflows` | Catalogue des workflows |
| Workflow Editor | `workflow.edit.<ref>` | Édition d'un workflow (historique : `workflow.history.<ref>`, publication : `workflow.publish.<ref>`) |
| Team Mode | `team.mode` | Accueil du mode équipe |
| Team Board | `team.board` | Board d'équipe |
| Teams | `teams` | Liste des équipes |
| Team Detail | `team.detail` | Détail d'une équipe |
| Team Activity | `team.activity` | Flux d'activité |
| Team Status | `team.status` | Statut de l'équipe |
| Team Briefs | `team.briefs` | Takeover/briefs |
| Team Patterns | `team.patterns` | Patterns d'équipe |
| Team Policies | `team.policies` | Politiques |
| Team Wiki | `team.wiki` | Wiki d'équipe |
| Team Models / MCP | `team.models`, `team.mcp` | Modèles et serveurs MCP de l'équipe |
| Settings | `settings` | Paramètres |
| Secrets | `secrets` | Gestion des secrets |
| Projects | `projects.list` | Liste des projets |
| Project Config | `project.config` | Configuration projet |
| Project Mode | `project.mode` | Mode du projet |
| Project Agents / Models / MCP | `project.agents`, `project.models`, `project.mcp` | Catalogue de briques, modèles et serveurs MCP du projet |
| Models | `models` | Modèles IA |
| Provider | `provider` | Configuration provider |
| MCP | `mcp` | Serveurs MCP |
| Metrics | `metrics` | Métriques (`oh dashboard` ouvre la TUI sur cette vue) |
| Worktrees | `worktrees` | Gestion des worktrees |
| Cleanup | `gc` | Nettoyage de ce qu'oh laisse derrière lui |
| Project Wiki | `project.wiki` | Wiki du projet (`docs/wiki/`) |
| Notifications | `notifications` | Historique des notifications |
| Status | `status` | Statut système |
| Doctor | `doctor` | Diagnostic |
| Help | `help` | Aide/documentation |
