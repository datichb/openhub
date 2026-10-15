> [Read in English](037-documentation-policy.en.md)

# ADR-037 — Politique de documentation

## Statut

Accepté

## Date

2026-10-02

## Contexte

Entre juillet et octobre 2026, le codebase openhub a significativement grandi : 1 171 commits sur 6 mois, 37 packages Go, 19 agents, 197 skills et 7 serveurs MCP. Cependant, le ratio commits documentation/code est passé de 58% (juillet) à 6% (octobre), entrainant :

- 285 commits non documentés dans le CHANGELOG
- 6 sous-systemes majeurs livrés sans documentation utilisateur (mode parallèle, mode sweep, review feedback, notifications, MCP Google Slides, mises à jour plugin RTK)
- Des erreurs factuelles dans la documentation existante (fonctionnalités documentées mais non implémentées)
- Aucun contrôle CI empêchant la dérive documentaire

Un audit rétrospectif (Sprint 1-4) a été nécessaire pour combler le retard. Cet ADR codifie les règles pour prévenir la recurrence.

## Décision

### 1. Règle du companion documentaire

Tout commit avec un préfixe `feat:` qui introduit un changement visible pour l'utilisateur (nouvelle commande CLI, nouveau flag, nouvel outil MCP, nouveau workflow, nouvelle capacité d'agent) **doit** avoir un commit `docs:` correspondant dans la même PR ou dans les 5 jours calendaires. "Visible pour l'utilisateur" signifie tout changement qui modifie un comportement observable via `oh --help`, le TUI, ou les prompts agents.

Les refactors internes (`refactor:`), les changements uniquement de tests (`test:`), et les changements CI (`ci:`) sont exemptés.

### 2. Discipline du CHANGELOG

- `CHANGELOG.md` **doit** être mis à jour avant tout tag de release (`v*`)
- La section `[Unreleased]` ne doit pas avoir plus de **50 commits** de retard sur HEAD (appliqué par la CI)
- Le CHANGELOG est maintenu en **français** (convention du projet) ; cela pourra être reevalue si le projet s'internationalise

### 3. Exigence bilingue

- **Bilingue obligatoire** (FR + EN) : `docs/guides/`, `docs/reference/`, `docs/architecture/adr/`
- **Exemptés du bilingue** : `docs/dev/` (notes internes développeur, monolingues par convention), `docs/design/` (specs visuelles)
- Les fichiers utilisent la convention de nommage `<slug>.{en,fr}.md` avec un lien de changement de langue en ligne 1
- La CI applique la parité : chaque `.fr.md` doit avoir un `.en.md` correspondant et vice versa (appliqué dans les répertoires bilingues obligatoires)

### 4. Exigence ADR

Un Architecture Décision Record est requis quand :
- Un choix de conception implique 2 alternatives ou plus considérées
- Le changement affecte des préoccupations transversales (modèle de sécurité, modèle de données, permissions agents, pipeline CI)
- Le changement est irréversible ou coûteux à reverser

Les ADR suivent le format existant : Statut, Date, Contexte, Décision, Conséquences, Alternatives considérées. Numérotés séquentiellement (`NNN-kebab-case.{en,fr}.md`).

### 5. Application CI

Deux nouveaux checks CI sont ajoutés à `.github/workflows/ci.yml` :
- **Fraîcheur du changelog** : échoue si CHANGELOG.md n'a pas été modifié dans les 50 derniers commits de la branche PR
- **Parité bilingue** : échoue si un `.fr.md` dans `docs/guides/`, `docs/reference/`, ou `docs/architecture/adr/` n'a pas de `.en.md` correspondant (et vice versa)

### 6. Template de PR

Un template de pull request (`.github/pull_request_template.md`) inclut une checklist documentation : tests passent, lint passe, documentation mise à jour (si visible utilisateur), CHANGELOG mis à jour (si visible utilisateur), docs bilingues fournies.

## Conséquences

### Positives

- La dette documentaire est structurellement prevenue, pas seulement retroactivement corrigée
- L'application CI supprime la dépendance à la discipline développeur pour le changelog et la parité bilingue
- Le template PR rend la documentation une étape visible et vérifiable dans chaque contribution
- Les exemptions claires (`docs/dev/`, `docs/design/`) évitent une charge inutile pour les notes internes

### Négatives / Compromis

- Léger overhead par PR pour les commits companion documentation (~15 min par feature)
- Les checks CI peuvent occasionnellement bloquer des PR légitimes qui reportent intentionnellement la documentation (utiliser `[skip-docs]` dans la description PR pour contourner, logue pour suivi)
- Le CHANGELOG uniquement en français est inconsistant avec les guides bilingues, mais évite de maintenir une traduction de 1600 lignes

## Alternatives considérées

| Alternative | Rejetée car |
|-------------|------------|
| Génération automatique du changelog depuis les messages de commit | Perd le regroupement thématique humain qui rend le changelog lisible |
| Documentation uniquement en anglais | Casse la convention FR-first existante et alienate la base de contributeurs actuelle |
| Pas d'application CI (convention uniquement) | La dérive de 58% à 6% sur 6 mois prouve que la convention seule est insuffisante |
| Bilingue obligatoire pour tous les docs y compris `docs/dev/` | Charge excessive pour les notes internes rarement consultees hors de l'équipe core |
