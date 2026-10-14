---
name: codebase-design
description: "Utiliser quand il faut choisir une conception dans un code existant (module, frontière, dépendance, interface) : contraintes du code relevées, deux options comparées, choix justifié."
---

# Concevoir dans un code existant

Une conception se choisit d'abord contre le code qui existe déjà, ensuite seulement selon le goût. Ce skill sert à recommander où placer un module, une interface ou une dépendance, et à le justifier par des faits vérifiables.

## 1. Relever les contraintes avant d'imaginer

Note chaque contrainte avec sa source (`fichier:ligne`). Cherche dans cet ordre :

1. **Règles outillées** (dures : la CI les fait respecter) : configuration de lint qui restreint les imports (`.golangci.yml` et `depguard`, `no-restricted-imports` d'ESLint, `import-linter`, `dependency-cruiser`, ArchUnit), tests d'architecture (`arch_test.go`, `*ArchTest*`), étapes de CI. Liste aussi les fichiers cachés de la racine : `read` du dossier, ou `glob` sur `.*`.
2. **Règles écrites** : ADR, `architecture.md`, `CONVENTIONS.md`, pages du wiki.
3. **Règles de fait** : le sens actuel des imports (`grep` des `import`), l'endroit où vivent les interfaces, le point d'assemblage (`main`, conteneur d'injection) ; un motif présent dans 3 fichiers ou plus est une convention.

❌ Jamais de recommandation avant cette recherche.
❌ Une option qui viole une règle outillée est écartée, même si l'utilisateur la préfère.

## 2. Deux options, vraiment différentes

Esquisse au moins deux conceptions qui diffèrent sur un choix structurant (où vit l'interface, dans quel sens va la dépendance, quel module porte la règle), pas sur un nom. Si la demande en propose deux, compare celles-là ; ajoute une troisième seulement si les deux violent une contrainte. Pour chacune : signatures publiques, modules touchés, dépendances ajoutées (`de → vers`).

## 3. Comparer

| Critère | Question |
|---|---|
| Contraintes (§1) | Respectée ? Une violation d'une règle outillée est éliminatoire |
| Sens des dépendances | Le cœur métier ignore-t-il les détails (I/O, frameworks, transport) ? Aucun cycle ? |
| Profondeur | Petite interface qui cache beaucoup, plutôt que large interface qui expose les détails |
| Coût | Fichiers touchés, tests à écrire, migration, réversibilité |
| Cohérence | Suit-elle les motifs déjà en place ? |

## 4. Recommander

```
**Recommandation : option <X>.**
- Contrainte décisive : <règle> (`<fichier>:<ligne>`)
- Pourquoi pas <Y> : <une phrase>
- Compromis accepté : <ce qu'on paie>
- À vérifier pendant l'implémentation : <risque>
```

- Préférence de l'utilisateur contraire à une contrainte : dis-le franchement, cite la règle, et propose la variante la plus proche de son intention qui la respecte.
- Aucune contrainte trouvée : écris-le (« aucune règle d'import outillée ni écrite ») et appuie le choix sur le §3.
- Contrainte supposée mais non vérifiée : marque-la comme hypothèse, jamais comme fait.

## Ce que ce skill ne couvre pas

- Concevoir deux fois une interface utilisateur : c'est le designer, avec son protocole UI.
- Choisir une librairie ou une stack externe : `research`.
- Déléguer un besoin UX/UI au designer depuis le planner : ses gabarits de délégation.
