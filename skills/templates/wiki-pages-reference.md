# Template — Pages de référence du wiki (`glossary.md`, `specs/`, `decisions/`, `postmortems/`)

Un seul référentiel : ces pages vivent dans `docs/wiki/`, pas à côté. Frontmatter obligatoire partout ; tags de confiance obligatoires dans le glossaire, facultatifs dans `specs/`, `decisions/` et `postmortems/` (décisions et faits datés).

## Format canonique — `docs/wiki/glossary.md`

```markdown
---
updated: YYYY-MM-DD
confidence: confirmed | mixed | uncertain
agents: [<agents>]
---

# Glossaire — [Nom du projet]

## Termes métier
- **<Terme>** : <définition précise dans ce projet> (<module ou contexte>)
  — `CONFIRMÉ` · <agent> · <date> · <fichier:ligne>

## Termes techniques du projet
- **<Terme>** : <ce qu'il signifie dans CE projet>
  — `DÉDUIT` · <agent> · <date>
```

Le glossaire ne compte pas pour les god nodes (il nomme tous les concepts) ; ses termes en sont des candidats.

## Format canonique — `docs/wiki/specs/<sujet>.md`

```markdown
---
updated: YYYY-MM-DD
confidence: mixed
agents: [<agents>]
---

# Spec — <Sujet>

## Objectif
## Périmètre (inclus, exclus)
## Critères d'acceptance
## Points de test
## Décisions et questions ouvertes
```

## Format canonique — `docs/wiki/decisions/`

- ADR : `decisions/NNN-<titre>.md`, au format des ADR du projet (s'il n'en a pas : contexte, décision, statut, conséquences).
- `decisions/decisions-rejetees.md` : une entrée `### <Concept rejeté>` par rejet (date, raison, tickets liés, statut `ACTIVE | EXPIRED | SUPERSEDED`, contexte de réévaluation).

## Format canonique — `docs/wiki/postmortems/YYYY-MM-DD-<incident>.md`

```markdown
---
updated: YYYY-MM-DD
confidence: confirmed
agents: [<agents>]
---

# Postmortem — <Incident> (YYYY-MM-DD)

## Résumé (impact, durée)
## Chronologie
## Cause racine
## Actions (avec tickets)
```
