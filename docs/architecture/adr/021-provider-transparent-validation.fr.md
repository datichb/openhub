> 🇬🇧 [Read in English](021-provider-transparent-validation.en.md)

# ADR-021 — Validation transparente des providers LLM

- **Statut :** Accepté
- **Date :** 2026-07-01
- **Décideurs :** Équipe OpenCode Hub

---

## Contexte et problème

Quand un provider LLM est mal configuré (clé API absente, expirée, endpoint injoignable, ou
`opencode.json` incohérent), OpenCode affiche une notification non bloquante de type
`ProviderInitError` au moment où l'utilisateur choisit un agent dans le TUI. L'interface
reste ouverte mais aucune conversation ne démarre — l'utilisateur se retrouve dans une
interface silencieusement inutilisable, sans indication sur la cause ni sur comment la résoudre.

Les problèmes racines identifiés :

1. **Aucun pre-flight** : le hub lance OpenCode sans vérifier que le provider est joignable.
2. **Modèles orphelins** : quand une clé API est absente, `_build_provider_json()` retourne vide
   mais le champ `model` dans `opencode.json` est quand même préfixé du nom du provider
   (ex: `amazon-bedrock/...`). OpenCode tente d'initialiser un provider sans configuration.
3. **Providers litellm sans `models{}`** : mammouth, ollama et github-models ne déclaraient
   pas de bloc `models` dans `opencode.json`, provoquant des `ProviderModelNotFoundError`.
4. **Provider openrouter non géré** : le cas `openrouter` était absent de `_build_provider_json()`,
   laissant le JSON sans bloc provider pour les projets configurés sur openrouter.
5. **Suffixe `/chat/completions` en doublon** : le AI SDK ajoute automatiquement ce suffixe au
   `baseURL`. Si le hub l'inclut également, l'endpoint devient invalide.

---

## Décision

Implémenter une validation **transparente et non bloquante** en 4 couches, intégrée au hub sans
modifier le comportement d'OpenCode lui-même :

### Couche 1 — Messages informatifs (`provider-warnings.sh`)

Nouveau module `scripts/lib/provider-warnings.sh` affiché dans le bloc contextuel de `oc start`.
Le statut s'affiche avec ✅ (OK) ou ⚠️ (problème) accompagné d'un hint actionnable vers
`/connect` ou `oc config set`.

### Couche 2 — Pre-flight check (Approche A)

Test de connectivité léger (`curl`, timeout 3s) vers l'endpoint du provider avant le lancement
d'OpenCode. Le test est adapté au type d'authentification :
- **API Key providers** : `GET /v1/models` ou test d'en-tête HTTP
- **Bedrock** : présence de `AWS_BEARER_TOKEN_BEDROCK`, `AWS_ACCESS_KEY_ID`, ou `~/.aws/credentials`
- **GitHub Copilot** : présence du token dans `~/.local/share/opencode/auth.json`

Skip automatique si `curl` est absent ou si la commande est exécutée hors TTY (CI/CD).

### Couche 3 — Validation post-deploy (Approche C)

Après chaque écriture d'`opencode.json`, vérification que le préfixe du champ `model`
correspond à un bloc `provider` existant dans le fichier généré. Si non (modèle orphelin),
stockage dans `_DEPLOY_PROVIDER_WARNING` pour affichage au prochain `oc start`.

### Couche 4 — Correctifs structurels (Approche B)

- Ajout d'un bloc `models` et d'un `name` dans les providers litellm pour éviter les
  `ProviderModelNotFoundError`.
- Ajout du cas `openrouter` dans `_build_provider_json()`.
- Détection et signalement des `baseURL` incluant `/chat/completions`.

---

## Alternatives considérées

### Alternative A — Bloquer le lancement si provider KO

Refuser de lancer OpenCode si le pre-flight échoue. Rejeté car :
- Faux positifs sur réseau lent ou VPN
- Empêche l'usage hors ligne (Ollama local, Bedrock sans réseau temporairement)
- Incompatible avec l'objectif de transparence

### Alternative B — Déléguer entièrement à `/connect`

Supprimer l'injection de credentials dans `opencode.json` et tout déléguer au mécanisme
natif d'OpenCode (`auth.json` via `/connect`). Rejeté car :
- Rupture de la UX actuelle (les projets déployés fonctionnent sans action manuelle)
- Perd le contrôle centralisé des credentials par projet via `api-keys.local.md`

### Alternative C — Fallback provider automatique

Configurer un provider de secours dans `hub.json`. Rejeté pour cette itération car :
- Complexité accrue (quel fallback choisir ? quelle UX ?)
- Masque les problèmes au lieu de les exposer clairement
- Peut être ajouté dans une itération future si le besoin se confirme

---

## Conséquences

### Positives

- L'utilisateur est **toujours informé** du statut de son provider avant que le problème
  n'apparaisse dans OpenCode.
- Les hints `→ Utilisez /connect` et `→ oc config set` réduisent le temps de résolution.
- La détection du suffixe `/chat/completions` prévient un problème récurrent avec MammouthAI.
- Les providers litellm fonctionnent correctement sans `ProviderModelNotFoundError`.
- La couverture de tous les chemins d'entrée (`adapter_start`) garantit le warning même
  depuis `oc quick`, `oc review`, `oc audit`, etc.

### Négatives / contraintes

- Le pre-flight ajoute ~3s de délai sur les commandes TTY si l'endpoint ne répond pas.
  Acceptable : ce délai n'apparaît que quand le provider est réellement problématique.
- `curl` est requis pour les tests de connectivité. Skip gracieux si absent.
- Les tests de connectivité Bedrock nécessitent le CLI `aws` pour la validation complète.
  En son absence, la détection est moins précise (vérification des env vars uniquement).

---

## Fichiers impactés

| Fichier | Modification |
|---------|-------------|
| `scripts/lib/provider-warnings.sh` | Nouveau fichier — validation + affichage |
| `scripts/lib/i18n.sh` | +12 clés i18n (6 FR + 6 EN) |
| `scripts/adapters/opencode.adapter.sh` | Bloc `models{}` litellm + openrouter + validation post-deploy + `_warn_provider_if_needed` dans `adapter_start` |
| `scripts/cmd-start.sh` | Intégration `_display_provider_status` dans le bloc contextuel |
| `tests/test_lib_provider_warnings.bats` | Nouveau fichier — ~35 tests BATS |
| `tests/test_opencode_adapter.bats` | +6 tests |
| `tests/test_lib_i18n.bats` | +3 tests |
| `docs/guides/providers.fr.md` | Nouvelle section "Diagnostic et résolution" |
| `docs/guides/providers.en.md` | Nouvelle section "Diagnosing and Resolving" |
