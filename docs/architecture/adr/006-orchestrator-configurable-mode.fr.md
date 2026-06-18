# ADR-006 — Mode de workflow configurable pour l'agent orchestrator dev

## Statut

Accepté

## Contexte

L'ADR-003 a établi un workflow entièrement manuel pour l'agent orchestrator : chaque
checkpoint (`[CP-0]` à `[CP-3]`) attend une réponse explicite de l'utilisateur
avant de continuer. Cette décision était justifiée pour garantir le contrôle et
éviter la propagation d'erreurs.

Cependant, l'usage en conditions réelles a mis en évidence un cas fréquent où
cette rigueur devient une friction sans valeur : les features avec de nombreux
tickets homogènes et bien planifiés (ex. CRUD sur N entités, migrations en série,
tâches de refactoring répétitives). L'utilisateur y tape `oui`, `suivant`, `oui`,
`suivant`... en boucle, sans jamais exercer de jugement réel sur ces étapes.

L'ADR-003 avait explicitement noté le mode configurable comme *"possible en évolution
future"*, sans preuve immédiate de valeur. Cette preuve est maintenant établie.

Depuis, l'agent orchestrator a été scindé en deux niveaux (voir la refonte de l'architecture) :
- `orchestrator` — chef de projet feature (conception → audit → implémentation)
- `orchestrator-dev` — tech lead d'implémentation (developer-* + QA + review)

**Les modes de cette ADR s'appliquent exclusivement à `orchestrator-dev`.**
L'`orchestrator` feature conserve des checkpoints toujours manuels (CP-0, CP-spec,
CP-audit, CP-feature) — aucun de ces checkpoints n'est automatisable.

## Décision

L'`orchestrator-dev` propose **trois modes de workflow**, choisi une fois pour toute la
session au moment du `[CP-0]` :

| Mode | Description |
|------|-------------|
| `manuel` | Comportement original de l'ADR-003 — tous les checkpoints sont des pauses |
| `semi-auto` | CP-1 et CP-3 automatiques, CP-0 / CP-QA / CP-2 restent manuels |
| `auto` | CP-0, CP-1, CP-3 automatiques + CP-QA fixé au démarrage, CP-2 **toujours manuel** |

**Le mode par défaut est `manuel`** — le comportement existant est préservé sans
changement pour les utilisateurs qui ne précisent pas de mode.

**CP-2 (merge ou corriger ?) est non automatisable dans tous les modes.** Cette règle
est absolue : "absence d'erreur technique" ≠ "conforme aux attentes fonctionnelles".
La décision de merger engage la responsabilité de l'utilisateur.

Le mode est déclaré à l'invocation de l'`orchestrator-dev` ou sélectionné au moment
du `[CP-0]`. Quand l'`orchestrator-dev` est invoqué depuis l'`orchestrator`, le mode
est transmis en paramètre.

## Conséquences

### Positives

- Élimine la friction répétitive sur les features avec tickets homogènes
- Préserve le comportement existant par défaut (`manuel`) — rétrocompatible
- CP-2 reste toujours manuel : le risque de propagation d'erreurs identifié dans
  l'ADR-003 est maintenu sous contrôle
- L'utilisateur reste libre de taper "stop" à n'importe quel moment —
  les modes `semi-auto` et `auto` réduisent les pauses mais n'empêchent pas
  d'interrompre
- La scission en deux orchestrateurs clarifie le périmètre : les modes ne concernent
  que la phase d'implémentation, jamais la phase de conception ou d'audit

### Négatives / compromis

- Légère complexité supplémentaire dans le skill `orchestrator-dev-protocol`
- L'utilisateur doit connaître les 3 modes pour en tirer parti (mitigé par la
  question posée au CP-0)

## Alternatives rejetées

**Mode configurable dans `projects.md`** : persistance utile sur des projets avec
un mode préféré, mais introduit un couplage entre la configuration projet et le
comportement d'un agent spécifique. Peut être greffé sur cette décision en
évolution future si le besoin se confirme.

**Suppression de CP-0** : rejetée — CP-0 est le consentement initial au démarrage
du workflow. Le supprimer signifierait qu'une invocation accidentelle de l'agent orchestrator
démarrerait un workflow complet sans confirmation.

**CP-2 automatique avec score de confiance** : rejetée — un score de confiance sur
un rapport de review IA introduit une fausse précision. La décision de merger est
une responsabilité humaine non délégable.
