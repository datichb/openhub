---
name: doc-standards
description: "Utiliser pour juger ou structurer une documentation : Diataxis, lisibilité, anti-patterns, critères par type."
---

# Skill — Standards de Documentation

## Framework Diataxis

Diataxis est un cadre de structuration de la documentation en 4 quadrants,
selon deux axes : **apprentissage vs travail** et **théorie vs pratique**.

```
                    APPRENTISSAGE
                         │
          Tutoriels       │       Guides pratiques
       (apprendre en      │       (accomplir une
         faisant)         │          tâche)
                          │
THÉORIE ──────────────────┼──────────────────── PRATIQUE
                          │
          Explication     │       Référence
       (comprendre        │       (information
         pourquoi)        │        précise)
                          │
                       TRAVAIL
```

### Les 4 quadrants

| Type | Question du lecteur | Exemple | Caractéristiques |
|------|-------------------|---------|-----------------|
| **Tutoriel** | "Comment apprendre ?" | "Créer sa première API REST" | Guidé, pas à pas, résultat garanti, orienté apprentissage |
| **Guide pratique** | "Comment faire X ?" | "Comment configurer l'authentification SSO" | Orienté tâche, suppose une connaissance de base |
| **Référence** | "Qu'est-ce que X ?" | "Documentation des endpoints API" | Exhaustif, précis, consulté ponctuellement |
| **Explication** | "Pourquoi X ?" | "Architecture de la couche de cache" | Contexte, décisions, compromis, alternatives |

**Règle principale : ne pas mélanger les quadrants dans un même document.**
Un README peut référencer les 4 types, mais chaque section doit être clairement identifiée.

---

## Principes de documentation lisible

### 1. Une page = un objectif

Chaque document doit répondre à une seule question principale.
Si le lecteur doit faire deux choses différentes, créer deux documents.

### 2. Progressive disclosure

Présenter l'information du plus simple au plus complexe :
1. Ce que c'est (une phrase)
2. Comment l'utiliser maintenant (exemple minimal)
3. Les détails et cas avancés (référence)

### 3. Exemples avant théorie

````markdown
# Mauvais
La fonction `formatDate` prend un objet Date JavaScript et applique
le formatage selon la locale ISO 8601 avec les options...

# Bien
```js
formatDate(new Date('2024-01-15')) // → "15 janvier 2024"
formatDate(new Date('2024-01-15'), { locale: 'en' }) // → "January 15, 2024"
```
Formate une date selon la locale du projet. Par défaut : `fr-FR`.
````

### 4. Test du nouveau contributeur

Avant de livrer une documentation, se poser la question :
> "Un développeur qui rejoint le projet aujourd'hui peut-il suivre ce guide
> sans aide extérieure ?"

Si la réponse est non, identifier ce qui manque.

### 5. Éviter le style "commit message"

```markdown
# Mauvais
Added user authentication. Updated API. Fixed bugs.

# Bien
## Authentification utilisateur

Le système utilise JWT pour l'authentification. Les tokens expirent après 24h.
Pour s'authentifier : POST /api/auth/login avec { email, password }.
```

---

## Structure type par document

### README.md

````markdown
# Nom du projet

Description en 1-2 phrases — ce que le projet fait, pour qui.

## Prérequis

- Node.js >= 18
- Docker

## Installation

```bash
git clone ...
npm install
cp .env.example .env
npm run dev
```

## Utilisation rapide

[Exemple minimal en < 10 lignes]

## Documentation

- [Guide de contribution](CONTRIBUTING.md)
- [Architecture](docs/architecture/overview.fr.md)
- [API Reference](docs/api/)
- [Changelog](CHANGELOG.md)

## Licence

MIT
````

### Guide pratique (how-to)

````markdown
# Comment [accomplir la tâche]

## Prérequis

[Ce dont on a besoin avant de commencer]

## Étapes

1. [Action concrète]
   ```bash
   commande --exemple
   ```

2. [Action suivante]

## Résultat attendu

[Ce qu'on voit quand ça fonctionne]

## Dépannage

[Erreurs fréquentes et solutions]
````

### Documentation de référence

````markdown
# [Nom du composant / module / commande]

[Description en 1 phrase]

## Paramètres / Options

| Paramètre | Type | Défaut | Description |
|-----------|------|--------|-------------|
| `name` | string | — | Nom de l'utilisateur |
| `locale` | string | `fr` | Langue de formatage |

## Valeur de retour

[Type et description]

## Exemples

```js
// Cas nominal
exemple()

// Cas avec options
exemple({ option: true })
```

## Voir aussi

- [Lien vers concept lié]
````

---

## Anti-patterns courants

### Doc qui dit "quoi" sans dire "pourquoi"

```markdown
# Mauvais
Nous utilisons Redis pour le cache.

# Bien
Nous utilisons Redis pour le cache de sessions utilisateurs (TTL : 24h).
Ce choix permet de partager l'état entre plusieurs instances sans affecter
la base de données principale. Alternative rejetée : cache mémoire in-process
(incompatible avec le déploiement multi-instances).
```

### Doc qui duplique le code

La documentation qui recopie l'implémentation se désynchronise immédiatement.
Documenter le **comportement observable**, pas l'implémentation interne.

```markdown
# Mauvais
La fonction itère sur le tableau avec un for loop, vérifie chaque élément...

# Bien
Retourne les utilisateurs actifs, triés par date de création (plus récent en premier).
```

### Doc jamais mise à jour

Symptômes :
- Versions dans la doc ≠ versions réelles
- Commandes qui ne fonctionnent plus
- Screenshots périmés

Remède : documenter uniquement ce qui est stable. Pour ce qui change souvent,
pointer vers la source de vérité (fichier de config, code source).

### Jargon sans glossaire

Si le projet a un vocabulaire métier spécifique, maintenir un glossaire.
Ne jamais supposer que le lecteur connaît les acronymes internes.

### Structure trop profonde

```
# Mauvais
docs/
  technical/
    backend/
      services/
        authentication/
          jwt/
            README.md   ← introuvable

# Bien
docs/
  guides/
    authentication.md
```

---

## Critères de qualité — checklist finale

Avant de livrer un document :

- [ ] Le titre répond à la question "Qu'est-ce que je vais apprendre/accomplir ?"
- [ ] La première section explique en 1-2 phrases ce que contient le document
- [ ] Au moins un exemple concret est présent
- [ ] Les commandes sont toutes copiables et fonctionnelles
- [ ] Les termes techniques sont définis ou ont un lien vers leur définition
- [ ] Un développeur nouveau peut suivre le guide sans aide extérieure
- [ ] Le document ne mélange pas les quadrants Diataxis (tutoriel ≠ référence)
- [ ] La langue est cohérente avec le reste du projet

---

## Documentation fonctionnelle

La documentation fonctionnelle s'adresse aux non-développeurs (product, métier, support).

### Principes spécifiques

- Pas de jargon technique — si nécessaire, expliquer entre parenthèses
- Orientée résultat : "L'utilisateur peut faire X" plutôt que "Le système fait Y"
- Avec des captures d'écran ou des exemples de flux quand c'est possible
- Glossaire métier obligatoire si le domaine est spécialisé

### Format type — description de fonctionnalité

```markdown
# [Nom de la fonctionnalité]

## À quoi ça sert

[1-2 phrases orientées valeur utilisateur]

## Qui peut l'utiliser

[Rôles / profils concernés]

## Comment ça fonctionne

[Description du flux principal — sans détails techniques]

## Cas d'usage

- **[Cas 1]** : [Description courte]
- **[Cas 2]** : [Description courte]

## Limitations connues

[Ce que la fonctionnalité ne fait pas — évite les surprises]
```
