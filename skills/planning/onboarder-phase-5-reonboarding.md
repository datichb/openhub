---
name: onboarder-phase-5-reonboarding
description: "Utiliser avant la phase 5 de l'onboarder quand docs/wiki/index.md existe déjà : re-onboarding (mises à jour proposées avec wiki_propose) et récap final."
---

## Phase 5 — Re-onboarding (wiki existant)

<!-- oh:when role=entry -->
**Uniquement après validation explicite.**
<!-- /oh:when -->
<!-- oh:when role=subagent -->
La demande de ton appelant vaut validation : propose les mises à jour du wiki, sauf si ton prompt demande de t'arrêter avant.
<!-- /oh:when -->

`docs/wiki/index.md` existe déjà : tu ne modifies **aucune** page existante (ni `edit` ni `write` dans `docs/wiki/`, même pour une date ou un tag) et tu n'en supprimes aucune. Chaque mise à jour est une proposition (charge `living-docs-enrichment` avant la première : format de `content`, tags) : l'utilisateur l'accepte, la modifie ou la refuse à la carte du checkpoint de commit, et oh applique les acceptées (fin de section, ligne de tag, frontmatter). Pas de réécriture complète : si l'utilisateur en veut une, il supprime `docs/wiki/` et relance l'onboarding (création).

1. Lis l'index (`wiki_index`), puis `wiki_check` : il signale les fichiers cités disparus (`cited_missing`) et les pages en retard sur le code (`stale`). Avec un champ `synced:` dans `index.md`, ajoute `since_sync: true` (dérives depuis la dernière synchronisation).
2. Pour chaque page (signalées d'abord) : lis-la (`wiki_read`) et compare chaque entrée au code lu en phases 1 à 4. Une entrée encore exacte ne donne lieu à rien : ni précision, ni reformulation, ni date rafraîchie.
3. Propose (`wiki_propose`) seulement :
   - une entrée périmée ou contredite (fichier, module, commande, fonction renommés ou disparus) : la correction, dans la même section, avec le nouveau `fichier:ligne`, en commençant par « Remplace l'entrée du <date> : » ;
   - un élément structurant absent du wiki : module, dépendance, convention d'équipe, contrainte d'architecture, règle métier. Un détail lisible dans le code (droits d'un fichier, format d'affichage, cas de test manquant) n'en est pas un : il va dans ton rapport, pas dans le wiki ;
   - `docs/wiki/decisions/decisions-rejetees.md` : `EXPIRED` pour une entrée `ACTIVE` de plus de 6 mois ou dont le contexte technique a évolué, `SUPERSEDED` avec la référence du ticket ou de l'ADR qui a décidé autrement (section de l'entrée, « Statut : … » et la raison).
4. Wiki fidèle au code : aucune proposition (`> 💾 Wiki documentaire : aucune nouvelle découverte à capitaliser.`), ce que ton rapport dit.

Ton rapport liste les dérives relevées et les `wp-…` proposés. Aucune écriture hors de `docs/wiki/`, `ONBOARDING.md` et `CONVENTIONS.md` ; ne JAMAIS modifier `.gitignore` ni `.git/`.

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
