# Template — Pages techniques du wiki (`docs/wiki/technical/`)

## Format canonique — `docs/wiki/technical/conventions.md`

```markdown
---
updated: YYYY-MM-DD
confidence: confirmed | mixed | uncertain
agents: [<agents>]
---

# Conventions — [Nom du projet]

## Linting & formatage
- <Convention 1> — `CONFIRMÉ` · <agent> · <date> · <fichier config>
- <Convention 2> — `DÉDUIT` · <agent> · <date>

## Nommage
- <Convention fichiers/dossiers> — `CONFIRMÉ` · <agent> · <date>
- <Convention variables/fonctions> — `CONFIRMÉ` · <agent> · <date>
- <Convention classes/interfaces> — `CONFIRMÉ` · <agent> · <date>

## Git
- <Format des commits> — `CONFIRMÉ` · <agent> · <date>
- <Convention de branches> — `DÉDUIT` · <agent> · <date>

## Configuration & secrets
- <Convention .env / secrets> — `CONFIRMÉ` · <agent> · <date>

## Patterns spécifiques à l'équipe
- <Pattern observé 1> — `CONFIRMÉ` · <agent> · <date> · <fichier:ligne>
- <Pattern observé 2> — `DÉDUIT` · <agent> · <date>

## À ne pas utiliser
- <Librairie/pattern explicitement exclu> — `CONFIRMÉ` · <agent> · <date>
```

---

## Format canonique — `docs/wiki/technical/architecture.md`

```markdown
---
updated: YYYY-MM-DD
confidence: confirmed | mixed | uncertain
agents: [<agents>]
---

# Architecture — [Nom du projet]

## Structure globale
<Monorepo / monolithe / microservices — avec justification observée>
— `CONFIRMÉ` · <agent> · <date>

## Découpage en couches
<Description du découpage observé — ex: Controller → Service → Repository>
— `CONFIRMÉ` · <agent> · <date> · <dossier>

## Communication entre modules
<HTTP, événements, queues, imports directs — selon ce qui est observé>
— `DÉDUIT` · <agent> · <date>

## Décisions architecturales notables
- <Décision 1 : pourquoi ce pattern a été choisi si documenté>
  — `CONFIRMÉ` · <agent> · <date> · <ADR ou fichier>
- <Décision 2>
  — `INCERTAIN` · <agent> · <date>

## Points de fragilité connus
- <Zone fragile 1> — `CONFIRMÉ` · <agent> · <date> · <fichier>
```

---

## Format canonique — `docs/wiki/technical/stack.md`

```markdown
---
updated: YYYY-MM-DD
confidence: confirmed | mixed | uncertain
agents: [<agents>]
---

# Stack — [Nom du projet]

## Dépendances principales

| Catégorie | Technologie | Version | Notes |
|-----------|-------------|---------|-------|
| Langage | TypeScript | 5.x | |
| Framework | Vue 3 | 3.4.x | |
| BDD | PostgreSQL | 15 | ORM : Prisma |

— `CONFIRMÉ` · <agent> · <date> · package.json

## Librairies clés

- `<lib>` — <rôle dans le projet> — `CONFIRMÉ` · <agent> · <date>
- `<lib>` — <rôle dans le projet> — `CONFIRMÉ` · <agent> · <date>

## Variables d'environnement requises

- `<VAR>` — <description> — `CONFIRMÉ` · <agent> · <date> · .env.example

## Contraintes de version

- <Contrainte 1 si applicable> — `CONFIRMÉ` · <agent> · <date>
```

---

## Format canonique — `docs/wiki/technical/tests.md`

```markdown
---
updated: YYYY-MM-DD
confidence: confirmed | mixed | uncertain
agents: [<agents>]
---

# Stratégie de tests — [Nom du projet]

## Frameworks

- Unitaires : <framework> — `CONFIRMÉ` · <agent> · <date> · <fichier config>
- E2E : <framework ou "Aucun"> — `CONFIRMÉ` · <agent> · <date>

## Organisation

- <Co-localisés `.spec.ts` à côté du code / dossier `tests/` séparé>
  — `CONFIRMÉ` · <agent> · <date>

## Seuil de couverture

- <X% configuré dans <fichier> / Non configuré>
  — `CONFIRMÉ` · <agent> · <date> · <fichier config>

## Philosophie

- <TDD / test-after / BDD — avec signe observé>
  — `DÉDUIT` · <agent> · <date>

## Commandes

```bash
# Tests unitaires
<commande>

# Tests E2E
<commande ou "Non configuré">

# Couverture
<commande ou "Non configuré">
```
— `CONFIRMÉ` · <agent> · <date> · package.json

## Conventions de nommage des tests

- <Convention observée> — `CONFIRMÉ` · <agent> · <date> · <fichier exemple>
```

---

## Format canonique — `docs/wiki/technical/review-rules.md`

Page **optionnelle** — créée quand le projet a des règles de review spécifiques qui ne sont pas
couvertes par les conventions générales ou les standards de développement. Consultée par l'agent
`reviewer` à chaque review (si elle existe) pour éviter les faux positifs et calibrer les findings.

```markdown
---
updated: YYYY-MM-DD
confidence: confirmed | mixed
agents: [documentarian, reviewer]
---

# Règles de review — [Nom du projet]

## Fichiers exclus (supplémentaires)

<Patterns de fichiers spécifiques au projet à exclure de la review, en complément
des exclusions par défaut du review-protocol (lock files, generated, minified, build output)>

- `<pattern>` — <raison>

## Patterns intentionnels — ne pas signaler

<Pratiques qui pourraient sembler inhabituelles ou contraires aux standards génériques
mais qui sont des choix délibérés du projet. Le reviewer NE DOIT PAS les signaler comme findings.>

- **<Pattern>** — <justification> — `CONFIRMÉ` · <agent> · <date> · <fichier(s) source>

## Points d'attention spécifiques

<Ce que le reviewer doit toujours vérifier sur ce projet, au-delà de la checklist standard.>

- <Règle spécifique> — <contexte / raison>

## Seuils du projet

<Seuils spécifiques qui overrident les standards génériques du review-protocol.>

| Seuil | Standard générique | Seuil projet | Raison |
|-------|--------------------|-------------|--------|
| <ex: Longueur max fonction> | <30 lignes> | <50 lignes> | <ex: handlers de migration complexes> |
```
