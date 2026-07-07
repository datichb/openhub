> 🇬🇧 [Read in English](external-agents.en.md)

# Guide — Agents externes par projet

Ce guide explique comment intégrer des agents existants dans un projet cible avec le hub openhub, sans les écraser ni les migrer.

---

## Prérequis

- Hub initialisé (`oh install`)
- Projet enregistré (`oh init PROJECT_ID`)
- Des agents `.md` déjà présents dans `<projet>/.opencode/agents/` (hors agents générés par le hub)

---

## Comment ça marche

Lors d'un `oh deploy PROJECT_ID`, le hub détecte automatiquement les agents dans `.opencode/agents/` qui ne lui appartiennent pas. Il vous propose alors de choisir comment les intégrer :

```
── Agents existants détectés dans le projet

  ● Agent trouvé : planner  (.opencode/agents/my-planner.md)
    → Similaire à l'agent hub : planner

    Que voulez-vous faire ?
      [s] Substituer notre agent 'planner' par cet agent
      [c] Ajouter en complément (les deux coexistent)
      [i] Ignorer (ne pas intégrer)

    Votre choix [s/c/i] :
```

Votre choix est **persisté** dans `projects.md` sous le champ `External agents`. Les deploys suivants n'interrompent plus pour les agents déjà configurés.

---

## Les deux modes d'intégration

| Mode | Ce qui se passe | Quand l'utiliser |
|------|----------------|-----------------|
| **Substitution** | L'agent projet **remplace** l'agent hub correspondant dans `.opencode/agents/` | L'agent projet est plus adapté au domaine du projet que le hub |
| **Complément** | L'agent projet **s'ajoute** en plus des agents hub | L'agent projet couvre un besoin non couvert par le hub |

---

## Déclenchement

### Automatique au deploy

```bash
oh deploy MON-PROJET
```

Si des agents non-hub sont trouvés dans `.opencode/agents/` et pas encore configurés → prompt interactif.

> **En CI/CD** (`OH_NON_INTERACTIVE=1`) : la découverte est silencieusement ignorée. Le deploy continue normalement.

### À la demande

```bash
oh agent discover MON-PROJET
```

Lance uniquement la découverte sans déclencher de deploy. Utile pour configurer avant le premier deploy.

---

## Exemple pas à pas

### Situation initiale

Votre projet `MON-APP` a déjà ce fichier dans son dépôt :

```
mon-app/
└── .opencode/
    └── agents/
        ├── planner.md          ← agent manuel, spécifique au domaine
        └── feature-reviewer.md ← agent custom sans équivalent hub
```

### Étape 1 — Premier deploy

```bash
oh deploy MON-APP
```

Le hub détecte `planner.md` (similaire à l'agent hub `planner`) et `feature-reviewer.md` (aucun équivalent) :

```
  ● Agent trouvé : planner  (.opencode/agents/planner.md)
    → Similaire à l'agent hub : planner

    [s] Substituer / [c] Complément / [i] Ignorer : s
    → Substitution : 'planner' remplacera 'planner' pour ce projet

  ● Agent trouvé : feature-reviewer  (.opencode/agents/feature-reviewer.md)
    → Aucun agent hub équivalent trouvé

    [c] Complément / [i] Ignorer : c
    → Complément : 'feature-reviewer' sera ajouté aux agents hub
```

### Étape 2 — Persistance automatique

`projects.md` est mis à jour :

```markdown
## MON-APP
- Nom : Mon Application
- Stack : TypeScript · Vue 3
- Agents : all
- External agents : .opencode/agents/planner.md:substitute:planner|.opencode/agents/feature-reviewer.md:complement
```

### Étape 3 — Résultat dans le projet

Après le deploy, `.opencode/agents/` contient :

```
mon-app/.opencode/agents/
├── planner.md          ← votre agent (copie depuis votre source)
├── feature-reviewer.md ← votre agent (copie depuis votre source)
├── orchestrator.md     ← hub
├── developer-frontend.md ← hub
├── reviewer.md         ← hub
└── ...                 ← autres agents hub
```

> L'agent `planner` du hub est **remplacé** par le vôtre. Tous les autres agents hub sont déployés normalement.

### Étape 4 — Deploys suivants

```bash
oh deploy MON-APP   # plus de prompt, les choix sont mémorisés
```

---

## Format du champ `External agents`

```markdown
- External agents : <entrée1>|<entrée2>|...
```

Chaque entrée suit ce format :

| Format | Signification |
|--------|---------------|
| `chemin:substitute:hub-id` | L'agent du projet remplace l'agent hub `hub-id` |
| `chemin:complement` | L'agent du projet s'ajoute aux agents hub |

Les chemins peuvent être :
- **Relatifs** au `project_path` (ex: `.opencode/agents/planner.md`)
- **Absolus** (ex: `/home/user/agents/planner.md`)

---

## Édition manuelle

Vous pouvez modifier directement `projects.md` sans passer par le prompt interactif :

```markdown
## MON-APP
- External agents : .opencode/agents/planner.md:substitute:planner
```

Pour **retirer** une intégration, supprimez l'entrée correspondante ou videz le champ :

```markdown
## MON-APP
- External agents :
```

> Le champ vide est équivalent à l'absence du champ.

---

## Résolution de similarité

Le hub reconnaît automatiquement les noms courants grâce à `config/agent-aliases.json` :

| Nom dans le projet | Agent hub reconnu |
|--------------------|-------------------|
| `planner`, `plan`, `planning`, `project-planner` | `planner` |
| `orchestrator`, `coordinator`, `router` | `orchestrator` |
| `frontend`, `front`, `ui-dev` | `developer-frontend` |
| `backend`, `back`, `server` | `developer-backend` |
| `reviewer`, `review`, `code-reviewer` | `reviewer` |
| `docs`, `documentation`, `writer` | `documentarian` |
| `devops`, `ops`, `ci-cd` | `developer-devops` |
| `debug`, `debugger`, `bug-hunter` | `debugger` |

Pour voir la liste complète : `cat config/agent-aliases.json`

---

## Comportement de l'agent substitué

Un agent de substitution est déployé **tel quel** depuis son fichier source, en passant par le pipeline de build normal. Cela signifie :

- Si le frontmatter déclare des `skills:` → ils sont injectés (Bucket A)
- Si le frontmatter déclare des `native_skills:` → ils sont déployés (Bucket B)
- Si le frontmatter est vide de skills → l'agent est déployé sans injection de skills hub
- Le `mode:` du frontmatter est respecté (ou l'override projet si configuré)

> Les **stack skills** (ADR-008) ne s'appliquent pas aux agents de substitution — ils n'en bénéficient que si déclarés dans leur propre frontmatter.

---

## Troubleshooting

| Symptôme | Cause probable | Solution |
|----------|---------------|----------|
| L'agent n'est pas détecté au deploy | Il a le marker `<!-- generated by openhub` | C'est un agent hub — il sera regénéré normalement |
| Le prompt n'apparaît pas | `OH_NON_INTERACTIVE=1` ou pas de TTY | Lancer `oh agent discover MON-PROJET` manuellement |
| "Fichier substitut introuvable" au deploy | Le chemin relatif est incorrect | Vérifier que le chemin est bien relatif à `project_path` |
| "Agent complément 'X' en doublon" | Un agent hub a le même ID | Changer l'ID de votre agent ou utiliser la substitution |
| Le choix n'est pas mémorisé | `projects.md` en lecture seule ou erreur Perl | Vérifier les permissions sur `projects.md` |

---

## Voir aussi

- [ADR-011 — Agents externes par projet](../architecture/adr/011-external-agents-per-project.fr.md)
- [Référence CLI — `oh agent discover`](../reference/cli.fr.md#oc-agent)
- [Guide d'authoring](./authoring.fr.md)
