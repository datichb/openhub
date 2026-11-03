---
name: risk-review
description: "Utiliser à l'entretien d'une spec, après grilling : passe d'avocat du diable sur la demande (cas limites, risques) ; chaque tension devient une question, un risque ou une hypothèse."
---

# Avocat du diable

`grilling` confronte les mots de la demande au code ; ici tu cherches ce qui casserait une fois la feature livrée. Une passe, dans le code et les tickets ouverts : chaque point retenu cite sa preuve (fichier, symbole, ticket).

## Checklist

Les valeurs limites (vide, zéro, négatif, doublon, volume, concurrence) sont dans le tableau de `grilling` ; ajoute :

- **Données existantes ou importées** : valeur absente, vide, `0`, non numérique, ancienne forme ; les seeds et les fixtures de test disent-ils la même chose que la production ?
- **État qui change en cours d'édition** : une valeur ou un choix modifié dans le même formulaire avant la sauvegarde ; que voit-on à chaque étape, et après la sauvegarde ?
- **Droits et rôles** : qui voit, qui modifie ; un rôle de plus ou de moins change-t-il le résultat ?
- **Autres lecteurs des mêmes données** : écrans, API, exports, imports, traitements, composants partagés qui lisent ou affichent ce que tu changes.
- **Accessibilité** (interface) : libellés, clavier, lecteur d'écran, contraste.
- **Migration et fixtures** : schéma ou données à migrer, fixtures de test à mettre à jour.
- **Coordination** : ticket ou MR en cours sur les mêmes fichiers ou le même écran.
- **Non-fonctionnel** : performance (volume, requêtes en plus), journalisation.
- **Cas particuliers du plan** : impacts indirects, configurations (env, feature flags), logique existante à réutiliser, comportement de librairie non vérifié, concept déjà rejeté (`docs/wiki/decisions/decisions-rejetees.md`), doublon avec un ticket ouvert.

## Trier chaque tension

1. Deux réponses mènent à deux plans : question bloquante (`grilling`, §5).
2. Le plan ne change pas, mais une équipe se la poserait : question facultative (second envoi de `grilling`).
3. Sinon, un risque (son impact, la parade ; un risque qui se teste devient un critère `CA-n` avec son test) ou une hypothèse numérotée du rapport.

## Proportion

- Lis les points que la feature touche, pas toute la liste ; un point sans objet ne s'écrit pas.
- Une story déjà détaillée : ne redemande pas ce qu'elle dit, cherche ce qu'elle ne dit pas.
- Rien trouvé : la section Risques de la spec le dit avec sa raison, en une ligne.
