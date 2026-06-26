---
name: debugger-standalone
description: Parcours d'exécution du debugger en mode direct (invoqué par l'utilisateur sans passer par un agent orchestrateur) — utilisation de l'outil question pour les validations, récap obligatoire avant chaque question, rapport de diagnostic complet sans bloc Retour vers orchestrator.
---

# Skill — Parcours Debugger Standalone

> Ce skill est chargé quand le debugger est invoqué directement par l'utilisateur (pas via `task` depuis un agent orchestrateur).

## Principe fondamental

En mode standalone, le contenu est directement visible par l'utilisateur dans la discussion. Les validations se font via l'outil `question` à chaque fin de phase.

**Confirmer le contexte au démarrage :**
> `[debugger] Mode standalone actif — je poserai une question de validation à chaque fin de phase via l'outil question.`

---

## Règles de communication — ABSOLUES

### À CHAQUE fin de phase :

1. **Afficher le récap complet de la phase en texte** dans la discussion
2. **PUIS appeler l'outil `question`** pour la validation

> ❌ Ne jamais appeler `question` sans avoir d'abord affiché le récap en texte
> ✅ Récap en texte → puis question

### ✅ Checklist visuelle — AVANT CHAQUE APPEL À `question`

| Vérification | Fait ? |
|--------------|--------|
| ✅ J'ai affiché le récap complet de la phase actuelle en texte dans la discussion | ⬜ |
| ✅ Le récap contient toutes les observations, découvertes et décisions de cette phase | ⬜ |
| ✅ Le récap n'est PAS résumé — il est complet et détaillé | ⬜ |
| ✅ Le récap est affiché AVANT cet appel à `question`, PAS après | ⬜ |
| ✅ Le récap n'est PAS inclus dans le champ `question` de l'outil | ⬜ |

**Si une seule case est ⬜ → ARRÊTER et produire le récap MAINTENANT.**

---

## Format des questions de validation

Le champ `question` de l'outil contient **uniquement la question**, jamais le récap.

```
question({
  questions: [{
    header: "<titre court>",
    question: "[Debugger — Phase X | Bug : <titre>]\n<question de validation>",
    options: [...]
  }]
})
```

---

## Format de retour final (Phase 5)

Produire uniquement :

1. **Le rapport de diagnostic complet** (texte narratif — symptôme, cause racine, fichiers impliqués, hypothèses, ticket suggéré)

**Ne pas produire** le bloc `## Retour vers orchestrator` — ce bloc est réservé au mode sous-agent.

---

## Autocontrôle avant chaque `question`

> « Ai-je produit le récap (ou le contexte de pause) en texte clair dans la discussion avant cet appel ? »
> - **Non** → produire le récap maintenant, puis appeler `question`
> - **Oui** → appeler `question`
