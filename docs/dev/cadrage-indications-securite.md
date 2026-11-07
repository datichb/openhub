# Cadrage : indications de niveau de sécurité

> **Date** : 2026-11-06
> **Statut** : cadrage, rien n'est développé. Sections 4 à 12 : version de référence, à jour des décisions du journal (§ 14). Issu de l'axe J de [la réflexion sur l'isolation](reflexion-isolation-securite.md). Le conteneur par défaut est **hors périmètre** (écarté le 2026-11-06).
> **Chemins** : relatifs à `cli/`, état du code au 2026-11-06.

---

## 1. Objectif

Dire à l'utilisateur, au moment où il lance une session et ensuite, **ce qu'un agent peut faire dans cette session** : où il peut écrire, ce qu'il peut lire, s'il peut sortir sur le réseau, qui valide ses actions, ce qui peut lui injecter des instructions. Et, quand une option plus sûre existe et qu'elle est possible tout de suite, la proposer.

### Non-objectifs

- Pas de note globale (« 7/10 », « A+ ») : elle masque ce qui compte et rassure à tort.
- Pas de nouvelle règle de sécurité : on montre la posture, on ne la change pas. Imposer un niveau minimal relève d'un second cadrage (§ 13).
- Pas de conteneur par défaut ni de repli automatique.
- Pas de promesse que le code ne tient pas : le réseau est **ouvert** partout tant que l'axe A n'est pas fait, et on l'affiche.

## 2. Principes

1. **Factuel** : chaque facette décrit une propriété vérifiable de la session (« Fichiers : toute la machine, avec vos droits »), jamais un jugement.
2. **Prévu, puis constaté** : avant le lancement on montre la posture **prévue** (paquet, réglages) ; après `Attest` on enregistre la posture **constatée** (plugins chargés, serveurs MCP vus). Un écart entre les deux est signalé.
3. **Informer, puis proposer** : chaque indication décrit un fait ; une action n'est proposée que si elle est possible tout de suite (mode, runtime autorisé et disponible, réglage).
4. **Pas de lassitude** : une ligne résumée à chaque lancement, des indications seulement en cas de risque, deux au plus, les plus fréquentes une seule fois par projet ; masquage par projet ou pour tous les projets, sauf les critiques.
5. **Même vérité partout** : un seul calcul (service), rendu par la CLI, la TUI, la sortie sans terminal et `oh doctor`.
6. **Hors d'atteinte des agents** : un agent ne peut ni masquer une indication ni changer ce qui la déclenche sans passer par les garde-fous existants (`oh config set` est déjà refusé aux agents).

## 3. Ce qui existe aujourd'hui (Code)

| Élément | Où | Limite |
|---|---|---|
| Récap de lancement | `cmd/run_start.go:110-169` (`launchRecap`), lignes `[][2]string` + avertissements `[]string` ; partagé CLI (`printRunRecap`, `:198-207`) et TUI (`cmd/tui_launch.go:188-211` → `views.LaunchRecap`, `internal/tui/v2/views/launch_form_model.go:148-161`) | Construit dans `cmd`, pas dans un service ; pas de structure par facette |
| Ligne « isolation » du récap | `cmd/run_start.go:136-140`, `cmd/bundle.go:184-188` | **Presque toujours `full`** : le niveau est fixé à `IsolationFull` à la construction du paquet (`internal/bundle/build.go:169`) ; le vrai niveau n'est connu qu'après `Attest` |
| Niveau constaté | `Attest` (`internal/adapters/opencodev2/attest.go:34-149`) → `VisibilityReport{…, Unexpected, Warnings, Level}` (`internal/adapters/adapter.go:91-110`) ; envoyé en événements `EventWarning` (`internal/services/launch/start.go:50-57`) | **Non enregistré** par session |
| Avertissements du plan | `runsvc.Warning{Code, Args}` (`internal/runsvc/run.go:28-68`), traduits `cmd.run.warn.<code>` | Pas de gravité, pas de regroupement |
| Disponibilité des runtimes | `views.LaunchRuntime{Kind, Label, Available, Reason}` (`launch_form_model.go:16-21`), `tuiRuntimeKinds` (`cmd/tui_launch.go:484-516`), en-tête projet `runtimesLine` (`project_mode_view.go:361-377`) | Disponibilité seulement, rien sur ce que chaque runtime protège |
| Sessions enregistrées | table `sessions` : `runtime`, `mode`, `type` (headless), `workflow_risk`, `fanout`, `parent_session_id`… (`internal/storage/sqlite/store.go`) | Ni niveau d'isolation, ni plugins, ni recherche web |
| Préférences | table `preferences` (`scope`, `key`, JSON), `prefsvc.Service` (`internal/prefsvc/service.go:197-218`), portées globale / projet / équipe ; exemple « vu une fois » : `tui.keys_notice` (`cmd/tui_keys_notice.go:24-56`) | Aucun mécanisme générique d'indications |
| `oh doctor` | `doctor.Result{ID, Name, Detail, OK, Warn, Optional, Fix}` (`internal/services/doctor/doctor.go:15-31`) ; `securityDoctorChecks` (`cmd/doctor_security.go:19-77`) ; `analyzeEnvAllow` (`cmd/doctor_env.go:28-60`) ; plugins utilisateur (`cmd/doctor_plugins.go`) | Contrôles éparpillés, pas de synthèse de posture |

## 4. Facettes

Huit facettes. Colonne « Ligne » : affichée dans le résumé d'une ligne (**toujours**, ou **si exception**), sinon seulement dans le détail et le JSON.

| Facette | Valeurs | Ligne | Calcul (sources) |
|---|---|---|---|
| **Fichiers** | `machine` (local : toute la machine, avec vos droits) · `project` (conteneur : emplacements montés + caches) · `ci_clone` (distant : clone dans le job) | toujours | `p.Resolution.Runtime`, `p.Remote` |
| **Validation** | `manual` / `semi-auto` / `auto` + nombre de pauses prévues ; `unattended` si sans terminal | toujours | `p.Resolution.Mode`, `bundle.PlannedPauses(wf, mode)` (`cmd/run_pauses.go:21-44`), `Request.Base.Headless`, `p.Resolution.Fanout` |
| **Réseau** | `open` (hors conteneur, et partout aujourd'hui) · `logged` (conteneur en phase de journal, non filtré) · `allowlist` (conteneur en refus) — voir [cadrage du filtrage réseau](cadrage-filtrage-reseau.md) | toujours | niveau et phase du filtrage du groupe de la session |
| **Sources non fiables** | recherche web, branche ou MR relue, plugins tiers, serveurs MCP hors oh | si exception | `bundle.ToolEffect(…, ActionWebSrch, "*")` (`internal/bundle/when.go:361`), `launch.ReadBranch` (`internal/services/launch/launch.go:521`), réf. GitLab `Kind == "mr"`, `bundle.Report.Plugins`, `UserPluginLister.UserPlugins()`, registre `~/.oh/mcp/` (prévu), MCP inattendus d'`Attest` (constaté) |
| **Monde fermé** | prévu : `full` / `full·strict` ; constaté : `full` / `partial` (plugins hors paquet chargés) / `none` (agent ou MCP inattendu) | si écart | `bundle.Report.Isolation`, `StrictIsolation` ; puis `VisibilityReport.Level`, `Warnings`, `Unexpected` |
| **Origine** | `human` · `session` (lancé depuis le shell d'une session : agent, ou humain via `!oh run`) · `oh` (enfant de fan-out, merger) | si `session` | `OH_SESSION_ID` dans l'environnement du lanceur **ou** filiation vérifiée par le démon (étape T3) ; `ParentSessionID`, `Session.Fanout.Role` |
| **Secrets** | « clés d'oh hors d'atteinte (proxy) » ; en local, en plus : « vos fichiers d'identifiants restent lisibles, protégés seulement par les règles shell » ; variables d'`env_allow` qui ressemblent à des secrets | si `env_allow` suspect | constant ; `envfilter.LooksSecret` sur `Config.Execution.EnvAllow` (comme `analyzeEnvAllow`, `cmd/doctor_env.go:28-60`) |
| **Écritures** | `read` / `plan` / `write` / `publish` | jamais | `p.Resolution.Spec.Risk` |

## 5. Ligne résumée

- Ordre : Fichiers · Validation · Réseau · exceptions (sources non fiables, écart de monde fermé, origine agent, `env_allow` suspect).
- Exemples :
  - `⌂ machine · semi-auto (3 pauses) · réseau ouvert`
  - `⌂ machine · auto (aucune pause) · réseau ouvert · ⚠ recherche web`
  - `▣ projet · semi-auto (2 pauses) · réseau ouvert · ⚠ 2 plugins perso`
  - `⌂ machine · sans terminal · réseau ouvert · ⚠ lancée par un agent`
- Affichée : récap TUI et CLI, `oh run` **toujours** (stderr), liste et détail des sessions (posture constatée, L3).

## 6. Indications

### 6.1 Catalogue v1

| ID | Niveau | Condition | Action proposée (si possible) | Masquable | Fréquence |
|---|---|---|---|---|---|
| `observed_drift` | critique | posture constatée moins bonne que prévue (`Level` `partial`/`none`, MCP ou agent inattendu) | voir le détail, `oh doctor` | non | à chaque écart |
| `env_allow_secret` | critique | entrée d'`env_allow` qui ressemble à un secret | retirer l'entrée (Réglages › Exécution) | non | à chaque lancement |
| `agent_launch` | critique | origine `agent` | aucune ; ID de la session d'origine affiché et enregistré | non | à chaque lancement |
| `local_auto_write` | important | local + mode `auto` + `risk` ≥ `write` | passer en semi-auto ; `▣ conteneur` si autorisé et disponible | oui | à chaque lancement |
| `local_unattended` | important | local + sans terminal (`--headless` ou fan-out) + écriture | `▣ conteneur` si autorisé et disponible | oui | à chaque lancement |
| `review_runs_foreign_code` | important | lecture d'une branche ou d'une MR + commandes de vérification du projet | refuser la commande de test ; `☁ distant` si activé et autorisé | oui | à chaque lancement |
| `infra_write_creds` | important | workflow `infra-change` en local | profil en lecture seule (conseil, pas d'action directe) | oui | à chaque lancement |
| `local_websearch_write` | info | local + recherche web + écriture | `▣ conteneur` si autorisé et disponible | oui | une fois par projet, puis marqueur dans la ligne |
| `user_plugins` | info | plugins de la config utilisateur chargés, sans config personnelle masquée | activer « Config personnelle masquée » (Réglages › Exécution) | oui | une fois par projet, puis marqueur dans la ligne |

### 6.2 Règles

- **Deux indications au plus** dans le récap : critique, puis important, puis info ; à niveau égal, ordre du catalogue. Les autres derrière « voir les N autres » (TUI) ; toutes listées par `--recap` (CLI).
- **Action seulement si possible** : une action de runtime n'est proposée que si le workflow l'autorise (`runtime.allowed`) et que le moteur est disponible ; le distant seulement s'il est activé. Sans action possible, l'indication reste (c'est un risque), sans proposition.
- **Pas de doublon** : une indication qui recouvre un avertissement existant (`runsvc.Warning`, avertissements d'`Attest` sur les plugins) le remplace.
- **Une fois par projet** : après le premier affichage, l'indication n'apparaît plus que comme marqueur dans la ligne résumée ; l'enregistrement « déjà vue » utilise les préférences du projet.

## 7. Réaction selon le niveau et la surface

| Niveau | TUI (récap) | CLI interactive (`oh run` dans un terminal) | Sans terminal (`--headless`, `--json`) |
|---|---|---|---|
| Critique connu avant lancement (`env_allow_secret`) | confirmation « Lancer quand même ? » | même confirmation | signalé, le lancement continue |
| Lancement par un agent (`agent_launch`) | toast + notification système quand la TUI est ouverte ; marque sur la session (L3) | — (un agent n'a pas de terminal) | dans la sortie (lue par l'agent) |
| Critique constaté après lancement (`observed_drift`) | toast + notification système ; marque sur la session (L3) ; pas de pause | message sur stderr | dans la sortie |
| Important | affichée + raccourci d'action | affichée sur stderr, action en texte | dans la sortie |
| Info | affichée une fois par projet, puis marqueur | idem | dans la sortie |

## 8. Surfaces et lots

| Surface | Contenu | Lot |
|---|---|---|
| Libellés « isolation » | renommage (§ 11) et ligne « Monde fermé : prévu complet (vérifié au lancement) » au lieu d'un `full` trompeur | L0 |
| `oh run` (toujours) | ligne résumée + indications critiques et importantes, sur stderr ; récap descendu dans `services/launch/recap`, service `posture` (entrée neutre), origine (variable + filiation vérifiée par le démon) | L1 |
| Récap CLI (`--recap`, `--dry-run`) | ligne + détail des facettes + toutes les indications | L1 |
| Fiche TUI, étape Options | une phrase sous le choix du runtime : ce qu'il protège (facette Fichiers), mise à jour au changement | L2 |
| Récap TUI | ligne + facettes repliables + deux indications au plus, actions sur `1`/`2` (recalcul sur place), confirmation de `env_allow_secret` | L2 |
| Écart après lancement (TUI) | toast + notification système (`observed_drift`, `agent_launch`) | L2 |
| Posture constatée enregistrée | colonne `posture` ; marque d'écart et badge dans la liste et le détail des sessions ; `oh session list --json` | L3 |
| `oh run --json` | flux JSON Lines (préparation, posture prévue et constatée, progression de la session via le flux du démon, aperçu des messages), dernière ligne `result` ; page `docs/reference/posture.*.md` et JSON Schema | L3 |
| `oh doctor` | contrôle « Posture de sécurité » : runtime par défaut, moteur disponible, config personnelle masquée, recherche web, `env_allow`, plugins utilisateur | L4 |
| Masquage | « ne plus afficher », case « pour tous mes projets », Réglages › Sécurité (afficher les indications, réafficher les masquées) | L5 |
| Board | marque sur la carte quand la session liée est en écart ou en `auto` local | plus tard |

L0 peut partir seul ; L1 conditionne les suivants. Quand l'axe A sera livré, la facette Réseau prendra la valeur `allowlist` sans autre changement.

## 9. Cas limites

| Cas | Comportement |
|---|---|
| Reprise d'une session en veille | posture recalculée ; seuls les changements sont affichés (nouvelle indication, facette qui baisse) |
| Transfert local ↔ conteneur | avant/après des facettes ; une baisse (conteneur → local) est signalée et confirmée dans la TUI |
| Fan-out | la parente porte la posture ; un écart constaté chez un enfant ou le merger remonte sur la parente (toast + marque) |
| Distant | posture prévue au lancement ; constatée au retour (`oh session fetch`) et comparée |

## 10. Masquage

- Portée **projet** par défaut (`ProjectPreferenceScope`) ; case « pour tous mes projets » (portée globale) pour les indications qui ne dépendent pas du projet (`user_plugins`, `local_websearch_write`).
- Réaffichage par un **réglage manuel**, sans expiration.
- Seulement depuis la **TUI et les Réglages** : aucune commande CLI, donc hors d'atteinte des agents.
- Non masquables : `observed_drift`, `env_allow_secret`, `agent_launch`.
- Masquer une indication ne masque jamais la facette ni le marqueur de la ligne.

## 11. Vocabulaire (dès L0)

| Notion | Aujourd'hui | Libellé à l'écran |
|---|---|---|
| `isolation` du workflow (`strict`/`standard`) | « isolation » dans le récap | **Monde fermé** (strict / standard) |
| `execution.strict_isolation` | « isolation stricte » dans les Réglages | **Config personnelle masquée** |
| Runtime | « exécution » | **Exécution** (local / conteneur / distant), avec la facette Fichiers |

Les clés de configuration ne changent pas ; seuls les libellés et la doc changent.

## 12. Enregistrement

- Posture **constatée** enregistrée après `Attest` dans une colonne JSON `posture` de `sessions` (prochaine migration libre ; v47 et v50 réservées par d'autres lots).
- La posture **n'entre pas** dans le hachage du paquet (elle dépend des réglages et de l'environnement).
- Reprise, transfert et retour du distant mettent la colonne à jour ; l'écart avec la valeur précédente est signalé.

## 13. Décisions initiales (2026-11-06)

Prises avant le cadrage partagé ; les sections 4 à 12 les intègrent.

| Sujet | Décision |
|---|---|
| Niveau minimal imposé par l'équipe | **Plus tard**, dans un second cadrage qui réutilisera le vocabulaire des facettes. Les indications restent informatives. |
| Session lancée par un agent | **Facette + indication**, sans refus pour l'instant (voir ci-dessous). |
| Sortie sans terminal | **Nouvelle option `oh run --json`** : plan, posture et avertissements en JSON structuré sur stdout (`RunPlan` a déjà des balises JSON, `internal/runsvc/run.go:120-124`). Lot L3. |
| Où vit le calcul | **Nouveau package `internal/services/posture`**, et le récap descend de `cmd` (`launchRecap`, `cmd/run_start.go:110-169`) vers un service partagé par la CLI et la TUI. Lot L1. |
| Serveurs MCP tiers avant lancement | **Oui** : la posture prévue lit le registre `~/.oh/mcp/<nom>/manifest.json` (`internal/mcp/mcpregistry/registry.go:97-139`) ; `Attest` confirme ensuite. Il faudra exposer la lecture (`customMCPServer` n'est pas exporté). |
| Enregistrement | **Colonne JSON `posture` dans `sessions`** (prochaine migration libre). |
| Premier lot | **L0 puis L1**, une fois les changements en cours fusionnés. |

### Facette « Origine » (ajoutée)

| Facette | Valeurs | Calcul | Niveau |
|---|---|---|---|
| **Origine** | `human` · `agent` (lancé depuis le shell d'une session) · `oh` (enfant de fan-out, merger, lancé par oh) | `OH_SESSION_ID` dans l'environnement du lanceur (`runsvc.EnvSessionID`, `internal/runsvc/env.go:31`) ; `ParentSessionID`, `Session.Fanout.Role` | `agent` = attention |

Indication associée :

| ID | Condition | Message (idée) | Action | Masquable |
|---|---|---|---|---|
| `agent_launch` | origine `agent` | Cette session a été lancée par un agent depuis la session `<id>`, pas par vous | aucune (informatif) ; la posture est enregistrée avec l'ID de la session d'origine | **non** |

Points à préciser en L1 : `OH_SESSION_ID` est-il présent dans tous les shells d'agents (local, conteneur) et absent des lancements humains depuis un terminal ouvert par oh (`termlaunch`) ? Faut-il un jeton de session plutôt qu'une simple variable, pour qu'un humain ne soit pas pris pour un agent et inversement ? Le refus éventuel de `oh run` aux agents relève du second cadrage (politique).

### Questions encore ouvertes

- Format exact de `oh run --json` : schéma versionné ? même objet pour `--dry-run --json` ?
- Gravité de `agent_launch` quand la session d'origine est elle-même en conteneur.

## 14. Journal du cadrage partagé

Cadrage mené ensemble, étape par étape ; chaque décision est notée ici, puis un ADR sera rédigé à la fin. Ordre : indications de sécurité, puis filtrage réseau sortant (axe A). Pour chacun : le **quoi** (comportement, règles, cas limites), puis le **comment** (code, données, tests).

### Étape 1 — Cadre (2026-11-06)

| Question | Décision |
|---|---|
| Profil prioritaire | **Développeur dans la TUI** ; la CLI et `--json` reprennent le même contenu ; l'enregistrement et les vues d'équipe viennent ensuite |
| Quand afficher | **Mixte** : résumé d'une ligne à chaque lancement, indications mises en avant seulement en cas de risque |
| Ton | **Informer + proposer** : le fait, puis une action seulement si elle est possible tout de suite |

### Étape 2 — Facettes affichées (2026-11-06)

| Question | Décision |
|---|---|
| Résumé d'une ligne | **Fichiers + Validation + Réseau toujours**, puis les exceptions seulement quand elles existent : sources non fiables, écart de monde fermé, origine agent, `env_allow` suspect. Exemple : `⌂ machine · semi-auto (3 pauses) · réseau ouvert · ⚠ recherche web`. Réseau est gardé pour ne pas laisser croire que le conteneur empêche la sortie. |
| Secrets | **Dans le détail**, avec la nuance locale (« clés d'oh hors d'atteinte ; en local, vos fichiers d'identifiants restent lisibles, protégés seulement par les règles shell ») ; dans la ligne seulement si `env_allow` est suspect |
| Écritures | **Détail et JSON seulement** ; le nom du workflow et le mode suffisent dans la ligne |
| Monde fermé, Origine | Dans la ligne seulement en cas d'écart ou de lancement par un agent (exceptions) ; sinon détail |

### Étape 3 — Catalogue des indications (2026-11-06)

| Question | Décision |
|---|---|
| Indications de la v1 | **Les neuf** : `local_auto_write`, `local_unattended`, `local_websearch_write`, `review_runs_foreign_code`, `user_plugins`, `env_allow_secret`, `infra_write_creds`, `observed_drift`, `agent_launch` |
| Indications fréquentes (`local_websearch_write`, `user_plugins`) | **Une fois par projet**, ensuite seulement le marqueur dans la ligne résumée |
| Limite | **Deux au plus** dans le récap, les plus graves d'abord ; les autres derrière « voir les N autres » |

### Étape 4 — Gravité et réaction (2026-11-06)

| Question | Décision |
|---|---|
| Niveaux | **Critique** : `observed_drift`, `env_allow_secret`, `agent_launch` (non masquables) · **Important** : `local_auto_write`, `local_unattended`, `review_runs_foreign_code`, `infra_write_creds` · **Info** : `local_websearch_write`, `user_plugins` |
| Réaction dans la TUI | Important et info : affichage + **raccourci d'action** dans le récap (ex. passer en semi-auto) ; critique connu avant lancement (`env_allow_secret`, `agent_launch`) : **confirmation** « Lancer quand même ? » ; `observed_drift` : message très visible, **pas de pause** automatique (une pause relèverait du second cadrage, politique) |
| Sans terminal (`--headless`, `--json`) | **Signaler et continuer** : indications dans la sortie, aucun refus |

### Étape 5 — Surfaces (2026-11-06)

| Question | Décision |
|---|---|
| Étape Options de la fiche | **Une phrase sous le choix du runtime** sur ce qu'il protège (facette Fichiers), mise à jour quand on change de runtime |
| Écart constaté après lancement (`observed_drift`) | **Toast + marque sur la session + notification système** (paquets existants `internal/notify`, `internal/notifypolicy`, `internal/sysnotify` : à suivre leur politique) |
| Badge dans les vues des sessions | **Avec L3** (enregistrement de la posture) ; L1-L2 restent fiche et récap |
| `oh run` sans `--recap` | **Ligne résumée toujours affichée**, plus les indications importantes et critiques |

Conséquences notées : la marque d'écart sur la session dépend de l'enregistrement (L3) ; en L1-L2, un écart donne toast + notification seulement. La ligne de `oh run` part sur stderr, comme les avertissements actuels.

### Étape 6 — Masquage et vocabulaire (2026-11-06)

| Question | Décision |
|---|---|
| Portée | **Projet par défaut**, case « pour tous mes projets » quand l'indication ne dépend pas du projet (`user_plugins`, `local_websearch_write`) ; portées `ProjectPreferenceScope` et globale de `prefsvc` |
| Réaffichage | **Réglage manuel** « réafficher les indications masquées », sans expiration |
| Commande CLI | **Aucune** : masquage seulement depuis la TUI et les Réglages, hors d'atteinte des agents |
| Renommage des libellés | **Dès L0** : « Monde fermé » (`isolation` du workflow), « Config personnelle masquée » (`execution.strict_isolation`), « Exécution » (runtime) ; les clés de configuration ne changent pas |

### Étape 7 — Cas limites (2026-11-06)

| Question | Décision |
|---|---|
| Reprise d'une session en veille | **Recalcul ; seuls les changements sont affichés** (nouvelle indication, facette qui baisse) |
| Transfert local ↔ conteneur | **Avant/après des facettes** ; une baisse (conteneur → local) est signalée et **confirmée** dans la TUI |
| Fan-out | **La parente porte la posture** ; un écart constaté chez un enfant ou le merger remonte sur la parente (toast + marque) |
| Distant | **Posture prévue au lancement, constatée au retour** (`oh session fetch`) et comparée |

### Étape 8 — Messages et ajustements (2026-11-06)

| Question | Décision |
|---|---|
| Messages (§ 15.1) | **Validés en bloc** ; retouches possibles au développement |
| `agent_launch` | **Comme un écart constaté** : pas de confirmation (impossible sans terminal), toast + notification dans la TUI, marque sur la session (L3) |
| Confirmation en CLI interactive | **Oui, comme la TUI**, pour `env_allow_secret` |
| Option « sans recherche web pour ce lancement » | **Non créée** |

Fin de la partie « quoi » des indications.

### Étape T1 — Architecture du calcul et du récap (2026-11-06)

| Question | Décision |
|---|---|
| Entrée du calcul | **Structure neutre `posture.Input`**, remplie par `launch` ; `posture` n'importe ni `launch` ni `bundle` (pas de cycle, tests sans paquet ni session) |
| Traduction | **Hybride** : la posture produit des codes stables, traduits à l'affichage et réutilisés par `--json` ; le reste du récap descend tel quel avec ses lignes traduites |
| Emplacement du récap | **Sous-paquet `internal/services/launch/recap`** (choix délégué : « le plus propre ») — voir le raisonnement ci-dessous |
| Posture constatée | **Dans `launch.Start`**, à la réception du rapport d'`Attest` (`start.go:50-57`) ; nouvel événement « posture » vers la TUI ; enregistrement branché là en L3 |

Raisonnement pour le récap : le récap est un **modèle de présentation** d'un lancement préparé, pas de l'orchestration. Le mettre dans `services/launch` mêlerait les deux responsabilités dans un paquet déjà large (préparation, résolution, fan-out, distant, démarrage). Un sous-paquet suit le précédent de `services/remote/runner`, et donne un graphe sans cycle :

```
posture            (feuille : Input → Posture, aucune dépendance oh hors i18n éventuel)
  ↑
launch             (remplit posture.Input ; Start calcule la posture constatée)
  ↑
launch/recap       (lignes du récap + posture ; utilise internal/glyph, pas la TUI)
  ↑
cmd, internal/tui  (affichage)
```

Conséquences : `theme.IconArrow` remplacé par l'icône de `internal/glyph` dans le récap ; `runRecap`/`launchRecap` (`cmd/run_start.go:99-169`) et leurs aides (`recapBeads`, `recapCheckpoints`, `recapPauses`, `recapModelRows`, `warningText`, `remoteRecapWarnings`) suivent le récap ; la règle des couches (`internal/archtest/layers_test.go:60-69`) le vérifie déjà.

### Étape T2 — Formats de données (2026-11-06)

| Question | Décision |
|---|---|
| Schéma | **Un seul objet `posture`, versionné** (`"version": 1`), dans la colonne et dans `--json` |
| Colonne `posture` | **Prévue + constatée + écarts** |
| Contrat public | **Codes documentés dans `docs/reference/` (fr/en) + test golden** du JSON |
| Forme de `oh run --json` | **Flux JSON Lines** (une ligne par événement, codes stables) dont la **dernière ligne est toujours un événement `result`** complet (plan + posture + résultat) ; `--dry-run --json` : plan, posture, puis `result` sans réponse |
| Contenu du flux en v1 | **Avec la progression de la session** (choix contre la recommandation « lancement seulement ») |

Faisabilité de la progression (Code) : le démon diffuse déjà un flux par session, utilisé par la TUI (`daemon/client.Client.Stream`, `internal/daemon/client/client.go:217`) : `domain.FeedItem` (`internal/domain/feed.go:20-30`, déjà sérialisé en JSON, genres stables `agent`, `text`, `tool`, `delegate`, `decision`, `state`, `usage`) et `domain.SessionChange` (état, décisions). Le mode sans terminal attend aujourd'hui la fin du tour sans s'y abonner (`runsvc.AwaitTurn`, `internal/runsvc/headless.go:37-89`) : il suffit de s'abonner pendant l'attente. En revanche, les événements de lancement (`launch.Event`, `internal/services/launch/launch.go:219-244`) ne portent que du texte traduit : il faut leur ajouter des codes.

| Question (suite) | Décision |
|---|---|
| Messages de l'agent (`text`) | **Aperçu** (première ligne, ~200 caractères) dans le flux ; texte complet seulement dans `result` (évite de verser le contenu du dépôt dans les journaux de CI) |
| Appels d'outils (`tool`) | **Titre comme dans la TUI**, commande comprise, avec le statut |
| Sous-agents | **Inclus** ; chaque ligne porte `session_id` et `parent_id` |

Esquisse d'une ligne (à figer avec la doc de référence) :

```json
{"v":1,"type":"feed","time":"…","session_id":"…","parent_id":"…","kind":"tool","tool":"bash","title":"npm test","status":"ok"}
{"v":1,"type":"posture","phase":"observed","session_id":"…","posture":{"version":1,"…":"…"}}
{"v":1,"type":"result","plan":{"…":"…"},"sessions":[{"session_id":"…","status":"done","text":"…","posture":{"…":"…"}}],"exit_code":0}
```

### Étape T3 — Fiabilité de la facette Origine (2026-11-06)

| Question | Décision |
|---|---|
| Détection | **Variable + filiation** : origine « depuis une session » si `OH_SESSION_ID` est présent **ou** si le démon voit que l'appelant descend d'un serveur de session (lecture du processus pair du socket et de ses parents, mécanisme des jetons de hook Beads : `internal/daemon/hooktoken.go`, `internal/procinfo`) |
| Libellé | « Cette session est lancée depuis le shell de la session %s. » / "This session is launched from the shell of session %s." — exact aussi pour un humain qui tape `!oh run` dans le TUI opencode (remplace le message validé à l'étape 8) |
| Promesse | **Indication, pas barrière** : documentée comme contournable dans `SECURITY.md` ; refuser `oh run` aux agents relève du second cadrage (politique) |

Limites notées : un processus détaché (`nohup`, `setsid`) perd sa filiation ; pas de lecture des processus sous Windows ; en conteneur, la question ne se pose pas (pas de binaire `oh`). La valeur de facette `agent` devient `session` (« depuis une session ») pour rester exacte.

### Étape T4 — Touches d'action dans le récap (constat, 2026-11-06)

Touches déjà prises à l'étape Récap de la fiche : `t`, `s`, `b` (répertoire modifié : worktree, remiser, laisser ; `internal/tui/v2/views/launch_form.go:603-610`), `R`, `T`, `K`, `d`, `p` (session existante ; `launch_form_existing.go:47-53`), `Ctrl+S` (lancer), `Échap`. La convention (`internal/tui/v2/keys/convention_test.go`, `docs/dev/tui-conventions.md`) réserve `g G / r ? d x R y n` ; `c` désigne « comparer ». Les touches proposées à l'étape 8 (`s` semi-auto, `c` conteneur, `d` distant) sont donc **en conflit**. Aucun chiffre n'est utilisé dans la TUI.

| Question | Décision |
|---|---|
| Touches | **Chiffres `1` et `2`** devant chaque action proposée (au plus deux indications affichées) ; à déclarer avec `keys.Own` et à ajouter à `docs/dev/tui-conventions.md` |
| Effet | **Recalculer et rester** : l'action change le choix (mode, runtime), le récap et la posture sont recalculés, on reste au récap (comme les choix du répertoire modifié, `dirtyKey`) |

Exemple : `[1] Passer en semi-auto   [2] Lancer en conteneur`. Les touches des messages (§ 15.1) deviennent `1`/`2` selon l'ordre d'affichage.

### Étape T5 — Tests et documentation (2026-11-06)

| Question | Décision |
|---|---|
| Tests du flux `--json` | **Faux flux du démon seulement** (pas de test e2e avec vrai modèle ajouté) |
| Documentation | **Page dédiée `docs/reference/posture.{fr,en}.md`** (facettes, indications, codes, format des lignes), liée depuis `cli-workflows` et `SECURITY.md` |
| JSON Schema | **Oui** : `posture.schema.json` et `run-events.schema.json` versionnés ; le golden est validé contre eux |

Tests prévus par ailleurs (sans décision nouvelle) : table du service `posture` (runtime × mode × risque × recherche web × plugins × origine) ; règle « action possible » ; golden du récap CLI ; écart constaté (`Level` `partial`/`none`) ; parité i18n et clés à préfixe dynamique (`TestNoOrphanKeys`) ; convention des touches (`1`/`2`).


## 15. Messages (brouillon à valider)

Règles d'écriture : le fait d'abord, à la 2ᵉ personne, une phrase ; l'action ensuite, introduite par sa touche dans la TUI. Pas de « attention », pas de point d'exclamation, pas de jugement. `%s` = argument.

### 15.1 Les neuf messages

| ID | Message (fr) | Message (en) | Action (fr / en) |
|---|---|---|---|
| `observed_drift` | Ce qui tourne ne correspond pas au paquet de la session : %s. | What runs does not match the session bundle: %s. | Voir le détail (`oh doctor`) / See the details (`oh doctor`) |
| `env_allow_secret` | La variable %s de votre environnement est transmise aux agents et ressemble à un secret. | The variable %s from your environment is passed to the agents and looks like a secret. | Retirer de Réglages › Exécution / Remove it in Settings › Execution |
| `agent_launch` | Cette session est lancée depuis le shell de la session %s. | This session is launched from the shell of session %s. | — |
| `local_auto_write` | Mode auto en local : l'agent modifie vos fichiers avec vos droits, sans pause. | Auto mode, local: the agent changes your files with your rights, with no pause. | `s` Passer en semi-auto · `c` Lancer en conteneur / `s` Switch to semi-auto · `c` Run in a container |
| `local_unattended` | Session sans terminal en local : personne ne relira les actions de l'agent avant qu'il les fasse. | Unattended local session: nobody reviews the agent's actions before it takes them. | `c` Lancer en conteneur / `c` Run in a container |
| `review_runs_foreign_code` | La branche relue vient d'ailleurs : si vous acceptez ses tests, ils s'exécutent sur votre machine. | The reviewed branch comes from elsewhere: if you accept its tests, they run on your machine. | Refuser la commande de test si vous ne connaissez pas l'auteur · `d` Lancer en distant / Refuse the test command if you do not know the author · `d` Run remotely |
| `infra_write_creds` | infra-change utilise les identifiants cloud de votre machine. | infra-change uses the cloud credentials of your machine. | Préférez un profil en lecture seule (voir SECURITY.md) / Prefer a read-only profile (see SECURITY.md) |
| `local_websearch_write` | Recherche web activée : une page lue par l'agent peut lui donner des instructions, et il écrit avec vos droits. | Web search is on: a page the agent reads can give it instructions, and it writes with your rights. | `c` Lancer en conteneur / `c` Run in a container |
| `user_plugins` | %d plugins de votre config opencode tournent dans la session : %s. | %d plugins from your opencode config run in the session: %s. | Activer « Config personnelle masquée » (Réglages › Exécution) / Turn on "Hide personal config" (Settings › Execution) |

Confirmation (critiques connus avant lancement) : « Lancer quand même ? » / "Launch anyway?" — `Entrée` lancer, `Échap` revenir.

Phrase sous le runtime (étape Options) :
- `⌂ local` : « L'agent travaille avec vos droits sur toute la machine. » / "The agent works with your rights on the whole machine."
- `▣ conteneur` : « L'agent ne voit que le projet ; vos clés restent hors d'atteinte. Le réseau reste ouvert. » / "The agent only sees the project; your keys stay out of reach. The network stays open."
- `☁ distant` : « L'agent travaille sur un clone dans un job CI ; votre machine n'est pas touchée. » / "The agent works on a clone in a CI job; your machine is not touched."

### 15.2 Problèmes relevés en rédigeant (tranchés à l'étape 8)

1. **`agent_launch` et la confirmation** : un agent lance `oh run` depuis son shell, donc **sans terminal interactif** ; la confirmation ne se déclencherait jamais, et le message part sur la sortie que lit l'agent. → Traité **comme un écart constaté** : toast + notification dans la TUI, marque sur la session (L3).
2. **CLI interactive** → **même confirmation** que la TUI pour `env_allow_secret`.
3. **Action « sans recherche web pour ce lancement »** : n'existe pas (réglage global `websearch.enabled`) → **pas d'option créée** ; l'indication propose le conteneur, ou rien.
4. **Touches** `s`, `c`, `d` : à vérifier contre les raccourcis existants de la fiche (partie technique).

## 16. Partie technique (brouillon, à cadrer ensemble)

Esquisse à discuter, pas une décision :

```go
type Level string // "critical", "important", "info"

type Facet struct {
    ID     string   // files, validation, network, sources, closed_world, origin, secrets, writes
    Value  string   // code stable traduit à l'affichage
    InLine bool     // affichée dans la ligne résumée
    Detail []string
}

type Hint struct {
    ID          string
    Level       Level
    Args        []any
    Actions     []string // "mode:semi-auto", "runtime:container", "runtime:remote", "settings:hide_personal_config"
    Dismissible bool
    OncePerProject bool
}

type Posture struct {
    Phase  string // "planned" | "observed"
    Facets []Facet
    Hints  []Hint
    Drift  []string
}

func Planned(p *launch.Prepared, env Env) Posture
func Observed(planned Posture, r adapters.VisibilityReport) Posture
```

Sujets à cadrer : descente du récap de `cmd` vers un service, schéma de `oh run --json` (versionné ?), format de la colonne `posture`, fiabilité de l'origine `agent` (`OH_SESSION_ID` ou jeton), lecture du registre MCP, clés i18n (`cmd.posture.*`, préfixes dynamiques pour `TestNoOrphanKeys`), tests (table runtime × mode × risque × recherche web × plugins ; règle « action possible » ; golden du récap ; écart constaté).
