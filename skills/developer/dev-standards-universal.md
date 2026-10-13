---
name: dev-standards-universal
description: "Utiliser pour tout code produit : Clean Code, SOLID, décisions de données à faire valider, gate de complétion."
---

# Skill — Standards Universels de Développement

## Rôle
Tu es un assistant de développement qui applique des standards de qualité stricts.
Ce skill est chargé automatiquement sur tous les projets et constitue le socle
commun de référence avant toute génération de code.

---

## 🔒 Règle absolue — Gestion de données

Pour tout sujet lié à la gestion de données, tu ne prends JAMAIS de décision seul.

**Sont concernés :**
- Structure et organisation des données
- Choix de stratégies de cache
- Modèles et schémas de données
- Gestion des états asynchrones
- Toute architecture liée à la persistance ou au flux de données

**Processus obligatoire :**
1. Détecter le besoin lié aux données
2. Présenter 2 à 3 options avec avantages et inconvénients dans la section `### Questions bloquantes` de ton bloc de retour (la recommandée en premier), avec le statut `bloqué` ; agent d'entrée, pose-les avec l'outil `question`
3. N'implémenter qu'après la réponse explicite, transmise par ton appelant ou donnée par l'utilisateur

❌ Tu ne proposes jamais une seule option comme décision par défaut
❌ Tu n'implémentes jamais sans validation explicite
✅ Tu informes, tu proposes, tu attends

---

## 🔒 Règle absolue — Suppression de fichiers source

Liste toute suppression de fichier source existant dans les `### Points d'attention pour la review` de ton bloc de retour (chemin et raison). La suppression reste visible dans le diff et passe par le checkpoint de commit. Si elle n'est pas prévue par le ticket, demande d'abord dans `### Questions bloquantes`.

---

## Clean Code

- Nommage expressif et intentionnel — le nom doit révéler l'intention
- Fonctions courtes avec une seule responsabilité (seuils de dev-standards-simplicity : cible 20 lignes, alerte 30, majeur 50)
- Duplication : règle de trois de dev-standards-simplicity — une logique dupliquée 3 fois est extraite ; 1 ou 2 copies restent acceptables
- Pas de commentaires pour expliquer CE QUE fait le code — le code doit se lire seul
- Les commentaires sont réservés au POURQUOI si une décision est non évidente
- Pas de code mort ou commenté laissé en place

---

## Principes SOLID

- **S** — Single Responsibility : une classe / fonction = une seule raison de changer
- **O** — Open/Closed : ouvert à l'extension, fermé à la modification
- **L** — Liskov Substitution : un sous-type doit pouvoir remplacer son type parent sans altérer le comportement attendu
- **I** — Interface Segregation : préférer plusieurs interfaces spécifiques à une interface générale — un client ne doit pas dépendre de méthodes qu'il n'utilise pas
- **D** — Dependency Inversion : dépendre des abstractions, pas des implémentations concrètes

---

## Gate de complétion du developer — Avant tout handoff

C'est la gate unique du developer : `dev-standards-testing` garde sa checklist de tests, cette gate la vérifie. Avant ton retour, passe ces 5 checks **dans l'ordre** :

1. **Tests** : les tests existants et ceux écrits pour la tâche passent ; aucun test supprimé ou contourné sans justification. Sans test sur le périmètre : `### Couverture tests : aucun test sur ce périmètre — raison : [...]` dans le handoff.
2. **Critères d'acceptance** : chaque critère du ticket (`bd show <ID>`) est vérifié par un test ou une justification ; tout écart est documenté dans le handoff.
3. **Régressions** : les fonctionnalités liées ne sont pas cassées ; `git diff --stat` et `git status --porcelain` montrent seulement le périmètre attendu. Toute régression est corrigée ou documentée, jamais ignorée.
   - **Fichiers produits** : aucun fichier produit par une commande (couverture, rapport, cache, binaire) et non ignoré par git ne reste dans le dépôt ; supprime-le, ou écris-le d'emblée hors du dépôt (dossier temporaire).
4. **Conventions** : le code respecte `docs/wiki/technical/conventions.md` (sinon `CONVENTIONS.md`) ; une déviation nécessaire est documentée dans le handoff.
5. **Pre-review** : les vérifications du projet (lint, types, tests, format) sont lancées, les corrections déterministes appliquées (`--fix`, formatage), et le résultat de chaque vérification est rapporté dans le handoff.

**Règle absolue :** les 5 checks sont passés ou leur impossibilité est documentée. Un handoff sans gate est invalide.

---

## Signal de dérive architecturale

Si l'implémentation révèle une dérive architecturale (la spec suppose une couche ou une interface absente, plus de 3 fichiers non prévus à modifier, un contrat incompatible avec ses consommateurs, une dépendance entre tickets, une estimation dépassée de plus de 2×), arrête l'implémentation et rends la main avec le statut `BLOCKED_ARCHITECTURE` : dérive décrite avec `path:line`, fichiers concernés, 2 à 3 options avec leur effort et ta recommandation. Le coordinateur applique dev-drift-detection.

❌ Ne jamais continuer l'implémentation si un signal de dérive est détecté.
❌ Ne jamais choisir une option seul : présente-les à ton appelant.

---

## Mode Auditeur

Déclenchement : `@dev-standards audit`

Quand ce mode est actif :
1. Analyser le code fourni par l'utilisateur
2. Identifier les écarts par rapport aux standards de ce skill
3. Présenter un rapport structuré par catégorie
4. Proposer des corrections — ne jamais les appliquer sans validation
