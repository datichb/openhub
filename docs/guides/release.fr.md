> [Read in English](release.en.md)

# Préparer une release

Le workflow `release` prépare une release du projet : il choisit la version, date le CHANGELOG, écrit le commit de release et le tag annoté, et, si vous le demandez, la description de la merge request de release. Vous validez tout à un seul checkpoint, **avant** le moindre commit ou tag. oh ne pousse jamais : la publication reste votre geste.

```bash
oh run release                          # version déduite des commits et du CHANGELOG
oh run release -i version=5.0.0         # version imposée
oh run release -i mr=true -i target=main  # avec la description de la MR de release
```

## Avant le lancement : les préconditions

oh vérifie le dépôt avant de démarrer la session et refuse le lancement si :

- l'arbre de travail a des modifications non commitées (fichiers modifiés, indexés ou non suivis ; `.beads/` exclu) — le message liste les premières ;
- la branche courante n'est ni `main`, ni `master`, ni `release/*` (un HEAD détaché est refusé aussi).

Sans `CHANGELOG.md` à la racine, oh avertit seulement : le release-manager le créera. Ces tests (`git_clean`, `branch`, `tools`) sont disponibles pour tout workflow : voir le [schéma](../reference/workflow-schema.fr.md#préconditions).

## Pendant la session

1. Le `release-manager` relève le dernier tag (`git describe --tags`), les commits depuis ce tag (corps compris, pour les pieds `BREAKING CHANGE:`), la section `[Unreleased]` et les fichiers où la version est écrite.
2. Il présente à **`cp-release`**, sans rien écrire : la version et sa raison (une rupture donne une majeure, une fonctionnalité une mineure, un correctif seul un correctif ; en `0.y.z`, une rupture donne une mineure), la section du CHANGELOG telle qu'elle sera écrite, le message du tag, les notes de release et, avec `mr=true`, le titre et la description de la MR.
3. Vous validez, corrigez (« fix : publie la 5.0.0 ») ou refusez. Un refus : rien n'est écrit.
4. Après la validation : CHANGELOG (et fichiers de version, chacun vous est demandé), commit de release, tag annoté sur ce commit, puis la MR si la branche existe déjà sur le dépôt distant.
5. Le rapport donne les commandes de push et la façon d'annuler.

## Ce qu'oh verrouille

| Opération | Avant `cp-release` | Après `cp-release` |
|---|---|---|
| `git commit` | refusé | permis jusqu'au commit de release |
| `git tag` (création, liste comprise ; `git push --tags`, `update-ref refs/tags/…`, `gh release create`, `glab release create`) | refusé | permis **une fois la release commitée** : le tag marque toujours le commit de release |
| ouverture de MR (`glab mr create`, `gh pr create`, leurs API, outil MCP `create_mr`) | refusée | permise |
| `git push` | refusé | refusé (le release-manager n'a pas le droit de pousser) |
| `git tag -f`, `git tag -d` | refusés | refusés |

Le tag et la MR sont refusés dans tout autre workflow, même à un agent qui aurait le droit de lancer `git tag` : seul un workflow `risk: publish` peut les déverrouiller. Un workflow `publish` ne tourne jamais en distant ; son checkpoint de publication fait une pause dans tous les modes.

## Pourquoi oh ne pousse pas

Pousser un tag publie la release : la CI (goreleaser pour oh) construit et diffuse les binaires, et un tag poussé ne se retire pas proprement. C'est la seule étape vraiment irréversible : elle reste entre vos mains, après relecture du commit et du tag en local.

```bash
git show --stat HEAD && git cat-file -p v5.0.0   # relire
git push origin main                              # la branche d'abord
git push origin v5.0.0                            # puis ce tag seul (jamais --tags)
```

Annuler avant le push : `git tag -d v5.0.0`, puis `git reset --soft HEAD~1`.

## La merge request de release

Avec `mr=true`, la description suit le skill `pr-description` : pourquoi, ce qui change, ruptures et migration, vérifications, risques et retour arrière, liens. Si la branche n'est pas encore sur le dépôt distant, le release-manager ne crée rien : il vous donne la description et la commande à lancer après votre push (`glab mr create …` ou `gh pr create …`).

## Recette : tester sur une copie, sans risque

Pour essayer le workflow sur un vrai projet sans toucher au dépôt :

```bash
git clone /chemin/du/projet /tmp/release-test
cd /tmp/release-test
git remote remove origin                 # plus aucun dépôt distant : rien ne peut partir
oh run release                           # ou -i version=X.Y.Z
git tag -l && git show --stat HEAD       # tag local et commit de release
```

À vérifier : le lancement est refusé après `echo x >> README.md` (arbre sale) et sur une branche `feat/x` ; aucune commande `git tag` ni `git commit` ne passe avant la validation de `cp-release` ; le tag est annoté (`git cat-file -t <tag>` répond `tag`) et pointe sur `HEAD` ; rien n'est poussé.
