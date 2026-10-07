> [Read in English](050-session-context-capability.en.md)

# ADR-050 — État de session évolutif par capacité de l'adaptateur

## Statut

Accepté

## Date

2026-10-07

## Contexte

L'agent d'entrée d'une session a besoin d'un état qui change pendant la session : checkpoints passés et en cours, budget restant après une décision `$`, consigne de reprise après une mise en veille. Jusqu'ici :

- l'état des checkpoints n'était lisible qu'à la demande, par l'outil MCP `workflow_status` ;
- le budget et la reprise n'étaient signalés par rien ;
- seul le refus d'un checkpoint arrivait à l'agent, en consigne `steer` (`SessionService.Send`), parce qu'opencode 2.0.20 ne transmet pas le message d'un refus de permission.

L'étude S8 (07/10/2026, `02-findings` § « Étude S8 ») a mesuré les **entrées d'instructions par session** d'opencode 2.0.20 (API expérimentale `PUT|DELETE …/session/{id}/instructions/entries/{key}`) :

- une entrée posée avant le premier tour est prise en compte ;
- une modification ou une suppression est annoncée au tour suivant, par un message système qui reste dans l'historique et invalide le cache du prompt à partir de là ;
- les sous-agents ne la reçoivent pas.

Le corps des agents et la carte du workflow restent injectés par le plugin (O2) : leur portée est l'agent, sous-agents compris, avec un préfixe de cache stable. La décision D19 ([ADR-049](049-tool-independence-architecture-guard.fr.md)) impose de passer par une capacité de l'adaptateur.

## Décision

1. **Capacité neutre** (`internal/adapters`) :
   - `Capabilities.SessionContext` ;
   - l'interface optionnelle `SessionContextSetter { SetSessionContext(ctx, h, sessionID, key, value any) ; ClearSessionContext(ctx, h, sessionID, key) }` ;
   - l'erreur `adapters.ErrUnsupported`.
   L'adaptateur opencode l'implémente avec les entrées d'instructions : clé `^[a-z0-9][a-z0-9._-]*$`, valeur JSON d'au plus 256 Kio. Une route absente (404 sans erreur typée, ou 405) donne `ErrUnsupported`, et la capacité est alors coupée pour ce serveur.
2. **Écriture** (`internal/sessionctx`) : la valeur est comparée à la dernière écrite (`~/.oh/sessions/<id>/context.json`), et rien n'est écrit tant qu'elle ne change pas.
3. **Clés neutres et stables** :
   - `oh.checkpoints` (workflow, mode, passés, en cours, suivant, coupe-circuit) : écrite par le démon à chaque transition ;
   - `oh.budget` (limite, relèvements compris, dépensé, restant, épuisé) : écrite à la décision `$`, puis quand la limite change (relèvement), jamais à chaque étape ;
   - `oh.resume` (consigne) : posée par le RunService quand `oh session resume` redémarre le serveur, retirée par le démon après l'étape suivante.
4. **Repli** sans la capacité, ou après `ErrUnsupported` :
   - `oh.resume` est envoyée en message `synthetic` d'oh ;
   - `oh.checkpoints` et `oh.budget` ne sont pas envoyées : l'agent les lit avec `workflow_status`, comme avant.
5. **Le refus d'un checkpoint reste une consigne `steer`** : c'est un ordre ponctuel, pas un état. Les sous-agents ne reçoivent pas l'état de session (limite de l'outil, documentée).

## Conséquences

### Positives

- L'agent d'entrée connaît l'état du workflow, du budget et de la reprise sans le demander, dès l'étape qui suit le changement.
- Peu de messages : une entrée par changement réel, et le budget n'est pas réécrit à chaque étape.
- L'API expérimentale est isolée dans l'adaptateur, avec un repli explicite si elle disparaît.

### Négatives / Compromis

- Chaque changement ajoute un message à l'historique et invalide le cache du prompt à partir de lui.
- Les sous-agents ne voient pas l'état : un sous-agent qui en a besoin doit appeler `workflow_status`.
- Une écriture faite pendant une étape n'est annoncée qu'à l'étape suivante.
- La dernière valeur écrite est gardée dans un fichier partagé par la CLI et le démon ; deux écritures simultanées de la même clé peuvent se recouvrir, et la suivante corrige.

## Alternatives considérées

| Alternative | Rejetée car |
|---|---|
| Injecter l'état par le plugin (`session.hook("context")`, O2) | Le corps du plugin vient du paquet, immuable : l'état ne change pas pendant la session. |
| Envoyer chaque changement en message `synthetic` | Un message à chaque transition et à chaque étape pour le budget, sans gain sur les outils qui gardent un état. |
| Corps d'agent et carte du workflow en entrées d'instructions | Portée par session racine seulement (pas les sous-agents) et cache invalidé (étude S8). |
| Réécrire le budget à chaque étape | Un message par étape dans l'historique. |
