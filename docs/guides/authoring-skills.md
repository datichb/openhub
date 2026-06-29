> 🏗️ Ce guide couvre la **méthodologie qualitative** d'écriture de skills : TDD, SDO, anti-patterns, gouvernance.
> Pour la conception structurelle (agent vs skill, buckets, checklists), voir [authoring.fr.md](authoring.fr.md).

# Guide — Méthodologie d'authoring skills

---

## 1 — Types de skills

Avant d'écrire un skill, identifier son type. Le type conditionne le format attendu et les pièges à éviter.

| Type | Rôle | Format dominant | Exemples |
|------|------|----------------|---------|
| **Technique** | Workflow procédural — guide l'agent étape par étape | Phases numérotées, templates, autocontrôles | `debugger-workflow`, `planner-workflow`, `auditor-workflow` |
| **Pattern** | Contrainte récurrente — impose un comportement à tout moment | Règles ✅/❌, exemples de violations, triggers | `posture/tool-question`, `posture/coordination-only` |
| **Reference** | Catalogue / lookup — consulté à la demande | Tableaux, listes structurées, source de vérité | `shared/hub-workflow-reference`, `developer/dev-standards-*` |
| **Discipline** | Norme comportementale transversale — façonne la posture globale | Principes, interdictions absolues, exemples | `posture/expert-posture`, `posture/retranscription-coordinateur` |

> Un skill peut être hybride (ex : Technique + Reference), mais l'un des types domine toujours. Identifier le type dominant pour choisir le format principal.

---

## 2 — TDD pour skills

La rédaction directe d'un skill produit souvent un contenu trop vague, contournable ou incomplet. Le TDD force à partir du comportement attendu, pas de la règle.

### RED — Écrire le scénario de test d'abord

Avant d'écrire une seule ligne de skill, rédiger en langage naturel :

**Cas nominal :**
> "Quand [situation déclenchante], l'agent doit [comportement précis et observable]."

Exemple :
> "Quand l'agent reçoit un handoff sans gate de complétion documenté, il doit bloquer la construction du CP-feature et poser une question via l'outil `question`."

**Cas de rationalisation :**
> "Si l'agent peut justifier [comportement à éviter] en disant [rationalisation plausible], le skill doit explicitement l'interdire."

Exemples :
> "Si l'agent dit 'le gate est implicitement passé puisque l'implémentation est terminée', le skill doit interdire cette hypothèse."
> "Si l'agent dit 'la question prend trop de temps, je continue', le skill doit interdire de bypasser."

---

### GREEN — Skill minimal qui passe le scénario

Écrire uniquement ce qui est nécessaire pour couvrir les cas RED. Règles :

- ✅ Couvrir le cas nominal avec une règle actionnable
- ✅ Couvrir chaque cas de rationalisation avec une interdiction explicite
- ❌ Ne pas anticiper des cas non testés — ils viendront au REFACTOR
- ❌ Ne pas écrire de contenu "au cas où" — chaque ligne doit répondre à un scénario

---

### REFACTOR — Fermer les loopholes

Après avoir un skill GREEN, relire en se posant pour chaque règle :
> "Comment un modèle pourrait-il respecter la lettre de cette règle tout en violant son esprit ?"

Chaque loophole identifié → ajouter une ligne dans la **rationalization table** et une règle supplémentaire si nécessaire.

**Ajouts systématiques au REFACTOR :**
1. Rationalization table (voir section 4)
2. Red Flags list — liste de signaux qui indiquent que l'agent dévie
3. Autocontrôles obligatoires si le skill est de type Technique

---

## 3 — SDO — Skill Description Optimization

Le champ `description:` du frontmatter est utilisé par OpenCode pour sélectionner les skills dynamiquement. Une mauvaise description = skill jamais chargé au bon moment.

### Critères SDO

**Description riche et discriminante**

La description doit permettre à OpenCode de distinguer ce skill de tous les autres.

❌ Trop générique :
```yaml
description: Guide de bonnes pratiques pour le debugging
```

✅ Discriminant :
```yaml
description: Gate de complétion obligatoire avant tout DONE — 3 checks (tests passent,
  comportement observable conforme, régressions documentées). Bloquant si absent. Charger
  dans orchestrator-protocol avant construction du CP-feature.
```

**Keyword coverage**

Inclure les synonymes et formulations alternatives que l'utilisateur ou l'agent pourrait utiliser.

Exemple pour un skill de complétion :
- "gate de complétion" ET "vérification avant DONE" ET "3 checks" ET "CP-feature"

**Token efficiency**

- ≤ 2 phrases dans `description:` — au-delà, la description est tronquée ou ignorée
- Condenser sans perdre les keywords discriminants
- Mettre les keywords les plus importants en premier

**Cross-references**

Si deux skills peuvent être confondus, la description doit pointer vers l'autre :
```yaml
description: Protocole de retransmission pour coordinateurs — règles d'affichage verbatim
  des résultats agents. Complémentaire à posture/coordination-only (restrictions d'outils).
```

---

## 4 — Rationalization table

Template standard à inclure dans la section REFACTOR de tout skill opérationnel :

```markdown
## Rationalization table

| Rationalisation à risque | Ce qui l'interdit |
|--------------------------|-------------------|
| "[Formulation de la rationalisation 1]" | Section X — "[Règle exacte qui l'empêche]" |
| "[Formulation de la rationalisation 2]" | Section Y — "[Règle exacte qui l'empêche]" |
| "[Formulation de la rationalisation 3]" | ❌ règle explicite ligne Z |
```

**Rationalisations fréquentes à tester systématiquement :**

| Rationalisation | Formulation type |
|-----------------|-----------------|
| Implicite suffisant | "Le résultat implique que X, donc je n'ai pas besoin de vérifier" |
| Optimisation du chemin | "Je peux sauter cette étape si le contexte est clair" |
| Interprétation favorable | "La règle ne dit pas que je ne peux pas faire Y" |
| Urgence | "Dans ce cas, les règles habituelles ne s'appliquent pas" |
| Délégation silencieuse | "Je vais supposer que l'étape précédente a été faite" |

---

## 5 — Anti-patterns

### Narrative sans règle actionnable

❌ Problème :
```markdown
Il est important de toujours vérifier les tests avant de terminer.
La qualité du code est une responsabilité partagée.
```

✅ Correct :
```markdown
Avant tout `DONE`, exécuter les 3 checks suivants — si l'un échoue, bloquer :
1. Tests passent (ou justification documentée)
2. Comportement observable conforme à la spec
3. Aucune régression non documentée
```

---

### Multi-language dilution

Un skill bilingue FR/EN dans le même fichier force le modèle à moyenner les deux instructions — les règles s'affaiblissent mutuellement.

❌ Problème :
```markdown
Ne jamais résumer le rapport. / Never summarize the report.
```

✅ Correct : un skill par langue cible, ou skill en anglais avec termes techniques uniquement (sans prose narrative bilingue).

---

### Labels génériques dans `description:`

❌ :
```yaml
description: Guide de bonnes pratiques
description: Protocole standard
description: Instructions pour l'agent
```

✅ : Voir critères SDO — toujours citer le déclencheur + le comportement imposé.

---

### Code dans flowcharts

Les blocs de code inline dans un flowchart Mermaid ou ASCII sont parfois exécutés par le modèle au lieu d'être lus comme de la documentation.

❌ :
```
flowchart TD
  A["bd create 'Titre' --json"] --> B["T_ID=$(echo $T | jq '.id')"]
```

✅ : Pseudo-code en langage naturel dans les flowcharts ; code réel dans des blocs ` ```bash ``` ` séparés.

---

### Over-specification

Spécifier chaque micro-décision tue le jugement du modèle sur les cas non couverts.

❌ : Skill de 800 lignes qui couvre chaque edge case imaginable → le modèle ne retient plus les règles importantes.

✅ : Skill focalisé sur les 5-10 règles critiques + rationalization table pour les cas limites. Les cas non couverts bénéficient du jugement général.

---

## 6 — Checklist de validation finale

Avant de merger ou de déclarer un skill terminé :

**Contenu**
- [ ] Scénario RED écrit (cas nominal + cas de rationalisation)
- [ ] Skill minimal GREEN qui passe le scénario
- [ ] REFACTOR effectué — loopholes fermés
- [ ] Rationalization table présente (si skill opérationnel)
- [ ] Anti-patterns vérifiés (narrative, bilingue, générique, code flowchart, over-spec)

**SDO**
- [ ] `description:` ≤ 2 phrases, keywords discriminants, cross-refs si nécessaire
- [ ] `bucket:` renseigné (A ou B)
- [ ] `name:` correspond au path du fichier

**Intégration**
- [ ] Bucket A : skill dans `skills:` des agents concernés + matrice `skills.fr.md` mise à jour
- [ ] Bucket B : skill dans `native_skills:` des agents concernés + déclencheur documenté
- [ ] `docs/architecture/skills.fr.md` — nouvelle entrée dans le bon domaine
- [ ] Si nouvel agent créé : `skills/shared/hub-workflow-reference.md` mis à jour

---

## 7 — Règle de gouvernance

> **Tout nouvel agent ajouté au hub doit inclure une mise à jour de `skills/shared/hub-workflow-reference.md`.**
>
> Ce skill est la source de vérité du catalogue agents. Sans cette mise à jour, le planner et l'orchestrator ne connaissent pas le nouvel agent et ne peuvent pas le router.

Pour la checklist structurelle complète (frontmatter, permissions, placement dans `agents/`), voir [authoring.fr.md](authoring.fr.md).
