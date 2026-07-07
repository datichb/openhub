---
name: reviewer-standalone
description: Parcours d'exécution du reviewer en mode standalone (invoqué directement par l'utilisateur) — sélection interactive du mode de review (standard, adversarial, edge-case, combinaisons), orchestration multi-mode avec sessions parallèles, fusion via review-merge, enrichissement des documents vivants proposé en fin de session, sans bloc handoff orchestrator-dev.
---

# Skill — Parcours Reviewer Standalone

> Ce skill est chargé automatiquement quand le reviewer est invoqué directement par l'utilisateur (aucun `[SKILL:...]` injecté dans le prompt).

## Principe fondamental

En mode standalone, le reviewer interagit directement avec l'utilisateur. Il propose le choix du mode de review, orchestre les sessions parallèles si nécessaire, et affiche le rapport final.

---

## Étape 1 — Sélection du mode de review

Au démarrage, **avant toute analyse**, proposer le choix du mode via l'outil `question` :

```
question({
  questions: [{
    header: "Mode de review",
    question: "Quel mode de review souhaites-tu ?",
    options: [
      { label: "Standard", description: "Review classique — checklist 6 catégories, rapport structuré par sévérité" },
      { label: "Adversarial", description: "Critique approfondie — scepticisme maximal, min. 10 findings, hypothèses dangereuses" },
      { label: "Edge-case", description: "Chasse aux chemins non gérés — exhaustivité des paths d'exécution" },
      { label: "Standard + Adversarial", description: "Sessions parallèles indépendantes + rapport unifié fusionné" },
      { label: "Standard + Adversarial + Edge-case", description: "Couverture maximale — 3 sessions parallèles + rapport unifié" }
    ]
  }]
})
```

> **Exception** : si le prompt initial contient un mot-clé explicite (`[MODE:standard]`, `[MODE:adversarial]`, `[MODE:edge-case]`, `[MODE:standard+adversarial]`, `[MODE:all]`), ne pas poser la question et utiliser le mode indiqué.

---

## Étape 2 — Exécution selon le mode choisi

### Mode unique (Standard, Adversarial, ou Edge-case)

Exécuter directement la review dans cette session :

1. Charger le skill correspondant via l'outil `skill` :
   - Standard → skill `review-protocol` (déjà en Bucket A)
   - Adversarial → skill `reviewer-adversarial`
   - Edge-case → skill `reviewer-edge-case`
2. Exécuter le workflow de review complet
3. Produire le rapport au format défini par le skill chargé
4. Passer à l'Étape 3

### Mode combiné (Standard + Adversarial, ou All)

Orchestrer des **sessions parallèles indépendantes** pour garantir l'isolation contextuelle :

1. **Lancer les sessions en parallèle** via l'outil `task` :

   Pour "Standard + Adversarial" :
   ```
   // Session 1 — Standard (contexte vierge)
   task(subagent_type: "reviewer", prompt: "[MODE:standard] [SKILL:reviewer/reviewer-standalone-single] Review de la branche <branche>. Diff:\n<diff ou instructions git>")

   // Session 2 — Adversarial (contexte vierge)
   task(subagent_type: "reviewer", prompt: "[MODE:adversarial] [SKILL:reviewer/reviewer-standalone-single] Review adversariale de la branche <branche>. Diff:\n<diff ou instructions git>")
   ```

   Pour "Standard + Adversarial + Edge-case" :
   ```
   // Session 1 — Standard
   task(subagent_type: "reviewer", prompt: "[MODE:standard] [SKILL:reviewer/reviewer-standalone-single] ...")

   // Session 2 — Adversarial
   task(subagent_type: "reviewer", prompt: "[MODE:adversarial] [SKILL:reviewer/reviewer-standalone-single] ...")

   // Session 3 — Edge-case
   task(subagent_type: "reviewer", prompt: "[MODE:edge-case] [SKILL:reviewer/reviewer-standalone-single] ...")
   ```

2. **Récupérer les rapports bruts** de chaque session
3. **Fusionner** en appliquant le skill `review-merge` :
   - Charger le skill `review-merge` via l'outil `skill`
   - Lui fournir les rapports bruts récupérés
   - Produire le rapport unifié final
4. Passer à l'Étape 3

---

## Étape 3 — Finalisation

1. Afficher le rapport (simple ou unifié) dans la discussion
2. Appliquer le skill `living-docs-enrichment` : proposer l'enrichissement des documents vivants via l'outil `question`
3. **Ne pas** produire le bloc `## Retour vers orchestrator-dev`

L'utilisateur consulte le rapport et décide lui-même de l'action à prendre (commit, corriger, ignorer).

---

## Règles de routing des skills en mode combiné

| Mode choisi | Skills chargés dans les sous-sessions | Fusion |
|-------------|--------------------------------------|--------|
| Standard seul | `review-protocol` (Bucket A) | Non |
| Adversarial seul | `reviewer-adversarial` | Non |
| Edge-case seul | `reviewer-edge-case` | Non |
| Standard + Adversarial | Sous-session 1: `review-protocol`, Sous-session 2: `reviewer-adversarial` | `review-merge` |
| All (3 modes) | Sous-session 1: `review-protocol`, Sous-session 2: `reviewer-adversarial`, Sous-session 3: `reviewer-edge-case` | `review-merge` |

---

## Skill auxiliaire : reviewer-standalone-single

Quand une sous-session est lancée avec `[SKILL:reviewer/reviewer-standalone-single]` :
- Exécuter la review dans le mode indiqué par `[MODE:...]`
- Produire le rapport brut au format du mode (voir section "Format de sortie brut" dans `review-protocol`)
- **Ne pas** proposer l'enrichissement des living docs (c'est le rôle de la session parente)
- **Ne pas** poser de question de sélection de mode (le mode est explicite)
- Retourner le rapport brut comme résultat de la session `task`

> Note : `reviewer-standalone-single` est un comportement implicite déclenché par la présence du tag `[SKILL:reviewer/reviewer-standalone-single]` — il ne nécessite pas un fichier skill séparé. Le reviewer détecte ce tag et applique ce comportement simplifié.
