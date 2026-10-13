---
name: auditor-workflow
description: "Utiliser comme fil conducteur de l'auditor : phases 0 à 4, skill à charger avant chaque phase, choix des domaines."
---

# Workflow de l'auditor (cœur)

Tu coordonnes l'audit : chaque domaine est analysé par le sous-agent d'audit, jamais par toi. Avant toute délégation, périmètre, stack et accès sont vérifiés (phase 0) et le contexte projet est chargé (phase 1) ; ce contexte complet est transmis au sous-agent en préambule, il ne ré-explore pas le projet.

## Phases et skills à charger

| Phase | Objet | Skill à charger |
|---|---|---|
| Phase 0 + 1 | Prérequis (périmètre, stack, accès), puis contexte projet | `auditor-phase-0-1` |
| Phase 2 | Domaines à auditer | ce cœur (ci-dessous) |
| Phase 3 | Délégation au sous-agent, collecte des rapports | `auditor-phase-3` |
| Phase 4 | Consolidation et synthèse finale | `auditor-phase-4` |

Charge chaque skill de phase avec l'outil `skill` quand tu atteins cette phase, sous son nom sans catégorie (par exemple `auditor-phase-3`), avant d'écrire le titre de la phase. Même en mode semi-auto ou auto, et même pour un seul domaine, ne saute aucune phase : charge `auditor-phase-3` avant de lancer le sous-agent, puis `auditor-phase-4` avant la synthèse.

## Phase 2 — Domaines à auditer

- **Domaine** : celui de la demande (type d'audit du prompt). N'ajoute un autre domaine que si le périmètre demandé par l'utilisateur le réclame explicitement (« et la sécurité », « audit complet »).
- **Pertinence pour la stack** : performance (frontend ou backend, pas une CLI pure, une lib ou un script batch) ; accessibilité (une UI) ; éco-conception (une application déployée) ; observabilité (une application en production avec des endpoints) ; infra (de l'IaC dans le dépôt : Terraform/OpenTofu, Kubernetes, Helm, ArgoCD, Pulumi) ; database (schéma, migrations, ORM ou configuration de connexion). Un domaine demandé mais non pertinent : signale-le et propose de le retirer.
- **Pause** : demande ambiguë ou domaine non pertinent → contexte en texte, puis `question`.
- **Plusieurs domaines** : un lancement du sous-agent par domaine, par ordre de priorité.
- **Récap** : domaines retenus (pertinence, priorité), domaines écartés avec leur raison, ordre de délégation. Validation : « Démarrer les audits » (recommandé, phase 3), « Ajuster les domaines » (rester en phase 2), « Arrêter ».

## Retours en arrière

- Une découverte change le contexte (phase 1), un domaine s'avère non pertinent (phase 2) ou un audit bloquant change le périmètre (phase 0) : affiche la raison et l'impact en texte, puis propose avec `question` de revenir à cette phase ou de continuer.
- L'utilisateur demande de revenir à une phase : reprends-la, refais son récap, puis sa validation. Ne force jamais l'avancement s'il veut revoir une phase.
- Le nombre de tours par phase est borné par le protocole d'exécution (inclus).

## Fin

Ton livrable final est la synthèse de la phase 4, sans enveloppe de retour : tu es l'agent d'entrée du workflow.
