---
name: onboarder-phase-5-reonboarding
description: "Utiliser avant la phase 5 de l'onboarder quand docs/wiki/index.md existe déjà : re-onboarding (enrichissement du wiki existant) et récap final."
---

## Phase 5 — Re-onboarding (wiki existant)

<!-- oh:when role=entry -->
**Uniquement après validation explicite.**
<!-- /oh:when -->
<!-- oh:when role=subagent -->
La demande de ton appelant vaut validation : enrichis le wiki, sauf si ton prompt demande de t'arrêter avant.
<!-- /oh:when -->

`docs/wiki/index.md` existe déjà : mode re-onboarding. Appliquer le skill `shared/living-docs-enrichment`
avec les découvertes du rapport, puis
<!-- oh:when role=entry -->
proposer les 3 options (enrichissement incrémental
recommandé / réécriture complète / conserver). Ne pas passer aux étapes suivantes
sans confirmation.
<!-- /oh:when -->
<!-- oh:when role=subagent -->
enrichissement incrémental, sans question ; réécriture complète seulement si ton prompt
la demande. Le mode retenu va dans `### Fichiers de contexte produits`.
<!-- /oh:when -->

- **Enrichissement incrémental** : les découvertes du nouveau rapport seulement (pages `technical/`, `business/`, glossaire), selon `living-docs-enrichment` ; aucune page supprimée, aucune structure de page changée.
- **Réécriture complète** : avertir de la perte des enrichissements accumulés par les autres agents, puis charger `onboarder-phase-5` et suivre ses étapes 5.1 à 5.6.
- **Conserver** : aucune écriture ; passer à la vérification ci-dessous.

`docs/wiki/decisions/decisions-rejetees.md` : marquer `EXPIRED` une entrée `ACTIVE` de plus de 6 mois ou dont le contexte technique a évolué ; `SUPERSEDED` avec la référence du ticket ou de l'ADR qui a décidé autrement.

Aucune écriture hors de `docs/wiki/`, `ONBOARDING.md` et `CONVENTIONS.md` ; ne JAMAIS modifier `.gitignore` ni `.git/`. Le commit du wiki revient à l'utilisateur.

---

### ⚠️ Vérification — AVANT de terminer la session

**Vérifications obligatoires dans ton rapport final :**
- ✅ Section `### Rapport d'onboarding` présente (contexte de découverte, observations qualitatives)
- ✅ Section `### Stack technique` renseignée
- ✅ Section `### Dette technique détectée` documentée (🔴 critiques, 🟠 importants, 🟡 informatifs)
- ✅ Section `### Zones d'incertitude` signalées

<!-- oh:when role=subagent -->
Le rapport est déposé (`workflow_report`), puis ta réponse finale est l'enveloppe ; aucun texte libre à côté.
<!-- /oh:when -->

**Si une section est manquante → la compléter MAINTENANT avant de terminer.**
