> [Read in English](living-wiki.en.md)

# Wiki Documentaire Vivant

Le **wiki documentaire vivant** est le système de documentation contextuelle des projets
gérés par le hub. Il remplace les anciens fichiers plats (`ONBOARDING.md`, `CONVENTIONS.md`,
`docs/context/`) par une arborescence structurée, navigable et enrichie de manière incrémentale.

---

## Concept

Inspiré du concept de "graphe de connaissance" de Graphify, le wiki documentaire vivant repose
sur deux idées fondamentales :

**1. Les god nodes** — certains concepts sont plus connectés que d'autres dans une codebase.
Ils apparaissent dans plusieurs domaines (technique ET métier) et représentent les zones
où se concentrent les décisions critiques. Le wiki les identifie explicitement dans `index.md`
pour guider les agents vers l'essentiel en premier.

**2. Les tags de confiance** — chaque enrichissement porte un niveau de confiance explicite
(`CONFIRMÉ`, `DÉDUIT`, `INCERTAIN`) avec la source (fichier + ligne si possible). Les agents
futurs savent immédiatement s'ils peuvent utiliser une information directement ou s'ils
doivent la vérifier.

---

## Structure

```
docs/wiki/
├── index.md                    ← carte globale obligatoire — toujours lue en premier
├── technical/
│   ├── architecture.md         ← patterns dominants, découpage, décisions structurantes
│   ├── stack.md                ← stack complète, versions, librairies clés
│   ├── tests.md                ← stratégie, conventions, seuils, frameworks
│   └── conventions.md          ← nommage, git, linting, config, patterns équipe
└── business/
    ├── index.md                ← carte des domaines métier
    └── <domain>.md             ← règles de gestion, flux, entités, risques
```

À la racine du projet :
```
ONBOARDING.md                   ← résumé minimaliste (15-25 lignes), redirige vers le wiki
```

---

## Format des tags de confiance

```markdown
- Description de l'observation
  — `CONFIRMÉ` · <agent> · <YYYY-MM-DD> · <fichier:ligne>

- Description d'une observation déduite
  — `DÉDUIT` · <agent> · <YYYY-MM-DD> · <fichier>

- Description incertaine
  — `INCERTAIN` · <agent> · <YYYY-MM-DD>
```

| Tag | Signification |
|-----|--------------|
| `` `CONFIRMÉ` `` | Observation directe dans le code, fichier + ligne citée |
| `` `DÉDUIT` `` | Raisonnement contextuel depuis plusieurs fichiers |
| `` `INCERTAIN` `` | Hypothèse ou convention non codée, à valider |

---

## Protocole de navigation (skill `wiki-navigation`)

Le skill `shared/wiki-navigation` est en **Bucket A** (inline) dans les agents qui consultent le contexte
d'un projet : `pathfinder`, `onboarder`, `developer`, `developer-refactor`, `developer-migrator`, `database`,
`infra`, `auditor-subagent`, `reviewer`, `debugger`, `benchmarker`, `test-generator`.

**Règle fondamentale :** lire `docs/wiki/index.md` en premier, puis charger uniquement
la page pertinente à la tâche. Ne jamais lire le wiki en entier par défaut.

```
Tâche courante
     │
     ▼
docs/wiki/index.md (toujours)
     │
     ├── Implémentation / nommage → technical/conventions.md
     ├── Architecture / découpage → technical/architecture.md
     ├── Stack / dépendances       → technical/stack.md
     ├── Tests / couverture        → technical/tests.md
     ├── Domaine métier précis     → business/<domain>.md
     └── Contexte général          → index.md suffit
```

---

## Algorithme des god nodes

Un concept devient un **god node** quand il apparaît dans ≥ 2 pages wiki distinctes.
Le `documentarian` réévalue le tableau après chaque enrichissement :

1. Recenser les concepts mentionnés dans la page modifiée
2. Compter dans combien de pages distinctes chaque concept apparaît
3. Si ≥ 2 pages → candidat god node → ajouter dans `index.md`
4. Criticité : `Critique` (≥ 4 pages ou dans "Points critiques"), `Haute` (3 pages), `Normale` (2 pages)

---

## Génération et enrichissement

### Génération initiale (onboarder)

L'`onboarder` génère le wiki en Phase 5, après validation du rapport de contexte, dans le workflow
`onboarding` (`oh run onboarding`).
Toutes les pages sont créées avec le format canonique défini dans le skill `doc-wiki-protocol`.

### Enrichissement incrémental (tous les agents)

Après chaque rapport (audit, diagnostic, implémentation, review), les agents
identifient les découvertes à capitaliser via le skill `shared/living-docs-enrichment` :

1. L'agent consolide les enrichissements et propose avec leurs tags de confiance
2. L'utilisateur confirme
3. L'agent délègue au `documentarian` via `task`, si le `documentarian` est membre du workflow de la session
4. Le `documentarian` enrichit les pages ciblées et réévalue les god nodes

### Re-onboarding

Si `docs/wiki/index.md` existe déjà, l'`onboarder` propose (entrée `refresh` du workflow :
`oh run onboarding -i refresh=true`) :
- **Enrichissement incrémental** (recommandé) — via `living-docs-enrichment`
- **Réécriture complète** — avec avertissement sur la perte des enrichissements accumulés
- **Conserver l'existant**

---

## Comparaison avant / après

| Avant | Après |
|-------|-------|
| 4 fichiers plats (`ONBOARDING.md`, `CONVENTIONS.md`, `docs/context/technical.md`, `docs/context/business/<domain>.md`) | Arborescence wiki structurée (`docs/wiki/`) |
| Agents lisent potentiellement tout le contexte à chaque session | Agents lisent `index.md` (40-80 lignes) puis une seule page |
| Aucun niveau de confiance sur les informations | 3 niveaux : `CONFIRMÉ` / `DÉDUIT` / `INCERTAIN` |
| Concepts importants non identifiés | God nodes explicites dans `index.md` |
| `CONVENTIONS.md` lu entièrement par chaque agent | `conventions.md` chargé uniquement si pertinent |
| Aucun protocole de navigation | Skill `wiki-navigation` Bucket A dans les agents qui lisent le contexte du projet |

---

## Le wiki dans une session v5

- **Création** : workflow `onboarding`, dont le seul agent est l'`onboarder`. Il n'écrit que dans `docs/wiki/` et
  `ONBOARDING.md` ; cette limite est une consigne du prompt, pas une permission
  ([ADR-039](./adr/039-declarative-workflows-oh-v1.fr.md)). Sortie déclarée : `wiki` (chemin).
- **Précondition** : `feature` et `cadrage` vérifient qu'au moins un de `docs/wiki/index.md`, `ONBOARDING.md` ou
  `CONVENTIONS.md` existe. Sinon, oh propose de lancer d'abord `onboarding`, puis de revenir.
- **Contexte injecté** : à la construction du paquet de session, `ONBOARDING.md`, `CONVENTIONS.md`,
  `.claude/CLAUDE.md` et les fichiers de `[deploy] instruction_files` sont intégrés au corps des agents
  ([ADR-043](./adr/043-session-bundle-deploy-removal.fr.md)). Le wiki lui-même n'est pas intégré : les agents le
  lisent à la demande, avec `wiki-navigation`. D'où un `ONBOARDING.md` court qui renvoie au wiki.
- **Écritures** : oh n'écrit jamais dans le wiki. Ce sont les agents de la session qui l'écrivent, dans le dossier
  de travail de la session (projet ou worktree), comme tout autre fichier.
- **Enrichissement** : le `documentarian` est membre `independent` de `feature` et `ticket`. Dans les autres
  workflows, il n'est pas dans le paquet : les enrichissements sont seulement proposés.

---

## Références

- Skill `shared/wiki-navigation` — protocole de navigation + algorithme god nodes
- Skill `documentarian/doc-wiki-protocol` — formats canoniques + règles d'enrichissement
- Skill `shared/living-docs-enrichment` — workflow d'enrichissement délégué au documentarian
- ADR à venir : décision d'adoption du wiki documentaire vivant
