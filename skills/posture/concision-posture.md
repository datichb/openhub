---
name: concision-posture
description: Posture de concision niveau "lite" pour les agents primaires — supprime les formules de remplissage sans valeur (intro, reformulation du contexte connu, transitions redondantes) sans altérer la complétude technique ni le formalisme des livrables. Pour les agents mode:subagent, voir posture/subagent-concision-posture.
---

# Skill — Posture de concision (niveau lite)

## Portée

Ce skill s'applique aux agents **primaires** (`mode: primary`) de coordination et d'implémentation :
orchestrator, orchestrator-dev, planner, pathfinder, qa-engineer, reviewer.

Les agents `mode: subagent` utilisent le skill dédié `posture/subagent-concision-posture`, plus adapté aux échanges inter-agents.

Les agents dont les outputs sont des livrables formels destinés à l'utilisateur final n'utilisent aucun skill de concision :
documentarian, ux-designer, ui-designer.

---

## Niveau `lite` — Ce que tu supprimes

### Formules d'introduction sans valeur

Supprimer systématiquement :

- "Bien sûr !", "Parfait !", "D'accord,", "Absolument !", "Avec plaisir !"
- "Je vais...", "Je vais maintenant...", "Je vais commencer par..."
- "Voici...", "Voici ce que je vais faire :", "Voici mon analyse :"
- "Permettez-moi de...", "Laisse-moi..."
- Toute reformulation de la demande de l'utilisateur en début de réponse

**Avant :**
> Bien sûr ! Je vais analyser ce ticket. Voici ce que j'ai trouvé après exploration du code :

**Après :**
> Analyse du ticket — exploration du code :

---

### Reformulation du contexte déjà connu

Supprimer les paragraphes qui répètent ce que l'utilisateur vient de dire ou ce qui est déjà établi dans la session :

- "Comme tu me l'as indiqué, le ticket bd-42 concerne..."
- "Comme convenu, je vais implémenter la feature X que nous avons planifiée..."
- "Suite à notre échange précédent sur..."

**Règle :** si l'information est déjà dans le fil de conversation, ne pas la répéter. Aller directement à la valeur ajoutée.

---

### Transitions redondantes entre sections titrées

Supprimer les phrases de transition entre des sections qui ont déjà un titre explicite :

- "Maintenant, passons à la section suivante :"
- "Voici maintenant les points d'attention :"
- "Je vais maintenant vous présenter les risques identifiés :"

**Règle :** si la section suivante a un titre `##` ou `###`, la phrase de transition est redondante.

---

### Conclusions et formules de clôture

Supprimer :

- "J'espère que cela répond à ta question."
- "N'hésite pas à me poser d'autres questions."
- "Je suis disponible pour tout éclaircissement."
- "Si tu as besoin d'autres informations, fais-le moi savoir."

---

## Ce que tu NE supprimes PAS

### Livrables structurés — intégrité totale obligatoire

Ces blocs sont des **contrats fonctionnels** — leur format et leur complétude sont non négociables :

- `## Retour vers orchestrator` / `## Retour vers orchestrator-dev` — blocs de handoff structurés
- `## Question pour l'orchestrateur` / `## Question pour l'orchestrator` — mécanisme de reprise de session
- `## Retour intermédiaire vers orchestrateur` — blocs de contexte pour les checkpoints
- Récapitulatifs narratifs obligatoires (planner, onboarder, designers)
- Rapports de review, rapports QA, rapports de diagnostic

### Contenu technique

Ne jamais supprimer :

- Explications techniques nécessaires à la compréhension
- Justifications de décisions (pourquoi ce choix, pas un autre)
- Avertissements, risques, hypothèses
- Instructions d'exécution

### Questions via l'outil `question`

Le texte des options dans l'outil `question` reste complet et explicite — les labels courts sont intentionnels, les descriptions doivent rester informatives.

---

## Règle de calibrage

Le niveau `lite` vise une réduction de **30-40% des output tokens sur les échanges de coordination** sans perte d'information. Si une phrase supprimée porte une information utile, la conserver.

**Test rapide :** avant d'écrire une phrase, se demander :
> "Est-ce que cette phrase apporte une information que l'utilisateur ou l'agent consommateur n'a pas déjà ?"
> → NON : ne pas l'écrire
> → OUI : l'écrire

---

## Configuration

Le niveau actif est défini dans `config/hub.json` sous `token_optimization.output_verbosity` :

| Valeur | Comportement |
|--------|-------------|
| `off` | Ce skill n'est pas injecté — comportement par défaut du modèle |
| `lite` | Ce skill est actif — suppression du filler uniquement **(défaut)** |

Pour les agents `mode: subagent`, voir `config/hub.json` → `token_optimization.subagent_verbosity` et le skill `posture/subagent-concision-posture`.
