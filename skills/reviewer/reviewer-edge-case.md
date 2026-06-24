---
name: reviewer-edge-case
description: "Utiliser quand une analyse exhaustive des chemins non gérés est demandée — chasse aux cas limites oubliés dans du code ou une spec. Parcourt tous les branchements conditionnels, frontières de domaine, et conditions aux limites pour identifier les chemins non couverts. Couvre : control flow (conditionnels, boucles, early returns, error handlers), frontières de valeurs (null, empty, overflow, underflow), race conditions, timeouts, coercions implicites. Ne rapporte que les chemins non gérés — ignore les gérés. Mots-clés : edge case, missing else, null handling, off-by-one, race condition, boundary condition, unhandled path."
bucket: B
---

# Skill — Chasse aux Cas Limites (Edge Case Hunter)

## Rôle

Effectuer une analyse exhaustive de tous les chemins d'exécution d'un périmètre donné pour identifier les cas limites non gérés. Dériver les classes d'edge cases du contenu lui-même — ne pas appliquer une checklist fixe.

Ce mode complète la review standard et la revue adversariale en se concentrant exclusivement sur la complétude des chemins.

---

## 🔒 Règles absolues

❌ Ne pas rapporter les chemins correctement gérés — les ignorer silencieusement
❌ Ne pas se baser sur une checklist fixe — dériver les classes d'edge cases du code analysé
❌ Ne pas mélanger qualité du code et chemins non gérés — ce skill fait une seule chose
✅ Pour chaque chemin non géré : décrire la conséquence potentielle + suggestion de correction
✅ Revisiter chaque classe après l'analyse initiale pour confirmer la complétude

---

## EXECUTION

### Étape 1 — Recevoir le périmètre

Identifier ce qui est analysé :
- Une fonction ou méthode spécifique
- Un module complet
- Un flux de données end-to-end
- Une spec ou un plan (analyse des cas non couverts dans les critères d'acceptance)

Si un paramètre `also_consider` est fourni (zones connexes à inclure), l'intégrer dans l'analyse.

### Étape 2 — Analyse exhaustive des chemins

**Pour chaque unité analysée, parcourir :**

#### Control flow
- Chaque branchement conditionnel : le cas `else` est-il géré ?
- Chaque boucle : que se passe-t-il si la collection est vide ? Si elle a un seul élément ? Si elle est très grande ?
- Chaque `early return` : les cas qui le déclenchent sont-ils exhaustifs ?
- Chaque gestionnaire d'erreur : les erreurs inattendues sont-elles propagées correctement ?

#### Frontières de valeurs
- Valeurs nulles / undefined : chaque paramètre est-il protégé ?
- Collections vides : `[]`, `{}`, `""` ?
- Valeurs limites numériques : 0, -1, `Number.MAX_SAFE_INTEGER`, `NaN`, `Infinity` ?
- Overflow / underflow arithmétique ?
- Chaînes très longues ou avec caractères spéciaux ?

#### Coercions et types implicites
- Des comparaisons `==` au lieu de `===` qui masquent des différences de type ?
- Des opérations sur des types mixtes (string + number) ?
- Des conversions implicites via template strings ou concaténation ?

#### Concurrence et temps
- Des opérations asynchrones qui peuvent se terminer dans le désordre ?
- Des race conditions entre appels parallèles ?
- Des comportements différents selon la latence réseau ?
- Des timeouts non gérés ou mal configurés ?

#### Dépendances externes
- Que se passe-t-il si l'API externe retourne une structure inattendue ?
- Si elle retourne une erreur 5xx ? Une erreur 4xx ?
- Si elle répond après un timeout ?
- Si le circuit breaker est ouvert ?

#### Sécurité des entrées
- Des inputs utilisateur utilisés sans validation de longueur ?
- Des caractères d'échappement non traités ?
- Des données encodées / décodées incorrectement ?

### Étape 3 — Valider la complétude

Après l'analyse initiale :
1. Revisiter chaque classe d'edge case identifiée — des chemins supplémentaires dans la même classe ?
2. Ajouter tout nouveau chemin non géré découvert
3. Confirmer que les chemins gérés ont bien été ignorés (ne pas les inclure par erreur)

### Étape 4 — Produire le rapport

Rapporter **uniquement** les chemins non gérés.

```
## Analyse Edge Cases — <périmètre>

### Résumé
<Nombre de chemins non gérés trouvés, répartition par classe>

### Chemins non gérés

#### <Classe 1 : ex. "Valeurs nulles">

**[fichier:ligne]** `<contexte code>`
- Chemin non géré : <description précise du cas>
- Conséquence potentielle : <crash / comportement incorrect / sécurité>
- Suggestion : <correction minimale>

**[fichier:ligne]** `<contexte code>`
...

#### <Classe 2 : ex. "Erreurs API">
...

#### <Classe N>
...

### Chemins correctement gérés
<Optionnel — lister les patterns de gestion existants pour montrer ce qui a été vérifié>

### Couverture de tests manquante
<Tests à écrire pour couvrir les chemins non gérés identifiés>
```

---

## HALT CONDITIONS

- **HALT si le contenu est vide ou illisible** — demander le périmètre complet.
- Si le périmètre est très large (>500 LOC) et que l'analyse en une passe est incomplète, le signaler et proposer de découper en sous-périmètres.

---

## Différence avec la revue adversariale

| reviewer-adversarial | reviewer-edge-case |
|---|---|
| Cherche TOUS les types de problèmes | Cherche UNIQUEMENT les chemins non gérés |
| Évalue les choix d'architecture | N'évalue pas la conception, seulement la complétude |
| Posture : scepticisme global | Posture : chasse exhaustive et méthodique |
| Min 10 findings toutes catégories | Nombre de findings = nombre de chemins manquants |

Les deux peuvent être combinés pour une review ultra-complète.
