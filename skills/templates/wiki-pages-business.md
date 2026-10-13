# Template — Pages métier du wiki (`docs/wiki/business/`)

## Format canonique — `docs/wiki/business/index.md`

```markdown
---
updated: YYYY-MM-DD
confidence: confirmed | mixed | uncertain
agents: [<agents>]
---

# Domaines métier — [Nom du projet]

## Vue d'ensemble

<2-3 phrases décrivant le domaine principal et les utilisateurs cibles>
— `CONFIRMÉ` · <agent> · <date>

## Domaines documentés

| Domaine | Fichier | Concepts clés |
|---------|---------|---------------|
| <domain> | [business/<domain>.md](business/<domain>.md) | <Concept A, Concept B> |

## Concepts transversaux

- <Concept présent dans plusieurs domaines> — lié à [<domain1>](business/<domain1>.md) et [<domain2>](business/<domain2>.md)
  — `CONFIRMÉ` · <agent> · <date>

## Utilisateurs cibles

- <Rôle utilisateur 1> — <description courte>
  — `CONFIRMÉ` · <agent> · <date>
```

---

## Format canonique — `docs/wiki/business/<domain>.md`

```markdown
---
updated: YYYY-MM-DD
confidence: confirmed | mixed | uncertain
agents: [<agents>]
---

# [Domaine] — [Nom du projet]

## Rôle de ce domaine
<1-2 phrases : ce que ce domaine gère, son périmètre dans le projet>
— `CONFIRMÉ` · <agent> · <date>

## Entités clés

- `<Entité>` — <description courte + où elle est définie>
  — `CONFIRMÉ` · <agent> · <date> · <fichier>

## Règles de gestion

- <Règle 1 — ex: "Un utilisateur ne peut avoir qu'un seul rôle actif">
  — `CONFIRMÉ` · <agent> · <date> · <fichier:ligne>

- <Règle 2>
  — `DÉDUIT` · <agent> · <date>

## Flux principaux

1. <Flux 1 — ex: "Inscription → Vérification email → Activation compte">
   — `CONFIRMÉ` · <agent> · <date>

## Risques et points d'attention

- <Risque ou zone fragile identifiée>
  — `CONFIRMÉ` · <agent> · <date> · <fichier>

## Liens vers les autres domaines

- Dépend de : [<domain>](business/<domain>.md) — <raison>
- Utilisé par : [<domain>](business/<domain>.md) — <raison>
```
