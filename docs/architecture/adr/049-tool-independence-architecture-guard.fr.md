> [Read in English](049-tool-independence-architecture-guard.en.md)

# ADR-049 — Indépendance vis-à-vis de l'outil et garde-fou d'architecture

## Statut

Accepté

## Date

2026-10-07

## Contexte

L'[ADR-038](038-sessionspec-tool-adapters.fr.md) a posé un modèle neutre (`SessionSpec`, `BundleSpec`) et une interface d'adaptateur (`adapters.ToolAdapter`), avec un seul adaptateur réel, `internal/adapters/opencodev2`. Le reste d'oh devait n'en dépendre que par l'interface.

Un inventaire du 07/10/2026 a trouvé 81 fichiers Go, hors de l'adaptateur, qui nommaient encore l'outil :

- **imports directs** de l'adaptateur dans six fichiers de `cmd` et dans le service des runners distants, qui manipulaient le type concret `*opencodev2.Adapter` ;
- **logique propre à l'outil** hors de l'adaptateur : identifiants de fournisseur (`bricks.OpencodeProviderID`), installation du binaire Linux des jobs distants, variable `OH_OPENCODE_VERSION`, identité d'image `+opencode:`, dossiers de données supprimés par `oh purge`, format de l'ancien déploiement (`internal/deploycleanup` : `.opencode/`, `opencode.json`), noms d'agents natifs ;
- **clés de configuration** au nom de l'outil (`[opencode] default_provider`, `[execution] opencode_version`) ;
- **textes affichés** : 58 messages des fichiers de langue, dont l'option `--include-opencode` de `oh purge` ;
- des commentaires.

Rien n'empêchait d'en ajouter. La décision D19 (07/10/2026) demande que toute spécificité d'un outil vive uniquement dans `internal/adapters/<outil>`.

## Décision

1. **Registre d'adaptateurs** (`adapters.Registry`) : nom → constructeur, ordre de préférence, `Detect` (le premier outil installé et pris en charge) et `Get` (l'adaptateur d'un groupe de serveur enregistré, anciens noms compris). Il est construit **explicitement** dans une seule **racine de composition**, `cmd/v5_adapters.go`, le seul fichier d'oh hors des adaptateurs qui nomme un outil. Le reste de `cmd` ne voit que `adapters.ToolAdapter` et `adapters.ToolInfo` (nom affiché, binaire, version, plage prise en charge).
2. **Capacités neutres** (interfaces optionnelles de `internal/adapters`), implémentées par l'adaptateur :
   - `ProviderMapper` (identifiant de fournisseur de l'outil) ;
   - `LinuxInstaller` (outil des images de job) ;
   - `DataLocator` (données de l'outil, fichiers de l'ancien déploiement) ;
   - `LegacyCleaner` (`oh migrate deploy-cleanup`, avec un plan neutre `LegacyPlan`) ;
   - `Pairer` (ouverture dans le navigateur) ;
   - `OutcomeReader` (issue du dernier tour).
   Les erreurs de détection sont neutres : `adapters.ErrToolNotInstalled` et `*adapters.UnsupportedVersionError`. Le format de l'ancien déploiement est déplacé dans `internal/adapters/opencodev2/deploycleanup`.
3. **Clés et variables neutres**, avec lecture des anciennes et sans migration :
   - `[llm] default_provider` et `[execution] tool_version` ; les anciennes clés sont lues dans `internal/config/legacy_keys.go`, et `oh config set` les accepte encore ;
   - `OH_TOOL_VERSION`, avec lecture de l'ancienne variable par l'adaptateur pendant la v5.0 ; le schéma du pipeline distant passe à 2 ;
   - `oh purge --include-tool-data` ;
   - `oh status --json` donne `tool` et `tool_version`.
   Les valeurs déjà écrites restent lisibles (`servers.adapter = "opencode-v2"`, plateforme des sessions).
4. **Textes affichés** : le nom de l'outil vient de `ToolInfo.DisplayName`, passé en paramètre (`%s`) aux messages. Les aides des commandes, qui ne prennent pas de paramètre, emploient une formulation neutre (« l'outil des sessions »). Les commentaires sont neutres, sauf un constat daté sur une version de l'outil.
5. **Garde-fou** (`internal/archtest`). Ces tests citent D19 dans leur message d'échec :
   - (1) aucun fichier ni paquet n'importe un adaptateur, hors de lui-même et de la racine de composition (analyse des imports et `go list`) ;
   - (2) aucun identifiant ni littéral Go contenant `opencode` / `Opencode` / `OPENCODE` hors de la **liste autorisée** : l'adaptateur, la racine de composition, `internal/config/legacy_keys*.go`, et les tests de contrat contre un vrai serveur (tags `integration`, `e2e`, `container`) ;
   - (3) aucune liste littérale d'agents natifs de l'outil ;
   - (4) aucun message des fichiers de langue qui nomme l'outil.

## Conséquences

### Positives

- Un deuxième outil s'ajoute avec un paquet sous `internal/adapters/`, une ligne dans la racine de composition et ses capacités ; rien d'autre ne change.
- Toute nouvelle fuite échoue en CI, avec le renvoi à D19.
- Les capacités optionnelles ont un repli explicite quand un outil ne sait pas faire (pas de navigateur, pas d'export, pas d'ancien déploiement).

### Négatives / Compromis

- Rupture pour les scripts : `oh purge --include-opencode` devient `--include-tool-data`, les clés JSON de `oh status --json` changent, et le pipeline distant doit être régénéré (`oh remote setup`).
- L'identité des images de job change (`+<adaptateur>:<version>`) : la première exécution distante après la mise à jour reconstruit l'image.
- Les aides des commandes ne nomment plus l'outil : elles sont plus vagues, et l'aide renvoie à `oh doctor` pour savoir lequel est installé.
- Le garde-fou repose sur une liste de noms d'outils (`toolWord`, `toolAdapters`) à compléter quand un adaptateur s'ajoute.

## Alternatives considérées

| Alternative | Rejetée car |
|---|---|
| Inscription des adaptateurs par `init()` et import anonyme dans `main.go` (pilotes `database/sql`) | Inscription implicite ; `main.go` serait devenu la racine de composition sans le dire. |
| Garde-fou limité au code Go, textes et commentaires nettoyés une fois | Les fuites reviennent par les fichiers de langue (58 messages au moment de l'inventaire). |
| Migration SQLite des valeurs `opencode-v2` et des clés de `hub.toml` | Rien à gagner : les valeurs sont lues par le registre (alias) et par `legacy_keys.go`, et réécrites au prochain enregistrement. |
| Option `--include-opencode` gardée en alias caché | Il aurait fallu un littéral au nom de l'outil dans `cmd`, et l'option ne sert qu'à tester une réinstallation. |
