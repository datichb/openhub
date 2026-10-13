---
name: review-merge
description: "Utiliser quand plusieurs rapports de review (standard, adversarial, edge-case) doivent être fusionnés et dédupliqués."
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
✅ Tagger chaque finding avec sa provenance `[STD]` / `[ADV]` / `[EDGE]` / `[VERIF]`
✅ Conserver les sections spécifiques de chaque mode en annexe

---

## Mode combiné — préparer et lancer les sessions

Modes combinés : `[MODE:standard+adversarial]`, `[MODE:adversarial+edge-case]`, `[MODE:all]` (ou le choix de l'utilisateur au démarrage).

| Mode | Skill de chaque sous-session | Fusion |
|---|---|---|
| Standard + Adversarial | `review-protocol` (inclus) ; `reviewer-adversarial` | ce skill |
| Adversarial + Edge-case | `reviewer-adversarial` ; `reviewer-edge-case` | ce skill |
| Les trois | `review-protocol` ; `reviewer-adversarial` ; `reviewer-edge-case` | ce skill |

1. **Préparer le contexte une seule fois** (étapes 1 à 5 du workflow du reviewer : contexte, périmètre, vérification, diff, standards), puis une synthèse compacte (60 lignes au plus : conventions clés, décisions d'architecture, règles de `review-rules.md`, god nodes touchés, périmètre après exclusions), formatée en `[WIKI-CONTEXT:<synthèse>]`, `[DIFF-SCOPE:<fichiers>]` et `[STANDARDS:<liste>]`.
2. **Lancer une sous-session par mode**, en parallèle, si `reviewer` figure dans tes agents à lancer ; sinon, fais les analyses toi-même, l'une après l'autre, avec le même contexte :
   <!-- capability: optional -->
   ```
   task(subagent_type: "reviewer", prompt: "[MODE:<mode>] [REVIEW:single] [WIKI-CONTEXT:<synthèse>] [DIFF-SCOPE:<fichiers>] [STANDARDS:<liste>] Review <mode> de <branche>. git diff <base>..<branche>")
   ```
3. **Fusionner** les rapports bruts reçus avec les étapes ci-dessous, en y ajoutant ta section `### Vérification` (préparée en 1).

---

## EXECUTION

### Étape 1 — Recevoir les rapports bruts

Les rapports d'entrée sont transmis dans le prompt. Chaque rapport est délimité par son header :

- `## Review — <branche>` → rapport standard
- `## Revue Adversariale — <périmètre>` → rapport adversarial
- `## Analyse Edge Cases — <périmètre>` → rapport edge-case

La vérification (build, tests) de la session parente, faite une seule fois avant les sous-sessions, est une entrée de plus : ses findings sont tagués `[VERIF]`.

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

### Vérification [VERIF]
<Tableau commande → résultat de la session parente, repris tel quel>

### Statistiques

| Provenance | Findings bruts | Après dédup |
|------------|---------------|-------------|
| Standard [STD] | X | Y |
| Adversarial [ADV] | X | Y |
| Edge-case [EDGE] | X | Y |
| Vérification [VERIF] | X | Y |
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

## Intégration avec le retour

Le verdict se base sur le rapport unifié (les sévérités les plus hautes après fusion). Sous-agent : le rapport unifié est ton rapport (payload du reviewer) ; `### Corrections requises` garde les tags de provenance.
