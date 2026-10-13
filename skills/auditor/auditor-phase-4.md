---
name: auditor-phase-4
description: "Utiliser quand l'auditor consolide (phase 4) : rapport d'un domaine ou synthèse multi-domaines, actions prioritaires, fin de l'audit."
---

# Auditor — Phase 4 : consolidation et synthèse

## Un seul domaine audité

Affiche le rapport du sous-agent tel quel (lu avec `workflow_report_get`) : pas de synthèse multi-domaines.

## Plusieurs domaines audités

Pour chaque rapport : score /10, problèmes par criticité, actions prioritaires, points positifs. Puis produis la synthèse dans cette structure exacte :

```markdown
## Synthèse Audit Multi-domaines — <nom du projet>

### Résumé exécutif
<3-5 phrases : périmètre audité (X domaines), score global estimé, tendance générale, problèmes les plus critiques>

### Vue d'ensemble

| Domaine | Score | Niveau | 🔴 Critiques | 🟠 Majeurs | 🟡 Mineurs | Rapport |
|---------|-------|--------|-------------|-----------|-----------|---------|
| Sécurité | 6/10 | ⚠️ Corrections requises | 2 | 5 | 8 | rp-… |
| Performance | 8/10 | ✅ Acceptable | 0 | 2 | 3 | rp-… |

### Score global estimé
<NOTE> /10 — <appréciation courte>
**Méthode de calcul :** moyenne pondérée des scores par domaine <pondération, ex. : sécurité et accessibilité comptent double pour une application grand public>

### Top 5 des actions prioritaires (tous domaines confondus)
1. 🔴 **[Domaine]** <action la plus urgente> — <fichier:ligne ou module>
2. …

### Points positifs globaux
- <ce qui est bien fait dans l'ensemble du projet>

### Recommandations stratégiques
- <recommandations transverses, interdépendances comprises — ou « Aucune recommandation stratégique » si les audits sont indépendants>

### Rapports détaillés
<identifiant rp-… de chaque rapport de domaine>
```

**Interdépendances** : un même problème peut ressortir dans plusieurs domaines (entrées non validées → sécurité et performance ; absence de tests → architecture et observabilité ; UI lourde → accessibilité, éco-conception et performance). Signale-les dans « Recommandations stratégiques ».

## Enrichissements du wiki

Plusieurs domaines : termine la synthèse par `### Enrichissements wiki proposés`, qui regroupe ceux des rapports de domaine : les `wp-…` tels quels (déjà en file, ne les repropose pas) et les découvertes listées sans identifiant. Un seul domaine : ceux du rapport suffisent.
<!-- oh:when mcp=wiki -->
Découvertes sans `wp-…` (outil absent chez le sous-agent ou proposition non enregistrée) : propose-les avec `wiki_propose` selon `living-docs-enrichment` et cite leurs identifiants.
<!-- /oh:when -->

## Récap et fin

```markdown
## [Phase 4] Synthèse produite

**Domaines audités :** <X> · **Score global estimé :** <NOTE> /10
**Problèmes critiques :** <nombre> 🔴 · **Actions prioritaires :** 5
```

Validation : « Terminer » (fin de session), « Relancer un audit » (demander le domaine, relancer le sous-agent, revenir en phase 3), « Revoir la consolidation » (rester en phase 4, ajuster la synthèse, refaire le récap).
