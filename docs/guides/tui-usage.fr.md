# Guide — Utiliser le TUI OpenHub

> Guide pratique pour naviguer et travailler dans l'interface TUI.

## Démarrage rapide

```bash
oh
```

Le TUI s'affiche avec le menu à gauche et la vue Home au centre.

## Workflow type

### 1. Naviguer

- Utilisez `Ctrl+N` pour placer le focus sur le menu
- Naviguez avec `j`/`k` (ou flèches)
- Dépliez une catégorie avec `l` ou `Space`
- Sélectionnez avec `Enter`

### 2. Lancer une session de code

1. Menu → Sessions → Start
2. Choisissez le mode : Standard, Dev, ou Onboard
3. Le TUI se suspend, opencode démarre
4. Travaillez dans opencode normalement
5. Quittez opencode → le TUI reprend

### 3. Gérer vos projets

1. Menu → Projets → Liste
2. `a` pour ajouter un nouveau projet (nom + chemin)
3. `d` pour supprimer un projet
4. Menu → Projets → Deploy pour déployer les agents sur le projet actif
5. Menu → Projets → Sync pour synchroniser tous les projets

### 4. Modifier la configuration

1. Menu → Configuration → Hub
2. Naviguez avec `j`/`k` dans la table des clés
3. `Enter` sur une clé → saisissez la nouvelle valeur
4. La modification est sauvegardée immédiatement dans `~/.oh/hub.toml`

### 5. Vérifier la santé du système

1. Menu → Système → Doctor
2. Les checks s'exécutent automatiquement
3. `r` pour relancer les vérifications

### 6. Recherche rapide (Command Palette)

À tout moment, appuyez sur `Ctrl+P` :
- Tapez le nom d'une commande ("start", "doctor", "mcp"...)
- La liste se filtre en temps réel (fuzzy search)
- `Enter` pour exécuter, `Esc` pour fermer

## Astuces

- `Esc` vous ramène toujours en arrière (vue précédente)
- `?` ouvre l'aide à tout moment
- `Ctrl+Q` quitte proprement le TUI
- Les toasts en haut à droite confirment les actions (succès/erreur)
- Le breadcrumb dans le header indique toujours votre position
