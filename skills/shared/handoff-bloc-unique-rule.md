---
name: handoff-bloc-unique-rule
description: "Utiliser quand tu produis un bloc de retour vers un coordinateur : un seul bloc, Questions bloquantes, task_id."
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

## Rapport déposé dans oh — `workflow_report`

Si l'outil `workflow_report` est disponible :

1. Dépose ton rapport complet **avant** d'écrire le bloc : `workflow_report` avec `title` et `body` (Markdown, avec les sections de ton format).
2. Dans le bloc, remplace la section du rapport (`### Rapport complet`, `### Changements par fichier`…) par la ligne `Rapport : rp-N` (l'identifiant reçu). Garde les autres champs obligatoires : verdict, statut, corrections requises, questions bloquantes, état de la session.
3. Si l'outil est absent, ou s'il répond « non enregistré » ou refuse le rapport : inclus le rapport complet dans le bloc, sans identifiant.

Côté consumer, `Rapport : rp-N` vaut section présente : oh joint ce rapport à la carte du checkpoint suivant. Cite l'identifiant (et passe-le dans `reports` de `workflow_checkpoint`) au lieu de le recopier ; `workflow_report_get` le relit au besoin, section par section.

## Contrat — consumer (orchestrator)

À la réception d'un bloc de handoff :

1. **Vérifier les champs obligatoires** — si l'un est absent, demander au sous-agent de compléter
2. **Vérifier `### Questions bloquantes`** — si non-vide (pas `Aucune.`), **escalader les questions à l'utilisateur** avant de continuer. Ne jamais ignorer, ne jamais auto-résoudre.
3. **Retranscrire les champs de manière formatée** dans la discussion (voir skill `posture/retranscription-coordinateur`)
4. **Afficher intégralement** la section critique spécifique à l'agent (rapport de review, spec design, diagnostic, etc.)
5. **Ne jamais résumer** les sections techniques — les transmettre telles quelles
