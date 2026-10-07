# Shell gotchas — pièges courants dans les scripts du hub

> **Historique (v5).** Pièges des scripts Bash du hub, retirés avec la migration vers Go. Pour l'architecture actuelle, voir [Vue d'ensemble de l'architecture](../architecture/overview.fr.md) et les ADR 038 à 048 (`docs/architecture/adr/`). La suite de cette page est conservée pour l'historique.

Ce document recense les pièges rencontrés lors du développement des scripts bash du hub,
avec le pattern correct à utiliser dans chaque cas.

---

## jq — L'opérateur `//` traite `false` comme une valeur absente

### Le problème

L'opérateur `//` en jq est l'opérateur d'**alternative** (alternative operator).
Il traite `false` comme une valeur **absente** (au même titre que `null`), ce qui
produit un comportement contre-intuitif lors de la lecture d'un champ booléen.

```bash
# ❌ BUG — false est traité comme null, l'alternative "true" s'applique toujours
requires_api_key=$(jq -r '.providers["ollama"].requires_api_key // true' providers.json)
# Résultat : "true" même si la valeur dans le JSON est false
```

Le champ `ollama.requires_api_key` vaut `false` dans `providers.json`, mais
l'expression `false // true` retourne `true` — jq interprète `false` comme
une valeur manquante et applique l'alternative.

### Pattern correct

Pour lire un booléen pouvant légitimement valoir `false`, utiliser un `if/else` explicite :

```bash
# ✅ CORRECT — distingue null (champ absent) de false (valeur explicite)
requires_api_key=$(jq -r --arg provider "ollama" \
  '.providers[$provider].requires_api_key | if . == null then "true" else tostring end' \
  providers.json)
# Résultat : "false" si le champ vaut false, "true" si le champ est absent
```

**Logique :**
- Si le champ est `null` (absent) → retourner la valeur par défaut `"true"` (la plupart des providers requièrent une clé)
- Si le champ est présent (y compris `false`) → le convertir en string avec `tostring`

### Où ce pattern est utilisé

- `scripts/adapters/opencode.adapter.sh` — fonctions `_build_provider_json()` et `_build_provider_block()` : lecture de `requires_api_key` depuis `config/providers.json` pour décider si un provider peut fonctionner sans clé API.

### Règle générale

> **Ne jamais utiliser `// <default>` pour lire un champ booléen en jq.**
> Utiliser `| if . == null then "<default>" else tostring end` à la place.

---

## Références

- [jq manual — Alternative operator](https://jqlang.org/manual/#alternative-operator)
- `config/providers.json` — catalogue des providers avec leur champ `requires_api_key`
- `scripts/adapters/opencode.adapter.sh` — usage concret du pattern
