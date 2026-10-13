---
name: living-docs-enrichment
description: "Utiliser après un rapport, diagnostic, plan, ticket ou review : proposer les découvertes à capitaliser dans docs/wiki/."
---

# Enrichissement du wiki

Après ton rapport ou ton livrable complet (jamais pendant l'analyse), propose les découvertes utiles à un agent futur. Tu ne fais que les consolider, les taguer et les proposer avec `wiki_propose` : l'utilisateur les accepte, les modifie ou les refuse à la carte du checkpoint de commit, puis oh les applique dans `docs/wiki/` (committées avec le code). Dans un workflow sans commit (audit, review…), elles restent en file.

❌ Ne jamais écrire directement dans `docs/wiki/`, ni invoquer le `documentarian` pour enrichir le wiki.
❌ Ne jamais citer `~/.oh/sessions` (données de session d'oh) dans une proposition : oh la refuse ; écris le constat lui-même, depuis le code
❌ Aucune découverte pertinente : ne propose rien, affiche `> 💾 Wiki documentaire : aucune nouvelle découverte à capitaliser.`

## Tags de confiance

| Tag | Quand | Source |
|-----|-------|--------|
| `CONFIRMÉ` | Observation directe dans le code, ligne identifiée | `fichier:ligne` obligatoire |
| `DÉDUIT` | Raisonnement sur plusieurs fichiers, sans ligne précise | `fichier` recommandé |
| `INCERTAIN` | Hypothèse, convention non codée, commentaire | facultative |

## Découvertes et pages cibles

Ce qui se capitalise : pattern ou convention observé mais non documenté, librairie ajoutée, retirée ou à risque, contrainte ou couplage d'architecture, zone fragile, règle métier implicite, zone d'ombre levée ou nouvelle. Sources : coordinateur d'audit, les sections `### Enrichissements wiki proposés` des rapports reçus, consolidées ; developer, après chaque fermeture de ticket.

Pages (relatives à `docs/wiki/`) :
- `index.md` : carte globale seulement (stack critique, architecture en 2 lignes au plus, points critiques 🔴, zones d'ombre) ; une découverte détaillée va dans la page technique ou métier ;
- `technical/conventions.md` : linting, nommage, git, configuration et secrets, patterns de l'équipe, à ne pas utiliser ;
- `technical/architecture.md` : structure, couches, communication entre modules, décisions, points de fragilité ;
- `technical/stack.md` : dépendances, librairies clés, variables d'environnement, contraintes de version ;
- `technical/tests.md` : frameworks, organisation, seuil de couverture, nommage des tests ;
- `technical/glossary.md` : termes du domaine ambigus ou incohérents ;
- `business/<domain>.md` : règles de gestion, flux, entités, risques, zones d'ombre ; domaine absent : propose sur `business/<domain>.md`, oh crée la page si l'utilisateur accepte ; domaine indéterminable : `index.md`, section « Zones d'ombre ».

### Pour l'onboarder

Seulement en re-onboarding, quand `docs/wiki/index.md` existe déjà (Phase 5 de `onboarder-workflow`) : les découvertes du nouveau rapport.

## Proposer

<!-- oh:when mcp=wiki -->
Un appel par enrichissement :

```
wiki_propose({ page: "technical/conventions.md", section: "Patterns spécifiques à l'équipe",
  content: "<constat factuel, une ligne ou un bloc court>", tag: "CONFIRMÉ", source: "src/services/user.service.ts:17" })
```

- `section` : titre de la section cible (créée à la fin si absente).
- `content` : le constat seul, sans titre ni ligne de tag : oh écrit le tag, ton nom (vu par oh), la date et la source.
- Cite les identifiants `wp-…` rendus dans ton rapport, au lieu de recopier les propositions.
- Ne pose pas de question : l'utilisateur décide à la carte du checkpoint de commit.
- Réponse « non enregistrée » (démon oh indisponible) : liste les enrichissements dans `### Enrichissements wiki proposés` de ton rapport.
<!-- /oh:when -->
<!-- oh:when mcp!=wiki -->
Sans l'outil `wiki_propose` dans cette session : liste les enrichissements (page, section, constat, tag et source) dans `### Enrichissements wiki proposés` de ton rapport.
<!-- /oh:when -->

## Qualité

✅ Factuel (fichier, ligne, pattern), concis (une ligne ou un bloc court), tagué avec sa source, absent de la page, compréhensible sans le rapport source.
❌ Ni rapport complet, ni avis subjectif ou spéculatif, ni changement de structure des pages (titres, sections existantes) : du contenu seulement.
