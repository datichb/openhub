---
name: documentarian-handoff-format
description: Source de vérité pour le format de retour du documentarian vers orchestrator-dev. Définit le bloc structuré unique à produire en fin de session de documentation quand invoqué depuis orchestrator-dev. Injecté dans le documentarian et dans orchestrator-dev pour garantir que producteur et consommateur partagent le même contrat.
---

# Skill — Format de handoff documentarian → orchestrator-dev

Ce skill est la **source de vérité** pour le format de retour du `documentarian` vers `orchestrator-dev`.
Il est injecté dans le `documentarian` et dans `orchestrator-dev` — producteur et consommateur partagent le même contrat.

---

## Principe fondamental — bloc unique

> **Contrat de handoff :** voir skill `shared/handoff-bloc-unique-rule` pour les règles universelles producer/consumer.

---

## Format du bloc `## Retour vers orchestrator-dev`

> **Template :** format défini dans `templates/documentarian-handoff-block.md` — charger via `read` quand tu produis ce bloc.

**Définitions du statut :**

| Statut | Condition |
|--------|-----------|
| `documenté` | Documentation produite et fichiers mis à jour sans blocage |
| `partiellement-documenté` | Documentation produite mais incomplète — certaines sections manquantes ou en attente de validation |
| `bloqué` | Impossible de produire la documentation — structure introuvable, format inconnu, confirmation en attente |

---

## Règles pour le producteur (documentarian)

- Le contenu de documentation est écrit dans les fichiers (via `write`/`edit`) — il n'est PAS reproduit dans la discussion
- **La section `### Résumé de l'entrée`** doit être suffisamment précise pour qu'`orchestrator-dev` puisse l'inclure dans le récap global sans relire le fichier
- Si statut = `bloqué` : expliquer clairement la raison du blocage dans le résumé
- **`### Questions bloquantes`** (obligatoire) — liste numérotée des questions nécessitant une réponse avant de poursuivre. Si aucune : écrire `Aucune.`. Le coordinateur **doit** résoudre ces questions avant de continuer le workflow.

---

## Règles pour le consommateur (orchestrator-dev)

### À la réception du bloc `## Retour vers orchestrator-dev` du documentarian

1. **Lire le `### Statut`** pour évaluer le résultat :
   - `documenté` → noter comme "CHANGELOG mis à jour" dans le récap global
   - `partiellement-documenté` → noter les sections manquantes comme point d'attention
   - `bloqué` → noter le blocage dans les `### Points d'attention globaux` du récap

2. **Intégrer le `### Résumé de l'entrée`** et les `### Fichiers modifiés` dans le récap global structuré.

3. **Si le bloc est absent** → demander explicitement au documentarian de le produire.

> ❌ Ne jamais ignorer un statut `bloqué` — le signaler dans le récap global.
