---
name: designer-standalone
description: Parcours d'exécution du Designer en mode standalone (invoqué directement par l'utilisateur, hors orchestrator) — utilise l'outil question aux checkpoints, propose l'enrichissement living-docs après la spec, bloc handoff non obligatoire.
bucket: B
---

# Skill — Parcours Designer Standalone

> Ce skill est chargé quand le designer est invoqué directement par l'utilisateur (pas via `task` depuis l'orchestrator).

## Principe fondamental

En mode standalone, l'utilisateur voit tout le texte de la session. L'outil `question` est disponible et recommandé aux checkpoints clés pour garantir que la direction prise correspond aux attentes.

**Confirmer le contexte au démarrage :**
> `[designer] Mode : <recon|ux|ui|ux+ui> — Session directe. J'utiliserai l'outil question aux checkpoints.`

---

## Workflow standalone — Mode recon

1. Charger `designer/figma-recon-protocol`
2. Exécuter la reconnaissance Figma
3. Produire le bloc `## Retour recon Figma`
4. Si escalade recommandée → proposer via `question` : "Souhaitez-vous que je produise une spec complète en mode ux/ui/ux+ui ?"

---

## Workflow standalone — Mode ux

1. Explorer le contexte (tickets Beads, codebase, description)
2. Charger `designer/figma-deep-protocol` si Figma disponible
3. **Checkpoint 1** — via `question` : identifier l'utilisateur cible et le problème réel (au moins 2 questions)
4. Produire le user flow nominal + alternatifs + états d'erreur
5. **Checkpoint 2** — via `question` : "Voici le user flow — des ajustements avant la spec complète ?"
6. Produire la spec UX complète (critères d'acceptance, contraintes, périmètre)
7. **Checkpoint final** — validation explicite de la spec
8. Proposer l'enrichissement living-docs si pertinent (voir section ci-dessous)

---

## Workflow standalone — Mode ui

1. Explorer le design system existant (tokens définis, composants existants)
2. Charger `designer/figma-deep-protocol` si Figma disponible
3. Si aucun design system détecté → **Checkpoint 0** via `question` : "Fondations d'abord ou composant directement ?"
4. **Checkpoint 1** — via `question` : identifier le périmètre exact (composant, token, guideline, fondations)
5. Produire la spécification
6. Si décision de direction artistique → **Checkpoint intermédiaire** : proposer 2-3 options justifiées
7. **Checkpoint final** — validation explicite de la spec
8. Proposer l'enrichissement living-docs si pertinent

---

## Workflow standalone — Mode ux+ui

1. Exécuter entièrement le workflow mode **ux** (checkpoints inclus)
2. **Transition** via `question` : "La spec UX est validée. Procéder à la spec UI pour les composants identifiés ?"
3. Exécuter le workflow mode **ui** en intégrant la spec UX comme contexte
4. Proposer l'enrichissement living-docs

---

## Proposition d'enrichissement living-docs

Après la validation de la spec, proposer systématiquement :

```
question({
  questions: [{
    header: "Enrichissement documentation",
    question: "[Designer | Mode <mode> | Feature : <nom>]\nSpec validée. Souhaitez-vous enrichir la documentation vivante du projet avec cette spec ?",
    options: [
      { label: "Oui — enrichir le wiki", description: "Ajouter la spec au wiki projet (docs/wiki/) selon le skill living-docs-enrichment" },
      { label: "Non — spec seule suffit", description: "Terminer ici" }
    ]
  }]
})
```

---

## Règles standalone

✅ Utiliser `question` aux checkpoints clés — ne pas continuer sans valider la direction
✅ Présenter les drafts intermédiaires pour correction avant la spec finale
✅ Proposer des options pour les décisions de direction artistique (mode ui)
❌ Pas d'obligation de produire le bloc `## Retour vers orchestrator` (standalone, pas de consommateur en aval)
✅ Le produire quand le planner a délégué en standalone (format `## Retour vers planner`)
