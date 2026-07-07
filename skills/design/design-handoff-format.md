---
name: design-handoff-format
description: Source de vérité pour le format de retour de l'agent designer vers l'orchestrator. Définit le bloc structuré unique à produire quand le designer termine sa spec et est invoqué depuis l'orchestrator. La spec complète est intégrée dans le bloc. Injecté dans designer et orchestrator pour garantir que le producteur et le consommateur partagent le même contrat.
---

# Skill — Format de handoff design → orchestrator

Ce skill est la **source de vérité** pour le format de retour de l'agent designer vers l'orchestrator.
Il est injecté dans `designer` et `orchestrator` — producteur et consommateur partagent le même contrat.

---

## Principe fondamental — bloc unique

### Détection du contexte d'invocation

Au démarrage, charger le skill de parcours selon le contexte :

- Si le prompt contient `[SKILL:designer/designer-subagent]` → charger le skill correspondant via l'outil `skill`
  - Mémoriser **CONTEXTE = orchestrator_feature** pour toute la session
  - Ne jamais utiliser l'outil `question` — toute interaction passe par les blocs structurés
  - En fin de session : produire **uniquement** le bloc `## Retour vers orchestrator`
  - En cas de clarification critique nécessaire en cours de session : produire `## Retour intermédiaire vers orchestrator` + `## Question pour l'orchestrator` et **terminer la session**
- Sinon (standalone ou depuis `planner`) :
  - Utiliser l'outil `question` normalement
  - Produire la spec sans le bloc `## Retour vers orchestrator`

---

Quand CONTEXTE = orchestrator_feature, ton **seul output** est le bloc `## Retour vers orchestrator` défini ci-dessous.

**Règle absolue :** aucun texte avant, après ou en dehors de ce bloc. La spec complète (user flows, wireframes textuels, tokens, composants, critères UX/UI) est **intégrée dans le bloc** (section `### Spec complète`), pas produite séparément en texte libre.

> **Autocontrôle obligatoire avant de terminer la session :**
> « Mon output contient-il du texte en dehors du bloc `## Retour vers orchestrator` ? Si oui, le supprimer et vérifier que la spec est bien dans la section `### Spec complète` du bloc. »

---

## Format du bloc `## Retour vers orchestrator`

```
---

## Retour vers orchestrator

**Agent :** designer
**Ticket :** #<ID> — <titre>

### Spec complète

<User flows intégraux avec tous les états, wireframes textuels, tokens, composants, critères d'acceptance UX/UI — JAMAIS résumée, même si longue.>

<Le contenu exact dépend du mode (ux, ui, ux+ui, recon) — voir les skills designer-protocol et designer-subagent pour le détail de ce que chaque mode produit.>

### Contraintes d'implémentation
- <contrainte 1 — ex : responsive mobile-first obligatoire, ratio de contraste WCAG AA minimum, etc.>
- <contrainte 2>
<"Aucune" si pas de contrainte spécifique identifiée>

### Points ouverts
- <question en suspens 1 — ce qui n'a pas été tranché et nécessite une décision avant ou pendant l'implémentation>
- <question en suspens 2>
<"Aucun" si tous les points ont été tranchés>

### Alternatives écartées
- `<alternative 1>` : <pourquoi écartée>
- `<alternative 2>` : <pourquoi écartée>
<"Aucune" si aucune alternative notable n'a été explorée>

### Statut
`spec-complète` | `spec-partielle` | `bloqué`
```

**Définitions du statut :**

| Statut | Condition |
|--------|-----------|
| `spec-complète` | Spec validée par l'utilisateur, tous les éléments nécessaires à l'implémentation sont présents |
| `spec-partielle` | Spec validée mais avec des points ouverts qui devront être résolus pendant l'implémentation |
| `bloqué` | Spec non finalisée — un blocage empêche de produire une spec exploitable |

---

## Règles pour le producteur (designer)

- **Produire UNIQUEMENT le bloc `## Retour vers orchestrator`** — aucun texte avant ou après
- **La spec complète est DANS le bloc** (section `### Spec complète`) — ne pas la produire séparément en texte libre
- **`### Spec complète`** ne doit JAMAIS être résumée ou abrégée, même si longue — c'est le livrable principal
- Le bloc est produit **après** la validation explicite de l'utilisateur, pas avant
- Si invoqué depuis l'orchestrator via `Task`, utiliser ce format à la place du `bd close` habituel
- Le `task_id` n'est pas requis dans ce format (contrairement au format `orchestrator-dev`) — l'orchestrator reprend naturellement après réception

> ❌ Ne jamais écrire de texte en dehors du bloc de handoff
> ❌ Ne jamais produire la spec comme texte libre avant le bloc — elle est DANS le bloc
> ❌ Ne jamais résumer la spec dans `### Spec complète` — elle doit être exhaustive et exploitable

---

## Bloc `## Retour intermédiaire vers orchestrator` (clarification en cours de session)

Produit quand une **clarification critique** est nécessaire en cours de session (CONTEXTE = orchestrator_feature uniquement) — ex : aucun design system détecté, informations utilisateur insuffisantes, décision de direction artistique bloquante.

> ⚠️ Réserver aux vrais blockers. Formuler une hypothèse documentée et continuer si possible.

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** designer
**Mode :** <recon|ux|ui|ux+ui>
**Phase :** Clarification en cours de session
**task_id :** <sessionID courant>

### Ce qui a été exploré jusqu'ici
- <observation 1>
- <observation 2>

### Problème détecté
<Description précise de la clarification nécessaire>

### Impact
<Conséquence sur la spec si on continue sans cette information>

### Hypothèse possible
<Formulation de l'hypothèse si l'utilisateur préfère continuer>
```

---

## Bloc `## Question pour l'orchestrator` (clarification en cours de session)

Accompagne toujours un `## Retour intermédiaire vers orchestrator`.

```markdown
## Question pour l'orchestrator

**Phase :** Clarification design
**task_id :** <sessionID courant>

**Contexte :** <Description du problème et de son impact>

**Question :** <Question précise>

**Options :**
- `<label-a>` — <description>
- `<label-b>` — <description>

**Instruction de reprise :** "Réponse clarification design : [option]. [Information si applicable]. Reprendre la production de la spec."
```

---

## Règles pour le consommateur (orchestrator)

**Spécificités design à vérifier :**

- **Champs obligatoires** : `Spec complète`, `Contraintes d'implémentation`, `Points ouverts`, `Statut`. Si l'un est absent ou vide sans mention explicite (`"Aucun"` / `"Aucune"`) → demander à l'agent design de compléter avant de continuer.
- **Retranscription** : afficher les champs du bloc de manière formatée dans la discussion (voir skill `retranscription-coordinateur`). La `### Spec complète` est affichée intégralement.
- **Délégation** : intégrer `### Contraintes d'implémentation` dans le prompt de délégation à `orchestrator-dev`.
- **CP-spec** : signaler `### Points ouverts` à l'utilisateur pour décision avant implémentation.
- **Statut** : `spec-complète` ou `spec-partielle` → CP-spec normal · `bloqué` → ne pas router vers `orchestrator-dev`.
