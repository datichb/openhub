---
name: resolving-merge-conflicts
description: "Utiliser quand le dépôt a un conflit de merge, rebase ou cherry-pick (marqueurs <<<<<<<, même déjà commités) : garder les deux changements, chercher les conflits sans marqueur, vérifier."
---

# Skill — Résoudre un conflit sans rien perdre

Un conflit, c'est deux changements voulus sur les mêmes lignes. La bonne résolution fait les **deux** ; choisir un côté entier n'est correct que si l'autre est devenu inutile, et il faut alors pouvoir le dire.

## 1. Comprendre l'état

- `git status` : opération en cours (merge, rebase, cherry-pick) et fichiers en conflit. Merge déjà commité avec ses marqueurs : `git show --stat HEAD` (deux parents) et cherche les marqueurs dans tout le dépôt (`<<<<<<<`, `=======`, `>>>>>>>`), pas seulement dans les fichiers que tu attends.
- Pendant un rebase, `HEAD` (« ours ») est la branche de destination et l'autre côté est ton commit rejoué : les étiquettes sont inversées par rapport à un merge.

## 2. Comprendre l'intention de chaque côté

Avant de toucher un fichier, pour chaque côté :

- les commits depuis l'ancêtre commun : `git log --oneline <base>..<côté>` (ancêtre : `git log --graph --oneline` ; merge commité : `HEAD^1` et `HEAD^2`) et leurs messages ;
- ce que chaque côté a changé **par rapport à l'ancêtre**, pas par rapport à l'autre côté : `git diff <base> <côté> -- <fichier>`. Le bloc entre marqueurs ne montre que les lignes en collision ; l'intention est dans le diff complet.

Écris l'intention de chacun en une phrase (« A : les dates s'affichent en UTC ; B : le formatage passe dans un helper »). Si tu ne peux pas l'écrire, tu n'as pas fini de lire.

## 3. Écrire la version qui fait les deux

- Pars de la structure la plus récente (renommage, extraction, déplacement) et **reporte-y** le changement de comportement de l'autre côté, là où le code vit maintenant. Un changement fait sur des lignes que l'autre côté a déplacées doit suivre ces lignes.
- Deux ajouts au même endroit (entrées de changelog, imports, cas de test, champs) : garde les deux, dans un ordre cohérent.
- Ne réécris rien d'autre : pas de nettoyage ni d'amélioration pendant une résolution.
- Côté abandonné volontairement : seulement s'il est rendu inutile par l'autre (même correction faite deux fois) ; dis-le dans ton rapport.

## 4. Chercher les conflits sans marqueur

Git fusionne sans conflit des changements qui se contredisent. Pour chaque intention du §2, cherche le code de l'**autre** côté qui en dépend : nouvel appelant d'une fonction renommée ou changée, copie de la logique modifiée (un autre calcul qui refait ce que l'un des côtés a corrigé ou factorisé), test qui suppose l'ancien comportement. `git diff <base> HEAD^1` et `git diff <base> HEAD^2` (ou les deux côtés du rebase) listent ce qu'il faut relire. Applique l'intention à ce code aussi.

## 5. Vérifier

- Aucun marqueur restant dans le dépôt (recherche sur tous les fichiers suivis).
- Build, lint et tests complets du projet (skill `verification-before-completion`) ; les tests des **deux** côtés passent.
- Un changement d'un côté sans test : écris le test qui le montre, pour qu'il ne se perde plus.
- Rapport : par fichier, l'intention de chaque côté et comment la résolution les garde.

## Opérations git

Tes permissions et oh décident de `git merge`, `git rebase --continue`, `git add` et `git commit` : un merge ou un rebase commite, il n'est possible qu'après le checkpoint qui déverrouille le commit. Jamais `--abort`, `reset`, `checkout --theirs/--ours` sur un fichier entier ni `--strategy-option` pour « faire passer » : ils jettent un côté sans le dire.
