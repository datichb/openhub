---
name: planner-phase-2-5
description: "Utiliser avant la phase 2.5 du planner (complexité Large ou Enterprise) : compréhension partagée et decision map."
---

# Phase 2.5 — Compréhension partagée (Understanding Gate)

## Déclenchement

- **Complexité >= Large (11+ pts)** : gate **obligatoire** — toujours produire le résumé de compréhension et demander validation avant Phase 3
- **Complexité Medium (7-10 pts)** : gate **optionnel** — proposé dans la validation de la phase 2 (« Vérifier la compréhension avant de continuer »)
- **Complexité Small (4-6 pts)** : gate **omis**

## Objectif

Vérifier l'alignement avec l'utilisateur sur ce qu'ont appris les phases 1 et 2 **avant** de décomposer en tickets. Un malentendu détecté ici coûte une question ; en Phase 5, une re-planification.

## Résumé de compréhension

```markdown
## [Phase 2.5] Compréhension partagée

### Ce que je comprends de la feature
<2-4 phrases : objectif, valeur métier, périmètre technique, utilisateurs concernés>

### Décisions prises (réponses de la phase 2)
- <décision — ex : "L'optimisation performance est hors scope pour cette itération">

### Hypothèses retenues (non confirmées mais intégrées au plan)
- <hypothèse — ex : "La lib X supporte le streaming (doc v3.2, non testé)">

### Hors périmètre confirmé
- <élément explicitement exclu>

### Points d'attention pour la décomposition
- <risque ou contrainte qui influencera la phase 3>
```

> Ce résumé devient le **brief de décomposition** utilisé en Phase 3. Toute correction apportée ici est intégrée avant de décomposer.
<!-- oh:when role=entry -->

```
question({
  questions: [{
    header: "Compréhension partagée",
    question: "[Planner — Phase 2.5 | Feature : <nom>]\nVoici ma compréhension de la feature (résumé ci-dessus). Est-ce correct et complet ?",
    options: [
      { label: "Correct — passer à Phase 3 (Recommandé)", description: "Compréhension validée, démarrer la décomposition" },
      { label: "Corriger", description: "Des points à ajuster dans la compréhension" },
      { label: "Poser d'autres questions", description: "Rester en Phase 2 pour préciser d'autres points" },
      { label: "Revenir à Phase 1", description: "Explorer à nouveau avec les nouvelles informations" }
    ]
  }]
})
```

**Selon la réponse :** correct → Phase 3, avec le résumé comme brief (Enterprise : decision map d'abord) ; corriger → mettre à jour le résumé et le re-présenter (au plus 3 itérations) ; d'autres questions → Phase 2 ; revenir à Phase 1 → Phase 1.
<!-- /oh:when -->
<!-- oh:when role=subagent -->

Sous-agent : pas de question ; le résumé ouvre ton rapport (brief de décomposition, hypothèses comprises) et ton appelant le valide avec le découpage.
<!-- /oh:when -->

---

## Split Enterprise — Decision Map (complexité >= 14 pts uniquement)

Compréhension validée et complexité Enterprise : produis un **decision map** et recommande une nouvelle session pour la décomposition (phases 3 à 6). Le contexte accumulé en phases 0 à 2 (exploration, librairies, impacts, rounds de questions) sature la fenêtre de contexte et dégrade le plan ; une session fraîche partant du decision map donne un meilleur plan.

Le decision map est auto-suffisant : tout ce dont la phase 3 a besoin.

```markdown
## Decision Map — <nom de la feature>

### Métadonnées
- **Feature :** <nom> · **Complexité :** Enterprise (<X> pts) · **Date :** <date>

### Objectif et périmètre
<résumé de compréhension de la phase 2.5, tel quel>

### Exploration codebase (phase 1)
- **Architecture :** <patterns> · **Stack :** <technologies>
- **Fichiers structurants :** <chemins relatifs> · **Tests existants :** <couverture>

### Bibliothèques analysées (1.2bis)
| Lib | Comportement | Statut | Source |
|-----|-------------|--------|--------|

### Impacts cascade (1.2ter)
| Fichier partagé | Consommateurs | Statut (couvert / ticket séparé / exclu) |
|-----------------|--------------|--------|

### Design (1.5)
<résumé des specs UX/UI, ou « N/A »>

### Décisions utilisateur [DECISION] · Faits vérifiés [FACT] (avec source)
### Hypothèses retenues · Hors périmètre confirmé · Risques identifiés
### Priorités déduites (P0 à P3)
```
<!-- oh:when role=entry -->

```
question({
  questions: [{
    header: "Split Enterprise",
    question: "[Planner — Phase 2.5 Enterprise | Feature : <nom>]\nLe decision map est prêt (ci-dessus). Pour un projet de cette complexité, je recommande de démarrer une nouvelle session pour la décomposition (Phase 3-6) avec un contexte frais.\n\nVoulez-vous :",
    options: [
      { label: "Nouvelle session (Recommandé)", description: "Démarrer Phase 3 dans une session fraîche avec le decision map" },
      { label: "Continuer ici", description: "Poursuivre dans cette session (contexte potentiellement saturé)" }
    ]
  }]
})
```

**Selon la réponse :** nouvelle session → le decision map est l'artefact final du récap ; l'utilisateur relance une session avec lui en contexte et tu démarres à la phase 3 ; continuer ici → phase 3 dans cette session (l'utilisateur assume le risque).
<!-- /oh:when -->
<!-- oh:when role=subagent -->

Sous-agent : la question du split va dans tes `### Questions bloquantes` (statut `en-attente`, decision map dans ton rapport) ; ton appelant relaie la réponse.
<!-- /oh:when -->
