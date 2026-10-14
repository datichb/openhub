---
name: debugger-phase-2-3
description: "Utiliser avant les phases 2 et 3 du debugger : questions sur artefacts manquants, diagnostic (reproduction, symptôme et cause, hypothèses vérifiées)."
---

# Phase 2 — Questions complémentaires (artefacts manquants)

Phase **optionnelle** : seulement si la phase 1 a laissé des questions. Sinon, passe à la phase 3.

Affiche d'abord les questions en texte, puis appelle l'outil `question` (répondre ou passer) :

```markdown
## [Phase 2] Questions complémentaires

1. **[Sujet]** : <question issue de la phase 1>
```

Le récap de Phase 2 doit contenir :
questions posées, réponse à chacune (ou « non répondu »), zones d'ombre levées, zones d'ombre restantes et leur effet sur le diagnostic. Selon la validation : phase 3, ou retour en phase 1 si les réponses changent le périmètre.

---

# Phase 3 — Diagnostic

Objectif : trouver **où la faute naît**, pas seulement où elle se voit.

## 3.1 — Reproduire

Comportement observé, comportement attendu, conditions (entrées, état, environnement), fréquence. Ton shell est limité à la
lecture (git, fichiers, Beads) : pour reproduire, lance un test existant du projet
(confirmation demandée à l'utilisateur) ou demande à l'utilisateur de lancer la
commande et de te donner la sortie ; ne cherche pas d'autre moyen d'exécuter du code.
Ce qui manque pour reproduire : liste-le.

**Brouillon de reproduction** — seulement si ton prompt contient la ligne « Brouillons de session (oh) » (workflow `debug`) : écris ton test ou script de reproduction et tes notes dans ton dossier de brouillons (chemin dans l'entrée `oh.scratch` du contexte de session), avec l'outil d'écriture de fichiers : le shell n'y a pas accès. Ce dossier est hors dépôt et supprimé avec la session : décris ce test **en texte** dans le ticket de correction (étapes de reproduction, entrées, résultat attendu), sans le chemin du brouillon. Le `developer`, lecteur déclaré, en fait le test définitif du projet. Le chemin d'un brouillon n'est jamais cité dans le rapport, la documentation ni le wiki.

## 3.2 — Séparer le lieu du symptôme de la cause

La pile, l'assertion qui échoue ou le dernier commit montrent **où une mauvaise valeur est constatée**. La cause est l'endroit où elle **devient** fausse, souvent plus haut.

1. **Lieu du symptôme** : la frame où l'erreur est levée (en haut de la pile en Go, Java, JavaScript ; en bas d'un traceback Python), la première dans le code du projet.
2. **Valeur fautive** : nomme-la précisément (liste vide, identifiant nul, date dans le mauvais fuseau, état périmé).
3. **Remonte sa provenance**, producteur après producteur (appelant, constructeur, parseur, sérialisation, configuration, données d'entrée), en lisant le code de chacun, jusqu'au premier endroit où la valeur n'est plus celle attendue.
4. **Arbitre avec les contrats** : documentation, commentaires, types et tests du producteur. Le producteur promet ce que le consommateur suppose et ne le tient pas → la cause est chez le producteur. Personne ne le promet → elle est chez le consommateur.
5. **Le dernier changement n'est pas coupable par défaut** : il peut seulement révéler un défaut plus ancien. `git log` et `git blame` sur le producteur autant que sur le lieu du symptôme.

## 3.3 — Hypothèses graduées, vérifiées

- Au moins deux hypothèses, dont une **en amont** du lieu du symptôme.
- Pour chacune : la prédiction qu'elle fait (« si H, l'entrée X donne Y »), puis sa vérification (code lu, test existant lancé, exemple tracé à la main) : confirmée, réfutée ou non vérifiée.
- Probabilité : **haute** = prédiction vérifiée ; **moyenne** = cohérente, non vérifiée ; **faible** = possible, sans élément.
- **Cause racine** = l'endroit dont la correction supprime le défaut pour tous ses consommateurs. Un garde-fou au lieu du symptôme (test de bornes, nil, `try`/`catch`) est un palliatif : présente-le comme tel, jamais comme la correction.
- Cherche les autres effets de la même cause (autres consommateurs de la valeur) : ils confirment l'hypothèse et vont dans le ticket.

## Récap de fin de Phase 3

Le récap de Phase 3 doit contenir :

- **Symptôme** : observé, attendu, conditions, fréquence, lieu du symptôme (`fichier:ligne`)
- **Périmètre analysé** : artefacts fournis (stacktrace, logs, description, ticket Beads) et ce qui n'était PAS disponible
- **Chaîne de provenance** : du lieu du symptôme à la cause, une ligne par étape (`fichier:fonction` → valeur)
- **Cause racine** : `fichier:fonction` (ligne), hypothèse principale (probabilité, éléments, vérification), hypothèse secondaire si besoin ; palliatifs écartés et pourquoi ; le ticket de correction (phase 5) reprend cette cause, nommée par son fichier et sa fonction
- **Fichiers impliqués** : fichier / rôle dans le bug

Selon la validation : **Passer à Phase 4** → charger `debugger-phase-4-5`, puis Phase 4 ; **Réviser** → rester en phase 3 ; **Skip Phase 4** → charger `debugger-phase-4-5`, puis Phase 5.
