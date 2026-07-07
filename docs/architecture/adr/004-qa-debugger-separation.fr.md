# ADR-004 — QA Engineer et Debugger comme agents séparés

## Statut

Supercédé partiellement (juillet 2026)

> **Note :** L'agent `qa-engineer` a été supprimé (juillet 2026). Ses responsabilités (écriture des tests) sont transférées à l'agent `developer`. Le `reviewer` vérifie la couverture. L'agent `debugger` reste inchangé. Voir ADR-023.

## Contexte

Deux responsabilités étaient initialement absentes ou diluées dans les agents existants :

1. **L'écriture des tests** : les agents développeurs écrivent leurs propres tests,
   ce qui introduit un biais de confirmation — on teste ce qu'on a codé, pas ce qui
   devrait fonctionner.

2. **Le diagnostic de bugs** : les agents développeurs peuvent aussi débugger, mais
   le diagnostic rigoureux (lecture de stacktraces, hypothèses graduées, isolation de
   la cause racine) est une compétence distincte de l'implémentation.

La question était : intégrer ces responsabilités dans les agents existants (developer,
reviewer) ou créer des agents dédiés ?

## Décision

Deux agents dédiés sont créés :

- **`qa-engineer`** : reçoit une implémentation, écrit les tests manquants
  (unit / integration / E2E), produit un rapport de couverture. Ne modifie jamais
  le code fonctionnel. Invocable standalone ou comme étape optionnelle `[CP-QA]`
  dans l'agent orchestrator.

- **`debugger`** : reçoit une stacktrace ou des logs, applique une méthodologie
  de diagnostic en 4 étapes, produit un rapport de cause racine avec hypothèses
  graduées, et crée un ticket Beads de correction après confirmation. Ne corrige
  jamais le bug.

## Conséquences

### Positives

- Séparation des responsabilités : implémenter ≠ tester ≠ diagnostiquer
- Le QA a un regard indépendant sur l'implémentation (pas de biais de l'auteur)
- Le Debugger formalise le diagnostic avant la correction, ce qui réduit les
  corrections dans la mauvaise direction
- Les deux agents sont invocables indépendamment du workflow orchestrateur

### Négatives / compromis

- 2 agents supplémentaires à maintenir
- Le QA doit comprendre l'implémentation sans en être l'auteur — dépend de la
  qualité du diff/contexte fourni
- La limite entre "debugger identifie" et "developer corrige" peut créer des
  allers-retours si le diagnostic est incomplet

## Alternatives rejetées

**QA intégré au developer** : rejeté — même agent, même biais, pas de regard externe.

**Review étendue** : donner au reviewer la responsabilité d'écrire les tests manquants.
Rejeté car le reviewer est en lecture seule par principe (ADR-001 implicite) et que
confondre review et écriture de tests brouille les responsabilités.

**Debugger intégré au developer** : rejeté car le diagnostic rigoureux avec hypothèses
graduées nécessite un mode de pensée distinct de l'implémentation — mélanger les deux
pousse à corriger avant d'avoir identifié la vraie cause.
