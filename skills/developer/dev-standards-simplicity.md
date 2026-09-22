---
name: dev-standards-simplicity
description: Principes de simplicité — KISS, YAGNI, pas d'abstraction prématurée, pas d'optimisation prématurée, limites de complexité mesurables. La solution la plus simple qui répond au besoin est toujours préférée.
---

# Skill — Standards de Simplicité

## Principe fondamental

La simplicité est un critère de qualité au même titre que la sécurité ou la testabilité.
Le code le plus simple qui répond au besoin actuel est toujours le bon choix.
La complexité accidentelle est une dette — elle ralentit, elle fragile, elle décourage.

**Règle d'or : avant d'écrire du code, demande-toi si tu peux faire moins.**

---

## KISS — Keep It Simple

Préfère toujours la solution la plus directe qui résout le problème posé.

- Signal: Pattern Strategy/Factory/Interface pour ≤ 2 cas → utiliser un simple if/else
- Signal: Classe avec injection de dépendance pour un usage unique → utiliser une fonction
- Signal: ORM/query builder custom pour une requête simple → utiliser la requête directe

**Question à poser avant d'implémenter :** "Est-ce que la version la plus naïve résout déjà le problème ?"
Si oui, implémenter la version naïve. L'optimisation et l'abstraction viennent ensuite, si les faits les justifient.

---

## YAGNI — You Aren't Gonna Need It

N'implémente pas ce qui n'est pas demandé par un ticket actif.

- Signal: système de plugins "au cas où"
- Signal: cache "parce que ça risque d'être lent un jour"
- Signal: abstraction "pour quand on aura plusieurs implémentations"
- Signal: paramètres de configuration pour des comportements inexistants

**Règle :** si ce n'est pas dans le ticket, ne l'implémente pas. Si c'est important, ça sera un ticket.

---

## Pas d'abstraction prématurée

N'extrais une abstraction qu'à partir de **3 cas d'usage concrets et existants**.

```
1 cas  → implémente directement, pas d'abstraction
2 cas  → duplique si nécessaire, note la ressemblance
3 cas  → extrait l'abstraction avec la connaissance des 3 cas réels
```

Une abstraction créée sur 1 ou 2 cas encode les mauvaises hypothèses.
Elle contraint l'évolution au lieu de la faciliter.

---

## Duplication > mauvaise abstraction

Copier-coller du code une fois est acceptable.
Créer une abstraction forcée pour éviter cette duplication est souvent une dette plus lourde.

- Signal: fonction générique avec 5+ paramètres booléens pour couvrir 2 cas → dupliquer
- Signal: composant "universel" avec prop `mode` pilotant des dizaines de comportements → séparer

**Critère :** si l'abstraction nécessite plus de paramètres que ce qu'elle économise en lignes, elle n'est pas prête.

---

## Pas d'optimisation prématurée

N'optimise pas sans mesure préalable.

- Signal: cache Redis ajouté "parce que ça va être lent"
- Signal: parallélisation sans avoir mesuré la durée séquentielle
- Signal: dénormalisation par anticipation
- Signal: structure de données complexe "théoriquement plus rapide"

**Processus correct :** implémenter lisiblement → mesurer (profiler, bench, APM) → identifier le vrai goulot → optimiser uniquement ce goulot avec test de non-régression perf.

La lisibilité et la correction passent toujours avant la performance, sauf contrainte explicite dans le ticket.

---

## Limites de complexité mesurables

Ces seuils sont des signaux d'alerte, pas des règles absolues.
Les dépasser nécessite une justification explicite dans le code (commentaire ou PR description).

| Mesure | Seuil recommandé | Signal d'alerte |
|--------|-----------------|-----------------|
| Longueur d'une fonction | ≤ 20 lignes | > 30 lignes → à scinder |
| Complexité cyclomatique | ≤ 10 | > 15 → à refactorer |
| Nombre de paramètres | ≤ 4 | > 4 → introduire un objet de configuration |
| Profondeur d'imbrication | ≤ 3 niveaux | > 3 → extraire une fonction ou inverser la condition |
| Nombre de dépendances injectées | ≤ 5 | > 5 → la classe a trop de responsabilités |

---

## Signaux d'alerte — over-engineering à challenger

Ces patterns ne sont pas interdits, mais chacun doit être justifié par un besoin réel et actuel :

- `AbstractFactory` ou `Builder` pour un seul type d'objet
- `Interface` avec une seule implémentation (hors test)
- Classe avec un seul constructeur et une seule méthode publique → probablement une fonction
- Middleware générique pour un comportement utilisé à un seul endroit
- Configuration externalisée pour une valeur qui ne changera jamais
- Event bus interne pour des communications entre deux modules seulement
- Pattern Repository sur une source de données qui n'a pas de logique d'accès
- Paramètre `options?: {}` vide ajouté "pour l'extensibilité future"

**Réponse attendue face à ces signaux :** challenger, proposer la version simple, documenter si la complexité est retenue.

---

## Ce que ce skill ne dit PAS

La simplicité ne dispense pas des bonnes pratiques. Une architecture hexagonale, un pattern Strategy, un cache ou une abstraction sont justifiés **quand le besoin actuel le démontre** (domaine complexe, variantes nombreuses, benchmark prouvant un problème perf, réduction effective de la complexité globale). Le critère : la complexité est-elle justifiée par le besoin actuel ?
