---
name: reviewer-handoff-format
description: Source de vérité pour le format de retour du reviewer vers orchestrator-dev. Définit le bloc structuré à produire en fin de review quand invoqué depuis orchestrator-dev. Injecté dans reviewer et orchestrator-dev pour garantir que producteur et consommateur partagent le même contrat.
---

# Skill — Format de handoff reviewer → orchestrator-dev

Ce skill est la **source de vérité** pour le format de retour du `reviewer` vers `orchestrator-dev`.
Il est injecté dans le `reviewer` et dans `orchestrator-dev` — producteur et consommateur partagent le même contrat.

---

## Quand produire ce bloc

Quand tu es invoqué depuis `orchestrator-dev` (via l'outil `Task`),
tu **dois** produire dans cet ordre :

1. **Le rapport de review complet** au format défini dans le skill `review-protocol` — toutes les sections pertinentes, y compris `✅ Points positifs` si applicable. **Ce rapport doit être produit même si la review ne trouve aucun problème** (review propre).
2. **Le bloc `## Retour vers orchestrator-dev`** défini ci-dessous — résumé actionnable structuré du rapport.

Ce bloc vient **après** ton rapport de review — il en est le résumé actionnable structuré. Il ne le remplace pas.

> **Autocontrôle obligatoire avant de produire ce bloc :**
> « Ai-je produit le rapport de review complet (format `review-protocol`) avant ce bloc ? Si non, le produire d'abord. »

---

## Format du bloc `## Retour vers orchestrator-dev`

```
---

## Retour vers orchestrator-dev

**Agent :** reviewer
**Ticket :** #<ID> — <titre>
**Branche :** <nom de la branche reviewée>

### Verdict
`commit` | `corriger` | `corriger-sécurité`

### Synthèse des problèmes

| Sévérité | Nombre | Résumé |
|----------|--------|--------|
| 🔴 Critique | X | <description courte des critiques, séparées par `;` si plusieurs> |
| 🟠 Majeur | X | <description courte des majeurs> |
| 🟡 Mineur | X | <description courte> |
| 💡 Suggestion | X | — |

### Corrections requises
<Liste des corrections à apporter — utilisée par orchestrator-dev pour remplir le commentaire Beads>
<Chaque correction sur une ligne, format : "[SÉVÉRITÉ] <fichier:ligne> — <action concrète attendue>">
<Vide si verdict = `commit`>

### Routing recommandé
`retour-initial` | `developer-security`
<`developer-security` uniquement si au moins un 🔴 Critique de nature sécurité (OWASP, secret, injection, CORS, auth)>

### Statut
`approuvé` | `corrections-requises` | `bloquant-sécurité`
```

**Définitions du verdict :**

| Verdict | Condition |
|---------|-----------|
| `commit` | Aucun Critique, aucun Majeur — le code peut être commité |
| `corriger` | Au moins un Critique ou Majeur non lié à la sécurité |
| `corriger-sécurité` | Au moins un Critique de nature sécurité — routing vers `developer-security` recommandé |

**Définitions du statut :**

| Statut | Condition |
|--------|-----------|
| `approuvé` | Verdict `commit` — pas de problème bloquant |
| `corrections-requises` | Verdict `corriger` — problèmes à résoudre avant commit |
| `bloquant-sécurité` | Verdict `corriger-sécurité` — faille de sécurité critique détectée |

---

## Règles pour le producteur (reviewer)

- **Toujours produire le rapport de review complet** (format `review-protocol`) avant ce bloc — même si la review est courte ou ne contient aucun problème. Le rapport est obligatoire dans tous les cas.
- **Toujours produire ce bloc** à la suite du rapport, même si le rapport est minimal
- **La section `### Corrections requises`** doit être suffisamment précise pour qu'`orchestrator-dev` puisse la coller directement dans un commentaire Beads sans reformulation
- Si verdict = `commit` → `### Corrections requises` est vide ou contient "Aucune correction requise"
- **Le `### Routing recommandé`** est `developer-security` si et seulement si au moins un 🔴 Critique est de nature sécurité — sinon `retour-initial`

> ❌ Ne jamais produire le bloc handoff sans avoir d'abord produit le rapport de review complet.
> ❌ Ne jamais résumer le rapport — le bloc handoff est un résumé structuré, pas un substitut.

---

## Règles pour le consommateur (orchestrator-dev)

### À la réception du bloc `## Retour vers orchestrator-dev` du reviewer

1. **Lire le `### Verdict`** pour décider de la suite — ne pas réinterpréter le rapport manuellement :
   - `commit` → passer directement au CP-2 avec verdict "commit" recommandé
   - `corriger` ou `corriger-sécurité` → passer au CP-2 avec verdict "corriger" + routing selon `### Routing recommandé`

2. **Utiliser la `### Synthèse des problèmes`** pour remplir le bloc `### Contexte complet` du `## Question pour l'orchestrator` (CP-2 en mode invoqué depuis orchestrator) — ne pas résumer manuellement le rapport.

3. **Utiliser la `### Corrections requises`** pour remplir le commentaire Beads lors d'une décision "corriger" :
   ```bash
   bd comments add <ID> "Retours reviewer : <contenu de ### Corrections requises>"
   ```
   Ne jamais reformuler ou résumer ces corrections — les coller telles quelles.

4. **Utiliser le `### Routing recommandé`** pour router la correction :
   - `developer-security` → router vers `developer-security` pour la correction
   - `retour-initial` → retourner à l'agent developer initial

5. **Si le bloc est absent** → demander explicitement au reviewer de le produire avant de continuer.

6. **Si le rapport de review complet est absent** (le reviewer a produit le bloc handoff sans avoir produit le rapport) → demander explicitement au reviewer de produire le rapport complet avant de continuer.

> ❌ Ne jamais interpréter le rapport de review sans ce bloc structuré — risque de mauvaise décision de routing.
> ❌ Ne jamais résumer les corrections — les transmettre intégralement au commentaire Beads.
> ❌ Ne jamais accepter un bloc handoff sans rapport de review complet — les deux sont obligatoires.
