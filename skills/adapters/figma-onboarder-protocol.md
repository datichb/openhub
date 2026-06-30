---
name: figma-onboarder-protocol
description: Protocole d'intégration Figma pour l'agent Onboarder — exploration des maquettes projet, extraction des design tokens, détection du design system
---

# Skill — Figma Onboarder Protocol (v1)

## Rôle

Ce skill enrichit la Phase 1 du workflow Onboarder avec les données Figma pour documenter :
- Les maquettes projet disponibles
- Le design system en place
- Les design tokens configurés

## Phase 1.5 — Exploration Figma (optionnelle)

### Objectif

Documenter les ressources design disponibles pour faciliter l'implémentation de features UI futures.

### Déclencheur

Lancer Phase 1.5 si :
- Le projet contient du code frontend (Vue, React, Angular détecté en Phase 1.1)
- OU des composants UI sont présents dans `src/components/` ou équivalent

**Si pas de frontend détecté → skipper Phase 1.5, passer à 1.6.**

### Workflow

#### Étape 1 : Recherche des fichiers Figma

**Annoncer avant d'explorer :**
> "Je vais rechercher les maquettes Figma liées au projet."

Stratégie de recherche progressive — s'arrêter à la première tentative qui retourne des résultats :

**Tentative 1 :** `search_figma_files(<nom depuis package.json "name"> ou <nom du dossier racine>)`

**Tentative 2 (si aucun résultat) :** `search_figma_files(<ID du projet — disponible dans le bootstrap prompt>)`

**Tentative 3 (si aucun résultat) :** `search_figma_files(<champ "Nom" du projet — disponible dans le bootstrap prompt>)`

**Si toujours aucun résultat après les 3 tentatives :**

Afficher :
> "J'ai recherché les fichiers Figma avec les termes [terme1], [terme2], [terme3] — aucun résultat."

Puis appeler `question` :

```
question({
  questions: [{
    header: "Fichiers Figma",
    question: "[Onboarder — Phase 1.5 | Projet : <nom>]\nJe n'ai trouvé aucun fichier Figma pour les termes suivants :\n- [terme1]\n- [terme2]\n- [terme3]\n\nComment procéder ?",
    options: [
      { label: "Fournir le nom du fichier", description: "Préciser le nom exact ou l'URL du fichier Figma à analyser" },
      { label: "Pas de maquettes Figma", description: "Ce projet n'a pas de maquettes Figma — passer à Phase 1.6" },
      { label: "Ignorer pour l'instant", description: "Continuer l'onboarding sans les maquettes" }
    ]
  }]
})
```

→ Ajouter au récap Phase 1 selon la réponse, puis passer à Phase 1.6

**Si fichier(s) trouvé(s) (quelle que soit la tentative) :**
→ Continuer vers Étape 2

#### Étape 2 : Analyse des fichiers Figma (max 3 fichiers pertinents)

Pour chaque fichier trouvé (limiter à 3 fichiers les plus pertinents) :

```
get_file_structure(fileId)
→ Obtenir : nom du fichier, pages, nombre de composants, date de modification

detect_ui_signals(fileId)
→ Obtenir : complexité estimée, signaux UX/UI détectés

extract_design_tokens(fileId)
→ Obtenir : tokens couleur, typo, spacing, effects
```

**Critères de pertinence des fichiers :**
- Nom contient le nom du projet
- Fichier modifié récemment (< 6 mois)
- Fichier contenant "Design System" ou "DS" dans le nom (priorité haute)

#### Étape 3 : Identification du design system

**Critères de détection :**
- Fichier Figma nommé "*Design System*" ou "*DS*" ou "*Components*" ou "*Composants*"
- Présence de tokens structurés dans Figma Variables (≥ 5 tokens)
- Composants nommés selon une convention reconnaissable :
  - DSFR* (Système de Design de l'État Français)
  - Material*, Mui*, Md* (Material Design)
  - Ant*, AntD* (Ant Design)
  - Chakra* (Chakra UI)
  - Custom* ou <NomProjet>* (Design system custom)

**Si design system détecté :**
- Lister les composants principaux (max 10 composants les plus courants)
- Extraire les design tokens via `extract_design_tokens`
- Identifier le framework si reconnu

**Si pas de design system :**
- Mentionner "Pas de design system centralisé détecté dans Figma"

#### Étape 4 : Récap Figma

Produire un résumé structuré à intégrer dans le récap Phase 1 :

```markdown
**Maquettes Figma détectées :**
- Fichiers trouvés : X fichiers
  - [Nom fichier 1](URL)
  - [Nom fichier 2](URL)
- Design system : <Oui — Framework : DSFR / Material / Custom | Non>
  - Composants disponibles : <liste des 10 composants principaux>
- Design tokens : <X tokens couleur, Y tokens typo, Z tokens spacing | Non configurés>
```

**Exemple de sortie complète :**

```markdown
**Maquettes Figma détectées :**
- Fichiers trouvés : 2 fichiers
  - [MonApp - Design System](https://figma.com/file/abc123...)
  - [MonApp - Dashboard](https://figma.com/file/def456...)
- Design system : Oui — Framework : DSFR
  - Composants disponibles : DsfrButton, DsfrInput, DsfrCard, DsfrModal, DsfrBadge, DsfrAlert, DsfrTabs, DsfrSelect, DsfrCheckbox, DsfrRadio
- Design tokens : 12 tokens couleur, 8 tokens typo, 5 tokens spacing
```

---

## Intégration dans les fichiers de sortie

### Dans ONBOARDING.md

Ajouter cette section après "## Architecture" :

```markdown
## Design et maquettes

**Fichiers Figma :**
- [Nom fichier — URL](lien)

**Design system :**
- Framework : <DSFR / Material / Custom / Aucun>
- Composants disponibles : <liste>

**Design tokens :**
- Couleurs : <tokens principaux>
- Typographie : <tokens typo>
- Espacements : <tokens spacing>

> Pour utiliser : consulter `config/figma.conventions.md`
```

**Si aucun Figma détecté :**
```markdown
## Design et maquettes

Non disponible — design probablement géré en code uniquement.
```

**Si projet backend (Phase 1.5 skippée) :**
```markdown
## Design et maquettes

Non applicable (projet backend).
```

### Dans CONVENTIONS.md

Ajouter cette section après "## Config & secrets" :

```markdown
---

## Design tokens

**Source :** <Figma Variables (fichier : [Design System](URL)) / Code CSS/SCSS / Aucun>

**Tokens couleurs :**
- `color/primary` : <valeur hex>
- `color/secondary` : <valeur hex>
- `color/error` : <valeur hex>
- `color/success` : <valeur hex>
<liste complète si Figma Variables configurées — max 15 tokens>

**Tokens typographie :**
- `text/heading-1` : <font-family, size, weight>
- `text/body` : <font-family, size, weight>
- `text/caption` : <font-family, size, weight>
<liste complète — max 10 tokens>

**Tokens espacements :**
- `space/xs` : <valeur>px
- `space/sm` : <valeur>px
- `space/md` : <valeur>px
- `space/lg` : <valeur>px
- `space/xl` : <valeur>px
<liste complète — max 8 tokens>

**Synchronisation :** <Manuel / Plugin Figma Tokens → CSS / Non configurée>

> ⚠️ Source de vérité : <Figma / Code CSS>

<Vide si aucun design token détecté — ne rien afficher dans ce cas>
```

---

## Règles importantes

✅ **Phase 1.5 est optionnelle** : Ne la déclencher que si le projet contient du code frontend  
✅ **Recherche progressive** : Essayer dans l'ordre — nom dossier/package.json → ID projet → Nom projet → question utilisateur  
✅ **Maximum 3 fichiers Figma** : Garder les plus pertinents pour ne pas surcharger le récap  
✅ **Toujours mentionner** : Même si aucun fichier trouvé, le dire explicitement dans le récap Phase 1  
✅ **Extraction tokens** : Utiliser `extract_design_tokens` — ne pas analyser manuellement  
✅ **URLs complètes** : Toujours inclure les liens directs vers les fichiers Figma  
❌ **Ne jamais abandonner sans question** : Si les 3 tentatives échouent, appeler `question` — ne pas passer à Phase 1.6 silencieusement  
❌ **Ne jamais bloquer** : Si erreur Figma (API, permissions), continuer sans et le mentionner dans "Zones d'ombre"  
❌ **Ne pas analyser manuellement** : Utiliser les outils MCP, pas d'inspection visuelle des maquettes  
❌ **Ne pas dupliquer** : Les données Figma enrichissent le récap Phase 1, pas une section séparée complète

---

## Exemples

### Exemple 1 : Projet backend (Phase 1.5 skippée)

**Profil détecté en Phase 1.1 :** Backend Node.js (Express + PostgreSQL)

**Phase 1.5 :** Skippée automatiquement (pas de code frontend détecté)

**Récap Phase 1 :** Section "Maquettes Figma" absente

**ONBOARDING.md :** Section "Design et maquettes" avec mention "Non applicable (projet backend)"

---

### Exemple 2 : Projet frontend avec design system DSFR

**Profil détecté en Phase 1.1 :** Frontend Vue 3 + Nuxt 4

**Phase 1.5 :**

1. `search_figma_files("MonApp")` → 2 fichiers trouvés :
   - "MonApp - Design System"
   - "MonApp - Dashboard"

2. Analyse du fichier "MonApp - Design System" :
   ```
   get_file_structure(fileId1)
   → 4 pages, 25 composants
   
   detect_ui_signals(fileId1)
   → Complexité : M, Signal UI détecté (composants nombreux)
   
   extract_design_tokens(fileId1)
   → 12 tokens couleur, 8 tokens typo, 5 tokens spacing
   ```

3. Design system détecté : DSFR (composants nommés Dsfr*)

**Récap Phase 1 enrichi :**
```markdown
**Maquettes Figma détectées :**
- Fichiers trouvés : 2 fichiers
  - [MonApp - Design System](https://figma.com/file/abc...)
  - [MonApp - Dashboard](https://figma.com/file/def...)
- Design system : Oui — Framework : DSFR
  - Composants disponibles : DsfrButton, DsfrInput, DsfrCard, DsfrModal, DsfrBadge, DsfrAlert, DsfrTabs, DsfrSelect, DsfrCheckbox, DsfrRadio
- Design tokens : 12 tokens couleur, 8 tokens typo, 5 tokens spacing
```

**ONBOARDING.md :** Section "Design et maquettes" complète avec URLs et tokens

**CONVENTIONS.md :** Section "Design tokens" complète avec valeurs extraites

---

### Exemple 3 : Projet frontend sans Figma

**Profil détecté en Phase 1.1 :** Frontend React

**Phase 1.5 :**

1. `search_figma_files("MonProjet")` → Aucun fichier trouvé
2. `search_figma_files("mp-monprojet")` (ID projet) → Aucun fichier trouvé
3. `search_figma_files("Mon Projet")` (Nom projet) → Aucun fichier trouvé
4. Appel `question` → utilisateur répond "Pas de maquettes Figma"

**Récap Phase 1 :**
```markdown
**Maquettes Figma détectées :**
- Aucune maquette Figma trouvée — design probablement géré en code uniquement
```

**ONBOARDING.md :** Section "Design et maquettes" avec mention "Non disponible"

**CONVENTIONS.md :** Section "Design tokens" absente (pas de tokens détectés)

---

### Exemple 4 : Erreur d'accès Figma

**Profil détecté en Phase 1.1 :** Frontend Vue.js

**Phase 1.5 :**

1. `search_figma_files("MonProjet")` → Erreur API (403 Forbidden)

**Récap Phase 1 :**
```markdown
**Maquettes Figma détectées :**
- Recherche impossible (erreur d'accès Figma)

**Zones d'ombre identifiées :**
- Maquettes Figma potentiellement présentes mais inaccessibles — vérifier les permissions MCP Figma
```

**ONBOARDING.md :** Section "Design et maquettes" avec mention "Non disponible (erreur d'accès)"

---

## Autocontrôle Phase 1.5

Avant de passer à Phase 1.6, vérifier :

- [ ] Phase 1.5 déclenchée uniquement si frontend détecté ?
- [ ] Les 3 tentatives de recherche effectuées (nom dossier/package → ID projet → Nom projet) avant de conclure à l'absence de résultats ?
- [ ] Si 3 tentatives échouées, `question` appelée pour demander à l'utilisateur ?
- [ ] Fichiers Figma trouvés mentionnés dans le récap avec URLs ?
- [ ] `extract_design_tokens` exécuté si fichiers pertinents trouvés ?
- [ ] Design system identifié si présent ?
- [ ] Erreurs Figma signalées dans "Zones d'ombre" si échec ?
- [ ] Maximum 3 fichiers analysés (les plus pertinents) ?
