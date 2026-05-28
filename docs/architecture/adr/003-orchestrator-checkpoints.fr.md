# ADR-003 — Orchestrateur avec checkpoints explicites

## Statut

Accepté

## Contexte

Lors de la conception de l'agent `orchestrator`, deux philosophies s'opposaient :

1. **Automatisation complète** : l'orchestrateur enchaîne planner → developer → qa →
   reviewer sans interruption, et présente un résultat final à l'utilisateur.
2. **Checkpoints explicites** : l'orchestrateur pause à chaque étape clé et attend
   une confirmation explicite avant de continuer.

L'automatisation complète semblait plus fluide, mais elle présentait des risques
importants dans un contexte où les agents IA peuvent produire des résultats incorrects,
incomplets ou non conformes aux attentes.

## Décision

L'orchestrateur impose des **checkpoints explicites** (notés `[CP-X]`) à chaque
étape critique :

- `[CP-0]` — Avant de démarrer le workflow (validation des tickets planifiés)
- `[CP-1]` — Avant chaque ticket (confirmation de démarrage)
- `[CP-QA]` — Avant l'étape QA (activation conditionnelle selon le risque détecté)
- `[CP-2]` — Après la review (merge ou corrections ?)
- `[CP-3]` — Après chaque ticket (ticket suivant ou stop ?)

L'orchestrateur ne passe jamais à l'étape suivante sans réponse explicite.

## Évolutions

### Mai 2026 — Activation conditionnelle du CP-QA

Le checkpoint `[CP-QA]` a été amélioré avec une **activation conditionnelle basée sur le niveau de risque** détecté automatiquement dans le diff :

**Comportement selon le risque :**

- **🔴 Risque élevé** (API, services, code critique, >200 lignes) → QA obligatoire, pas de checkpoint
- **🟡 Risque moyen** (utils, logique métier dans composants) → QA recommandé par défaut
- **⚪ Risque faible** (UI pure, doc, config) → QA optionnel

**Tickets TDD :** Au lieu de skipper automatiquement le QA, un audit rapide de couverture est effectué pour valider que le TDD a été correctement appliqué (couverture >= 80%, tous critères couverts). Si le TDD est incomplet, le qa-engineer écrit les tests manquants.

**Valeur ajoutée du qa-engineer :** Le qa-engineer produit désormais une section `### Points d'attention pour la review` dans son handoff, transmise au reviewer pour orienter sa review sur les zones critiques (code non testable, edge cases non couverts, hypothèses faites).

Cette approche maximise la qualité sur le code critique sans ralentir les tickets simples.

## Conséquences

### Positives

- L'utilisateur garde le contrôle à chaque étape
- Les erreurs d'un agent sont détectées avant de se propager aux étapes suivantes
- Permet d'interrompre, de passer un ticket ou de changer de direction à tout moment
- Adapté à un contexte où les agents IA ne sont pas infaillibles

### Négatives / compromis

- Plus lent qu'un workflow entièrement automatisé
- Requiert une présence active de l'utilisateur pendant tout le workflow
- Peut devenir fastidieux sur des features avec de nombreux tickets simples

## Alternatives rejetées

**Automatisation complète** : rejetée car un bug d'implémentation non détecté au
ticket 2 peut contaminer les tickets 3 à N avant que l'utilisateur intervienne.

**Automatisation avec alerte uniquement sur erreur** : rejetée car "pas d'erreur"
ne signifie pas "conforme aux attentes" — la review peut signaler des problèmes
fonctionnels qui ne génèrent pas d'erreur technique.

**Mode configurable** (auto / manuel) : ~~possible en évolution future, mais introduit
de la complexité de configuration sans valeur immédiate prouvée~~ → **Implémenté** (mai 2026) avec 3 modes : `manuel`, `semi-auto`, `auto`. Le mode `auto` permet un workflow fluide tout en gardant le contrôle sur les étapes critiques grâce à l'activation conditionnelle du QA.
