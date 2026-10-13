---
name: beads-dev
description: "Utiliser pour travailler un ticket Beads en tant que developer : réserver, passer en review, bloquer, commenter, commit puis fermeture."
---

## Workflow obligatoire

```
1. ticket donné dans ton prompt          → c'est celui que tu traites (sans prompt de ticket : bd ready --label ai-delegated --json)
2. bd show <ID>                          → lire le détail (description, acceptance, notes, commentaires)
3. bd update <ID> --claim                → clamer avant de commencer
4. [implémenter + tester]
5. bd update <ID> --add-label ready-for-review → prêt pour la review (le statut reste in_progress), rendre la main (ni commit, ni clôture)
6. [review, puis checkpoint « Commit ou correction » validé par l'utilisateur]
   → commit  : instruction de commit de ton coordinateur → git commit -m "..." puis bd close <ID> --reason "..." --suggest-next
   → corriger : bd comments add <ID> "Retours reviewer : ..." + bd update <ID> --remove-label ready-for-review + corriger + retour étape 5
   → pre-review échouée : bd comments add <ID> "Pre-review échouée : ..." + corriger + retour étape 5
```

`bd update <ID> --claim` est atomique (il échoue si un autre acteur a déjà réclamé le ticket) : il met le statut à `in_progress` et t'assigne le ticket. Le label `ready-for-review` signale la review (Beads n'a pas de statut `review` sans configuration du projet).

## Commit et fermeture (instruction de commit de ton coordinateur)

Quand ton coordinateur te transmet l'instruction de commit (le checkpoint « Commit ou correction » est validé), dans cet ordre :

```bash
git commit -m "<type>(<scope>): <description> (<ID>)"
bd close <ID> --reason "Implemented in commit <hash>" --suggest-next
```

Jamais de commit ni de clôture avant cette instruction : oh refuse `git commit` et `bd close` tant que le checkpoint n'est pas validé, et refuse la clôture tant que le travail n'est pas commité. Si le commit échoue (hook, conflit), ne clos pas le ticket et ne contourne pas les hooks : signale l'erreur à ton coordinateur. `--suggest-next` affiche les tickets débloqués par la clôture.

Tu ne modifies pas la documentation toi-même : les patterns, conventions ou contraintes découverts se proposent avec `wiki_propose` (skill `living-docs-enrichment`), leurs `wp-…` cités dans `### Enrichissements wiki proposés` de ton rapport ; sans l'outil, liste-les dans cette section.

## Corrections demandées (prompt de re-délégation)

- **Retours du reviewer** : `bd comments add <ID> "Retours reviewer : <contenu intégral des corrections requises>"` — copiés tels quels depuis le prompt reçu, jamais résumés ni reformulés ; `bd update <ID> --remove-label ready-for-review` ; corriger ; `bd update <ID> --add-label ready-for-review`.
- **Pre-review échouée** : `bd comments add <ID> "Pre-review échouée : <check> : <message d'erreur>"` ; corriger ; repasser en review (`--add-label ready-for-review`).
- Le nombre de cycles de correction suit `limits.max_fix_rounds` (règle générée de ta carte du workflow) ; à la limite, signale le blocage à ton coordinateur.

## Blocage

```bash
bd update <ID> -s blocked
bd comments add <ID> "Bloqué par : <raison>"
bd update <ID> --add-label needs-decision       # en attente d'une décision humaine
bd update <ID> --add-label needs-clarification   # description insuffisante
```

Blocage résolu → `bd update <ID> -s in_progress`. Les commentaires (`bd comments add <ID> "…"`) tracent décisions et échanges sans toucher la description ni les notes.

## Règles strictes

- Toujours `bd show <ID>` avant d'implémenter — ne jamais supposer le contenu d'un ticket
- Toujours clamer avant d'implémenter — évite les conflits si plusieurs agents tournent
- Toujours ajouter le label `ready-for-review` après implémentation — ne jamais clore directement
- Toujours clore explicitement après l'instruction de commit, juste après le commit — ne pas laisser de tickets `in_progress` orphelins
- Ne pas modifier le titre ou la description d'un ticket sans y être invité
- Un ticket `closed` ou `cancelled` n'est jamais rouvert — créer un nouveau ticket si nécessaire
- Le label `ai-delegated` ne sert que quand tu cherches toi-même un ticket (`bd ready`, sans ticket dans le prompt) ; **l'humain décide quels tickets déléguer** — tu n'ajoutes jamais ce label toi-même
