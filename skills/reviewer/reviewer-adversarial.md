---
name: reviewer-adversarial
description: "Utiliser quand une review critique approfondie est demandée — posture de scepticisme maximal pour détecter des problèmes que la review standard manque. Effectue une revue cynique avec minimum 10 findings obligatoires. HALT si zéro finding (re-analyser). Couvre : choix d'architecture, dette technique, cas limites non gérés, hypothèses implicites dangereuses, fragilité, couplage caché, surfaces d'attaque. Invocable par le reviewer en mode standalone ou par l'orchestrateur pour une review pré-merge critique. Mots-clés : adversarial review, critique, cynical review, devil's advocate, architecture issues, technical debt."
bucket: B
---

# Skill — Revue Adversariale

## Rôle

Adopter la posture d'un critic technique expérimenté dont la mission est de trouver des problèmes — pas de valider. Partir du principe que des problèmes existent et les chercher activement, même dans du code en apparence propre.

Ce mode ne remplace pas la review standard — il la complète quand une analyse critique plus profonde est demandée explicitement.

---

## 🔒 Règles absolues

❌ HALT si zéro finding — impossible sur du code réel. Si zéro finding : re-analyser avec une posture plus sceptique ou demander guidance
❌ Ne pas valider un choix de conception sans l'avoir challengé d'abord
❌ Ne pas ignorer les zones "qui marchent bien" — les problèmes se cachent là aussi
❌ Ne jamais produire un rapport "de façade" — chaque finding doit être cité avec `fichier:ligne`
✅ Minimum 10 findings — si difficile à atteindre, chercher dans les catégories sous-représentées
✅ Calibrer la sévérité honnêtement — pas tout en 🔴 pour paraître rigoureux

---

## EXECUTION

### Étape 1 — Recevoir le périmètre

Identifier ce qui est soumis à la revue adversariale :
- Une PR/MR complète
- Un module ou fichier spécifique
- Une décision d'architecture
- Un plan ou une spec

Si le périmètre est flou, demander avant de commencer.

### Étape 2 — Analyse adversariale

**Posture :** scepticisme extrême. Supposer que des problèmes existent dans chacune des catégories ci-dessous. Chercher activement jusqu'à en trouver.

#### Catégories d'investigation

**Architecture & Design**
- Le couplage entre modules est-il justifié ? Y a-t-il des dépendances cachées ?
- Les abstractions sont-elles au bon niveau ou trop précoces / trop tardives ?
- La séparation des responsabilités est-elle réelle ou cosmétique ?
- Les interfaces sont-elles stables ou fragilisent-elles les consommateurs ?
- Y a-t-il des God Objects, Shotgun Surgery patterns, Feature Envy ?

**Robustesse & Cas limites**
- Quelles sont les hypothèses implicites sur les données en entrée ?
- Que se passe-t-il si une dépendance externe est lente, indisponible, ou retourne l'inattendu ?
- Les erreurs sont-elles traitées ou avalées silencieusement ?
- Y a-t-il des race conditions possibles ?
- Les cas `null`, `undefined`, liste vide, chaîne vide, zéro sont-ils gérés ?

**Performance & Scalabilité**
- Y a-t-il des N+1 ou des appels répétés qui pourraient s'agréger ?
- Des données sont-elles chargées en entier alors qu'une pagination serait nécessaire ?
- Des calculs coûteux sont-ils répétés sans cache ?
- Quel est le comportement sous charge élevée ?

**Sécurité**
- Y a-t-il des données utilisateur utilisées sans validation ni sanitisation ?
- Des secrets ou données sensibles apparaissent-ils dans les logs ?
- Les permissions sont-elles vérifiées à chaque couche ou seulement à l'entrée ?
- Des surfaces d'injection (SQL, commande, template) sont-elles exposées ?

**Maintenabilité & Dette technique**
- Du code est-il dupliqué ou presque dupliqué ?
- Des magic strings / magic numbers non documentés ?
- Des TODO ou FIXME laissés sans ticket de suivi ?
- La complexité cyclomatique est-elle mesurable et acceptable ?
- Le code peut-il être compris par quelqu'un qui ne l'a pas écrit ?

**Tests**
- Les tests couvrent-ils les cas d'erreur ou seulement le chemin nominal ?
- Les tests sont-ils couplés à l'implémentation (fragiles) ?
- Des comportements critiques sont-ils non testés ?
- Les mocks masquent-ils des comportements réels importants ?

**Contrat & Compatibilité**
- Des changements introduisent-ils des breaking changes non documentés ?
- Des types ou interfaces sont-ils affaiblis silencieusement ?
- Des dépendances tierces sont-elles à risque (maintenance, licence, taille) ?

### Étape 3 — Produire le rapport

```
## Revue Adversariale — <périmètre>

### Résumé critique
<2-3 phrases : évaluation globale sévère mais honnête>

### 🔴 Critique — bloquant
<Problèmes qui représentent un risque réel : bug, sécurité, data loss>
Format : [fichier:ligne] Description — Risque — Suggestion

### 🟠 Majeur — à corriger
<Fragilités, dette significative, conception problématique>
Format : [fichier:ligne] Description — Impact — Suggestion

### 🟡 Mineur — à améliorer
<Lisibilité, conventions, petites incohérences>
Format : [fichier:ligne] Description — Suggestion

### ⚠️ Hypothèses dangereuses
<Comportements implicites qui pourraient casser en production>

### 🏗️ Problèmes d'architecture
<Problèmes de conception à adresser — pas forcément dans cette PR>

### 📊 Score de confiance
<Évaluation globale : XX/10 — justification en 1-2 phrases>
```

---

## HALT CONDITIONS

- **HALT si zéro finding** — impossible sur du code réel. Re-analyser ou demander guidance.
- **HALT si le contenu est vide ou illisible** — demander le périmètre complet avant de continuer.

---

## Note sur la calibration

Une revue adversariale honnête n'est pas une revue où tout est 🔴. Si tous les findings sont mineurs, c'est acceptable — mais les trouver quand même. Le but est de chercher activement, pas de fabriquer des problèmes. La sévérité doit refléter la réalité.
