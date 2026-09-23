# Sweep Task Workflow

## Contexte

Tu exécutes une sous-tâche d'un **sweep parallèle**. Plusieurs agents travaillent
simultanément sur le même projet, chacun dans son périmètre (scope) assigné.

Le sweep mode décompose un objectif haut niveau en sous-tâches indépendantes
qui sont exécutées en parallèle dans des worktrees git séparés.

## Objectif

Ton objectif est précisé dans le prompt sous **"Objectif global"** et **"Ta sous-tâche"**.
Ton périmètre de fichiers est indiqué sous **"Scope"**.

## Règles impératives

### 1. Restriction de scope
- **Ne modifie QUE les fichiers listés dans ton scope assigné.**
- Si tu identifies un problème hors scope, note-le dans un commentaire TODO mais ne le corrige pas.
- Les fichiers hors scope sont potentiellement modifiés par un autre agent en parallèle.

### 2. Fichiers partagés
- **Minimise les changements** sur les fichiers partagés :
  - `go.mod`, `go.sum`
  - `package.json`, `package-lock.json`, `yarn.lock`
  - Fichiers de configuration racine (`.eslintrc`, `tsconfig.json`, etc.)
- Si tu dois modifier un fichier partagé, fais-le de manière additive (pas de suppression).

### 3. Vérification
- Lance les tests dans ton scope avant de terminer :
  - Go : `go test ./<ton-scope>/...`
  - JS/TS : `npm test -- --scope=<ton-scope>`
- Si les tests ne passent pas, corrige les erreurs avant de terminer.
- Si tu ne peux pas corriger, documente l'échec dans un commit.

### 4. Commits
- Utilise le format **Conventional Commits** : `feat:`, `fix:`, `refactor:`, `test:`, `chore:`
- Chaque commit doit être atomique et compilable.
- Inclus le scope dans le message : `feat(auth): add token validation`

### 5. Format de sortie
- Termine ta session par un **résumé** des fichiers modifiés et des actions effectuées.
- Indique clairement si les tests passent ou non.
- Signale tout conflit potentiel avec d'autres scopes.

## Anti-patterns

- Ne modifie PAS de fichiers hors de ton scope.
- N'ajoute PAS de dépendances sans nécessité absolue.
- Ne reformate PAS du code hors scope (même si le linter le suggère).
- Ne fais PAS de refactoring opportuniste hors scope.
- Ne supprime PAS de code utilisé par d'autres packages.

## Gestion des conflits

Si tu reçois une notification `[PARALLEL-NOTIFICATION]` signalant un conflit :
1. Vérifie si le fichier en conflit est dans ton scope.
2. Si oui, minimise tes changements sur ce fichier spécifique.
3. Si non, ignore la notification — l'autre agent est responsable.

## Exemple de session

```
[SWEEP MODE] Tu fais partie d'un sweep parallèle.

Objectif global : Ajouter des tests unitaires manquants
Ta sous-tâche : Ajouter les tests pour le package auth
Scope : internal/auth

→ Tu ne modifies que les fichiers dans internal/auth/
→ Tu lances go test ./internal/auth/... avant de terminer
→ Tu commits avec : test(auth): add unit tests for token validation
```
