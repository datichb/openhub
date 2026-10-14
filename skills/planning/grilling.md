---
name: grilling
description: "Utiliser avant de poser des questions : confronter la demande au code pour faire remonter exigences implicites et contradictions, une question par décision."
---

# Interroger la demande

Une demande dit ce que l'utilisateur veut ; le code dit ce que le projet garantit déjà. Les questions utiles naissent là où les deux se frottent. Une question n'a sa place que si deux réponses possibles mènent à deux plans différents.

## 1. Relever ce que la demande affirme

Souligne dans la demande les mots qui engagent :

- les quantificateurs : « tous », « chaque », « toujours », « jamais », « aucun » ;
- les acteurs : qui déclenche, qui reçoit, qui voit le résultat ;
- le temps et la fréquence : « chaque lundi », « en temps réel », « avant l'échéance » ;
- les objets et leurs états : actif, supprimé, archivé, brouillon, expiré, en attente.

## 2. Confronter chaque mot au projet

Pour chacun, cherche ce qui le restreint ou le contredit :

| Où chercher | Ce qui fait naître une exigence implicite |
|---|---|
| Filtres, droits, visibilité (fonctions de lecture, middlewares) | La demande s'adresse à un acteur qui ne voit pas tout : l'élargir ou le restreindre est une décision |
| Cycle de vie des objets (champs d'état, dates de suppression, purge) | « Tous » inclut-il les objets supprimés, archivés, en attente ? |
| Invariants (unicité, plafonds, cardinalités, ordre) | La demande peut les casser sans le dire |
| Données sensibles (personnelles, privées, secrètes) | Les exposer, même à un rôle interne, se décide |
| Limites de valeurs (vide, zéro, négatif, doublon, très grand volume, concurrence) | Le comportement attendu n'est souvent pas écrit |
| Ce qui existe déjà (fonction voisine, ticket ouvert) | Réutiliser ou remplacer change le périmètre |

Note chaque tension avec sa preuve : fichier et symbole (`Store.Visible`), ligne de doc, ticket.

## 3. Trier

- **Fait** : la réponse se lit dans le code ou la doc. Ne la pose pas ; consigne-la avec sa source.
- **Décision** : arbitrage métier, confidentialité, périmètre, préférence. Pose-la.

Une décision déjà « répondue » par une formule qui en confond plusieurs (« appliquer la logique existante », « comme d'habitude ») n'est pas tranchée si cette logique contredit un mot de la demande : repose chaque décision séparément.

## 4. Formuler

Une question par décision, jamais deux décisions dans une question ni une décision cachée dans une option. Le texte de la question se suffit à lui-même (ton appelant le relaie tel quel) :

- il nomme la règle en jeu avec le mot du projet (le champ, l'état, le rôle) ;
- il dit la tension : « la demande dit X, le code garantit Y » ;
- les options sont concrètes, chacune avec sa conséquence sur le plan ; la recommandée en premier.

Exemple (autre domaine) : « La demande dit d'envoyer la lettre d'information à tous les clients, mais `Client.OptOut` marque ceux qui l'ont refusée : faut-il les exclure ? » — options « Exclure les clients `OptOut` (recommandé, conforme au consentement) », « Inclure tout le monde ».

## 5. Prioriser

1. Ce qui change le périmètre ou touche à une donnée sensible ou à un invariant.
2. Ce qui change le découpage (un ticket de plus, une dépendance).
3. Le reste devient une hypothèse écrite dans le rapport, pas une question.

Cinq questions au plus par envoi ; les bloquantes d'abord.

## Interdits

- ❌ Le questionnaire générique (« Quel est l'objectif métier ? », « Des contraintes de délai ? ») quand la demande ou le code y répondent déjà.
- ❌ Une question dont la réponse se lit dans le code.
- ❌ Trancher seul une tension entre la demande et une règle du projet, même avec une hypothèse « raisonnable ».

Sous-agent : les décisions vont dans tes `### Questions bloquantes` (statut `en-attente`), une entrée par décision, avec ses options. Besoin flou dans son ensemble (pas une tension précise) : choisis une technique dans `elicitation-techniques`.
