> 🇬🇧 [Read in English](018-hub-workflow-reference.en.md)

# ADR-018 — Extraction du workflow hub vers un skill canonique `hub-workflow-reference`

## Statut

Proposé

## Contexte

L'analyse comparative BMAD × Superpowers (2026-06-24) a révélé que la description du workflow hub — catalogue des agents, heuristique de routing, tableau des handoffs — est **dupliquée dans au moins 4 endroits** :

| Source | Contenu dupliqué | Taille estimée |
|---|---|---|
| `agents/planning/orchestrator.md` lignes ~40–91 | Catalogue agents + heuristique routing | ~60 lignes |
| `skills/orchestrator/orchestrator-protocol.md` section routing | Routing complet + modes A/B/D/E | ~300 lignes |
| `skills/planning/planner-workflow.md` lignes 42–110 | Routing table orchestrateur, tous agents | ~70 lignes |
| `agents/planning/orchestrator-dev.md` | Domain→agent mapping partiel | ~50 lignes |

Cette duplication crée plusieurs problèmes :

**Dérive de cohérence :** Lorsqu'un agent est ajouté ou modifié, les 4 sources doivent être mises à jour manuellement. En pratique, certaines divergent déjà (ex : `planner-workflow.md` déclare être "source de vérité du routing" alors que `orchestrator-protocol.md` contient une version plus détaillée).

**Apprentissage difficile :** Un nouvel utilisateur ne peut pas comprendre le workflow hub depuis un seul point d'entrée. Le guide doit être reconstitué depuis plusieurs fichiers disparates.

**Coût de maintenance :** L'ajout de chaque nouvel agent (ex : les nouveaux skills issus du plan BMAD × Superpowers) exige de mettre à jour 4 endroits au lieu d'un.

**Précédent existant :** Le hub a déjà résolu ce problème pour un cas similaire — `orchestrator-workflow-modes.md` est déclaré "source de vérité unique" pour les modes d'exécution et est consommé par `orchestrator` et `orchestrator-dev` via bucket-A. Ce pattern est éprouvé et fonctionnel.

## Décision

Créer un skill `skills/shared/hub-workflow-reference.md` déclaré **source de vérité canonique** pour :

1. Le catalogue des agents (famille, rôle, mode, quand invoquer, output attendu)
2. L'heuristique de routing pathfinder vs planner (critères formalisés)
3. Le tableau des handoffs (émetteur → format → récepteur)
4. L'ordre d'enchaînement standard et ses variantes
5. L'intégration du complexity scoring (ADR-018 / item K du plan BMAD)

Remplacer les sections dupliquées dans les 4 sources existantes par des références au skill canonique, en suivant le pattern `orchestrator-workflow-modes.md`.

## Conséquences

### Positives

- **Source de vérité unique** — tout ajout d'agent se fait en un seul endroit
- **Orienteur in-session** — n'importe quel agent peut charger `@hub-workflow-reference` pour connaître sa position dans le workflow et les agents disponibles
- **Réduction de maintenance** — les 4 sources dupliquées deviennent des pointeurs
- **Cohérence garantie** — le routing que voit l'orchestrateur = le routing que voit le planner
- **Base pour `oc-help`** — le skill devient le substrat d'une commande de guidage in-session

### Négatives / Risques

- **Risque de dérive post-refacto** — si un agent est modifié sans MAJ du skill central. Mitigation : règle formalisée dans `docs/guides/authoring-skills.md` (item F du plan) + mention dans ce skill lui-même
- **Rupture du contrat "planner = source de vérité du routing"** — `planner-workflow.md` se déclare actuellement source de vérité. Il faut transférer explicitement ce statut au nouveau skill et mettre à jour le header de `planner-workflow.md`
- **Risque de régression comportementale** — si le contenu opérationnel (templates d'invocation, règles comportementales) est déplacé en même temps que le contenu descriptif. Mitigation : ne déplacer **que** le catalogue/routing/handoffs, laisser les règles opérationnelles dans chaque agent/skill

### Ce qui reste dans chaque fichier

| Fichier | Ce qui reste (ne bouge pas) |
|---|---|
| `orchestrator-protocol.md` | Règles opérationnelles, templates d'invocation, gestion des CPs, protocoles de retransmission |
| `planner-workflow.md` | Les 7 phases de planification, templates Beads, contraintes comportementales |
| `orchestrator.md` | Permissions, behavioral rules, handoff-format loading |
| `orchestrator-dev.md` | Domain→native_skills mapping (spécifique à l'implémentation), délégation rules |

## Alternatives considérées

### Option 1 — Skill standalone sans modification des sources existantes

Créer `hub-workflow-reference.md` comme documentation pure sans modifier les 4 sources. Avantage : zéro risque de régression. Inconvénient : la duplication persiste et le skill dérive immédiatement dès le premier ajout d'agent.

**Rejetée** — résout le problème de discovery mais pas celui de la maintenance et de la cohérence.

### Option 2 (retenue) — Extraction + remplacement par pointeurs

Pattern identique à `orchestrator-workflow-modes.md`. Extraction du contenu dupliqué, remplacement par `@hub-workflow-reference` dans les 4 sources.

### Option 3 — Tout centraliser dans `orchestrator-protocol.md`

`orchestrator-protocol.md` est déjà le document le plus complet (1172 lignes). On pourrait le déclarer source de vérité et faire pointer les autres vers lui. Inconvénient : ce fichier est spécifique à l'orchestrateur — le planner et l'orchestrator-dev consommeraient un skill "orchestrateur" pour leur workflow propre, ce qui est sémantiquement incorrect.

**Rejetée** — couplage inapproprié.

## Règle de gouvernance

Tout ajout d'un nouvel agent dans le hub **doit** inclure une mise à jour de `hub-workflow-reference.md`. Cette règle est documentée dans `docs/guides/authoring-skills.md` (item F).

Le header de `hub-workflow-reference.md` doit déclarer explicitement : `source-of-truth: true`.
