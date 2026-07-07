---
name: review-merge
description: "Skill de fusion de rapports multi-modes du reviewer. Reçoit N rapports bruts (standard, adversarial, edge-case) issus de sessions parallèles indépendantes et produit un rapport unifié dédupliqué. Travail purement structurel — aucune analyse de code. Mots-clés : merge, fusion, dedup, rapport unifié, multi-mode."
bucket: B
---

# Skill — Fusion de rapports multi-modes (Review Merge)

## Rôle

Recevoir les rapports bruts produits par des sessions de review indépendantes (standard, adversarial, edge-case) et produire un **rapport unifié unique** — dédupliqué, structuré et tagué par provenance.

Ce skill ne fait **aucune analyse de code**. C'est un travail de structuration et de fusion.

---

## Règles absolues

❌ Ne jamais réanalyser le code source — travailler uniquement à partir des rapports bruts reçus
❌ Ne jamais inventer de findings — ne fusionner que ce qui existe dans les rapports d'entrée
❌ Ne jamais modifier la sévérité d'un finding à la baisse
✅ En cas de doublon : conserver la sévérité la plus haute
✅ Tagger chaque finding avec sa provenance `[STD]` / `[ADV]` / `[EDGE]`
✅ Conserver les sections spécifiques de chaque mode en annexe

---

## EXECUTION

### Étape 1 — Recevoir les rapports bruts

Les rapports d'entrée sont transmis dans le prompt. Chaque rapport est délimité par son header :

- `## Review — <branche>` → rapport standard
- `## Revue Adversariale — <périmètre>` → rapport adversarial
- `## Analyse Edge Cases — <périmètre>` → rapport edge-case

Identifier les rapports présents. Minimum 2 pour justifier une fusion.

### Étape 2 — Parser les findings

Pour chaque rapport, extraire les findings :
- **Identité d'un finding** : `fichier:ligne` + nature du problème
- **Attributs** : sévérité, description, suggestion, provenance

### Étape 3 — Déduplication

Deux findings sont des **doublons** si :
1. Même `fichier:ligne` (exactement)
2. ET même nature de problème (même cause racine, même conséquence)

Règles de fusion en cas de doublon :
- **Sévérité** : conserver le niveau le plus élevé (🔴 > 🟠 > 🟡 > 💡)
- **Description** : fusionner les descriptions en gardant la plus complète
- **Suggestion** : conserver la plus actionnable
- **Provenance** : tagger avec toutes les provenances `[STD+ADV]`, `[STD+EDGE]`, etc.

Si deux findings concernent le même fichier:ligne mais des problèmes **différents**, ce ne sont PAS des doublons — les conserver séparément.

### Étape 4 — Structurer le rapport unifié

```
## Review unifiée — <branche ou périmètre>

### Modes appliqués
<Liste des modes ayant produit un rapport : Standard ✓ / Adversarial ✓ / Edge-case ✓>

### Résumé global
<Synthèse de 3-5 phrases combinant les évaluations des différents modes>

### Statistiques

| Provenance | Findings bruts | Après dédup |
|------------|---------------|-------------|
| Standard [STD] | X | Y |
| Adversarial [ADV] | X | Y |
| Edge-case [EDGE] | X | Y |
| **Total** | **X** | **Y** |

---

### 🔴 Critique — bloquant
<Findings dédupliqués, chacun tagué [STD] / [ADV] / [EDGE] / [STD+ADV] etc.>

### 🟠 Majeur — à corriger
<Findings dédupliqués>

### 🟡 Mineur — amélioration recommandée
<Findings dédupliqués>

### 💡 Suggestion — optionnel
<Findings dédupliqués>

### ✅ Points positifs
<Issus du rapport standard — les positifs ne sont pas dupliqués>

### 🔍 Hors scope
<Issus du rapport standard>

---

## Annexes spécifiques par mode

### ⚠️ Hypothèses dangereuses [ADV]
<Section complète issue du rapport adversarial — non dédupliquée>

### 🏗️ Problèmes d'architecture [ADV]
<Section complète issue du rapport adversarial — non dédupliquée>

### 📊 Score de confiance [ADV]
<Score et justification issus du rapport adversarial>

### 🛤️ Chemins non gérés par classe [EDGE]
<Résumé structuré par classe issu du rapport edge-case — non dédupliqué>

### 🧪 Couverture de tests manquante [EDGE]
<Section issue du rapport edge-case>
```

### Étape 5 — Validation

Avant de produire le rapport final :
1. Vérifier qu'aucun finding des rapports bruts n'a été perdu (sauf déduplication explicite)
2. Vérifier que les tags de provenance sont corrects
3. Vérifier que les annexes spécifiques sont complètes

---

## HALT CONDITIONS

- **HALT si un seul rapport est fourni** — pas besoin de fusion, retourner le rapport tel quel
- **HALT si les rapports sont vides ou illisibles** — signaler et demander les rapports complets

---

## Intégration avec le handoff

Quand invoqué en contexte subagent (depuis orchestrator-dev ou CP-feature) :
- Le rapport unifié est suivi du bloc `## Retour vers orchestrator-dev` comme pour une review standard
- Le verdict se base sur le rapport unifié (les sévérités les plus hautes post-fusion déterminent le verdict)
- Le champ `### Corrections requises` inclut les tags de provenance pour traçabilité
