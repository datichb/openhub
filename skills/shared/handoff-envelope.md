---
name: handoff-envelope
description: "Utiliser à la fin d'une tâche lancée par un agent : déposer ton rapport, puis rendre l'enveloppe de retour de 15 lignes."
when: role=subagent
---

# Enveloppe de retour

<!-- oh:when role=entry -->
**Seulement quand un agent t'a lancé** (marqueur `[CONTEXTE]` ou `[REVIEW:single]`, reprise avec `task_id`). Agent d'entrée : jamais d'enveloppe ni de « Retour vers » ; ta réponse finale est ton livrable complet, affiché à l'utilisateur, même déposé avec `workflow_report`.
<!-- /oh:when -->

1. Dépose ton rapport complet (sections de ton payload) avec `workflow_report` (`title`, `body`), **avant** l'enveloppe : il répond `rp-…`.
2. Ta réponse finale est l'enveloppe, en 15 lignes au plus ; elle commence par sa ligne de titre, rien au-dessus ni à part (ni récit de méthode ni récapitulatif : le détail va dans le rapport) :

```
## Retour vers <appelant>
**Agent** : <ton identifiant> · **Statut** : <statut>
**Rapport** : rp-<n>
### Questions bloquantes
Aucune.
### Suite proposée
<l'étape suivante pour ton appelant, 1 ou 2 lignes>
### Résumé
<3 lignes au plus : fait, reste, point d'attention>
task_id : <ton identifiant de session, si tu le connais>
```

- `<appelant>` : l'agent qui t'a lancé, tel que ton contrat le nomme (ligne « Retour ») ; jamais un autre nom.
- **Rapport non enregistré** (outil absent, refus, réponse « non enregistré ») : `**Rapport** : inclus`, puis, sous l'enveloppe (jamais au-dessus), `### Rapport complet` et le rapport entier.
- **Statut** : un statut de ton payload, ou `en-attente` quand une question bloquante t'arrête.
- **Questions bloquantes** : `Aucune.`, ou une liste numérotée (contexte en une ligne, 2 à 3 options, la recommandée en premier). Ce qui ne bloque pas devient une hypothèse documentée dans ton rapport. Avec une question, arrête-toi : ton appelant la pose et te relance avec ton `task_id` et la réponse ; reprends où tu t'étais arrêté.
- Le résumé désigne le rapport, il ne le remplace pas.
