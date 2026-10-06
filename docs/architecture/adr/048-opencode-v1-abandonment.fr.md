> [Read in English](048-opencode-v1-abandonment.en.md)

# ADR-048 — Abandon d'opencode V1

## Statut

Accepté

## Date

2026-10-06

## Contexte

La décision D3 de l'étude v5 (05/10/2026) faisait d'opencode V2 la cible tout en gardant V1 compatible : un adaptateur V1 était prévu (rendu au schéma V1, lancement `--agent`, capacités réduites, l'interface masquant ce que V1 ne sait pas faire).

En pratique, la phase 0 a gardé l'**ancien chemin** pour V1 plutôt que d'écrire cet adaptateur :

- `launcher.Launch` et `internal/opencode` (lancement interactif et sans interface) ;
- le client serveur V1 de `internal/parallel`, ainsi que `internal/sweep` et `internal/llm` ;
- les plugins globaux V1 (`internal/plugin`, RTK, context-mode) et `oh upgrade opencode` ;
- les statistiques lues dans la base d'opencode ;
- un repli des alias (`oh start`, `audit`, `review`, `debug`, `takeover-brief`) quand V2 ou le workflow manquait, et la variable `OH_V5=0` pour forcer l'ancien chemin.

Le coût était double :

- **deux produits** : rien de ce qui fait la v5 n'existait sous V1 (paquet par workflow, monde fermé vérifié, checkpoints contrôlés, décisions depuis oh, conteneur, distant) ;
- **deux chemins à maintenir et à tester** dans chaque piste : replis dans la CLI et la TUI, stats V1, code de déploiement gardé pour V1.

D3 a été révisée le 06/10/2026 : **V1 n'est plus pris en charge en v5**.

## Décision

1. **opencode V2 obligatoire**. Sans opencode, ou avec une version hors plage, oh refuse de lancer avec un message clair (`cmd.v1.unsupported.*`) qui renvoie vers `oh doctor` et le guide de migration. La plage est déclarée dans `opencodev2/compatibility.json` (oh 5.0 : opencode 2.0.0 → 2.99.99). Le contrôle Doctor « opencode V2 » passe en premier.
2. **L'ancien lancement est supprimé** (lot 3.E, P3-T30) :
   - `internal/opencode`, `internal/parallel`, `internal/sweep`, `internal/llm`, `internal/headlesstrack`, `internal/plugin` ;
   - `launcher.Launch`, `platform.SessionPlatform` et ses types ;
   - `oh upgrade opencode`, `oh plugin`, la vue Plugins ;
   - le moniteur parallèle et la vue de fusion ;
   - les clés `[opencode] version|channel|auto_update|install_dir` ;
   - `OH_V5`.
3. **Le modèle neutre et l'interface d'adaptateur restent** ([ADR-038](./038-sessionspec-tool-adapters.fr.md)) pour d'autres outils (BL-6). Seul l'adaptateur `opencodev2` existe.
4. **Les anciennes commandes passent toujours par `oh run`**, sans repli :
   - `oh start --agent X` et la commande TUI `coder` lancent le workflow livré `libre` (agent d'entrée au choix, ajout additif `entry.selectable` au schéma `oh/v1`) ;
   - `oh start --resume <id>` devient `oh session attach <id>` ;
   - `--parallel` sans `--tickets` est refusé ;
   - un workflow absent donne une erreur explicite.
5. Les **métriques** (`oh metrics`, tableau de bord, vue Métriques) sont lues dans `oh.db` (`internal/sessionstats`).
6. Les **plugins** d'opencode se déclarent par workflow (`plugins:`, [ADR-039](./039-declarative-workflows-oh-v1.fr.md)) ; un plugin écrit pour V1 (export `server()`) n'est pas chargé par V2. L'[ADR-014](./014-context-mode-plugin.fr.md) (context-mode installé globalement) est donc déprécié.
7. Migration documentée : `docs/guides/migration-v5.{fr,en}.md`, `MIGRATION.md`.

## Conséquences

### Positives

- Un seul chemin de lancement : toutes les garanties de la v5 (monde fermé, checkpoints, secrets hors de l'outil) valent pour chaque session.
- Beaucoup de code supprimé (six paquets, les replis de la CLI et de la TUI, 125 clés i18n orphelines), et des tests sans matrice V1/V2.
- Plus de lecture de la base d'opencode ni de configuration globale de l'utilisateur modifiée par oh (plugins V1).

### Négatives / Compromis

- Un utilisateur encore sous V1 doit passer à V2 avant oh 5 ; il n'y a pas de version de transition.
- Les plugins qui n'existent qu'en V1 sont perdus : `context-mode` ne se charge pas sous V2, et la prise en charge de RTK est retirée.
- `oh upgrade opencode` disparaît : opencode s'installe ou se met à jour hors d'oh. V2 est publié par Homebrew, pas par le paquet npm `opencode-ai`.
- oh dépend d'une seule famille de versions d'un seul outil ; une V3 incompatible bloquerait tous les lancements jusqu'à une mise à jour d'oh.

## Alternatives considérées

| Alternative | Rejetée car |
|---|---|
| Écrire l'adaptateur V1 prévu (capacités réduites) | Coût de développement et de tests pour un outil remplacé ; aucune des fonctions de la v5 n'y aurait été disponible. |
| Garder l'ancien chemin V1 pendant v5.x | Deux produits à maintenir, replis dans chaque commande, et du code de déploiement gardé pour V1 seulement ([ADR-043](./043-session-bundle-deploy-removal.fr.md)). |
| Publier d'abord une version de transition (4.x) compatible V1 et V2 | Retarde la v5 sans éviter la migration ; V2 était déjà la cible et il est disponible. |
