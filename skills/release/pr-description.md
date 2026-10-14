---
name: pr-description
description: "Utiliser pour écrire le titre et la description d'une merge request ou pull request (de release ou non) : pourquoi, ce qui change, ruptures, vérifications, risques, liens."
---

# Skill — Description de merge request

La description sert au relecteur qui ne connaît pas ton travail, et à celui qui lira l'historique dans un an. Elle dit ce qui change et pourquoi, et montre ce qui a été vérifié.

## Sources

- Commits de la branche : `git log <base>..HEAD --format='%h %s%n%b'` ; volume : `git diff --stat <base>...HEAD`.
- CHANGELOG (section de la version ou `[Unreleased]`), tickets cités dans les commits.
- La langue et la forme des descriptions déjà écrites dans le projet (modèle `.gitlab/merge_request_templates/`, `.github/pull_request_template.md`) : un modèle du projet prime sur la structure ci-dessous, ses cases à cocher comprises.

## Titre

Une ligne, à l'impératif, 72 caractères au plus, au format des commits du projet. Release : `chore(release): vX.Y.Z`.

## Corps

```markdown
## Pourquoi
Le besoin ou le problème, en deux ou trois phrases ; le ticket s'il existe.

## Ce qui change
Les changements visibles, groupés par thème (pas la liste des fichiers).

## Ruptures et migration
Chaque rupture et ce que l'utilisateur doit faire ; « Aucune » sinon.

## Vérifications
Les commandes lancées et leur résultat ; ce qui n'a pas été vérifié, dit comme tel.

## Risques et retour arrière
Ce qui peut casser, comment le voir, comment revenir en arrière.

## Liens
Tickets, CHANGELOG, documentation.
```

Pour une **release**, ajoute la version et son incrément (avec la raison), le tag, et ce que la fusion déclenche (publication par la CI, image, paquet).

## Règles

- Rien d'inventé : un test non lancé n'est pas « vérifié », un risque inconnu se dit « non évalué ».
- Une rupture n'est jamais noyée dans « Ce qui change » : elle a sa section.
- Pas d'emoji, pas de formule creuse (« cette MR améliore le code »).

## Créer la merge request

Seulement une fois son ouverture déverrouillée, et seulement si la branche existe sur le dépôt distant (`git rev-parse --abbrev-ref @{upstream}` répond). Sinon, donne à l'utilisateur la description et la commande à lancer après son push :

- GitLab : `glab mr create --title "<titre>" --description "<corps>" --target-branch <base>`
- GitHub : `gh pr create --title "<titre>" --body "<corps>" --base <base>`
