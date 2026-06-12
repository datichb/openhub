---
name: wiki-navigation
description: Protocole de navigation du wiki documentaire vivant — lecture de l'index en premier, chargement ciblé des pages pertinentes, jamais le wiki en entier. Actif pour tous les agents qui consultent le contexte d'un projet.
---

# Skill — Navigation du Wiki Documentaire Vivant

## Rôle

Ce skill définit le protocole que tous les agents doivent suivre pour consulter
la documentation vivante d'un projet. Le wiki remplace les anciens fichiers plats
`ONBOARDING.md`, `CONVENTIONS.md` et `docs/context/`.

L'objectif est d'économiser les tokens en chargeant **uniquement les pages pertinentes**
à la tâche courante, jamais le wiki en entier.

---

## Structure du wiki (rappel)

```
docs/wiki/
├── index.md                    ← toujours lu en premier
├── technical/
│   ├── architecture.md
│   ├── stack.md
│   ├── tests.md
│   └── conventions.md
└── business/
    ├── index.md
    └── <domain>.md
ONBOARDING.md                   ← résumé minimaliste, redirige vers docs/wiki/index.md
```

---

## Protocole de navigation — 4 règles absolues

### Règle 1 — Toujours lire `docs/wiki/index.md` en premier

Avant toute action sur un projet, vérifier si `docs/wiki/index.md` existe.

**Si le fichier existe :**
1. Le lire immédiatement via l'outil `Read`
2. Mémoriser : stack critique, architecture, god nodes, domaines métier, points critiques actifs
3. Utiliser ces informations pour décider quelles pages charger ensuite

**Si le fichier n'existe pas :**
- Ne rien faire — le wiki n'est pas encore généré pour ce projet
- Continuer le workflow normal sans contexte wiki
- Ne pas signaler l'absence comme une erreur (l'onboarder génère le wiki)

### Règle 2 — Charger uniquement la page pertinente à la tâche

Après avoir lu `index.md`, identifier quelle page charger selon la nature de la tâche :

| Nature de la tâche | Page à charger |
|--------------------|----------------|
| Implémentation code, patterns, nommage | `docs/wiki/technical/conventions.md` |
| Architecture, découpage, décisions techniques | `docs/wiki/technical/architecture.md` |
| Stack, dépendances, versions, librairies | `docs/wiki/technical/stack.md` |
| Tests, couverture, stratégie de test | `docs/wiki/technical/tests.md` |
| Logique métier d'un domaine spécifique | `docs/wiki/business/<domain>.md` |
| Vue globale des domaines métier | `docs/wiki/business/index.md` |
| Contexte général du projet | `index.md` suffit |

**Si la tâche touche à plusieurs domaines :** charger chaque page concernée,
une par une, uniquement si nécessaire.

### Règle 3 — Ne jamais lire le wiki en entier par défaut

❌ Ne jamais faire `ls docs/wiki/` puis lire tous les fichiers
❌ Ne jamais charger `technical/` et `business/` en parallèle sans raison
✅ Toujours partir de `index.md` → décider → charger une page

**Exception :** lors d'un re-onboarding ou d'une mise à jour complète du wiki,
le `documentarian` et l'`onboarder` peuvent lire plusieurs pages. Cela reste
une exception explicite, pas le comportement par défaut.

### Règle 4 — Suivre les références des god nodes

Quand `index.md` indique qu'un concept est un god node avec plusieurs pages liées,
ne charger les pages liées que si la tâche courante concerne directement ce concept.

Exemple : si `AuthService` est un god node lié à `technical/architecture.md#auth`
et `business/auth.md`, charger `business/auth.md` uniquement si la tâche
porte sur la logique métier de l'authentification.

---

## Décision de navigation — arbre rapide

```
La tâche concerne...
│
├── ...les règles de code (nommage, patterns, linting)
│   → docs/wiki/technical/conventions.md
│
├── ...l'architecture ou les décisions techniques
│   → docs/wiki/technical/architecture.md
│
├── ...la stack ou les dépendances
│   → docs/wiki/technical/stack.md
│
├── ...les tests ou la stratégie de qualité
│   → docs/wiki/technical/tests.md
│
├── ...un domaine métier précis (auth, billing, orders...)
│   → docs/wiki/business/<domain>.md
│   (le nom du domaine est dans la table des domaines de index.md)
│
├── ...plusieurs domaines à la fois
│   → charger chaque page concernée séquentiellement
│
└── ...le contexte général du projet
    → index.md suffit
```

---

## Interprétation des tags de confiance

Chaque enrichissement dans le wiki porte un tag de confiance. Lors de la lecture,
interpréter ainsi :

| Tag | Signification | Comportement recommandé |
|-----|--------------|------------------------|
| `` `CONFIRMÉ` `` | Observation directe dans le code (fichier + ligne citée) | Faire confiance, utiliser directement |
| `` `DÉDUIT` `` | Raisonnement contextuel, fichier cité sans ligne précise | Faire confiance, vérifier si la tâche est sensible |
| `` `INCERTAIN` `` | Hypothèse à valider par l'équipe | Vérifier dans le code avant d'utiliser |

Quand un enrichissement `INCERTAIN` est pertinent pour la tâche courante,
lire le fichier source mentionné pour confirmer avant d'agir.

---

## Mise à jour des god nodes — règle pour le documentarian

Après chaque enrichissement d'une page wiki, le `documentarian` doit réévaluer
le tableau des god nodes dans `docs/wiki/index.md` :

**Algorithme :**
1. Recenser les concepts mentionnés dans la page qui vient d'être modifiée
2. Pour chaque concept, compter dans combien de pages wiki distinctes il apparaît
3. Si un concept apparaît dans **≥ 2 pages distinctes** et n'est pas encore dans le tableau → l'ajouter
4. Si un concept existant est désormais cité dans plus de pages → mettre à jour la colonne "Pages liées"
5. Mettre à jour le frontmatter `updated` de `index.md` si le tableau a changé

**Criticité d'un god node :**
- `Critique` — concept cité dans ≥ 4 pages OU mentionné explicitement dans "Points critiques actifs"
- `Haute` — concept cité dans 3 pages
- `Normale` — concept cité dans 2 pages

---

## Autocontrôle avant toute action sur un projet

**Vérification rapide :**

| Question | Si oui | Si non |
|----------|--------|--------|
| `docs/wiki/index.md` existe ? | Le lire maintenant | Continuer sans wiki |
| Ma tâche est identifiée dans le tableau de navigation ? | Charger la page correspondante | Charger `index.md` suffit |
| La page chargée mentionne un god node pertinent ? | Charger la page liée si nécessaire | Continuer avec ce qui est chargé |
| Un enrichissement `INCERTAIN` est pertinent ? | Vérifier dans le code source | Utiliser l'enrichissement directement |
