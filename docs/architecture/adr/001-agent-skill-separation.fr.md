# ADR-001 — Séparation agent / skill

## Statut

Accepté — **Évolué par [ADR-010](./010-hybrid-skills-architecture.fr.md)** et **[ADR-016](./016-execution-path-skills.fr.md)**

La séparation fondamentale (identité agent vs protocole skill) reste valide. L'ADR-010 étend le modèle de déploiement : les skills sont désormais divisées en deux buckets — Bucket A (inline, toujours actif via `skills:`) et Bucket B (natif, à la demande via `native_skills:`). L'ADR-016 ajoute un nouveau type de skill Bucket B : les **skills de parcours d'exécution** (`-standalone` et `-subagent`), chargées selon le contexte d'invocation plutôt que selon le domaine technique.

## Contexte

Lors de la conception du hub, deux approches étaient possibles pour définir le comportement
d'un agent IA : mettre toute la logique directement dans le fichier agent, ou séparer
l'identité de l'agent (qui il est, ce qu'il fait) de son protocole (comment il le fait).

Le premier prototype concentrait tout dans le fichier agent. Cela rendait les fichiers
longs (200-300 lignes), difficiles à maintenir, et empêchait la réutilisation des
protocoles entre agents. Par exemple, le format de rapport de review était dupliqué
dans le reviewer et dans l'orchestrateur.

## Décision

Le comportement d'un agent est divisé en deux couches :

- **Agent (`agents/<id>.md`)** : identité, rôle, ce qu'il fait / ne fait pas, workflow
  condensé, exemples d'invocation. Fichier court (~40-80 lignes).
- **Skill (`skills/<domaine>/<nom>.md`)** : protocole détaillé, formats de sortie,
  checklists, règles de comportement, exemples complets. Fichier de référence (~100-300 lignes).

Les skills sont déclarés dans le frontmatter de l'agent via la clé `skills: [...]`.
Le système hub assemble agent + skills au moment du déploiement.

## Conséquences

### Positives

- Un skill peut être partagé entre plusieurs agents (ex: `dev-standards-universal`
  est injecté dans tous les agents développeurs et dans le reviewer)
- Les agents restent lisibles et modifiables rapidement
- Les protocoles évoluent indépendamment des agents
- La séparation identité / comportement facilite la composition

### Négatives / compromis

- Un agent sans son skill est incomplet — il faut toujours déployer les deux ensemble
- La logique est répartie sur deux fichiers, ce qui peut dérouter au premier abord
- Nécessite de connaître la structure `skills/` pour comprendre le comportement réel

## Alternatives rejetées

**Tout dans l'agent** : fichiers trop longs, pas de réutilisation, maintenance difficile.

**Skills comme imports Markdown** : techniquement possible mais non supporté nativement
par les outils cibles (OpenCode, OpenCode) — l'assemblage par le hub est la seule
approche portable.
