> [Read in English](tui-inline-wizard.en.md)

# Référence — InlineWizardView

> Composant TUI réutilisable pour les wizards multi-step dans le shell OpenHub.

## Vue d'ensemble

`InlineWizardView` est une vue (`views.View`) qui implémente un wizard multi-step
directement dans le shell TUI, sans créer de `tview.Application` séparé. Il est pushé
sur le router stack via `shell.PushView(v)` et se comporte comme n'importe quelle autre
vue du shell.

Le composant réutilise le type `WizardStep` (`wizard.go`) pour la compatibilité
des définitions de steps entre le CLI standalone et le TUI inline.

**Fichier source** : `cli/internal/tui/v2/views/inline_wizard.go`

---

## API

### `InlineWizardConfig`

```go
type InlineWizardConfig struct {
    ID                     string                          // identifiant unique (ex: "wizard.team.init")
    Title                  string                          // titre pour le breadcrumb shell
    Steps                  []WizardStep                    // étapes du wizard
    OnComplete             func(completed bool, err error) // callback de fin
    SummaryTargetView      string                          // vue cible après le résumé (ex: "team.detail")
    SummaryTargetLabel     string                          // label du lien (ex: "Voir la config équipe")
    SummaryTargetViewFunc  func() string                   // variante calculée au rendu du résumé (prioritaire)
    SummaryTargetLabelFunc func() string                   // idem pour le label
    Groups                 []StepGroup                     // non vide : disposition « groupée » (voir ci-dessous)
}
```

### `WizardStep`

Type partagé avec `RunWizard` (`wizard.go`) :

| Champ | Rôle |
|-------|------|
| `ID` | Identifiant facultatif (mise à jour des libellés par nom) |
| `Label` | Libellé dans la barre d'étapes et le panneau d'infos |
| `Form` | Construit un `*tview.Form` ; doit appeler `onDone()` à la validation |
| `CustomView` | Contenu libre dans le conteneur fourni (prioritaire sur `Form`) |
| `Validate` | Appelé avant `OnDone` ; un texte non vide bloque avec ce message |
| `OnDone` | Effets de bord (écritures, appels), exécuté avec le spinner |
| `Processing` | Message du spinner pendant `OnDone` |
| `InfoFields` | Paires clé/valeur ajoutées au panneau d'infos après succès |
| `Skip` / `SkipIf` | Étape déjà satisfaite / condition évaluée juste avant le rendu |
| `Required` | Interdit de passer l'étape avec `Esc` (il reste `Ctrl+C` pour quitter) |
| `SidebarHidden` | Exclut l'étape de la barre latérale en mode groupé (pages d'intro) |

### Mode groupé (`Groups`)

Avec `Groups` (`StepGroup{Label, StartIdx}`), le contenu est centré, le panneau d'infos passe à droite et la barre d'étapes affiche les groupes au lieu des étapes. Utilisé par l'assistant du premier lancement (`oh init` : Langue, Provider, Équipe).

### Constructeur

```go
func NewInlineWizardView(cfg InlineWizardConfig) *InlineWizardView
```

### Injection shell

`InlineWizardView` implémente l'interface implicite `shellAware` (`SetShell(ShellAccess)`).
L'injection est automatique quand le wizard est pushé via `shell.PushView(v)`.

---

## Layout

```
┌─────────────────────────────────────────────────────┐
│ ● Dépôt ─── ◆ Config ─── ○ Identité ─── ○ Notifs  │  StepBar (1 ligne, fixe)
├─────────────────────────────────────────────────────┤
│ ◆ 2/5 — Configuration globale                      │  Header step (2 lignes, fixe)
├─────────────────────────────────────────────────────┤
│                                                     │
│ [Form / CustomView / Spinner]                       │  Contenu (extensible, weight=1)
│                                                     │
├─────────────────────────────────────────────────────┤
│ ── Résumé ──                                        │
│ ✓ Dépôt: git@gitlab.com:team/state.git             │  Info panel (hauteur dynamique)
│ ✓ Config: stale_days=3                              │
├─────────────────────────────────────────────────────┤
│ ctrl+s valider · ctrl+b retour · esc passer         │  Hints bar (1 ligne, fixe)
└─────────────────────────────────────────────────────┘
```

Widgets utilisés (tous existants dans le design system) :

| Widget | Source | Usage |
|--------|--------|-------|
| `widgets.StepBar` | `stepbar.go` | Barre de progression horizontale |
| `widgets.StatusBar` | `stepbar.go` | Hints contextuels en bas |
| `widgets.Spinner` | `spinner.go` | Indicateur pendant les opérations async |
| `tview.TextView` | tview | Header step, info panel |
| `tview.Form` | tview | Formulaires des steps |
| `tview.Flex` | tview | Layout principal et zone de contenu swappable |

---

## Cycle de vie

### Mount

1. Initialise les step states (`StepPending`, `StepDone` pour les `Skip: true`)
2. Trouve le premier step non-skip, le marque `StepActive`
3. Construit le layout (stepBar + header + content + infoPanel + hintsBar)
4. Rend le premier step actif

### HandleKey

- Délègue au form/customview du step actif (via `InputCapture` sur le form)
- En mode résumé : `Enter` → naviguer vers `SummaryTargetView`, `Esc` → pop

### Unmount

- Stop le spinner si actif
- Si le wizard n'est pas terminé, appelle `OnComplete(false, nil)` (abort)
- Nil-out toutes les références tview

---

## Navigation entre steps

| Action | Déclencheur | Comportement |
|--------|-------------|-------------|
| **Valider** | Bouton form / `Ctrl+S` | Validate → Spinner → OnDone → avancer |
| **Retour** | `Ctrl+B` | Reset step actuel → Pending, précédent → Active, pop info |
| **Passer** | Double `Esc` | 1er Esc = hint, 2ème = skip (bloqué si `Required`) |
| **Quitter** | `Esc` au shell | Pop le wizard (retour vue précédente) |

---

## Créer un nouveau wizard

### Exemple minimal

```go
func actionMyWizard() {
    a := MustApp()

    // État partagé entre les steps (capturé par closures)
    var name, email string

    steps := []views.WizardStep{
        {
            Label:    "Identité",
            Required: true,
            Form: func(_ *tview.Application, onDone func()) *tview.Form {
                form := tview.NewForm()
                form.AddInputField("Nom", "", 0, nil, func(t string) { name = t })
                form.AddInputField("Email", "", 0, nil, func(t string) { email = t })
                form.AddButton("Suivant", func() { onDone() })
                return form
            },
            Validate: func() string {
                if name == "" { return "Nom requis" }
                return ""
            },
            OnDone: func() error {
                return saveUser(name, email) // opération async
            },
            InfoFields: func() []views.InfoField {
                return []views.InfoField{
                    {Label: "Nom", Value: name},
                    {Label: "Email", Value: email},
                }
            },
            Processing: "Enregistrement...",
        },
        {
            Label: "Confirmation",
            CustomView: func(app *tview.Application, container *tview.Flex, onDone func()) {
                tv := tview.NewTextView().SetDynamicColors(true)
                tv.SetText("Tout est prêt ! Appuyez sur Enter.")
                tv.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
                    if e.Key() == tcell.KeyEnter { onDone(); return nil }
                    return e
                })
                container.AddItem(tv, 0, 1, true)
                app.SetFocus(tv)
            },
        },
    }

    wizard := views.NewInlineWizardView(views.InlineWizardConfig{
        ID:                 "wizard.my.feature",
        Title:              "Mon wizard",
        Steps:              steps,
        SummaryTargetView:  "settings",
        SummaryTargetLabel: "Voir les paramètres",
        OnComplete: func(completed bool, err error) {
            if completed && err == nil {
                // Rafraîchir l'état de l'application
            }
        },
    })

    tuiShell.PushView(wizard)
}
```

### Patterns importants

**État partagé via closures** : les variables partagées entre steps sont déclarées avant les
steps et capturées par les closures `Form`, `OnDone`, `Validate`, `InfoFields`. C'est le
même pattern que `RunWizard` dans `team.go`.

**Pré-remplissage dynamique** : le `OnDone` d'un step peut modifier les valeurs par défaut
des steps suivants. Le `Form` callback est appelé au moment du rendu (pas à l'init), donc
il voit les valeurs mises à jour.

**Skip conditionnel** : `SkipIf` est évalué dynamiquement juste avant le rendu du step.
Un step peut devenir skipable en fonction d'un choix fait dans un step précédent.

**Processing-only** : un step sans `Form` ni `CustomView` lance directement le spinner
+ `OnDone`. Utile pour les étapes de traitement pur (extraction, construction d'un paquet).

### Intégration omnibar

```go
// Dans tui_commands.go
commands = append(commands, shell.Command{
    ID:          "my.wizard",
    Label:       "Mon wizard",
    Aliases:     []string{"wizard"},
    Description: "Lancer le wizard de configuration",
    Category:    "Configuration",
    Action:      actionMyWizard,
})
```

---

## Types de steps

### Form (le plus courant)

Le callback `Form` reçoit `*tview.Application` et `onDone func()`. Il construit un
`*tview.Form` standard tview. Le wizard applique automatiquement le thème, wire
`Ctrl+S`/`Ctrl+B`/`Esc`, et gère le focus.

```go
Form: func(_ *tview.Application, onDone func()) *tview.Form {
    form := tview.NewForm()
    form.AddInputField("Champ", defaultValue, 0, nil, func(t string) { val = t })
    form.AddButton("Suivant", func() { onDone() })
    return form
}
```

### CustomView (contenu arbitraire)

Pour les UI non-formulaire (écran de bienvenue, sélection Yes/No, preview). Le callback
reçoit `*tview.Application`, le `*tview.Flex` container, et `onDone`. Il doit ajouter
ses widgets au container et gérer le focus.

```go
CustomView: func(app *tview.Application, container *tview.Flex, onDone func()) {
    tv := tview.NewTextView().SetDynamicColors(true)
    tv.SetText("Contenu personnalisé")
    tv.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
        if e.Key() == tcell.KeyEnter { onDone(); return nil }
        return e
    })
    container.AddItem(tv, 0, 1, true)
    app.SetFocus(tv)
}
```

### Processing-only (pas de UI)

Ni `Form` ni `CustomView`. Le wizard affiche le spinner avec le message `Processing`
et lance `OnDone` en goroutine. Utile pour les opérations longues sans input utilisateur.

```go
{
    Label:      "Extraction",
    Processing: "Extraction du contenu du hub...",
    OnDone:     func() error { return hubcontent.Extract(hubcontent.HubContentDir()) },
    InfoFields: func() []views.InfoField {
        return []views.InfoField{{Label: "Hub", Value: "extrait"}}
    },
}
```

---

## Écran résumé

Quand tous les steps sont terminés (ou skip), le wizard affiche automatiquement un écran
résumé reprenant tous les `InfoFields` accumulés, avec deux propositions de navigation :

- `[Enter]` → `shell.NavigateTo(SummaryTargetView)` (si configuré)
- `[Esc]` → pop le wizard (retour à la vue précédente)

Le callback `OnComplete(true, nil)` est appelé à ce moment. Il est exécuté avant que
l'utilisateur ne choisisse sa navigation, ce qui permet de persister les changements
(écriture hub.toml, reload config, etc.) pendant que le résumé est affiché.

---

## Différences avec `RunWizard` (standalone)

| Aspect | `RunWizard` (standalone) | `InlineWizardView` (inline) |
|--------|--------------------------|----------------------------|
| `tview.Application` | Crée la sienne | Utilise celle du shell |
| Omnibar | Non disponible | Visible et accessible |
| Toasts | Non disponibles | Fonctionnels |
| Navigation | `Ctrl+C` quitte | `Esc` pop vers la vue précédente |
| Info panel | Panneau latéral dédié | Intégré dans le layout vertical |
| Cycle de vie | Blocking (`Run()` → `WizardResult`) | Async (`Mount/Unmount`, callback `OnComplete`) |
| Utilisation | CLI one-shot, `--no-tui` | TUI shell interactif |
| Pré-requis | Aucun | Shell TUI actif |

**Quand utiliser lequel :**
- `RunWizard` : commandes CLI one-shot (`oh team init`, `oh project add`, `oh project configure`, `oh project remove`, `oh provider`)
- `InlineWizardView` : action TUI (omnibar, premier lancement, intégration vue)
- `RunInlineWizardStandalone` : monte un `InlineWizardView` dans sa propre `tview.Application`, pour réutiliser en CLI le même assistant que la TUI (`oh init` = assistant du premier lancement)
