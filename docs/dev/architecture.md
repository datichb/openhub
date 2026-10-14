# Architecture du CLI oh — page déplacée

Cette page décrivait le CLI de juillet 2026, avant la v5 (déploiement par projet, `SessionPlatform`, opencode V1, TUI Bubble Tea). Elle était périmée et redondante ; son contenu historique reste dans l'historique git (`git log -p -- docs/dev/architecture.md`). Elle reste un simple renvoi, pour ne pas casser les liens qui y mènent : l'architecture se tient à jour à un seul endroit, en paire fr/en, dans `docs/architecture/`.

L'architecture actuelle se lit ici :

| Sujet | Document |
|-------|----------|
| Vue système, concepts, flux d'une session, composition des workflows, vérification par oh, wiki, fan-out, principes de design, structure des fichiers | [Vue d'ensemble de l'architecture](../architecture/overview.fr.md) ([en](../architecture/overview.en.md)) |
| Agents, contrat de session, instructions par niveau, catalogue de skills par agent | [Agents](../architecture/agents.fr.md), [Skills](../architecture/skills.fr.md), [Délégation inter-agents](../architecture/task-delegation.fr.md) |
| Workflows livrés, schéma `oh/v1`, checkpoints et vérification par oh | [Workflows livrés](../reference/workflows.fr.md), [Schéma](../reference/workflow-schema.fr.md), [Checkpoints](../reference/checkpoints.fr.md) |
| Packages du CLI et conventions de code (commandes, services, stockage, i18n, tests) | [Contribuer — Vue d'ensemble du code](../guides/contributing.fr.md#vue-densemble-du-code) ([en](../guides/contributing.en.md#code-overview)) |
| Décisions d'architecture (v5 : ADR 038 à 053) | [`docs/architecture/adr/`](../architecture/adr/) |
| Shell, vues et widgets de la TUI (tview) | [TUI Architecture](tui-architecture.md), [TUI Testing](tui-testing.md) |
| Indépendance vis-à-vis de l'outil agentique (règle vérifiée par les tests) | `cli/internal/archtest/` |
