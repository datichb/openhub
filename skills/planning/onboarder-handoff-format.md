---
name: onboarder-handoff-format
description: Source de vérité pour le format de retour de l'onboarder vers l'orchestrator. Définit le bloc structuré unique à produire quand l'onboarder termine son exploration et est invoqué depuis l'orchestrator (Mode C). Le rapport d'onboarding est intégré dans le bloc. Injecté dans l'onboarder et dans l'orchestrator pour garantir que producteur et consommateur partagent le même contrat.
---

# Skill — Format de handoff onboarder → orchestrator

Ce skill est la **source de vérité** pour le format de retour de l'`onboarder` vers l'orchestrator.
Il est injecté dans l'`onboarder` et dans l'`orchestrator` — producteur et consommateur partagent le même contrat.

---

## Principe fondamental — bloc unique

Quand tu es invoqué depuis l'`orchestrator` (Mode C — projet inconnu),
ton **seul output** est le bloc `## Retour vers orchestrator` défini ci-dessous.

**Règle absolue :** aucun texte avant, après ou en dehors de ce bloc. Le rapport d'onboarding (contexte de découverte, observations narratives) est **intégré dans le bloc** (section `### Rapport d'onboarding`), pas produit séparément en texte libre.

> **Autocontrôle obligatoire avant de terminer la session :**
> « Mon output contient-il du texte en dehors du bloc `## Retour vers orchestrator` ? Si oui, le supprimer et vérifier que le rapport est bien dans la section `### Rapport d'onboarding` du bloc. »

---

## Format du bloc `## Retour vers orchestrator`

```
---

## Retour vers orchestrator

**Agent :** onboarder
**Projet :** <nom du projet>

### Rapport d'onboarding

<Contexte de découverte narratif : comment les éléments ont été trouvés, ce qui a été surprenant ou notable, zones d'incertitude avec leur contexte. Ce texte apporte les observations qualitatives qui ne sont pas encodables dans les listes structurées ci-dessous. Minimum 3-5 phrases.>

<Ex : "Le projet utilise une architecture CQRS non documentée mais détectable via la séparation commands/queries dans src/. La couverture de tests est à 73% mais concentrée sur les commandes — les queries sont presque non testées. Un design system DSFR est configuré mais semble partiellement abandonné (tokens obsolètes dans le fichier theme.ts).">

### Stack technique
**Langages :** <liste>
**Frameworks :** <liste>
**Base de données :** <liste>
**Infrastructure :** <liste — cloud, containers, etc.>
**Outils :** <liste — CI/CD, tests, linting, etc.>
**Versions clés :** <ex : Node 20, PHP 8.2, Python 3.11, etc.>

### Contexte métier
**Domaine(s) :** <liste> (ou "Non identifié — projet générique")
**Utilisateurs cibles :** <liste> (ou "Non documentés")
**Concepts clés :** <liste des concepts métier récurrents>
**Glossaire :** <Présent dans docs/glossary.md / Absent>
**Pattern architecture :** <DDD / CQRS / Layered / MVC / Non documenté>

### Design et maquettes
**Fichiers Figma :** <X fichiers — [URLs]> (ou "Aucun fichier détecté")
**Design system :** <DSFR / Material / Custom / Aucun>
**Design tokens :** <X tokens couleur, Y typo, Z spacing / Non configurés>
<"Non applicable (projet backend)" si pas de frontend>

### Stratégie de test
**Frameworks :** <unitaires : X, E2E : Y>
**Seuil couverture :** <X% configuré / Non configuré>
**Ratio test/source :** <calculé>
**Philosophie :** <TDD / BDD / Test-after>

### Conventions identifiées
- <convention 1 — ex : nommage en camelCase pour les variables, PascalCase pour les composants>
- <convention 2 — ex : tests unitaires avec Jest, colocalisés avec le code source>
- <convention 3 — ex : branches au format feature/<ID>-<description>>
<"Conventions non déterminables sans clarification" si aucune convention claire n'a pu être détectée>

### Dette technique détectée
- 🔴 <dette critique 1 — ex : dépendances avec CVE critiques connues>
- 🟠 <dette importante 1 — ex : absence de tests sur les composants métier principaux>
- 🟡 <dette mineure 1 — ex : fichiers de configuration dupliqués>
<"Aucune dette technique identifiée" si le projet est en bon état>

### Zones d'incertitude
- <point 1 — ce que l'exploration n'a pas pu déterminer et qui pourrait impacter la feature>
- <point 2 — question à poser à l'utilisateur avant de démarrer>
<"Aucune zone d'incertitude" si le projet est bien documenté et sans ambiguïté>

### Fichiers de contexte produits
- `ONBOARDING.md` — <créé | mis à jour | non créé (raison)>
- `CONVENTIONS.md` — <créé | mis à jour | non créé (raison)>
- `docs/context/technical.md` — <créé | mis à jour | non créé (raison)>
- `docs/context/business/` — <liste des fichiers créés/mis à jour, ex : auth.md, billing.md | aucun>

### Statut
`contexte-établi` | `contexte-partiel` | `bloqué`
```

**Définitions du statut :**

| Statut | Condition |
|--------|-----------|
| `contexte-établi` | Exploration complète, fichiers produits, contexte suffisant pour démarrer la feature |
| `contexte-partiel` | Exploration réalisée mais avec des zones d'incertitude significatives — feature démarrable avec précautions |
| `bloqué` | Exploration impossible ou contexte insuffisant pour démarrer — intervention manuelle requise |

---

## Règles pour le producteur (onboarder)

- **Produire UNIQUEMENT le bloc `## Retour vers orchestrator`** — aucun texte avant ou après
- **Le rapport d'onboarding est DANS le bloc** (section `### Rapport d'onboarding`) — ne pas le produire séparément en texte libre
- **`### Rapport d'onboarding`** doit capturer les observations qualitatives — minimum 3-5 phrases
- **Renseigner toutes les sections** — même si vides, utiliser la mention explicite correspondante
- **Ne pas inventer** de conventions ou de stack — uniquement ce qui a été effectivement observé dans la codebase
- **Signaler honnêtement les zones d'incertitude** — l'orchestrator en a besoin pour informer l'utilisateur avant de démarrer
- Ce bloc est produit **après** l'écriture des fichiers (ou après refus explicite de les écrire)

> ❌ Ne jamais écrire de texte en dehors du bloc de handoff
> ❌ Ne jamais produire de rapport narratif séparé avant le bloc — il est DANS le bloc
> ❌ Ne jamais omettre `### Rapport d'onboarding` — les listes structurées seules ne suffisent pas

---

## Règles pour le consommateur (orchestrator)

**Spécificités onboarder à vérifier :**

- **Champs obligatoires** : `Rapport d'onboarding`, `Stack technique`, `Contexte métier`, `Design et maquettes`, `Stratégie de test`, `Conventions identifiées`, `Dette technique détectée`, `Zones d'incertitude`, `Fichiers de contexte produits`, `Statut`. Si l'un est absent → demander à l'onboarder de compléter.
- **Retranscription** : afficher les champs du bloc de manière formatée dans la discussion (voir skill `retranscription-coordinateur`). Le `### Rapport d'onboarding` est affiché en premier.
- **CP-onboard** : présenter `### Zones d'incertitude` à l'utilisateur pour décision, signaler les éléments 🔴 de `### Dette technique détectée`.
- **Délégation** : intégrer `### Stack technique` dans le prompt de délégation à `orchestrator-dev`.
- **Statut** : `contexte-établi` → CP-onboard normal · `contexte-partiel` → signaler les incertitudes · `bloqué` → ne pas démarrer la feature.
