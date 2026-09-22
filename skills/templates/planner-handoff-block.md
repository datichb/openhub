---

## Retour vers orchestrator

**Agent :** planner
**Feature :** <nom de la feature planifiée>

### Récapitulatif de planification

<Contexte narratif de la décomposition : pourquoi ces tickets, pourquoi cet ordre, quels compromis, quelles hypothèses faites. Ce texte apporte le "pourquoi" qui ne peut pas être encodé dans le tableau ci-dessous. Minimum 3-5 phrases pour toute planification non triviale.>

<Ex : "La feature a été décomposée en 3 tickets séquentiels car le middleware JWT dépend du service d'authentification. L'endpoint login a été priorisé car il est bloquant pour bd-43 et bd-44. Le stockage en localStorage a été choisi comme hypothèse par défaut faute de précision dans la demande — les cookies httpOnly seraient une alternative plus sécurisée.">

### Tickets créés

| ID | Titre | Type | Priorité | Labels | Agent prévu | TDD | Dépend de |
|----|-------|------|----------|--------|-------------|-----|-----------|
| bd-XX | <titre> | feature | P1 | <labels> | developer-backend | — | — |
| bd-YY | <titre> | task | P1 | <labels> | developer-frontend | ✅ | bd-XX |
| bd-ZZ | <titre> | feature | P2 | audit-security | auditor | — | — |

**Total :** X tickets créés (Y epics + Z tickets fils)

### Dépendances
- `bd-YY` dépend de `bd-XX` : <raison — ex : le composant frontend consomme l'API créée par bd-XX>
- `bd-ZZ` peut être traité en parallèle de `bd-XX` et `bd-YY`
<"Aucune dépendance entre les tickets" si tous sont indépendants>

### Ordre de traitement
1. bd-XX — <raison : bloquant pour bd-YY / ticket fondation>
2. bd-YY, bd-ZZ — <parallélisables après bd-XX>
<Séquence exacte d'exécution que l'agent orchestrator doit suivre sans interprétation>

### Hypothèses et ambiguïtés
- <hypothèse 1 — ce qui n'était pas explicite dans la demande et a été inféré>
- <ambiguïté 1 — point non tranché qui pourrait influencer l'implémentation>
<"Aucune" si la demande était complète et sans ambiguïté>

### Estimation globale
**Tickets :** X | **Complexité estimée :** <faible | moyenne | élevée>
<estimation en jours ou sprints si des signaux suffisants sont disponibles, sinon omettre>

### Risques identifiés
- <risque 1 — ex : dépendance externe non maîtrisée, ticket avec critères d'acceptance flous>
- <risque 2>
<"Aucun risque identifié" si le plan est clair et sans risque notable>

### Statut
`planification-complète` | `planification-partielle` | `bloqué`
