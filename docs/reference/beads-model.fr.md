# Référence du data model Beads

Document canonique décrivant le modèle de données utilisé par `bd` (Beads CLI)
dans le contexte du hub. Toute skill, agent ou script du hub doit se conformer
à ce modèle.

---

## Statuts (6)

Un ticket passe par un sous-ensemble de ces statuts au cours de son cycle de vie.

| Statut | Terminal | Description | Commande `bd` |
|--------|----------|-------------|---------------|
| `open` | non | Créé, pas encore pris en charge | État par défaut à la création |
| `in_progress` | non | En cours d'implémentation | `bd update <ID> --claim` (atomique : assigne + passe en `in_progress`) |
| `review` | non | Implémentation terminée, en attente de validation par le reviewer humain. Le reviewer accepte (clôture) ou renvoie en `in_progress` avec ses retours. | `bd update <ID> -s review` |
| `blocked` | non | Bloqué par une dépendance ou un facteur externe | `bd update <ID> -s blocked` |
| `cancelled` | **oui** | Abandonné — ne sera pas implémenté | `bd update <ID> -s cancelled` |
| `closed` | **oui** | Terminé et validé | `bd close <ID>` |

### Transitions autorisées

```
open ──────────→ in_progress ──→ review ──→ closed
  │                  │              │
  │                  ↓              ↓
  │               blocked      in_progress  (rejet → retour en dev)
  │                  │
  │                  ↓
  │              in_progress  (déblocage)
  │
  ↓
cancelled
```

**Règles :**

- **Pas de réouverture.** Un ticket `closed` ou `cancelled` n'est jamais rouvert.
  Si du travail supplémentaire est nécessaire, créer un nouveau ticket.
- **`cancelled` n'utilise pas `bd close`** — on utilise `bd update <ID> -s cancelled`.
- **`review`** est un statut custom accepté nativement par `bd`.
  Un ticket passe en `review` quand le développeur considère son implémentation terminée.
  Le **reviewer humain** (ou agent reviewer) décide ensuite :
  - **Accepté** → `bd close <ID> --reason "..."` — le ticket passe en `closed`
  - **Rejeté** → laisse ses retours via `bd comments add <ID> "Retours : ..."`, puis
    `bd update <ID> -s in_progress` — le ticket revient au développeur pour un cycle de correction
- **`blocked`** peut survenir depuis `in_progress` uniquement.
  Le déblocage repasse en `in_progress`.

---

## Types (5)

Chaque ticket possède exactement un type, défini à la création.

| Type | Flag `bd create` | Description |
|------|------------------|-------------|
| `epic` | `-t epic` | Conteneur de tickets — ne porte pas d'implémentation directe |
| `feature` | `-t feature` | Nouvelle fonctionnalité |
| `task` | `-t task` | Tâche technique (refactoring, migration, configuration) |
| `bug` | `-t bug` | Correction de bug |
| `chore` | `-t chore` | Maintenance, CI/CD, documentation, nettoyage |

> **`decision` n'est pas un type.** Les décisions architecturales (ADR) sont
> portées par un ticket `-t task` avec le label approprié si besoin.

---

## Priorités (4)

Échelle P0–P3. Le format `bd` accepte `-p <N>` avec N = 0 à 3.

| Priorité | Flag | Sémantique |
|----------|------|------------|
| **P0** | `-p 0` | Critique — bloquant pour la production ou pour tous les autres tickets |
| **P1** | `-p 1` | Haute — chemin critique de la feature, valeur métier principale |
| **P2** | `-p 2` | Normale (défaut) — enrichissement fonctionnel, confort utilisateur |
| **P3** | `-p 3` | Basse — nice-to-have, backlog, améliorations optionnelles |

> **`--priority high` / `--priority medium` ne sont pas des syntaxes valides.**
> Toujours utiliser la forme numérique : `-p 0`, `-p 1`, `-p 2`, `-p 3`.

> **P4 n'existe pas** dans ce modèle. Les éléments backlog lointain restent en P3
> ou ne sont tout simplement pas créés.

---

## Labels système (5)

Labels réservés par le hub. Ils ne doivent pas être détournés pour un autre usage.

| Label | Posé par | Description |
|-------|----------|-------------|
| `ai-delegated` | Humain uniquement | Marque un ticket comme délégué à un agent IA. L'agent ne pose **jamais** ce label lui-même sauf accord explicite. |
| `needs-decision` | Agent ou humain | Le ticket est bloqué par une décision humaine (choix technique, arbitrage métier). |
| `needs-clarification` | Agent ou humain | Le ticket manque d'information — la description ou les critères d'acceptance sont insuffisants. |
| `from-diagnostic` | Agent debugger | Le ticket a été créé suite à un diagnostic de bug (rapport du debugger). |
| `split-from-<ID>` | Agent planner | Le ticket résulte de la scission d'un ticket trop gros. `<ID>` est l'identifiant du ticket d'origine. |

### Commandes labels

```bash
# Créer un label au niveau projet (l'enregistrer pour utilisation dans le projet)
bd label create <label>

# Ajouter un label à un ticket
bd label add <ID> <label>
# ou
bd update <ID> --add-label <label>

# Retirer un label d'un ticket
bd update <ID> --remove-label <label>

# Lister les labels disponibles dans le projet
bd label list-all
```

> **Import automatique à l'init** — lors de `oc beads init`, si un tracker (GitLab ou Jira) est déjà configuré, les labels du tracker distant sont automatiquement récupérés et enregistrés dans Beads via `bd label create`. Les labels définis dans `projects.md` sont toujours enregistrés en premier ; les labels distants sont fusionnés par-dessus (union). `projects.md` n'est jamais modifié automatiquement.

---

## Champs d'un ticket (10)

| Champ | Flag création / mise à jour | Description |
|-------|----------------------------|-------------|
| `title` | Positional arg de `bd create` | Titre court et actionnable |
| `description` | `--description` | Description détaillée en langage naturel |
| `acceptance` | `--acceptance` | Critères d'acceptance observables et vérifiables |
| `notes` | `--notes` | Contexte technique, risques, points d'attention |
| `design` | `--design` | Notes de design (maquettes, specs UI/UX) |
| `estimate` | `--estimate <minutes>` | Estimation en minutes (60 = 1h, 480 = 1 jour) |
| `external-ref` | `--external-ref <ref>` | Référence tracker externe (ex : `jira-PROJ-42`, `gitlab-17`) |
| `assignee` | `-a <nom>` ou `--claim` | Responsable du ticket. `--claim` est atomique (assigne + `in_progress`). |
| `close-reason` | `--reason "..."` sur `bd close` | Raison de clôture — commit, PR, explication |
| `labels` | `--add-label` / `--remove-label` / `bd label add` | Labels attachés au ticket |

---

## Relations entre tickets (5 types)

| Relation | Commande | Effet |
|----------|----------|-------|
| **Parent / enfant** | `bd create "..." --parent <EPIC_ID>` | Hiérarchie epic → tickets. `bd children <ID>` pour lister. |
| **Dépendance** | `bd dep add <ID> <DEP_ID>` | `<ID>` est bloqué tant que `<DEP_ID>` n'est pas clos. `bd ready` respecte ces blocages. |
| **Duplication** | `bd duplicate <ID> --of <CANONICAL>` | Marque `<ID>` comme doublon de `<CANONICAL>` (auto-ferme `<ID>`). |
| **Supersession** | `bd supersede <ID> --with <NEW>` | `<ID>` est remplacé par `<NEW>` (auto-ferme `<ID>`). |
| **Relation libre** | `bd dep relate <ID> <OTHER>` | Lien informatif sans blocage. `bd dep unrelate` pour retirer. |

### Commandes de dépendance

```bash
# Ajouter une dépendance
bd dep add <ID> <DEP_ID>

# Retirer une dépendance
bd dep remove <ID> <DEP_ID>

# Lister les dépendances d'un ticket
bd dep list <ID>

# Arbre complet des dépendances
bd dep tree

# Détecter les cycles
bd dep cycles
```

---

## Tickets prêts — `bd ready`

`bd ready` retourne les tickets **non bloqués** (toutes les dépendances sont closes)
et **non terminaux** (ni `closed`, ni `cancelled`).

```bash
# Tickets prêts à travailler
bd ready --json

# Tickets prêts avec un label spécifique
bd ready --label ai-delegated --json
```

> **`bd ready` est la commande recommandée.**
> Elle applique une sémantique blocker-aware plus complète que le filtre `--ready` de `bd list`.

---

## Commentaires

```bash
# Ajouter un commentaire à un ticket
bd comments add <ID> "Texte du commentaire"
```

Les commentaires servent à tracer les décisions, blocages et échanges sans modifier
la description ou les notes du ticket.

---

## Cycle de vie complet — workflow type

```
 ┌─────────────────────────────────────────────────────────────┐
 │  PLANIFICATION (agent planner / humain)                     │
 │                                                             │
 │  1. bd create "Titre" -t feature -p 1 --parent $EPIC --json│
 │  2. bd update $ID --description "..." --acceptance "..."    │
 │  3. bd label add $ID ai-delegated  (humain uniquement)      │
 └──────────────────────────┬──────────────────────────────────┘
                            ↓
 ┌─────────────────────────────────────────────────────────────┐
 │  EXÉCUTION (agent developer)                                │
 │                                                             │
 │  4. bd ready --label ai-delegated --json                    │
 │  5. bd show $ID                                             │
 │  6. bd update $ID --claim                  → in_progress    │
 │  7. [implémenter, tester, committer]                        │
 │  8. bd update $ID -s review                → review         │
 └──────────────────────────┬──────────────────────────────────┘
                            ↓
 ┌─────────────────────────────────────────────────────────────┐
  │  REVIEW (agent reviewer / humain)                           │
  │                                                             │
  │  9a. Accepté  → bd close $ID --reason "..." → closed        │
  │  9b. Rejeté   → bd comments add $ID "Retours : ..."         │
  │                 bd update $ID -s in_progress                │
  │                 → retour étape 7 (cycle de correction)      │
 └──────────────────────────┬──────────────────────────────────┘
                            ↓
 ┌─────────────────────────────────────────────────────────────┐
 │  BLOCAGE (si dépendance ou décision nécessaire)             │
 │                                                             │
 │  bd update $ID -s blocked                                   │
 │  bd comments add $ID "Bloqué par : <raison>"                │
 │  bd update $ID --add-label needs-decision  (si applicable)  │
 │  ... résolution ...                                         │
 │  bd update $ID -s in_progress              → retour en dev  │
 └─────────────────────────────────────────────────────────────┘

 ┌─────────────────────────────────────────────────────────────┐
 │  ANNULATION (humain uniquement)                             │
 │                                                             │
 │  bd update $ID -s cancelled                                 │
 │  bd comments add $ID "Raison : ..."                         │
 └─────────────────────────────────────────────────────────────┘
```

---

## Synchronisation tracker externe

Le hub supporte deux trackers : **Jira** et **GitLab**.

| Commande | Description |
|----------|-------------|
| `oc beads tracker setup <PROJECT_ID>` | Configuration interactive des credentials |
| `oc beads tracker set-sync-mode <PROJECT_ID> [mode]` | Définir la direction de sync par défaut (`bidirectional` \| `pull-only` \| `push-only`) |
| `oc beads sync <PROJECT_ID>` | Synchronisation selon le `Sync mode` configuré (défaut : bidirectional) |
| `oc beads sync <PROJECT_ID> pull` | Import seul depuis le tracker (surcharge le `Sync mode`) |
| `oc beads sync <PROJECT_ID> push` | Export seul vers le tracker (surcharge le `Sync mode`) |
| `oc beads sync <PROJECT_ID> --dry-run` | Simulation sans écriture |
| `oc beads tracker status <PROJECT_ID>` | État de la connexion au tracker |

#### Exclusion locale de `.beads/`

`oc beads init` ajoute automatiquement `.beads/` au fichier `.git/info/exclude` du projet cible.
Ce fichier est local à la machine et non versionné — les credentials tracker (token GitLab, token Jira) stockés par `bd config set` ne sont jamais exposés dans le dépôt partagé.

> Ce comportement est identique à l'exclusion de `opencode.json` et `.opencode/` appliquée par `oc init` / `oc deploy`.

#### Configuration GitLab — `gitlab.project_id`

Lors du `tracker setup`, le champ **ID ou chemin du projet GitLab** accepte trois formats :

| Format | Exemple | Comportement |
|--------|---------|--------------|
| ID numérique | `12345` | Utilisé tel quel |
| Chemin namespace/projet | `mon-groupe/mon-projet` | Utilisé tel quel |
| URL complète | `https://gitlab.com/mon-groupe/mon-projet` | Chemin extrait automatiquement avec un avertissement |

> **Conseil** : préférer l'ID numérique ou le chemin `namespace/projet`.
> L'ID numérique est visible dans GitLab sous **Settings → General** (en haut de page).

Après le setup, la connexion est testée automatiquement via `bd gitlab status`.
En cas d'échec, les valeurs configurées sont affichées pour faciliter le diagnostic.

> **Champ `Sync mode`** — stocké dans `projects.md` sous `- Sync mode : <bidirectional|pull-only|push-only>`.
> Valeur par défaut si absent : `bidirectional`. Une sous-commande CLI (`pull`, `push`) prend toujours le dessus sur le mode configuré.

### Références externes

```bash
# À la création
bd create "Titre" --external-ref jira-PROJ-42 --json

# Sur un ticket existant
bd update <ID> --external-ref gitlab-17
```

Convention de nommage :
- Jira : `jira-<PROJET>-<NUMERO>` (ex : `jira-MYAPP-42`)
- GitLab : `gitlab-<NUMERO>` (ex : `gitlab-17`)

---

## Résumé des commandes `bd` autorisées

### Lecture

| Commande | Description |
|----------|-------------|
| `bd list -s <status> --json` | Lister par statut |
| `bd list -s <status> --label <label> --json` | Lister par statut + label |
| `bd ready --json` | Tickets prêts (blocker-aware) |
| `bd ready --label <label> --json` | Tickets prêts avec label |
| `bd show <ID>` | Détail complet d'un ticket |
| `bd children <EPIC_ID>` | Tickets enfants d'un epic |
| `bd search <query>` | Recherche textuelle |
| `bd count` | Nombre de tickets |
| `bd label list-all` | Labels disponibles |
| `bd dep list <ID>` | Dépendances d'un ticket |
| `bd dep tree` | Arbre complet |
| `bd dep cycles` | Détection de cycles |

### Écriture

| Commande | Description |
|----------|-------------|
| `bd create "Titre" -t <type> -p <N> [options] --json` | Créer un ticket |
| `bd update <ID> --claim` | Clamer (assigne + `in_progress`) |
| `bd update <ID> -s <status>` | Changer le statut |
| `bd update <ID> --description / --acceptance / --notes / --design` | Mettre à jour les champs |
| `bd update <ID> -a <assignee>` | Assigner |
| `bd update <ID> --add-label <label>` | Ajouter un label |
| `bd update <ID> --remove-label <label>` | Retirer un label |
| `bd label add <ID> <label>` | Ajouter un label (syntaxe alternative) |
| `bd close <ID> [--reason "..."] [--suggest-next]` | Clore un ticket |
| `bd dep add <ID> <DEP_ID>` | Ajouter une dépendance |
| `bd dep remove <ID> <DEP_ID>` | Retirer une dépendance |
| `bd dep relate <ID> <OTHER>` | Relation libre (sans blocage) |
| `bd dep unrelate <ID> <OTHER>` | Retirer une relation libre |
| `bd duplicate <ID> --of <CANONICAL>` | Marquer comme doublon |
| `bd supersede <ID> --with <NEW>` | Marquer comme remplacé |
| `bd comments add <ID> "..."` | Ajouter un commentaire |
