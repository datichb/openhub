# Guide — Utiliser le TUI OpenHub

> Guide pratique pour naviguer et travailler dans l'interface TUI.

## Démarrage rapide

```bash
oh
```

Le TUI affiche un écran d'accueil avec des hints de démarrage rapide et l'omnibar en bas.

## Workflow principal

### 1. Lancer une commande

À tout moment, appuyez sur `Ctrl+P` ou commencez à taper :
- L'omnibar s'active
- Tapez un nom de commande (recherche fuzzy)
- `Enter` pour exécuter

### 2. Lancer une session de code

```
> start
```

Sélectionnez le mode (Standard, Dev, Onboard). Le TUI se suspend, opencode démarre. Quand vous quittez opencode, le TUI reprend.

### 3. Accès rapide aux actions spécifiques

Tapez la commande directement — le fuzzy matching la trouve vite :
```
> secu        → lance un audit sécurité
> dep         → déploie sur le projet actif
> doc         → ouvre le diagnostic doctor
```

### 4. Gérer les projets

```
> projects
```

Dans la vue projets :
- `a` pour ajouter un nouveau projet
- `d` pour supprimer
- `Enter` pour configurer
- `r` pour renommer

### 5. Modifier la configuration

```
> config
```

Naviguez avec `j`/`k`, appuyez sur `Enter` pour modifier une valeur.

### 6. Vérifier la santé du système

```
> doctor
```

Les checks s'exécutent automatiquement. `r` pour relancer.

## Astuces

- **Toute lettre active l'omnibar** — pas besoin de `Ctrl+P` si la vue n'utilise pas cette touche
- `Esc` ramène toujours en arrière (vue précédente, ou ferme l'omnibar)
- `Ctrl+Q` quitte à tout moment
- Les toasts (en haut à droite) confirment les résultats d'actions
- Toutes les commandes supportent le fuzzy matching — tapez des mots partiels, abréviations ou alias
- Les vues ont des raccourcis contextuels affichés dans le texte passif de l'omnibar

## Types de sessions

| Type | Description |
|------|-------------|
| Start Standard | Session interactive opencode |
| Start Dev | Session orientée développement (workflow ticket) |
| Start Onboard | Session d'onboarding projet |
| Audit (6 types) | Audit de code spécialisé avec prompts dédiés |
| Review (4 modes) | Code review avec profondeur et focus variés |
| Debug | Session debug avec description du problème |
| Quick | Lancement direct d'opencode (sans sélection) |
