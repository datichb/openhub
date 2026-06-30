---
name: figma-recon-protocol
description: Protocole de reconnaissance Figma légère pour l'agent Designer — mode recon. Détecte les signaux UX/UI, extrait la structure et les tokens de base, produit un bloc structuré compact. Ne réalise pas de spec — recommande l'escalade si pertinent.
bucket: B
---

# Skill — Figma Recon Protocol

## Rôle

Ce skill guide le mode `recon` de l'agent Designer — une reconnaissance Figma légère et rapide (2-3 minutes maximum) pour :
- Détecter les fichiers et la structure design disponibles
- Identifier les signaux UX et UI présents
- Extraire les tokens de base si demandé
- Recommander l'escalade vers un mode plus approfondi

Ce mode **ne produit pas de spec** — il produit un rapport de contexte.

---

## Workflow de reconnaissance

### Étape 1 — Recherche des fichiers

Stratégie progressive — s'arrêter à la première tentative qui retourne des résultats :

**Tentative 1 :** `search_figma_files(<nom du projet>)`

**Tentative 2 (si aucun résultat) :** `search_figma_files(<terme alternatif — feature, composant, ou nom d'équipe>)`

**Si aucun résultat :**
```
question({
  questions: [{
    header: "Fichiers Figma",
    question: "[Designer | Mode recon]\nAucun fichier Figma trouvé pour les termes : [terme1], [terme2].\nComment procéder ?",
    options: [
      { label: "Fournir l'URL ou le nom exact", description: "Préciser le fichier Figma à analyser" },
      { label: "Pas de fichiers Figma disponibles", description: "Conclure la recon sans données Figma" }
    ]
  }]
})
```

### Étape 2 — Analyse de structure

Pour chaque fichier trouvé (max 3) :

1. `get_figma_file(<file_key>)` — obtenir la structure du fichier
2. Identifier : nombre de pages, noms des pages, présence de pages "Components", "Tokens", "Design System"
3. Estimer la complexité : nombre de frames par page, profondeur des composants

### Étape 3 — Extraction des signaux

À partir de la structure, détecter :

**Signaux UX :**
- Présence de flows multi-écrans (plusieurs frames liées)
- Pages de parcours utilisateur ("Flows", "User Journey", "Wireframes")
- Frames avec annotations UX (notes, callouts, états d'erreur)
- Navigation entre écrans visible

**Signaux UI :**
- Page "Design System", "Components", "Tokens", "UI Kit"
- Bibliothèque de composants (library)
- Frames avec variants et propriétés

### Étape 4 — Extraction tokens (si demandé)

Si le prompt mentionne "tokens", "couleurs", "design system" :

1. Identifier la page Tokens/Foundations
2. `get_figma_file_nodes(<file_key>, <node_ids page tokens>)`
3. Extraire : couleurs principales, typographie de base, valeurs de spacing si présentes

---

## Format de retour

```markdown
## Retour recon Figma

### Fichiers trouvés
- [nom fichier 1] — [URL] — [date dernière modification]
- [nom fichier 2] — [URL] — [date]
(Aucun fichier trouvé — si applicable)

### Signaux détectés
- UX: oui/non — [détail — ex: 3 flows multi-écrans, page "User Flows" présente]
- UI: oui/non — [détail — ex: page "Components" avec 24 composants, design tokens définis]
- Complexité: [XS/S/M/L/XL] — [justification courte]
- Composants: [nombre estimé] (si applicable)

### Design tokens (si demandé)
- Couleurs: [résumé — ex: palette primary blue #3B82F6 + 8 niveaux, sémantique error/success/warning]
- Typographie: [résumé — ex: Inter, 3 tailles définies (14/16/24px)]
- Spacing: [résumé — ex: échelle 4px de base, 8 niveaux]
(Section omise si tokens non demandés)

### Recommandation
[Escalade vers mode ux/ui/ux+ui si pertinent — ex: "Signal UX fort détecté (3 flows multi-étapes) → recommande mode ux+ui pour spec complète avant planification"]
[Ou: "Recon suffisante pour le besoin identifié — aucune escalade nécessaire"]
```

---

## Règles

✅ Rester en mode recon — ne pas produire de spec
✅ Signaler explicitement les signaux UX et UI détectés
✅ Recommander l'escalade quand les signaux sont clairs
❌ Ne pas analyser en profondeur les composants individuels (mode recon uniquement)
❌ Ne pas déduire des flows sans frames liées visibles
