---
name: handoff-bloc-unique-rule
description: "Contrat universel de handoff — règles producer/consumer communes à tous les agents produisant un bloc structuré pour un orchestrator."
---

# Règle du bloc unique — Contrat universel de handoff

## Principe fondamental — producer

Quand tu es invoqué via `task` depuis un agent coordinateur, ton **seul output** est le bloc structuré défini dans ton skill `*-handoff-format`.

- **Aucun texte** avant, après, ou en dehors du bloc
- Le rapport / la spec / le récap est **intégré dans le bloc** (section dédiée), pas produit séparément en texte libre
- Le bloc est **auto-suffisant** — l'orchestrator doit pouvoir le traiter sans contexte supplémentaire

## Prohibitions — producer

❌ Ne jamais écrire de texte en dehors du bloc de handoff
❌ Ne jamais produire le rapport/spec comme texte libre avant le bloc
❌ Ne jamais omettre le `task_id` dans `### État de la session`
❌ Ne jamais omettre les champs obligatoires définis dans le skill `*-handoff-format`

## Champ obligatoire — `### Questions bloquantes`

Chaque bloc de handoff **doit** contenir une section `### Questions bloquantes` juste avant `### Statut`.

- **Si aucune question :** écrire `Aucune.`
- **Si des questions existent :** liste numérotée, chaque entrée avec contexte + options si possible
- Ce champ est **contractuellement bloquant** — le coordinateur ne peut pas poursuivre le workflow tant que les questions ne sont pas résolues

## Contrat — consumer (orchestrator)

À la réception d'un bloc de handoff :

1. **Vérifier les champs obligatoires** — si l'un est absent, demander au sous-agent de compléter
2. **Vérifier `### Questions bloquantes`** — si non-vide (pas `Aucune.`), **escalader les questions à l'utilisateur** avant de continuer. Ne jamais ignorer, ne jamais auto-résoudre.
3. **Retranscrire les champs de manière formatée** dans la discussion (voir skill `posture/retranscription-coordinateur`)
4. **Afficher intégralement** la section critique spécifique à l'agent (rapport de review, spec design, diagnostic, etc.)
5. **Ne jamais résumer** les sections techniques — les transmettre telles quelles
