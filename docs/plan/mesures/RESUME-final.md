# Mesures finales — session de tests 4 (HEAD 1de669e5)

Décision : **à reprendre** — un seul point bloquant (voir « Couverture ») ; tout le reste est vert.

## Contrôles statiques
- `go vet ./...` : propre (vet.txt). `golangci-lint run` (caches neufs) : 0 issues (lint.txt).

## Charge
- short : 18,55 (avant) → 28,97 (après). full-race : 26,81 au premier relevé (> 25) → sleep 180 → 7,20 → lancé ; 16,85 après.
  Charge de 26,8 due en partie à la suite short qui venait de finir ; au lancement la machine était calme (pas « sous charge »).
- cover-full : 15,82 avant. Relance couverture ciblée : 13,80.

## Durées (réel, premier→dernier événement)
| suite | référence 7ac4a25c | vague2bis 9c5930c7 | vague3 34a16efa | final |
|---|---|---|---|---|
| short -cover | 260,7 s | 166,9 s | 173,8 s | **107,0 s** (−59 % vs réf) |
| full -race | 974,3 s | 362,3 s | 430 s (run1, 1 échec) / 1138 s (run2, 0 échec) / 1517 s (run « full », 6 échecs, sous charge) | **383,8 s** (−61 % vs réf) |
| cover-full (sans -race) | — | — | — | 250 s (wall, sans json) |

Résultats : short 4144 pass / 0 fail / 136 skip ; full-race 4286 pass / 0 fail / 8 skip (réf 4242/0/7). 0 DATA RACE, 0 échec de paquet, .err vides. cover-full : aucun FAIL.

## Tests
- Premier niveau exécutés (JSON) : 2949 → 2505 ; total avec sous-tests 4235 → 4280 (short), 4249 → 4294 (full-race). Le net +45 tests = tests en table (sous-tests) plus nombreux.
- `func Test…` dans les sources : 3062 → 2749.
- Fichiers *_test.go : 709 → 816 (+107, nouveaux paquets helper *test + tests découpés).
- Lignes de test : `git diff --shortstat 7ac4a25c HEAD -- '*_test.go'` = 387 fichiers, +8008 / −10064 (net −2056 lignes).

## Paquets full-race (réf → final)
Top 10 : bundle 964,5→165,7 ; evalrun 184,1→48,9 ; fanout 224,9→182,1 ; teamstate 175,7→136,3 ; cmd 357,4→377,8 (+6 %) ; workflow 188,0→198,1 ; daemon 126,9→155,2 ; runsvc 97,0→116,9 ; checkpoint ; beads 164,1→234,1.
Gains (top) : bundle −798,9 ; evalrun −135,3 ; fanout −42,8 ; teamstate −39,4 ; wiki −7,9 ; … (seuls ~5 paquets gagnent en full-race, voir compare-full-race-vs-reference.txt).
Paquets plus lents que la référence (full-race) : beads +70, gitutil +47, gateway +39, daemon +28, hubcontent +27, docstest +21, cmd +20, runsvc +20, gc +19, ciconfig +16, gittest +15, i18n +13, workflow +10, + ~50 petits paquets (+0,1…+6 s). Les durées de paquet sont des durées murales avec paquets concurrents : le parallélisme accru (t.Parallel) et le démarrage `-race` sous contention les gonflent ; le total réel baisse de 61 %. En short, mêmes tendances (gateway +12, gitutil +8, ciconfig +8…) mais total −59 %. Détails : compare-*.txt.

## Couverture -short vs référence (coverage-by-package.tsv)
- Paquets : 101 → 105. Disparu : `internal/task` (attendu). Nouveaux : beads/bdtest 38,1 ; config/configtest 100 ; i18n/i18ntest 100 ; storage/sqlite/sqlitetest 100 ; testutil/golden 78,4.
- Baisses : **internal/beads 65,7 → 56,9 (−8,8) — PERSISTANTE** (relance ciblée : 56,9 %) ; internal/daemon 84,8 → 84,7 (−0,1) — fluctuation (relance : 84,8 %).
- Cause beads : commit 1f658c4e « real bd tests out of -short » — 5 tests contre le vrai `bd` (TestQueryRealBd, TestLintRealBd, TestTrackingRealBd, TestCleanImpactAfterRealBdInit, TestCleanImpactUndoesBdInitCommit, ~120 s en réf) sont désormais ignorés en -short. Couverture rétablie en complet : 65,8 % (réf short 65,7 %). Arbitrage vitesse/couverture-short manifestement délibéré, mais c'est une baisse -short persistante au sens du critère.
- Hausses : app +39,2 ; tui/v2/layout +33,3 ; gittest +22,4 ; widgets +6,0 ; i18n +5,3 ; config +2,7 ; fanout +1,1 ; autres ≤ +0,5.

## Couverture complète (cover-full.txt, coverage-full-by-package.tsv)
Moyenne simple des 105 paquets : 64,4 %. (Pas de total pondéré : `go test` n'en donne pas sans -coverprofile.) Quelques valeurs : cmd 31,1 ; beads 65,8 ; daemon 84,8 ; fanout 83,7 ; bundle 93,7 ; app 95,7 ; gateway 81,0 ; runsvc 74,3 ; teamstate 66,4 ; checkpoint 88,4 ; evalrun 90,1 ; workflow 75,1 ; views 58,5 ; shell 58,6. Voir le tsv pour tous.

## Goldens / testdata
- Blobs uniques réf 320 ; HEAD 350 ; `comm -23` : **vide** → aucun contenu perdu.
- `git diff -M --name-status 7ac4a25c HEAD` : R100 = 256, A = 29, M = 0, D = 0 → aucune anomalie. (Fichiers : 341 en réf par ls-tree, 371 à HEAD ; la consigne parlait de 342.)

## Dossiers temporaires
- go-build* dans $TMPDIR : 61 → 61 (aucun laissé).
- Test* : 401 → 404 : **3 nouveaux, tous `TestOhGC<n>`** (un par suite lancée : 20:50, 20:59, 21:04), contenant un sous-dossier `cache`. Fuite préexistante (≈ 245 dossiers du même nom datant du 12 oct. et après) du test `TestOhGC` (cmd/gc_test.go), non introduite par la vague 4 ; à corriger à part si souhaité.

## Segfaults git
Aucun (« segmentation fault »/« signal: » absents des trois sorties).

## Fichiers
short.json/err/exit, full-race.json/err/exit, cover-full.txt, cover-rerun.txt, analyse-auto.md, testdurations-*.txt, compare-{short,full-race}-vs-{reference,vague3}.txt, coverage-by-package.tsv, coverage-full-by-package.tsv, golden-*.txt, tmp-*.txt, load-*.txt, vet.txt, lint.txt.
