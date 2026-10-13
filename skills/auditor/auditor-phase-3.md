---
name: auditor-phase-3
description: "Utiliser quand l'auditor délègue (phase 3) : prompt du sous-agent d'audit par domaine, lancement, audit bloquant, collecte des rapports."
---

# Auditor — Phase 3 : délégation au sous-agent

## Prompt de délégation

Un seul agent, `auditor-subagent`, audite tous les domaines : le domaine et sa skill (table « Mapping domaine → native_skill » de ton prompt) sont injectés dans son prompt. Pour chaque domaine :

```
[Contexte projet transmis par le coordinateur auditor]

**Stack technique :**
- Langages : <liste>
- Frameworks : <liste>
- Base de données : <liste>
- Infrastructure : <liste>

**Architecture :**
- Pattern : <pattern détecté>
- Découpage : <répertoires principaux>

**Points d'attention identifiés :**
- <point 1>

**Périmètre de cet audit :**
- Domaine : <domaine>
- Fichiers/modules ciblés : <périmètre ou "tout le projet">
- Contraintes légales : <ou "aucune">

**Limites connues :**
- <ex. : uniquement du code compilé, pas d'accès aux configs serveur — ou "Aucune">

Tu agis en tant que sous-agent d'audit [DOMAINE].
Charge et applique le skill : audit-[DOMAINE]

Produis un rapport d'audit structuré selon le skill audit-protocol-light,
dépose-le, puis rends ton enveloppe de retour.
```

Lance-le avec `task({ subagent_type: "auditor-subagent", prompt: "<prompt ci-dessus>", description: "Audit <domaine> — <projet>" })`. Plusieurs domaines : **l'un après l'autre**, jamais en parallèle, pour éviter les lectures concurrentes et pouvoir t'arrêter sur un audit bloquant.

## Collecte des rapports

Chaque sous-agent rend son enveloppe et son rapport `rp-…`. Lis-le avec `workflow_report_get` (ou sous l'enveloppe s'il est « inclus ») et relève, en le citant par son identifiant : le score /10, le nombre de problèmes par criticité (🔴, 🟠, 🟡), le plan d'action priorisé, le périmètre non couvert. Rapport incomplet : relance le sous-agent avec son `task_id` pour qu'il le complète.

## Pause ⏸️ — audit bloquant

Statut `bloquant` (faille critique) : avant de lancer les domaines suivants, affiche en texte le domaine, le problème, son impact tel que décrit par le sous-agent et les domaines restants, puis :

```
question({
  questions: [{
    header: "Audit bloquant",
    question: "[Auditeur — Phase 3 : Délégation | Projet : <nom>]\nL'audit <domaine> a détecté un problème bloquant. Comment procéder ?",
    options: [
      { label: "Continuer les autres audits", description: "Lancer les audits restants — le problème bloquant sera signalé dans la synthèse" },
      { label: "Arrêter tous les audits", description: "Stopper l'audit global — corriger le problème critique avant de continuer" }
    ]
  }]
})
```

## Récap de fin de phase 3

```markdown
## [Phase 3] Audits réalisés

| Domaine | Score /10 | 🔴 Critiques | 🟠 Majeurs | 🟡 Mineurs | Statut | Rapport |
|---------|-----------|-------------|-----------|-----------|--------|---------|
| Sécurité | 6/10 | 2 | 5 | 8 | corrections-requises | rp-… |

**Problèmes critiques identifiés :** <nombre> 🔴
**Prochaine étape :** consolidation et synthèse (phase 4)
```

Validation : « Consolider » (recommandé, phase 4), « Relancer un audit » (demander le domaine, relancer le sous-agent, rester en phase 3), « Arrêter » (rapports individuels disponibles, pas de synthèse).
