---
id: prototype-protocol
bucket: B
agent: designer
---

# Prototype Protocol — Artefact rapide pour répondre à une question

## Quand ce skill est chargé

Ce skill est injecté dans le prompt du designer par l'orchestrateur (ou le planner via l'orchestrateur) quand une **question spécifique** nécessite un artefact visuel pour être résolue. Il est toujours chargé **en complément** d'un mode existant (ux, ui, ou ux+ui) — jamais seul.

Cas typiques :
- Le planner en Phase 1.5 détecte un signal design ambigu et propose une option "prototype rapide"
- L'orchestrateur en cours d'implémentation détecte une ambiguïté visuelle non résoluble par texte
- L'utilisateur demande explicitement un prototype sur un point précis

## Ce que ce skill change

Quand `prototype-protocol` est chargé conjointement avec un skill de mode (ux-protocol, ui-protocol), les **overrides suivants** s'appliquent :

| Règle du mode normal | Override prototype |
|---------------------|-------------------|
| Minimum 2 questions de contexte | **Skip** — le contexte est fourni dans la question |
| Review bornée 3 rounds | **Skip** — pas de review, c'est un artefact jetable |
| Spec complète obligatoire | **Override** — artefact minimal ciblé sur 1 question |
| Validation explicite requise | **Override** — l'artefact est la réponse, pas un livrable |
| Reuse > Adapt > Create | **Conservé** — vérifier le design system avant de proposer |

## Contrainte fondamentale

Le designer en mode prototype **ne produit PAS de code**. L'artefact est un artefact de spec rapide :
- Wireframe ASCII ou description structurée de layout
- Schéma de flow (étapes numérotées avec transitions)
- Description de comportement d'interaction (état A → action → état B)
- Tableau de variantes (si la question porte sur un choix)

❌ Pas de HTML, pas de code, pas de branche git. Le designer reste un agent de spec.

## Workflow

1. **Lire la question** dans le champ `[QUESTION: ...]` du prompt d'invocation
2. **Identifier le type d'artefact** nécessaire (wireframe, flow, interaction, variantes)
3. **Produire l'artefact minimal** qui répond à la question — rien de plus
4. **Formuler la réponse** en 1-3 phrases basées sur l'artefact
5. **Recommander la suite** : spec complète (ux/ui/ux+ui), aucun besoin, ou clarification supplémentaire

## Format de sortie

### Mode standalone

Afficher l'artefact directement dans la discussion, puis :

```
question({
  questions: [{
    header: "Suite après prototype",
    question: "[Designer — Prototype | Question : <question>]\nVoici l'artefact (ci-dessus). La question est-elle résolue ?",
    options: [
      { label: "Résolu — continuer", description: "La question est résolue, pas besoin de spec supplémentaire" },
      { label: "Spec complète nécessaire", description: "L'artefact révèle qu'une spec UX/UI complète est nécessaire" },
      { label: "Autre question", description: "L'artefact soulève une nouvelle question" }
    ]
  }]
})
```

### Mode subagent

Produire le bloc handoff prototype (voir `design-handoff-format`) et TERMINER LA SESSION.

## Compatibilité

- **Compatible avec** : `Mode: ux`, `Mode: ui`, `Mode: ux+ui`
- **Incompatible avec** : `Mode: recon` (la reconnaissance Figma n'a pas de question à résoudre par prototype)
- Si invoqué avec `Mode: recon` + `prototype-protocol` → ignorer le prototype, exécuter le recon normalement
