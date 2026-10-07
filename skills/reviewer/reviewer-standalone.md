---
name: reviewer-standalone
description: Parcours d'exécution du reviewer en mode standalone (invoqué directement par l'utilisateur) — sélection interactive du mode de review (standard, adversarial, edge-case, combinaisons), préparation du contexte wiki pour injection dans les sous-sessions parallèles, fusion via review-merge, enrichissement des documents vivants proposé en fin de session, sans bloc handoff orchestrator-dev.
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

1. Exécuter le workflow complet du `reviewer.md` (étapes 0 à 6)
2. Charger le skill correspondant via l'outil `skill` :
   - Standard → skill `review-protocol` (déjà en Bucket A)
   - Adversarial → skill `reviewer-adversarial`
   - Edge-case → skill `reviewer-edge-case`
3. Produire le rapport au format défini par le skill chargé
4. Passer à l'Étape 3

### Mode combiné (Standard + Adversarial, ou All)

Orchestrer des **sessions parallèles indépendantes** avec injection du contexte projet :

1. **Préparer le contexte à injecter** (session parente — avant de lancer les sous-sessions) :
   a. Exécuter les étapes 0 et 1 du workflow `reviewer.md` (wiki + diff + périmètre)
   b. Identifier les standards pertinents (étape 1.5 du workflow)
   c. Extraire une **synthèse compacte** (max ~60 lignes) contenant :
      - Les conventions clés du projet (nommage, patterns, imports, structure)
      - Les décisions architecturales structurantes
      - Les review-rules spécifiques au projet (si `review-rules.md` existe)
      - Les god nodes touchés par les fichiers du diff
      - Le périmètre de fichiers modifiés (après exclusions path filters)
   d. Formater :
      - `[WIKI-CONTEXT:<synthèse>]` — les conventions/architecture/review-rules extraites
      - `[DIFF-SCOPE:<liste fichiers>]` — les fichiers modifiés (périmètre de review)
      - `[STANDARDS:<liste>]` — les dev-standards à charger (ex: `frontend,testing,security,git`)

2. **Lancer les sessions en parallèle** via l'outil `task` :

   Pour "Standard + Adversarial" :
   ```
   // Session 1 — Standard (contexte wiki injecté)
   task(subagent_type: "reviewer", prompt: "[MODE:standard] [REVIEW:single] [WIKI-CONTEXT:<synthèse>] [DIFF-SCOPE:<liste fichiers>] [STANDARDS:<liste>] Review de la branche <branche>. git diff <base>..<branche>")

   // Session 2 — Adversarial (contexte wiki injecté)
   task(subagent_type: "reviewer", prompt: "[MODE:adversarial] [REVIEW:single] [WIKI-CONTEXT:<synthèse>] [DIFF-SCOPE:<liste fichiers>] [STANDARDS:<liste>] Review adversariale de la branche <branche>. git diff <base>..<branche>")
   ```

   Pour "Standard + Adversarial + Edge-case" :
   ```
   // Session 1 — Standard
   task(subagent_type: "reviewer", prompt: "[MODE:standard] [REVIEW:single] [WIKI-CONTEXT:<synthèse>] [DIFF-SCOPE:<liste fichiers>] [STANDARDS:<liste>] ...")

   // Session 2 — Adversarial
   task(subagent_type: "reviewer", prompt: "[MODE:adversarial] [REVIEW:single] [WIKI-CONTEXT:<synthèse>] [DIFF-SCOPE:<liste fichiers>] [STANDARDS:<liste>] ...")

   // Session 3 — Edge-case
   task(subagent_type: "reviewer", prompt: "[MODE:edge-case] [REVIEW:single] [WIKI-CONTEXT:<synthèse>] [DIFF-SCOPE:<liste fichiers>] [STANDARDS:<liste>] ...")
   ```

3. **Récupérer les rapports bruts** de chaque session
4. **Fusionner** en appliquant le skill `review-merge` :
   - Charger le skill `review-merge` via l'outil `skill`
   - Lui fournir les rapports bruts récupérés
   - Produire le rapport unifié final
5. Passer à l'Étape 3

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

## Sous-session mono-mode (`[REVIEW:single]`)

Quand une sous-session est lancée avec `[REVIEW:single]` :

- Si `[WIKI-CONTEXT:...]` est présent → l'utiliser comme contexte conventions/architecture (ne PAS relire le wiki depuis le disque — le contexte a été préparé par la session parente)
- Si `[DIFF-SCOPE:...]` est présent → l'utiliser comme périmètre de fichiers modifiés pour le scope enforcement
- Si `[STANDARDS:...]` est présent → charger uniquement ces dev-standards (ne PAS charger les autres)
- Exécuter la review dans le mode indiqué par `[MODE:...]`
- Inclure le walkthrough dans le rapport (construit à partir du diff et du DIFF-SCOPE)
- Inclure le score de confiance sur chaque finding
- Appliquer la checklist d'auto-vérification du `review-protocol`
- Produire le rapport brut au format du mode (voir section "Format de sortie brut" dans `review-protocol`)
- **Ne pas** proposer l'enrichissement des living docs (c'est le rôle de la session parente)
- **Ne pas** poser de question de sélection de mode (le mode est explicite)
- Retourner le rapport brut comme résultat de la session `task`

> Note : `[REVIEW:single]` est un marqueur de mode, pas une skill (ne pas la charger avec l'outil `skill`). Le reviewer détecte ce marqueur et applique ce comportement simplifié.
