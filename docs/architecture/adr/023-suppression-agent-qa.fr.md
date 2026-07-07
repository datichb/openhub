# 023 — Suppression de l'agent qa-engineer et du checkpoint CP-QA

## Statut

accepted

## Contexte

L'agent `qa-engineer` était un agent dédié qui intervenait entre l'implémentation (developer) et la review (reviewer) pour écrire les tests manquants et produire un rapport de couverture. Son invocation était gérée par le checkpoint CP-QA dans `orchestrator-dev`, avec une activation conditionnelle basée sur le niveau de risque du diff (élevé/moyen/faible).

En pratique, cette séparation introduisait plusieurs problèmes :

- **Latence du workflow** : un agent supplémentaire dans la boucle ajoutait un cycle complet de délégation/retour entre l'implémentation et la review
- **Redondance** : l'agent `developer` écrivait déjà des tests à l'étape 5 de son workflow, rendant le passage QA souvent redondant
- **Complexité du protocole orchestrator-dev** : la gestion du CP-QA (évaluation du risque, modes, configuration auto/manuel) représentait ~220 lignes de protocole
- **La pre-review (étape 3.5) exécute déjà `npm test`** : les tests sont lancés automatiquement avant toute review, garantissant que le code est fonctionnel

## Décision

Nous avons décidé de :

1. **Supprimer l'agent `qa-engineer`** et tous ses skills associés (`qa-protocol`, `qa-standalone`, `qa-subagent`, `qa-handoff-format`)
2. **Supprimer le checkpoint CP-QA** du workflow `orchestrator-dev`
3. **Transférer la responsabilité de couverture des tests au `developer`** — enrichissement du skill `dev-standards-testing` avec la checklist systématique et la gate de complétion issues de `qa-protocol`
4. **Ajouter un critère de couverture au `reviewer`** — le reviewer vérifie que les critères d'acceptance sont couverts par des tests et peut demander des tests supplémentaires via un finding 🟠 Majeur

## Conséquences

### Positives

- **Workflow simplifié** : Developer → Pre-review (tests auto) → Reviewer → CP-2
- **Moins de latence** : suppression d'un cycle agent complet dans la boucle
- **Protocole orchestrator allégé** : −220 lignes, suppression d'une étape complète et de sa logique conditionnelle
- **Responsabilité unique claire** : le developer est propriétaire de sa couverture de tests, le reviewer valide
- **Boucle de feedback directe** : si les tests sont insuffisants, le reviewer le signale et le developer corrige au cycle suivant

### Négatives / Compromis

- **Perte de la spécialisation QA** : l'expertise concentrée de l'agent QA (focus exclusif sur les tests, rapport structuré) est diluée dans le developer
- **Charge cognitive accrue pour le developer** : il doit appliquer la checklist systématique de couverture en plus de l'implémentation
- **Moins de séparation des préoccupations** : le même agent qui écrit le code écrit les tests — biais potentiel vers des tests qui suivent l'implémentation plutôt que le comportement

## Alternatives rejetées

| Alternative | Raison du rejet |
|-------------|----------------|
| Garder le QA mais le rendre obligatoire uniquement en risque élevé | Ne résout pas la latence ni la redondance — le developer écrit déjà des tests |
| Transformer le QA en simple vérification de couverture automatique | La pre-review (étape 3.5) assure déjà le run des tests — doublon |
| Fusionner QA dans le reviewer au lieu du developer | Le reviewer est read-only — il ne peut pas écrire de tests, seulement les demander |
