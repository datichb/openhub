---
name: wiki-navigation
description: "Utiliser quand docs/wiki/index.md existe : lire l'index d'abord, puis seulement la page utile à la tâche."
when: wiki | agent=onboarder
---

# Navigation du wiki

Le wiki (`docs/wiki/`) se lit à la demande, page par page (`ONBOARDING.md` et `CONVENTIONS.md` sont déjà dans ton prompt).

1. **L'index d'abord** : avant toute action, lis `docs/wiki/index.md` (stack, architecture, god nodes, domaines, points critiques), puis décide des pages à charger. Absent : continue sans wiki, ce n'est pas une erreur.
<!-- oh:when mcp=wiki -->
   Avec le MCP du wiki : `wiki_index` donne l'index, les pages et les god nodes **calculés par oh** ; `wiki_read(page, section)` charge une page, ou une seule section.
<!-- /oh:when -->
2. **Seulement la page utile** : `technical/` `conventions.md` (code, nommage, patterns), `architecture.md` (découpage, décisions), `stack.md` (dépendances), `tests.md` ; `business/<domain>.md` pour un domaine (nom dans la table des domaines de l'index), `business/index.md` pour leur vue d'ensemble ; l'index suffit pour le contexte général. Plusieurs domaines : une page après l'autre, si nécessaire.
3. **Jamais le wiki en entier** : ni `ls docs/wiki/` suivi de toutes les lectures, ni `technical/` et `business/` en parallèle sans raison. Seule exception, explicite : un re-onboarding ou une mise à jour complète du wiki.
4. **God nodes** : charge les pages liées à un god node seulement si la tâche porte directement sur ce concept.

## Confiance et fraîcheur

- `CONFIRMÉ` (fichier et ligne cités) : utilise-le. `DÉDUIT` : vérifie-le si la tâche est sensible. `INCERTAIN` : vérifie dans le code source avant d'agir ; daté de plus de 14 jours, il est non fiable par défaut.
- Champ `updated` du frontmatter : de 30 à 90 jours, signale `⚠️ Page potentiellement obsolète (dernière MAJ : {date})` et vérifie les sections sensibles dans le code ; au-delà, `⚠️⚠️ Page obsolète` : recommande de relancer l'onboarding sur ce périmètre avant de t'y appuyer.
<!-- oh:when agent=onboarder -->

## Tableau des god nodes

oh calcule les god nodes et réécrit le tableau de `docs/wiki/index.md` quand il applique une proposition ou sur `oh wiki check --fix` ; `wiki_check` signale un tableau qui s'en écarte. Quand tu écris le tableau : un concept cité dans au moins 2 pages distinctes est un god node, avec ses pages liées ; criticité `Critique` à 4 pages ou plus (ou cité dans « Points critiques actifs »), `Haute` à 3, `Normale` à 2 ; mets à jour le `updated` de l'index si le tableau change.
<!-- /oh:when -->
