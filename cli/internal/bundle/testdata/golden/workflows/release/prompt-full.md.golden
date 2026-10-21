Mode de workflow : semi-auto
Langue de réponse : fr

Prépare la release du projet demo (/src/demo). oh a vérifié au lancement que l'arbre de travail est propre et que la branche est une branche de release.

1. Charge `release-management` avec l'outil `skill`, puis `pr-description`. Relève le dernier tag, les commits depuis ce tag et la section `[Unreleased]` du CHANGELOG. Rien de visible à publier : dis-le et termine, sans checkpoint.
2. Prépare, **sans rien écrire** : la version demandée (ci-dessous) et sa raison, la section du CHANGELOG telle qu'elle sera écrite (datée du jour), le message du tag annoté et les notes de release, le titre et la description de la merge request de release vers la branche cible (ci-dessous).
3. Appelle aussitôt `workflow_checkpoint` (`id: "cp-release"`, `summary` : tout ce que tu as préparé à l'étape 2). Avant sa validation, oh refuse le commit, le tag et la merge request. Une correction : reprends la proposition et rappelle `cp-release`. Un refus : n'écris rien et termine.
4. Après la validation, applique exactement la proposition validée : CHANGELOG, commit de release, puis tag annoté sur ce commit (oh refuse le tag tant que la release n'est pas commitée), puis la merge request si la branche est déjà sur le dépôt distant. Lance chaque commande seule (sans `cd`, sans tube ni `&&`), hooks compris, jamais `--no-verify`.
5. **Ne pousse rien** : ni branche ni tag. Termine par le rapport : version, commit, tag vérifié (annoté, sur `HEAD`), commandes de push à lancer par l'utilisateur, description complète de la merge request et commande pour l'ouvrir si tu n'as pas pu le faire, et comment annuler avant le push.

Version demandée — écrite par l'utilisateur lui-même, à vérifier contre les changements :
Exemple de valeur pour version

Branche cible de la merge request — écrite par l'utilisateur lui-même :
Exemple de valeur pour target

Consignes de l'utilisateur pour les notes — écrites par l'utilisateur lui-même :
Exemple de valeur pour notes
