---
name: figma-planner-protocol
description: Protocole d'intégration Figma pour l'agent Planner — enrichissement Phase 1 avec exploration contextuelle Figma et détection automatique de signaux design
---

# Skill — Figma Planner Protocol (v1)

## Rôle

Ce skill enrichit la Phase 1 (Exploration contextuelle) du Planner avec les données Figma pour améliorer la détection des signaux UX/UI et pré-remplir le contexte design.

## Phase 1.3 — Exploration Figma (optionnelle)

Cette phase se place **après Phase 1.2 (Codebase)** et **avant Phase 2 (Questions)**.

### Objectif

Enrichir le contexte avec les maquettes et specs design disponibles dans Figma pour :
- Détecter automatiquement les signaux UX/UI
- Identifier les composants concernés
- Préparer les données pour Phase 1.5 (Délégation design)

### Déclencheur

Lancer Phase 1.3 si **au moins un** de ces critères :
- La feature mentionne des composants UI (bouton, formulaire, page, modal, etc.)
- La feature touche l'interface utilisateur
- Des composants Vue/React sont identifiés en Phase 1.2

### Workflow

#### Étape 1 : Recherche

```
Utiliser l'outil : search_figma_files
Argument : nom de la feature ou mots-clés UI principaux
```

**Si aucun fichier trouvé :**
- Mentionner dans le récap Phase 1 : "Aucune maquette Figma trouvée"
- Passer directement à Phase 2 (questions)

**Si fichier(s) trouvé(s) :**
- Continuer vers Étape 2

#### Étape 2 : Analyse des fichiers

Pour chaque fichier pertinent (max 3) :

```
Utiliser l'outil : get_file_structure
Argument : fileId du fichier
→ Obtenir : frames, composants count

Utiliser l'outil : detect_ui_signals
Argument : fileId du fichier
→ Obtenir : signaux UX/UI, complexité, recommandations
```

#### Étape 2bis : Analyse d'un nœud spécifique (si node ID fourni)

Si l'utilisateur a fourni une URL Figma contenant `?node-id=` (ex: `https://www.figma.com/file/ABC123?node-id=122-29189`) :

```
Utiliser l'outil : get_node_details
Arguments : fileId (clé du fichier), nodeId (valeur de ?node-id=)
→ Obtenir : layout (colonnes/lignes), spacing, padding, propriétés composant, géométrie
```

Utiliser ces détails pour :
- Décrire précisément les colonnes, filtres ou composants d'une zone
- Affiner le découpage en tickets (chaque nœud enfant peut être un ticket)
- Pré-remplir le champ `--design` des tickets avec les propriétés Figma exactes

#### Étape 3 : Enrichissement du contexte

Intégrer les données Figma dans le récap Phase 1 :

**À ajouter dans le récap :**
- URLs Figma des fichiers trouvés
- Nombre de frames et composants
- Signaux UX/UI détectés automatiquement
- Composants identifiés (pour aide au découpage)

### Intégration au récap Phase 1

Ajouter cette section dans le récap de fin de Phase 1 :

```markdown
## [Phase 1] Exploration contextuelle terminée

**Fichiers explorés :** X fichiers lus (codebase)

**Observations principales :**
- Architecture : [pattern]
- Stack : [stack détecté]
- Conventions : [conventions]

**Maquettes Figma explorées :**
- **[Nom fichier]** — [URL Figma]
  - Frames : X
  - Composants : Y
  - Dernière modification : [date]

**Signaux design détectés (via Figma) :**
- **UX** : ⚠️ oui — Flow multi-étapes détecté (3 frames séquentiels)
- **UI** : ⚠️ oui — 7 composants à créer/modifier

**Composants Figma identifiés :**
- DsfrButton (variantes: primary, secondary, tertiary)
- DsfrInput (avec états error, disabled)
- DsfrModal
- CustomCard (composant custom)

**Zones d'ombre identifiées :**
- [zones non couvertes par Figma]

**Dépendances techniques identifiées :**
- [dépendances]

**Risques détectés :**
- [risques]

**Points d'attention :**
- [points d'attention]
```

### Impact sur Phase 1.5 (Délégation design)

Si `detect_ui_signals` retourne :
- **hasUXSignal = true** → Proposer délégation à `ux-designer` (workflow standard Phase 1.5)
- **hasUISignal = true** → Proposer délégation à `ui-designer` (workflow standard Phase 1.5)

La Phase 1.5 reste inchangée, mais le contexte est enrichi :
- Les specs Figma sont déjà identifiées
- Les URLs sont déjà disponibles
- Les composants sont listés

### Impact sur Phase 5 (Création tickets)

Lors de la création des tickets UI, utiliser les données Figma pour :

1. **Pré-remplir le champ `--design`** avec :
   ```markdown
   ## Maquettes Figma
   - [Nom fichier — URL]
   
   ## Composants identifiés
   - [Liste des composants Figma]
   
   ## États détectés
   - [États visuels : hover, focus, error, etc.]
   ```

2. **Enrichir les critères d'acceptance** avec :
   - Référence au fichier Figma
   - Composants DSFR à utiliser (si détectés)

## Exemples

### Exemple 1 : Aucune maquette trouvée

**Feature :** "Ajouter un endpoint API /users"

**Workflow Phase 1.3 :**
1. `search_figma_files("users")` → Aucun résultat
2. Récap : "Aucune maquette Figma trouvée (feature backend uniquement)"
3. Phase 1.5 : Non déclenchée
4. Continuer vers Phase 2

### Exemple 2 : Maquette trouvée, signaux détectés

**Feature :** "Tableau de bord utilisateur"

**Workflow Phase 1.3 :**
1. `search_figma_files("tableau de bord")` → 1 fichier trouvé
2. `get_file_structure(fileId)` → 8 frames, 5 composants
3. `detect_ui_signals(fileId)` → 
   - Complexité : M
   - hasUXSignal : false
   - hasUISignal : true (5 composants)
4. Récap Phase 1 enrichi avec données Figma
5. Phase 1.5 : Proposer délégation à `ui-designer` (signal UI détecté)

### Exemple 3 : Flow complexe avec signaux forts

**Feature :** "Processus d'inscription multi-étapes"

**Workflow Phase 1.3 :**
1. `search_figma_files("inscription")` → 1 fichier trouvé
2. `get_file_structure(fileId)` → 15 frames, 12 composants
3. `detect_ui_signals(fileId)` →
   - Complexité : L
   - hasUXSignal : true (flow multi-étapes)
   - hasUISignal : true (12 composants)
4. Récap Phase 1 enrichi
5. Phase 1.5 : Proposer délégation à **ux-designer ET ui-designer**

## Question de validation Phase 1 (modifiée)

Si maquettes Figma trouvées, adapter la question de validation :

```
question({
  questions: [{
    header: "Suite du workflow",
    question: "[Planner — Phase 1 complétée | Feature : <nom>]\nExploration terminée (codebase + Figma). X fichiers code lus, Y maquettes Figma analysées. Signaux design détectés : [UX ⚠️ / UI ⚠️]. Comment procéder ?",
    options: [
      { label: "Phase 1.5 — Délégation design (Recommandé)", description: "Invoquer ux-designer/ui-designer avant de planifier" },
      { label: "Skip design — Phase 2", description: "Passer aux questions complémentaires sans spec design" },
      { label: "Explorer davantage", description: "Lire d'autres fichiers avant de décider" }
    ]
  }]
})
```

## Règles importantes

✅ **Phase 1.3 est optionnelle** : Ne la déclencher que si la feature touche l'UI
✅ **Maximum 3 fichiers Figma** : Ne pas surcharger le récap (garder les plus pertinents)
✅ **Toujours mentionner** : Même si aucun fichier trouvé, le dire dans le récap
✅ **URLs Figma** : Toujours inclure les liens directs
✅ **Détection automatique** : Utiliser `detect_ui_signals`, ne pas analyser manuellement
❌ **Ne jamais bloquer** : Si un appel Figma échoue, continuer sans et le mentionner dans le récap Phase 1 :
   - Message contient `indisponible` ou `timeout` → `⚠️ Figma indisponible (timeout) — contexte design non disponible`
   - Message contient `401` ou `Token Figma invalide` → `⚠️ Token Figma invalide — demander à l'utilisateur de vérifier : oc figma status`
   - Message contient `404` ou `Team ID invalide` → `⚠️ Team ID Figma invalide ou inaccessible — demander à l'utilisateur de vérifier : oc figma status`
   - Message contient `403` ou `scopes` → `⚠️ Permissions Figma insuffisantes — vérifier les scopes du token`
   - Résultat vide → `ℹ️ Aucun fichier Figma trouvé pour cette feature`
   - Autre erreur → noter le message brut dans le récap
❌ **Ne pas dupliquer** : Les données Figma enrichissent le récap existant, pas une section séparée complète

## Autocontrôle Phase 1.3

Avant de passer à Phase 2, vérifier :

- [ ] Recherche Figma effectuée si feature UI ?
- [ ] Fichiers Figma trouvés mentionnés dans le récap ?
- [ ] `detect_ui_signals` exécuté sur fichiers pertinents ?
- [ ] Signaux UX/UI intégrés dans le récap ?
- [ ] Phase 1.5 proposée si signaux détectés ?
