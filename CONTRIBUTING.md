# Contributing / Contribuer

> **[English]** Full contribution guide: [docs/guides/contributing.en.md](docs/guides/contributing.en.md)
>
> **[Francais]** Guide de contribution complet : [docs/guides/contributing.fr.md](docs/guides/contributing.fr.md)

## Quick Start (code contributors)

```bash
# Prerequisites: Go 1.26.4+ (the CI uses go1.26.10), golangci-lint v2.14
git clone https://github.com/datichb/openhub.git
cd openhub/cli
make deps          # go mod tidy + download
make embed-sync    # sync agents/skills/permissions into Go embed
make build         # produces bin/oh
make test          # all tests with -race
make test-shards   # the suite split like the CI does, all shards at once (faster, no -race)
make lint          # golangci-lint
```

See the full guides for agent/skill contributions and conventions. Maintainers cutting a release of oh itself follow [docs/dev/release-checklist.md](docs/dev/release-checklist.md) (French).
