---
name: planner-phase-5-6
description: Phase 5 (création tickets dans Beads), Phase 5.5 (délégation ai-delegated) et Phase 6 (vérification finale) du workflow planner.
---

# Phase 5 — Production du livrable : Création dans Beads

**Uniquement après validation explicite du plan.**

→ Charger le skill `planning/planner-beads-templates` via l'outil `skill` pour les templates complets de création (epics, features, tasks, dépendances, labels, estimations, règles d'enrichissement).

---

## Template — Création et enrichissement d'un epic

```bash
EPIC=$(bd create "Nom de l'epic" -t epic --json)
EPIC_ID=$(echo $EPIC | jq -r '.id')
bd update $EPIC_ID \
  --description "$(cat <<'EOF'
## Objectif métier
[Valeur apportée à l'utilisateur — pourquoi cet epic existe]

## Périmètre
[Ce qui est inclus dans cet epic]

## Hors périmètre
[Ce qui ne l'est pas pour cette itération]

## Risques
[Principaux risques identifiés sur cet epic]
EOF
)" \
  --notes "$(cat <<'EOF'
## Ordre d'implémentation
1. [ticket X] — bloquant
2. [tickets Y, Z] — parallélisables après X

## Dépendances inter-epics
[Liens avec d'autres epics si applicable — sinon : aucun]

## Estimation
~[X] heures au total
EOF
)"
```

---

## Template — Création d'un ticket fonctionnel (feature)

```bash
T=$(bd create "Titre du ticket" -t feature -p 1 --parent $EPIC_ID --estimate [minutes] --json)
T_ID=$(echo $T | jq -r '.id')
bd update $T_ID \
  --description "$(cat <<'EOF'
## Contexte métier
[Pourquoi ce ticket existe — valeur pour l'utilisateur ou le système]

## État actuel
[Ce qui existe aujourd'hui — comportement, fichiers, structure]

## État cible
[Ce qui doit exister après — comportement attendu, ce qui change]

## Contraintes et règles métier
[Rétrocompatibilité, cas limites, règles de gestion à respecter]
EOF
)" \
  --acceptance "$(cat <<'EOF'
## Comportement fonctionnel
- [Critère observable 1]
- [Critère observable 2]
- [Critère observable 3]

## Tests
- [ ] Test unitaire (Vitest) : [cas nominal — décrire le scénario]
- [ ] Test unitaire (Vitest) : [cas limite — décrire le scénario]
- [ ] Pas de régression sur [fonctionnalité connexe]

## Jeux de données représentatifs
- Nominal : [exemple d'entrée → sortie attendue]
- Limite : [exemple d'entrée limite → comportement attendu]
EOF
)" \
  --notes "$(cat <<'EOF'
## Dépendances
- Dépend de : [ID + titre des tickets bloquants]
- Bloque : [ID + titre des tickets dépendants]

## Architecture concernée
- Couche(s) : [use case / service / API handler / composant / store / DTO / etc.]
- Pattern(s) : [DDD aggregate / value object / port-adapter / composant présentationnel / etc.]
- Fichiers structurants : [chemins relatifs]

## Approches alternatives considérées
| Approche | Avantage | Inconvénient | Retenue ? |
|---|---|---|---|
| [Approche A] | ... | ... | ✓ |
| [Approche B] | ... | ... | ✗ |

## Risques et points d'attention
- [Risque technique, couplage, impact sur d'autres modules]
EOF
)"
```

---

## Template — Création d'un ticket technique (task)

```bash
T=$(bd create "Titre du ticket" -t task -p 2 --parent $EPIC_ID --estimate [minutes] --json)
T_ID=$(echo $T | jq -r '.id')
bd update $T_ID \
  --description "$(cat <<'EOF'
## Objectif technique
[Pourquoi ce ticket technique est nécessaire — problème résolu ou dette adressée]

## État actuel
[Ce qui existe aujourd'hui — structure, comportement, limitation]

## État cible
[Ce qui doit exister après — nouvelle structure, interface, contrat]

## Contraintes
[Rétrocompatibilité, contrat d'interface à respecter, contraintes de performance]
EOF
)" \
  --acceptance "$(cat <<'EOF'
## Contrat technique
- [Interface ou comportement observable 1]
- [Interface ou comportement observable 2]

## Tests
- [ ] Test unitaire (Vitest) : [cas nominal — décrire le scénario]
- [ ] Test unitaire (Vitest) : [cas limite ou cas d'erreur]
- [ ] Pas de régression : [ce qui ne doit pas changer]

## Jeux de données représentatifs
- Entrée : [structure d'entrée exemple]
- Sortie : [structure de sortie attendue]
EOF
)" \
  --notes "$(cat <<'EOF'
## Dépendances
- Dépend de : [ID + titre]
- Bloque : [ID + titre]

## Architecture concernée
- Couche(s) : [use case / DTO / port / adapter / repository / etc.]
- Pattern(s) : [pattern DDD ou clean arch concerné]
- Fichiers structurants : [chemins relatifs]

## Approches alternatives considérées
| Approche | Avantage | Inconvénient | Retenue ? |
|---|---|---|---|
| [Approche A] | ... | ... | ✓ |
| [Approche B] | ... | ... | ✗ |

## Risques et points d'attention
- [Couplages, impacts en cascade, migrations nécessaires]
EOF
)"
```

---

## Template — Ticket avec composant UI/frontend (ajouter --design)

Pour tout ticket touchant un composant Vue, une page ou un composable :

**Cas A — spec UI disponible (rapportée par l'UI Designer en Phase 1.5) :**

```bash
bd update $T_ID \
  --design "$(cat <<'EOF'
## Composants du design system utilisés
- [Nom du composant DSFR ou interne — variante utilisée]
- [Autre composant si applicable]

## Comportement UX
- État initial : [ce que l'utilisateur voit au chargement]
- Interaction(s) : [ce qui se passe au clic / saisie / survol]
- État de chargement : [skeleton / spinner / disabled — préciser]
- État d'erreur : [message, comportement du formulaire]
- État vide : [ce qui s'affiche si aucune donnée]

## Accessibilité
- [aria-label, aria-describedby, rôles ARIA si applicable]
- [Navigation clavier si applicable]
- [Contrastes et lisibilité si applicable]

## Responsive
- [Comportement mobile / tablette si différent du desktop]
EOF
)"
```

**Cas B — spec UI non disponible (Phase 1.5 ignorée ou non déclenchée) :**

Remplir `--design` avec le contexte disponible (partiel), puis tracer la spec manquante via un commentaire :

```bash
bd update $T_ID \
  --design "$(cat <<'EOF'
## À compléter par l'UI Designer
Voir commentaire sur ce ticket pour les instructions d'invocation.

## Contexte disponible
- Composant(s) concerné(s) : [NomComposant.vue]
- Comportement attendu : [description fonctionnelle extraite de la description du ticket]
- Design system : [DSFR / autre]
EOF
)"

bd comments add $T_ID "⚠️ Spec UI à compléter — ce ticket nécessite une spécification visuelle.

Invoquer l'agent designer (Mode: ui) avec ce contexte :
---
Composant : [NomComposant.vue]
Feature : [nom de la feature]
Comportement attendu : [coller la description du ticket]
Design system : [DSFR / autre]
Spec UX associée : [coller le user flow si disponible]
---
Demander : 'Spec UI pour [NomComposant]'

Après la spec, mettre à jour ce ticket :
  bd update $T_ID --design '...' (remplacer le contenu existant par la spec complète)
  bd update $T_ID --acceptance '...' (compléter avec les critères visuels issus de la spec)"
```

---

## Template — Création d'un ticket avec dépendance

```bash
T=$(bd create "Titre" -t task -p 2 --parent $EPIC_ID --estimate [minutes] --json)
T_ID=$(echo $T | jq -r '.id')
bd dep add $T_ID $T_PRECEDENT_ID
bd update $T_ID \
  --description "[...template selon type...]" \
  --acceptance "[...template selon type...]" \
  --notes "[...template selon type — dans la section Dépendances, indiquer explicitement : 'Ne pas démarrer avant que $T_PRECEDENT_ID soit clos.']"
```

---

## Template — Ticket issu d'une scission

```bash
T=$(bd create "Titre" -t task -p 2 -l split-from-$ORIGINAL_ID --parent $EPIC_ID --estimate [minutes] --json)
```

---

## Estimation — référence rapide

| Estimation | Durée |
|---|---|
| `--estimate 30` | 30 min |
| `--estimate 60` | 1h |
| `--estimate 120` | 2h |
| `--estimate 240` | demi-journée |
| `--estimate 480` | 1 jour |

Si l'estimation est incertaine, utiliser la borne haute et signaler dans les notes :
> "Estimation haute — à affiner après exploration plus fine."

---

## Avec assignee et labels

```bash
T=$(bd create "Titre" -t task -p 2 -l ai-delegated -a dev-agent --parent $EPIC_ID --estimate [minutes] --json)
```

---

## Types disponibles (5)

- `-t epic` → epic (conteneur de tickets)
- `-t feature` → nouvelle fonctionnalité
- `-t task` → tâche technique (refactoring, migration, configuration, ADR)
- `-t bug` → correction de bug
- `-t chore` → maintenance, CI/CD, documentation, nettoyage

---

## Priorités (4) — forme numérique uniquement

- `-p 0` → P0 critique / bloquant
- `-p 1` → P1 haute priorité
- `-p 2` → P2 normale (défaut)
- `-p 3` → P3 basse priorité

---

## Règles impératives

- Toujours utiliser `--json` sur `bd create`
- Toujours capturer l'ID via `jq -r '.id'`
- Ne jamais utiliser `bd edit`
- Les descriptions sont en langage naturel, jamais en code
- Les critères d'acceptance sont observables et vérifiables
- **Toujours renseigner `--estimate`** — même approximatif
- **Toujours renseigner `--design`** pour tout ticket touchant un composant UI
- **Toujours enrichir les epics** avec `--description` et `--notes` immédiatement après création
- **Toujours inclure une section "Approches alternatives"** dans les notes quand un choix technique existe

---

## Gestion des aléas en cours de création

| Situation | Réponse |
|-----------|---------|
| L'utilisateur modifie le scope | Stopper la création. Re-présenter le delta (tickets à ajouter/retirer). Valider avant de reprendre. |
| Un ticket semble trop gros en le rédigeant | Proposer de le scinder avec le label `split-from-<ID>`. Attendre la validation. |
| Dépendance découverte à la création | `bd dep add` sur le ticket en cours. Signaler dans les notes. |
| Erreur sur un `bd create` | Signaler, ne pas créer de doublon, reprendre proprement. |
| Doublon détecté | `bd duplicate <ID> --of <CANONICAL>` (auto-ferme le doublon). Signaler à l'utilisateur. |
| Choix technique non tranché | Ajouter le label `needs-decision`. Documenter les options dans les notes. |
| Infos manquantes pour rédiger | Ajouter le label `needs-clarification`. Indiquer ce qui manque dans les notes. |

---

## Récap de fin de Phase 5

```markdown
## [Phase 5] Création dans Beads terminée

**Epics créés :**
- bd-X : <titre> — enrichi (description + notes)
- (aucun si plan à plat)

**Tickets créés :**
- bd-Y : <titre> — <type> — P<X> — enrichi (description + acceptance + notes + estimate + design si UI)
- bd-Z : <titre> — <type> — P<X> — enrichi (...)
- ...

**Total :** X epics + Y tickets créés

**Dépendances ajoutées :**
- bd-Y → bd-Z (bd-Y dépend de bd-Z)
- ...

**Labels ajoutés :**
- bd-W : split-from-bd-42
- bd-V : needs-clarification (raison : [raison])

**Specs design tracées (si Phase 1.5 skippée) :**
- bd-Y : commentaire ajouté avec instructions pour invoquer designer (Mode: ui)
```

## Transition automatique

**Pas de question de validation ici** — passage automatique à Phase 5.5.

---

# Phase 5.5 — Délégation ai-delegated (optionnelle)

## Objectif
Proposer de déléguer certains tickets à l'agent IA en ajoutant le label `ai-delegated`.

## Récap avant question

```markdown
## [Phase 5.5] Délégation ai-delegated

Le label `ai-delegated` indique qu'un ticket peut être délégué à un agent IA pour implémentation.

**Tickets créés :** X tickets
**Tickets éligibles à la délégation :** Y tickets (ceux sans dépendance bloquante non terminée)
```

## Question de validation obligatoire

**Si CONTEXTE = standalone :**
```
question({
  questions: [{
    header: "Délégation ai-delegated",
    question: "[Planner — Phase 5.5 | Feature : <nom>]\nSouhaitez-vous déléguer certains tickets à l'agent IA (label ai-delegated) ?",
    options: [
      { label: "Non", description: "Aucun ticket délégué à l'IA" },
      { label: "Oui — certains tickets", description: "Indiquer les IDs dans la réponse libre" },
      { label: "Oui — tous les tickets éligibles", description: "Déléguer tous les tickets sans dépendance bloquante" }
    ]
  }]
})
```

**Uniquement si l'utilisateur valide :**
```bash
# Déléguer un ticket
bd label add <ID> ai-delegated

# Déléguer plusieurs tickets
bd label add bd-1 ai-delegated
bd label add bd-2 ai-delegated
```

**Règles absolues :**
- Ne jamais ajouter `ai-delegated` sans validation explicite
- Ne jamais déléguer un ticket bloqué par un ticket non terminé
- Si l'utilisateur dit "tous", demander confirmation une dernière fois avant d'exécuter

## Récap de fin de Phase 5.5

```markdown
## [Phase 5.5] Délégation ai-delegated terminée

**Tickets délégués :**
- bd-X : label `ai-delegated` ajouté
- bd-Y : label `ai-delegated` ajouté
- (aucun si non validé)

**Tickets non délégués :**
- bd-Z : dépend de bd-W non terminé
- bd-V : choix de l'utilisateur
```

## Transition automatique

**Pas de question de validation ici** — passage automatique à Phase 6.

---

# Phase 6 — Vérification finale

## Objectif
Vérifier que tous les tickets sont correctement créés, enrichis, et liés entre eux.

## Commandes de vérification

```bash
# Arbre des tickets par epic
bd children <epic-id>

# Tous les tickets ouverts créés dans cette session
bd list -s open --json
```

## Récap de fin de Phase 6

```markdown
## [Phase 6] Récapitulatif de planification

### Tickets créés

**Epic bd-X — [Nom de l'epic]**

**bd-Y (P1, feature, ~2h) — [Titre]**
- **Description :** [Résumé en 2-3 phrases de ce que fait ce ticket]
- **Critères d'acceptance :** [Liste des critères — format : "Le système doit..."]
- **Notes :** [Choix techniques, alternatives considérées, points d'attention]
- **Dépendances :** Aucune — ticket fondation

**bd-Z (P2, task, ~4h) — [Titre]**
- **Description :** [Résumé en 2-3 phrases]
- **Critères d'acceptance :** [Critères]
- **Notes :** [Notes]
- **Dépendances :** bd-Y (raison : consomme le service créé par bd-Y)

**bd-W (P2, task, ~1h) — [Titre]**
- **Description :** [Résumé]
- **Critères d'acceptance :** [Critères]
- **Notes :** [Notes]
- **Dépendances :** bd-Y (raison : utilise l'endpoint créé par bd-Y)

**Epic bd-A — [Nom de l'epic]**

**bd-B (P1, feature, ~3h) — [Titre]**
- **Description :** [Résumé]
- **Critères d'acceptance :** [Critères]
- **Notes :** [Notes]
- **Dépendances :** bd-Z (raison : intègre le middleware créé par bd-Z)

---

### Ordre d'implémentation suggéré

1. **bd-Y** (bloquant) — ticket fondation, doit être fait en premier
2. **bd-Z, bd-W** (parallélisables) — peuvent être implémentés en parallèle après bd-Y
3. **bd-B** (après bd-Z) — dépend du middleware créé par bd-Z

---

### Hypothèses et ambiguïtés

- **Hypothèse 1 :** [Ex : "J'ai supposé que l'authentification utilise JWT — l'utilisateur n'a pas précisé le mécanisme"]
- **Ambiguïté 1 :** [Ex : "Le délai d'expiration du token n'était pas précisé — j'ai fixé 24h par défaut"]
- (Aucune hypothèse ni ambiguïté si la demande était complète et sans ambiguïté)

---

### Risques identifiés

- **Risque 1 :** [Ex : "bd-Z dépend d'une librairie externe non encore évaluée — risque de complexité sous-estimée"]
- **Risque 2 :** [Ex : "bd-B touche 3 composants UI — risque de régression si les tests ne couvrent pas tous les parcours"]
- (Aucun risque identifié si le plan est clair et sans risque notable)

---

### Résumé

- **Epics créés :** N
- **Tickets créés :** M
- **Estimation totale :** ~Xh
- **Tickets délégués à l'IA :** K tickets avec label `ai-delegated`

### Points d'attention

- [Point 1 si applicable — ex : ticket bd-Z marqué needs-clarification (raison)]
- [Point 2 si applicable]
```

---

## Question de validation obligatoire

**Si CONTEXTE = standalone :**

```
question({
  questions: [{
    header: "Validation finale",
    question: "[Planner — Phase 6 complétée | Feature : <nom>]\nLes tickets correspondent-ils à vos attentes ? Souhaitez-vous des ajustements ?",
    options: [
      { label: "Oui — c'est bon", description: "Planning terminé" },
      { label: "Ajustements à faire", description: "Apporter des modifications aux tickets créés" }
    ]
  }]
})
```

**Selon la réponse (standalone uniquement) :**
- **C'est bon** → Fin de session
- **Ajustements** → rester en Phase 6, appliquer les ajustements via `bd update`, re-produire le récap
