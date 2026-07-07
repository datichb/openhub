---
name: doc-adr
description: Format et protocole des ADR (Architecture Decision Records) — détection du format existant, format MADR de référence, règles de nommage, statuts, critères de création.
---

# Skill — ADR (Architecture Decision Records)

## Principe

Un ADR documente une décision d'architecture significative : le contexte qui l'a motivée,
la décision prise, ses conséquences et les alternatives rejetées.
Un ADR est **immuable** une fois accepté — on ne le modifie pas, on le supersède.

---

## Étape 0 — Détecter le format existant

Avant de créer un ADR, toujours explorer ce qui existe :

```bash
# Emplacements courants des ADR
find . -name "*.md" -path "*/adr/*" 2>/dev/null
find . -name "*.md" -path "*/decisions/*" 2>/dev/null
find . -name "*.md" -path "*/decision-records/*" 2>/dev/null
find . -name "*.md" -path "*/architectural-decisions/*" 2>/dev/null

# Formats de nommage courants
ls docs/adr/ 2>/dev/null
ls doc/adr/ 2>/dev/null
ls architecture/decisions/ 2>/dev/null
```

Lire au moins **2 ADR existants** pour identifier :
- Le format utilisé (MADR ? Nygard ? Y-Statements ? format maison ?)
- La numérotation (3 chiffres ? 4 chiffres ? sans numéro ?)
- La langue (français ? anglais ?)
- L'emplacement des fichiers

**S'y conformer strictement**, sauf si l'utilisateur demande explicitement un changement.

---

## Formats courants — identification

### Format Nygard (original, 2011)

Reconnaissable à ses 5 sections courtes :

```markdown
# [Numéro]. [Titre]

Date: YYYY-MM-DD

## Status
[Proposed | Accepted | Deprecated | Superseded by ADR-NNN]

## Context
[Situation et forces en présence]

## Decision
[Ce qui a été décidé]

## Consequences
[Conséquences positives et négatives]
```

### Format MADR (Markdown Architectural Decision Records)

Reconnaissable à ses sections `Considered Options` et `Decision Outcome` :

```markdown
# [Titre]

## Status
[proposed | accepted | deprecated | superseded by ADR-NNN]

## Context and Problem Statement
[Description du problème]

## Decision Drivers
- [Driver 1]

## Considered Options
- Option A
- Option B

## Decision Outcome
Chosen option: "Option A", because [justification].

### Positive Consequences
- [...]

### Negative Consequences
- [...]

## Pros and Cons of the Options

### Option A
- Good, because [...]
- Bad, because [...]
```

### Format Y-Statements

Reconnaissable à sa phrase en une ligne :

```markdown
In the context of [situation],
facing [concern],
we decided [option],
to achieve [quality],
accepting [downside].
```

### Format maison

Si aucun format standard n'est reconnu, noter les sections présentes dans les ADR
existants et les reproduire à l'identique.

---

## Format de référence — MADR léger (hub)

Utilisé dans le hub et proposé par défaut quand aucun format n'existe.
5 sections, concis, sans les "Pros and Cons" détaillés sauf si pertinent.

```markdown
# NNN — Titre en kebab-case développé

## Statut

accepted | proposed | deprecated | superseded by [ADR-NNN](NNN-titre.md)

## Contexte

[Situation qui a motivé la décision. Quelles contraintes, quels besoins,
quels problèmes à résoudre. 3-10 phrases.]

## Décision

[Ce qui a été décidé, exprimé clairement. Commencer par "Nous avons décidé de..."
ou "Nous utilisons...". 1-5 phrases.]

## Conséquences

### Positives
- [Bénéfice concret]
- [Bénéfice concret]

### Négatives / Compromis
- [Contrainte acceptée]
- [Risque à surveiller]

## Alternatives rejetées

| Alternative | Raison du rejet |
|-------------|----------------|
| [Option A] | [Pourquoi rejetée] |
| [Option B] | [Pourquoi rejetée] |
```

---

## Règles de nommage

### Numérotation

- Format à 3 chiffres : `001`, `002`, ... `099`, `100`
- Séquentiel — ne jamais réutiliser un numéro, même si un ADR est supprimé
- Si le projet utilise déjà 4 chiffres (`0001`), s'y conformer

### Nom de fichier

```
NNN-titre-en-kebab-case.md
```

Exemples :
```
001-choix-du-framework-frontend.md
002-strategie-authentification.md
003-architecture-microservices.md
```

Règles :
- Tout en minuscules
- Tirets comme séparateurs (pas d'underscores)
- Pas d'articles (`le`, `la`, `les`, `the`, `a`) sauf si indispensable au sens
- Maximum 60 caractères au total (numéro inclus)

### Titre dans le document

Le titre dans le fichier doit correspondre au nom de fichier, développé :

```markdown
# 003 — Architecture microservices pour les services de paiement
```

---

## Statuts

| Statut | Signification |
|--------|--------------|
| `proposed` | En cours de discussion — pas encore validé |
| `accepted` | Décision prise et appliquée |
| `deprecated` | N'est plus applicable mais pas remplacé |
| `superseded by ADR-NNN` | Remplacé par un ADR plus récent (avec lien) |

**Règle de supersession :**
Quand une décision est révisée, ne pas modifier l'ADR existant.
Créer un nouvel ADR et mettre à jour le statut de l'ancien :

```markdown
## Statut

superseded by [ADR-007](007-nouveau-choix-framework.md)
```

---

## Quand créer un ADR

Créer un ADR pour toute décision qui répond à **au moins un** de ces critères :

- **Irréversible ou coûteuse à changer** : choix de base de données, framework, langage
- **Non évidente** : la raison du choix n'est pas immédiatement claire pour un nouveau développeur
- **Avec alternatives significatives** : plusieurs options ont été envisagées
- **Avec compromis importants** : la décision a des conséquences négatives connues
- **Structurante** : affecte l'architecture globale ou plusieurs composants

### Ne PAS créer un ADR pour

- Des décisions tactiques facilement réversibles (nommage de variable, organisation de dossier)
- Des décisions imposées par des contraintes externes sans alternative (obligation légale, standard imposé)
- Des décisions déjà documentées dans une RFC ou un ticket de spec

---

## Checklist avant de créer un ADR

- [ ] Vérifier le format des ADR existants (Étape 0)
- [ ] Déterminer le prochain numéro disponible
- [ ] Le contexte décrit clairement le problème (pas la solution)
- [ ] La décision est exprimée comme une action passée ("nous avons décidé")
- [ ] Au moins une alternative est mentionnée avec la raison de son rejet
- [ ] Les conséquences négatives sont honnêtement documentées
- [ ] Le fichier est dans le bon répertoire avec le bon nom
- [ ] Le statut est `accepted` (si la décision est prise) ou `proposed` (si en discussion)

---

## Exemple complet — MADR léger

```markdown
# 023 — Suppression de l'agent qa-engineer et du checkpoint CP-QA

## Statut

accepted

## Contexte

L'agent `qa-engineer` intervenait entre l'implémentation (developer) et la review
(reviewer) pour écrire les tests manquants. En pratique, le developer écrivait déjà
des tests, rendant le passage QA souvent redondant. La pre-review exécutait aussi
les tests automatiquement. Le checkpoint CP-QA ajoutait ~220 lignes de protocole
et un cycle complet de latence au workflow.

## Décision

Nous avons décidé de supprimer l'agent `qa-engineer` et le checkpoint CP-QA.
La responsabilité de couverture des tests est transférée au `developer`
(via le skill `dev-standards-testing` enrichi). Le `reviewer` vérifie la couverture
des critères d'acceptance et peut demander des tests supplémentaires.

## Conséquences

### Positives
- Workflow simplifié : Developer → Pre-review → Reviewer → CP-2
- Moins de latence : suppression d'un cycle agent complet
- Protocole orchestrator allégé de ~220 lignes

### Négatives / Compromis
- Perte de la spécialisation QA concentrée
- Le developer doit appliquer la checklist de couverture en plus de l'implémentation

## Alternatives rejetées

| Alternative | Raison du rejet |
|-------------|----------------|
| Garder le QA en risque élevé uniquement | Ne résout ni la latence ni la redondance |
| Fusionner QA dans le reviewer | Le reviewer est read-only — ne peut pas écrire de tests |
```
