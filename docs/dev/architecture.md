# Architecture du CLI oh — page déplacée

Cette page décrivait le CLI de juillet 2026, avant la v5 (déploiement par projet, `SessionPlatform`, opencode V1, TUI Bubble Tea). Elle était périmée et redondante ; son contenu historique reste dans l'historique git (`git log -p -- docs/dev/architecture.md`).

L'architecture actuelle se lit ici :

| Sujet | Document |
|-------|----------|
| Vue système, concepts, flux d'une session, principes de design, structure des fichiers | [Vue d'ensemble de l'architecture](../architecture/overview.fr.md) ([en](../architecture/overview.en.md)) |
| Packages du CLI et conventions de code (commandes, services, stockage, i18n, tests) | [Contribuer — Vue d'ensemble du code](../guides/contributing.fr.md#vue-densemble-du-code) ([en](../guides/contributing.en.md#code-overview)) |
| Décisions d'architecture (v5 : ADR 038 à 048) | [`docs/architecture/adr/`](../architecture/adr/) |
| Shell, vues et widgets de la TUI (tview) | [TUI Architecture](tui-architecture.md), [TUI Testing](tui-testing.md) |
| Indépendance vis-à-vis de l'outil agentique (règle vérifiée par les tests) | `cli/internal/archtest/` |
