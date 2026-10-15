> [Read in English](retro-triage.en.md)

# Rétrospective et tri

Deux workflows pour apprendre de ce qui s'est passé et remettre de l'ordre dans le travail à faire : `retro` lit la trace de sessions passées et en tire des actions, `triage` trie un lot de tickets ou de constats sans rien écrire avant votre accord. Ils s'enchaînent : les actions d'une rétrospective se trient. Référence complète : [Workflows livrés](../reference/workflows.fr.md#retro).

## `retro` : apprendre des sessions passées

```bash
oh run retro                                   # la dernière session terminée du projet
oh run retro --parent ses_ab12                 # une session précise (celle dont on enchaîne)
oh run retro -i sessions="ticket 7d"           # les sessions ticket des 7 derniers jours
oh run retro -i sessions="ses_ab12 ses_cd34" -i focus="Pourquoi cp-2 a-t-il été refusé si souvent ?"
```

oh établit lui-même le résumé de la trace de chaque session (6 au plus, les plus récentes d'abord) : durée, coût total et par agent, checkpoints demandés plusieurs fois, décisions refusées avec votre consigne, commandes refusées, délégations. Le `debugger` ne lit que ces résumés et, pour éclairer une cause, le dépôt.

1. **Relevé** : chaque fait cite sa source (`ses_ab12 · Appels refusés · bash docker compose up -d db`). Ce que le résumé ne dit pas n'existe pas.
2. **Constats** : les faits regroupés par cause, comptés et chiffrés (relances, durée, dollars), avec une cause dans le système (une consigne, une permission, une étape du workflow, une convention du projet), **sans blâme** : aucune personne nommée, même si votre question en cite une. Une cause non prouvée est marquée « hypothèse ».
3. **Actions** : chacune dit ce qui change et le signe qui montrera, dans une prochaine trace, qu'elle a marché (« plus de refus de `docker compose` dans les sessions `ticket` »).
4. La page est écrite dans `docs/wiki/retros/<date>-<slug>.md` (sessions examinées, ce qui a bien marché, constats, actions).
5. **`cp-retro`** : vous validez ou demandez une correction ; ensuite seulement, la page est commitée, seule.
6. **`cp-actions`** : les actions deviennent des tickets si vous les validez ; un refus n'en crée aucun.

Deux pauses en semi-auto. Sans session à examiner, le debugger le dit et donne la commande pour relancer, sans rien écrire. À la fin, oh affiche la page et les tickets créés, et propose (sans le lancer) `triage` sur ces tickets.

Pour rejouer une rétrospective sur un résumé gardé : `oh run retro -i trace="$(cat resume.txt)"`.

## `triage` : trier sans surprise

```bash
oh run triage --tickets ev-1,ev-2,ev-3          # un lot précis
oh run triage -i query="status=open AND label=bug"
oh run triage -i findings="$(cat constats-revue.md)"   # des constats à rattacher ou à transformer en tickets
oh run triage                                   # les tickets ouverts du projet (30 au plus)
```

1. Le `planner` lit les tickets, les étiquettes du projet et les candidats doublons de `bd find-duplicates`.
2. Il trie : priorité changée seulement si le ticket le justifie, doublon seulement si les deux tickets décrivent le même problème au même endroit, étiquettes du projet seulement, ticket qui mêle plusieurs livrables découpé en tickets enfants, constat déjà couvert rattaché par un commentaire. Il ne touche ni au statut, ni à l'assignation, ni à la description.
3. **`cp-triage`** : le tableau de tous les changements (ticket, champ, avant → après, raison), les tickets à créer et les inchangés. **Avant votre validation, oh refuse toute écriture dans Beads** (création, priorité, étiquette, lien, commentaire, doublon), dans le shell comme dans la passerelle Beads. Un refus : rien n'est écrit.
4. Après validation, exactement les changements du tableau, et eux seuls.

Une pause en semi-auto. oh relève lui-même les tickets modifiés, créés et fermés comme doublons (sorties `updated`, `created`, `duplicates`), et propose `ticket` sur les tickets créés.

## Pour une couche d'équipe

- Le verrou `update` (modification de tickets) est nouveau : un workflow d'équipe peut le déclarer sur un checkpoint (`unlocks: [update]`) pour montrer des changements de tickets avant de les faire. Une version d'oh plus ancienne l'ignore (avertissement `unlock_unknown`) : mettez oh à jour sur tous les postes.
- `session.traces(<entrée>)` calcule les résumés de plusieurs sessions pour une entrée `text` d'un workflow d'équipe ; voir le [schéma](../reference/workflow-schema.fr.md).
