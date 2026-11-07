> Brouillon — à relire avant d'écrire dans `docs/architecture/adr/061-ui-contract-architecture.fr.md` (et sa version EN)

# ADR-061 — Architecture des interfaces : contrat, services applicatifs et formulaires

## Statut

Proposé

## Date

2026-11-05

## Contexte

oh n'a qu'une interface interactive, le TUI (`internal/tui`, tview et tcell). On veut ajouter une interface web locale et une app macOS, pour les personnes peu à l'aise avec le terminal, puis pour de futurs utilisateurs non-devs. Le TUI reste, et les interfaces doivent cohabiter.

Un audit du 05/11/2026 a fait ces constats :

- **Les vues ne peuvent pas être réutilisées** : l'interface `views.View` dépend de tview (`Mount(*tview.Flex, *tview.Application)`, `HandleKey(*tcell.EventKey)`), avec environ 1 000 références à tview et tcell dans `internal/tui/v2/views`.
- **La logique métier est déjà séparée** : `archtest/layers_test.go` interdit à `services`, `runsvc`, `daemon` et `domain` d'importer le TUI.
- **Une partie des écrans passe déjà par des interfaces**, qui font office d'API : `SessionsBackend`, `CheckpointBackend`, `PolicyService`, `TeamTicketSource`, `BeadsSource`, `SessionBatcher`, `SessionRemote`… (environ 20). Mais leurs types (`SessionRow`, `SessionDecision`, `SelectOption`…) vivent dans `views`, qu'une autre interface ne peut pas importer sans tirer tview.
- **Le reste passe par environ 250 fonctions** dans des structures de configuration (`ProjectsViewConfig`, `TeamDetailViewConfig`…). Leurs implémentations sont dans `cmd/tui_*.go` (environ 10 700 lignes), et lisent la variable globale `tuiShell` et le contexte actif (`ActiveProject`, `ActiveTeam`, `Mode`).
- **Certaines renvoient du texte déjà traduit ou mis en forme** pour l'affichage : `StateLabel`, `StateIcon`, `Timeline` rendue, message de `DeleteSession`.
- **Les 57 étapes d'assistant** écrites dans `cmd` (102 références dans 23 fichiers) (`views.WizardStep`) fabriquent des formulaires tview (`Form func(*tview.Application, func()) *tview.Form`, `CustomView`). Elles servent au TUI et à la ligne de commande (`runCLIWizard`).

Sans couche commune, chaque fonctionnalité serait écrite deux ou trois fois, alors que le TUI évolue vite (568 commits sur 3 mois).

## Décision

### 1. Trois paquets neutres

| Paquet | Rôle | Ne doit pas importer |
|---|---|---|
| `internal/ui/contract` | Interfaces par domaine, types de données, codes d'erreur, événements, capacités du client | `internal/tui`, `cmd`, tview, tcell, `net/http` |
| `internal/appsvc/<domaine>` | Implémentations du contrat, sorties de `cmd/tui_*.go` ; elles s'appuient sur `services`, `runsvc`, `daemon/client`, etc. | `internal/tui`, `cmd`, tview, tcell, `net/http` |
| `internal/ui/forms` | Schéma des formulaires et des assistants, et machine à états `WizardSession` | `internal/tui`, `cmd`, tview, tcell, `net/http` |

Le TUI, le serveur d'interface ([ADR-062](adr-062-ui-server.fr.md)) et son client HTTP dépendent de ces paquets, jamais l'inverse. `cmd` reste la racine de composition : il construit les `appsvc` à partir d'`app.App` (dans `cmd/service_wiring.go` uniquement).

**La CLI passe aussi par `appsvc`.** Les commandes `oh session …`, `oh claim …`, `oh project …`, etc. appellent les mêmes services que le TUI ; seule leur mise en forme (tableaux, JSON de sortie) reste dans `cmd`. C'est ce qui supprime le code écrit deux fois, relevé par l'inventaire : liste et contrôle des sessions, regroupement du board (écrit à la fois dans `serve_api.go` et dans `views/board_data.go`), prise de ticket, repli des secrets, contexte git des politiques, etc.

**L'affichage propre au TUI va dans `internal/tui/v2/present`** : libellés d'état, icônes, timeline, badge, texte des messages et des erreurs à partir de leur code. Il reprend ce que le branchement calcule aujourd'hui (`stateLabel`, `StateIcon`, `KindIcon`, `TimelineText`, `fanoutLine`, `SessionsBadge`).

**La portée sort du shell.** `ResolveTeam` (`cmd/team_resolve.go`) lit aujourd'hui le mode et l'équipe active du TUI (`tuiShell.Mode()`, `ActiveTeam()`) : un résultat métier qui dépend de l'état de l'interface. Il devient une fonction pure de `appsvc/scope`, qui prend un `Scope` construit par le client.

### 2. Règles du contrat

1. **Uniquement des données sérialisables.** Des structures avec des tags JSON, sans fonction ni canal dans les structures, sans type tview. Les dates sont des `time.Time` (RFC 3339 en JSON).
2. **Données brutes, pas de mise en forme.** Le contrat donne un état (`state: "waiting"`), pas un texte traduit ni une icône. Chaque client calcule son affichage (`StateLabel`, `StateIcon`, timeline). Une liste d'exceptions gelée accepte les textes rédigés par oh qui n'ont pas d'équivalent structuré (rapports, descriptions de MR) : ils sont alors renvoyés comme du contenu (Markdown), pas comme un message d'interface.
3. **Contexte explicite.** Chaque appel reçoit un `contract.Scope{Kind: hub|project|team, ProjectID, TeamID}`. `appsvc` ne lit jamais le contexte actif d'un shell.
4. **Erreurs et messages avec un code stable.** `contract.Error{Code, Params}`, par exemple `override_required` ou `provider_credentials`. Les erreurs sentinelles actuelles (`views.ErrOverrideRequired`, `views.ErrProviderCredentials`, `config.ErrConfigConflict`…) y correspondent. Les retours d'action affichés en message sont des `contract.Message{Code, Params, Level}`, et non des chaînes déjà traduites (`DeleteSession`, résumé de synchronisation, `TestMCP`…).
5. **Un seul flux d'événements.** `Events(ctx, Scope) (<-chan contract.Event, error)`. Un événement a un numéro de séquence, un type (`sessions.changed`, `decision.created`, `config.changed`, `team.changed`, `notification`…), un `Scope` et des données. Il remplace les `Changes()` propres à chaque domaine, les crochets de synchronisation (`views.teamSyncHook`) et les relectures périodiques. Prérequis : le flux de `ohd` (`/v1/stream`) reçoit lui aussi un numéro de séquence et une reprise, puisqu'il perd aujourd'hui des événements quand un client est lent.
6. **`appsvc` ne navigue jamais et n'affiche rien.** Il renvoie un résultat. Les actions qui ouvrent un écran (`Chain`, `Open`) renvoient une **intention** (`contract.Intent{Kind: "open_launch", Params}`) que le client applique.
7. **Capacités du client.** `contract.ClientCaps{Terminal, Suspend, Browser, Editor}` est passé aux actions qui en dépendent : `Attach` ouvre un onglet de terminal pour le TUI, opencode dans le navigateur (mode `pair`) pour le web. Ce qui suspend l'affichage (`SuspendAndExec`, éditeur) reste dans le client, ou se traduit par une intention.
8. **Pas d'état caché.** Ce que les fonctions de branchement gardent en mémoire (`tuiRunCache`, préparation d'un lancement) devient un objet du contrat avec un identifiant (`PrepareLaunch → LaunchDraft{ID}`, `Launch(draftID)`).
9. **Les flux sont des canaux** (`<-chan`) fermés à l'annulation du contexte. En HTTP, ils deviennent des flux SSE ([ADR-062](adr-062-ui-server.fr.md)).

### 3. Exemple : le domaine Sessions

Aujourd'hui (`internal/tui/v2/views/sessions_types.go`) :

```go
type SessionsBackend interface {
    List(ctx, projectID string, all bool) ([]SessionRow, error)
    Changes(ctx) <-chan struct{}
    Follow(ctx, sessionID string) (<-chan SessionFeedLine, error)
    Decide(ctx, decisionID, choice, message string, answers map[string]string) error
    Send / Interrupt / SwitchModel / Stop / Resume ...
    Attach(sessionID, how string)
    OpenBrowser(ctx, sessionID string) (string, error)
    MRDescription(ctx, sessionID string) (string, error)
}
```

Demain (`internal/ui/contract/sessions.go`) :

```go
type Sessions interface {
    List(ctx context.Context, scope Scope, q SessionQuery) ([]Session, error)
    Get(ctx context.Context, id string) (SessionDetail, error)
    Follow(ctx context.Context, id string) (<-chan FeedLine, error)
    Decide(ctx context.Context, decisionID string, d DecisionAnswer) error
    Send(ctx context.Context, id, text string) error
    Interrupt(ctx context.Context, id string) error
    SwitchModel(ctx context.Context, id, model string) error
    Stop(ctx context.Context, id string) error
    Resume(ctx context.Context, id string) error
    Attach(ctx context.Context, id string, caps ClientCaps, how string) (AttachResult, error)
    Results(ctx context.Context, id string) (SessionResults, error) // diff, coût, MR (Markdown)
    NextSteps(ctx context.Context, id string) ([]NextStep, error)
    PlanDelete(ctx context.Context, id string) (DeletePlan, error)
    Delete(ctx context.Context, id string, discardBase bool) (DeleteResult, error)
}

type Session struct {
    ID        string     `json:"id"`
    ProjectID string     `json:"project_id"`
    Title     string     `json:"title"`
    Workflow  string     `json:"workflow"`
    Agent     string     `json:"agent"`
    Mode      string     `json:"mode"`
    Runtime   string     `json:"runtime"`
    Location  string     `json:"location"`
    State     string     `json:"state"` // active, waiting, idle, sleeping, completed, failed, stopped
    CostUSD   float64    `json:"cost_usd"`
    Started   time.Time  `json:"started"`
    Changed   time.Time  `json:"changed"`
    Decisions []Decision `json:"decisions"`
    Fanout    *FanoutRef `json:"fanout,omitempty"`
    Remote    *RemoteRef `json:"remote,omitempty"`
    Next      string     `json:"next,omitempty"`
}

type Decision struct {
    ID        string          `json:"id"`
    SessionID string          `json:"session_id"`
    Kind      string          `json:"kind"` // permission, question, checkpoint, budget, error, circuit
    Summary   string          `json:"summary"`
    Message   string          `json:"message"`
    Created   time.Time       `json:"created"`
    Fields    []DecisionField `json:"fields,omitempty"`
    Approver  string          `json:"approver,omitempty"`
}

type DecisionAnswer struct {
    Choice  string            `json:"choice"` // once, always, reject, dismiss, <choix du checkpoint>
    Message string            `json:"message,omitempty"`
    Answers map[string]string `json:"answers,omitempty"`
    Override bool             `json:"override,omitempty"` // après le code override_required
}
```

Les événements remplacent `Changes()` : `sessions.changed{scope}`, `decision.created{session_id, decision_id}`, `decision.resolved{…}`. Le TUI garde ses fonctions d'affichage (`StateLabel`, icônes, timeline), calculées à partir de `State` et des données brutes.

### 4. Formulaires décrits par des données (`internal/ui/forms`)

- Une étape devient une **description** : identifiant, titre, champs (`text`, `secret`, `select`, `multiselect`, `confirm`, `path`, `readonly`), options (statiques ou calculées), valeurs par défaut, champ obligatoire, saut conditionnel (`SkipIf`), retour en arrière en sautant l'étape (`BackSkip`), message pendant le traitement, informations du panneau latéral (`InfoFields`).
- Les **validations et les effets restent côté serveur** (`Validate`, `OnDone`) : le client ne reçoit que le schéma.
- Une **machine à états** `WizardSession` : `Start(wizardID, scope) → Step` ; `Submit(stepID, answers) → Next | Problem | Done` ; `Back()` ; `Cancel()`.
- Deux moteurs d'affichage : tview (TUI et ligne de commande, qui remplace l'intérieur d'`InlineWizardView`) et web ([ADR-063](adr-063-web-desktop-technologies.fr.md)).
- Pendant la migration, `WizardStep.Form` (tview) reste accepté, avec une liste d'exceptions `archtest` gelée. Les étapes au rendu vraiment particulier (éditeur de colonnes) deviennent des écrans dédiés du contrat.

### 5. Migration progressive

Domaine par domaine, en branches courtes : Sessions → Lancement → Projets → Équipe → Wiki → Workflows → Administration → Transverse. Pour chaque domaine :

1. déplacer les types vers `contract`, avec des alias provisoires dans `views` ;
2. déplacer l'interface ;
3. déplacer l'implémentation vers `appsvc`, sans `tuiShell` ;
4. remplacer la mise en forme par des données brutes ;
5. tests au vert ;
6. supprimer les alias.

### 6. Garde-fous (`internal/archtest`)

- `ui/contract`, `appsvc` et `ui/forms` n'importent ni `internal/tui`, ni `cmd`, ni tview ou tcell, ni `net/http`.
- Pas de type `func` ni `chan` dans les champs des structures de `ui/contract`.
- Taille du branchement dans `cmd/tui_*.go` gelée : elle ne peut que diminuer.
- Liste gelée des `WizardStep.Form` restants : elle ne peut que diminuer.

### 7. Langue

**À trancher.** Option A : le contrat renvoie des codes et des paramètres, et chaque client traduit à partir des mêmes fichiers `fr.json`/`en.json`. Option B : le serveur traduit selon la langue du client (`Accept-Language`), ce qui suppose une fonction de traduction par appel plutôt que la langue globale actuelle d'`i18n`. Recommandation : A pour les messages d'interface, et B seulement pour les contenus rédigés (rapports).

## Conséquences

### Positives

- Une fonctionnalité métier s'écrit une seule fois ; seul l'affichage est propre à chaque interface.
- Le web, l'app macOS et une future app Tauri se branchent sur le même contrat ; le passage au client/serveur ([ADR-062](adr-062-ui-server.fr.md)) devient une seconde implémentation du contrat.
- La ligne de commande profite des formulaires décrits par des données (assistants de `oh init`, `oh provider setup`…).
- La variable globale `tuiShell` et le contexte implicite disparaissent ; le code de branchement devient testable sans terminal.

### Négatives / Compromis

- Chantier long (N1 : 14 à 18 semaines, N2 : 8 à 10 semaines pour 1 personne), mené pendant que le TUI évolue : risque de conflits, à limiter par des branches courtes.
- Le TUI doit recalculer ce que le branchement lui donnait tout prêt (libellés, icônes, timeline).
- Les assistants très interactifs perdent un peu de liberté : ce qui ne rentre pas dans le schéma devient un écran dédié.
- Des alias provisoires dans `views` pendant la migration.

## Alternatives considérées

| Alternative | Rejetée car |
|---|---|
| Garder les interfaces dans `views` et les exposer en HTTP depuis `cmd` | Le web tirerait tview, et la logique resterait mêlée à `tuiShell`. |
| Réécrire chaque écran pour le web en appelant directement `services` | La logique de branchement serait écrite deux fois, avec le rythme actuel du TUI. |
| Faire tourner le TUI dans un terminal web (xterm.js) | Rapide, mais ne répond pas au besoin des non-devs : l'interface reste celle d'un terminal. |
| Garder les formulaires en tview, et refaire chaque assistant à la main pour le web | Les étapes à écrire deux fois, et des validations dupliquées. |
