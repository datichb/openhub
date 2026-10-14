> [Read in English](incident-security-spike.en.md)

# Incident, correction de sécurité et spike

Trois workflows qui bouclent la boucle après un problème ou avant une décision : `incident` arrête l'impact puis capitalise, `security-fix` corrige les constats d'un audit, `spike` tranche une question technique. Chacun écrit ce qu'il apprend dans le wiki ou dans Beads, jamais à côté. Référence complète : [Workflows livrés](../reference/workflows.fr.md#incident).

## `incident` : atténuer d'abord

```bash
oh run incident -i incident="Depuis le déploiement de 14h02, les paiements échouent (429 du prestataire). Journal : ops/incident.log"
```

1. Le `debugger` mesure l'impact, date le début et cherche le changement déclencheur (`git log` autour de l'heure de début, diff du commit suspect).
2. Il propose la mesure la plus sûre : annuler ce changement, couper la fonction, changer un réglage. Si c'est du code, le `developer` l'applique **dans l'arbre de travail**, sans commit. Si c'est hors du dépôt (variable d'environnement, retour à la version précédente), il vous donne la commande exacte.
3. **`cp-mitigated`** : oh lance les tests, puis vous choisissez ou confirmez la mesure. Validé, le correctif de code (s'il y en a un) est commité aussitôt, seul : vous pouvez le livrer sans attendre le postmortem.
4. Le debugger écrit le postmortem, `docs/wiki/postmortems/<date>-<slug>.md`, **sans blâme** : aucun nom de personne, même si le signalement en cite un ; la chronologie, l'impact et le délai de détection sont chiffrés et sourcés.
5. **`cp-postmortem`** : vous validez ou demandez une correction ; ensuite seulement, le postmortem est commité, seul.
6. **`cp-actions`** : les actions du postmortem (prévenir, détecter, atténuer) deviennent des tickets si vous les validez ; un refus n'en crée aucun.

Trois pauses en semi-auto, chacune obligatoire : un incident ne se mène pas en automatique.

À la fin, oh affiche le postmortem et les tickets créés, et propose (sans les lancer) `ticket` sur ces tickets et `spec` avec le postmortem pour page d'origine, pour des actions qui demandent d'abord une spec.

## `security-fix` : corriger des constats

```bash
oh run audit -i type=security
# à la fin, oh propose : oh run security-fix --parent <session de l'audit>
oh run security-fix --parent <session de l'audit>
```

Lancé depuis un audit `security` (la suite que propose oh), `security-fix` lit le rapport de cet audit ; un extrait se donne en entrée (`-i findings="…"`).

1. **`cp-triage`** : les constats reçus sont numérotés (`F1`, `F2`…) ; vous dites lesquels corriger maintenant, et pourquoi vous écartez les autres (plus tard, risque accepté…).
2. Chaque constat écarté devient un ticket Beads (`-l security`), avec la raison du tri : rien n'est perdu. oh relève ces tickets.
3. Le `developer` corrige seulement les constats retenus, à la cause et pour toutes les entrées concernées, chacun avec un test de non-régression, sans commit. Un faux positif est signalé, sans changement. Il rend, constat par constat, la correction, le test et les fichiers modifiés.
4. L'`auditor-subagent` refait une analyse **ciblée** : chaque constat est `corrigé`, `encore présent` ou `contournable` (autre chemin, préfixe voisin, lien symbolique…).
5. **`cp-rescan`** (sans pause en semi-auto) : oh lance les tests et refuse un changement sans test ; un constat encore présent repart au developer (deux tours au plus).
6. **`cp-2`** : vous voyez les corrections, les tests et le verdict ; le commit n'a lieu qu'après.

## `spike` : trancher une question

```bash
oh run spike -i question="Peut-on remplacer csvlite par encoding/csv ? go si le relevé réel se lit sans prétraitement ; kill sinon. Durée : 30 min."
```

1. **`cp-question`** : le `pathfinder` reformule la question, les critères (ce qui ferait dire go ou kill) et la durée ; rien ne commence avant votre validation.
2. Recherche sourcée, critère par critère : le dépôt d'abord, puis la documentation officielle ; chaque fait cite son fichier ou son URL.
3. Si un critère ne se tranche qu'en exécutant du code, le `developer` écrit un **prototype jetable** dans `.oh-spike/`, le lance, relève la sortie et le supprime. Il n'est jamais commité.
4. La décision — **go**, **clarify** (il manque une décision ou une information de votre part) ou **kill** — est écrite dans `docs/wiki/decisions/<slug>.md`, avec ses sources.
5. **`cp-decision`** : oh vérifie que `.oh-spike/` a disparu ; vous validez ou corrigez la décision ; seule la décision est commitée.

oh relève la décision et son verdict. Après un **go**, il propose `spec` avec la décision pour page d'origine (`oh run spec --parent <session> -i source=docs/wiki/decisions/<slug>.md`) ; après clarify ou kill, rien.

## Ce qu'oh vérifie et verrouille

| Workflow | Avant le checkpoint | Vérifié par oh |
|---|---|---|
| `incident` | aucun commit avant `cp-mitigated` (puis un seul, le correctif), celui du postmortem après `cp-postmortem`, aucun ticket avant `cp-actions` ; le debugger n'écrit que `docs/wiki/postmortems/` | tests de la stack à `cp-mitigated` |
| `security-fix` | aucun ticket avant `cp-triage`, aucun commit avant `cp-2` ; le developer n'est lancé qu'après `cp-triage` | tests et au moins un fichier de test modifié, à `cp-rescan` et `cp-2` |
| `spike` | aucun commit avant `cp-decision` ; le developer n'est lancé qu'après `cp-question` ; le pathfinder n'écrit que `docs/wiki/decisions/` | `.oh-spike/` supprimé, à `cp-decision` |

Les cas d'évaluation S41 à S46 et S49 à S51 (`evals/incident`, `evals/security-fix`, `evals/spike`) vérifient ces comportements avec un vrai modèle.
