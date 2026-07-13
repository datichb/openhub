---
id: takeover-context-protocol
bucket: A
scope: orchestrator
condition: team_enabled
---

# Takeover Context Protocol

Ce skill definit comment un agent doit utiliser un brief de reprise (takeover brief)
quand il demarre une session sur un ticket qui a ete transfere d'un autre membre.

## Detection

Au demarrage d'un ticket, verifier si un takeover brief existe :
- Appeler `team_takeover_brief` avec le project et ticket_id courants
- Si le tool retourne "No takeover brief found" → continuer normalement
- Si un contenu est retourne → appliquer le protocole ci-dessous

## Protocole de chargement du contexte

### 1. Lecture du brief

Lire attentivement le brief retourne. Il contient :
- **Meta** : qui a transfere, quand, pourquoi (transfer ou stale)
- **Fichiers touches** : quels fichiers ont ete modifies/crees
- **Etat** : dernier commit, derniere session, progression estimee
- **Historique** : resume chronologique des sessions precedentes
- **Si enrichi** : decisions architecturales, questions ouvertes, risques, prochaines etapes

### 2. Consultation des fichiers

Apres lecture du brief :
1. Lire les fichiers principaux mentionnes (les 2-3 plus importants)
2. Identifier l'etat actuel du code vs ce que decrit le brief
3. Reperer les TODO, FIXME, ou code incomplet

### 3. Synthese pour l'utilisateur

INFORMER l'utilisateur au debut de la session :

```
Ce ticket a ete repris de [nom predecesseur] (raison: [transfer|stale]).

Contexte charge :
- [N] fichiers modifies, [M] crees
- Etat : [resume 1 ligne de l'avancement]
- [S'il y a des questions ouvertes : les lister]

Je reprends le travail la ou [predecesseur] s'est arrete.
```

### 4. Reprise du travail

- NE PAS repartir de zero
- Continuer a partir de l'etat existant
- Respecter les decisions architecturales deja prises (sauf si problematiques)
- Si une decision est questionnable : informer l'utilisateur, ne pas changer unilateralement
- Adresser les questions ouvertes en priorite (poser la question a l'utilisateur si necessaire)

## Regles

- Un takeover brief est informatif — il ne remplace pas la lecture du code
- En cas de doute entre ce que dit le brief et ce que montre le code : le code fait foi
- Ne JAMAIS ignorer silencieusement un brief existant
- Si le brief mentionne des tests manquants : les ajouter fait partie du scope
