---
name: auditor-phase-0-1
description: "Utiliser quand l'auditor démarre un audit (phases 0 et 1) : vérifier périmètre, stack et accès, puis charger le contexte projet."
---

# Auditor — Phases 0 et 1

## Phase 0 — Vérification des prérequis

Trois conditions, vérifiées avant toute délégation :

1. **Périmètre clair** : le ou les domaines à auditer, les fichiers, modules ou endpoints visés (ou « tout le projet »), les contraintes légales ou référentiels imposés (ex. : RGAA niveau AA, RGPD pour des données de santé).
2. **Stack identifiable** : la reconnaissance de la phase 1 permet de déterminer au moins le langage et le framework principal. Insuffisant : projet sans fichier de dépendances lisible, structure non standard.
3. **Accès aux fichiers pertinents** : les répertoires sources principaux sont lisibles (pas uniquement du code compilé ou minifié, pas uniquement de l'infra sans code applicatif quand le domaine vise l'application).

### Pause ⏸️ — informations manquantes

Une ou plusieurs conditions non remplies : affiche en texte ce qui manque (périmètre, stack, accès) et l'impact (sans ces éléments, le sous-agent signalera des limites importantes dans son rapport), puis regroupe toutes les questions en **un seul** appel :

```
question({
  questions: [{
    header: "Informations manquantes",
    question: "[Auditeur — Phase 0 : Prérequis | Projet : <nom>]\nPour déléguer dans de bonnes conditions, j'ai besoin des précisions listées ci-dessus. Comment procéder ?",
    options: [
      { label: "Fournir les précisions", description: "Préciser le périmètre, la stack ou les chemins d'accès manquants" },
      { label: "Lancer quand même", description: "Démarrer l'audit avec les informations disponibles — le sous-agent signalera les limites" }
    ]
  }]
})
```

### Récap de fin de phase 0

```markdown
## [Phase 0] Prérequis vérifiés

**Périmètre :** domaines <liste> · cible <fichiers/modules ou « tout le projet »> · contraintes légales <… ou « aucune »>
**Accès :** répertoires sources lisibles <oui / partiellement / non> · limites <ex. : uniquement du code compilé>
**Prochaine étape :** chargement du contexte projet (phase 1)
```

Validation : « Charger le contexte » (recommandé, phase 1), « Préciser le périmètre » (rester en phase 0), « Arrêter ».

## Phase 1 — Chargement du contexte projet

But : la stack, l'architecture et les points d'attention, à transmettre au sous-agent.

1. **`ONBOARDING.md` à la racine** : lis-le en priorité et extrais la stack (langages, frameworks, base de données, infrastructure, outils), l'architecture (patterns, découpage), les points d'attention (dette, zones de risque, contraintes) et sa date (fraîcheur du contexte). Il sert de base à toute la session : ne ré-explore pas le projet.
2. **Sinon, reconnaissance rapide (3 à 4 fichiers seulement)** :
   - le fichier de dépendances racine (`package.json`, `composer.json`, `requirements.txt`, `pom.xml`, `Cargo.toml`…) : langage, framework, dépendances critiques (ORM, client HTTP, authentification) ;
   - la structure des répertoires principaux (`src/`, `app/`, `lib/`…) : pattern d'architecture (MVC, hexagonal, monorepo, microservices), configuration, tests, infra ;
   - 1 ou 2 fichiers de configuration (`.env.example`, `docker-compose.yml`, `nginx.conf`, `tsconfig.json`…) : services externes (base, cache, broker), variables sensibles ;
   - résume en 5 lignes (stack, architecture, points d'attention visibles) et suggère à l'utilisateur de lancer `oh run onboarding` pour enrichir les prochains audits.

### Pause ⏸️ — stack non identifiable

Aucun fichier de dépendances lisible, structure non standard, langage non identifiable : affiche ces constats et l'impact (le sous-agent ne pourra pas calibrer son analyse), puis :

```
question({
  questions: [{
    header: "Stack non identifiable",
    question: "[Auditeur — Phase 1 : Contexte | Projet : <nom>]\nStack non identifiable. Comment procéder ?",
    options: [
      { label: "Préciser la stack", description: "Indiquer le langage, le framework et l'architecture" },
      { label: "Lancer l'onboarding d'abord", description: "Arrêter l'audit et lancer oh run onboarding pour une exploration complète" },
      { label: "Continuer quand même", description: "Auditer sans contexte de stack — le sous-agent fera au mieux" }
    ]
  }]
})
```

### Récap de fin de phase 1

```markdown
## [Phase 1] Contexte projet chargé

**Source :** ONBOARDING.md (généré le <date>) | reconnaissance rapide (3-4 fichiers)
**Stack :** langages <…> · frameworks <…> · base de données <…> · infrastructure <…> · outils <CI/CD, tests, lint>
**Architecture :** pattern <…> · découpage <répertoires principaux et leur rôle>
**Points d'attention :** <liste, ou « Aucun »>
**Prochaine étape :** sélection des domaines (phase 2)
```

Validation : « Sélectionner les domaines » (recommandé, phase 2), « Recharger le contexte » (rester en phase 1, refaire le récap), « Arrêter ».
