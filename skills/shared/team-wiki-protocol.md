---
id: team-wiki-protocol
bucket: B
agent: documentarian
---

> **Condition préalable :** Si les outils `team_*` ne sont pas disponibles dans la session courante, ce protocole ne s'applique pas — continuer sans.

# Team Wiki Protocol

Ce skill définit les règles d'utilisation de `team_wiki_write` par le documentarian.
Seul le documentarian a le droit de proposer des entrées au wiki partagé cross-projet.

## Quand proposer une entrée wiki

Propose une entrée wiki partagé quand :

- Une **décision architecturale** impacte ou pourrait impacter plusieurs projets
- Un **pattern** récurrent a été identifié et documenté dans un projet spécifique
- Une **convention d'équipe** a été discutée et validée par l'utilisateur
- Un **retour d'expérience** (post-mortem, lesson learned) mérite d'être partagé

Ne PAS proposer quand :

- L'information est spécifique à un seul projet → utilise le wiki projet (`docs/wiki/`)
- L'information est éphémère ou temporaire
- Tu n'as pas de confirmation ou source fiable (confidence = UNCERTAIN sans contexte)

## Format des propositions

Chaque proposition via `team_wiki_write` doit :

1. **Cibler une page existante** quand possible (`team_wiki_list` pour vérifier)
2. **Utiliser le bon tag de confidence** :
   - `CONFIRMED` : information vérifiée, citant un fichier, un commit, ou une décision explicite de l'utilisateur
   - `INFERRED` : déduit de l'analyse multi-fichiers ou de patterns observés
   - `UNCERTAIN` : hypothèse à valider, basée sur des indices indirects
3. **Être concis** : max 200 lignes, aller droit au but
4. **Suivre le format markdown** avec headers appropriés

## Structure du contenu

```markdown
## [Titre de l'entrée]

**Confidence:** [CONFIRMED|INFERRED|UNCERTAIN]
**Source:** [projet d'origine, fichier/commit si applicable]
**Date:** [date de la proposition]

[Contenu de l'entrée]

### Contexte

[Pourquoi cette information est pertinente pour l'équipe]
```

## Pages standard du wiki

| Page | Contenu type |
|------|-------------|
| `decisions` | Décisions architecturales cross-projet |
| `patterns` | Patterns et conventions récurrentes |
| `lessons-learned` | Retours d'expérience, post-mortems |
| `onboarding` | Informations utiles pour les nouveaux membres |
| `conventions` | Conventions d'équipe (naming, branching, etc.) |

## Workflow

1. Identifie que l'information est cross-projet et pertinente
2. Demande confirmation à l'utilisateur : "J'ai identifié [X] qui pourrait enrichir le wiki d'équipe. Souhaites-tu que je propose une entrée ?"
3. Si oui : appelle `team_wiki_write` avec page, contenu, confidence, et projet
4. Le MCP crée une proposition en attente
5. L'équipe est notifiée sur Mattermost
6. Un membre valide via `oh team wiki review`

## Contraintes strictes

- **TOUJOURS demander confirmation** à l'utilisateur avant d'appeler `team_wiki_write`
- Ne jamais proposer plus de **2 entrées** par session
- Ne jamais proposer de contenu **sensible** (credentials, tokens, données personnelles)
- Respecter la limite de **200 lignes** par proposition
- Le contenu doit être **utile à toute l'équipe**, pas seulement au projet courant
