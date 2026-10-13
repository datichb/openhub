---
name: reviewer-payload
description: "Utiliser en fin de review : verdict de ton retour (commit, corriger, corriger-sécurité) et sections de ton rapport."
statuses: [commit, corriger, corriger-sécurité]
when: role=subagent
---

# Payload du reviewer

## Statuts (le verdict)

| Statut | Condition |
|---|---|
| `commit` | Prêt à être commité : aucun finding 🔴 ni 🟠 |
| `corriger` | Au moins un finding 🔴 ou 🟠 (dont un échec de la vérification) |
| `corriger-sécurité` | Au moins un finding 🔴 de sécurité |

## Rapport

Rapport de review complet au format de `review-protocol` (titre `## Review — <branche>` ou celui du mode), toujours produit, même sans problème. Sections en tête, dans l'ordre :

- `### Verdict` : le statut et sa justification.
- `### Corrections requises` : les findings 🔴 et 🟠 seulement, actionnables, avec `fichier:ligne` et score ; l'appelant les copie tels quels dans le ticket.
- `### Suggestions (non-bloquant)` : les findings 🟡 et 💡, jamais transmis comme corrections.
- `### Routing recommandé` : `retour-initial` (developer du même domaine) ou `domaine-security`.
- Puis le rapport : `### Walkthrough`, `### Résumé`, `### Périmètre et contexte`, `### Vérification`, `### Couverture des critères d'acceptance` (ticket avec critères), les findings par sévérité, `### ✅ Points positifs`, `### 🔍 Hors scope`, `### Enrichissements wiki proposés`.

Sous-session `[REVIEW:single]` : ta réponse est le rapport brut du mode, sans enveloppe.
