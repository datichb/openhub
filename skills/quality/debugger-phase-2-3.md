---
name: debugger-phase-2-3
description: Phase 2 (questions complémentaires sur artefacts manquants) et Phase 3 (diagnostic en 4 étapes — reproduction, isolation, identification, hypothèse) du workflow debugger.
---

# Phase 2 — Questions complémentaires (artefacts manquants)

## Objectif
Poser les questions de clarification identifiées en Phase 1 pour lever les zones d'ombre.

## Ce qu'on fait

Cette phase est **optionnelle** — elle n'est exécutée que si des questions complémentaires ont émergé en Phase 1.

Si aucune question → passer directement à Phase 3.

## Format de la question

Afficher d'abord le contexte en texte :

```markdown
## [Phase 2] Questions complémentaires

Quelques questions issues de l'exploration pour affiner le diagnostic :

1. **[Sujet 1]** : <question contextualisée issue de Phase 1>
2. **[Sujet 2]** : <question contextualisée issue de Phase 1>
```

Puis appeler l'outil `question` pour demander comment procéder (répondre aux questions ou skip).

## Récap de fin de Phase 2

Le récap de Phase 2 doit contenir :

- **Questions posées** : nombre de questions
- **Réponses reçues** : pour chaque question, la question et la réponse (ou "non répondu")
- **Zones d'ombre levées** : ce qui était flou et qui est maintenant clair
- **Zones d'ombre persistantes** : ce qui reste flou et son impact sur le diagnostic

**Selon la réponse à la question de validation :**
- **Passer à Phase 3** → Phase 3
- **Revenir à Phase 1** → Phase 1 (les réponses reçues modifient le périmètre d'exploration)

---

# Phase 3 — Analyse approfondie : Diagnostic en 4 étapes

## Objectif
Appliquer la méthodologie de diagnostic pour identifier la cause racine.

## ÉTAPE 3.1 — Reproduction

Identifier et documenter le scénario de reproduction :

- **Comportement observé** : ce qui se passe
- **Comportement attendu** : ce qui devrait se passer
- **Conditions de déclenchement** : données d'entrée, état du système, environnement
- **Fréquence** : systématique, intermittent, sous charge

Si les informations sont insuffisantes pour reproduire, lister explicitement ce qui manque.

---

## ÉTAPE 3.2 — Isolation

Réduire le périmètre du problème :

- Identifier la **couche concernée** : UI, API, service, repository, base de données, infra
- Identifier le **point d'entrée** : première ligne/fonction où le comportement dévie
- Écarter les causes improbables : changements récents (git log), dépendances externes, config

---

## ÉTAPE 3.3 — Identification

Analyser les artefacts disponibles pour localiser la cause :

### Lecture d'une stacktrace

```
1. Lire de bas en haut : le bas est l'origine, le haut est la propagation
2. Identifier la première frame dans le code applicatif (hors node_modules, hors framework)
3. Repérer le fichier et la ligne — c'est le point de départ du diagnostic
4. Identifier le type d'erreur (TypeError, NullPointerException, etc.) et son message
```

### Lecture des logs applicatifs

```
1. Chercher les entrées ERROR et WARN dans la fenêtre temporelle du bug
2. Identifier la corrélation entre les logs et le comportement décrit
3. Repérer les patterns : répétitions, séquences anormales, timestamps inhabituels
4. Vérifier les logs des dépendances (base de données, cache, message broker)
```

### Lecture des logs système / réseau

```
1. Codes HTTP : 4xx → erreur client, 5xx → erreur serveur
2. Timeouts : identifier si le problème est de latence ou d'absence de réponse
3. Vérifier les erreurs de connexion (DNS, TLS, ports)
```

---

## ÉTAPE 3.4 — Hypothèse et vérification

Formuler la ou les hypothèses de cause racine :

```
Hypothèse 1 (haute probabilité) : <description>
  → Éléments qui l'étayent : <preuves dans les artefacts>
  → Pour confirmer : <action à effectuer (log supplémentaire, test, breakpoint)>

Hypothèse 2 (probabilité moyenne) : <description>
  → Éléments qui l'étayent : ...
  → Pour confirmer : ...
```

## Récap de fin de Phase 3

Le récap de Phase 3 doit contenir :

- **Symptôme** : comportement observé vs attendu, conditions de déclenchement, fréquence
- **Périmètre analysé** : artefacts fournis (stacktrace, logs, description, ticket Beads) et ce qui n'était PAS disponible
- **Localisation probable** : `chemin/vers/fichier.ts:ligne` + description courte
- **Cause racine** :
  - Hypothèse principale (probabilité haute/moyenne/faible) : explication en 2-5 phrases, éléments qui l'étayent, action pour confirmer
  - Hypothèse secondaire (si applicable) : même structure
- **Fichiers impliqués** : tableau fichier / rôle dans le bug

**Selon la réponse à la question de validation :**
- **Passer à Phase 4** → Phase 4
- **Réviser** → rester en Phase 3, ajuster le diagnostic, re-présenter
- **Skip Phase 4** → Phase 5 (pas de vérification de cas particuliers)
