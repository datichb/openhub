# Méthode d'un lot N1 (versions 5.6 à 5.8)

À lire avec le plan de la version, pour tout lot de refactoring d'un domaine. Les règles du contrat sont dans l'ADR-061 ; il n'est pas nécessaire de relire ce dernier si cette page suffit.

## Les règles du contrat, en bref

1. `internal/ui/contract` ne contient que des **données sérialisables** : tags JSON en `snake_case`, `omitempty` sur les champs facultatifs, dates en `time.Time` (UTC). Pas de champ `func` ni `chan`, pas de type tview.
2. **Données brutes, sans mise en forme** : un état (`state: "waiting"`), jamais un libellé traduit ni une icône. L'affichage du TUI se calcule dans `internal/tui/v2/present`.
3. **Le contexte est explicite** : chaque appel reçoit un `contract.Scope`, et `appsvc` ne lit jamais l'état d'un shell.
4. **Erreurs et messages avec un code** : `contract.Error{Code, Params}` et `contract.Message{Code, Params, Level}`. Pas de chaîne déjà traduite.
5. **Un seul flux d'événements** (`appsvc/events`) remplace `Changes()`, les crochets de synchronisation, `RemountIf` et les relectures périodiques.
6. **`appsvc` ne navigue jamais et n'affiche rien** : il renvoie un résultat, ou une `contract.Intent` que le client applique.
7. **Capacités du client** (`contract.ClientCaps`) pour ce qui dépend du terminal : `Attach`, éditeur, suspension de l'affichage.
8. **Pas d'état caché** : un cache ou un brouillon devient un objet avec un identifiant.
9. Les énumérations sont des types `string` nommés ; les montants et les durées portent leur unité dans le nom (`cost_usd`, `*_ms`).
10. `appsvc` n'importe ni `internal/tui`, ni `cmd`, ni tview ou tcell, ni `net/http`, ni `storage/sqlite`. Il est construit **uniquement** dans `cmd/service_wiring.go`.

## Liste de contrôle d'un lot

**Avant de modifier quoi que ce soit**

1. [ ] Rebaser sur `main` ; vérifier qu'aucun lot en cours ne touche les fichiers du domaine (tableau de recoupement de la version).
2. [ ] **Tests de caractérisation**, commités en premier (`test: characterize <domaine> wiring`) :
   - la **sortie** de chaque méthode de backend et de chaque fonction de configuration du domaine, en JSON golden, sur une base `sqlitetest.Seed` déterministe (outil `characterize`, créé en N1-0) ;
   - les **écrans** du domaine en golden de rendu, alimentés par un faux construit à partir des mêmes données ; en ajouter là où il en manque. **Ces goldens d'écran ne doivent pas changer pendant tout le lot.**
3. [ ] Revérifier l'inventaire du domaine donné dans le plan de la version (numéros de ligne).

**Pendant**

4. [ ] Types → `ui/contract/<domaine>.go`, avec des alias provisoires dans `views` (ou une fonction de correspondance si la forme change).
5. [ ] Interfaces → `ui/contract`, avec `ctx` et `Scope`, des erreurs avec un code et des `Message`.
6. [ ] Implémentation → `appsvc/<domaine>`, à partir de `cmd/tui_<domaine>*.go`. Pas de `tuiShell`, de navigation, de `theme`, ni de `i18n.T` (hors contenus rédigés comme les rapports).
7. [ ] **Les commandes de la CLI du domaine appellent aussi `appsvc`** ; seule leur mise en forme (tableau, JSON de sortie) reste dans `cmd`. C'est ce qui supprime le code écrit deux fois.
8. [ ] Affichage (libellés, icônes, timeline, textes des messages) → `tui/v2/present`.
9. [ ] Logique métier des vues → `appsvc` ; les vues du domaine n'importent plus `teamstate`, `config`, `workflow`, `domain`, `app`…
10. [ ] Écritures faites sur la boucle d'affichage → appels à `appsvc` dans une goroutine `safego`, puis `QueueUpdateDraw`.
11. [ ] Rafraîchissement → abonnement au bus d'événements.
12. [ ] Navigation déclenchée depuis `cmd` → `Intent` renvoyée et appliquée par le TUI.
13. [ ] Suite de contrat `Run<Domaine>Contract` (`ui/contract/contracttest`) exécutée sur `appsvc/<domaine>`.
14. [ ] Les tests de `cmd/tui_<domaine>*_test.go` déménagent vers `appsvc/<domaine>`, avec `t.Parallel()` quand c'est possible ; regrouper les familles de tests en doublon.

**Fin de lot**

15. [ ] Plus aucun type de données `views` du domaine dans `cmd` ; `cmd/tui_<domaine>*.go` se limite à la composition (construire la vue avec `appsvc.Services` et `present`).
16. [ ] Listes gelées raccourcies : `views_business_imports.txt`, `tuiShell` hors `tui_*`, budget du branchement.
17. [ ] Goldens d'écran **identiques** ; goldens de caractérisation régénérés **seulement** pour les champs d'affichage retirés, avec une liste justifiée.
18. [ ] Alias supprimés ; `internal/appsvc/<domaine>` ajouté à `.github/windows-test-packages.txt`.
19. [ ] Durées des tests pas plus mauvaises (`make test-durations` avec `-compare` sur `mesures/ui-baseline.json`).
20. [ ] Recette manuelle de la section du domaine dans `~/.opencode/plan/oh-tui-recette.md`.

## Dans le récapitulatif du lot

- Fichiers déplacés, avec les lignes avant et après (`cmd/tui_*` et `appsvc/<domaine>`).
- Listes gelées : avant et après.
- Goldens régénérés et pourquoi.
- Durées avant et après.
- Code en double supprimé.
- Différences de comportement : aucune attendue ; sinon, une décision listée.
