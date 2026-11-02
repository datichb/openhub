> [Read in English](058-primary-agent-called-is-all.en.md)

# ADR-058 — Un agent `primary` appelé par le workflow est livré en `mode: all`

## Statut

Accepté — complète l'[ADR-038](038-sessionspec-tool-adapters.fr.md) (modèle de session neutre et adaptateurs) et l'[ADR-039](039-declarative-workflows-oh-v1.fr.md) (workflows déclaratifs)

## Date

2026-11-02

## Contexte

Le paquet de session ([ADR-043](043-session-bundle-deploy-removal.fr.md)) livre chaque agent du workflow avec un `mode` (`primary`, `subagent`), tiré de `agents.<id>.mode` du workflow, sinon du frontmatter de l'agent. opencode **refuse de lancer par `task` un agent `primary`** (« Agent X cannot run as a subagent »).

Le défaut est apparu dans une session réelle du workflow `review` : le `reviewer` (`mode: primary` dans son frontmatter) se délègue à lui-même pour les modes combinés (`standard+adversarial`, `all`), qui lancent des sessions indépendantes en parallèle. opencode refusait l'auto-délégation. Les modes combinés ne lançaient donc jamais de session parallèle, et `oh workflow validate` répondait « valide ». Le même cas existait pour `documentarian` (`primary` dans son frontmatter), lancé par le `developer` dans `ticket` et `implement-spec` d'après la permission `task`.

Corriger à la main chaque workflow n'aurait pas tenu : le graphe de délégation d'un workflow se compose par `extends` et par couches (hub, équipe, projet), et un agent change de rôle d'un workflow à l'autre (`developer` est `primary` dans `quick`, appelé dans `ticket`).

## Décision

**Nous avons décidé que le mode livré se calcule à partir du graphe de délégation du workflow, pas seulement du mode écrit.**

- `workflow.Spec.ResolveMode(id, base, graph)` donne le mode livré : `agents.<id>.mode` si le workflow en écrit un, sinon `base` (le frontmatter de l'agent, `primary` à défaut). **Un agent `primary`, écrit ou non, que le graphe laisse lancer à un membre du workflow, lui-même compris, devient `all`.** Les agents `subagent` et `all` et ceux que personne n'appelle gardent leur mode.
- Le graphe est celui de `DelegationGraph` : les `calls:` du workflow, ou à défaut la permission `task` de l'agent restreinte au workflow. Une auto-délégation n'y figure que si `calls` nomme l'agent lui-même (c'est ainsi que `review` déclare `calls: [reviewer]`). `bundle` applique `ResolveMode` à la construction du paquet (`workflowgen.go`, `applyWorkflowModes`) : la carte du workflow générée et le fichier de l'agent disent la même chose.
- `all` est une valeur valide de `agents.<id>.mode` (`AgentMode.Valid`, proposée par l'éditeur de workflow de la TUI) : visible dans le sélecteur d'opencode et lançable par `task`. Elle n'existe pas dans le frontmatter d'un agent.
- **Avertissement `calls_target_primary`** (`oh workflow validate`) : un workflow qui écrit `mode: primary` sur un agent qu'il fait appeler est averti — le paquet le livre en `all` quand même ; il propose d'écrire `mode: all` pour le dire, ou `mode: subagent` si l'utilisateur ne le choisit jamais. Un agent `primary` du catalogue qui est appelé sans que le workflow écrive son mode n'est pas signalé : il est livré en `all`, sans surprise.
- opencode 2.0.20 accepte `all` (vérifié dans le binaire). L'agent d'entrée reste `primary` (`entry_not_primary`).

## Conséquences

### Positives

- L'auto-délégation du `reviewer` fonctionne : les modes combinés lancent leurs sessions parallèles.
- Aucun workflow à réécrire : le graphe suffit, y compris pour un workflow d'équipe ou de projet qui étend un workflow du hub.
- Le mode livré est le même dans la carte du workflow et dans le fichier de l'agent.

### Négatives / compromis

- Un agent peut être `primary` dans un workflow et `all` dans un autre : le mode effectif dépend du workflow (priorité : workflow > frontmatter > `primary`).
- Un agent `all` apparaît dans le sélecteur d'opencode alors qu'il est avant tout un sous-agent du workflow ; c'est le prix de la contrainte d'opencode.
- 18 fichiers de référence (goldens) de paquets ont changé, pour le seul `mode` : `reviewer` en `review`, `documentarian` en `ticket` et `implement-spec`, et les autres agents `primary` appelés.
- La règle vient d'une contrainte d'opencode ; elle est calculée dans le modèle neutre du workflow (`workflow`, `bundle`), pas dans l'adaptateur : le paquet décrit le mode, l'adaptateur le rend tel quel.

## Alternatives envisagées

| Alternative | Rejetée parce que |
|-------------|-------------------|
| Écrire `mode: all` à la main dans chaque workflow | Les workflows se composent par couches et `extends` : un agent appelé par un workflow d'équipe resterait `primary`, et l'erreur n'apparaîtrait qu'à l'exécution |
| Passer l'agent en `subagent` | L'utilisateur ne pourrait plus le choisir quand il est l'agent d'entrée d'un autre workflow |
| Faire échouer `validate` sur un `primary` appelé | Le paquet sait corriger seul ; bloquer ne servirait qu'à obliger une réécriture sans bénéfice |
| Ne pas déléguer à soi-même (une session par mode dans le prompt) | Perd l'isolation de contexte des sessions parallèles, qui est le but des modes combinés |
