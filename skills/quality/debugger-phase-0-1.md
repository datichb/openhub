---
name: debugger-phase-0-1
description: Phase 0 (vérification des prérequis/artefacts) et Phase 1 (exploration contextuelle) du workflow debugger.
---

# Phase 0 — Vérification des prérequis (artefacts)

## Objectif
Vérifier que les artefacts fournis sont suffisants pour conduire un diagnostic sérieux.

## Ce qu'on vérifie

**Artefacts suffisants pour démarrer** (au moins un doit être présent) :
- Une stacktrace complète avec le nom du fichier et la ligne
- Des logs applicatifs avec au moins un timestamp et un message d'erreur
- Une description précise du comportement observé ET du comportement attendu avec les conditions de déclenchement
- Un ticket Beads avec une description détaillée du bug

**Artefacts insuffisants — pause obligatoire :**
- Une description vague sans comportement observable ni conditions ("ça ne marche pas", "c'est cassé", "j'ai un bug")
- Un message d'erreur tronqué ou sans contexte (ex : "Error: undefined" seul)
- Aucun élément sur les conditions de déclenchement (systématique ? intermittent ? après quelle action ?)

## Déclencheur de pause ⏸️

Si **les artefacts sont insuffisants**, afficher le contexte en texte puis regrouper TOUTES les questions en un seul appel `question` :

```
[Texte de réponse]
## ⏸️ Phase 0 — Artefacts insuffisants

Pour conduire un diagnostic sérieux, j'ai besoin des informations suivantes :
1. <information manquante 1 — ex : stacktrace complète>
2. <information manquante 2 — ex : conditions de déclenchement>
3. <information manquante 3 — ex : logs applicatifs>

**Impact :** Sans ces éléments, le diagnostic sera partiel et formulé en hypothèses.
```

**Règle :** une seule pause, regroupant toutes les questions.

## Récap de fin de Phase 0

Le récap de Phase 0 doit contenir :

- **Artefacts disponibles** : liste détaillée des artefacts fournis (ex : stacktrace complète avec 15 frames, logs applicatifs sur une fenêtre de 2 min, ticket Beads bd-X avec description)
- **Artefacts manquants (si applicable)** : chaque artefact manquant avec son impact sur le diagnostic
- **Ticket Beads lié (si fourni)** : ID, titre, contexte extrait

**Selon la réponse à la question de validation :**
- **Démarrer** → Phase 1
- **Préciser** → rester en Phase 0, intégrer les nouvelles informations, re-produire le récap
- **Arrêter** → fin de session

---

# Phase 1 — Exploration contextuelle

## Objectif
Explorer le contexte du projet pour calibrer le diagnostic.

## Ce qu'on explore

### ÉTAPE 1.1 — Lire CONVENTIONS.md (si existe)

Si `CONVENTIONS.md` existe à la racine du projet → le lire pour contextualiser :
- Patterns attendus (gestion d'erreurs, logging, validation)
- Conventions d'architecture (couches, découpage)
- Patterns spécifiques à l'équipe

### ÉTAPE 1.2 — Lire le ticket Beads (si fourni)

Si un ID de ticket est fourni :
```bash
bd show <ID>
```

**Ce qu'on cherche :**
- La description du comportement attendu (pour comparer avec l'observé)
- Les notes techniques et contraintes du ticket d'origine
- Le contexte de l'implémentation récente liée au bug

**Tu ne modifies jamais le ticket.**

### ÉTAPE 1.3 — Identifier les fichiers impliqués

À partir de la stacktrace ou des logs :
- Identifier les fichiers applicatifs (hors node_modules, hors framework)
- Repérer le premier fichier applicatif dans la stacktrace (point d'origine probable)
- Lire les fichiers identifiés pour comprendre le contexte

## Déclencheur de pause ⏸️

Si une **information critique** émerge pendant l'exploration qui nécessite une clarification immédiate → afficher le contexte en texte puis utiliser l'outil `question`.

## Récap de fin de Phase 1

Le récap de Phase 1 doit contenir :

- **Contexte projet** : CONVENTIONS.md lu/absent, architecture détectée, patterns de gestion d'erreurs observés
- **Ticket Beads** : ID, titre, comportement attendu résumé (ou "aucun si non fourni")
- **Fichiers impliqués (préliminaire)** : chemin:ligne + rôle supposé pour chaque fichier
- **Observations préliminaires** : observations factuelles (ex : erreur de type TypeError, fonction appelée avec un paramètre null)

**Selon la réponse à la question de validation :**
- **Passer à Phase 2** → Phase 2 si questions détectées, sinon Phase 3 directement
- **Questions à poser** → Phase 2
