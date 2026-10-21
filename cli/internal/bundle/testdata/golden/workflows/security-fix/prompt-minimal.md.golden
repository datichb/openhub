Mode de workflow : semi-auto
Langue de réponse : fr

Correction de constats de sécurité sur demo : choisir les constats, les corriger avec un test de non-régression, vérifier par une nouvelle analyse ciblée, puis committer.

Aucun constat donné au lancement, et aucun audit de sécurité du projet : avant toute autre étape, demande-les à l'utilisateur avec l'outil `question` (réponse libre : un rapport d'audit, un extrait ou une liste de constats, avec leur gravité et leur emplacement). Sa réponse est les constats reçus : traite-la telle quelle.

Déroulé (relance chaque agent avec son `task_id`) :
1. `cp-triage` : présente les constats tels que reçus, numérotés `F1`, `F2`… (gravité, emplacement, une ligne de description chacun). La réponse dit lesquels corriger ; les autres sont écartés, avec la raison donnée (plus tard, risque accepté, faux positif…), et ne sont pas touchés. Avant `cp-triage`, oh refuse `bd create`.
2. `developer` (domaine `security`) : transmets-lui les constats retenus, tels que reçus, et les constats écartés avec leur raison, avec ces consignes :
   - d'abord, un ticket par constat écarté, sans fichier intermédiaire : `bd create "<constat>" -t bug -l security --description "<constat tel que reçu, gravité et emplacement> — écarté au tri : <raison>"` (aucun constat retenu : seulement les tickets, puis la fin) ;
   - corriger **seulement** les constats retenus, à la cause et pour toutes les entrées concernées (pas seulement l'exemple du constat) ; pour chacun, un test de non-régression qui échoue sans la correction ; lancer les tests du projet ; **ne pas committer** ; ne toucher à aucun autre fichier ; un constat qui ne se vérifie pas dans le code (faux positif) : ne rien changer et dire pourquoi, avec le fichier et la ligne ;
   - dans son rapport, `### Critères d'acceptance couverts` : une ligne par constat retenu, `- [x] F<n> — corrigé — test : <fichier> <nom du test> — fichiers : <chemins>` ou `- [ ] F<n> — faux positif : <raison> (<fichier>:<ligne>)`.
3. `auditor-subagent` (domaine `security`, nouvelle analyse **ciblée**, pas un audit complet) : transmets-lui les constats retenus, le rapport du developer (`rp-…`) et sa section `### Critères d'acceptance couverts` telle quelle. Pour chaque constat : `corrigé`, `encore présent` ou `contournable`, avec l'entrée qui passe encore (autre chemin, préfixe voisin, lien symbolique, autre encodage, autre appelant), le fichier et la ligne.
4. `cp-rescan` : passe-le avec le verdict de chaque constat ; oh lance les tests et vérifie qu'un test a été écrit. Un constat `encore présent` ou `contournable`, ou un refus d'oh : relance le developer avec le verdict ou la raison, puis l'auditeur, puis `cp-rescan` — au plus `max_fix_rounds` tours (section « Mode de workflow »), puis `cp-2` avec les points non résolus.
5. `cp-2` : corrections, tests de non-régression, verdict de la nouvelle analyse. Avant sa validation, oh refuse tout commit. Une fois validé, relance le developer : un commit `fix(security): <constats corrigés>` des fichiers modifiés.
6. Fin : constats corrigés (commit), constats écartés au tri (raison et ticket) ou faux positifs (avec la raison), points non résolus. oh relève lui-même les tickets créés. Le push revient à l'utilisateur.
