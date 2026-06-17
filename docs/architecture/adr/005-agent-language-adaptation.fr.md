# ADR-005 — Adaptation linguistique des agents au projet cible

## Statut

Accepté

## Contexte

Le hub openhub est rédigé entièrement en français : agents, skills, documentation.
Cependant, les projets sur lesquels les agents sont déployés peuvent avoir des langues
de travail différentes (anglais, espagnol, etc.).

Actuellement, un agent déployé sur un projet anglophone va produire ses rapports,
ses comptes rendus et ses messages en français, ce qui crée une friction pour
les équipes non francophones.

La question est : comment permettre aux agents de s'adapter à la langue du projet
cible sans dupliquer tous les fichiers agents/skills pour chaque langue ?

## Décision

**Option A retenue** : injection d'une instruction de langue en tête de chaque agent
déployé, conditionnée à la présence d'un champ `Langue` dans `projects.md`.

L'instruction est injectée par le script de déploiement — les fichiers sources (agents,
skills) restent en français. Comportement par défaut (champ absent) : aucun changement,
les agents s'expriment en français.

## Implémentation

### 1. Champ optionnel dans `projects.md`

```markdown
## MON-APP
- Nom : Mon Application
- Stack : Vue 3 + Laravel
- Board Beads : MON-APP
- Tracker : jira
- Labels : feature, fix
- Langue : english        # optionnel — si absent : français par défaut
```

### 2. Lecture du champ via `common.sh`

Nouvelle fonction `get_project_language <PROJECT_ID>` :
- Lit le champ `- Langue :` dans `projects.md` pour le projet donné
- Retourne la valeur normalisée en minuscules, ou chaîne vide si absent

### 3. Injection dans `prompt-builder.sh`

La fonction `build_agent_content` accepte un 3e paramètre `$3` = langue (optionnel).
Si non vide, une instruction est insérée après le commentaire d'en-tête généré :

```markdown
> **Langue de travail : english.** Rédige toutes tes réponses, rapports et commentaires
> en english, quelle que soit la langue des instructions ci-dessous.
```

### 4. Passage de la langue dans les adapters

Les 2 adapters (`opencode.adapter.sh`)
lisent la langue du projet via `get_project_language "$PROJECT_ID"` et la passent
comme 3e argument à `build_agent_content`.

Si `PROJECT_ID` est vide (déploiement hub-level sans projet cible), aucune instruction
n'est injectée.

## Options rejetées

### Option B — Traduction des skills par l'adapter

Le déploiement (`oc deploy`) traduit automatiquement les fichiers skills via l'API
d'un LLM avant injection. Les agents déployés sont dans la langue du projet.

**Rejetée** : coût API à chaque déploiement, temps de déploiement allongé, maintenance
des traductions difficile à versionner et à auditer.

### Option C — Skills multi-langues

Les skills existent en plusieurs versions linguistiques :
`skills/developer/dev-standards-universal.fr.md`, `.en.md`, etc.
L'adapter sélectionne la version selon la langue du projet.

**Rejetée** : multiplication des fichiers (×N langues), maintenance lourde, risque
de divergence entre versions linguistiques.

## Conséquences

- La structure de `projects.md` gagne un champ optionnel `Langue`
- Le comportement par défaut est inchangé pour les projets existants (rétrocompatible)
- Les adapters et `prompt-builder.sh` gèrent un paramètre supplémentaire
- La fiabilité de l'instruction dépend du modèle IA utilisé — acceptable pour l'usage visé
