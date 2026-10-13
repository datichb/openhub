---
updated: 2026-10-01
confidence: mixed
agents: [onboarder, documentarian]
---

# Atlas — Index Wiki

## Stack critique
Go 1.24 (API, workers), React 19 + TypeScript (web), PostgreSQL 16, NATS JetStream.
— `CONFIRMÉ` · onboarder · 2026-09-12 · go.mod

## Architecture (résumé)
Monorepo ; API en couches Controller → Service → Repository ; workers événementiels idempotents.
— `CONFIRMÉ` · onboarder · 2026-09-12

## God nodes — concepts les plus connectés

| Concept | Pages liées | Criticité |
|---------|-------------|-----------|
| Commande | [business/orders.md](business/orders.md), [technical/architecture.md#events](technical/architecture.md#events) | Critique |
| Facturation et TVA | [business/billing.md](business/billing.md), [technical/conventions.md#billing](technical/conventions.md#billing) | Critique |
| Authentification | [technical/security.md](technical/security.md), [business/accounts.md](business/accounts.md) | Haute |

## Carte des domaines métier

- [Comptes](business/accounts.md) — inscription, rôles, équipes
- [Commandes](business/orders.md) — panier, paiement, remboursement
- [Facturation](business/billing.md) — factures, TVA par pays

## Points critiques actifs 🔴

- Calcul de TVA intra-UE non couvert par les tests d'intégration — `INCERTAIN` · reviewer · 2026-09-30

## Zones d'ombre

- Politique de rétention des exports
