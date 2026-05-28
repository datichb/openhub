---
name: figma-scout-protocol
description: Protocole d'intégration Figma pour l'agent Scout — enrichissement contextuel avec maquettes et détection automatique de signaux UX/UI
---

# Skill — Figma Scout Protocol (v1)

## Rôle

Ce skill enrichit le workflow du Scout avec les données Figma pour améliorer la précision des estimations et la détection des signaux design.

## Workflow enrichi

### Étape 3bis — Vérification Figma (optionnelle, après exploration codebase)

Si la feature touche une interface utilisateur, explorer les maquettes Figma disponibles.

#### Quand l'utiliser

- La feature mentionne des composants UI (bouton, formulaire, modal, etc.)
- La feature touche une page/vue existante
- Le nom de la feature correspond potentiellement à un fichier Figma

#### Comment procéder

1. **Rechercher les fichiers Figma**
   ```
   Utiliser l'outil : search_figma_files
   Argument : nom de la feature ou mots-clés UI
   ```

2. **Si fichier(s) trouvé(s) :**
   - Utiliser `detect_ui_signals` sur chaque fichier pertinent
   - Analyser les métriques retournées :
     - Complexité (XS/S/M/L/XL)
     - Signal UX détecté (oui/non)
     - Signal UI détecté (oui/non)
     - Nombre de composants
   - Ajuster l'estimation initiale si nécessaire

3. **Si aucun fichier trouvé :**
   - Mentionner dans le rapport qu'aucune maquette Figma n'a été trouvée
   - Continuer avec l'estimation basée sur le code uniquement

### Ajustement de l'estimation

Utiliser les données Figma pour ajuster la complexité :

| Contexte | Ajustement |
|----------|------------|
| Composants Figma < 3 | Aucun ajustement |
| Composants Figma 3-5 | +1 ticket UI |
| Composants Figma 6-10 | +2 tickets UI |
| Composants Figma > 10 | +1 niveau de complexité (M → L) |
| Signal UX détecté | Mentionner besoin spec UX |
| Signal UI détecté | Mentionner besoin spec UI |

### Format de sortie enrichi

Ajouter cette section dans le rapport Scout si maquettes trouvées :

```markdown
## 🎨 Contexte Figma détecté

**Fichiers Figma liés :**
- [Nom du fichier — URL]

**Analyse automatique :**
- Complexité estimée : M (ajustée depuis S)
- Composants détectés : 7
- États visuels : hover, focus, error
- Signal UX : ⚠️ Oui — Flow multi-étapes détecté
- Signal UI : ⚠️ Oui — 7 composants à créer/modifier

**Impact sur l'estimation :**
- Ajout de 2 tickets UI (composants)
- Complexité relevée de S à M
- Recommandé : Escalader au planner (complexité + signaux design)
```

## Exemples

### Exemple 1 : Feature simple sans maquette

**Feature :** "Ajouter un champ email dans le formulaire profil"

**Workflow :**
1. `search_figma_files("profil")` → Aucun résultat
2. Rapport : "Aucune maquette Figma trouvée. Estimation basée sur le code uniquement."
3. Estimation : 1 ticket task, ~45min, **XS**

### Exemple 2 : Feature avec maquette simple

**Feature :** "Page de tableau de bord utilisateur"

**Workflow :**
1. `search_figma_files("tableau de bord")` → 1 fichier trouvé
2. `detect_ui_signals(fileId)` → Retour :
   - Complexité : S
   - Composants : 4
   - Signal UX : Non
   - Signal UI : Oui (4 composants)
3. Ajustement : +1 ticket UI
4. Estimation finale : **S** (2 tickets : 1 backend + 1 frontend)

### Exemple 3 : Feature complexe avec flow

**Feature :** "Processus d'inscription multi-étapes"

**Workflow :**
1. `search_figma_files("inscription")` → 1 fichier trouvé
2. `detect_ui_signals(fileId)` → Retour :
   - Complexité : L
   - Composants : 12
   - Flow multi-étapes : Oui
   - Signal UX : ⚠️ Oui
   - Signal UI : ⚠️ Oui
3. Ajustement : Complexité passée de M à L
4. **Recommandation : 🎯 Escalade au planner**
5. Raison : Complexité L + signaux design forts

## Règles importantes

✅ **Toujours mentionner** si une recherche Figma a été effectuée (trouvé ou non)
✅ **Justifier les ajustements** d'estimation basés sur Figma
✅ **Inclure les URLs Figma** dans le rapport (liens directs)
✅ **Rester rapide** : Ne pas analyser en profondeur, juste collecter les métriques
❌ **Ne jamais forcer** : Si search échoue, continuer sans Figma
❌ **Ne pas analyser** les maquettes manuellement (l'outil le fait)

## Autocontrôle

Avant de finaliser le rapport, vérifier :

- [ ] Recherche Figma effectuée si feature UI ?
- [ ] URLs Figma incluses si fichiers trouvés ?
- [ ] Estimation ajustée selon les métriques Figma ?
- [ ] Recommandations cohérentes avec les signaux détectés ?
