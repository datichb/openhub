# Esquisses des vues TUI (référence des lots tui-*, 15/10/2026)

Esquisses texte, pas des maquettes définitives : libellés, couleurs et largeurs se règlent dans les lots. Structure `tview` (liste à gauche, détail à droite, fenêtres modales, formulaires). Touches selon la convention ci-dessous.

## Convention de touches (décidée)
- **Partout** : `↑↓`/`jk` déplacer, `g`/`G` premier/dernier, `/` filtrer, `r` rafraîchir, `?` aide, `Entrée` ouvrir, `Échap` revenir.
- **Liste** : `n` créer, `e` éditer, `d` supprimer (toujours confirmé), `x` archiver/classer (jamais destructif), `R` reprendre/restaurer.
- **Décision** (fiche de checkpoint, proposition wiki, élément d'un lot) : `y` valider/accepter, `n` refuser (confirmé, consigne), `e` modifier, `s` laisser en attente.
- Changements : `D`→`d` (Sessions), `g`→`R` (récupérer une session), `r`→`R` (restaurer, Historique), `a`→`y` (accepter une proposition wiki) ; diff des Workflows sur une autre touche (à fixer par tui-keys, ex. `c` comparer). `a` reste « attacher » dans Sessions. Message unique au premier lancement.

## 1. Sessions : fan-out regroupé, onglets du détail (tui-sessions)
```
┌ Sessions ─────────────────────────────────┐┌ implement-spec · ses_8f2a ───────────────────────────┐
│ ● ⑂ implement-spec  spec auth   $1.84  2m ││ [Détail]  Trace  Rapports  Fan-out                   │
│   ├ ✓ ↳ ticket  oh-12  merged    $0.41    ││                                                      │
│   ├ ● ↳ ticket  oh-13  en cours  $0.37    ││ implement-spec · semi-auto · 🐳 conteneur · a1b2c3d4  │
│   └ ⏸ ↳ ticket  oh-14  cp-2      $0.22    ││ ⏸ En attente · orchestrator-dev · $1.84 · il y a 2 m │
│ ✓   spec           auth OAuth  $0.93  1h  ││ ⏸ cp-merge : 2/3 enfants mergés, vérification à venir│
│ ✓   incident       prod 502    $0.71  1j  ││                                                      │
│ ✗   security-fix   rp-4        $0.40  2j  ││ Sorties relevées par oh                              │
│                                           ││   tickets   oh-12, oh-13, oh-14                      │
│                                           ││   method    tdd                                      │
│                                           ││ ↪ Suites proposées (next:)                           │
│                                           ││   review  -i branch=feat/auth                        │
└───────────────────────────────────────────┘└──────────────────────────────────────────────────────┘
 Entrée ouvrir · y/n décider · a attacher · e enchaîner · B lot · d supprimer · R reprendre · Tab onglet · ? aide
```

## 2. Fiche de checkpoint : `y` sur vérification échouée (tui-sessions)
```
┌ cp-2 · ticket oh-14 ────────────────────────────────────────────────────────┐
│ Valideur attendu : Grace Hopper  (vous : Ben Datiche ⚠ hors valideur)        │
│ ✗ Vérification par oh : échec                                                │
│   go test ./...      FAIL  internal/auth  TestTokenRefresh  (0.84 s)         │
│   go vet ./...       ok                                                      │
│ La validation est impossible tant que la vérification échoue.                │
│  [ Corriger (consigne au developer) ]  [ Passer outre… ]  [ Diff complet ]   │
│  [ Lire rp-7 ]  [ Annuler ]                                                  │
└──────────────────────────────────────────────────────────────────────────────┘
```
« Passer outre… » : motif + confirmation, tracé ; absent là où oh l'interdit (`cp-merge`).

## 3. Fiche : propositions wiki sans limite (tui-wiki-bricks, composant de tui-keys)
```
┌ cp-wiki · onboarding · 9 propositions ──────────────────────────────────────┐
│ ▸ wp-1  technical/stack.md      ajout       CONFIRMÉ   ✓ acceptée            │
│   wp-2  technical/stack.md      remplace    PROBABLE   ? en attente          │
│   wp-3  business/index.md       ajout       PROBABLE   ✗ refusée             │
│   wp-4  technical/tests.md      remplace    CONFIRMÉ   ? en attente          │
│   …  5 autres                                                                │
├──────────────────────────────────────────────────────────────────────────────┤
│ wp-2 · technical/stack.md · section « Paquets »                              │
│   − - `internal/store` : persistance SQLite                                  │
│   + - `internal/storage` : persistance SQLite (renommé, commit 3f2a1c)       │
│   source : internal/storage/store.go:1 · auteur onboarder                    │
└──────────────────────────────────────────────────────────────────────────────┘
 y accepter · n refuser (confirm.) · e modifier · s laisser · A tout accepter · Entrée valider cp-wiki
```

## 4. File wiki du projet, hors checkpoint (tui-wiki-bricks)
```
┌ Wiki du projet · openhub ───────────────────┐┌ wp-17 · technical/conventions.md ─────────────┐
│ Pages 14 · CONFIRMÉ 61 % · dernier sync 2 j ││ doc-sync · ses_41c0 · il y a 3 h · PROBABLE   │
│ En attente (doc-sync, sans commit)          ││ + - Les commits suivent Conventional Commits, │
│ ▸ wp-17  conventions.md   ajout     3 h     ││ +   suffixe `(modernisation <lot>)`.          │
│   wp-18  stack.md         remplace  3 h     ││ source : CONTRIBUTING.md:12                   │
│ Acceptées, à appliquer au prochain commit   ││                                               │
│   wp-12  tests.md         ajout     1 j     ││                                               │
│ Périmées (ligne disparue)                   ││                                               │
│   wp-9   architecture.md  remplace  2 j     ││                                               │
└─────────────────────────────────────────────┘└───────────────────────────────────────────────┘
 y accepter · n refuser · e modifier · s laisser · / filtrer · r rafraîchir
```
Vue d'équipe renommée « Propositions d'équipe (team-state) ».

## 5. Onglet Fan-out et actions (tui-sessions)
```
┌ implement-spec · ses_8f2a ───────────────────────────────────────────────────┐
│  Détail  Trace  Rapports  [Fan-out]                                          │
│ Expérimental · parallèle 2 · pilote ● actif · budget $1.84 / $10.00 ▓▓░░░░░  │
│  ticket  enfant      état      branche                       coût   fin      │
│  oh-12   ses_91aa    ✓ mergé   oh/fanout/8f2a/oh-12          $0.41  signal   │
│  oh-13   ses_91ab    ● en cours oh/fanout/8f2a/oh-13         $0.37  —        │
│  oh-14   ses_91ac    ⏸ cp-2    oh/fanout/8f2a/oh-14          $0.22  —        │
│ Tours de correction : 1 / 3 · dernier refus de cp-merge : « tests auth »     │
│ Branche d'intégration : oh/fanout/8f2a/integration (3 commits)               │
└──────────────────────────────────────────────────────────────────────────────┘
 Entrée ouvrir l'enfant · p relancer le pilote · c nettoyer… · B valider en lot
```
```
┌ Nettoyer le fan-out ses_8f2a ───────────────────────────────┐
│ Supprimés :  worktree oh-12, branche oh/fanout/8f2a/oh-12   │
│ Gardés :     oh-13 (session active), oh-14 (non commité)    │
│ [ ] Supprimer aussi les branches non mergées (--discard)    │
│                      [ Nettoyer ]  [ Annuler ]              │
└─────────────────────────────────────────────────────────────┘
```

## 6. Trace et Rapports (tui-sessions, lecteur simple)
```
┌ ses_8f2a · Trace ────────────────────────────────────────────────────────────┐
│ /cp-merge                                                       3 résultats  │
│ 10:02  orchestrator-dev  workflow_checkpoint cp-plan   ✓ validé par Ben      │
│ 10:05  ↳ ses_91aa        bash  git commit -m "feat(auth)…"                   │
│ 10:31  oh                cp-merge  ✗ refusé · consigne « tests auth »        │
│ 10:32  oh                tour de correction 1 → ses_93f0 (quick, fixer)      │
│ 10:40  oh                cp-merge  ⚠ hors valideur · validé par Ben          │
└──────────────────────────────────────────────────────────────────────────────┘
 / rechercher · n/N suivant/précédent · g/G début/fin · Échap retour
```
```
┌ ses_8f2a · Rapports ───────────────────┐┌ rp-7 · reviewer · 10:28 ──────────────────────┐
│ ▸ rp-7  reviewer          10:28  4 Ko  ││ ## Résumé                                     │
│   rp-6  developer (oh-13) 10:21  2 Ko  ││ Deux constats bloquants sur le rafraîchisse-  │
│   rp-5  developer (oh-12) 10:09  2 Ko  ││ ment du jeton…                                │
└────────────────────────────────────────┘└───────────────────────────────────────────────┘
```

## 7. Validation par lot avec filtre (tui-sessions)
```
┌ Valider en lot · fan-out ses_8f2a · filtre : cp-2 ▾ ──────────────────────────┐
│  ses_91ab  cp-2  oh-13  ✓ vérif ok                      ( ) valider  (•) laisser│
│  ses_91ac  cp-2  oh-14  ✓ vérif ok  ⚠ hors valideur     (•) valider  ( ) laisser│
│  ses_91ad  cp-2  oh-15  ✗ vérif     → ouvrir la fiche   ( ) valider  ( ) refuser│
│                        [ Confirmer (1 validé, 2 laissés) ]  [ Annuler ]       │
└───────────────────────────────────────────────────────────────────────────────┘
```

## 8. Fiche de lancement (tui-launch)
```
┌ Lancer · feature → implement-spec (avec tickets) ───────────────────────────┐
│ Tickets       oh-12, oh-13, oh-14                                           │
│ Instructions  [respecter l'API publique                              ]       │
│               Consignes données à chaque ticket (texte, facultatif)         │
│ Méthode       [tdd ▾]      standard · tdd                                   │
│ Fan-out       [ ] expérimental (désactivé dans les réglages)                │
│ Runtime       [🐳 conteneur ▾]                                              │
│ Spec          ⚙ calculé par oh : session.output(spec.spec) → ses_41c0       │
│                                      [ Lancer ]  [ Annuler ]                │
└─────────────────────────────────────────────────────────────────────────────┘
```
```
┌ Lancer · libre ─────────────────────────────────────────────┐
│ Agent d'entrée  [orchestrator ▾]   (agents « selectable »)  │
│ Demande         [                                       ]   │
└─────────────────────────────────────────────────────────────┘
```
Erreur de type sous le champ (« Entier attendu », « Branche introuvable : feat/x »).

## 9. Briques en lecture enrichie (tui-wiki-bricks)
```
┌ Briques · équipe acme ─────────────────────────────────────┐┌ skill:review/review-protocol ─────────┐
│  id                         origine                 v  état││ équipe (remplace hub:review-protocol) │
│ ▸ review-protocol            équipe ⟳ hub            3  ✓  ││ version 3 · publiée par Grace, 12/10  │
│   dev-standards-go           hub                     —  ✓  ││ utilisée par : reviewer (review, …)   │
│   acme-conventions           équipe                  1  !  ││ coût : 1 240 tokens                   │
│   legacy-helper              équipe (non scellée)    —  ✗  ││ ! description > 200 caractères        │
└────────────────────────────────────────────────────────────┘└───────────────────────────────────────┘
 / filtrer · f famille · Entrée détail · r rafraîchir        (écriture : oh brick …)
```

## 10. Message du premier lancement (tui-keys)
```
┌ Nouvelles touches ──────────────────────────────────────────────┐
│ Les touches sont maintenant les mêmes dans toutes les vues :    │
│  Liste      n créer · e éditer · d supprimer · x archiver       │
│             R reprendre/restaurer                               │
│  Décision   y valider · n refuser · e modifier · s laisser      │
│  Partout    g/G début/fin · / filtrer · r rafraîchir · ? aide   │
│ Changé : D→d (Sessions), g→R (récupérer), r→R (historique),     │
│          a→y (accepter une proposition wiki)                    │
│                                   [ Compris ]                   │
└─────────────────────────────────────────────────────────────────┘
```

## 11. Petits ajouts
- **Nettoyage (`oh gc`)** (tui-launch) : liste par catégorie (sessions, brouillons, fan-out, worktrees), aperçu, confirmation.
- **Doctor** (tui-launch) : bouton « Corriger » (`--fix`) sur les contrôles qui le permettent, avec confirmation.
- **`n` sur un checkpoint** (tui-sessions) : le formulaire affiche le valideur attendu et l'avertissement.
