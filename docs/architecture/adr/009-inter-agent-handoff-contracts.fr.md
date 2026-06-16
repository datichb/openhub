> 🇬🇧 [Read in English](009-inter-agent-handoff-contracts.en.md)

# ADR-009 — Formalisation des contrats de communication inter-agents comme skills dédiés

## Statut

Accepté

## Contexte

L'architecture multi-agents du hub repose sur une chaîne d'orchestration où les agents invoquent des sous-agents via l'outil `Task` et exploitent leurs résultats pour piloter les checkpoints de décision. Cette chaîne couvrait deux niveaux :

**Niveau 1 — orchestrator-dev → orchestrator :** déjà formalisé en v1.3.0 via `orchestrator/orchestrator-handoff-format`. Ce skill définissait les blocs `## Retour vers orchestrator` et `## Question pour l'orchestrator`, partagés entre producteur et consommateur.

**Niveau 2 — tous les autres sous-agents → leurs consommateurs respectifs :** non formalisé. Les sous-agents (developer-*, reviewer, qa-engineer, planner, onboarder, debugger, ux-designer, ui-designer, auditor-*) retournaient des résultats en texte libre. Les agents consommateurs (orchestrator-dev, orchestrator) devaient extraire manuellement les informations de ces sorties non structurées, causant :

- Des récapitulatifs incomplets : le récap global d'orchestrator-dev était mal rempli car il manquait de données structurées provenant de ses sous-agents
- Un routing inconsistant : la décision de router vers `developer-security` après une review de sécurité nécessitait une analyse manuelle du texte du rapport, plutôt que la lecture d'un champ `### Routing recommandé`
- Des corrections verbatim perdues : les commentaires Beads contenaient des résumés manuels des corrections du reviewer, et non le libellé exact requis pour que le developer puisse agir dessus
- Une review sans contexte : le reviewer ne recevait aucune information structurée sur les zones que le developer considérait fragiles ou techniquement risquées
- Des checkpoints incomplets : les CP-spec et CP-audit au niveau de l'orchestrator étaient construits à partir du texte libre retourné par les agents design et audit, sans garantie d'exhaustivité

Le problème se manifestait concrètement dans le récap global d'orchestrator-dev : la section `### Points d'attention` était systématiquement vide ou superficielle, car l'information existait dans les sorties des sous-agents mais n'était pas suffisamment structurée pour être agrégée de façon fiable.

## Décision

Formaliser **tous** les contrats de communication inter-agents comme skills dédiés, suivant le même pattern établi par `orchestrator/orchestrator-handoff-format` :

1. **Un skill par paire producteur-consommateur** (ou par famille d'agents), injecté à la fois dans l'agent producteur et dans l'agent consommateur — garantissant un contrat partagé sans risque de désynchronisation.

2. **Bloc `## Retour vers <consommateur>` normalisé** pour chaque sous-agent, produit en fin de session quand invoqué depuis son parent. Contient : la sortie complète (jamais résumée), des champs de métadonnées actionnables (statut, routing, verdict), et des informations structurées prêtes à être transmises à l'étape suivante de la chaîne.

3. **L'agent consommateur est responsable de détecter la présence du bloc** et de le demander explicitement au producteur si absent — il ne construit jamais un checkpoint à partir d'une entrée incomplète ou non structurée.

4. **Skills créés :**

| Skill | Producteur | Consommateur | Champs clés |
|-------|-----------|-------------|-------------|
| `developer/developer-handoff-format` | developer-* | orchestrator-dev | Fichiers modifiés, critères cochés, **points d'attention pour la review**, statut |
| `reviewer/reviewer-handoff-format` | reviewer | orchestrator-dev | **Verdict actionnable** (commit/corriger/corriger-sécurité), corrections verbatim, **routing recommandé** |
| `qa/qa-handoff-format` | qa-engineer | orchestrator-dev | Tests écrits, critères cochés, zones non testables |
| `auditor/audit-handoff-format` | auditor-* | orchestrator | Tableau des vulnérabilités, recommandations priorisées, risque résiduel |
| `design/design-handoff-format` | ux-designer, ui-designer | orchestrator | Spec complète, **contraintes d'implémentation**, points ouverts |
| `planning/planner-handoff-format` | planner | orchestrator | Tableau complet des tickets avec agents prévus et dépendances |
| `planning/onboarder-handoff-format` | onboarder | orchestrator | Stack, conventions, dette technique, zones d'incertitude |
| `quality/debugger-handoff-format` | debugger | orchestrator | Cause racine avec niveau de certitude, impact, actions d'urgence |

5. **Exploitation en cascade :** les champs de chaque bloc sont explicitement utilisés dans le protocole du consommateur :
   - Les `### Points d'attention pour la review` du developer sont transmis verbatim au reviewer
   - Les `### Corrections requises` du reviewer sont copiées verbatim dans le commentaire Beads (pas de résumé manuel)
   - Le `### Routing recommandé` du reviewer détermine si l'on va vers `developer-security` ou l'agent initial
   - Les critères non couverts du QA sont transmis au reviewer
   - Le récap global d'orchestrator-dev agrège les points d'attention de toute la chaîne

## Conséquences

### Positives

- **Récapitulatifs complets :** le récap global d'orchestrator-dev est maintenant alimenté par des données structurées de tous les sous-agents — la section `### Points d'attention` est peuplée depuis les données reviewer, QA et developer.
- **Routing déterministe :** la décision de router vers `developer-security` est basée sur le champ `### Routing recommandé`, pas sur une analyse manuelle du texte de review.
- **Corrections fiables :** les commentaires Beads contiennent le libellé exact du reviewer, prêt à être appliqué par le developer — aucune perte d'information par résumé manuel.
- **Review informée :** le reviewer reçoit les points d'attention du developer pour chaque ticket, lui permettant de concentrer son analyse sur les zones sensibles.
- **Checkpoints fiables :** les CP-spec, CP-audit, CP-onboard sont construits à partir de blocs structurés avec champs obligatoires — un bloc absent ou incomplet déclenche une demande explicite avant de continuer.
- **Zéro désynchronisation :** en injectant le même skill dans producteur et consommateur, tout changement de format se propage automatiquement aux deux côtés.

---

## Amendement — Récap d'implémentation condensé dans le fil de l'orchestrateur

**Contexte :** après le déploiement initial, un manque a été identifié à la frontière `orchestrator-dev` → `orchestrator` : le bloc structuré `## Retour vers orchestrator` ne contenait qu'un résumé minimal (tickets traités, points d'attention, statut global). L'orchestrateur n'avait aucune visibilité sur le détail de l'implémentation (fichiers modifiés, cycles de review, couverture des critères d'acceptance) avant de présenter le [CP-feature] à l'utilisateur.

**Décision :** étendre les skills `orchestrator/orchestrator-handoff-format` et `orchestrator-dev-protocol` pour formaliser un **retour en deux étapes obligatoires et complémentaires** :

1. `orchestrator-dev` doit émettre une **synthèse structurée par ticket** (statut, fichiers clés, critères couverts, points d'attention + points d'attention globaux agrégés) **avant** le bloc structuré `## Retour vers orchestrator`. Ce récap apporte le **contexte condensé** que le bloc structuré ne contient pas, sans reproduire verbatim les comptes rendus narratifs des developer-* (trop verbeux pour N tickets).
2. Le bloc structuré `## Retour vers orchestrator` contient le tableau de détail par ticket et les statistiques — données actionnables non présentes dans le récap.
3. La règle consommateur de l'`orchestrator` est mise à jour : il doit **afficher ce récap dans son fil de discussion** avant de construire le [CP-feature] — symétrique avec l'affichage du rapport de review avant le [CP-2].

**Impact :** le fil de discussion de l'orchestrateur affiche une synthèse d'implémentation concise avant chaque [CP-feature], donnant à l'utilisateur une visibilité ciblée sur ce qui a été réalisé sans surcharger le fil avec N comptes rendus narratifs complets.

---

## Amendement — Déduplication des règles consommateur dans les handoff-formats

**Contexte :** chaque skill `*-handoff-format` contenait une section "Règles pour le consommateur" de 30-40 lignes reproduisant la séquence obligatoire d'affichage (afficher rapport → afficher bloc structuré → appeler `question`) déjà définie dans `posture/retranscription-coordinateur`. Cette duplication représentait ~1 200 tokens injectés en Bucket A pour une information déjà présente dans un skill dédié injecté dans les mêmes agents. Le skill `orchestrator-protocol` contenait également 5 templates de retranscription quasi-identiques (planner, onboarder, debugger, design, audit) qui reproduisaient la même structure générique que `retranscription-coordinateur`.

**Décision :** réduire les 5 sections consommateur dans les handoff-formats (planner, onboarder, auditor, debugger, design) à leur contenu **spécifique à ce type de retour** — champs obligatoires à vérifier, conditions de statut, actions de routing particulières — accompagné d'une référence explicite vers `posture/retranscription-coordinateur` pour le protocole commun. Simultanément, les 5 templates génériques dans `orchestrator-protocol` sont remplacés par une ligne de référence + un bloc contextuel compact listant uniquement les spécificités (sections critiques, actions prioritaires).

**Impact :** ~1 400 tokens retirés du Bucket A des agents orchestrator et orchestrator-dev sans perte d'information — le protocole de retranscription complet reste dans `posture/retranscription-coordinateur` qui est injecté en Bucket A dans les deux agents.

| Fichier modifié | Réduction |
|----------------|-----------|
| `skills/planning/planner-handoff-format.md` | Section consommateur : 41 lignes → 8 lignes |
| `skills/planning/onboarder-handoff-format.md` | Section consommateur : 39 lignes → 7 lignes |
| `skills/auditor/audit-handoff-format.md` | Section consommateur : 33 lignes → 6 lignes |
| `skills/quality/debugger-handoff-format.md` | Section consommateur : 37 lignes → 7 lignes |
| `skills/design/design-handoff-format.md` | Section consommateur : 32 lignes → 6 lignes |
| `skills/orchestrator/orchestrator-protocol.md` | 5 templates → 5 blocs contextuels compacts (~8 lignes chacun) |

- **Plus de skills injectés :** les agents reçoivent désormais davantage de skills, augmentant la taille des fichiers agents assemblés au déploiement. Ce surcoût est acceptable étant donné que les skills sont injectés une seule fois au déploiement et non à l'inférence.
- **Contrat strict :** les sous-agents qui ne produisent pas le bloc attendu déclenchent une demande de relance depuis le consommateur. C'est un comportement intentionnel — un résultat incomplet doit être signalé explicitement plutôt qu'ignoré silencieusement.
- **Obligation de double injection :** l'ajout d'un nouveau skill de handoff nécessite de mettre à jour deux frontmatters (producteur + consommateur). Cette règle est documentée dans la checklist du guide de contribution.

## Alternatives rejetées

**Parsing structuré du texte libre au niveau du consommateur :** instruire orchestrator-dev et orchestrator pour analyser le texte des sous-agents et en extraire les informations requises. Rejeté car cela crée une dépendance au libellé exact des sous-agents, est fragile dans le temps, et ne garantit pas l'exhaustivité — le parser peut manquer une mention ou mal interpréter une formulation.

**Fichier d'état partagé entre agents :** stocker les résultats dans un fichier JSON dans `.beads/` ou un emplacement similaire, que chaque agent lit et écrit. Rejeté car cela crée un couplage fort avec le système de fichiers, rend le workflow non reproductible entre sessions, et contredit le principe d'architecture stateless du hub.

**Extension du `orchestrator-handoff-format` existant :** ajouter tous les nouveaux formats dans le skill unique existant. Rejeté car le skill deviendrait un monolithe tentaculaire couvrant 10 paires d'agents différentes — difficile à maintenir et à comprendre. L'approche un-skill-par-domaine est plus cohérente avec l'organisation du hub.
