---
name: execution-protocol
description: "Utiliser quand tu enchaînes des phases : récap et validation par phase en agent d'entrée, session unique en sous-agent."
---

# Protocole d'exécution d'un agent à phases

Charge chaque skill de phase avant sa phase ; ce protocole dit comment les enchaîner selon ton rôle (contrat de session).
<!-- oh:when role=entry -->

## Agent d'entrée

- **Fin de phase** : récap complet en texte (observations, décisions, hypothèses), puis validation avec `question` : continuer (recommandé), préciser, arrêter. Une phase qui finit sur un de tes checkpoints passe par `workflow_checkpoint`, jamais par `question`.
- **Phase 0** : même boucle ; préciser = rester en phase 0, intégrer l'information, refaire le récap ; arrêter = fin de session.
- Au plus 3 tours de précision par phase ; ensuite, propose d'avancer avec les hypothèses listées.
<!-- /oh:when -->
<!-- oh:when role=subagent -->

## Sous-agent

- **Une seule session** : enchaîne tes phases sans t'arrêter ; les validations de phase deviennent des hypothèses documentées dans ton rapport.
- Arrête-toi seulement sur une décision qui bloque la suite (statut `en-attente`), quand ton prompt demande un arrêt à une étape (par exemple avant de créer des tickets), ou à la fin.
- **Relancé avec ton `task_id`** : applique la réponse et reprends à la phase où tu t'étais arrêté.
<!-- /oh:when -->
