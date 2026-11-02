> [Read in English](059-return-envelope-and-workflow-report.en.md)

# ADR-059 — Enveloppe de retour, `workflow_report` et skills filtrés par rôle

## Statut

Accepté — **remplace** l'[ADR-009](009-inter-agent-handoff-contracts.fr.md) (skills `*-handoff-format`) ; **fait évoluer** l'[ADR-015](015-concision-posture.fr.md) (postures de concision) et l'[ADR-016](016-execution-path-skills.fr.md) (skills `-standalone` / `-subagent`) ; s'appuie sur l'[ADR-042](042-checkpoints-headless-decisions.fr.md) (cartes de décision) et l'[ADR-043](043-session-bundle-deploy-removal.fr.md) (paquet de session)

## Date

2026-11-02

## Contexte

L'[ADR-009](009-inter-agent-handoff-contracts.fr.md) avait formalisé les retours entre agents en onze skills `*-handoff-format` (un par paire producteur-consommateur), injectés des deux côtés, avec un bloc `## Retour vers <consommateur>` qui contenait le rapport complet. Le coordinateur devait **retranscrire** ce bloc (skill `retranscription-coordinateur`, environ 2 000 tokens par coordinateur) avant de décider. À l'usage :

- **Le rapport passait par la bouche du coordinateur** : retranscription et résumés perdaient de l'information (corrections du reviewer, enrichissements du wiki), et coûtaient les tokens deux fois. Le rapport de modernisation d'octobre 2026 ([rapport](../../dev/rapport-modernisation-2026-10.md), § 7.2 et décisions C, § 12) en fait un défaut structurel et décide de remplacer les formats par une enveloppe, un petit payload par agent et `workflow_report`.
- **Producteur et consommateur étaient mélangés** dans les mêmes skills : les formats voyageaient chez les deux, et changer un champ obligeait à toucher deux frontmatters.
- **Le titre du retour nommait un agent écrit dans un skill**, alors que l'appelant réel dépend du workflow.
- **Deux parcours par agent** ([ADR-016](016-execution-path-skills.fr.md)) et **deux niveaux de concision** ([ADR-015](015-concision-posture.fr.md)) se chargeaient chacun dans un fichier séparé, sélectionnés par un marqueur `[SKILL:…]` que l'orchestrateur devait penser à injecter. Le mécanisme (`prompt-builder.sh`, `token_optimization.*` de `hub.json`) a disparu avec le déploiement par script ([ADR-043](043-session-bundle-deploy-removal.fr.md)).

oh, de son côté, sait désormais qui appelle qui (le graphe du workflow, [ADR-039](039-declarative-workflows-oh-v1.fr.md)) et garde un état par session (démon, [ADR-047](047-session-interaction-daemon.fr.md)).

## Décision

### 1. Une enveloppe courte, un rapport stocké par oh

- **Enveloppe** (`skills/shared/handoff-envelope.md`, pour un agent lancé par un autre) : 15 lignes au plus, qui commence par `## Retour vers <appelant>` : agent et statut, identifiant du rapport (`rp-<n>`), `### Questions bloquantes`, `### Suite proposée`, `### Résumé` (3 lignes), `task_id`. Le détail va dans le rapport, pas dans l'enveloppe.
- **Rapport** : l'agent le dépose en entier avec l'outil `workflow_report` (`title`, `body`) du serveur MCP `workflow`, **avant** l'enveloppe. oh le garde (`~/.oh/sessions/<session>/reports/`, métadonnées et empreinte en base) et répond `rp-<n>`. L'auteur est l'agent que oh voit appeler l'outil, jamais une valeur donnée par l'agent. Bornes : 64 Kio par rapport, 100 rapports et 4 Mio par session. `workflow_report_get` relit un rapport (ou une section, par titre Markdown), `workflow_reports` les liste.
- **Le coordinateur ne retranscrit plus** : il lit l'enveloppe, lit le rapport (ou une section) avec `workflow_report_get` quand il en a besoin, transmet l'**identifiant** (ou la section telle quelle, jamais un résumé de sa main) à l'agent suivant, et cite les rapports dans `reports` de `workflow_checkpoint`. La carte du checkpoint joint les rapports déposés depuis le précédent, la chronologie les montre, `oh session report` les donne à l'utilisateur ; en session distante, ils reviennent dans l'artefact `reports.json`.
- **Repli** : sans démon (ou `OH_WORKFLOW_REPORTS=off`), l'outil répond « non enregistré » sans erreur ; l'enveloppe porte alors `**Rapport** : inclus` et le rapport entier suit sous l'enveloppe.
- **Pas de `question` pour un sous-agent** : l'utilisateur ne voit pas ses appels. Le paquet lui retire l'outil (`QuestionGuard`) ; ses questions vont dans `### Questions bloquantes`, son appelant les pose et le relance avec le `task_id`.

### 2. Un payload par agent, un contrat généré

- Chaque agent qui rend une enveloppe a un petit skill `<agent>-payload` (200 à 400 tokens, borné par un test) : ses **statuts** (`statuses:` du frontmatter) et les **sections de son rapport**.
- Le **contrat de session** de chaque agent, généré par oh d'après le graphe, nomme ses appelants (le titre `## Retour vers <appelant>` vient de là), et pour un coordinateur la table des statuts de retour des agents qu'il peut lancer.

### 3. Les variantes par rôle sont filtrées à la construction

Au lieu de charger deux skills et d'injecter un marqueur, un skill ou un paragraphe porte `when: role=entry` ou `role=subagent` (`<!-- oh:when … -->`) et oh ne livre à chaque agent que ce qui s'applique à son rôle dans le workflow. Conséquences sur les décisions antérieures :

- les paires `-standalone` / `-subagent` ([ADR-016](016-execution-path-skills.fr.md)) deviennent un seul `execution-protocol` à une section par rôle ;
- les postures de concision ([ADR-015](015-concision-posture.fr.md)) deviennent `core-posture` (commune), `user-facing-posture` (agents d'entrée) et `coordinator-posture` (coordinateurs) ; le but de l'ADR-015 (aller droit au but sans perdre d'information) est gardé dans `core-posture`, sans niveaux `lite` ni `subagent` ni clé de configuration.

## Conséquences

### Positives

- Plus de retranscription : le coordinateur ne consomme que l'enveloppe, et les tokens des rapports ne sont payés qu'une fois.
- Rien n'est perdu entre l'agent et l'utilisateur : la carte de décision, la chronologie, `oh session report` et la reprise montrent le rapport original.
- Un seul format de retour au lieu de onze ; changer les sections d'un agent ne touche que son payload.
- Les agents n'ont plus à penser aux marqueurs `[SKILL:…]` : le rôle vient du workflow.

### Négatives / compromis

- **Dépendance à l'outil et au démon** : sans eux, le repli « inclus » redonne le rapport dans la réponse (l'ancien comportement, pour ce cas seulement).
- Les rapports sont limités (64 Kio, 100, 4 Mio) ; au-delà, l'outil refuse et l'agent inclut le rapport.
- Un rapport est conservé dans `~/.oh/sessions` : les agents ne le lisent ni ne l'écrivent que par les outils (`~/.oh/sessions` est refusé par les gardes de oh).
- Le comportement des agents dépend de prompts remaniés : seule `oh eval`, avec un vrai modèle, le mesure.

## Alternatives envisagées

| Alternative | Rejetée parce que |
|-------------|-------------------|
| Garder les `*-handoff-format` et les alléger | Le défaut est structurel : le rapport passe par le coordinateur, et le format est partagé entre producteur et consommateur |
| Rapport dans un fichier du projet | Pollue le dépôt, dépend du système de fichiers de l'agent, ne survit pas à une session distante |
| Rapport dans Beads (commentaire de ticket) | Tous les workflows n'ont pas de ticket, et la taille est limitée |
| Retirer l'enveloppe et ne rendre que l'identifiant | Le coordinateur n'aurait ni statut ni question bloquante pour décider du routage |
| Fichier d'état partagé entre agents | Déjà écarté par l'[ADR-009](009-inter-agent-handoff-contracts.fr.md) : couplage au système de fichiers, session non reproductible |
