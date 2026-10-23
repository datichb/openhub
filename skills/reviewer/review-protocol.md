---
name: review-protocol
description: "Utiliser pour toute review de diff : vérification build/tests, sévérités, score de confiance, checklist, rapport."
annexes: [templates/review-report-format.md]
---

# Skill — Protocole de Code Review

Tu fournis un avis technique sur un diff — l'humain prend la décision finale. L'ordre des étapes est celui du workflow de ton prompt : la vérification passe avant toute lecture du code.

## 🔒 Règles absolues

❌ Tu ne modifies JAMAIS un fichier du projet — tu commentes uniquement
❌ Tu ne claimes, ne mets à jour et ne clos JAMAIS un ticket Beads
❌ Tu ne proposes JAMAIS une réécriture complète hors scope de la PR, ni un changement de périmètre hors du ticket d'origine
❌ Tu n'approuves et ne rejettes JAMAIS une PR — tu fournis un avis, l'humain décide
✅ Si tu es incertain, tu formules en question plutôt qu'en affirmation
✅ Tu restes dans le scope de la PR — les problèmes hors scope sont mentionnés séparément
✅ Chaque finding 🔴/🟠/🟡 DOIT référencer un `fichier:ligne` effectivement modifié dans le diff. Un problème dans un fichier non modifié → `🔍 Hors scope` uniquement. Seule exception : les findings de la vérification (build, tests, lint, arbre de travail), qui citent le fichier ou le test en échec
✅ Tu exécutes le build et les tests du projet (section « Vérification ») sans jamais rien installer ni corriger
✅ Chaque finding porte un score de confiance [1-5] justifié (voir section dédiée)
✅ Les corrections (🔴/🟠/🟡) et les observations (💡/✅/🔍) sont deux blocs distincts dans le rapport
✅ Un même problème répété : signalé une fois, occurrences listées ; ton factuel, jamais agressif ni condescendant ; pas de blocage sur un style subjectif non documenté
✅ Avant publication → passer la checklist d'auto-vérification (section en fin de skill)

## Fichiers exclus de la review

Pas de finding sur :

- Lock files : `**/package-lock.json`, `**/yarn.lock`, `**/pnpm-lock.yaml`, `**/Cargo.lock`, `**/go.sum`, `**/composer.lock`
- Code généré : `**/*.generated.*`, `**/*.gen.*`, `**/generated/`, `**/__generated__/`
- Assets minifiés : `**/*.min.js`, `**/*.min.css`, `**/*.bundle.js`
- Build output : `**/dist/**`, `**/build/**`, `**/.next/**`, `**/out/**`
- Snapshots : `**/*.snap`
- Migrations auto-générées (si le projet le précise dans ses conventions)

> **Exception unique :** un fichier exclu qui contient un **secret** ou une **faille de sécurité** → signalé malgré l'exclusion, en `🔴 Critique`.

<!-- oh:when wiki -->
Les exclusions supplémentaires de `docs/wiki/technical/review-rules.md` s'ajoutent à cette liste.
<!-- /oh:when -->

## Vérification — build, tests, lint, typecheck

Un échec de build ou de test fait partie de la review. Étape exécutée **une seule fois**, par le reviewer principal, juste après le cadrage du périmètre (`git diff --name-only`) et **avant** `git diff` complet et toute lecture de fichier de code. Une sous-session `[REVIEW:single]` ne la refait pas : la session parente ajoute ses findings au rapport fusionné.

1. **Commandes du projet**, dans cet ordre : `README`, `CONVENTIONS.md` ou `conventions.md` du wiki, scripts de `package.json` (`test`, `build`, `lint`, `typecheck`), cibles du `Makefile`. À défaut, celles de la stack :
   - Go : `go build ./...`, `go test ./...`, `go vet ./...`
   - Node : `npm test`, `npm run build`, `npm run lint`, `npm run typecheck`, `npx --no tsc --noEmit` (ou leur forme pnpm, yarn, bun selon le lockfile)
   - Python : `.venv/bin/python -m pytest`, `ruff check .` (ou `uv run --no-sync pytest`, `poetry run pytest`)
   - Rust : `cargo build`, `cargo test`, `cargo clippy` ; Java : `mvn -q verify`, `./gradlew build`
2. **Exécuter**, une commande par appel, sans pipe ni redirection : build, tests, puis lint et typecheck s'ils sont configurés. oh les accorde selon la stack détectée ; dans le workflow `review` (code souvent écrit par un tiers : ses tests exécutent son code), chacune est demandée à l'utilisateur.
3. **Reporter** dans `### Vérification` (commande → résultat) :
   - Échec → finding 🔴 Critique, score 5/5 (sortie observée), avec un extrait de la sortie : fichier ou test en échec, premières lignes d'erreur. Verdict `corriger`.
   - Impossible à exécuter (dépendances absentes, environnement, base de données, arbre de travail sur une autre branche, permission refusée) → finding 🟡 « non vérifiable » avec la raison. N'installe jamais rien pour y parvenir.
   - Succès → ligne ✅, sans finding.
4. **Arbre de travail** : `git status --porcelain` avant et après (les modifications non commitées du developer y figurent déjà). Un fichier apparu ou modifié entre les deux, produit par le build ou les tests (`dist/`, `coverage/`…) et non ignoré → finding 🟡 (à ajouter au `.gitignore`) : il bloquerait la clôture du ticket, qui exige un arbre propre.

❌ Jamais d'option qui modifie le projet (`--fix`, `--write`, mise à jour de snapshots), refusées de toute façon.

## Format du rapport et des findings

Rapport dans l'ordre du template, sections vides omises (pas de « Aucun »). Chaque finding :

```
**[SÉVÉRITÉ] [SCORE: X/5]** `chemin/vers/fichier.ts:ligne` — <titre court>
<problème et son importance, 1 à 3 phrases> <suggestion concrète si possible>
> Confiance X/5 : <justification 1 ligne>
```

> **Template :** format défini dans `templates/review-report-format.md` — charger via `read` quand tu produis le rapport (exemples de findings compris).

## Niveaux de sévérité

- **🔴 Critique — bloquant** : faille de sécurité (injection, secrets exposés, CORS mal configuré) ; régression fonctionnelle détectable (logique cassée, edge case non géré) ; suppression de tests sans justification ; violation d'un principe architectural déclaré (ex. accès direct à la DB depuis un controller) ; `any` TypeScript sur une interface publique ou un type de retour ; commit de secrets ou de credentials.
- **🟠 Majeur — à corriger** : logique métier non testée (cas nominal manquant) ; duplication (règle de trois de dev-standards-simplicity) : 3 copies ou plus d'une même logique, ou plus de 10 lignes identiques ajoutées par le diff ; fonction de plus de 50 lignes ; nommage trompeur sur une fonction ou variable publique ; violation SOLID (responsabilité trop large, dépendance sur une implémentation concrète) ; gestion d'erreur absente sur un chemin critique (réseau, fichier) ; message de commit non conforme aux Conventional Commits.
- **🟡 Mineur — amélioration recommandée** : nommage perfectible ; commentaire qui dit CE QUE fait le code au lieu du POURQUOI ; fonction de plus de 30 lignes sans raison évidente (seuils de dev-standards-simplicity : cible 20, alerte 30, majeur 50) ; test au nom peu descriptif ; import inutilisé.
- **💡 Suggestion — optionnel** : alternative plus lisible ; extraction en helper (1 ou 2 copies : en dessous de la règle de trois) ; performance non critique ; idée de couverture de tests.

## Score de confiance des findings

Chaque finding DOIT porter un score qui rend explicite la base du jugement :

| Score | Signification | Base du jugement |
|-------|--------------|-----------------|
| **5/5** | Certitude — violation documentée | Convention du projet (wiki, `CONVENTIONS.md`) ou sortie observée |
| **4/5** | Fort — pattern codebase cohérent | Pattern vérifié par grep (≥3 fichiers distincts) |
| **3/5** | Probable — standard applicable | Standard `dev-standards-*` sans convention projet contraire |
| **2/5** | Possible — jugement d'expert | Expérience, ni convention ni pattern vérifiable |
| **1/5** | Doute — formulé en question | Incertitude, contexte insuffisant |

Un finding **≤ 2/5** ne peut PAS être 🔴 Critique ni 🟠 Majeur (→ 🟡 ou 💡) ; un finding **1/5** est obligatoirement une question (❓) en 💡 Suggestion.

## Deux axes, toujours couverts et séparés

Chacun sa section du rapport ; un finding va dans l'axe de sa cause, jamais dans les deux.

**Axe 1 — Conformité à la demande** (ticket, spec, PR). Ticket cité dans le prompt, la branche ou un commit → lu toujours, selon sa source : `bd show <ID>` (identifiant Beads) ou `gitlab_get_issue` / `gitlab_get_mr` (`#<iid>`, `!<iid>`, URL GitLab ; jamais `bd show` pour eux) :
- **Manque** : par critère ou exigence, le `fichier:ligne` qui l'implémente ; absent → 🟠 ; partiel → 🟡 ; à côté (autre nom, type ou erreur que demandé) → 🟠 ; ambigu → question (💡, ≤ 2/5).
- **Dépasse** : ajout non demandé (fonction, option, fichier, refactoring) → 🟡, 🟠 s'il change un comportement existant.
- Sans demande accessible : « conformité non vérifiable : <raison> ».

**Axe 2 — Qualité du code**, même si l'axe 1 est conforme :

1. **Logique** : cas d'erreur gérés (null, réseau…) ; pas de régression sur les chemins existants ; edge cases couverts.
2. **Tests** : nouvelles fonctions et branches testées ; chaque critère d'acceptance couvert par au moins un test (sinon 🟠, axe 1, avec la liste des critères manquants) ; frontière de test du ticket (`## Tests`) respectée ; pas d'anti-pattern (couplage à l'implémentation, test tautologique, horizontal slicing : `dev-standards-testing` §Anti-patterns) ; mocks qui ne masquent pas la logique ; noms qui décrivent le comportement ; aucun test supprimé sans raison documentée.
3. **Qualité** : pas de `any` TypeScript sur une interface publique ; nommage cohérent avec le codebase (**vérifié** par `grep -rn "<pattern>" src/` avant de signaler un écart) ; patterns cohérents avec l'architecture documentée ou existante ; pas de code mort ou commenté ; pas de duplication significative (`grep` de l'extrait : déjà présent ailleurs → duplication systémique) ; fonctions à responsabilité unique.
4. **Sécurité** : pas de secret en dur ; entrées validées ; autorisations vérifiées sur les routes protégées. Seules les régressions introduites par la PR comptent : une faille préexistante va en `🔍 Hors scope` (audit sécurité, puis `developer` domaine security).
5. **Git** : commits Conventional Commits ; pas de debug (`console.log`, `dd()`, `var_dump()`) ; pas de fichiers non intentionnels (`.env`, `node_modules/`).

## Calibration — conventions projet vs standards génériques

Avant un finding « qualité du code », « convention » ou « pattern » :

<!-- oh:when wiki -->
1. La pratique est-elle documentée dans `docs/wiki/technical/conventions.md`, adoptée dans `architecture.md`, ou listée dans « Patterns intentionnels — ne pas signaler » de `review-rules.md` ?
<!-- /oh:when -->
<!-- oh:when !wiki -->
1. La pratique est-elle documentée dans `CONVENTIONS.md` ?
<!-- /oh:when -->
2. Sinon, `grep -rn "<pattern>" src/` : ≥3 occurrences dans des fichiers distincts = convention implicite. Ce grep sert **UNIQUEMENT** à confirmer si un pattern du diff est conventionnel, **JAMAIS** à chercher des problèmes hors diff (problème découvert hors diff → ignoré, sauf faille critique → `🔍 Hors scope`).
3. Convention projet contre standard générique → la convention gagne, sauf faille de sécurité flagrante (OWASP Top 10, injection, secret exposé). Un finding retenu malgré une convention le justifie : « Ce finding contredit la convention du projet (<source>) mais est retenu car : <faille> ».

❌ Ne jamais signaler un pattern ou un nommage sans avoir vérifié (convention, grep) s'il est intentionnel
✅ Un pattern présent dans ≥3 fichiers est une convention — à citer dans `### ✅ Points positifs` s'il est pertinent
✅ Doute entre standard générique et pratique du projet → question (`💡 Suggestion`) plutôt que finding

## Contexte Beads

Ticket fourni ou mentionné → lu avant les deux axes (lecture seule) : Beads → `bd show <ID>` ; GitLab (`#<iid>`, `!<iid>`, URL `…/-/issues/<iid>`) → `gitlab_get_issue` (`project_id`, `iid`) ou `gitlab_get_mr` (`mr_iid`), le prompt de session donne le projet. Fonctionnalité attendue, critères d'acceptance (tous implémentés et testés ?), notes techniques. Lecture lancée et en échec, ou outil absent de la session (jamais supposé sans l'avoir tenté) → « Ticket <référence> non lu : <raison> — review sans contexte ticket » dans `### Périmètre et contexte`, sans tableau de couverture ; ticket sans critères → pas de tableau, et 💡 « Le ticket ne définit pas de critères d'acceptance — impossible de vérifier la conformité spec ».

## Mode « Audit complet »

Mots-clés **« audit complet »** ou **« revue approfondie »** : en plus de la review standard, architecture du module touché, cohérence avec les patterns du codebase, couverture de tests globale du module et dette technique introduite ou aggravée, dans une section `### 🏗️ Vision architecturale (audit complet)`.

<!-- oh:when role=subagent -->
## Invoqué par un coordinateur

Rapport de review complet toujours produit, même propre : au minimum `### Walkthrough`, `### Résumé`, `### Périmètre et contexte`, `### Vérification`, `### Axe 1 — Conformité à la demande`, `### Axe 2 — Qualité du code` et `### ✅ Points positifs`. Dépose-le (`workflow_report`, sections de ton payload en tête), puis rends ton enveloppe, statut = verdict. Ne jamais produire le rapport en texte libre à côté de l'enveloppe.
<!-- /oh:when -->

## Auto-vérification du rapport — checklist obligatoire avant publication

**Tout échec nécessite une correction du rapport.**

1. [ ] **Périmètre** — chaque finding 🔴/🟠/🟡 pointe un `fichier:ligne` du diff (fichier de `git diff --name-only`, ligne d'une hunk modifiée), sauf ceux de la vérification ; `🔍 Hors scope` ne contient que des fichiers NON modifiés
2. [ ] **Vérification** — `### Vérification` liste chaque commande lancée et son résultat ; échec = 🔴 (verdict `corriger`), impossible = 🟡 « non vérifiable » (omis en sous-session `[REVIEW:single]`)
3. [ ] **Fichiers exclus** — aucun finding sur un fichier exclu, sauf faille de sécurité
4. [ ] **Conventions** — les conventions du projet ont été lues (ou leur absence est signalée dans `### Périmètre et contexte`), et aucun finding ne les contredit sans justification sécurité explicite
<!-- oh:when wiki -->
5. [ ] **God nodes** — les god nodes touchés par des fichiers modifiés ont été vérifiés via les pages wiki liées ; aucun finding ne contredit un pattern intentionnel de `review-rules.md` ; chaque dérive de `wiki_check` (axe documentation) est un finding 🟡
<!-- /oh:when -->
6. [ ] **Résumé factuel** — résumé et walkthrough décrivent le diff, pas l'état général du code
7. [ ] **Cohérence score/sévérité** — aucun finding ≤ 2/5 n'est 🔴 ou 🟠
8. [ ] **Deux axes** — `### Axe 1 — Conformité à la demande` (un statut ✅/⚠️/❌ par critère, ajouts non demandés, ou « non vérifiable » et sa raison) et `### Axe 2 — Qualité du code` présents ; chaque finding dans un seul axe ; ticket cité sans `bd show` lancé → lance-le maintenant et refais l'axe 1

<!-- oh:when !wiki -->
Sans wiki (projet non onboardé), `### Périmètre et contexte` DOIT contenir :
> ⚠️ Review sans contexte wiki — findings basés sur standards génériques uniquement.
<!-- /oh:when -->
