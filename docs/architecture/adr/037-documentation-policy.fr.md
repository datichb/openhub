> [Read in English](037-documentation-policy.en.md)

# ADR-037 — Politique de documentation

## Statut

Accepte

## Date

2026-10-02

## Contexte

Entre juillet et octobre 2026, le codebase openhub a significativement grandi : 1 171 commits sur 6 mois, 37 packages Go, 19 agents, 197 skills et 7 serveurs MCP. Cependant, le ratio commits documentation/code est passe de 58% (juillet) a 6% (octobre), entrainant :

- 285 commits non documentes dans le CHANGELOG
- 6 sous-systemes majeurs livres sans documentation utilisateur (mode parallele, mode sweep, review feedback, notifications, MCP Google Slides, mises a jour plugin RTK)
- Des erreurs factuelles dans la documentation existante (fonctionnalites documentees mais non implementees)
- Aucun controle CI empechant la derive documentaire

Un audit retrospectif (Sprint 1-4) a ete necessaire pour combler le retard. Cet ADR codifie les regles pour prevenir la recurrence.

## Decision

### 1. Regle du companion documentaire

Tout commit avec un prefixe `feat:` qui introduit un changement visible pour l'utilisateur (nouvelle commande CLI, nouveau flag, nouvel outil MCP, nouveau workflow, nouvelle capacite d'agent) **doit** avoir un commit `docs:` correspondant dans la meme PR ou dans les 5 jours calendaires. "Visible pour l'utilisateur" signifie tout changement qui modifie un comportement observable via `oh --help`, le TUI, ou les prompts agents.

Les refactors internes (`refactor:`), les changements uniquement de tests (`test:`), et les changements CI (`ci:`) sont exemptes.

### 2. Discipline du CHANGELOG

- `CHANGELOG.md` **doit** etre mis a jour avant tout tag de release (`v*`)
- La section `[Unreleased]` ne doit pas avoir plus de **50 commits** de retard sur HEAD (applique par la CI)
- Le CHANGELOG est maintenu en **francais** (convention du projet) ; cela pourra etre reevalue si le projet s'internationalise

### 3. Exigence bilingue

- **Bilingue obligatoire** (FR + EN) : `docs/guides/`, `docs/reference/`, `docs/architecture/adr/`
- **Exemptes du bilingue** : `docs/dev/` (notes internes developpeur, monolingues par convention), `docs/design/` (specs visuelles)
- Les fichiers utilisent la convention de nommage `<slug>.{en,fr}.md` avec un lien de changement de langue en ligne 1
- La CI applique la parite : chaque `.fr.md` doit avoir un `.en.md` correspondant et vice versa (applique dans les repertoires bilingues obligatoires)

### 4. Exigence ADR

Un Architecture Decision Record est requis quand :
- Un choix de conception implique 2 alternatives ou plus considerees
- Le changement affecte des preoccupations transversales (modele de securite, modele de donnees, permissions agents, pipeline CI)
- Le changement est irreversible ou couteux a reverser

Les ADR suivent le format existant : Statut, Date, Contexte, Decision, Consequences, Alternatives considerees. Numerotes sequentiellement (`NNN-kebab-case.{en,fr}.md`).

### 5. Application CI

Deux nouveaux checks CI sont ajoutes a `.github/workflows/ci.yml` :
- **Fraicheur du changelog** : echoue si CHANGELOG.md n'a pas ete modifie dans les 50 derniers commits de la branche PR
- **Parite bilingue** : echoue si un `.fr.md` dans `docs/guides/`, `docs/reference/`, ou `docs/architecture/adr/` n'a pas de `.en.md` correspondant (et vice versa)

### 6. Template de PR

Un template de pull request (`.github/pull_request_template.md`) inclut une checklist documentation : tests passent, lint passe, documentation mise a jour (si visible utilisateur), CHANGELOG mis a jour (si visible utilisateur), docs bilingues fournies.

## Consequences

### Positives

- La dette documentaire est structurellement prevenue, pas seulement retroactivement corrigee
- L'application CI supprime la dependance a la discipline developpeur pour le changelog et la parite bilingue
- Le template PR rend la documentation une etape visible et verifiable dans chaque contribution
- Les exemptions claires (`docs/dev/`, `docs/design/`) evitent une charge inutile pour les notes internes

### Negatives / Compromis

- Leger overhead par PR pour les commits companion documentation (~15 min par feature)
- Les checks CI peuvent occasionnellement bloquer des PR legitimes qui reportent intentionnellement la documentation (utiliser `[skip-docs]` dans la description PR pour contourner, logue pour suivi)
- Le CHANGELOG uniquement en francais est inconsistant avec les guides bilingues, mais evite de maintenir une traduction de 1600 lignes

## Alternatives considerees

| Alternative | Rejetee car |
|-------------|------------|
| Generation automatique du changelog depuis les messages de commit | Perd le regroupement thematique humain qui rend le changelog lisible |
| Documentation uniquement en anglais | Casse la convention FR-first existante et alienate la base de contributeurs actuelle |
| Pas d'application CI (convention uniquement) | La derive de 58% a 6% sur 6 mois prouve que la convention seule est insuffisante |
| Bilingue obligatoire pour tous les docs y compris `docs/dev/` | Charge excessive pour les notes internes rarement consultees hors de l'equipe core |
