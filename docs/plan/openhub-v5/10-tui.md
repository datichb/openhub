# 10 — Spécification de la TUI

TUI = **tview/tcell** (pas BubbleTea). Principes : (1) aucune logique métier dans `cmd/tui_*.go` ; (2) le workflow est l'unité de lancement ; (3) contexte actif unique ; (4) sessions pilotables ; (5) réutiliser `SectionedList`, `configField`, `InlineWizardView`, `ShowInlineForm`, `WorkflowGraph`, `UndoStack`, notifications. Tous les textes via i18n `tui.*` (fr/en, parité testée).

Légende : `▸` sélection · `★` épinglé · `●` en cours · `⏸` checkpoint · `?` question · `!` permission · `$` budget · `✗` erreur · `✔` terminé · `✎` brouillon · `⚠` avertissement · `⌂` local · `▣` conteneur · `☁` distant · `🔒` verrouillé. 🅿N = phase.

## 0. Navigation cible

```
home ─┬─ Démarrer (★ épinglés · récents) ───────────────┐
      ├─ Sessions (● en cours · récentes) → sessions 🅿3 │
      ├─ Équipes → team.mode                             │   ┌──────────────────────────┐
      ├─ Projets → project.mode                          ├──▶│ Fiche de lancement        │
      └─ Système (settings · metrics · doctor · secrets  │   │ Entrées → Options → Récap │
                  · notifications · catalogue briques)   │   └────────────┬─────────────┘
project.mode / team.mode ─┬─ Démarrer ───────────────────┤                ▼
                          ├─ Sessions → sessions         │       RunWorkflow (⌂ ▣ ☁)
                          ├─ Board ─[a]→ workflows ──────┘                ▼
                          ├─ Workflows du contexte → workflows     sessions / À traiter
                          └─ Config (+ Exécution 🅿4)
workflows ─Enter→ workflow.edit ─p→ publication ─h→ historique    (🅿1 lecture, 🅿2 écriture)
Omnibar : run <wf> (+ alias) · sessions · workflows · catalogue · pin
Badge barre de mode : ● sessions en cours · ⏸ décisions en attente (🅿3)
```

## 1. Accueil hub (🅿1 bloc Démarrer ; 🅿3 Sessions + badge)

Changement **minimal** (D16) : structure, colonnes, Équipes, Projets, Système inchangés.

```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│   ___  ___ ___ _  _ _  _ _   _ ___                                                   │
│  / _ \| _ \ __| \| | || | | | | _ )        opencode v2.0.20 · adaptateur V2 ✔        │
│ ─ Démarrer                               ─ Projets                                   │
│ ▸ ★ ticket    Implémenter un ticket  é.v3  openhub   ~/workspace/openhub  main       │
│   ★ cadrage   Cadrage sans dev       h.v1  api       ~/workspace/api      dev        │
│   ─ Récents                                + Ajouter un projet                       │
│     review    Revue · il y a 2h      h.v1                                            │
│     … Tous les workflows (12)                                                        │
│ ─ Sessions                     ● 2  ⏸ 1                    (🅿3)                     │
│   ● ticket bd-42  openhub ⌂  ⏸ cp-2 en attente                                       │
│   ● review feat/x openhub ▣  reviewer                                                │
│ ─ Équipes                                                                            │
│   core  4 membres · 7 tickets actifs                                                 │
│ ─ Système                                                                            │
│   Réglages · Métriques · Doctor · Secrets · Notifications · Catalogue des briques    │
├──────────────────────────────────────────────────────────────────────────────────────┤
│ 🏠 Hub   ● 2 ⏸ 1                    * épingler · Ctrl+P omnibar · ? aide · Ctrl+Q    │
└──────────────────────────────────────────────────────────────────────────────────────┘
```
- Remplacé : section « Sessions » (Quick/Dev/Audit/Review) → « Démarrer ».
- Ajouté : section « Sessions » (🅿3), Notifications et Catalogue dans Système, version opencode.

## 2. Mode projet (🅿1, 🅿3)

```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│  OPENHUB  · ~/workspace/openhub · main                                               │
│  opencode v2.0.20 (V2) · exécution: ⌂ local ✔  ▣ conteneur ✔ colima  ☁ distant ✗     │
│ ─ Démarrer                               ─ Configuration                             │
│ ▸ ★ ticket   Implémenter un ticket  é.v3   Config projet (+ Exécution)               │
│   ─ Récents                                Workflows du projet (3)                   │
│     review   il y a 2h                     Worktrees                                 │
│   ▸ Développer (3)  ▸ Cadrer (2)         ─ Équipe core                               │
│   ▸ Qualité (3)     ▸ Connaissance (1)     Board équipe · Statut · Activité · Wiki   │
│ ─ Sessions du projet          ● 2 ⏸ 1    ─ Mode Hub                                  │
│ ─ Projet                                                                             │
│   Board Beads · Métriques · Statut                                                   │
├──────────────────────────────────────────────────────────────────────────────────────┤
│ 💻 openhub  ● 2 ⏸ 1               Enter lancer · * épingler · { } sections · Esc hub │
└──────────────────────────────────────────────────────────────────────────────────────┘
```
- Supprimé (🅿3) : section Déploiement, entrée Parallel, toast d'écart. En-tête : adaptateur + runtimes à la place de l'état de déploiement.
- « Workflow » → « Workflows du projet ».

## 3. Mode équipe (🅿1, 🅿2, 🅿3)

```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│  CORE  · 4 membres · 7 tickets actifs · team-state ✔ synchronisé il y a 3 min        │
│ ─ Démarrer                               ─ Board                                     │
│ ▸ ★ ticket · ★ cadrage-produit             Board équipe · Statut · Activité          │
│   ▸ Développer · Cadrer · Qualité          Résumés de reprise · Wiki                 │
│ ─ Sessions de l'équipe       ● 3 ☁ 1     ─ Configuration                             │
│   ● alice  ticket bd-50 ⌂ developer        Workflows de l'équipe (5 · 1 ✎)           │
│   ☁ ben    ticket bd-51 pipeline #812      Politiques · Patterns · Détail équipe     │
│                                          ─ Navigation  Projets · Mode Hub            │
└──────────────────────────────────────────────────────────────────────────────────────┘
```
Sessions des autres membres : lecture seule (claims team-state).

## 4. Fiche de lancement (🅿1 ; ▣ 🅿4 ; ☁ 🅿5)

Générée depuis le YAML (`inputs`, `modes`, `runtime`). `InlineWizardView` (`Ctrl+S` lancer, `Ctrl+B` retour, `Esc` annuler).

### 4.1 Étape 1 — Entrées
```
┌─ Lancer · ticket ─────────────────────────────── équipe core v3 ─────────────────────┐
│  ● Entrées ─── ○ Options ─── ○ Récap                                                  │
│  ticket *   [ bd-42  Ajouter l'export CSV                                    ▾ ]      │
│             Ticket Beads délégué à l'IA (ai-delegated)                                │
│  branch     [ feat/bd-42                                                       ]      │
├───────────────────────────────────────────────────────────────────────────────────────┤
│  Tab champ suivant · Enter ouvrir · Ctrl+S lancer avec défauts · Ctrl+B · Esc         │
└───────────────────────────────────────────────────────────────────────────────────────┘
```
Types → champs : `beads-id(s)` sélecteur Beads ; `string/text/int/path` champ ; `enum` liste ; `bool` case ; `branch` liste des branches.

### 4.2 Sélecteur Beads (tview, T2)
```
┌─ Choisir un ticket ──────────────────────────────────────────────────────────────────┐
│  / export█                                     filtre: ai-delegated ▾  épopée: tout ▾ │
│  ─ Épopée E-12 · Exports                                                              │
│  ▸ bd-42  Ajouter l'export CSV           P1  open        ai-delegated                │
│    bd-43  Export Excel                   P2  open        ai-delegated                │
│  ─ Épopée E-15 · Auth                                                                 │
│    bd-51  Refresh token                  P1  claimed ⚠ bob                            │
│ ┌ Aperçu ──────────────────────────────────────────────────────────────────────────┐ │
│ │ bd-42 · Ajouter l'export CSV — critères d'acceptation : 3                         │ │
│ └───────────────────────────────────────────────────────────────────────────────────┘ │
│  Enter choisir · Space multi-sélection (si multi) · Esc                               │
└───────────────────────────────────────────────────────────────────────────────────────┘
```

### 4.3 Étape 2 — Options
```
│  Mode          ( ) manuel   (●) semi-auto                  ← modes autorisés          │
│  Exécution     (●) ⌂ local                                                            │
│                ( ) ▣ conteneur  colima ✔ · image openhub-dev:9f3c en cache            │
│                ( ) ☁ distant    ✗ aucun runner configuré (Réglages › Distant)         │
│  Emplacement   (●) base (main → checkout feat/bd-42)                                  │
│                ( ) worktree existant  ../openhub-bd42 [active]                        │
│                ( ) + nouveau worktree                                                 │
│  Ouverture     auto (iTerm2 · onglet) ▾                                               │
```
Multi-sélection : « 5 sessions · 1 serveur · 5 worktrees » (🅿1/🅿3).

### 4.4 Étape 3 — Récap
```
│  Agents (4)     orchestrator-dev (entrée) · developer · reviewer · documentarian      │
│  Skills         23 · ~14k tokens          MCP     gitlab        Code Mode  off        │
│  Beads          show · update · close · list · children                               │
│  Checkpoints    cp-1 auto (semi-auto) · cp-2 ⏸ obligatoire                            │
│  Isolation      monde fermé ✔ full (opencode V2)                                      │
│  ⚠ Le dépôt contient des modifications non commitées  [s] stash  [w] worktree         │
│  ⚠ 3 sessions demandent déjà une action                                               │
```
Préparation (non bloquante) puis toast de fin :
```
┌──────────────────────────────────────────────┐    ┌─────────────────────────────────────┐
│ ⠋ Préparation de la session ticket            │    │ ✔ Session ticket terminée            │
│   ✔ Paquet construit (9f3c…)                  │    │   bd-42 · 38 min · $1.12             │
│   ✔ Serveur prêt (127.0.0.1:47213)            │    │   cp-0 ✔ cp-1 ✔ cp-2 ✔ (vous)        │
│   ⠋ Vérification de l'isolation…              │    │   [Enter] Enchaîner avec… (review)   │
└──────────────────────────────────────────────┘    └─────────────────────────────────────┘
```
Échec d'isolation :
```
│ ✗ Isolation non respectée : agent visible hors paquet → « scout »   [d] Diagnostic   │
```

## 5. Omnibar (🅿1)
```
 > tick█
   ★ run ticket         Implémenter un ticket Beads       Workflows · équipe v3
     run ticket ⟨bd-42⟩ Sur le ticket sélectionné         Contexte (board)
     dev → run ticket   (alias)                           Workflows
     sessions           Sessions en cours (● 2 ⏸ 1)       Navigation
```

## 6. Board : actions rapides (🅿1)
```
┌─ Lancer sur bd-42 · Ajouter l'export CSV ────────────────────────────────────────────┐
│  ▸ ★ ticket   Implémenter                      write  ⌂ ▣ ☁                           │
│      review   Revue de la branche du ticket     read   ⌂ ☁                            │
│      debug    Diagnostiquer                     write  ⌂                              │
│      audit    Auditer (type : à choisir)        read   ⌂ ☁                            │
│  Enter → fiche (ticket prérempli, ouverte à l'étape Options) · Esc                    │
└───────────────────────────────────────────────────────────────────────────────────────┘
```

## 7. Sessions, À traiter, décisions, flux (🅿3)

### 7.1 Vue Sessions
```
┌─ Sessions ─────────────────────────────── filtre: toutes ▾ · projet: tous ▾ ──────────┐
│ ─ À traiter (3)                                                                        │
│ ▸ ⏸ ticket bd-42  openhub ⌂  cp-2 « Commit ou correction »          il y a 2 min      │
│   ? cadrage —     openhub ⌂  « Périmètre MVP ? »                     il y a 5 min      │
│   ! review feat/x api    ▣  shell `npm run e2e`                      il y a 7 min      │
│ ─ En cours                                                                             │
│   ● review   feat/x  openhub  ▣  developer → reviewer               4m   $0.21         │
│   ◌ ticket   bd-51   api      ☁  pipeline #812 running             18m   —             │
│ ─ À récupérer                                                                          │
│   ✔ ticket   bd-39   api      ☁  MR !214 prête · journal Beads 3 op.                   │
│ ─ Terminées (7 j)                                                                      │
│   ✔ cadrage  —       openhub  ⌂  5 tickets créés                         $1.10         │
│ ┌ Détail · ticket bd-42 ──────────────────────────────────────────────────────────────┐│
│ │ équipe core v3 · paquet 9f3c… · semi-auto · ⌂ · ../openhub-bd42 · feat/bd-42 · 1 cl.││
│ │ ✔ cp-0 10:02 → ✔ cp-1 10:03 → developer (3) → reviewer → ⏸ cp-2                      ││
│ └──────────────────────────────────────────────────────────────────────────────────────┘│
├────────────────────────────────────────────────────────────────────────────────────────┤
│ Enter décider · y valider · n refuser · a attacher · t suivre · m consigne · i interr. │
│ M modèle · s arrêter · c reprendre · o MR · w navigateur · g récupérer · x nettoyer    │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

### 7.2 Fiche de décision — checkpoint
```
┌─ ⏸ cp-2 · Commit ou correction · ticket bd-42 ──────────────────────────────────────┐
│ Le reviewer a terminé : 0 bloquant · 2 mineurs                                       │
│ ─ Changements (session.diff)       +142 −18 · 6 fichiers          [d] diff complet   │
│ ─ Derniers messages                                                                   │
│   reviewer › « 2 remarques mineures : nommage, test de bord manquant »               │
│ ─ Décision                                                                            │
│   (●) Commit   ( ) Corriger d'abord les mineurs   ( ) Autre consigne…                 │
│   Message à l'agent [                                                         ]       │
│                          [ Valider ]   a attacher · w navigateur · Esc                 │
└──────────────────────────────────────────────────────────────────────────────────────┘
```
Question = formulaire généré depuis `form.fields` (options + réponse libre si `custom`). Permission = action, ressource, agent ; « Toujours » désactivé si `isolation: strict`. Résolu ailleurs → « résolu dans l'interface / le navigateur ».

### 7.3 Flux en direct (`t`)
```
┌ Sessions ───────────────────────┬ Flux · ticket bd-42 ─────────────────── $0.84 ─┐
│ ▸ ● ticket bd-42  ⌂ developer   │ 10:14 developer › edit src/export/csv.go         │
│   ● ticket bd-43  ⌂ reviewer    │ 10:14 developer › shell go test ./export/... ✔   │
│   ⏸ ticket bd-44  ▣ cp-1        │ 10:15 → reviewer                                 │
└─────────────────────────────────┴ m consigne · i interrompre · a attacher ───────┘
```

### 7.4 Récupération distante et conflits Beads (🅿5)
```
┌─ Récupérer · ticket bd-39 (pipeline #808) ──────────────────────────────────────────┐
│  ✔ Artefacts · ✔ Session importée [a] · ⠋ Rejeu du journal (3 op.)                   │
│     ✔ bd-39 status open → review  ✔ bd-39 note  ⚠ bd-40 status : conflit             │
└───────────────────────────────────────────────────────────────────────────────────────┘
┌─ Conflit Beads · bd-40 ───────────────────────────────────────────────────────────────┐
│  status   in_progress (alice, 11:20)  |  done (session ☁ #808, 11:05)                  │
│  [l] Garder local   [r] Appliquer distant   [m] Fusionner les notes   [Esc] Plus tard  │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

### 7.5 Fermeture de oh (I7)
```
┌─ Quitter oh ──────────────────────────────────────────────────────────────────────────┐
│ 2 sessions travaillent encore :                                                        │
│  ● ticket bd-42  developer   (●) Finir l'étape puis veille  ( ) Arrière-plan  ( ) Stop │
│  ● review feat/x reviewer    (●) Finir l'étape puis veille  ( ) Arrière-plan  ( ) Stop │
│ 1 session en attente (⏸) et 1 inactive → mises en veille maintenant                    │
│                                      [ Quitter ]   Esc annuler                          │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

## 8. Catalogue des workflows (🅿1 lecture ; 🅿2 écriture)
```
┌─ Workflows ────────────────────── équipe core · projet openhub ── / chercher ─────────┐
│ ─ Hub · intégrés (lecture seule)                                                       │
│   feature  v2 write ⌂        ticket v2 write ⌂▣☁      [e] étendre [c] dupliquer       │
│ ─ Équipe core                                                                          │
│ ▸ ★ ticket       v3  write  ⌂ ▣ ☁  ↳ étend hub:ticket        publié  ✔ intègre          │
│   cadrage-produit v1 read   ⌂                                publié  ✔                  │
│ ─ Projet openhub                                                                       │
│   ticket         v1  patch      ↳ surcharge équipe:ticket   publié  ⚠ modifié hors hub  │
│ ─ Mes brouillons                                                                       │
│   ✎ release      —   write  ⌂                                ✗ 2 erreurs                │
├────────────────────────────────────────────────────────────────────────────────────────┤
│ Enter éditer · n nouveau · v valider · t tester · p publier · D diff · h historique · * │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

## 9. Éditeur (🅿2)
Sections : **Général** (description, catégorie, extends, risque, entrée, modes, exécution, isolation, code mode, coupe-circuit, modèles — chaque champ avec origine/verrou) · **Graphe** (`WorkflowGraph` : checkpoints avec comportement par mode et `remote`, agents avec rôle ; appels autorisés déduits) · **Entrées & prompt** (liste des entrées ; gabarit ouvert dans `$EDITOR` ; aperçu calculé avec valeurs d'exemple ; BL-1 pour l'éditeur intégré) · **Ressources** (skills +/−, MCP, Beads, plugins, sorties) · **Aperçu du paquet** (agents, fermeture des skills + budget, permissions générées, isolation par adaptateur, diagnostics cliquables).
Raccourcis : `Tab` section · `Enter` éditer · `u`/`U` annuler/rétablir · `y` YAML `$EDITOR` · `w` enregistrer (brouillon) · `Esc` (garde si modifié).

```
┌─ ✎ équipe:ticket-hotfix ──────────────────────────────────────────────────────────────┐
│ Général [Graphe] Entrées & prompt  Ressources  Aperçu paquet                           │
│   ┌────────────────┐    ◆ cp-1      ┌───────────┐    ┌──────────┐    ◆ cp-2            │
│   │orchestrator-dev│──▶ Démarrer ──▶│ developer │──▶ │ reviewer │──▶ Commit/corr. ⏸    │
│   └────────────────┘    auto/auto   └───────────┘    └──────────┘    remote: defer     │
│           └┄┄▶ documentarian (indépendant)                                             │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

## 10. Publication et historique (🅿2)
```
┌─ Publier · équipe:ticket v3 → v4 ─────────────────────────────────────────────────────┐
│ ✔ Validation OK                                                                        │
│ ─ Impact   ⚠ distant autorisé (nouveau) · ⚠ Beads « create » ajouté · ✔ risque inchangé│
│ ─ Diff     runtime.allowed + remote · beads.allow + create · cp-2.remote → defer        │
│ Message  [ Autorise le distant pour les tickets détaillés              ]               │
│                          [ Publier ]  Esc · publication : tout membre                  │
└────────────────────────────────────────────────────────────────────────────────────────┘
┌─ Historique · équipe:ticket ──────────────────────────────────────────────────────────┐
│ ▸ v4  ben    05/10 14:12  Autorise le distant…  (actuelle)                             │
│   v3  alice  02/10 09:40  Coupe-circuit à 8          Enter diff · r restaurer · Esc    │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

## 11. Config projet, Réglages, Doctor, Catalogue des briques
- **Config projet** (🅿3/🅿4) : sections Général · Équipe · Tracker · **Exécution** (Dockerfile dev, build args, volumes, workflow/runtime par défaut) · Liens (Modèles, MCP, Workflows du projet). Lien Agents **supprimé**.
- **Réglages** : Ouverture des sessions (`auto|iterm|terminal|tmux|browser|suspend`, style iTerm) 🅿0 · Veille (min, défaut 5) 🅿0 · Restrictions (désactivées par défaut) 🅿3 · Exécution (runtime, moteur, cache, opencode figé, isolation stricte) 🅿4 · Distant (projet runner, registre, assistant) 🅿5.
- **Doctor** : opencode + adaptateur + isolation · autorisation Automation macOS · proxy · git ≥ 2.48 · moteur conteneur, virtiofs, keep-id · intégrité `workflows.lock` · restes de déploiement → `migrate deploy-cleanup` · runner distant.
- **Catalogue des briques** (🅿3, remplace `project.agents`) : agents et skills, origine (hub/équipe/communauté), `requires`, workflows qui les utilisent, coût estimé.

## 12. Assistants
- Premier lancement : Langue → opencode (version + adaptateur) → Conteneurs (détection) → Projet → **Espace de workflows** (équipe existante | solo) → MCP → Fin. **Étape Agents supprimée.**
- Ajout de projet : Chemin → Nom → Équipe (ou solo) → Dockerfile dev (détecté) → Provider → MCP.
- Espace solo, « Passer en équipe » (🅿2).
- Nettoyage des anciens déploiements (🅿3) :
```
┌─ Nettoyage des anciens déploiements ──────────────────────────────────────────────────┐
│  openhub   − .opencode/agents (19)  − .opencode/skills (64)  − .deploy-state           │
│            opencode.json : − agent.*  − plugin  − compaction  (clés écrites par oh)    │
│            conservé : vos clés personnelles (provider.litellm)                         │
│                     [ Nettoyer ]  [ Voir le diff ]  [ Plus tard ]                      │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

## 13. Éléments retirés
| Retiré | Remplacé par |
|---|---|
| Blocs « Sessions » codés en dur | « Démarrer » |
| `translateOhFlags`, sous-menus audit/review | Entrées des workflows |
| Section Déploiement, `deploy`/`sync` omnibar, toast d'écart | Paquet au lancement, `oh bundle show` |
| `project.agents`, étapes Agents | Catalogue des briques |
| Vue `parallel` vide | Sessions |
| Vue `workflow` embryon | Catalogue + Éditeur |
| huh pendant la TUI | Sélecteurs tview |
| Suspension par défaut | iTerm2 / Terminal.app (suspension en dernier recours) |

## 14. Tests TUI
`mockShell` (`views_test.go:19-43`) ; montage/démontage ; `SimulationScreen` pour interactions ; golden de la fiche générée à partir de YAML ; parité i18n.
