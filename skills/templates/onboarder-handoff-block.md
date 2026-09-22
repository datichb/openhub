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
