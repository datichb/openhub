> 🇬🇧 [Read in English](017-auditor-consolidation.en.md)

# ADR-017 — Consolidation des agents auditor-* en un agent auditor-subagent générique

## Statut

Accepté

## Contexte

Le hub disposait de 7 agents `mode: subagent` spécialisés par domaine d'audit :
`auditor-security`, `auditor-performance`, `auditor-accessibility`, `auditor-architecture`,
`auditor-privacy`, `auditor-ecodesign`, `auditor-observability`.

Cette architecture présentait plusieurs problèmes :

**Duplication structurelle (80% de code identique) :**
- Frontmatter identique sur les 7 agents (permissions, skills, sauf `id`, `label`, `description`, `native_skills`)
- Body identique à ~80% : même préambule, mêmes règles "Ce que tu fais / NE fais PAS", même workflow en 10 étapes, même pattern de renvoi au coordinateur
- La différence réelle se limitait au domaine, aux référentiels et au `native_skill` spécifique

**Problèmes de fiabilité détectés (analyse du 2026-06-18) :**
- **C-2** : conflit entre `posture/expert-posture` (qui prescrit d'appeler `question` pour les risques critiques) et `posture/subagent-concision-posture` (qui interdit `question` en mode subagent). Aucune règle de priorité. Présent dans les 7 agents.
- **C-5** : permission `task: { "auditor-*": allow }` dans `auditor.md` utilise un wildcard dont la sémantique n'est pas documentée dans OpenCode. Si non supporté, aucun sous-agent ne peut être invoqué.
- **m-3** : `subagent-concision-posture` listait les 7 agents dans sa portée — liste à maintenir manuellement à chaque ajout/suppression.

**Asymétrie avec le modèle developer :**
ADR-013 a déjà consolidé les developer-* spécialisés en un agent générique `developer`. Les auditors restaient sur l'ancienne architecture, créant une incohérence dans le modèle mental du hub.

## Décision

Reproduire exactement le modèle ADR-013 pour les auditors :

1. **Créer un seul agent `auditor-subagent`** (mode: subagent) qui remplace les 7 agents spécialisés
2. **Le coordinateur `auditor` injecte le domaine et le `native_skill`** dans le prompt d'invocation — c'est l'agent qui se spécialise selon ce qu'il reçoit, pas l'ID qui change
3. **Les 7 agents spécialisés sont supprimés**
4. **La permission `task: { "auditor-*": allow }` est remplacée** par `task: { "auditor-subagent": allow }` — ID explicite, pas de wildcard (résout C-5)

### Format d'invocation du coordinateur vers `auditor-subagent`

```
task({
  subagent_type: "auditor-subagent",
  prompt: "
    [contexte projet transmis]
    ...
    Tu agis en tant que sous-agent d'audit [DOMAINE].
    Charge et applique le skill : auditor/audit-[DOMAINE]
  "
})
```

### Table domaine → native_skill

| Domaine | Native skill |
|---------|-------------|
| `security` | `auditor/audit-security` |
| `performance` | `auditor/audit-performance` |
| `accessibility` | `auditor/audit-accessibility` |
| `ecodesign` | `auditor/audit-ecodesign` |
| `architecture` | `auditor/audit-architecture` |
| `privacy` | `auditor/audit-privacy` |
| `observability` | `auditor/audit-observability` |

### Résolution C-2 — Règle de priorité `expert-posture` vs `subagent-concision-posture`

La règle est inscrite dans le body de `auditor-subagent.md` :
> En mode subagent, les risques critiques remontent via le champ `risques` du bloc handoff. Ne jamais appeler `question` — l'outil n'est pas disponible dans ce contexte.

### Évolution de `cmd-audit.sh`

Le flag `--type` est conservé pour la compatibilité CLI mais son rôle change :
- **Avant** : sélecteur d'agent (`REQUIRED_AGENTS=("auditor" "auditor-security")`)
- **Après** : paramètre de prompt transmis au coordinateur (`REQUIRED_AGENTS=("auditor")` dans tous les cas)

## Conséquences

### Positives

- **-7 fichiers agents** : les 7 agents spécialisés sont supprimés. Un seul `auditor-subagent.md` à maintenir.
- **Zéro duplication** : frontmatter et body communs ne sont plus dupliqués 7 fois.
- **C-2 résolu** : règle de priorité explicite dans le body de l'agent unique.
- **C-5 résolu** : permission `auditor-subagent` explicite, pas de wildcard.
- **m-3 résolu** : `subagent-concision-posture` ne liste plus qu'un seul ID dans sa portée.
- **Cohérence avec ADR-013** : même architecture que le `developer` générique.
- **Maintenance facilitée** : toute règle commune (format handoff, règles de lecture seule, risques critiques) est modifiée en un seul endroit.

### Négatives / compromis

- **Perte des descriptions riches par domaine** dans le picker OpenCode. Les 7 agents avaient des descriptions ciblées (`"analyse OWASP Top 10, CVE..."`, `"analyse WCAG 2.1 AA..."`). `auditor-subagent` a une description générique. Impacte la lisibilité du picker `@` — mais les subagents `mode: subagent` n'apparaissent pas dans le picker utilisateur, uniquement dans les invocations `task` du coordinateur. Risque nul en pratique.
- **Le `--type` CLI change de sémantique** : il ne sélectionne plus un agent déployé, il paramètre un prompt. Les utilisateurs qui listaient manuellement leurs agents déployés (`auditor-security.md` dans `.opencode/agents/`) doivent déployer `auditor-subagent.md` à la place.

## Alternatives rejetées

**Conserver les 7 agents avec règles de priorité ajoutées** : résout C-2 mais oblige à modifier 7 fichiers identiques. Ne résout pas la duplication ni C-5. Rejeté.

**Skill unique multi-niveaux** (même approche que les niveaux `lite`/`subagent` de ADR-015) : réduirait le nombre de fichiers mais n'élimine pas la duplication des agents. Rejeté.

## Impact

| Fichier | Action |
|---------|--------|
| `agents/auditor/auditor-subagent.md` | Créé |
| `agents/auditor/auditor.md` | Modifié — table domaine→native_skill, permission `task: auditor-subagent`, prompt d'invocation |
| `agents/auditor/auditor-security.md` | Supprimé |
| `agents/auditor/auditor-performance.md` | Supprimé |
| `agents/auditor/auditor-accessibility.md` | Supprimé |
| `agents/auditor/auditor-architecture.md` | Supprimé |
| `agents/auditor/auditor-privacy.md` | Supprimé |
| `agents/auditor/auditor-ecodesign.md` | Supprimé |
| `agents/auditor/auditor-observability.md` | Supprimé |
| `skills/auditor/auditor-workflow.md` | Modifié — table sous-agents, prompt d'invocation Phase 3 |
| `skills/posture/subagent-concision-posture.md` | Modifié — portée : 7 auditors → `auditor-subagent` |
| `scripts/cmd-audit.sh` | Modifié — `REQUIRED_AGENTS` toujours `("auditor")`, pattern scan `auditor\|auditor-subagent` |
| `scripts/lib/agent-discovery.sh` | Modifié — `hub_ids` : 7 auditors → `auditor-subagent` |
| `scripts/lib/prompt-builder.sh` | Modifié — commentaire d'exemple |
| `tests/test_cmd_audit.bats` | Modifié — suppression fixtures `auditor-*`, ajout section B-bis |
| `tests/test_lib_agent_discovery.bats` | Modifié — ajout tests `auditor-subagent` match, 7 auditors sans match |
| `tests/test_lib_prompt_builder.bats` | Modifié — ajout tests `build_audit_bootstrap_prompt` |
| `docs/architecture/agents.fr.md` + `.en.md` | Modifié |
| `docs/architecture/skills.fr.md` + `.en.md` | Modifié |
| `docs/architecture/task-delegation.fr.md` | Modifié |
| `docs/guides/workflows.fr.md` + `.en.md` | Modifié |
| `docs/dev/fiabilisation-agents-en-cours.md` | Modifié — C-2, C-5, m-3 marqués résolus |
