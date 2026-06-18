# Architecture du cache — openhub

## Vue d'ensemble

openhub utilise deux niveaux de cache complémentaires, chacun avec un rôle distinct.

---

## Deux niveaux de cache

### Cache natif OpenCode (API / Runtime)

**Rôle :** Réutiliser le contexte de prompt entre requêtes API côté fournisseur.

**Configuration :** `opencode.json` (racine du hub)

```json
{
  "provider": {
    "anthropic": {
      "options": {
        "setCacheKey": true
      }
    }
  },
  "compaction": {
    "auto": true,
    "prune": true,
    "reserved": 10000
  }
}
```

**Options :**
| Clé | Description |
|-----|-------------|
| `setCacheKey` | Maintient un cache key stable pour réutilisation côté Anthropic (économie de tokens ~30-50%) |
| `compaction.auto` | Compact automatiquement le contexte quand il est plein |
| `compaction.prune` | Supprime les anciens tool outputs pour libérer de l'espace |
| `compaction.reserved` | Buffer de tokens réservé pour la compaction (évite le débordement) |

**Bénéfice :** Économie de tokens par réutilisation du cache de prompt.
**Portée :** Chaque session OpenCode (transversal à tous les projets).

---

### Cache projet (Session / Exploration)

**Rôle :** Éviter la ré-exploration du projet à chaque session.

**Fichier :** `.opencode/context.json` (dans chaque projet)

**Format :**
```json
{
  "version": "1.0",
  "generated_at": "2026-05-28T10:30:00Z",
  "stack": {
    "languages": ["typescript"],
    "frameworks": ["vue"]
  },
  "conventions": {
    "source": "CONVENTIONS.md",
    "hash": "sha256:abc123..."
  },
  "key_files": {
    "package.json": "sha256:def456...",
    "tsconfig.json": "sha256:ghi789...",
    "CONVENTIONS.md": "sha256:abc123..."
  }
}
```

**Bénéfice :** Gain de temps au démarrage (~2-5s), contexte projet disponible immédiatement.
**Portée :** Un fichier par projet, local à la machine.

---

## Quand utiliser quoi ?

| Besoin | Solution |
|--------|----------|
| Réduire les tokens API | Cache natif (`setCacheKey`) |
| Accélérer le démarrage d'une session | Cache projet (`.opencode/context.json`) |
| Projet ou conventions modifiés | `oc start --onboard --refresh` |
| Re-onboarder sans effacer le cache | `oc start --onboard` (cache écrasé automatiquement) |
| Diagnostiquer un démarrage lent | Vérifier si le cache projet est valide |

---

## Cycle de vie du cache projet

### Génération

Le cache `.opencode/context.json` est généré automatiquement par l'agent **onboarder** à la fin de la Phase 5 (étape 5.4).

**Déclencheur :** `oc start --onboard [PROJECT_ID]`

**Contenu :** Stack détectée en Phase 1, hashes SHA-256 des fichiers structurants (package.json, tsconfig.json, CONVENTIONS.md, etc.)

### Validation au démarrage

À chaque `oc start`, le hub vérifie le cache avant d'afficher le contexte :

1. Cache absent → pas d'affichage (comportement normal)
2. Cache valide (hashes identiques) → `✅ Cache contexte valide (2026-05-28)`
3. Cache invalide (un fichier a changé) → `⚠️ Cache invalide — oc start --onboard --refresh recommandé`

La validation n'est jamais bloquante.

### Régénération forcée

```bash
# Invalider le cache et re-onboarder
oc start --onboard --refresh [PROJECT_ID]
```

Comportement :
1. Supprime `.opencode/context.json`
2. Lance l'agent onboarder qui re-explore le projet
3. Génère un nouveau cache à la fin de l'onboarding

### Invalidation automatique

Le cache est considéré invalide si :
- Un fichier listé dans `key_files` a été modifié (hash SHA-256 différent)
- Un fichier listé dans `key_files` a été supprimé
- `context.json` est corrompu (JSON invalide)

---

## Graphe de dépendances

**Fichier :** `.opencode/dependency-graph.json` (optionnel)

**Génération :** Automatiquement lors de `oc deploy [PROJECT_ID]`, si le projet contient des fichiers TypeScript ou JavaScript.

**Format :**
```json
{
  "version": "1.0",
  "generated_at": "2026-05-28T10:30:00Z",
  "root": "/path/to/project",
  "stats": { "files_scanned": 42, "total_imports": 128 },
  "nodes": {
    "src/services/user.service.ts": {
      "imports": ["src/repositories/user.repository.ts"],
      "imported_by": ["src/controllers/user.controller.ts"]
    }
  }
}
```

**Utilisation :** L'orchestrateur-dev consulte ce graphe avant de lancer des tickets en parallèle pour détecter les conflits potentiels (fichiers dans la même chaîne d'imports).

**Limites :**
- TypeScript, JavaScript, TSX et JSX uniquement
- Imports relatifs uniquement (`./` ou `../`)
- Maximum 2000 fichiers scannés par projet
- Regex simplifié (pas d'AST) — précision ~90%

---

## Interaction entre les deux caches

Les deux systèmes sont **complémentaires**, pas concurrents :

```
Session OpenCode
│
├── Cache natif (setCacheKey)
│   └── Réduit les tokens API pour chaque échange avec le modèle
│
└── Cache projet (.opencode/context.json)
    └── Injecté automatiquement dans la session via le champ instructions
        └── L'orchestrateur dispose du contexte sans aucune lecture de fichier
```

Le cache natif agit au niveau de l'API (économie de tokens), le cache projet agit au niveau du contexte de l'agent (économie de temps et d'efforts d'exploration).

---

## Injection automatique dans la session (champ `instructions`)

Le cache projet est injecté dans chaque session OpenCode via le champ `instructions` de `opencode.json`. Ce mécanisme est géré automatiquement par les scripts — l'agent orchestrator n'a jamais besoin de lire de fichier.

### Priorité d'injection

```
1. Cache valide (.opencode/context.json)  → instructions: [".opencode/context.json"]
2. Fallback : fichiers contexte présents  → instructions: ["ONBOARDING.md", "CONVENTIONS.md"]
3. Aucun contexte disponible              → champ instructions supprimé (Mode C proposé)
```

> **Note — structure docs/context/** : Depuis la refactorisation des documents vivants, `ONBOARDING.md` est
> un résumé exécutif compact et `CONVENTIONS.md` contient uniquement les conventions de code condensées.
> Les détails (architecture, tests, librairies, contexte métier par domaine) sont dans `docs/context/`
> et chargés à la demande par les agents via `Read` — ils ne sont jamais injectés automatiquement.
> Le fallback reste `["ONBOARDING.md", "CONVENTIONS.md"]` : léger et suffisant pour orienter un agent
> en début de session sans reconstituer un contexte massif.

### Déclencheurs

| Événement | Action |
|-----------|--------|
| `oc deploy opencode <project>` | Injection au moment de l'écriture de `opencode.json` |
| `oc start <project>` | Injection/mise à jour à chaque démarrage |
| `oc start --onboard <project>` | Injection avec fallback fichiers (avant génération cache) |
| `oc start --onboard --refresh <project>` | Invalidation cache + fallback fichiers |

### Fonction technique

La fonction `_inject_context_instructions()` dans `scripts/lib/context-cache.sh` gère cette logique :
- Vérifie l'existence et la validité du cache
- Met à jour `opencode.json` de façon atomique (écriture via fichier tmp)
- Supprime le champ `instructions` si aucun contexte n'est disponible

---

## Fichiers techniques

| Fichier | Rôle |
|---------|------|
| `scripts/lib/context-cache.sh` | Lib bash : génération, validation, lecture, injection instructions du cache projet |
| `scripts/lib/dependency-graph.sh` | Lib bash : génération et requêtes du graphe de dépendances |
| `scripts/cmd-start.sh` | Valide le cache au démarrage, injecte instructions (`--refresh` pour invalider) |
| `scripts/adapters/opencode.adapter.sh` | Injecte instructions lors de la génération de `opencode.json` |
| `skills/planning/onboarder-workflow.md` | Étape 5.4 : génère `.opencode/context.json` |
| `skills/orchestrator/orchestrator-protocol.md` | CP-0 : contexte disponible via session (plus de lecture directe) |
| `skills/orchestrator/orchestrator-dev-protocol.md` | Détection de conflits via graphe avant parallélisme |
