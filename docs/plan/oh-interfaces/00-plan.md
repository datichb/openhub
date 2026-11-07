# Plan global — Interfaces web et macOS pour oh

Mis à jour le 05/11/2026. **Une session de travail ne lit pas ce fichier** : elle lit le plan de sa version dans [versions/](versions/), plus [versions/regles.md](versions/regles.md), et [versions/methode-n1.md](versions/methode-n1.md) pour les versions 5.6 à 5.8.

## 1. Objectif

- Rendre oh accessible aux personnes peu à l'aise avec le terminal, puis à de futurs utilisateurs non-devs.
- Le TUI **reste** : il cohabite avec le web et l'app macOS.
- Pour commencer : un seul utilisateur, local, développeur. La distribution vient ensuite (6.1).

## 2. Architecture cible (ADR-061, ADR-062)

```
            services / runsvc / daemon / domain   (inchangés)
                              ↑
   internal/ui/contract   interfaces et types sérialisables (JSON), sans tview
   internal/appsvc        logique applicative (sortie de cmd/tui_*.go), utilisée aussi par la CLI
   internal/ui/forms      assistants décrits par des données (machine à états)
                              ↑
   internal/uiserver      HTTP + SSE (socket Unix + 127.0.0.1 avec jeton), parle à ohd
   internal/ui/client     implémentation HTTP du contrat
        ↑                     ↑                     ↑
   TUI (client ; affichage   navigateur         app macOS
   dans tui/v2/present)      (oh web, Vue)      (Swift, WKWebView)
```

**Trois niveaux de découplage :**
- N1 : contrat et `appsvc` (5.6 à 5.8) ;
- N2 : formulaires décrits par des données (5.9) ;
- N3 : client/serveur (5.10).

Ensuite viennent le web et l'app (6.0).

**Technologies (ADR-063)** :
- web : Vue 3, TypeScript, Vite, Reka UI et shadcn-vue, vue-query, vue-i18n, types générés depuis l'OpenAPI ; accessibilité WCAG 2.2 AA contrôlée en CI ; fichiers compilés intégrés au binaire et non committés ;
- app : Swift avec une WKWebView ;
- plus tard : Tauri pour Windows et Linux ;
- **pas de Mac App Store**.

## 3. Feuille de route

| Version | Plan | Contenu | Durée | Visible ? |
|---|---|---|---|---|
| **5.4.1** | [v5.4.1](versions/v5.4.1.md) | **Sécurité : `GH_TOKEN` filtré** ; restes de la 5.4 qui touchent le TUI (titre « circuit », `audit.finding`, messages, rotation de `notifications.jsonl`, espaces solo à l'accueil, garde-fou `ActiveTeam()`) ; réparation de Beads | 1 à 1,5 sem | correctifs |
| **5.5** | [v5.5](versions/v5.5.md) | Fonctionnalités restées après la 5.4 (SSH, Linear et MCP d'équipe à trois états, défauts MCP trouvés par la session F, `homebrew_casks`, `ticket` sur une issue GitLab, `doctor --fix` pour les réservations) ; prérequis (team-state avec interfaces fines, `ctx`, identité du projet et écriture groupée ; flux de `ohd` avec reprise ; écritures atomiques) ; cadrage L0 (ADR, `archtest`, **gel du branchement du TUI**, mesures de référence, tickets) | 4 à 5 sem | oui |
| **5.6** | [v5.6](versions/v5.6.md) | N1-0, socle du contrat (`Scope` et `ResolveTeam` sorti du shell, erreurs, messages, intentions, capacités, présence, bus d'événements, `present`, harnais) ; N1-1 Sessions, lot pilote | 4 à 5 sem | non |
| **5.7** | [v5.7](versions/v5.7.md) | Lancement ∥ Workflows ∥ Configuration, puis Projets ∥ Outils | 4 à 6 sem | non |
| **5.8** | [v5.8](versions/v5.8.md) | Équipe (board, équipes, tracker) ∥ Wiki, puis Transverse (fin de `tuiShell`) : **fin de N1** | 4 à 6 sem | non |
| **5.9** | [v5.9](versions/v5.9.md) | N2, formulaires décrits par des données, et fin de `huh` | 8 à 10 sem | un peu |
| **5.10** | [v5.10](versions/v5.10.md) | N3 : `uiserver`, OpenAPI, SSE, `ui/client`, TUI en client, `oh web`, fin de `oh serve` | 4 à 6 sem | oui |
| **6.0** | [v6.0](versions/v6.0.md) | Web (écrans du profil utilisateur), app macOS, retraits de la v6 | 10 à 12 sem | oui |
| **6.1** | [v6.1](versions/v6.1.md) | Non-devs et distribution : outils gérés, assistant d'accueil, kit d'équipe, notarisation, Sparkle | 4,5 à 6 sem | oui |

**Total : environ 44 à 57 semaines calendaires** pour 1 personne, en tenant compte des lots parallèles (au plus 3 à la fois).

**En dehors de cette feuille de route, au fil de l'eau** :
- CI Windows (20 paquets encore exclus) ;
- isolation stricte, `test-gen`/`db-migration` ;
- bastions SSH (priorité basse) ;
- C9 (icônes encore dans 3 fichiers) ;
- petits points d'interface et de traduction (voir [02](02-backlog-recupere.md) § 7 et § 8).

## 4. Règles structurantes

- **À partir de la fin de la 5.5, le branchement du TUI est gelé** (budget `archtest`). Une nouvelle fonctionnalité met sa logique dans `services/` ou `appsvc/`.
- **Les versions 5.6 à 5.8 ne changent rien pour l'utilisateur.** Chaque lot commence par des tests de caractérisation, et ses goldens d'écran ne doivent pas changer.
- **La CLI passe par les mêmes services `appsvc`** que le TUI : le code écrit deux fois disparaît.
- **Jamais deux lots sur les mêmes fichiers en même temps.** Chaque plan de version donne son ordre et ses recoupements.

## 5. Décisions

| Sujet | Décision | Où |
|---|---|---|
| Niveaux de découplage | Les trois (N1, N2, N3), avant le web et l'app | 05/11 |
| Front | Vue 3 (préférence personnelle) avec Reka UI ; accessibilité prioritaire | 05/11 |
| App | Swift maintenant ; Tauri plus tard pour Windows et Linux | 05/11 |
| Fichiers compilés du front | Non committés ; page de remplacement et étape de release | 05/11 |
| App Store | Non | 05/11 |
| Langue du contrat | **À trancher** (recommandation : codes traduits par le client) | 5.5, D1 |
| `huh` | **À trancher** (recommandation : le remplacer en 5.9) | 5.5, D2 |
| Cask Homebrew sous Linux | **À vérifier** | 5.5, D3 |
| Linear dans `oh mcp setup` | **À trancher** | 5.5, D4 |
| Écritures atomiques en 5.5 | **À trancher** (recommandation : oui pour le team-state, le board et la config) | 5.5, D5 |
| Revue d'ergonomie du TUI | **À trancher** (recommandation : après N1, puis base des maquettes du web) | fin de 5.8 |
| Emplacement du front | **À trancher** (recommandation : `cli/web/`) | 6.0, E1 |
| opencode, kit d'équipe, clonage GitLab, Git masqué | **À trancher** quand les non-devs arriveront | 6.1, F1 à F4 |

## 6. Documents de référence (à ne pas charger dans les sessions)

| Fichier | Contenu |
|---|---|
| [01-etat-des-lieux.md](01-etat-des-lieux.md) | Inventaire du 05/11 au fichier et à la ligne : branchement, globales, logique dans les vues, code en double, tests |
| [02-backlog-recupere.md](02-backlog-recupere.md) | Points repris des autres plans, de l'audit v5, de Beads et du § A de la 5.4, avec la version qui les traite |
| [03-refacto-detaille.md](03-refacto-detaille.md) | Raisonnement complet du refactoring (méthode, vagues, recoupements) ; **les plans de version en sont la forme à jour** |
| [adr-061](adr-061-ui-contract-architecture.fr.md), [adr-062](adr-062-ui-server.fr.md), [adr-063](adr-063-web-desktop-technologies.fr.md) | Brouillons des ADR (écrits dans le dépôt en 5.5) |

## 7. Prochaine étape

Terminer et publier la **5.4.0**, puis lancer la [5.4.1](versions/v5.4.1.md) avec un agent d'implémentation.
