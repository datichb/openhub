---
name: triage
description: "Utiliser pour trier un lot de tickets ou de constats : priorité justifiée, doublon prouvé, étiquettes du projet, ticket trop gros découpé, chaque changement présenté avant d'écrire."
---

# Skill — Trier des tickets

Un tri rend le lot lisible : ce qui passe en premier, ce qui est en double, ce qui est trop gros pour être fait d'un coup. Il ne réécrit pas les tickets.

## 1. Priorité

- Échelle de Beads : **0** casse la production ou bloque tout le monde, **1** gêne beaucoup d'utilisateurs ou bloque un autre ticket, **2** normal, **3** utile sans urgence, **4** un jour peut-être.
- Lis l'impact dans le ticket lui-même (qui est touché, combien, depuis quand, contournement possible) et ce qu'il bloque (`bd dep list`). Pas d'impact écrit : ne monte pas la priorité, signale-le.
- Ne monte pas tout : un lot où tout est en 0 n'est pas trié. Baisser une priorité se justifie aussi.

## 2. Doublon

- Deux tickets sont en double s'ils décrivent **le même problème au même endroit** : même symptôme, même composant. Cite une phrase de chacun.
- `bd find-duplicates` propose des candidats sur les mots ; c'est à toi de vérifier.
- Garde le plus complet (description, critères, historique) ; l'autre devient doublon. S'ils se recoupent sans être le même, un lien `related` suffit.

## 3. Étiquettes

- Seulement celles qui existent (`bd label list-all`) ; deux étiquettes pour une même idée (`bug`, `bogue`) : garde la plus utilisée et signale l'autre.
- Une étiquette ajoutée doit servir à filtrer (domaine, type de travail), pas à résumer le ticket.

## 4. Découper

- Signes : plusieurs critères d'acceptation qui se vérifient séparément, plusieurs composants touchés, des « et » dans le titre.
- Découpe en tranches livrables chacune (un comportement visible, testable seul), pas en couches (« base de données », « interface »). Le parent garde le titre ; chaque enfant reprend ses critères.

## 5. Constats

Un constat d'une revue ou d'un audit cherche d'abord son ticket (`bd search` sur ses mots clés, le fichier qu'il cite). Déjà couvert : un commentaire sur ce ticket, avec la source. Sinon : un ticket, avec le constat tel que reçu.

## 6. Présenter, puis écrire

- Une ligne par changement : ticket, champ, avant → après, raison. Les tickets à créer et les doublons à part, les inchangés en une ligne.
- Après validation, fais exactement le tableau validé ; un changement découvert en cours de route revient au tableau, pas à la commande.
