> 🇬🇧 [Read in English](014-context-mode-plugin.en.md)

# ADR-014 — Plugin context-mode comme couche d'optimisation de tokens complémentaire à RTK

## Statut

Accepté

## Contexte

Le hub dispose de RTK depuis v1.3.0 pour compresser les outputs bash (60-90% de réduction). Trois gaps ont été identifiés que RTK ne couvre pas :

**Gap 1 — Outputs read/webfetch/MCP entrent entiers dans le contexte.** Quand un agent lit un fichier de 2 000 lignes ou fait un webfetch sur une documentation, la totalité de l'output est injectée dans le contexte LLM. RTK n'intercepte que les commandes bash — les outils natifs OpenCode (`read`, `webfetch`) et les appels MCP ne passent pas par ce hook.

**Gap 2 — Pas de session continuity après compaction.** Quand OpenCode compacte automatiquement le contexte (seuil configurable dans `compaction.auto`), l'agent perd le fil de la session. Il doit souvent re-explorer la codebase pour retrouver l'état, ce qui gaspille des tokens identiques à ceux déjà consommés.

**Gap 3 — Exploration multi-fichiers coûteuse.** Un agent qui cherche une information spécifique dans une codebase va typiquement chaîner 10-15 appels `read`/`glob`/`grep`. Un script d'analyse ciblé réalise la même tâche en 1-2 appels avec un output structuré, mais ce pattern n'est pas naturellement adopté sans instruction explicite.

## Décision

Intégrer le plugin OpenCode `context-mode` en installation globale (`~/.config/opencode/plugins/context-mode.ts`).

**Raisons du choix :**
- Plugin npm avec plugin OpenCode natif — pas de proxy HTTP requis, pas de dépendance Python/Rust
- Couverture orthogonale à RTK : RTK gère bash, context-mode gère les outils natifs
- Installation globale via `oh plugin install context-mode` — pas d'AGENTS.md par projet requis grâce au hook `experimental.chat.system.transform` qui injecte les instructions dans chaque session
- Complémentarité avec le mécanisme de compaction existant (`compaction.auto`, `compaction.prune`)

**Architecture :**

Le plugin `plugins/context-mode/context-mode.ts` est un thin wrapper qui :
1. Vérifie la disponibilité du package npm `context-mode`
2. Importe et délègue au plugin npm upstream (hooks stables + expérimentaux)
3. Ajoute un tracking de session propre au hub (toasts, logs, métriques)

L'AGENTS.md n'est pas nécessaire pour l'installation globale car le hook `experimental.chat.system.transform` injecte les instructions directement dans le system prompt de chaque session.

**Configuration documentée** dans `config/hub.json` sous `token_optimization.plugins.context-mode`.

## Conséquences

### Positives

- **-80-98% sur les outputs outils volumineux** : fichiers > 1K tokens, pages webfetch complètes, outputs MCP volumineux — indexés hors-contexte, seul le passage pertinent entre dans le LLM.
- **Session continuity** : 0 tokens gaspillés après compaction automatique. L'agent retrouve l'état de session via BM25 sans re-exploration.
- **Think in Code** : réduction du nombre d'appels `read`/`glob`/`grep` pour les explorations larges.
- **Zéro friction** : installation en une commande, pas de configuration par projet.
- **Complémentarité RTK** : les deux plugins coexistent sans conflit. Stack complète = RTK (bash) + context-mode (read/webfetch/MCP).

### Négatives / compromis

- **Dépendance Node.js >= 22.5.0** : prérequis non négociable du package npm `context-mode`. Les environnements avec Node < 22.5 ne peuvent pas utiliser ce plugin. RTK reste fonctionnel sans context-mode.
- **Hooks expérimentaux** : `experimental.chat.system.transform` et `experimental.session.compacting` sont susceptibles de changer lors des mises à jour d'OpenCode. Le plugin fonctionne en mode dégradé (hooks stables uniquement) si ces hooks sont absents — la sandbox de base reste active mais les instructions context-mode ne sont pas injectées dans le system prompt.
- **Import dynamique du package npm** : si le package npm `context-mode` change son API d'export, le wrapper doit être mis à jour. Le plugin est conçu pour être résilient (fallback en mode dégradé si l'import échoue).

## Alternatives rejetées

**headroom (chopratejas/headroom)** : couche de compression Python + Rust + modèle HuggingFace. Trop lourde pour un hub déjà bien optimisé. En mode MCP (la seule option légère), headroom est opt-in — l'agent doit choisir d'appeler les outils `headroom_compress` explicitement. L'interception automatique requiert le mode proxy HTTP qui introduit une dépendance réseau dans l'infra. headroom cite RTK comme la bonne couche pour les outputs shell — les deux outils ont des périmètres qui se recoupent sans se compléter aussi proprement que RTK + context-mode.

**MCP server context-mode sans plugin OpenCode** : le MCP server seul expose les outils de sandboxing mais ne peut pas intercepter automatiquement les appels `read`/`webfetch` natifs d'OpenCode. L'agent doit choisir d'appeler `ctx_fetch_and_index` explicitement — efficacité réduite, pas de session continuity.

**Ne rien faire** : les 3 gaps identifiés (outputs read/webfetch, compaction, exploration multi-fichiers) ont un impact réel sur les sessions longues avec exploration de codebase étendue. Le coût d'intégration est faible (thin wrapper, installation globale). Le rapport bénéfice/risque justifie l'adoption.

## Impact

| Fichier | Action |
|---------|--------|
| `plugins/context-mode/context-mode.ts` | Créé — plugin thin wrapper |
| `plugins/context-mode/package.json` | Créé — métadonnées npm |
| `scripts/cmd-plugin.sh` | Modifié — ajout bloc vérification context-mode |
| `config/hub.json` | Modifié — ajout `token_optimization.plugins.context-mode` |
| `docs/guides/context-mode-plugin.fr.md` | Créé — guide d'installation |
| `docs/guides/context-mode-plugin.en.md` | Créé — guide d'installation (EN) |
