---
name: dev-standards-vuejs
description: Bonnes pratiques Vue.js — Composition API, typage, templates, composables, watchers, performances, conventions de nommage.
---

# Skill — Standards Vue.js

## Rôle
Ce skill définit les bonnes pratiques spécifiques à Vue.js.
Il complète `dev-standards-universal.md` et `dev-standards-frontend.md`.

---

## 🔒 Gestion de données — Règle héritée

Toute décision liée à Pinia (structure des stores, découpage,
communication inter-stores) est soumise à validation explicite.
Tu proposes, l'utilisateur décide.

---

## Composition API

- Composition API systématiquement — pas d'Options API
- `<script setup>` privilégié sur `defineComponent`
- Logique réutilisable extraite dans des composables (`useXxx`)
- Les composables sont purs — pas de dépendances implicites

---

## Typage Vue.js

- Props typées avec `defineProps<T>()`
- Emits typés avec `defineEmits<{ eventName: [payload: Type] }>()`
- `ref` et `reactive` typés explicitement si l'inférence est ambiguë
- `computed` typé en retour si complexe

---

## Templates

- Pas de logique métier dans les templates
- Expressions ternaires simples tolérées — logique complexe dans `computed`
- `v-for` toujours accompagné d'une `:key` stable et unique
- Pas d'index comme `:key` si la liste peut être réordonnée
- `v-if` et `v-for` jamais sur le même élément

---

## Composables

- Un composable = une responsabilité
- Nommage en `useXxx` systématiquement
- Retour explicite et typé
- Pas d'état global dans un composable — utiliser Pinia pour ça

---

## Watchers

- `watch` et `watchEffect` uniquement si `computed` ne suffit pas
- Pas de watchers pour synchroniser des états — revoir l'architecture
- `watchEffect` pour les effets de bord liés à des réactifs multiples
- Cleanup systématique si le watcher crée des effets persistants

---

## Performances Vue.js

- `computed` pour toute valeur dérivée — jamais recalculée dans le template
- `v-memo` uniquement si un problème de performance est mesuré et prouvé
- `defineAsyncComponent` pour les composants lourds ou chargés conditionnellement
- `shallowRef` et `shallowReactive` si la réactivité profonde n'est pas nécessaire

---

## Conventions de nommage

- Composants : PascalCase (`UserCard.vue`)
- Composables : camelCase préfixé use (`useUserProfile.ts`)
- Props : camelCase en JS, kebab-case dans le template
- Événements émis : kebab-case (`user-updated`)
- Fichiers de store Pinia : camelCase suffixé store (`userStore.ts`)
