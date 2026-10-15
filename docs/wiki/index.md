---
updated: 2026-10-15
confidence: confirmed
agents: [onboarder, developer, documentarian]
---

# openhub (oh) — Index du wiki

Wiki vivant du dépôt openhub lui-même. Les pages sont bilingues (`x.fr.md` / `x.en.md`) : chacune commence par son frontmatter, suivi du lien vers l'autre langue. Cet index est le point d'entrée, en français.

*The living wiki of the openhub repository itself. Pages exist in French and English (`x.fr.md` / `x.en.md`); this index is in French.*

## Stack critique

CLI en Go (binaire statique, `CGO_ENABLED=0`), Cobra et Viper, SQLite en Go pur, TUI tview ; sessions opencode V2 pilotées par le démon `ohd` ; agents, skills, permissions et workflows embarqués (`go:embed`). — `CONFIRMÉ` · onboarder · 2026-10-06 · cli/go.mod

## Architecture (résumé)

Un workflow déclaratif `oh/v1` est résolu par couches (hub, équipe, projet), compilé en paquet de session immuable, puis lancé par le RunService dans un monde fermé (seuls les agents et skills du paquet). — `CONFIRMÉ` · onboarder · 2026-10-06 · cli/internal/bundle/build.go

## Pages

| Page | Sujet |
|------|-------|
| [Architecture](technical/architecture.fr.md) ([en](technical/architecture.en.md)) | Architecture v5 : CLI Go, workflows, paquet de session, adaptateurs, démon `ohd`, passerelles, environnements d'exécution, serveurs MCP, TUI |
| [Stack](technical/stack.fr.md) ([en](technical/stack.en.md)) | Stack technique et dépendances |
| [Conventions](technical/conventions.fr.md) ([en](technical/conventions.en.md)) | Style de code, format des commits, branches, revue, nommage des fichiers |
| [Cascade de configuration](technical/config-cascade.fr.md) ([en](technical/config-cascade.en.md)) | Résolution de la configuration entre équipe, hub et projet, workflows, environnement, limites de session |

## God nodes — concepts les plus connectés

| Concept | Pages liées | Criticité |
|---------|-------------|-----------|
| Sessions | [technical/architecture.en.md](technical/architecture.en.md), [technical/architecture.fr.md](technical/architecture.fr.md), [technical/config-cascade.en.md](technical/config-cascade.en.md), [technical/config-cascade.fr.md](technical/config-cascade.fr.md), [technical/conventions.en.md](technical/conventions.en.md), [technical/conventions.fr.md](technical/conventions.fr.md) | Critique |
| Contenu embarque | [technical/architecture.fr.md](technical/architecture.fr.md), [technical/stack.fr.md](technical/stack.fr.md) | Normale |
| Embedded content | [technical/architecture.en.md](technical/architecture.en.md), [technical/stack.en.md](technical/stack.en.md) | Normale |
| modelOverridesFor | [technical/config-cascade.en.md](technical/config-cascade.en.md), [technical/config-cascade.fr.md](technical/config-cascade.fr.md) | Normale |

## Carte des domaines métier

*(Aucun domaine métier : oh est un outil.)*

## Fonctionnement

- Les pages sont créées par l'`onboarder` pendant `oh run onboarding` et enrichies par les agents au fil des sessions (`wiki_propose`, appliqué par oh après validation) ; `oh wiki check` vérifie le format.
- Conception : [ADR-024](../architecture/adr/024-team-state-repository.fr.md) et [Wiki vivant](../architecture/living-wiki.fr.md).

## Zones d'ombre

*(Aucune relevée.)*
