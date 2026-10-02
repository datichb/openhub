# Templates — Handoff design → orchestrator

## Bloc `## Retour vers orchestrator`

```
---

## Retour vers orchestrator

**Agent :** designer
**Ticket :** #<ID> — <titre>

### Spec complète

<User flows intégraux avec tous les états, wireframes textuels, tokens, composants, critères d'acceptance UX/UI — JAMAIS résumée, même si longue.>

<Le contenu exact dépend du mode (ux, ui, ux+ui, recon) — voir les skills designer-protocol et designer-subagent pour le détail de ce que chaque mode produit.>

### Contraintes d'implémentation
- <contrainte 1 — ex : responsive mobile-first obligatoire, ratio de contraste WCAG AA minimum, etc.>
- <contrainte 2>
<"Aucune" si pas de contrainte spécifique identifiée>

### Points ouverts
- <question en suspens 1 — ce qui n'a pas été tranché et nécessite une décision avant ou pendant l'implémentation>
- <question en suspens 2>
<"Aucun" si tous les points ont été tranchés>

### Alternatives écartées
- `<alternative 1>` : <pourquoi écartée>
- `<alternative 2>` : <pourquoi écartée>
<"Aucune" si aucune alternative notable n'a été explorée>

### Statut
`spec-complète` | `spec-partielle` | `bloqué`
```

---

## Bloc `## Retour intermédiaire vers orchestrator`

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** designer
**Mode :** <recon|ux|ui|ux+ui>
**Phase :** Clarification en cours de session
**task_id :** <sessionID courant>

### Ce qui a été exploré jusqu'ici
- <observation 1>
- <observation 2>

### Problème détecté
<Description précise de la clarification nécessaire>

### Impact
<Conséquence sur la spec si on continue sans cette information>

### Hypothèse possible
<Formulation de l'hypothèse si l'utilisateur préfère continuer>
```

---

## Bloc `## Question pour l'orchestrator`

```markdown
## Question pour l'orchestrator

**Phase :** Clarification design
**task_id :** <sessionID courant>

**Contexte :** <Description du problème et de son impact>

**Question :** <Question précise>

**Options :**
- `<label-a>` — <description>
- `<label-b>` — <description>

**Instruction de reprise :** "Réponse clarification design : [option]. [Information si applicable]. Reprendre la production de la spec."
```

---

## Bloc Prototype

Produit quand le skill `prototype-protocol` est chargé. Remplace le bloc standard.

```markdown
## Retour prototype vers orchestrator

**Agent :** designer (prototype)
**Mode :** <ux | ui | ux+ui>
**Question :** <la question exacte qui a déclenché le prototype>
**task_id :** <sessionID courant>

### Artefact
<wireframe ASCII, description structurée de layout, schéma de flow,
ou tableau de variantes — selon le type de question>

### Réponse à la question
<1-3 phrases : ce que l'artefact révèle sur la question posée>

### Recommandation
- `spec-complete-necessaire` — L'artefact révèle qu'une spec UX/UI complète est nécessaire pour ce ticket
- `resolu` — La question est résolue, pas besoin de spec supplémentaire
- `clarification` — L'artefact soulève une nouvelle question : <question>

### Statut : prototype-livré
```
