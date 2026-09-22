---
name: handoff-bloc-unique-rule
description: "Contrat universel de handoff — règles producer/consumer communes à tous les agents produisant un bloc structuré pour un orchestrator."
bucket: B
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

## Contrat — consumer (orchestrator)

À la réception d'un bloc de handoff :

1. **Vérifier les champs obligatoires** — si l'un est absent, demander au sous-agent de compléter
2. **Retranscrire les champs de manière formatée** dans la discussion (voir skill `posture/retranscription-coordinateur`)
3. **Afficher intégralement** la section critique spécifique à l'agent (rapport de review, spec design, diagnostic, etc.)
4. **Ne jamais résumer** les sections techniques — les transmettre telles quelles
