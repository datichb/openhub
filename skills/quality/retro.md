---
name: retro
description: "Utiliser pour la rétrospective de sessions d'agents à partir de leurs traces : faits sourcés, regroupement par cause, coût des relances, actions vérifiables dans une trace suivante, sans blâme."
---

# Skill — Rétrospective sur des traces de sessions

Une rétrospective sert à ce que la prochaine session aille plus droit. Elle part de ce que la trace prouve, pas de l'impression laissée par la session.

## 1. Lire un résumé de trace

| Section | Ce qu'elle dit | Ce qu'elle ne dit pas |
|---|---|---|
| Checkpoints demandés plusieurs fois | une étape refaite (relance) : le travail montré ne convenait pas, ou oh l'a refusé | pourquoi : la consigne est dans « Décisions refusées » |
| Décisions refusées ou corrigées | l'utilisateur a demandé autre chose, avec sa consigne entre guillemets | que l'agent avait tort : la demande a pu changer |
| Appels refusés | une commande ou un outil bloqué par une règle ou un garde-fou d'oh | si c'était voulu : un garde-fou qui bloque un `git push` fait son travail |
| Coût par agent, durée | où sont partis l'argent et le temps | le coût d'une relance seule : estime-le (coût de la session ÷ passages), marqué « estimé » |

Un refus isolé est un fait, pas encore un constat. Un constat, c'est un fait qui revient (deux sessions, deux relances) ou qui coûte (une relance du checkpoint de commit, un agent qui fait la moitié du coût).

## 2. Remonter à une cause du système

Pour chaque constat, demande ce qui l'a rendu possible, jusqu'à quelque chose qu'on peut changer :

- **une consigne** (prompt du workflow, convention du projet) floue ou absente : l'agent ne pouvait pas savoir ;
- **une permission** trop étroite (une commande légitime refusée, refaite en boucle) ou un garde-fou contourné ;
- **une vérification** qui manque au bon moment (le checkpoint montre du travail que seul l'utilisateur pouvait juger) ;
- **la demande** elle-même, ambiguë ou changée en route.

Lis le dépôt pour confirmer (`CONVENTIONS.md`, Makefile, CI) et cite `fichier:ligne`. Ce que ni la trace ni le dépôt ne prouvent reste une hypothèse, dite comme telle.

## 3. Sans blâme

Ni l'utilisateur, ni un collègue, ni « le modèle » : on décrit ce que le système a laissé passer. Si la question nomme quelqu'un, réponds sur le système et ne reprends pas le nom.

## 4. Actions

- Une action change une chose précise à un endroit précis (une ligne de permission, une phrase du prompt, une cible du Makefile).
- Elle porte son signe de réussite, lisible dans une prochaine trace : « plus de refus de `docker compose` dans les sessions `ticket` », « `cp-2` demandé une seule fois ».
- Garde aussi ce qui a bien marché : une session sans relance dit ce qu'il ne faut pas casser.
