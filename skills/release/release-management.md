---
name: release-management
description: "Utiliser pour préparer une release : choisir la version SemVer d'après les commits et le CHANGELOG, dater la section, écrire le tag annoté et les notes, vérifier avant de rendre la main."
---

# Skill — Préparer une release

Une release fige un état du projet sous un numéro que d'autres vont lire et installer. Chaque étape se vérifie sur le dépôt, jamais de mémoire.

## 1. Partir du dernier tag publié

- `git describe --tags --abbrev=0` donne le dernier tag atteignable ; `git for-each-ref --sort=-v:refname --format='%(refname:short)' refs/tags` les liste tous. Garde le **format du projet** (`v1.4.2`, `1.4.2`, `paquet@1.4.2`).
- Aucun tag : la dernière version datée du CHANGELOG sert de point de départ ; sans elle, propose `0.1.0` (ou `1.0.0` si l'utilisateur dit l'API stable) et dis-le dans la proposition.

## 2. Lire tous les changements, corps compris

- `git log <tag>..HEAD --format='%h %s%n%b%n---'` : le **corps** des commits porte les pieds `BREAKING CHANGE:` (ou `BREAKING-CHANGE:`) qu'un `--oneline` cache.
- Lis aussi la section `[Unreleased]` du CHANGELOG : elle peut annoncer une rupture qu'aucun sujet de commit ne montre (`Removed`, « ⚠ », changement de comportement).
- Beaucoup de commits ou une longue section : une sortie tronquée cache des ruptures. Cherche-les explicitement : `git log <tag>..HEAD -i -E --grep 'BREAKING[ -]CHANGE' --grep '^[a-z]+(\(.+\))?!:' --format='%h %s'`, et dans la section, l'outil `grep` sur `⚠|BREAKING|incompatible|changement de comportement|### Removed`.

## 3. Choisir l'incrément

Le changement le plus fort décide :

| Ce que tu trouves | À partir de 1.0.0 | En 0.y.z |
|---|---|---|
| Rupture : `type!:`, pied `BREAKING CHANGE`, suppression ou changement incompatible annoncé | majeure (`2.0.0`) | mineure (`0.5.0`) |
| Fonctionnalité : `feat`, section `Added` | mineure | mineure |
| Correctif seul : `fix`, `perf`, `Security`, `Fixed` | correctif (`1.4.3`) | correctif |
| Rien de visible (`docs`, `chore`, `ci`, `test`, `refactor`) | pas de release : dis-le et arrête-toi | idem |

- Une rupture annoncée (« ⚠ », « changement de comportement », `Removed`, pied `BREAKING CHANGE`) compte comme telle : ne la requalifie pas en changement mineur de ton propre chef. Si tu doutes qu'elle touche les utilisateurs, propose quand même l'incrément de rupture et cite les lignes qui te font douter : l'utilisateur tranche au checkpoint.
- La version demandée par l'utilisateur prime ; si elle contredit les changements (correctif demandé, rupture trouvée), signale-le dans la proposition avec la preuve (commit, ligne).
- Une préversion (`-rc.1`, `-beta.2`) seulement si elle est demandée ; la suivante incrémente son dernier nombre.
- Le tag ne doit pas exister : `git rev-parse -q --verify refs/tags/<tag>` ne renvoie rien.

## 4. Mettre le CHANGELOG d'accord avec les commits

- Chaque changement visible (fonctionnalité, correctif, rupture, sécurité) a sa ligne ; ajoute celles qui manquent, signalées comme telles dans la proposition. Une ligne sans commit correspondant se signale, elle ne se supprime pas.
- Une rupture se dit dans `Changed` ou `Removed`, avec ce que l'utilisateur doit faire pour migrer.
- `[Unreleased]` devient `## [X.Y.Z] - AAAA-MM-JJ` (date du jour, `date +%Y-%m-%d`, au format des titres déjà présents) ; une section `## [Unreleased]` vide reste au-dessus.
- Liens de comparaison en bas du fichier, s'il y en a : `[Unreleased]` compare le nouveau tag à `HEAD`, `[X.Y.Z]` le tag précédent au nouveau.
- Les sections déjà publiées ne changent jamais.

## 5. Fichiers de version

La version peut être écrite ailleurs que dans le tag : cherche la version actuelle dans tout le dépôt (outil `grep` sur `1.4.2`, sans filtre d'extension : un script n'en a souvent pas ; hors CHANGELOG) — `package.json`, `pyproject.toml`, `Cargo.toml`, `Chart.yaml`, `VERSION`, constante dans le code ou un script. Chaque occurrence qui désigne la version du projet passe à la nouvelle, dans le commit de release ; nomme ces fichiers dans la proposition. Une version lue dans le tag (goreleaser, `git describe`) ne s'écrit nulle part.

## 6. Commit, puis tag annoté

- Commit de release au format des commits du projet (souvent `chore(release): vX.Y.Z`), avec le CHANGELOG et les fichiers de version seulement.
- Tag **annoté**, sur ce commit, avec un message court sur une seule ligne : `git tag -a <tag> -m "Release <tag>"` (au plus un second `-m` d'une ligne). Les notes complètes restent dans le CHANGELOG : un long message sur plusieurs lignes peut être refusé par les garde-fous d'oh, qui lisent toute la commande. Jamais de tag léger, jamais `-f`, jamais un tag existant déplacé.
- Vérifie : `git cat-file -t <tag>` répond `tag` ; `git rev-parse <tag>^{commit}` égale `git rev-parse HEAD` ; `git status` est propre.

## 7. Notes de release

Écrites pour qui installe, tirées de la section du CHANGELOG : d'abord les ruptures et leur migration, puis les nouveautés, puis les correctifs. Pas de commit interne, pas de liste de fichiers.

## 8. Rendre la main

- Commandes de push à lancer par l'utilisateur, la branche puis **ce tag seul** : `git push origin <branche>`, puis `git push origin <tag>` (jamais `--tags`, qui pousse aussi des tags locaux oubliés). <!-- capability: user -->
- Annuler avant le push : `git tag -d <tag>`, puis `git reset --soft HEAD~1` pour reprendre le commit de release. <!-- capability: user -->
