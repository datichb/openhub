# Agent Scout — Guide utilisateur

## 🎯 Qu'est-ce que l'agent Scout ?

L'agent **Scout** est un agent de reconnaissance rapide et flexible qui explore une feature en 2-5 minutes et produit un rapport structuré exploitable.

Le scout comble le gap entre exploration légère et planification complète :

```
Simple ← Scout → Planner → Complexe
```

## 🚀 Quand utiliser Scout ?

### ✅ Utilisez Scout pour :

- **Features simples** : "Ajouter un champ email au profil"
- **Phase exploratoire** : "Voir si on peut intégrer Stripe rapidement"
- **POC/Prototype** : "Tester l'idée d'un système de tags"
- **Estimation rapide** : "Combien de temps ça prendrait ?"
- **Doute sur complexité** : "Est-ce que ça vaut le coup de planifier en détail ?"

### 🎯 Utilisez Planner pour :

- **Features complexes** : "Refondre le système d'authentification avec OAuth + 2FA"
- **Signaux design/audit** : "Dashboard analytics avec UX optimisée et audit performance"
- **Production critique** : "Migration de la base de données PostgreSQL vers MongoDB"
- **Multi-agents** : Feature nécessitant designers, auditors, architectes
- **Planification détaillée** : Besoin de tickets Beads enrichis complets

## 📊 Comment fonctionne Scout ?

### Workflow Scout (2-5 min)

```
1. Comprendre la demande (30 sec)
   ↓
2. Explorer le contexte (2-3 min)
   - Fichiers/modules clés
   - Tickets Beads existants
   - Patterns réutilisables
   ↓
3. Estimer la complexité (1 min)
   - XS / S / M / L / XL
   - Facteurs de complexité
   ↓
4. Structurer un draft (1 min)
   - Epic + tickets estimés
   ↓
5. Identifier risques & signaux (30 sec)
   ↓
6. Recommander (30 sec)
   - ✅ Traitement direct OU
   - 🎯 Escalade au planner
```

### Échelle de complexité

| Taille | Tickets estimés | Durée totale | Exemples | Recommandation Scout |
|--------|-----------------|--------------|----------|----------------------|
| **XS** | 1 task | < 1h | Ajouter un champ, changer une couleur | ✅ Direct |
| **S** | 1-2 tickets | 1-3h | Formulaire simple, endpoint CRUD | ✅ Direct |
| **M** | 3-5 tickets | 0.5-1 jour | Système de tags, filtre avancé | ⚠️ Au choix |
| **L** | 6-10 tickets | 1-3 jours | Auth OAuth, dashboard analytics | 🎯 Escalade planner |
| **XL** | 10+ tickets | 1+ semaine | Refonte auth, migration BDD | 🎯 Escalade planner |

**Facteurs qui augmentent la complexité (+1 niveau) :**
- ⚠️ Signaux design (UX/UI)
- ⚠️ Signaux audit (sécurité, performance, RGPD, accessibilité)
- ⚠️ Dépendances multiples (>3 tickets liés)
- ⚠️ Migration de données
- ⚠️ Impact multi-modules (>3 modules touchés)

## 💼 Cas d'usage

### Exemple 1 : Feature simple (XS) → Traitement direct

**Demande :**
```
"J'aimerais ajouter un champ téléphone au profil utilisateur"
```

**Résultat Scout (1 min) :**
- **Complexité :** S (2 tickets, ~1h15)
- **Structure :**
  1. Migration BDD + modèle User (~30min)
  2. Input dans ProfileForm + validation (~45min)
- **Recommandation :** ✅ **Direct** - Feature simple, traitement immédiat

**Prochaine étape :**
→ Scout → orchestrator-dev → Implémentation directe

---

### Exemple 2 : Feature complexe (L) → Escalade au planner

**Demande :**
```
"J'aimerais implémenter un système de notifications temps réel"
```

**Résultat Scout (3 min) :**
- **Complexité :** L (6 tickets, ~14h)
- **Signaux détectés :**
  - ✅ Architecture (nouveau système, choix WebSocket vs SSE)
  - ✅ UX/UI (design NotificationCenter nécessaire)
  - ✅ Performance (connexions persistantes, scaling)
- **Recommandation :** 🎯 **Escalade** - Complexité élevée + signaux forts

**Prochaine étape :**
→ Scout → Planner (avec handoff complet) → Planification 7 phases

---

### Exemple 3 : Doute sur complexité → Au choix utilisateur

**Demande :**
```
"Voir si on peut intégrer un système de paiement Stripe"
```

**Résultat Scout (2 min) :**
- **Complexité :** M (4-5 tickets, ~6-8h)
- **Signaux détectés :**
  - ⚠️ Sécurité (paiements sensibles, PCI-DSS)
  - ⚠️ Architecture (webhook handlers, gestion erreurs)
- **Recommandation :** ⚠️ **Au choix**
  - Option 1 : POC rapide avec orchestrator-dev
  - Option 2 : Planification complète avec planner (audit sécurité recommandé)

**Prochaine étape :**
→ Utilisateur décide selon le contexte (POC vs Production)

## 🎨 Format du rapport Scout

Le scout produit un rapport structuré en markdown :

```markdown
# 🔍 Scout Report

**Feature:** [Nom]
**Complexité:** [XS|S|M|L|XL]
**Date:** [timestamp]

## 📝 Contexte rapide
[2-3 phrases de compréhension]

## 🔎 Exploration (2-3 min)
- Fichiers clés identifiés
- Tickets Beads existants
- Patterns réutilisables

## 🎯 Structure proposée (draft)
- Epic suggéré
- Tickets estimés (1, 2, 3...)

## ❓ Questions ouvertes
- Questions catégorisées

## ⚠️ Risques identifiés
- Niveau + description

## 🚦 Signaux détectés
| Signal | Statut | Détails |

## 🎯 Recommandation
✅ Direct OU 🎯 Escalade (avec justification)

## 📦 Handoff vers planner (si escalade)
[Section complète pour transmission au planner]
```

**Le rapport est exploitable par :**
- 👤 **L'utilisateur** (lecture directe, décision éclairée)
- 🤖 **orchestrator-dev** (contexte pour implémentation directe)
- 🤖 **planner** (handoff complet si escalade)

## 🔄 Workflow complet avec Scout

### Cas 1 : Feature simple

```
User: "Ajoute un champ email au profil"
      ↓
Orchestrator (heuristique: simplicité détectée)
      ↓
Scout (2 min)
      ↓
Report: ✅ Direct (S, 2 tickets, ~1h)
      ↓
orchestrator-dev
      ↓
Implémentation
```

### Cas 2 : Feature complexe

```
User: "Système de notifications temps réel"
      ↓
Orchestrator (heuristique: pas de signal clair)
      ↓
Scout (3 min)
      ↓
Report: 🎯 Escalade (L, signaux archi/UX/perf)
      ↓
User: "OK, escalade au planner"
      ↓
Planner (workflow 7 phases avec handoff scout)
      ↓
Tickets Beads enrichis + routing
```

### Cas 3 : Planner direct (complexité évidente)

```
User: "Refonte complète du système d'authentification avec OAuth + 2FA"
      ↓
Orchestrator (heuristique: mot-clé "refonte" détecté)
      ↓
Planner direct (pas besoin de scout)
      ↓
Tickets Beads enrichis
```

## 🎛️ Heuristique de routage Orchestrator

L'orchestrator choisit automatiquement entre Scout et Planner selon des critères :

### → Scout (rapide)

- **Mots-clés** : "simple", "rapide", "ajouter", "modifier", "quick scan", "scout"
- **Exploration** : "explorer", "voir si", "POC", "prototype"
- **Par défaut** si pas de signal clair

### → Planner (complet)

- **Mots-clés** : "refonte", "système", "architecture", "migration"
- **Signaux** : "UX", "design", "sécurité", "performance", "audit"
- **Complexité évidente**

### → Question utilisateur

- **Doute** : critères mixtes
- **Ambiguïté** sur la complexité

## 📚 Différences clés : Scout vs Planner

| Aspect | Scout | Planner |
|--------|-------|---------|
| **Durée** | 2-5 min | 10-20 min |
| **Workflow** | Libre et flexible | 7 phases rigides |
| **Modèle** | Claude Sonnet 4 | Claude Opus 4 |
| **Sortie** | Rapport structuré | Tickets Beads enrichis |
| **Profondeur** | Reconnaissance | Analyse complète |
| **Délégation** | Non (sauf documentarian) | Oui (designers, auditors) |
| **Escalade** | Peut escalader au planner | Point final |
| **Usage** | Features simples, exploration | Features complexes, production |

## 💡 Bonnes pratiques

### ✅ Faire

1. **Utilisez Scout en premier** si vous ne savez pas la complexité
2. **Lisez le rapport complet** avant de décider
3. **Suivez la recommandation** du scout (mais vous décidez)
4. **Posez des questions** si le rapport manque d'info
5. **Escaladez au planner** si la complexité augmente en cours de route

### ❌ Éviter

1. **Ne sautez pas le scout** pour des features inconnues (gain de temps si simple)
2. **N'ignorez pas les signaux** détectés par le scout
3. **Ne forcez pas le traitement direct** si scout recommande escalade
4. **N'utilisez pas le planner** pour des features évidemment simples (perte de temps)

## 🔧 Configuration

Le scout est configuré dans `config/hub.json` :

```json
{
  "agent_models": {
    "families": {
      "planning": "claude-opus-4"
    },
    "agents": {
      "scout": "claude-sonnet-4"
    }
  }
}
```

**Le scout utilise Claude Sonnet 4 pour :**
- Rapidité d'exécution (2-5 min)
- Coût réduit vs Opus 4
- Qualité suffisante pour reconnaissance

## 🆘 FAQ

### Q: Puis-je invoquer Scout directement ?

**R:** Oui ! Vous pouvez demander explicitement :
- "Scout cette feature pour moi"
- "Fais un quick scan de cette idée"
- "Estime la complexité rapidement"

L'orchestrator invoquera automatiquement le scout.

### Q: Scout peut-il créer des tickets Beads ?

**R:** Oui, mais il demande confirmation avant (permissions `ask`). Généralement, scout recommande plutôt que l'orchestrator-dev ou le planner le fasse.

### Q: Que se passe-t-il si je refuse l'escalade ?

**R:** Vous êtes libre de refuser. Scout aura fourni le contexte suffisant pour que orchestrator-dev puisse implémenter, même si la complexité est moyenne.

### Q: Scout peut-il consulter les designers/auditors ?

**R:** Non, seul le planner peut déléguer aux agents spécialisés (ux-designer, auditor-*, etc.). C'est une raison d'escalader si ces signaux sont forts.

### Q: Comment forcer le planner sans passer par scout ?

**R:** Demandez explicitement :
- "Planifie complètement cette feature"
- "Analyse approfondie avec le planner"
- "Structure détaillée en tickets Beads"

L'orchestrator comprendra et invoquera directement le planner.

### Q: Scout remplace-t-il le planner ?

**R:** Non, ils sont complémentaires :
- **Scout** = reconnaissance rapide, estimation, triage
- **Planner** = analyse complète, enrichissement, coordination multi-agents

Scout peut **escalader** vers planner, mais ne le remplace pas.

## 📖 Ressources

- **Agent Scout** : `/agents/planning/scout.md`
- **Protocole Scout** : `/skills/planning/scout-protocol.md`
- **Format Handoff** : `/skills/planning/scout-handoff-format.md`
- **Agent Planner** : `/agents/planning/planner.md`
- **Orchestrator** : `/agents/planning/orchestrator.md`

## 🎯 Résumé

Le **Scout** est votre allié pour :
- ⚡ Gagner du temps sur les features simples
- 🔍 Explorer rapidement une idée
- 📊 Estimer la complexité avant de s'engager
- 🎯 Décider en connaissance de cause (direct ou planner complet)

**Règle d'or :** *"En cas de doute, commencez par Scout — il vous dira s'il faut escalader."*
