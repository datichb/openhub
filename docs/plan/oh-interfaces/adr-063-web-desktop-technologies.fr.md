> Brouillon — à relire avant d'écrire dans `docs/architecture/adr/063-web-desktop-technologies.fr.md` (et sa version EN)

# ADR-063 — Technologies des interfaces web et desktop

## Statut

Proposé

## Date

2026-11-05

## Contexte

Les [ADR-061](adr-061-ui-contract-architecture.fr.md) et [ADR-062](adr-062-ui-server.fr.md) posent un contrat d'interface servi par `uiserver`. Il faut choisir les technologies du front web et de l'app macOS, sous ces contraintes :

- **Un seul binaire Go, sans CGO** (`CGO_ENABLED=0`, SQLite en Go pur) : le front est intégré au binaire, Node ne sert qu'à la compilation.
- **Usage local et hors ligne** : aucune ressource externe (CDN, polices), CSP stricte.
- **Le contrat Go est la référence** : les types du front sont générés, pas réécrits.
- **Des formulaires décrits par des données** ([ADR-061](adr-061-ui-contract-architecture.fr.md) §4) : un seul composant les affiche.
- **Des événements en direct** (SSE).
- **Des utilisateurs non-devs** : la qualité de l'interface et l'**accessibilité** sont des exigences.
- **Ce qui existe déjà** : Node et TypeScript dans le dépôt (plugin `internal/adapters/opencodev2/plugin`, présentations), les traductions en JSON (`internal/i18n/locales/fr.json`, `en.json`), un runner macOS dans la CI. Les contenus intégrés générés à la compilation ne sont pas committés (`internal/runtime/container/assets/bd-linux-*` par `make embed-bd`, `internal/hubcontent/hub/` par `make sync-content`).
- **Le Mac App Store est exclu** : bac à sable obligatoire (données séparées de `~/.oh`, processus des agents limités), interdiction de télécharger du code exécutable (règle 2.5.2), démon `ohd`, pilotage du terminal par `osascript`, et une revue Apple à chaque version (12 versions en 3 mois).

## Décision

### 1. Front web

| Besoin | Choix |
|---|---|
| Langage | TypeScript (mode strict) |
| Framework | **Vue 3** (Composition API, `<script setup>`), page unique compilée par **Vite** |
| Composants | **Reka UI** (primitives accessibles) et **shadcn-vue** (Tailwind), code copié dans le dépôt ; couleurs reprises de `internal/tui/theme` |
| Données et événements | **@tanstack/vue-query** ; un client SSE qui invalide les requêtes selon le type d'événement |
| Navigation | **Vue Router**, routes typées ; la portée (hub, projet, équipe) dans l'URL |
| État de l'interface | **Pinia**, réduit au minimum (les données métier sont dans le cache de vue-query) |
| Types et appels à l'API | OpenAPI 3.1 généré ([ADR-062](adr-062-ui-server.fr.md)) → **openapi-typescript** et **openapi-fetch** ; contrôle en CI des fichiers générés |
| Formulaires et assistants | Un composant `<SchemaForm>` qui lit le schéma de `ui/forms` ; validation côté serveur |
| Langue | **vue-i18n** alimenté par les `fr.json`/`en.json` d'oh ; conversion des paramètres (`%s` → `{x}`) à la compilation |
| Board | **vue-draggable-plus**, plus un déplacement au clavier (« Déplacer vers… », raccourcis, annonce `aria-live`) |
| Modifications, Markdown, graphiques | **diff2html** ; **markdown-it** avec **DOMPurify** ; **uPlot** |
| Tests | **Vitest** et Vue Testing Library ; **Playwright** sur Chromium **et** WebKit, contre un vrai `uiserver` et un dossier `~/.oh` de test |

### 2. Accessibilité

- Cible **WCAG 2.2 AA**.
- `eslint-plugin-vuejs-accessibility` et tests **axe-core** dans Playwright : une violation fait échouer la CI.
- Tout se fait au clavier ; les raccourcis reprennent ceux du TUI quand c'est possible (registre `internal/tui/v2/keys` exposé par le contrat, à évaluer).
- Les mises à jour en direct sont regroupées avant d'être annoncées aux lecteurs d'écran.
- Respect de `prefers-reduced-motion` et `prefers-contrast` ; contrastes du thème vérifiés ; focus toujours visible ; attribut `lang` à jour.

### 3. Build et intégration

- Projet front dans `cli/web/` (à confirmer). pnpm, Node à version fixée (`.nvmrc`, `packageManager`), `pnpm install --frozen-lockfile`.
- **Les fichiers compilés ne sont pas committés.** Une page de remplacement (`internal/webui/dist/index.html` : « Interface web non compilée : lancez `make web` ») est committée pour que `go build` et `go test` fonctionnent toujours sans Node.
- `make web` compile le front dans `internal/webui/dist/`, intégré par `embed.FS`. goreleaser l'appelle dans ses étapes préalables, à côté de `sync-content` et `embed-bd` ; `release.yml` installe Node et pnpm.
- En développement : `pnpm dev` (Vite relaie les appels vers `uiserver`).
- CI : une tâche front dédiée (lint, types, Vitest, axe, Playwright, audit des dépendances).
- README : `go install .` donne la page de remplacement tant que `make web` n'a pas été lancé.

### 4. App macOS

- **Swift** (SwiftUI et AppKit si nécessaire) avec une **WKWebView**.
- Un **`WKURLSchemeHandler` `oh://`** relaie vers `~/.oh/run/ui.sock` : pas de port, pas de jeton dans l'URL.
- Le binaire `oh` (Intel et Apple Silicon) est intégré dans `OpenHub.app/Contents/Helpers/` ; l'app démarre `uiserver` par lui.
- **Intégration native** : icône dans la barre des menus avec le badge `● N ⏸ M`, badge sur l'icône du Dock, notifications natives avec boutons d'action, sélecteur de dossier natif pour ajouter un projet, lancement à l'ouverture de session, sortie conforme à l'[ADR-047](047-session-interaction-daemon.fr.md) §5.
- Le PATH est lu dans un shell de connexion.
- « Installer la commande `oh` » crée un lien symbolique vers le binaire de l'app.
- **Adresse du serveur configurable**, pour ne pas fermer la porte à une variante « client seul » ([ADR-062](adr-062-ui-server.fr.md) §8).
- **Distribution** : maintenant, `make app` produit un `.dmg` avec signature ad hoc (usage local) ; plus tard, Developer ID, notarisation dans la CI macOS, **Sparkle** (l'app et son `oh` mis à jour ensemble, après la fin des sessions actives), cask Homebrew `datichb/tap/openhub-app`.

### 5. Windows et Linux, plus tard

**Tauri v2**, sur le même front. Les tests Playwright sur WebKit (WebKitGTK sous Linux) et Chromium (WebView2 sous Windows) le préparent dès maintenant. Le point d'accès de `uiserver` sous Windows suit l'[ADR-062](adr-062-ui-server.fr.md) §3.

## Conséquences

### Positives

- Un seul front pour le navigateur, l'app macOS et une future app Tauri.
- Le binaire reste du Go sans CGO ; Node et Swift ne servent qu'à la compilation.
- L'accessibilité est vérifiée automatiquement, et pas seulement annoncée.
- Une modification du contrat Go casse la compilation du front (types générés).
- L'app apporte ce dont les non-devs ont besoin : sélecteur de dossier, notifications avec boutons d'action, présence dans la barre des menus.

### Négatives / Compromis

- Deux chaînes de plus : Node et pnpm pour le front, Swift et Xcode pour l'app.
- La release a une étape de plus (`make web`), et `go install .` sans `make web` ne donne que la page de remplacement.
- Le glisser-déposer accessible au clavier est à écrire nous-mêmes en Vue (dnd-kit n'existe qu'en React).
- L'app Swift ne sert que macOS ; Windows et Linux demanderont Tauri.
- Hors App Store : pas de vitrine ni de recherche dans l'App Store, et un compte Developer ID est nécessaire pour la distribution.

## Alternatives considérées

| Alternative | Rejetée car |
|---|---|
| React (avec dnd-kit, TanStack Router) | Écosystème plus large, mais choix de l'équipe pour Vue ; les besoins sont couverts (Reka UI, vue-query). Second choix défendable. |
| Svelte 5 | Proche, mais écosystème de composants accessibles plus jeune. |
| templ et htmx (tout en Go) | L'API de l'[ADR-062](adr-062-ui-server.fr.md) est en JSON : il faudrait une seconde couche qui produit du HTML. Le board, le suivi en direct et les panneaux multiples deviennent pénibles. |
| SolidJS | Écosystème réduit. |
| Next.js, Nuxt (rendu côté serveur), SvelteKit | Ils supposent un serveur Node, alors que le serveur est le binaire Go. |
| Committer les fichiers compilés du front | Fichiers minifiés à chaque modification, conflits sur les noms à hash, risque de décalage avec les sources ; contraire à la pratique du dépôt (`embed-bd`, `sync-content`). |
| Wails | Impose CGO et pousse à appeler `appsvc` directement depuis l'app, ce qui contourne `uiserver`. |
| Tauri pour macOS dès maintenant | Ajoute Rust pour une seule plateforme ; Swift s'intègre mieux à macOS. Gardé pour Windows et Linux. |
| Electron | Plus de 150 Mo avec Chromium, sans avantage pour une app qui n'affiche que le web. |
| Mac App Store | Bac à sable incompatible avec `~/.oh`, `ohd`, les processus des agents et les outils gérés ; revue Apple à chaque version. |
| xterm.js (TUI dans le navigateur) | Ne répond pas au besoin des non-devs ; éventuellement gardé comme onglet « Terminal avancé » plus tard. |
