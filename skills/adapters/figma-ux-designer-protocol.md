---
name: figma-ux-designer-protocol
description: Protocole d'intégration Figma pour l'agent UX Designer — lecture des maquettes existantes, extraction des user flows et composants, détection des frictions UX à partir des écrans Figma. Charger via native_skills avant d'analyser une feature avec des maquettes disponibles.
bucket: B
---

# Skill — Figma UX Designer Protocol

## Rôle

Ce skill guide l'agent UX Designer dans l'exploitation des fichiers Figma disponibles pour :
- Lire les maquettes et user flows existants avant de spécifier
- Identifier les frictions UX déjà présentes dans les designs
- Contextualiser les spécifications UX par rapport à l'existant visuel

---

## Quand charger ce skill

Charger ce skill (via l'outil `skill`) dès qu'une des conditions suivantes est remplie :
- La demande mentionne des écrans, maquettes, ou composants UI existants
- Un lien Figma ou un nom de fichier Figma est fourni dans le prompt
- Le projet a un historique de design (onboarder a détecté Figma)

---

## Workflow d'exploration Figma

### Étape 1 — Rechercher le fichier Figma

Stratégie progressive — s'arrêter à la première tentative qui retourne des résultats :

**Tentative 1 :** `search_figma_files(<nom du projet ou de la feature>)`

**Tentative 2 (si aucun résultat) :** `search_figma_files(<terme alternatif — ex : nom du ticket ou du composant>)`

**Si aucun résultat :**
```
question({
  questions: [{
    header: "Fichiers Figma",
    question: "[UX Designer | Feature : <nom>]\nAucun fichier Figma trouvé pour les termes : [terme1], [terme2].\nComment procéder ?",
    options: [
      { label: "Fournir l'URL ou le nom", description: "Préciser le fichier Figma à analyser" },
      { label: "Pas de maquettes Figma", description: "Continuer sans données Figma" }
    ]
  }]
})
```

### Étape 2 — Explorer le fichier

Une fois le fichier identifié :

1. `get_figma_file(<file_key>)` — obtenir la structure du fichier
2. Identifier les pages pertinentes pour la feature analysée
3. Pour chaque page pertinente : `get_figma_file_nodes(<file_key>, <node_ids>)` pour lire les frames et composants clés

### Étape 3 — Extraire les informations UX

À partir des données Figma, extraire et documenter :

| Information | Comment l'extraire |
|-------------|-------------------|
| **User flows existants** | Lire la séquence des frames dans chaque page |
| **Points de friction visibles** | Chercher les états d'erreur, loaders, messages vides |
| **Navigation** | Identifier les liens entre frames, les back buttons, modals |
| **Formulaires** | Lister les champs, labels, messages de validation présents |
| **États manquants** | Comparer l'état "happy path" avec les états d'erreur / vide / chargement |

### Étape 4 — Intégrer dans la spec UX

Les observations Figma enrichissent directement la spec UX :

- **Contexte existant** : "Les maquettes Figma montrent X — la spec s'appuie sur cette base"
- **Frictions détectées** : "L'écran de confirmation (frame Y) ne gère pas l'état d'erreur réseau"
- **Incohérences** : "Le flow Figma saute l'étape de validation — à clarifier avec le Product Owner"

---

## Règles

✅ Toujours lire les maquettes avant de spécifier pour une feature qui en a
✅ Signaler explicitement les frictions détectées dans Figma
✅ Référencer les frames Figma dans la spec (ex : "Frame 'Confirmation — Error state'")
❌ Ne pas reproduire les frictions existantes dans la spec — les corriger
❌ Ne pas inventer des flows non présents dans Figma sans le préciser
