# Template — Pages racine du wiki (`index.md`, `ONBOARDING.md`)

## Format canonique — `docs/wiki/index.md`

```markdown
---
updated: YYYY-MM-DD
confidence: confirmed | mixed | uncertain
agents: [<agents ayant contribué>]
---

# [Nom du projet] — Index Wiki

## Stack critique
<3-5 lignes condensées : langages, frameworks principaux, BDD — les éléments
qui conditionnent tout le reste>
— `CONFIRMÉ` · <agent> · <date> · <fichier>

## Architecture (résumé)
<2-3 lignes : pattern dominant, découpage, communication entre couches>
— `CONFIRMÉ` · <agent> · <date>

## God nodes — concepts les plus connectés

| Concept | Pages liées | Criticité |
|---------|-------------|-----------|
| <Concept A> | [technical/architecture.md#section](), [business/domain.md]() | Critique |
| <Concept B> | [technical/conventions.md#section](), [business/other.md]() | Haute |

*(Vide si aucun concept n'apparaît dans ≥ 2 pages)*

## Carte des domaines métier

- [<domain>](business/<domain>.md) — <description courte en 1 phrase>

*(Vide si aucun domaine métier détecté)*

## Points critiques actifs 🔴

- <Point critique 1> — `CONFIRMÉ` · <agent> · <date>

*(Vide si aucun point critique actif)*

## Zones d'ombre

- <Zone non résolue 1>
- <Zone non résolue 2>

*(Vide si tout est documenté)*
```

---

## Format canonique — `ONBOARDING.md` (racine, minimaliste)

```markdown
# [Nom du projet]

> Documentation vivante disponible dans [`docs/wiki/index.md`](docs/wiki/index.md)

## Démarrage rapide

<Commande(s) de démarrage — 1-3 lignes maximum>

## Liens

- [Index wiki](docs/wiki/index.md) — vue globale, god nodes, points critiques
- [Conventions](docs/wiki/technical/conventions.md)
- [Architecture](docs/wiki/technical/architecture.md)
```
