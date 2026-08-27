---
name: debugger-phase-4-5
description: Phase 4 (détection des cas particuliers) et Phase 5 (production du livrable — rapport structuré + ticket Beads de correction) du workflow debugger.
---

# Phase 4 — Détection des cas particuliers

## Objectif
Vérifier les cas limites et situations non standards qui pourraient avoir été manqués.

## Ce qu'on vérifie

**Checklist des cas particuliers :**

- ✅ **Bug intermittent / race condition** : Le comportement est-il lié à un timing, une concurrence, ou un état transitoire ?
- ✅ **Problème d'environnement** : Le bug est-il spécifique à un environnement (dev / staging / prod) ?
- ✅ **Données spécifiques** : Le bug se produit-il uniquement avec certaines données (edge cases, valeurs nulles, caractères spéciaux) ?
- ✅ **Configuration** : Le bug est-il lié à une configuration (env vars, feature flags, paramètres) ?
- ✅ **Dépendances externes** : Le bug dépend-il d'un service externe (API, BDD, cache) ?
- ✅ **Régression** : Y a-t-il eu un changement récent (commit, déploiement, migration) qui coïncide avec l'apparition du bug ?

## Déclencheur de pause ⏸️

Si un **cas particulier critique** est détecté (ex : race condition confirmée, bug prod uniquement) :
- Afficher le contexte en texte (description du cas, impact, options)
- Puis utiliser l'outil `question` pour demander comment le traiter

## Récap de fin de Phase 4

Le récap de Phase 4 doit contenir :

- **Cas particuliers vérifiés** : nombre de vérifications effectuées
- **Cas particuliers détectés** : pour chaque cas — description + impact + recommandation
- **Cas particuliers écartés** : pour chaque cas — raison de l'écarter
- **Impact sur le diagnostic** : ajustements apportés (ex : hypothèse principale confirmée comme race condition, priorité relevée à P0) ou "aucun ajustement"

**Selon la réponse à la question de validation :**
- **Produire** → Phase 5
- **Vérifier d'autres cas** → rester en Phase 4, vérifier d'autres cas, re-produire le récap
- **Revenir à Phase 3** → Phase 3 (les cas particuliers nécessitent une refonte du diagnostic)

---

# Phase 5 — Production du livrable

**Uniquement après validation explicite.**

→ Charger le skill `quality/debugger-report-templates` via l'outil `skill` pour la structure exacte du rapport de diagnostic et le template de création du ticket Beads.

## Récap de fin de Phase 5

Le récap de Phase 5 doit contenir :

- **Rapport** : symptôme résumé, localisation (`fichier:ligne`), hypothèse principale (probabilité + résumé), nombre de fichiers impliqués
- **Ticket Beads** : ✅ bd-X créé (titre, priorité, label `from-diagnostic`) ou ❌ Non créé (refus de l'utilisateur)

**Selon la réponse à la question de validation :**
- **Terminer** → Fin de session
- **Ajustements** → demander quelle phase (1, 2, 3, 4, 5) et y retourner

---

## Gestion de l'itération entre phases

### Retour en arrière déclenché par l'agent

L'agent peut proposer de revenir à une phase précédente si :
- Une découverte en Phase 3 ou 4 nécessite une nouvelle exploration
- Une réponse en Phase 2 nécessite une nouvelle exploration
- Un cas particulier en Phase 4 nécessite une révision du diagnostic en Phase 3

Afficher le contexte en texte :
```markdown
## ⏸️ Retour en arrière recommandé

<raison du retour — découverte, nouvelle information, incohérence>

**Impact :** <ce qui change si on revient en arrière>

**Options disponibles :**
- Revenir à Phase X → <ce qui sera fait>
- Continuer → <conséquence si on ne revient pas>
```

Puis appeler l'outil `question` pour demander la décision.

### Retour en arrière demandé par l'utilisateur

Si l'utilisateur demande explicitement de revenir à une phase ("reviens à l'exploration", "refais la Phase 2") :
1. Revenir à la phase demandée
2. Reproduire le récap de cette phase avec les nouvelles informations
3. Poser la question de validation de cette phase

### Compteur d'itérations

Pour éviter les boucles infinies, maintenir un compteur interne par phase :
- **Limite : 3 itérations par phase maximum**
- À la 3ème itération, proposer de terminer ou de passer à la phase suivante même si incomplet

Afficher le contexte en texte :
```markdown
## ⏸️ Limite d'itérations atteinte

La Phase X a été répétée 3 fois. Pour éviter une boucle infinie, je recommande de passer à la suite.

**Options disponibles :**
- Continuer quand même → passer à la phase suivante avec l'information actuelle
- Itération finale → une dernière itération puis passage forcé
- Terminer → arrêter le diagnostic ici
```

Puis appeler l'outil `question` pour demander la décision.

---

## Résumé des transitions possibles

```
Phase 0 → Phase 1 (normal)
Phase 0 → Phase 0 (préciser artefacts)
Phase 0 → Stop (abandon)

Phase 1 → Phase 2 (questions détectées)
Phase 1 → Phase 3 (pas de questions)

Phase 2 → Phase 3 (normal)
Phase 2 → Phase 1 (nouvelle exploration)

Phase 3 → Phase 4 (normal)
Phase 3 → Phase 3 (réviser diagnostic)
Phase 3 → Phase 5 (skip Phase 4)

Phase 4 → Phase 5 (normal)
Phase 4 → Phase 4 (vérifier autres cas)
Phase 4 → Phase 3 (réviser diagnostic)

Phase 5 → Fin (normal)
Phase 5 → Phase X (ajustements — demander quelle phase)
```
