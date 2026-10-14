---
name: postmortem
description: "Utiliser pour écrire le postmortem d'un incident : chronologie sourcée, impact chiffré, délai de détection, déclencheur et cause, actions vérifiables, sans blâme."
---

# Skill — Écrire un postmortem

Un postmortem sert à ce que l'incident ne se reproduise pas, ou soit vu et arrêté plus tôt. Il décrit un système, pas des coupables.

## 1. Sans blâme

- Aucun nom de personne, ni pseudonyme, ni rôle qui désigne une seule personne (« le stagiaire ») : ni l'auteur du commit, ni la personne citée dans le signalement, même si l'utilisateur la met en cause.
- Écris ce que le système a permis : « le changement a été fusionné sans test sur le cas qui a cassé », pas « X a oublié les tests ». Chaque « erreur humaine » cache une question : qu'est-ce qui l'a rendue possible, et qu'est-ce qui ne l'a pas arrêtée (revue, test, alerte, déploiement progressif) ?
- Pas de jugement (« négligence », « faute ») ; des faits et des conditions.

## 2. Chronologie sourcée

- Une ligne par événement, heure en tête, dans l'ordre : changement déclencheur (commit, déploiement), premier symptôme, détection (alerte, signalement), diagnostic, atténuation, retour à la normale.
- Chaque heure vient d'une source citée (ligne de log, date du commit, message de l'utilisateur). Une heure estimée est marquée « environ ».

## 3. Impact et détection, chiffrés

- **Impact** : durée (du premier symptôme au retour à la normale), volume touché (requêtes, factures, utilisateurs), données à reprendre. Un chiffre lu dans un log ou un fichier se cite ; un chiffre inconnu se dit inconnu.
- **Délai de détection** : du premier symptôme à la détection (la première alerte ou le premier signalement, pas le diagnostic), en minutes, avec les deux heures exactes et la soustraction faite sur elles. C'est souvent le meilleur levier d'amélioration.

## 4. Déclencheur, cause et facteurs

Sépare :

- **le déclencheur** : le changement qui a lancé l'incident (commit, configuration, pic de trafic) ;
- **la cause racine** : le défaut qui l'a rendu possible (fonction, fichier, hypothèse fausse dans le code) ;
- **les facteurs aggravants** : ce qui a allongé l'impact ou retardé la détection (pas de test, pas d'alerte, pas d'interrupteur, relances sans délai).

Remonte d'un « pourquoi » à l'autre jusqu'à un défaut qu'une action peut supprimer ; arrête-toi là.

## 5. Ce qui a aidé, ce qui a gêné

Deux listes courtes : ce qui a rendu le diagnostic ou l'atténuation rapides (interrupteur prévu, logs clairs), ce qui les a ralentis. Les deux nourrissent les actions.

## 6. Actions

- Une action par ligne, chacune vérifiable (« test de non-régression sur l'export d'une liste vide », pas « mieux tester »), rattachée à un type : **prévenir** (la cause ne revient pas), **détecter** (l'alerte arrive plus tôt), **atténuer** (l'arrêt est plus rapide).
- Au moins une action sur la cause racine et une sur la détection, si le délai de détection n'est pas nul.
- Pas d'action qui consiste à « faire attention » : elle ne change pas le système.
