# Marketplace de Skills - Guide

> 🇬🇧 [Read in English](skill-marketplace.en.md)

## Vue d'ensemble

Le marketplace de skills permet à la communauté d'étendre `oh` avec des skills d'agents personnalisées — sans modifier le binaire. Les skills communautaires sont téléchargées depuis le [oh-skills-index](https://github.com/datichb/oh-skills-index) ou directement depuis une URL Git, puis déployées automatiquement avec les skills intégrées lors de `oh deploy`.

### Pourquoi des skills communautaires ?

Les skills intégrées couvrent les workflows principaux (planner, pathfinder, onboarder, etc.). Les skills communautaires ajoutent :
- Des idiomes spécifiques à un langage ou framework (`golang-idioms`, `react-patterns`)
- Des protocoles spécifiques à un domaine (`fintech-compliance`, `accessibility-audit`)
- Des adaptateurs d'intégration au-delà du set intégré
- Des skills internes à une équipe distribuées via des repos Git privés

---

## Installer une Skill

### Depuis l'index (recommandé)

```bash
oh skill add golang-idioms
```

Cette commande :
1. Cherche `golang-idioms` dans l'index des skills
2. Télécharge le package de la skill dans `~/.oh/skills/golang-idioms/`
3. Valide le checksum du manifest
4. Marque la skill comme active

La skill est déployée sur tes projets au prochain `oh deploy`.

### Depuis une URL Git

```bash
oh skill add https://github.com/monorg/ma-oh-skill
```

Supporte tout dépôt Git public ou privé (SSH). Le dépôt doit contenir un `manifest.json` valide à sa racine.

### Épingler une version

```bash
oh skill add golang-idioms@1.2.0
oh skill add golang-idioms@latest  # par défaut
```

---

## Lister les Skills Installées

```bash
oh skill list
```

Exemple de sortie :

```
SKILL                  VERSION  SOURCE   STATUS
golang-idioms          1.2.0    index    active
react-patterns         0.8.1    git      active
fintech-compliance     2.0.0    index    inactive
```

---

## Rechercher dans l'Index

```bash
oh skill search go
oh skill search --tags backend,testing
oh skill search --author monorg
```

Retourne une liste paginée de skills correspondantes avec nom, description, version et auteur.

---

## Supprimer une Skill

```bash
oh skill remove golang-idioms
```

La skill est dissociée des déploiements mais ses fichiers restent dans `~/.oh/skills/golang-idioms/` jusqu'à purge :

```bash
oh skill remove golang-idioms --purge
```

---

## Comment ça Fonctionne

1. **Téléchargement** : `oh skill add` récupère le package de la skill depuis l'index ou Git et le stocke dans `~/.oh/skills/<nom>/`
2. **Validation** : le checksum du manifest est vérifié avant activation
3. **Déploiement** : `oh deploy` lit toutes les skills actives et les inclut dans le `opencode.json` généré aux côtés des skills intégrées
4. **Accès agent** : les skills apparaissent dans la liste de skills de l'agent au démarrage de session, indiscernables des skills intégrées

Les skills sont des fichiers SKILL.md sans état — elles ne modifient pas le binaire et peuvent être supprimées en toute sécurité à tout moment.

---

## Créer une Skill Communautaire

### Format du package

Un package de skill est un répertoire (ou la racine d'un repo Git) contenant au minimum :

```
ma-skill/
├── manifest.json    # Obligatoire : métadonnées du package
└── SKILL.md         # Obligatoire : instructions de la skill (chargées par les agents)
```

Des fichiers supplémentaires (exemples, tests, sous-skills) peuvent être inclus ; seul `SKILL.md` est chargé par les agents.

### Structure de SKILL.md

Suis les mêmes conventions que les skills intégrées. Le fichier doit contenir :
- Un `# Titre` clair décrivant l'objectif de la skill
- `## Quand utiliser` — conditions dans lesquelles l'agent doit activer cette skill
- `## Instructions` — guide étape par étape
- `## Exemples` optionnel avec des paires entrée/sortie concrètes

Voir [authoring-skills.md](./authoring-skills.md) pour le guide complet d'authoring.

### Versioning

Utilise le **versioning sémantique** (`majeur.mineur.patch`) :
- `patch` — corrections de bugs et clarifications
- `minor` — nouvelles instructions ou exemples (rétrocompatible)
- `major` — changements cassants de l'interface ou du comportement de la skill

---

## Format de manifest.json

```json
{
  "name": "golang-idioms",
  "description": "Patterns Go idiomatiques et anti-patterns pour l'agent code-writer",
  "version": "1.2.0",
  "author": "monorg",
  "skill_file": "SKILL.md",
  "tags": ["go", "golang", "backend", "idioms"],
  "min_oh_version": "0.8.0",
  "homepage": "https://github.com/monorg/golang-idioms-skill",
  "license": "MIT"
}
```

| Champ | Requis | Description |
|-------|--------|-------------|
| `name` | Oui | Identifiant unique de la skill (kebab-case) |
| `description` | Oui | Description d'une ligne affichée dans les résultats de recherche |
| `version` | Oui | Chaîne de version sémantique |
| `author` | Oui | Nom d'utilisateur GitHub ou organisation |
| `skill_file` | Oui | Chemin vers le fichier SKILL.md (relatif à la racine du package) |
| `tags` | Non | Tableau de tags recherchables |
| `min_oh_version` | Non | Version minimale de `oh` requise |
| `homepage` | Non | URL vers la documentation ou les sources |
| `license` | Non | Identifiant de licence SPDX |

---

## Publier dans l'Index

Pour rendre ta skill disponible via `oh skill search` et `oh skill add <nom>` :

1. Héberge ta skill dans un dépôt GitHub public
2. Assure-toi que `manifest.json` et `SKILL.md` sont à la racine du dépôt
3. Crée un tag git correspondant à la version dans `manifest.json` (ex. `v1.2.0`)
4. Soumets une PR sur [https://github.com/datichb/oh-skills-index](https://github.com/datichb/oh-skills-index) en ajoutant ton entrée de skill dans `index.json`

Le template de PR inclut une checklist : manifest valide, SKILL.md présent, version taguée, pas d'instructions malveillantes.

---

## Bonnes Pratiques d'Authoring

Voir le guide complet d'authoring : [authoring-skills.md](./authoring-skills.md)

Principes clés :
- **Sois spécifique** : les skills qui s'activent dans des conditions larges diluent la concentration de l'agent
- **Fournis des exemples** : les paires entrée/sortie concrètes réduisent les hallucinations de l'agent
- **Responsabilité unique** : une skill = un domaine ; compose via plusieurs skills plutôt qu'un fichier monolithique
- **Versionne avec soin** : les changements cassants nécessitent un bump de version majeur pour éviter de casser les déploiements existants
- **Teste avant de publier** : utilise `oh skill test <nom-skill>` pour exécuter la skill sur des sessions d'exemple

---

## Ressources

- [Dépôt oh-skills-index](https://github.com/datichb/oh-skills-index)
- [authoring-skills.md](./authoring-skills.md) — guide complet d'authoring de skills
- [Protocole MCP](https://modelcontextprotocol.io/)
