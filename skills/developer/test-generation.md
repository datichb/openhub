---
name: test-generation
description: "Utiliser pour le domaine tests (« ajouter des tests sur X ») : gaps de couverture, priorités, tests dans le style du projet."
---

# Skill — Génération de tests

Méthode pour ajouter des tests à du code qui existe déjà. Les règles d'écriture (AAA, nommage,
mocking, anti-patterns) sont celles de `dev-standards-testing` : charge-la aussi.

## Périmètre

- Tu ajoutes des tests ; le code de production reste tel quel.
- Code non testable en l'état (couplage fort, effets de bord globaux, dépendance cachée) : signale-le
  dans le handoff (points d'attention) avec la modification qui le rendrait testable — elle fera l'objet
  d'un ticket séparé (domaine `refactor`).
- Un test vérifie un comportement observable, pas l'implémentation.

## 1. Analyse des gaps

1. Lis les tests existants : framework, emplacement des fichiers, nommage, fixtures, helpers. Les
   nouveaux tests suivent ce style.
2. Lance le runner de couverture du projet (script du projet en priorité ; sinon par exemple
   `npx vitest run --coverage`, `go test -cover ./...`, `.venv/bin/python -m pytest --cov`).
3. Liste les fichiers, fonctions et branches non couverts, puis classe-les :

| Priorité | Code concerné |
|----------|---------------|
| CRITIQUE | Logique métier, calculs, autorisations, validation des entrées |
| MAJEUR | Intégrations (API ↔ service, service ↔ base), gestion d'erreurs |
| MINEUR | Utilitaires, glue, configuration |

Cibles indicatives : 80 % des lignes, 70 % des branches sur le code métier — sauf seuil propre au projet.

Si la demande est une analyse seule (ou si le volume dépasse le périmètre), rends le rapport avant de générer :

```
## Gaps de couverture — <périmètre>
- Couverture actuelle : <X> % lignes, <Y> % branches
- [CRITIQUE] <fichier> — fonctions / branches non couvertes — <N> tests à écrire (<type>)
- [MAJEUR] …
- [MINEUR] …
```

## 2. Génération

Pour chaque élément, dans l'ordre des priorités, couvre : cas nominal, limites (vide, null, zéro,
maximum), entrées invalides, cas d'erreur.

**Unitaires** — un test par comportement ; dépendances externes (base, réseau, fichiers, horloge)
isolées par des doublures.

**Intégration** — contrats entre composants (route ↔ service, service ↔ base) : fixtures
déterministes, état remis à zéro entre les tests (transaction annulée, base jetable, testcontainers),
codes et messages d'erreur vérifiés.

**Property-based** — quand le code a des invariants : fast-check (JS/TS), Hypothesis (Python),
rapid (Go), proptest (Rust). Invariants courants :
- aller-retour : `parse(serialize(x)) == x` ;
- idempotence : `f(f(x)) == f(x)` ;
- conservation : le tri garde les mêmes éléments, un total reste égal à la somme des parties.

Générateurs contraints par les règles métier ; en cas d'échec, note le cas minimal réduit dans le
test (exemple explicite) et dans le handoff.

## 3. Validation

- Lance les tests ajoutés : ils passent.
- Vérifie qu'ils détectent vraiment une régression : fausse temporairement la valeur attendue d'une
  assertion, constate l'échec, puis remets-la.
- Indique dans le handoff la couverture avant/après et les gaps restants.
