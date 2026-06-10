# ADR-013 — Fusion des agents developer-* en un agent developer générique

## Statut

Accepté — remplace [ADR-002](./002-developer-segmentation.fr.md)

## Contexte

L'ADR-002 avait divisé l'agent `developer.md` d'origine en 9 agents spécialisés pour
réduire le contexte injecté et permettre un routing précis. À l'époque, c'était le bon compromis :
chaque agent ne chargeait que les skills pertinents à son domaine.

Depuis, le hub a évolué significativement :

- **Architecture hybride des skills (ADR-010)** : introduction du Bucket B (native_skills),
  chargés à la demande par le LLM via l'outil `skill` — pas injectés au démarrage.
  La contrainte de "taille de contexte" de l'ADR-002 est désormais résolue au niveau architectural.
- **Injection dynamique de skills (ADR-008)** : les stacks sont injectées au moment de l'invocation,
  pas figées dans le fichier agent.
- **9 fichiers agents à structure identique** — mêmes permissions, même workflow Beads, mêmes
  skills Bucket A — créaient une charge de maintenance où tout correctif (format de handoff,
  étapes du workflow Beads, changement de permission) devait être appliqué 9 fois.

Le bénéfice de réduction de contexte de l'ADR-002 provient désormais du Bucket B (native_skills
chargés à la demande), non de l'existence de fichiers agents séparés. La segmentation a rempli
son rôle mais n'est plus le bon levier architectural.

## Décision

Les 9 agents `developer-*` sont fusionnés en un unique agent générique `developer`.
La spécialisation n'est plus encodée dans le fichier agent — elle est transmise au
**moment de l'invocation** par `orchestrator-dev` via le prompt, qui précise :

1. Le **domaine** (`frontend`, `backend`, `fullstack`, `api`, `mobile`, `data`, `devops`, `platform`, `security`)
2. Les **native_skills à charger** pour ce domaine (liste explicite par domaine dans le protocole de routing)

`developer-refactor` et `developer-migrator` restent des agents séparés — leurs workflows
sont fondamentalement différents (pas de nouvelles features, contraintes de sécurité spécifiques,
vérifications de préconditions).

## Conséquences

### Positives

- **Un seul fichier à maintenir** — tout changement au workflow Beads, aux permissions ou au format
  de handoff est appliqué en un seul endroit
- **Cohérence garantie** — la divergence de comportement entre agents (ex : un agent qui manque
  un correctif de permission) devient impossible
- **Extensibilité** — ajouter un nouveau domaine nécessite uniquement un nouvel entrée dans le
  protocole de routing et `stack-skills.json`, pas un nouveau fichier agent
- **Isolation parallèle confirmée** — quand `orchestrator-dev` invoque plusieurs agents `developer`
  en parallèle via `task`, chaque instance reçoit son propre contexte de session isolé.
  Les skills chargés dans une session ne fuient pas dans une autre.

### Négatives / compromis

- **Le prompt d'invocation est désormais le porteur du contexte de domaine** — si `orchestrator-dev`
  envoie un prompt malformé ou incomplet (domaine ou liste de skills manquants), l'agent manquera de
  spécialisation. Atténué par le format explicite défini dans `orchestrator-dev-protocol.md`.
- **Perte de granularité des descriptions au niveau agent** — le sélecteur d'agents OpenCode et les
  listes d'agents affichent désormais une seule entrée `developer` au lieu de 9 spécialisations.
  Acceptable car `developer` est un subagent (caché du sélecteur) et uniquement invoqué par
  `orchestrator-dev`.

## Alternatives rejetées

**Conserver les 9 agents, ajouter un skill de base partagé** : réduit la duplication mais ne
l'élimine pas — les 9 fichiers existent toujours, divergent dans le temps, nécessitent 9 mises à
jour pour chaque changement structurel.

**Conserver les agents spécialisés, charger tous leurs Bucket B skills de façon générique** :
va à l'encontre du principe de spécialisation — l'agent recevrait tous les standards de domaine à la fois.

## Impact

| Fichier | Action |
|---------|--------|
| `agents/developer/developer.md` | Créé |
| `agents/developer/developer-frontend.md` | Supprimé |
| `agents/developer/developer-backend.md` | Supprimé |
| `agents/developer/developer-fullstack.md` | Supprimé |
| `agents/developer/developer-api.md` | Supprimé |
| `agents/developer/developer-mobile.md` | Supprimé |
| `agents/developer/developer-data.md` | Supprimé |
| `agents/developer/developer-devops.md` | Supprimé |
| `agents/developer/developer-platform.md` | Supprimé |
| `agents/developer/developer-security.md` | Supprimé |
| `agents/developer/developer-refactor.md` | Conservé |
| `agents/developer/developer-migrator.md` | Conservé |
| `agents/planning/orchestrator-dev.md` | Mis à jour (table agents + permissions task) |
| `skills/orchestrator/orchestrator-dev-protocol.md` | Mis à jour (matrice de routing + format d'invocation) |
| `config/stack-skills.json` | Mis à jour (clé `_agent_scope`) |
