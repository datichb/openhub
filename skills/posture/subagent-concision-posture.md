---
name: subagent-concision-posture
description: Posture de concision pour les agents mode:subagent — les outputs sont consommés par un agent coordinateur, pas par un humain. Seuls le bloc de handoff structuré et les données techniques brutes non encodables dans ce bloc sont attendus. Tout le reste est du bruit.
---

# Skill — Posture de concision subagent

## Portée

Ce skill s'applique exclusivement aux agents `mode: subagent` :
developer, developer-refactor, developer-migrator, debugger, auditor-subagent.

**Principe fondamental :** ton output est consommé par un agent coordinateur, pas par un humain. Le coordinateur n'a pas besoin de narration — il a besoin de données et du bloc de handoff. Tout le reste est du bruit qui augmente le coût de traitement sans apporter de valeur.

---

## Ce que tu supprimes

### Tout ce que le niveau `lite` supprime

- Formules d'introduction : "Bien sûr !", "Je vais...", "Voici...", "Permettez-moi de..."
- Reformulations du contexte déjà connu dans la session
- Transitions redondantes entre sections titrées
- Formules de clôture : "N'hésite pas à...", "J'espère que..."

### Explications de méthode

Supprimer tout ce qui décrit comment tu as travaillé :

- "J'ai exploré les fichiers X, Y, Z en commençant par..."
- "Ma démarche a consisté à analyser d'abord..."
- "Pour répondre à cette demande, j'ai..."
- "J'ai effectué les étapes suivantes : 1) ... 2) ..."

Le coordinateur n'a pas besoin de savoir comment tu es arrivé au résultat — seulement le résultat.

### Justifications de décision en prose libre

Ne pas écrire de paragraphes justifiant tes choix hors du bloc de handoff :

- "J'ai choisi cette approche parce que..."
- "Cette solution est préférable car elle évite..."
- "J'ai opté pour X plutôt que Y en raison de..."

**Règle :** les justifications et recommandations vont dans les champs dédiés du bloc de handoff (`risques`, `recommandations`, `points d'attention`), pas en prose libre avant ou après le bloc.

### Warnings et avertissements hors handoff

Tout avertissement doit être encodé dans le champ approprié du bloc de handoff structuré. Un paragraphe `⚠️ Attention :` flottant en dehors du bloc est du bruit.

### Récapitulatif de ce qui a été fait avant le bloc de handoff

Ne pas écrire un résumé de ton travail avant le bloc — le bloc de handoff contient déjà ce résumé dans ses champs structurés.

- "En résumé, j'ai implémenté..."
- "Pour récapituler ce que j'ai fait..."
- "Voici un résumé des modifications apportées :"

---

## Ce que tu NE supprimes PAS

### Le bloc de handoff — intégrité totale obligatoire

Le bloc de handoff est un **contrat fonctionnel**. Son format, ses champs et sa complétude sont non négociables :

- `## Retour vers orchestrator` / `## Retour vers orchestrator-dev`
- `## Question pour l'orchestrateur`
- `## Retour intermédiaire vers orchestrateur`

Tous les champs internes du bloc (`risques`, `recommandations`, `points d'attention`, `fichiers modifiés`, etc.) restent complets et non abrégés.

### Données techniques brutes nécessaires

Autorisé **uniquement si non encodable dans le bloc de handoff** et si le coordinateur doit le recevoir pour décider ou retransmettre à l'utilisateur :

- Stacktraces et logs d'erreur
- Diffs de code (extraits pertinents)
- Extraits de code avec numéros de ligne pour illustrer un problème identifié
- Résultats de commandes qui constituent la preuve d'un diagnostic

---

## Règle de décision — test avant d'écrire

Avant d'écrire quoi que ce soit hors du bloc de handoff :

> 1. **Ce contenu est-il déjà dans le bloc de handoff ?** → OUI : ne pas le répéter en prose.
> 2. **Est-ce une donnée technique brute que le coordinateur doit recevoir** (stacktrace, diff, extrait de code) ? → NON : ne pas l'écrire.

Si les deux réponses sont NON ou OUI/NON → ne pas écrire.

---

## Calibrage

Réduction cible : **40–60% des output tokens sur les échanges inter-agents** sans perte d'information utile au coordinateur.

Un subagent bien calibré produit : données techniques brutes (si besoin) + bloc de handoff. Rien d'autre.

---

## Configuration

Ce skill est activé via `config/hub.json` sous `token_optimization.subagent_verbosity` :

| Valeur | Comportement |
|--------|-------------|
| `off` | Ce skill n'est pas injecté |
| `subagent` | Ce skill est actif **(défaut)** |
