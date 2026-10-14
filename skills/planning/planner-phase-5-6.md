---
name: planner-phase-5-6
description: "Utiliser avant les phases 5 et 6 du planner : création dans Beads (templates), ai-delegated, vérification finale."
---

# Phase 5 — Production du livrable : Création dans Beads

**Uniquement après validation explicite du plan.**
<!-- oh:when role=subagent -->

Sous-agent : la création attend le checkpoint de ton appelant ; tu arrives ici relancé avec ton `task_id`, découpage validé.
<!-- /oh:when -->

## Ordre de création

1. Créer les epics en premier (si applicable) et les enrichir immédiatement
2. Créer les tickets fils avec `--parent`
3. Enrichir chaque ticket avec description + acceptance + notes + estimate + design (si UI)
4. Ajouter les dépendances via `bd dep add` après création
5. Ajouter les labels pertinents (`-l` à la création ou `bd label add` après)

---

## Template — Création et enrichissement d'un epic

```bash
bd create "Nom de l'epic" -t epic --json
# l'id de la sortie JSON → <EPIC_ID>, dans la commande suivante (un appel par commande)
bd update <EPIC_ID> \
  --description "## Objectif métier
[Valeur apportée à l'utilisateur — pourquoi cet epic existe]

## Périmètre
[Ce qui est inclus dans cet epic]

## Hors périmètre
[Ce qui ne l'est pas pour cette itération]

## Risques
[Principaux risques identifiés sur cet epic]" \
  --acceptance "## Critères de succès (Success Criteria)
- [Résultat observable qui prouve que l'epic a atteint son objectif]" \
  --notes "## Ordre d'implémentation
1. [ticket X] — bloquant
2. [tickets Y, Z] — parallélisables après X

## Dépendances inter-epics
[Liens avec d'autres epics si applicable — sinon : aucun]

## Estimation
~[X] heures au total"
```

---

## Template — Création d'un ticket fonctionnel (feature)

```bash
bd create "Titre du ticket" -t feature -p 1 --parent <EPIC_ID> --estimate [minutes] --json
# l'id de la sortie JSON → <T_ID>
bd update <T_ID> \
  --description "## Contexte métier
[Pourquoi ce ticket existe — valeur pour l'utilisateur ou le système]

## État actuel
[Ce qui existe aujourd'hui — comportement, fichiers, structure]

## État cible
[Ce qui doit exister après — comportement attendu, ce qui change]

## Contraintes et règles métier
[Rétrocompatibilité, cas limites, règles de gestion à respecter]" \
  --acceptance "## Comportement fonctionnel
- [Critère observable 1]
- [Critère observable 2]
- [Critère observable 3]

## Tests (frontière : [interface publique identifiée, si identifiable])
- [ ] Test unitaire (Vitest) : [cas nominal — décrire le scénario]
- [ ] Test unitaire (Vitest) : [cas limite — décrire le scénario]
- [ ] Pas de régression sur [fonctionnalité connexe]

## Jeux de données représentatifs
- Nominal : [exemple d'entrée → sortie attendue]
- Limite : [exemple d'entrée limite → comportement attendu]" \
  --notes "## Dépendances
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
- [Risque technique, couplage, impact sur d'autres modules]"
```

---

## Template — Création d'un ticket technique (task)

```bash
bd create "Titre du ticket" -t task -p 2 --parent <EPIC_ID> --estimate [minutes] --json
# l'id de la sortie JSON → <T_ID>
bd update <T_ID> \
  --description "## Objectif technique
[Pourquoi ce ticket technique est nécessaire — problème résolu ou dette adressée]

## État actuel
[Ce qui existe aujourd'hui — structure, comportement, limitation]

## État cible
[Ce qui doit exister après — nouvelle structure, interface, contrat]

## Contraintes
[Rétrocompatibilité, contrat d'interface à respecter, contraintes de performance]" \
  --acceptance "## Contrat technique
- [Interface ou comportement observable 1]
- [Interface ou comportement observable 2]

## Tests (frontière : [interface publique identifiée, si identifiable])
- [ ] Test unitaire (Vitest) : [cas nominal — décrire le scénario]
- [ ] Test unitaire (Vitest) : [cas limite ou cas d'erreur]
- [ ] Pas de régression : [ce qui ne doit pas changer]

## Jeux de données représentatifs
- Entrée : [structure d'entrée exemple]
- Sortie : [structure de sortie attendue]" \
  --notes "## Dépendances
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
- [Couplages, impacts en cascade, migrations nécessaires]"
```

---

## Template — Ticket avec composant UI/frontend (ajouter --design)

Pour tout ticket touchant un composant Vue, une page ou un composable :

**Cas A — spec UI disponible (rapportée par l'UI Designer en Phase 1.5) :**

```bash
bd update <T_ID> \
  --design "## Composants du design system utilisés
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
- [Comportement mobile / tablette si différent du desktop]"
```

**Cas B — spec UI non disponible (Phase 1.5 ignorée ou non déclenchée) :**

Remplir `--design` avec le contexte disponible (partiel), puis tracer la spec manquante via un commentaire :

```bash
bd update <T_ID> \
  --design "## À compléter par l'UI Designer
Voir commentaire sur ce ticket pour les instructions d'invocation.

## Contexte disponible
- Composant(s) concerné(s) : [NomComposant.vue]
- Comportement attendu : [description fonctionnelle extraite de la description du ticket]
- Design system : [DSFR / autre]"

bd comments add <T_ID> "⚠️ Spec UI à compléter — ce ticket nécessite une spécification visuelle.

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
  bd update <T_ID> --design '...' (remplacer le contenu existant par la spec complète)
  bd update <T_ID> --acceptance '...' (compléter avec les critères visuels issus de la spec)"
```

---

## Template — Création d'un ticket avec dépendance

```bash
bd create "Titre" -t task -p 2 --parent <EPIC_ID> --estimate [minutes] --json
# l'id de la sortie JSON → <T_ID>
bd dep add <T_ID> <T_PRECEDENT_ID>
bd update <T_ID> \
  --description "[...template selon type...]" \
  --acceptance "[...template selon type...]" \
  --notes "[...template selon type — dans la section Dépendances, indiquer explicitement : 'Ne pas démarrer avant que <T_PRECEDENT_ID> soit clos.']"
```

---

## Template — Ticket issu d'une scission

```bash
bd create "Titre" -t task -p 2 -l split-from-<ORIGINAL_ID> --parent <EPIC_ID> --estimate [minutes] --json
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
- Toujours lire l'ID dans la sortie JSON de `bd create` et l'écrire en clair dans les commandes suivantes : chaque commande est lancée seule (pas de variable shell, ni `echo`, ni `jq`, ni `cat <<EOF`)
- Texte des champs entre guillemets doubles, sur plusieurs lignes si besoin : échappe `"`, `$` et les backquotes (`\"`, `\$`, `` \` ``)
- Ne jamais utiliser `bd edit`
- Les descriptions sont en langage naturel, jamais en code
- Les critères d'acceptance sont observables et vérifiables
- Un ticket `bug` a une section `## Étapes de reproduction (Steps to Reproduce)` dans sa description (titre exigé par `bd lint`)
- **Toujours renseigner `--estimate`** — même approximatif
- **Toujours renseigner `--design`** pour tout ticket touchant un composant UI
- **Toujours enrichir les epics** avec `--description`, `--acceptance` (critères de succès) et `--notes` immédiatement après création
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

**Epics créés :** bd-X : <titre> — enrichi (aucun si plan à plat)
**Tickets créés :** bd-Y : <titre> — <type> — P<X> — enrichi (description + acceptance + notes + estimate + design si UI)
**Total :** X epics + Y tickets
**Dépendances ajoutées :** bd-Y → bd-Z (bd-Y dépend de bd-Z)
**Labels ajoutés :** bd-W : split-from-bd-42 · bd-V : needs-clarification (raison)
**Specs design tracées (si Phase 1.5 skippée) :** bd-Y : commentaire avec les instructions pour le designer (Mode: ui)
```

Pas de question de validation ici — passage automatique à Phase 5.5.

---

# Phase 5.5 — Délégation ai-delegated (optionnelle)

Le label `ai-delegated` indique qu'un ticket peut être délégué à un agent IA pour implémentation. Éligibles : les tickets sans dépendance bloquante non terminée.

## Question de validation obligatoire
<!-- oh:when role=entry -->

Récap en texte (`## [Phase 5.5] Délégation ai-delegated` : tickets créés, tickets éligibles), puis :

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

**Uniquement si l'utilisateur valide :** `bd label add <ID> ai-delegated` pour chaque ticket retenu.

- Si l'utilisateur dit "tous", demander confirmation une dernière fois avant d'exécuter
<!-- /oh:when -->
<!-- oh:when role=subagent -->

Sous-agent : n'ajoute pas le label ; liste les tickets éligibles dans ta `### Suite proposée`, l'utilisateur tranche par ton appelant.
<!-- /oh:when -->

**Règles absolues :**
- Ne jamais ajouter `ai-delegated` sans validation explicite
- Ne jamais déléguer un ticket bloqué par un ticket non terminé

Récap de fin de Phase 5.5 : tickets délégués (label ajouté) et non délégués (raison : dépendance non terminée, choix de l'utilisateur). Passage automatique à Phase 6.

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

# Sections manquantes des tickets créés (code de sortie 1 = au moins un ticket incomplet)
bd lint <id-1> <id-2> … --json
```

`bd lint` signale les sections recommandées absentes : `## Acceptance Criteria` pour une feature ou une task (le champ `--acceptance` suffit), `Steps to Reproduce` pour un bug, `Success Criteria` (ou le champ `--acceptance`) pour un epic. Compléter chaque ticket signalé (`bd update <id> --acceptance …` ou `--description …`), puis relancer `bd lint` jusqu'à ce qu'il ne signale plus rien. Un ticket qui reste incomplet est listé dans « Points d'attention » du récap.

Chercher ensuite les doublons parmi les tickets ouverts : `bd find-duplicates --status open --json` (méthode mécanique par défaut).
La méthode `--method ai` est refusée par oh (les tickets ne partent pas vers un modèle externe). Pour chaque paire qui contient un ticket créé dans cette session : vrai doublon d'un ticket existant → le signaler dans « Points d'attention » avec la commande à lancer après validation (`bd duplicate <nouveau> --of <existant>`) ; simple proximité de titre → ignorer.

## Récap de fin de Phase 6

Narratif et détaillé, il reprend le plan validé en phase 3.
<!-- oh:when role=subagent -->
Sous-agent : ce récap est ton rapport, aux sections de ton payload.
<!-- /oh:when -->

```markdown
## [Phase 6] Récapitulatif de planification

### Tickets créés
**Epic bd-X — [Nom]**
**bd-Y (P1, feature, ~2h) — [Titre]**
- **Description :** [2-3 phrases]
- **Critères d'acceptance :** [« Le système doit… »]
- **Notes :** [choix techniques, alternatives, points d'attention]
- **Dépendances :** [Aucune — ticket fondation | bd-Z (raison)]

### Ordre d'implémentation suggéré
1. **bd-Y** (bloquant) — ticket fondation
2. **bd-Z, bd-W** (parallélisables après bd-Y)

### Hypothèses et ambiguïtés
- **Hypothèse :** [ex : « J'ai supposé que l'authentification utilise JWT »]

### Risques identifiés
- **Risque :** [ex : « bd-Z dépend d'une librairie non évaluée »]

### Résumé
Epics : N · Tickets : M · Estimation totale : ~Xh · Tickets `ai-delegated` : K

### Points d'attention
- [ex : bd-Z marqué needs-clarification (raison)]
```

## Question de validation obligatoire
<!-- oh:when role=entry -->

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

**Selon la réponse :** c'est bon → fin de session ; ajustements → rester en Phase 6, appliquer via `bd update`, refaire le récap.
<!-- /oh:when -->
<!-- oh:when role=subagent -->

Sous-agent : pas de question ; dépose ton rapport puis rends ton enveloppe.
<!-- /oh:when -->
