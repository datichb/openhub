---
name: skill-authoring-protocol
description: Protocole condensé d'authoring skills — TDD RED/GREEN/REFACTOR, SDO checklist
  (description discriminante, keyword coverage, token efficiency), 5 anti-patterns, rationalization
  table template, checklist de validation 12 points. Charger quand le documentarian crée ou
  améliore un skill.
bucket: B
---

# Skill — Authoring Protocol

## Quand charger ce skill

Charger via `skill("shared/skill-authoring-protocol")` quand :
- Création d'un nouveau skill
- Amélioration qualitative d'un skill existant (REFACTOR)
- Revue SDO d'un skill existant

> Guide complet : `docs/guides/authoring-skills.md`

---

## Types de skills

| Type | Quand l'utiliser |
|------|-----------------|
| **Technique** | Workflow procédural — phases, templates, autocontrôles |
| **Pattern** | Contrainte récurrente — règles ✅/❌, triggers |
| **Reference** | Catalogue / lookup — tableaux, source de vérité |
| **Discipline** | Norme comportementale transversale — posture |

---

## TDD — 3 étapes

### RED (avant d'écrire)
Rédiger en langage naturel :
1. **Cas nominal** : "Quand [situation], l'agent doit [comportement observable]."
2. **Cas de rationalisation** : "Si l'agent peut justifier [comportement à éviter], le skill doit l'interdire explicitement."

### GREEN (skill minimal)
- Couvrir uniquement les cas RED
- Chaque ligne répond à un scénario — rien de plus

### REFACTOR (fermer les loopholes)
Pour chaque règle : "Comment un modèle respecterait-il la lettre en violant l'esprit ?"
→ Ajouter à la rationalization table
→ Ajouter un autocontrôle si nécessaire

---

## SDO — Checklist description

| Critère | ✅ Correct | ❌ À éviter |
|---------|-----------|------------|
| **Discriminant** | Cite déclencheur + comportement imposé | "Guide de bonnes pratiques" |
| **Keywords** | Synonymes + formulations alternatives | Un seul terme exact |
| **Token efficiency** | ≤ 2 phrases, keywords en premier | Description > 3 lignes |
| **Cross-refs** | Pointe vers skills connexes si ambiguïté | Isolation silencieuse |

---

## Rationalization table — template

```markdown
## Rationalization table

| Rationalisation à risque | Ce qui l'interdit |
|--------------------------|-------------------|
| "Le résultat implique X, pas besoin de vérifier" | Section X — règle Y |
| "Je peux sauter cette étape si le contexte est clair" | ❌ ligne Z |
| "La règle ne dit pas que je ne peux pas faire Y" | Section Z — interdiction explicite |
```

**Rationalisations fréquentes à tester :**
- Implicite suffisant / Optimisation du chemin / Interprétation favorable / Urgence / Délégation silencieuse

---

## Anti-patterns (5)

| Anti-pattern | Symptôme | Fix |
|---|---|---|
| **Narrative** | "Il est important de..." | Règle actionnable + interdiction explicite |
| **Bilingue** | FR + EN dans le même fichier | Un skill par langue cible |
| **Label générique** | `description: Guide standard` | SDO — citer déclencheur + comportement |
| **Code dans flowchart** | `bd create...` dans Mermaid | Pseudo-code naturel + blocs bash séparés |
| **Over-spec** | Skill > 500 lignes sur tout | 5-10 règles critiques + rationalization table |

---

## Checklist de validation (12 points)

**Contenu**
- [ ] Scénario RED : cas nominal + cas de rationalisation écrits
- [ ] Skill GREEN : couvre les scénarios RED sans excès
- [ ] REFACTOR : loopholes fermés, rationalization table présente
- [ ] Anti-patterns vérifiés

**SDO**
- [ ] `description:` ≤ 2 phrases, discriminante, keywords couverts
- [ ] `bucket:` renseigné (A ou B)
- [ ] `name:` correspond au path

**Intégration**
- [ ] Bucket A → dans `skills:` agents + `skills.fr.md` mis à jour
- [ ] Bucket B → dans `native_skills:` agents + déclencheur documenté
- [ ] Entrée dans `docs/architecture/skills.fr.md`
- [ ] Si nouvel agent : `skills/shared/hub-workflow-reference.md` mis à jour
- [ ] Si skill de type Reference : `source-of-truth: true` dans frontmatter
