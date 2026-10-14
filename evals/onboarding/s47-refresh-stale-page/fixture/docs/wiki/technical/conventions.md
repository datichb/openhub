---
updated: 2026-10-01
confidence: mixed
agents: [onboarder]
---

# Conventions — todo

## Linting & formatage
- `gofmt` ; aucun linter configuré dans le dépôt.
  — `DÉDUIT` · onboarder · 2026-10-01 · main.go

## Nommage
- Commentaires de documentation et messages d'erreur en français.
  — `CONFIRMÉ` · onboarder · 2026-10-01 · main.go:1

## Git
- Aucune convention écrite dans le dépôt.
  — `DÉDUIT` · onboarder · 2026-10-01 · README.md

## Configuration & secrets
- Aucune configuration ni secret.
  — `CONFIRMÉ` · onboarder · 2026-10-01 · main.go

## Patterns spécifiques à l'équipe
- Les erreurs sont écrites sur la sortie d'erreur, puis `os.Exit(1)` ; un usage invalide sort avec le code 2.
  — `CONFIRMÉ` · onboarder · 2026-10-01 · main.go:14

## À ne pas utiliser
- Aucune dépendance externe.
  — `DÉDUIT` · onboarder · 2026-10-01 · go.mod
